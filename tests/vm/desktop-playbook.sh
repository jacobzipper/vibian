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
# headless_sway: a Sway with no screen or input (the sway section installed
# it), for checks that need Wayland. Sets sway_pid and wl (its WAYLAND_DISPLAY).
headless_sway() {
  XDG_RUNTIME_DIR=/run/user/$(id -u); export XDG_RUNTIME_DIR
  printf 'output HEADLESS-1 resolution 1280x800\n' > /tmp/hl-sway.conf
  env -u WAYLAND_DISPLAY -u SWAYSOCK WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1 WLR_RENDERER=pixman \
    sway -c /tmp/hl-sway.conf >/tmp/hl-sway.log 2>&1 &
  sway_pid=$!
  for _ in $(seq 30); do ls "$XDG_RUNTIME_DIR"/sway-ipc.*."$sway_pid".sock >/dev/null 2>&1 && break; sleep 0.5; done
  # shellcheck disable=SC2010 # wayland-N names are plain
  wl=$(ls -t "$XDG_RUNTIME_DIR" | grep -E '^wayland-[0-9]+$' | head -1)
}

say "listed=$(vibian playbooks | grep -c "^$id .*tested")"
vibian playbooks show "$id" | grep -q '## Interview'; say "show=$?"
term=foot; [ "$id" = i3 ] && term=alacritty  # foot is Wayland-only
owner env VIBIAN_TERMINAL=$term bash $pb/system.sh > /tmp/pb-system.log 2>&1; say "system=$?"
tail -3 /tmp/pb-system.log
# COSMIC has no accent knob (cosmic-settings derives the theme from it);
# the desk's own knob is checked before the hyprland one it shares
bad=VIBIAN_ACCENT
case $id in cosmic) bad=VIBIAN_THEME ;; quickshell-desk) bad=VIBIAN_WIDGETS ;; esac
env $bad=zzz bash $pb/user.sh >/dev/null 2>&1; say "bad-knob=$?"
bash $pb/user.sh >/dev/null; say "user=$?"
# earlier sections may leave configs behind (the rice test writes a
# hyprland.conf): those are backed up once, and a re-run must not back up
# the playbook's own files
before=$(baks); bash $pb/user.sh >/dev/null; say "rerun=$? new-baks=$(($(baks) - before))"
bash $pb/verify.sh > /tmp/pb-verify.log; say "verify=$?"
grep -v '^ok ' /tmp/pb-verify.log
if [ "$id" = quickshell-desk ]; then
  # the desk's QML loads: Quickshell on a headless Sway (there's no Hyprland
  # here, so the desk's Hyprland IPC only warns)
  headless_sway
  WAYLAND_DISPLAY=$wl QT_QUICK_BACKEND=software timeout 20 qs -c vibian-desk > /tmp/qs-load.log 2>&1
  kill $sway_pid; wait $sway_pid 2>/dev/null
  # the volume widget's PipeWire isn't running without a login session
  grep ERROR /tmp/qs-load.log | grep -v 'service\.pipewire' > /tmp/qs-errors.log
  say "qml=$(grep -c 'Configuration Loaded' /tmp/qs-load.log) qml-errors=$(wc -l < /tmp/qs-errors.log)"
  head -n 5 /tmp/qs-errors.log
fi
owner bash $pb/user.sh >/dev/null 2>&1; say "user-as-root=$?"

