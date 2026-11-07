#!/bin/bash
# Headless playbook check, as the user, against the choices system.sh kept
# in /etc/vibian/headless.env. Prints one line per check and VERIFY OK or
# VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
env=/etc/vibian/headless.env
check "system.sh has run ($env)" "test -r $env"
ssh=1 passwords=yes firewall=1 mdns=1 sleep=never updates=1 reboot=off
# shellcheck disable=SC1090
[ -r $env ] && . $env

check "boots to the console (multi-user.target)" "test \$(systemctl get-default) = multi-user.target"
# (gdm3 is an alias of gdm, and is-enabled says "alias" for it)
check "no login screen: display-manager.service gone" "! test -e /etc/systemd/system/display-manager.service"
for dm in greetd sddm gdm lightdm cosmic-greeter; do
  check "no login screen: $dm not enabled" "[ \"\$(systemctl is-enabled $dm 2>/dev/null)\" != enabled ]"
done
for p in tmux htop; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
if [ "$ssh" = 1 ]; then
  check "ssh enabled" "systemctl is-enabled -q ssh.socket || systemctl is-enabled -q ssh.service"
  check "ssh listening on port 22" "ss -Hltn 'sport = :22' | grep -q ."
  check "sshd drop-in: no root login, passwords $passwords" \
    "grep -qx 'PermitRootLogin no' /etc/ssh/sshd_config.d/01-vibian-headless.conf && grep -qx 'PasswordAuthentication $passwords' /etc/ssh/sshd_config.d/01-vibian-headless.conf"
  [ "$passwords" = no ] && check "a key in ~/.ssh/authorized_keys" "grep -q '^[a-z]' ~/.ssh/authorized_keys"
fi
[ "$firewall" = 1 ] && check "firewall on (ufw)" "grep -qx 'ENABLED=yes' /etc/ufw/ufw.conf && systemctl is-enabled -q ufw"
[ "$mdns" = 1 ] && check "mDNS: $(hostname).local (avahi-daemon)" "systemctl is-active -q avahi-daemon"
if [ "$sleep" = never ]; then
  check "never sleeps (sleep.target masked)" "test \$(systemctl is-enabled sleep.target) = masked"
  check "lid switch ignored" "grep -qx 'HandleLidSwitch=ignore' /etc/systemd/logind.conf.d/50-vibian-headless.conf"
fi
if [ "$updates" = 1 ]; then
  check "automatic security updates" "grep -q 'Unattended-Upgrade \"1\"' /etc/apt/apt.conf.d/20auto-upgrades"
  [ "$reboot" != off ] && check "reboots for updates at $reboot" "grep -q '\"$reboot\"' /etc/apt/apt.conf.d/52vibian-headless"
fi
# parse the config in a throwaway tmux server
tmux_ok() { tmux -L vibian-verify -f /dev/null new-session -d \; source-file "$cfg/tmux/tmux.conf"; local r=$?; tmux -L vibian-verify kill-server 2>/dev/null; return $r; }
check "tmux config" tmux_ok
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
