#!/bin/bash
# computer-use.sh: runs inside the test VM (as the vibian user, passwordless
# sudo) and drives `vibian mcp computer` on headless Sway twice (Sway's own
# pointer and wtype, then the uinput tablet and keyboard KDE and GNOME use)
# and on Xvfb. Prints key=value lines for headless-install.sh to check.
set -u
export XDG_RUNTIME_DIR=/run/user/$(id -u)
drive() { python3 /tmp/computer-drive.py "$1"; }
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -q sway foot grim wtype seatd xvfb xdotool wmctrl maim xterm >/tmp/cu-apt.log 2>&1
echo "cu-apt=$?"
echo "uinput-rule=$(test -f /usr/lib/udev/rules.d/70-vibian-uinput.rules && echo present)"
mkdir -p ~/.config/sway
printf 'output HEADLESS-1 resolution 1280x720 position 0 0\ndefault_border none\n' > ~/.config/sway/config
focused() { swaymsg -r -t get_tree | python3 -c 'import json,sys
def w(n):
  if n.get("focused") and n.get("pid"): print(n["name"])
  for c in n.get("nodes",[])+n.get("floating_nodes",[]): w(c)
w(json.load(sys.stdin))'; }
win_id() { swaymsg -r -t get_tree | python3 -c 'import json,sys
def w(n):
  if n.get("pid") and n.get("name")==sys.argv[1]: print(n["id"])
  for c in n.get("nodes",[])+n.get("floating_nodes",[]): w(c)
w(json.load(sys.stdin))' "$1"; }
start_sway() {
  env -u WAYLAND_DISPLAY -u SWAYSOCK WLR_RENDERER=pixman "$@" sway >/tmp/sway.log 2>&1 &
  for _ in $(seq 30); do ls "$XDG_RUNTIME_DIR"/sway-ipc.*.sock >/dev/null 2>&1 && break; sleep 0.5; done
  export SWAYSOCK=$(ls -t "$XDG_RUNTIME_DIR"/sway-ipc.*.sock | head -1)
  swaymsg exec "foot -T left" >/dev/null; sleep 2; swaymsg exec "foot -T right" >/dev/null; sleep 2
}
# both foot windows tile side by side: left is x 0-639, right 640-1279
steps() { cat <<J
[{"tool":"windows"},
 {"tool":"focus_window","args":{"id":"$(win_id left)"}},
 {"sh":"echo focus=\$(bash /tmp/focused.sh)"},
 {"tool":"click","args":{"x":960,"y":360}},
 {"sh":"echo click=\$(bash /tmp/focused.sh)"},
 {"tool":"type_text","args":{"text":"echo '$1 \"ok\" ~?' > /tmp/typed-$1"}},
 {"tool":"key","args":{"combo":"Return"}},
 {"sleep":0.5},
 {"sh":"echo typed=\$(cat /tmp/typed-$1)"},
 {"tool":"scroll","args":{"dy":2}},
 {"tool":"drag","args":{"x1":300,"y1":300,"x2":320,"y2":320}},
 {"sh":"echo drag=\$(bash /tmp/focused.sh)"},
 {"tool":"screenshot","save":"/tmp/$1.png"},
 {"tool":"screenshot","args":{"region":"640,0,640,360","scale":0.5},"save":"/tmp/$1-region.png"},
 {"tool":"key","args":{"combo":"hyper+x"}}]
J
}
declare -f focused > /tmp/focused.sh; echo focused >> /tmp/focused.sh

summary() {  # $1: label, stdin: driver output
  python3 -c '
import re,sys
label=sys.argv[1]; out=sys.stdin.read(); print(out, file=sys.stderr)
def get(pat, d="none"):
  m=re.search(pat, out); return m.group(1) if m else d
wins=get(r"CALL windows OK (.*)", "")
print("%s-windows=%d" % (label, wins.count("\"app\": \"foot\"")))
print("%s-%s" % (label, " ".join(re.findall(r"SH => (\w+=\S*)", out))))
print("%s-shots=%s %s" % (label, get(r"CALL screenshot OK image (\d+x\d+)"), re.findall(r"CALL screenshot OK image (\d+x\d+)", out)[-1:] ))
print("%s-errors=%d badkey=%s" % (label, out.count(" ERR "), "yes" if "unknown modifier \"hyper\"" in out else "no"))' "$1"
}

start_sway env WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1
steps sway > /tmp/steps.json; drive /tmp/steps.json | summary sway
pkill -x sway; sleep 1

# the uinput path: libinput (through seatd) has to see the new devices
sudo chmod 666 /dev/uinput
sudo seatd -g "$(id -gn)" >/tmp/seatd.log 2>&1 &
sleep 1
start_sway env WLR_BACKENDS=headless,libinput LIBSEAT_BACKEND=seatd
steps uinput > /tmp/steps.json
VIBIAN_COMPUTER_POINTER=uinput VIBIAN_COMPUTER_KEYBOARD=uinput drive /tmp/steps.json | summary uinput
pkill -x sway; sudo pkill -x seatd; sleep 1
unset SWAYSOCK

Xvfb :7 -screen 0 1024x768x24 >/tmp/xvfb.log 2>&1 &
sleep 1
DISPLAY=:7 xterm -geometry 80x24+0+0 &
sleep 2
cat > /tmp/steps.json <<'J'
[{"tool":"click","args":{"x":100,"y":100}},
 {"tool":"type_text","args":{"text":"echo x11 > /tmp/typed-x11"}},
 {"tool":"key","args":{"combo":"Return"}},
 {"sleep":0.5},
 {"sh":"echo typed=$(cat /tmp/typed-x11)"},
 {"tool":"screenshot","args":{"scale":1},"save":"/tmp/x11.png"},
 {"tool":"screenshot","args":{"region":"0,0,200,100"},"save":"/tmp/x11-region.png"}]
J
drive /tmp/steps.json | summary x11
pkill -x xterm; pkill -x Xvfb
