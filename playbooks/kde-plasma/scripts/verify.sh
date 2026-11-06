#!/bin/bash
# KDE Plasma playbook check: packages, the login screen and the user's
# settings. Prints one line per check and VERIFY OK or VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
for p in plasma-desktop plasma-workspace kwin-wayland plasma-session-wayland sddm sddm-theme-breeze konsole dolphin xdg-desktop-portal-kde; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
check "Plasma Wayland session" "test -f /usr/share/wayland-sessions/plasma.desktop"
check "Breeze looks are installed" "test -d /usr/share/plasma/look-and-feel/org.kde.breezedark.desktop"
check "SDDM is the login screen" "readlink -f /etc/systemd/system/display-manager.service | grep -q sddm"
check "SDDM greeter on Wayland" "grep -q '^DisplayServer=wayland' /etc/sddm.conf.d/10-vibian.conf"
check "no tiling-desktop bar in Plasma" "! systemctl --global is-enabled -q waybar.service"
check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
check "keyboard layout set" "test -n \"\$(kreadconfig6 --file $cfg/kxkbrc --group Layout --key LayoutList)\""
check "screen lock configured" "test -n \"\$(kreadconfig6 --file $cfg/kscreenlockerrc --group Daemon --key Autolock)\""
check "SUPER+A runs the agent" "kreadconfig6 --file $cfg/kglobalshortcutsrc --group services --group vibian-agent.desktop --key _launch | grep -q Meta+A"
check "agent launcher" "grep -q 'vibian agent' $data/applications/vibian-agent.desktop"
check "look script" "bash -n $data/vibian/kde-apply.sh"
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
