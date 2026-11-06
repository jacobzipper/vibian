#!/bin/bash
# headless-install.sh ISO WORKDIR
#
# Install a --unattended Vibian ISO into a fresh UEFI VM, boot the result and
# check the disk layout over ssh. Works without KVM (slow TCG), so it can run
# on build machines. Build the ISO with VIBIAN_CMDLINE_EXTRA=console=ttyS0,115200
# so the installer log lands in WORKDIR/install.serial.log.
set -euo pipefail
here=$(dirname "$(realpath "$0")")
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
  echo "motd=$(run-parts --lsbsysinit --test /etc/update-motd.d | tr "\n" " ")"
  echo "issue=$(head -c 6 /etc/issue)"
  echo "nm-wired=$(nmcli -t -f TYPE,STATE device | grep -m1 ^ethernet:)"
  # wifi: a virtual radio pair, a hotspot on one, and the user (no sudo, via
  # the polkit rule) joins it from the other, as onboarding would
  if s modprobe mac80211_hwsim radios=2 2>/dev/null; then
    sleep 3
    s nmcli device wifi hotspot ifname wlan1 ssid vibian-test password correct-horse >/dev/null
    for i in 1 2 3 4 5 6; do
      nmcli device wifi rescan ifname wlan0 2>/dev/null; sleep 5
      nmcli -t -f SSID device wifi list ifname wlan0 | grep -qx vibian-test && break
    done
    nmcli device wifi connect vibian-test password correct-horse ifname wlan0 >/dev/null 2>&1 &&
      echo "wifi=$(nmcli -t -f GENERAL.STATE device show wlan0 | cut -d" " -f2-)"
    s nmcli connection delete vibian-test Hotspot >/dev/null 2>&1
    s modprobe -r mac80211_hwsim
  else
    echo "wifi=no-hwsim"
  fi
  vibian snap "cli test"
  sudo touch /etc/vibian-cli-test
  touch ~/cli-test
  vibian log add "cli test change" --files /etc/vibian-cli-test,~/cli-test --gotcha "only a test"
  vibian log tail
  # the vibian-system MCP server, as a harness would start it
  for m in "\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\",\"capabilities\":{},\"clientInfo\":{\"name\":\"t\",\"version\":\"1\"}}" \
           "\"tools/list\"" "\"tools/call\",\"params\":{\"name\":\"ledger_tail\",\"arguments\":{\"count\":1}}"; do
    echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":$m}"
  done | vibian mcp --agent test | python3 -c "import json,sys; r=[json.loads(l) for l in sys.stdin]; print(\"mcp-tools=%d\" % len(r[1][\"result\"][\"tools\"])); print(\"mcp-ledger=\" + r[2][\"result\"][\"content\"][0][\"text\"].replace(chr(10), \" \"))"
  vibian undo --last --yes
  echo "undo-root=$(test -e /etc/vibian-cli-test && echo kept || echo reverted)"
  echo "undo-home=$(test -e ~/cli-test && echo kept || echo reverted)"
  vibian log tail 1
  # per-turn snapshots, run the way harness hooks run them: a turn that
  # changes nothing keeps only its open pre, the next one that does keeps a
  # pair, and one right after is debounced. Hooks never print to stdout.
  turns() { s snapper -c root list | grep -c "agent turn: Claude Code"; }
  t0=$(turns)
  { vibian hook turn-start --agent claude; vibian hook turn-end --agent claude; } >/tmp/turn.out
  echo "turn-clean=$(( $(turns) - t0 ))"
  { vibian hook turn-start --agent claude; sudo touch /etc/vibian-turn-test; vibian hook turn-end --agent claude; } >>/tmp/turn.out
  echo "turn-kept=$(( $(turns) - t0 ))"
  { vibian hook turn-start --agent claude; sudo rm /etc/vibian-turn-test; vibian hook turn-end --agent claude; } >>/tmp/turn.out
  echo "turn-debounced=$(( $(turns) - t0 )) stdout=$(wc -c </tmp/turn.out)"
  rm -f /tmp/turn.out
  # history joins the ledger to snapshot pairs; a pair nobody logged (this
  # apt install) can be undone by number
  echo "history-ledger=$(vibian history -n 0 | grep -cE "^[0-9-]+ [0-9:]+ +L-0001 ") history-turn=$(vibian history -n 0 | grep -c "agent turn: Claude Code")"
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -q sl >/dev/null 2>&1
  pre=$(vibian history --json -n 1 | python3 -c "import json,sys; c=json.load(sys.stdin)[-1]; print(c[\"snapshots\"][\"root\"][0] if c[\"what\"]==\"apt\" else 0)")
  vibian undo --pair "$pre" --yes >/dev/null
  echo "pair-undo=$(test -e /usr/games/sl && echo kept || echo reverted)"
  # export: secrets scrubbed, private files dropped, gitleaks as the last
  # gate; then apply puts the dotfiles back
  mkdir -p ~/.config/kitty ~/.config/hypr
  printf "font_size 12\nexport GITHUB_TOKEN=ghp_abcdefghijklmnopqrstuvwxyz0123456789AB\n" > ~/.config/kitty/kitty.conf
  printf "SECRET=1\n" > ~/.config/hypr/.env
  printf "bind = SUPER, Q, exec, kitty\n" > ~/.config/hypr/hyprland.conf
  vibian export ~/rice --name test >/tmp/export.out 2>&1; echo "export-rc=$? $(grep -c "gitleaks: no secrets found" /tmp/export.out)"
  echo "export-leaks=$(grep -rl ghp_ ~/rice --exclude-dir=.git | wc -l) env=$(test -e ~/rice/dotfiles/.config/hypr/.env && echo copied || echo dropped) commits=$(git -C ~/rice rev-list --count HEAD)"
  # a fake Stripe key, built at runtime so the repo itself holds none
  printf "STRIPE=sk_%s_51H8abcdefghijklmnopqrstuvwxyzAB\n" live > ~/.config/hypr/pay.conf
  vibian export ~/rice >/tmp/export.out 2>&1; echo "export-gate=$? $(grep -c stripe /tmp/export.out) commits=$(git -C ~/rice rev-list --count HEAD)"
  rm ~/.config/hypr/pay.conf
  vibian export ~/rice >/dev/null 2>&1
  rm ~/.config/kitty/kitty.conf
  vibian apply ~/rice --scripts-only --yes >/tmp/apply.out 2>&1; echo "apply-rc=$? kitty=$(grep -c "redacted by vibian export" ~/.config/kitty/kitty.conf)"
  vibian log tail 1 | head -1
  rm -f /tmp/export.out /tmp/apply.out' 2>&1 | tee cli.log
