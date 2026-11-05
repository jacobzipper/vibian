#!/bin/bash
# drive.sh boot | key KEY [N] | shot NAME | ssh CMD... | off
#
# Drive an installed test VM by hand: boot the disk headless-install.sh left in
# $VIBIAN_VM, press keys on its console (QEMU sendkey names: ret, down, y,
# ctrl-d), save screendumps as PNG and run commands over ssh. Used to walk
# through onboarding the way a person would.
cd "${VIBIAN_VM:?set VIBIAN_VM to the headless-install.sh WORKDIR}"
ovmf=/usr/share/OVMF
mon() { echo "$1" | socat -t 1 - UNIX-CONNECT:mon.sock >/dev/null; }
case $1 in
boot)
  qemu-system-x86_64 -machine q35 -accel kvm -cpu max -smp 4 -m 4096 \
    -drive "if=pflash,format=raw,readonly=on,file=$ovmf/OVMF_CODE_4M.fd" \
    -drive if=pflash,format=raw,file=vars.fd -drive file=disk.qcow2,if=virtio,format=qcow2 \
    -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:2222-:22 -display none -no-reboot \
    -serial file:drive.serial.log -monitor unix:mon.sock,server=on,wait=off -daemonize -pidfile vm.pid ;;
key) for _ in $(seq 1 "${3:-1}"); do mon "sendkey $2"; sleep 0.5; done ;;
shot) mon "screendump $PWD/$2.ppm"; sleep 1; python3 -c "from PIL import Image; Image.open('$2.ppm').save('$2.png')" 2>/dev/null || convert "$2.ppm" "$2.png"; echo "$PWD/$2.png" ;;
ssh) shift; sshpass -p vibian ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 -o LogLevel=ERROR vibian@127.0.0.1 "$@" ;;
off) mon system_powerdown ;;
esac
