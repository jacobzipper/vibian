#!/bin/bash
# The quickshell-desk playbook in the rice's colours, with its wallpaper on
# the desktop and the lock screen. Builds Quickshell first (with sudo) if
# it isn't here. The desk keeps the rice's theme.json: the playbook never
# overwrites it.
set -euo pipefail
pb=${VIBIAN_PLAYBOOKS:-/usr/share/vibian/playbooks}/quickshell-desk
[ -f "$pb/scripts/user.sh" ] || { echo "no quickshell-desk playbook in $pb" >&2; exit 1; }
if ! command -v qs >/dev/null; then
  echo "building Quickshell (15-30 minutes, once)"
  sudo bash "$pb/scripts/system.sh"
fi
wall=${XDG_DATA_HOME:-$HOME/.local/share}/backgrounds/nhk/room.jpg
[ -f "$wall" ] || wall=
export VIBIAN_ACCENT=${VIBIAN_ACCENT:-ff79b0} VIBIAN_BG=${VIBIAN_BG:-16121f} VIBIAN_WALLPAPER=${VIBIAN_WALLPAPER:-$wall}
bash "$pb/scripts/user.sh"
bash "$pb/scripts/verify.sh"
