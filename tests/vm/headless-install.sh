#!/bin/bash
# headless-install.sh ISO WORKDIR
#
# Install a --unattended Vibian ISO into a fresh UEFI VM, boot the result and
# check the disk layout over ssh. Works without KVM (slow TCG), so it can run
# on build machines. Build the ISO with VIBIAN_CMDLINE_EXTRA=console=ttyS0,115200
# so the installer log lands in WORKDIR/install.serial.log.
set -euo pipefail
iso=$(realpath "${1:?usage: headless-install.sh ISO WORKDIR}")
wd=${2:?usage: headless-install.sh ISO WORKDIR}
mkdir -p "$wd" && cd "$wd"

accel=tcg,thread=multi
[ -w /dev/kvm ] && accel=kvm
ovmf=/usr/share/OVMF
qemu=(qemu-system-x86_64 -machine q35 -accel "$accel" -cpu max -smp "${VM_CPUS:-8}" -m "${VM_MEM:-6144}"
      -drive "if=pflash,format=raw,readonly=on,file=$ovmf/OVMF_CODE_4M.fd"
      -drive if=pflash,format=raw,file=vars.fd
      -drive file=disk.qcow2,if=virtio,format=qcow2
      -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:2222-:22
      -display none -no-reboot)

rm -f disk.qcow2 vars.fd
qemu-img create -f qcow2 disk.qcow2 40G >/dev/null
cp "$ovmf/OVMF_VARS_4M.fd" vars.fd

# a cancelled job must not leave a VM holding the disk and the ssh port
trap 'pkill -P $$ 2>/dev/null; [ -f install.pid ] && kill "$(cat install.pid)" 2>/dev/null; true' EXIT TERM INT
echo "== install ($accel) $(date -u +%T)"
# serial goes to a log and a socket, so a stuck installer can be poked with
# `socat - UNIX-CONNECT:WORKDIR/serial.sock`
timeout "${INSTALL_TIMEOUT:-4h}" "${qemu[@]}" -cdrom "$iso" -boot d \
  -chardev socket,id=s0,path=serial.sock,server=on,wait=off,logfile=install.serial.log \
  -serial chardev:s0 -pidfile install.pid
echo "== installer exited $(date -u +%T)"

# the QEMU monitor is how the test sees the screen and presses keys
mon() { echo "$1" | socat -t 1 - UNIX-CONNECT:mon.sock >/dev/null; }
press() { local k=$1 n=${2:-1}; for _ in $(seq 1 "$n"); do mon "sendkey $k"; sleep 0.4; done; }
boot() { "${qemu[@]}" -serial "file:$1" -monitor unix:mon.sock,server=on,wait=off -daemonize -pidfile vm.pid; }
wait_off() { for _ in $(seq 1 60); do [ -e vm.pid ] && kill -0 "$(cat vm.pid)" 2>/dev/null || break; sleep 2; done; }

echo "== first boot"
boot boot.serial.log
trap 'kill "$(cat vm.pid)" 2>/dev/null; true' EXIT TERM INT

ssh_vm() {
  sshpass -p vibian ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o ConnectTimeout=10 -o LogLevel=ERROR vibian@127.0.0.1 "$@"
}
for _ in $(seq 1 180); do ssh_vm true 2>/dev/null && break; sleep 10; done
ssh_vm true

echo "== checks"
ssh_vm 'set -x
  s() { echo vibian | sudo -S -p "" "$@"; }
  findmnt -t btrfs -o TARGET,OPTIONS
  s btrfs subvolume list /
  cat /etc/fstab
  s snapper --csvout list-configs
  s apt-get install -y -q cowsay >/dev/null
  s snapper --csvout -c root list --columns number,description
  sleep 10  # grub-btrfsd regenerates the menu after each snapshot
  echo "grub-btrfsd=$(systemctl is-active grub-btrfsd)"
  echo "grub-cfg-hooks=$(s grep -c snapshots-btrfs /boot/grub/grub.cfg)"
  echo "grub-snapshot-entries=$(s grep -c "@snapshots/" /boot/grub/grub-btrfs.cfg)"
  echo "initrd-overlayfs=$(s lsinitrd 2>/dev/null | grep -c overlayfs)"
  echo "snapshot-cmdline=$(s grep -c "rd.overlay=1" /boot/grub/grub-btrfs.cfg)"
  cat /proc/cmdline
  ls /var/log/installer/vibian-layout.log && tail -2 /var/log/installer/vibian-layout.log' 2>&1 | tee checks.log

