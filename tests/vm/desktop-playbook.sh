#!/bin/bash
# desktop-playbook.sh ID: runs inside the test VM as the vibian user
# (password vibian) and walks one desktop playbook the way the
# desktop-playbooks skill tells an agent to: system.sh as root, user.sh,
# verify.sh. Then it starts the login screen (greetd on VT 7, SDDM for
# kde-plasma, GDM for gnome, LightDM for i3 and xfce, cosmic-greeter for
# cosmic), checks the greeter is up, and
# puts the machine back on the text console. Prints ID.key=value lines for
# headless-install.sh to check.
set -u
id=${1:?usage: desktop-playbook.sh ID}
pw=vibian
owner() { echo "$pw" | sudo -S -p '' "$@"; }
say() { echo "$id.$1"; }
pb=/usr/share/vibian/playbooks/$id/scripts
baks() { find ~/.config -name '*.vibian-bak' | wc -l; }

say "listed=$(vibian playbooks | grep -c "^$id .*tested")"
vibian playbooks show "$id" | grep -q '## Interview'; say "show=$?"
term=foot; [ "$id" = i3 ] && term=alacritty  # foot is Wayland-only
owner env VIBIAN_TERMINAL=$term bash $pb/system.sh > /tmp/pb-system.log 2>&1; say "system=$?"
tail -3 /tmp/pb-system.log
# COSMIC has no accent knob (cosmic-settings derives the theme from it)
bad=VIBIAN_ACCENT; [ "$id" = cosmic ] && bad=VIBIAN_THEME
env $bad=zzz bash $pb/user.sh >/dev/null 2>&1; say "bad-knob=$?"
bash $pb/user.sh >/dev/null; say "user=$?"
# earlier sections may leave configs behind (the rice test writes a
# hyprland.conf): those are backed up once, and a re-run must not back up
# the playbook's own files
before=$(baks); bash $pb/user.sh >/dev/null; say "rerun=$? new-baks=$(($(baks) - before))"
bash $pb/verify.sh > /tmp/pb-verify.log; say "verify=$?"
grep -v '^ok ' /tmp/pb-verify.log
owner bash $pb/user.sh >/dev/null 2>&1; say "user-as-root=$?"

case $id in
  kde-plasma) dm=sddm; greeter='sddm-greeter' ;;
  gnome) dm=gdm3; greeter='gnome-shell --mode=gdm' ;;
  i3|xfce) dm=lightdm; greeter='lightdm-gtk-greeter' ;;
  cosmic) dm=cosmic-greeter; greeter='^cosmic-greeter$' ;;
  *) dm=greetd; greeter='^tuigreet' ;;
esac
owner systemctl start $dm
for _ in $(seq 40); do pgrep -f "$greeter" >/dev/null && break; sleep 1; done
sleep 3
say "greeter=$(pgrep -f "$greeter" >/dev/null && echo up || echo down) vt=$(cat /sys/class/tty/tty0/active)"
owner systemctl stop $dm
owner systemctl disable -q $dm
owner systemctl set-default -q multi-user.target
owner systemctl start getty@tty1
owner chvt 1
