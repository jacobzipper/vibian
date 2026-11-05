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

echo "== first boot"
"${qemu[@]}" -serial file:boot.serial.log -daemonize -pidfile vm.pid
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
  cat /proc/cmdline
  ls /var/log/installer/vibian-layout.log && tail -2 /var/log/installer/vibian-layout.log' 2>&1 | tee checks.log

fail=0
for want in 'subvol=/@,' 'subvol=/@home' 'subvol=/@snapshots' 'subvol=/@home_snapshots' 'subvol=/@var_log' \
            'root,/' 'home,/home' 'apt' 'vibian: btrfs layout done'; do
  grep -qF -- "$want" checks.log || { echo "MISSING: $want"; fail=1; }
done
[ $fail = 0 ] && echo "VM-TEST-PASS" || { echo "VM-TEST-FAIL"; exit 1; }
