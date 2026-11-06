#!/bin/bash
# desktop-playbook.sh: runs inside the test VM as the vibian user (password
# vibian) and walks the hyprland playbook the way the desktop-playbooks
# skill tells an agent to: system.sh as root, user.sh, verify.sh. Then it
# starts the login screen and checks tuigreet is up on VT 7. Puts the
# machine back on the text console at the end. Prints key=value lines for
# headless-install.sh to check.
set -u
pw=vibian
owner() { echo "$pw" | sudo -S -p '' "$@"; }
pb=/usr/share/vibian/playbooks/hyprland/scripts

echo "listed=$(vibian playbooks | grep -c '^hyprland .*tested')"
vibian playbooks show hyprland | grep -q '## Interview'; echo "show=$?"
owner env VIBIAN_TERMINAL=foot bash $pb/system.sh > /tmp/pb-system.log 2>&1; echo "system=$?"
tail -3 /tmp/pb-system.log
VIBIAN_ACCENT=zzz bash $pb/user.sh >/dev/null 2>&1; echo "bad-knob=$?"
baks() { find ~/.config -name '*.vibian-bak' | wc -l; }
bash $pb/user.sh >/dev/null; echo "user=$?"
# earlier sections may leave a hyprland.conf (the rice test): that one is
# backed up once, and a re-run must not back up the playbook's own files
before=$(baks); bash $pb/user.sh >/dev/null; echo "rerun=$? new-baks=$(($(baks) - before))"
bash $pb/verify.sh > /tmp/pb-verify.log; echo "verify=$?"
grep -v '^ok ' /tmp/pb-verify.log
owner bash $pb/user.sh >/dev/null 2>&1; echo "user-as-root=$?"

owner systemctl start greetd
for _ in $(seq 20); do pgrep -x tuigreet >/dev/null && break; sleep 1; done
sleep 3
echo "greeter=$(pgrep -x tuigreet >/dev/null && echo up || echo down) vt=$(cat /sys/class/tty/tty0/active)"
owner systemctl stop greetd
owner systemctl disable -q greetd
owner systemctl set-default -q multi-user.target
owner chvt 1