for want in 'tty1=.*vibian' 'nopasswd=yes' 'doctor-rootfs=ok' 'doctor-snapper=ok' 'ledger-subvol=subvol=/@var_lib_vibian' 'kit=ok' \
            'motd=/etc/update-motd.d/00-vibian /etc/update-motd.d/9' 'issue=Vibian' 'nm-wired=ethernet:connected' 'wifi=\(connected\)|wifi=no-hwsim' \
            'mcp-tools=7' 'mcp-ledger=.*cli test change' \
            'turn-clean=1$' 'turn-kept=2$' 'turn-debounced=2 stdout=0$' \
            'history-ledger=1 history-turn=[1-9]' 'pair-undo=reverted' \
            'export-rc=0 1$' 'export-leaks=0 env=dropped commits=1$' 'export-gate=1 [1-9] commits=1$' 'apply-rc=0 kitty=1$' 'apply rice test' \
            'L-0001 recorded \(snapshots root' 'undo-root=reverted' 'undo-home=reverted' 'undo L-0001'; do
  grep -qE -- "$want" cli.log || { echo "MISSING (cli): $want"; fail=1; }
done
[ $fail = 0 ] || { echo "VM-TEST-FAIL"; exit 1; }

# Computer use: the vibian-computer MCP server on headless Sway and Xvfb.
echo "== computer use"
ssh_vm 'cat > /tmp/computer-drive.py' < "$here/computer-drive.py"
ssh_vm 'bash -s' < "$here/computer-use.sh" 2>&1 | tee computer.log
for want in 'cu-apt=0' 'uinput-rule=present' \
            'sway-windows=2' 'sway-focus=left click=right typed=sway drag=left$' 'sway-shots=1280x720 \[.320x180.\]' 'sway-errors=1 badkey=yes' \
            'uinput-windows=2' 'uinput-focus=left click=right typed=uinput drag=left$' 'uinput-shots=1280x720 \[.320x180.\]' 'uinput-errors=1 badkey=yes' \
            'x11-typed=x11' 'x11-shots=1024x768 \[.200x100.\]' 'x11-errors=0'; do
  grep -qE -- "^$want" computer.log || { echo "MISSING (computer): $want"; fail=1; }
done
[ $fail = 0 ] || { echo "VM-TEST-FAIL"; exit 1; }

# Scoped powers: vibian sudo, the risky list and the owner's approval.
echo "== scoped powers"
ssh_vm 'bash -s' < "$here/scoped-powers.sh" 2>&1 | tee scoped.log
for want in 'set=0 mode=scoped' 'plain-sudo=no' 'plain=0 owner=root' 'nowhy=1' 'false=1' 'mkdir=0' 'risky=3' 'id=[0-9a-f]{8}' \
            'self-approve=1' 'sudo-approve=3' 'direct=3' 'wrapper=3' 'own-program=3' 'listed=[1-9]' 'approve=0' \
            'rerun=0 gone=yes' 'once=3' 'deny=0 left=0' 'ledger-cmd=1 ledger-ok=1 ledger-snap=1' 'restored=root'; do
  grep -qE -- "^$want" scoped.log || { echo "MISSING (scoped): $want"; fail=1; }
done
echo "== desktop playbooks"
: > playbook.log
for id in hyprland sway kde-plasma gnome; do
  ssh_vm "bash -s $id" < "$here/desktop-playbook.sh" 2>&1 | tee -a playbook.log
  greeter='greeter=up vt=tty7'; case $id in kde-plasma|gnome) greeter='greeter=up' ;; esac
  for want in 'listed=1' 'show=0' 'system=0' 'bad-knob=1' 'user=0' 'rerun=0 new-baks=0' 'verify=0' 'user-as-root=1' "$greeter"; do
    grep -qE -- "^$id\.$want" playbook.log || { echo "MISSING (playbook $id): $want"; fail=1; }
  done
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
