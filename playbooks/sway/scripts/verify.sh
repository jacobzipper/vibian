#!/bin/bash
# Sway playbook check: packages, configs and the login setup. Prints
# one line per check and VERIFY OK or VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
for p in sway swayidle swaylock waybar fuzzel mako-notifier xdg-desktop-portal-wlr autotiling; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
# the headless backend lets --validate run without a seat (over SSH, from a TTY)
out=$(WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1 sway --validate --config "$cfg/sway/config" 2>&1)
if [ $? = 0 ] && ! echo "$out" | grep -q 'sway/config.*Error'; then echo "ok   sway --validate"; else echo "FAIL sway --validate:"; echo "$out" | tail -15 | sed 's/^/     /'; bad=1; fi
check "waybar config is JSON" "grep -v '^ *//' $cfg/waybar/config.jsonc | python3 -c 'import json,sys; json.load(sys.stdin)'"
check "swayidle runs from the config" "grep -q '^exec swayidle' $cfg/sway/config"
check "fuzzel.ini" "test -s $cfg/fuzzel/fuzzel.ini"
check "mako config" "test -s $cfg/mako/config"
if grep -qF 'vibian playbook sway' "$HOME/.profile" 2>/dev/null; then
  check "tty1 starts Sway" "true"
else
  check "greetd enabled" "systemctl is-enabled -q greetd"
  check "greetd runs tuigreet with Sway" "grep -q 'tuigreet.*--cmd .sway' /etc/greetd/config.toml"
  check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
fi
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