if [ "$id" = ask-overlay ]; then
  # the overlay on a headless Sway: a request typed into it goes through
  # vibian ask to a stand-in agent, and its answer is listed. A private
  # session bus, as a desktop has: the overlay is one instance per bus.
  owner apt-get -o DPkg::Lock::Timeout=600 install -y -q wtype >/dev/null 2>&1
  h=/tmp/ask-home; rm -rf $h; mkdir -p $h/bin $h/.config/vibian
  printf '#!/bin/sh\necho "fake answer to: $2"\n' > $h/bin/pi; chmod +x $h/bin/pi
  echo '{"harness": "pi", "powers": "root"}' > $h/.config/vibian/agent.json
  headless_sway
  as() { env HOME=$h XDG_CONFIG_HOME=$h/.config XDG_STATE_HOME=$h/.local/state PATH=$h/bin:$PATH WAYLAND_DISPLAY=$wl "$@"; }
  # (to files: the daemon keeps a $(…) pipe open)
  as dbus-daemon --session --fork --print-address=3 --print-pid=4 3>/tmp/ask-dbus.addr 4>/tmp/ask-dbus.pid </dev/null >/dev/null 2>&1
  DBUS_SESSION_BUS_ADDRESS=$(cat /tmp/ask-dbus.addr); export DBUS_SESSION_BUS_ADDRESS
  # the seat has no keyboard, so one wtype keyboard is there before the
  # overlay maps (a keyboard added later doesn't get the overlay's focus); it
  # types once the overlay is up, software-rendered in the VM
  { sleep 12; printf 'hello overlay\n'; sleep 1; } | as wtype - &
  type_pid=$!
  sleep 1
  as vibian-ask > /tmp/ask-overlay.log 2>&1 &
  ask_pid=$!
  wait $type_pid
  for _ in $(seq 30); do as vibian asks | grep -q ' done ' && break; sleep 0.5; done
  say "overlay-layer=$(grep -c 'layer shell' /tmp/ask-overlay.log) overlay-ask=$(as vibian asks --json | grep -c '"answer": "fake answer to: hello overlay"')"
  # the hotkey again closes it
  open=$(kill -0 $ask_pid 2>/dev/null && echo yes || echo no)
  as vibian-ask >/dev/null 2>&1; sleep 2
  say "toggle-closes=$([ "$open" = yes ] && ! kill -0 $ask_pid 2>/dev/null && echo yes || echo no)"
  kill $ask_pid 2>/dev/null; kill "$(cat /tmp/ask-dbus.pid)" $sway_pid; wait 2>/dev/null
  unset DBUS_SESSION_BUS_ADDRESS
  grep -iE 'error|critical|traceback' /tmp/ask-overlay.log | grep -vE 'MESA|libEGL' | head -n 5
  exit 0
fi

if [ "$id" = headless ]; then
  # keys only: refused while nobody has a key; then a key logs in and a
  # password doesn't. Passwords go back on at once: this test logs in with one.
  say "target=$(systemctl get-default) ufw=$(owner ufw status | head -1)"
  owner env VIBIAN_SSH_PASSWORDS=no bash $pb/system.sh >/dev/null 2>&1; say "keys-only-without-key=$?"
  rm -f /tmp/hk /tmp/hk.pub; ssh-keygen -q -t ed25519 -N '' -C headless-test -f /tmp/hk
  VIBIAN_SSH_KEY="$(cat /tmp/hk.pub)" VIBIAN_TMUX_ON_SSH=1 bash $pb/user.sh >/dev/null; say "key-added=$? keys=$(grep -c headless-test ~/.ssh/authorized_keys)"
  VIBIAN_SSH_KEY="$(cat /tmp/hk.pub)" VIBIAN_TMUX_ON_SSH=1 bash $pb/user.sh >/dev/null; say "key-again=$(grep -c headless-test ~/.ssh/authorized_keys)"
  owner env VIBIAN_SSH_PASSWORDS=no bash $pb/system.sh >/dev/null 2>&1; say "keys-only=$?"
  o=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o BatchMode=yes -o LogLevel=ERROR)
  ssh "${o[@]}" -i /tmp/hk vibian@127.0.0.1 true; say "key-login=$?"
  ssh "${o[@]}" -o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive vibian@127.0.0.1 true 2>/tmp/hk.err
  say "password-login=$? $(grep -o 'Permission denied ([a-z,-]*)' /tmp/hk.err)"
  # an interactive ssh login (it reads ~/.profile) lands in tmux's "main"
  # (with a terminal type, as a real client sends: the test runs without one)
  (sleep 8) | TERM=xterm-256color timeout 10 ssh "${o[@]}" -tt -i /tmp/hk vibian@127.0.0.1 >/dev/null 2>&1 &
  sleep 5; say "tmux-on-ssh=$(tmux ls -F '#S' 2>/dev/null | grep -cx main)"
  wait; tmux kill-server 2>/dev/null
  owner env VIBIAN_SSH_PASSWORDS=yes bash $pb/system.sh >/dev/null 2>&1; say "passwords-back=$?"
  bash $pb/verify.sh | tail -1
  # leave the next sections a machine like the others: no tmux on login, no key
  VIBIAN_TMUX_ON_SSH=0 bash $pb/user.sh >/dev/null
  sed -i '/headless-test/d' ~/.ssh/authorized_keys
  owner ufw --force disable >/dev/null
  exit 0
fi

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
