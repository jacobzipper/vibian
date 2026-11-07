#!/bin/bash
# Quickshell desk playbook, user part: the hyprland playbook's user.sh with
# the desk in place of waybar and fuzzel's SUPER+Space, then the desk itself
# in ~/.config/quickshell/vibian-desk/ and its settings in desk.json there
# (theme.json there is the look: created empty, never overwritten).
# Never run as root. Every hyprland knob works here too; the desk adds
# VIBIAN_WIDGETS. A desk folder that wasn't Vibian's is kept as
# vibian-desk.vibian-bak (only the first time).
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
here=$(cd "$(dirname "$0")/.." && pwd)
widgets=${VIBIAN_WIDGETS:-clock,ask,keys,system,changes}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
term=${VIBIAN_TERMINAL:-foot}
mod=${VIBIAN_MOD:-SUPER}
[[ $widgets =~ ^(none|(clock|ask|keys|system|changes)(,(clock|ask|keys|system|changes))*)$ ]] ||
  { echo "VIBIAN_WIDGETS: none, or a comma list of clock, ask, keys, system, changes" >&2; exit 1; }
command -v qs >/dev/null || { echo "Quickshell isn't installed (run system.sh first)" >&2; exit 1; }

# checks the shared knobs (accent, terminal, mod…) and writes Hyprland's files
VIBIAN_DESK=quickshell bash "$here/../hyprland/scripts/user.sh"

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
desk=$cfg/quickshell/vibian-desk
if [ -d "$desk" ] && [ ! -e "$desk/.vibian" ] && [ ! -e "$desk.vibian-bak" ]; then
  mv "$desk" "$desk.vibian-bak"
fi
mkdir -p "$desk"
cp "$here"/files/vibian-desk/*.qml "$desk/"
echo "the vibian playbook quickshell-desk wrote this folder; user.sh rewrites the .qml files" > "$desk/.vibian"
[ "$widgets" = none ] && widgets=
list=$(printf '%s' "$widgets" | sed 's/[a-z]\+/"&"/g')
cat > "$desk/desk.json" <<EOF2
{
  "accent": "$accent",
  "background": "$bg",
  "terminal": "$term",
  "mod": "${mod//_/+}",
  "widgets": [$list]
}
EOF2
# the look, for themes and rices: written once, never overwritten
[ -e "$desk/theme.json" ] || printf '{}\n' > "$desk/theme.json"
echo "wrote the desk to $desk (widgets: ${widgets:-none})"