fail=0
for want in 'subvol=/@,' 'subvol=/@home' 'subvol=/@snapshots' 'subvol=/@home_snapshots' 'subvol=/@var_log' \
            'root,/' 'home,/home' 'apt' 'vibian: btrfs layout done'; do
  grep -qF -- "$want" checks.log || { echo "MISSING: $want"; fail=1; }
done
for want in 'grub-btrfsd=active' 'grub-cfg-hooks=[1-9]' 'grub-snapshot-entries=[1-9]' 'initrd-overlayfs=[1-9]' 'snapshot-cmdline=[1-9]'; do
  grep -qE -- "^$want" checks.log || { echo "MISSING: $want"; fail=1; }
done
[ $fail = 0 ] || { echo "VM-TEST-FAIL"; exit 1; }

# The vibian CLI: tty1 autologin into onboarding, doctor, and a snap → change
# → log → undo round trip.
echo "== vibian cli"
mon "screendump $PWD/onboard.ppm"
ssh_vm 'set -x
  s() { echo vibian | sudo -S -p "" "$@"; }
  vibian version
  echo "tty1=$(ps -t tty1 -o comm= | tr "\n" " ")"
  s vibian powers root
  echo "nopasswd=$(sudo -n true 2>/dev/null && echo yes || echo no)"
  vibian doctor || true
  vibian doctor --json | python3 -c "import json,sys; c={x[\"id\"]: x[\"status\"] for x in json.load(sys.stdin)}; print(\"doctor-rootfs=\" + c[\"rootfs\"]); print(\"doctor-snapper=\" + c[\"snapper\"])"
  echo "ledger-subvol=$(findmnt -no OPTIONS /var/lib/vibian | grep -o "subvol=[^,]*")"
  test -f /usr/share/vibian/agent/skills/recovery/SKILL.md && echo kit=ok
  vibian snap "cli test"
  sudo touch /etc/vibian-cli-test
  touch ~/cli-test
  vibian log add "cli test change" --files /etc/vibian-cli-test,~/cli-test --gotcha "only a test"
  vibian log tail
  vibian undo --last --yes
  echo "undo-root=$(test -e /etc/vibian-cli-test && echo kept || echo reverted)"
  echo "undo-home=$(test -e ~/cli-test && echo kept || echo reverted)"
  vibian log tail 1' 2>&1 | tee cli.log
for want in 'tty1=.*vibian' 'nopasswd=yes' 'doctor-rootfs=ok' 'doctor-snapper=ok' 'ledger-subvol=subvol=/@var_lib_vibian' 'kit=ok' \
            'L-0001 recorded \(snapshots root' 'undo-root=reverted' 'undo-home=reverted' 'undo L-0001'; do
  grep -qE -- "$want" cli.log || { echo "MISSING (cli): $want"; fail=1; }
done
[ $fail = 0 ] || { echo "VM-TEST-FAIL"; exit 1; }

