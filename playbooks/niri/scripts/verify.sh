#!/bin/bash
# niri playbook check: the build, packages, configs and the login setup.
# Prints one line per check and VERIFY OK or VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
check "niri 26.04 built" "/usr/local/bin/niri --version | grep -q ' 26.04'"
check "niri session files" "test -x /usr/local/bin/niri-session -a -f /usr/local/lib/systemd/user/niri.service -a -f /usr/local/share/wayland-sessions/niri.desktop"
check "niri.service runs /usr/local/bin/niri" "grep -q '^ExecStart=/usr/local/bin/niri --session' /usr/local/lib/systemd/user/niri.service"
check "niri portals config" "test -f /etc/xdg/xdg-desktop-portal/niri-portals.conf"
check "niri links to installed libraries" "! ldd /usr/local/bin/niri | grep -q 'not found'"
if dpkg-query -W -f '${Status}' xwayland 2>/dev/null | grep -q 'install ok installed'; then
  check "xwayland-satellite (X11 apps)" "test -x /usr/local/bin/xwayland-satellite && ! ldd /usr/local/bin/xwayland-satellite | grep -q 'not found'"
fi
for p in swayidle swaylock waybar fuzzel mako-notifier swaybg xdg-desktop-portal-gnome xdg-desktop-portal-gtk; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
out=$(/usr/local/bin/niri validate -c "$cfg/niri/config.kdl" 2>&1)
if [ $? = 0 ]; then echo "ok   niri validate"; else echo "FAIL niri validate:"; echo "$out" | tail -15 | sed 's/^/     /'; bad=1; fi
check "waybar config is JSON" "grep -v '^ *//' $cfg/waybar/config.jsonc | python3 -c 'import json,sys; json.load(sys.stdin)'"
check "fuzzel.ini" "test -s $cfg/fuzzel/fuzzel.ini"
check "mako config" "test -s $cfg/mako/config"
check "no tiling-desktop bar in every session" "! systemctl --global is-enabled -q waybar.service"
if grep -qF 'vibian playbook niri' "$HOME/.profile" 2>/dev/null; then
  check "tty1 starts niri" "true"
else
  check "greetd enabled" "systemctl is-enabled -q greetd"
  check "greetd runs tuigreet with niri" "grep -q 'tuigreet.*--cmd niri-session' /etc/greetd/config.toml"
  check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
fi
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
