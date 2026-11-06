#!/bin/bash
# i3 playbook check: packages, configs and the login setup. Prints one line
# per check and VERIFY OK or VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
for p in xserver-xorg-core i3-wm i3status i3lock xss-lock rofi dunst picom feh maim autotiling; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
# -C only parses the config, so it needs no X server
out=$(i3 -C -c "$cfg/i3/config" 2>&1)
if [ $? = 0 ] && ! echo "$out" | grep -qi 'error'; then echo "ok   i3 -C"; else echo "FAIL i3 -C:"; echo "$out" | tail -15 | sed 's/^/     /'; bad=1; fi
check "i3status config" "test -s $cfg/i3status/config"
check "rofi theme" "test -s $cfg/rofi/config.rasi"
check "dunstrc" "test -s $cfg/dunst/dunstrc"
check "picom.conf" "test -s $cfg/picom/picom.conf"
check "xss-lock runs from the config" "grep -q 'exec --no-startup-id xss-lock' $cfg/i3/config"
check "no Wayland bar in every session" "! systemctl --global is-enabled -q waybar.service"
if grep -qF 'vibian playbook i3' "$HOME/.profile" 2>/dev/null; then
  check "tty1 starts i3" "grep -q '^exec i3' $HOME/.xinitrc"
else
  check "i3 session file" "test -f /usr/share/xsessions/i3.desktop"
  check "lightdm enabled" "systemctl is-enabled -q lightdm"
  check "LightDM is the login screen" "readlink -f /etc/systemd/system/display-manager.service | grep -q lightdm"
  check "lightdm starts i3" "grep -q '^user-session=i3' /etc/lightdm/lightdm.conf.d/50-vibian.conf"
  check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
fi
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