# Snapshot boot: take a snapshot, break the system, then pick the snapshot in
# the GRUB menu with the arrow keys (like a person would) and check we're back
# in the pre-break state.
echo "== snapshot boot"
ssh_vm 'set -x
  s() { echo vibian | sudo -S -p "" "$@"; }
  n=$(s snapper -c root create -d pre-break --print-number)
  s touch /etc/vibian-broken
  s apt-get remove -y -q cowsay >/dev/null
  sleep 10
  echo "snapshot-menu=$(s grep -c "Vibian snapshots" /boot/grub/grub.cfg)"
  # menu positions: snapshot n in the snapshots list, the running kernel in its submenu
  s awk -v p="@snapshots/$n/snapshot " -v k="vmlinuz-$(uname -r)" "
    /^(menuentry|submenu) /{i++; in_snap=index(\$0,p)>0; j=0; if (in_snap) print \"snap-pos=\" i-1}
    in_snap && /^    menuentry /{j++; if (index(\$0,k)) print \"kernel-pos=\" j}" /boot/grub/grub-btrfs.cfg
  s systemctl poweroff' 2>&1 | tee snapshot.log || true
wait_off
snap_pos=$(sed -n 's/^snap-pos=//p' snapshot.log | head -1)
kernel_pos=$(sed -n 's/^kernel-pos=//p' snapshot.log | head -1)
echo "snapshot at menu position ${snap_pos:-?}, kernel at ${kernel_pos:-?}"

boot snapshot.serial.log
# "End" is harmless until GRUB shows up and then stops the countdown on the
# last entry, which is the snapshots submenu
wait_s=10; [ "$accel" = kvm ] || wait_s=120
for _ in $(seq 1 $((wait_s * 2))); do mon "sendkey end"; sleep 0.5; done
press ret; sleep 2
press down "${snap_pos:-1}"; press ret; sleep 2
press down "${kernel_pos:-1}"; press ret
for _ in $(seq 1 60); do ssh_vm true 2>/dev/null && break; sleep 5; done
ssh_vm 'set -x
  cat /proc/cmdline
  findmnt -no FSTYPE,SOURCE /
  echo "root-fs=$(findmnt -no FSTYPE /)"
  echo "broken-marker=$(test -e /etc/vibian-broken && echo present || echo gone)"
  echo "cowsay=$(test -x /usr/games/cowsay && echo present || echo gone)"
  echo "root-writable=$(echo vibian | sudo -S -p "" touch /etc/vibian-overlay-test && echo yes || echo no)"
  systemctl --failed --no-legend
  echo "system-state=$(systemctl is-system-running)"' 2>&1 | tee -a snapshot.log

for want in 'snapshot-menu=[1-9]' 'snap-pos=[0-9]' 'kernel-pos=[0-9]' 'subvol=@snapshots/' 'root-fs=overlay' 'broken-marker=gone' 'cowsay=present' 'root-writable=yes' 'system-state=running'; do
  grep -qE -- "$want" snapshot.log || { echo "MISSING (snapshot boot): $want"; fail=1; }
done
[ $fail = 0 ] || { echo "VM-TEST-FAIL"; exit 1; }

# Keep it: from the snapshot boot, make the snapshot the system, then boot
# normally and check the pre-break state stuck.
echo "== rollback"
snap_n=$(grep -o '@snapshots/[0-9]*/snapshot' snapshot.log | head -1 | cut -d/ -f2)
ssh_vm "set -x
  vibian undo --to $snap_n --yes
  vibian log tail 1
  sudo systemctl poweroff" 2>&1 | tee rollback.log || true
wait_off
boot rollback.serial.log
for _ in $(seq 1 60); do ssh_vm true 2>/dev/null && break; sleep 5; done
ssh_vm 'set -x
  cat /proc/cmdline
  echo "root-subvol=$(findmnt -no OPTIONS / | grep -o "subvol=[^,]*")"
  echo "broken-marker=$(test -e /etc/vibian-broken && echo present || echo gone)"
  echo "cowsay=$(test -x /usr/games/cowsay && echo present || echo gone)"
  sudo btrfs subvolume list / | grep -E "@rollback-|[0-9] path @$"
  echo "system-state=$(systemctl is-system-running --wait)"' 2>&1 | tee -a rollback.log
for want in 'roll back / to snapshot' 'root-subvol=subvol=/@$' 'broken-marker=gone' 'cowsay=present' 'path @rollback-' 'system-state=running'; do
  grep -qE -- "$want" rollback.log || { echo "MISSING (rollback): $want"; fail=1; }
done
[ $fail = 0 ] && echo "VM-TEST-PASS" || { echo "VM-TEST-FAIL"; exit 1; }
