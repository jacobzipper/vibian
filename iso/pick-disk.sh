#!/bin/bash
# pick-disk.sh — choose the disk Vibian installs to (an autoinstall early-command).
#
# The storage config in autoinstall.yaml says `match: {size: largest}`. This
# swaps that for the path of one disk. With more than one disk to choose from it
# asks on a spare virtual terminal before the installer's own screens start.
# subiquity re-reads /autoinstall.yaml after the early-commands, so the
# installer's "Confirm destructive action" screen still comes afterwards.
#
# vibian.disk=/dev/X on the kernel command line picks a disk without asking
# (serial consoles, scripted installs).
set -euo pipefail

conf=${VIBIAN_AUTOINSTALL:-/autoinstall.yaml}
min=$((25 * 1000 * 1000 * 1000))  # docs/INSTALL.md: 25 GB at the very least

# The install medium never counts, whether it's a USB stick or a CD drive.
media=$(findmnt -no SOURCE /cdrom 2>/dev/null || true)
media_disk=""
if [ -n "$media" ]; then
  media_disk=$(lsblk -no PKNAME "$media" 2>/dev/null | head -1)
  [ -n "$media_disk" ] || media_disk=$(basename "$media")
fi

disks=()
while read -r name type size ro; do
  case $name in zram*) continue ;; esac
  if [ "$type" = disk ] && [ "$ro" = 0 ] && [ "$size" -ge "$min" ] && [ "$name" != "$media_disk" ]; then
    disks+=("$name")
  fi
done < <(lsblk -dnbo NAME,TYPE,SIZE,RO)

use() {
  sed -i "s|match: {size: largest}|path: /dev/$1|" "$conf"
  grep -qF "path: /dev/$1" "$conf"
  echo "vibian: installing to /dev/$1"
}

# no disk is big enough: leave it to the installer to say so
[ "${#disks[@]}" -gt 0 ] || exit 0

want=$(sed -n 's/.*\bvibian\.disk=\([^ ]*\).*/\1/p' /proc/cmdline)
if [ -n "$want" ]; then
  want=${want#/dev/}
  for d in "${disks[@]}"; do
    if [ "$d" = "$want" ]; then use "$d"; exit 0; fi
  done
  echo "vibian: vibian.disk=$want is not a disk Vibian can install to (${disks[*]})" >&2
  exit 1
fi

if [ "${#disks[@]}" = 1 ]; then use "${disks[0]}"; exit 0; fi

# One menu line per disk: size, model or transport, and what's on it now.
items=()
largest=""
largest_size=0
for d in "${disks[@]}"; do
  read -r size model < <(lsblk -dnbo SIZE,MODEL "/dev/$d")
  tran=$(lsblk -dno TRAN "/dev/$d")
  have=$(lsblk -nlo FSTYPE "/dev/$d" | sed '/^$/d' | sort -u | paste -sd, -)
  desc="$(numfmt --to=si "$size" | sed "s/[A-Z]$/ &B/")  ${model:-${tran:-disk}}  (${have:-empty})"
  items+=("/dev/$d" "$desc")
  if [ "$size" -gt "$largest_size" ]; then largest=/dev/$d; largest_size=$size; fi
done

# whiptail runs on its own VT. openvt makes that VT its terminal, so the answer
# (whiptail's --output-fd 3) and the exit status come back through a file.
vt=8
back=$(fgconsole 2>/dev/null || echo 1)
answer=$(mktemp)
trap 'rm -f "$answer"; chvt "$back" || true' EXIT
pick=""
ask() {
  : >"$answer"
  # shellcheck disable=SC2016 # expanded by the inner bash
  TERM=linux openvt -c "$vt" -f -s -w -- bash -c '"$@" 3>"$0"; echo " $?" >>"$0"' "$answer" "$@" || true
  local rc
  read -r pick rc <"$answer" || true
  [ -n "$rc" ] || { rc=$pick; pick=""; }
  if [ -z "$rc" ]; then
    echo "vibian: no console to ask on; boot with vibian.disk=/dev/X (one of: ${disks[*]})" >&2
    exit 1
  fi
  return "$rc"
}
while true; do
  if ask whiptail --output-fd 3 --title "Vibian" --default-item "$largest" --ok-button "Use this disk" \
    --cancel-button "Power off" --menu "\nWhich disk should Vibian install to?\n\nEverything on it is erased. The installer asks once more before it starts.\n" \
    20 76 8 "${items[@]}"; then
    [ -n "$pick" ] && break
  elif ask whiptail --title "Vibian" --yes-button "Power off" --no-button "Back" \
    --yesno "\nPower off without installing anything?" 9 50; then
    poweroff
    exit 1
  fi
done
use "${pick#/dev/}"
