#!/bin/bash
# Hyprland playbook check: packages, configs and the login setup. Prints
# one line per check and VERIFY OK or VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
for p in hyprland hyprlock hypridle waybar fuzzel mako-notifier swaybg xdg-desktop-portal-hyprland; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
out=$(Hyprland --verify-config -c "$cfg/hypr/hyprland.conf" 2>&1)
if echo "$out" | grep -qi 'config ok'; then echo "ok   Hyprland --verify-config"; else echo "FAIL Hyprland --verify-config:"; echo "$out" | tail -15 | sed 's/^/     /'; bad=1; fi
check "waybar config is JSON" "grep -v '^ *//' $cfg/waybar/config.jsonc | python3 -c 'import json,sys; json.load(sys.stdin)'"
check "hypridle.conf" "test -s $cfg/hypr/hypridle.conf"
check "hyprlock.conf" "test -s $cfg/hypr/hyprlock.conf"
check "fuzzel.ini" "test -s $cfg/fuzzel/fuzzel.ini"
check "mako config" "test -s $cfg/mako/config"
if grep -qF 'vibian playbook hyprland' "$HOME/.profile" 2>/dev/null; then
  check "tty1 starts Hyprland" "true"
else
  check "greetd enabled" "systemctl is-enabled -q greetd"
  check "greetd runs tuigreet with Hyprland" "grep -q 'tuigreet.*--cmd start-hyprland' /etc/greetd/config.toml"
  check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
fi
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
