#!/bin/bash
# GNOME playbook check: packages, the login screen and the user's settings.
# Prints one line per check and VERIFY OK or VERIFY FAILED.
set -u
data=${XDG_DATA_HOME:-$HOME/.local/share}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
for p in gnome-shell gdm3 gnome-control-center xdg-desktop-portal-gnome ptyxis nautilus dconf-cli; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
check "a GNOME Wayland session" "ls /usr/share/wayland-sessions/gnome.desktop /usr/share/wayland-sessions/ubuntu.desktop 2>/dev/null | grep -q ."
check "GDM is the login screen" "readlink -f /etc/systemd/system/display-manager.service | grep -q gdm"
check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
check "no tiling-desktop bar in GNOME" "! systemctl --global is-enabled -q waybar.service"
check "wallpaper set" "dconf read /org/gnome/desktop/background/picture-uri | grep -q vibian"
check "accent and scheme set" "dconf read /org/gnome/desktop/interface/accent-color | grep -q ."
check "SUPER+Return opens a terminal" "dconf read /org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/vibian-terminal/binding | grep -q Super"
check "SUPER+A runs the agent" "dconf read /org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/vibian-agent/command | grep -q 'vibian agent'"
check "SUPER+A no longer opens the app grid" "dconf read /org/gnome/shell/keybindings/toggle-application-view | grep -q '\\[\\]'"
check "old settings backed up" "test -f $data/vibian/gnome-dconf.vibian-bak.ini"
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
