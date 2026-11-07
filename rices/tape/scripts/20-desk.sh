#!/bin/bash
# The hyprland playbook in amber on black, then tape on workspace 1: it
# starts full screen at login and SUPER+B brings it back. Keeps the
# quickshell desk if that's what's running. The tape lines go in
# ~/.config/hypr/local.conf, between markers, so rerunning is safe.
set -euo pipefail
pb=${VIBIAN_PLAYBOOKS:-/usr/share/vibian/playbooks}/hyprland
[ -f "$pb/scripts/user.sh" ] || { echo "no hyprland playbook in $pb" >&2; exit 1; }
if ! command -v Hyprland >/dev/null && ! command -v hyprland >/dev/null; then
  echo "Hyprland isn't installed: run $pb/scripts/system.sh with sudo first" >&2
  exit 1
fi
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
tape=$HOME/.local/bin/tape
[ -x "$tape" ] || { echo "$tape is missing: copy the rice's dotfiles first" >&2; exit 1; }
"$tape" --demo --dump 120x40 >/dev/null  # it runs here

desk=${VIBIAN_DESK:-}
[ -n "$desk" ] || { grep -qs 'qs -c vibian-desk' "$cfg/hypr/hyprland.conf" && desk=quickshell || desk=waybar; }
export VIBIAN_ACCENT=${VIBIAN_ACCENT:-ffb000} VIBIAN_BG=${VIBIAN_BG:-000000} VIBIAN_DESK=$desk
bash "$pb/scripts/user.sh"

local=$cfg/hypr/local.conf
touch "$local"
sed -i '/^# >>> tape rice/,/^# <<< tape rice/d' "$local"
cat >>"$local" <<EOF2
# >>> tape rice (scripts/20-desk.sh rewrites this block)
windowrule = fullscreen on, match:class ^tape\$
exec-once = [workspace 1] foot --app-id tape $tape
bind = \$mod, B, exec, hyprctl clients | grep -q 'class: tape\$' && hyprctl dispatch focuswindow class:^tape\$ || foot --app-id tape $tape
# <<< tape rice
EOF2
bash "$pb/scripts/verify.sh"
if [ -n "${HYPRLAND_INSTANCE_SIGNATURE:-}" ]; then hyprctl reload >/dev/null || true; fi
echo "tape: SUPER+B opens it; it starts on workspace 1 at login"
