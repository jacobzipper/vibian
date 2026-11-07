#!/bin/bash
# Ask overlay playbook, user part: SUPER+/ opens the overlay in every desktop
# a Vibian playbook has set up for this user (Hyprland and the Quickshell
# desk, Sway, niri, i3, XFCE, KDE Plasma, GNOME, COSMIC), plus a menu entry.
# Never run as root. Each binding is one marked line or block, so a re-run
# replaces it and the rest of the user's settings stay. Knobs: VIBIAN_ACCENT,
# VIBIAN_BG.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
for v in accent bg; do
  [[ ${!v} =~ ^[0-9a-fA-F]{6}$ ]] || { echo "VIBIAN_${v^^} must be 6 hex digits, got '${!v}'" >&2; exit 1; }
done
command -v vibian-ask >/dev/null || { echo "vibian-ask is missing: run system.sh first" >&2; exit 1; }

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
cmd="vibian-ask --accent $accent --bg $bg"
mark='vibian playbook ask-overlay'
gnome=0 xfce=0
command -v gnome-shell >/dev/null && [ -e "$cfg/dconf/user" ] && gnome=1
command -v xfconf-query >/dev/null && [ -d "$cfg/xfce4" ] && xfce=1
# GNOME's dconf and XFCE's xfconf write through the session bus: the running
# session's, or a private one over SSH or a TTY
if [ $((gnome + xfce)) -gt 0 ] && [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
  if [ -S "/run/user/$(id -u)/bus" ]; then DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus; export DBUS_SESSION_BUS_ADDRESS
  else exec dbus-run-session -- bash "$0" "$@"; fi
fi

bound=()
# drop_marked FILE: remove this playbook's lines (and // blocks) from FILE
drop_marked() {
  awk -v m="$mark" 'index($0, "// " m " {") { skip = 1 } !skip && !index($0, m) { print } index($0, "// } " m) { skip = 0 }' "$1" > "$1.vibian-tmp"
  cat "$1.vibian-tmp" > "$1" && rm -f "$1.vibian-tmp"
}

# a menu entry, so the launchers find it too
mkdir -p "$data/applications"
cat > "$data/applications/vibian-ask.desktop" <<T
[Desktop Entry]
# from the $mark
Type=Application
Name=Ask your agent
Comment=Send a request to this computer's agent
Exec=$cmd
Icon=/usr/share/vibian/vibian-logo.png
Categories=System;
T

# Hyprland and the Quickshell desk: local.conf is the user's file
if [ -f "$cfg/hypr/hyprland.conf" ]; then
  f=$cfg/hypr/local.conf
  touch "$f"; drop_marked "$f"
  printf 'bind = SUPER, slash, exec, %s # %s\n' "$cmd" "$mark" >> "$f"
  bound+=(hyprland)
fi
# Sway and i3 include config.d/*
if [ -d "$cfg/sway/config.d" ]; then
  printf '# from the %s; its user.sh rewrites this file\nbindsym Mod4+slash exec %s\n' "$mark" "$cmd" > "$cfg/sway/config.d/vibian-ask"
  bound+=(sway)
fi
if [ -d "$cfg/i3/config.d" ]; then
  # X11 has no layer shell: the overlay is a window, so it floats
  printf '# from the %s; its user.sh rewrites this file\nbindsym Mod4+slash exec --no-startup-id %s\nfor_window [title="^Ask your agent$"] floating enable, border pixel 2\n' \
    "$mark" "$cmd" > "$cfg/i3/config.d/vibian-ask"
  bound+=(i3)
fi
# niri: a binds block in local.kdl, which config.kdl includes last
if [ -f "$cfg/niri/local.kdl" ]; then
  f=$cfg/niri/local.kdl
  drop_marked "$f"
  printf '// %s {\nbinds {\n    Mod+Slash hotkey-overlay-title="Ask your agent" { spawn "vibian-ask" "--accent" "%s" "--bg" "%s"; }\n}\n// } %s\n' \
    "$mark" "$accent" "$bg" "$mark" >> "$f"
  bound+=(niri)
fi
if [ $xfce = 1 ]; then
  xfconf-query -c xfce4-keyboard-shortcuts -p "/commands/custom/<Super>slash" -n -t string -s "$cmd"
  bound+=(xfce)
fi
# KDE: kglobalaccel launches the menu entry
if [ -f "$cfg/kglobalshortcutsrc" ] && command -v kwriteconfig6 >/dev/null; then
  kwriteconfig6 --file "$cfg/kglobalshortcutsrc" --group services --group vibian-ask.desktop --key _launch "Meta+/"
  bound+=(kde-plasma)
fi
if [ $gnome = 1 ]; then
  mk=/org/gnome/settings-daemon/plugins/media-keys
  path=$mk/custom-keybindings/vibian-ask/
  list=$(dconf read $mk/custom-keybindings)
  if ! grep -qF "'$path'" <<<"$list"; then
    case $list in ''|'@as []') list="['$path']" ;; *) list="${list%]}, '$path']" ;; esac
    dconf write $mk/custom-keybindings "$list"
  fi
  dconf write "${path}name" "'Ask your agent'"
  dconf write "${path}command" "'$cmd'"
  dconf write "${path}binding" "'<Super>slash'"
  bound+=(gnome)
fi
# COSMIC: one line in the custom shortcuts map. The last duplicate key wins
# in RON, so it goes first: a SUPER+/ of the user's own stays theirs.
f=$cfg/cosmic/com.system76.CosmicSettings.Shortcuts/v1/custom
if [ -d "$cfg/cosmic/com.system76.CosmicSettings.Shortcuts" ] || [ -d /usr/share/cosmic/com.system76.CosmicSettings.Shortcuts ]; then
  mkdir -p "$(dirname "$f")"
  line="    (modifiers: [Super], key: \"slash\"): Spawn(\"$cmd\"), // $mark"
  if [ ! -s "$f" ] || [ "$(tr -d ' \n\t' < "$f")" = "{}" ]; then
    printf '{\n%s\n}\n' "$line" > "$f"
  else
    # cosmic-settings may rewrite the file without the comment
    drop_marked "$f"; grep -vF 'Spawn("vibian-ask ' "$f" > "$f.vibian-tmp" || true
    cat "$f.vibian-tmp" > "$f"
    L=$line awk '!done && i = index($0, "{") { $0 = substr($0, 1, i) "\n" ENVIRON["L"] substr($0, i + 1); done = 1 } 1' "$f" > "$f.vibian-tmp"
    cat "$f.vibian-tmp" > "$f" && rm -f "$f.vibian-tmp"
  fi
  bound+=(cosmic)
fi

[ ${#bound[@]} -gt 0 ] || { echo "no desktop set up by a Vibian playbook: run one first (vibian playbooks)" >&2; exit 1; }
echo "SUPER+/ opens the overlay in: ${bound[*]}"
