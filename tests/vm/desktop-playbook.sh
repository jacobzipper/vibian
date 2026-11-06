#!/bin/bash
# desktop-playbook.sh ID: runs inside the test VM as the vibian user
# (password vibian) and walks one desktop playbook the way the
# desktop-playbooks skill tells an agent to: system.sh as root, user.sh,
# verify.sh. Then it starts the login screen and checks tuigreet is up on
# VT 7, and puts the machine back on the text console. Prints ID.key=value
# lines for headless-install.sh to check.
set -u
id=${1:?usage: desktop-playbook.sh ID}
pw=vibian
owner() { echo "$pw" | sudo -S -p '' "$@"; }
say() { echo "$id.$1"; }
pb=/usr/share/vibian/playbooks/$id/scripts
baks() { find ~/.config -name '*.vibian-bak' | wc -l; }

say "listed=$(vibian playbooks | grep -c "^$id .*tested")"
vibian playbooks show "$id" | grep -q '## Interview'; say "show=$?"
owner env VIBIAN_TERMINAL=foot bash $pb/system.sh > /tmp/pb-system.log 2>&1; say "system=$?"
tail -3 /tmp/pb-system.log
VIBIAN_ACCENT=zzz bash $pb/user.sh >/dev/null 2>&1; say "bad-knob=$?"
bash $pb/user.sh >/dev/null; say "user=$?"
# earlier sections may leave configs behind (the rice test writes a
# hyprland.conf): those are backed up once, and a re-run must not back up
# the playbook's own files
before=$(baks); bash $pb/user.sh >/dev/null; say "rerun=$? new-baks=$(($(baks) - before))"
bash $pb/verify.sh > /tmp/pb-verify.log; say "verify=$?"
grep -v '^ok ' /tmp/pb-verify.log
owner bash $pb/user.sh >/dev/null 2>&1; say "user-as-root=$?"

owner systemctl start greetd
for _ in $(seq 20); do pgrep -x tuigreet >/dev/null && break; sleep 1; done
sleep 3
say "greeter=$(pgrep -x tuigreet >/dev/null && echo up || echo down) vt=$(cat /sys/class/tty/tty0/active)"
owner systemctl stop greetd
owner systemctl disable -q greetd
owner systemctl set-default -q multi-user.target
owner chvt 1
