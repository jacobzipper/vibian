#!/bin/bash
# Ask overlay check, as the user: the packages, the overlay and its Python
# modules, `vibian asks`, and SUPER+/ in each desktop user.sh found. Prints
# one line per check and VERIFY OK or VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
bad=0
# XFCE's settings live in xfconfd until it saves them: ask it over the bus
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ] && [ -S "/run/user/$(id -u)/bus" ]; then
  DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus; export DBUS_SESSION_BUS_ADDRESS
fi
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
for p in python3-gi gir1.2-gtk-4.0 gir1.2-gtk4layershell-1.0 libgtk4-layer-shell0 libnotify-bin; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
check "vibian-ask installed" "test -x /usr/local/bin/vibian-ask && test -r /usr/local/lib/vibian/ask-overlay.py"
check "GTK 4 and gtk4-layer-shell load in Python" \
  "python3 -c 'import gi; gi.require_version(\"Gtk\", \"4.0\"); gi.require_version(\"Gtk4LayerShell\", \"1.0\"); from gi.repository import Gtk, Gtk4LayerShell; Gtk4LayerShell.get_major_version()'"
check "vibian asks works" "vibian asks --json -n 1"
check "menu entry" "grep -q '^Exec=vibian-ask' $data/applications/vibian-ask.desktop"
n=0
key() { n=$((n + 1)); check "SUPER+/ in $1" "$2"; }
[ -f "$cfg/hypr/hyprland.conf" ] && key Hyprland "grep -q '^bind = SUPER, slash, exec, vibian-ask' $cfg/hypr/local.conf && grep -q '^source = ~/.config/hypr/local.conf' $cfg/hypr/hyprland.conf"
[ -d "$cfg/sway/config.d" ] && key Sway "grep -q '^bindsym Mod4+slash exec vibian-ask' $cfg/sway/config.d/vibian-ask"
[ -d "$cfg/i3/config.d" ] && key i3 "grep -q '^bindsym Mod4+slash exec --no-startup-id vibian-ask' $cfg/i3/config.d/vibian-ask"
[ -f "$cfg/niri/local.kdl" ] && key niri "grep -q 'Mod+Slash.*spawn \"vibian-ask\"' $cfg/niri/local.kdl && niri validate -c $cfg/niri/config.kdl"
if command -v xfconf-query >/dev/null && [ -d "$cfg/xfce4" ]; then
  if [ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
    key XFCE "xfconf-query -c xfce4-keyboard-shortcuts -p '/commands/custom/<Super>slash' | grep -q '^vibian-ask'"
  else
    key XFCE "grep -q 'name=\"&lt;Super&gt;slash\" type=\"string\" value=\"vibian-ask' $cfg/xfce4/xfconf/xfce-perchannel-xml/xfce4-keyboard-shortcuts.xml"
  fi
fi
[ -f "$cfg/kglobalshortcutsrc" ] && command -v kreadconfig6 >/dev/null &&
  key "KDE Plasma" "kreadconfig6 --file $cfg/kglobalshortcutsrc --group services --group vibian-ask.desktop --key _launch | grep -qF 'Meta+/'"
command -v gnome-shell >/dev/null && [ -e "$cfg/dconf/user" ] &&
  key GNOME "dconf read /org/gnome/settings-daemon/plugins/media-keys/custom-keybindings | grep -q vibian-ask && dconf read /org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/vibian-ask/binding | grep -q '<Super>slash'"
[ -d /usr/share/cosmic/com.system76.CosmicSettings.Shortcuts ] &&
  key COSMIC "grep -q 'key: \"slash\"): Spawn(\"vibian-ask' $cfg/cosmic/com.system76.CosmicSettings.Shortcuts/v1/custom"
check "at least one desktop has SUPER+/" "test $n -gt 0"
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
