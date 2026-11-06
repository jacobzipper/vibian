#!/bin/bash
# XFCE playbook check: packages, xfconf settings and the login setup. Prints
# one line per check and VERIFY OK or VERIFY FAILED.
set -u
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
  if [ -S "/run/user/$(id -u)/bus" ]; then export DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus
  else exec dbus-run-session -- bash "$0" "$@"; fi
fi
q() { xfconf-query -c "$1" -p "$2" 2>/dev/null; }
for p in xserver-xorg-core xfce4-session xfwm4 xfce4-panel xfdesktop4 xfce4-settings thunar xfce4-terminal \
         xfce4-screensaver xfce4-power-manager xfce4-whiskermenu-plugin xfce4-notifyd greybird-gtk-theme xcape; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
for t in whiskermenu tasklist systray pulseaudio power-manager-plugin notification-plugin xfce4-clipman-plugin clock actions; do
  check "panel has $t" "xfconf-query -c xfce4-panel -l -v | grep -q ' $t\$'"
done
check "panel plugins are installed" "! for t in \$(xfconf-query -c xfce4-panel -l -v | awk '\$1 ~ /^\/plugins\/plugin-[0-9]+\$/ {print \$2}'); do [ \$t = separator ] || ls /usr/share/xfce4/panel/plugins/\$t.desktop >/dev/null 2>&1 || echo \$t; done | grep -q ."
check "GTK theme is installed" "test -d /usr/share/themes/\$(q xsettings /Net/ThemeName)/gtk-3.0"
check "window theme is installed" "test -d /usr/share/themes/\$(q xfwm4 /general/theme)/xfwm4"
check "icon theme is installed" "test -d /usr/share/icons/\$(q xsettings /Net/IconThemeName)"
check "keyboard layout set" "test -n \"\$(q keyboard-layout /Default/XkbLayout)\""
check "SUPER+Return opens a terminal" "q xfce4-keyboard-shortcuts '/commands/custom/<Super>Return' | grep -q terminal"
check "SUPER+A opens the agent" "q xfce4-keyboard-shortcuts '/commands/custom/<Super>a' | grep -q 'vibian agent'"
check "SUPER opens the menu (xcape, Alt+F1)" "q xfce4-keyboard-shortcuts '/commands/custom/<Alt>F1' | grep -q whiskermenu && grep -q 'Super_L=Alt_L|F1' \${XDG_CONFIG_HOME:-\$HOME/.config}/autostart/vibian-xcape.desktop"
check "no Super_L grab (it blocks SUPER+arrow tiling)" "! q xfce4-keyboard-shortcuts /commands/custom/Super_L"
check "SUPER+arrows tile" "(for d in Left Right Up Down; do q xfce4-keyboard-shortcuts \"/xfwm4/custom/<Super>\$d\" | grep -q tile_ || exit 1; ! q xfce4-keyboard-shortcuts \"/xfwm4/custom/<Super>KP_\$d\" || exit 1; done)"
check "lock is configured" "test -n \"\$(q xfce4-screensaver /lock/enabled)\""
check "wallpaper login script" "test -x \${XDG_DATA_HOME:-\$HOME/.local/share}/vibian/xfce-apply.sh"
check "no Wayland bar in every session" "! systemctl --global is-enabled -q waybar.service"
check "xfce session file" "test -f /usr/share/xsessions/xfce.desktop"
check "lightdm enabled" "systemctl is-enabled -q lightdm"
check "LightDM is the login screen" "readlink -f /etc/systemd/system/display-manager.service | grep -q lightdm"
check "lightdm starts XFCE" "grep -q '^user-session=xfce' /etc/lightdm/lightdm.conf.d/50-vibian.conf"
check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
