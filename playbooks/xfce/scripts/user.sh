#!/bin/bash
# XFCE playbook, user part: theme, panel, wallpaper, keyboard, lock and
# suspend timeouts, terminal colours and the SUPER shortcuts, all through
# xfconf. Never run as root. The first run copies the xfconf files to
# ~/.config/xfce4/xfconf/xfce-perchannel-xml.vibian-bak.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }
command -v xfconf-query >/dev/null || { echo "xfconf-query is missing: run system.sh first" >&2; exit 1; }

theme=${VIBIAN_THEME:-dark}
panel=${VIBIAN_PANEL:-bottom}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
reset_panel=${VIBIAN_PANEL_RESET:-0}
num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $theme in dark) gtk=Greybird-dark; icons=elementary-xfce-dark ;; light) gtk=Greybird; icons=elementary-xfce ;; *) echo "VIBIAN_THEME: dark or light" >&2; exit 1 ;; esac
case $panel in top) pos='p=6;x=0;y=0' ;; bottom) pos='p=8;x=0;y=0' ;; *) echo "VIBIAN_PANEL: top or bottom" >&2; exit 1 ;; esac
case $reset_panel in 0|1) ;; *) echo "VIBIAN_PANEL_RESET: 0 or 1" >&2; exit 1 ;; esac
for v in accent bg; do
  [[ ${!v} =~ ^[0-9a-fA-F]{6}$ ]] || { echo "VIBIAN_${v^^} must be 6 hex digits, got '${!v}'" >&2; exit 1; }
done
[[ $kb =~ ^[a-z,]+$ && $kbv =~ ^[a-z0-9_,-]*$ ]] || { echo "bad keyboard layout" >&2; exit 1; }

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
# xfconf-query talks to xfconfd on the session bus. Use the running session's
# bus (XFCE then sees each change live); over SSH or a TTY with no user bus,
# start a private one, and xfconfd saves the files when it exits.
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
  if [ -S "/run/user/$(id -u)/bus" ]; then export DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus
  else exec dbus-run-session -- bash "$0" "$@"; fi
fi

xml=$cfg/xfce4/xfconf/xfce-perchannel-xml
# the first run only: an empty backup means XFCE's defaults, and a backup made
# on a later run would hold this script's own settings
[ -e "$xml.vibian-bak" ] || { mkdir -p "$xml" && cp -a "$xml" "$xml.vibian-bak"; }

set_() {  # set_ CHANNEL PROPERTY TYPE VALUE: create or change one property
  xfconf-query -c "$1" -p "$2" -n -t "$3" -s "$4"
}

# look
set_ xsettings /Net/ThemeName string "$gtk"
set_ xsettings /Net/IconThemeName string "$icons"
set_ xsettings /Gtk/FontName string "Noto Sans 10"
set_ xsettings /Gtk/MonospaceFontName string "Noto Sans Mono 11"
set_ xfwm4 /general/theme string "$gtk"
set_ xfwm4 /general/title_font string "Noto Sans Bold 10"
set_ xfwm4 /general/use_compositing bool true

# keyboard
set_ keyboard-layout /Default/XkbDisable bool false
set_ keyboard-layout /Default/XkbLayout string "$kb"
set_ keyboard-layout /Default/XkbVariant string "$kbv"

# lock after $lock minutes idle; lock before suspend
if [ "$lock" = 0 ]; then
  set_ xfce4-screensaver /saver/enabled bool false
  set_ xfce4-screensaver /lock/enabled bool false
else
  set_ xfce4-screensaver /saver/enabled bool true
  set_ xfce4-screensaver /saver/mode int 0
  set_ xfce4-screensaver /saver/idle-activation/enabled bool true
  set_ xfce4-screensaver /saver/idle-activation/delay int "$lock"
  set_ xfce4-screensaver /lock/enabled bool true
  set_ xfce4-screensaver /lock/saver-activation/enabled bool true
  set_ xfce4-screensaver /lock/saver-activation/delay int 0
fi
pm=/xfce4-power-manager
set_ xfce4-power-manager $pm/lock-screen-suspend-hibernate bool true
set_ xfce4-power-manager $pm/dpms-enabled bool true
# the power manager counts idle minutes from 15; 14 means never
idle=$([ "$suspend" = 0 ] && echo 14 || { [ "$suspend" -lt 15 ] && echo 15 || echo "$suspend"; })
for src in ac battery; do
  set_ xfce4-power-manager $pm/inactivity-on-$src uint "$idle"
  set_ xfce4-power-manager $pm/inactivity-sleep-mode-on-$src uint 1
done

# shortcuts: SUPER opens the menu, SUPER+Return a terminal, SUPER+A the agent.
# Binding Super_L itself grabs the key, and then xfwm4 never sees SUPER+arrow:
# instead xcape turns a lone SUPER tap into Alt+F1, which opens the menu
k=/commands/custom
xfconf-query -c xfce4-keyboard-shortcuts -p "$k/Super_L" -r 2>/dev/null || true
set_ xfce4-keyboard-shortcuts "$k/<Alt>F1" string xfce4-popup-whiskermenu
set_ xfce4-keyboard-shortcuts "$k/<Super>Return" string xfce4-terminal
set_ xfce4-keyboard-shortcuts "$k/<Super>a" string "xfce4-terminal --title=vibian-agent -x vibian agent"
set_ xfce4-keyboard-shortcuts "$k/<Super>l" string xflock4
set_ xfce4-keyboard-shortcuts "$k/<Super>e" string thunar
set_ xfce4-keyboard-shortcuts "$k/<Super><Shift>s" string "xfce4-screenshooter -r -c"
set_ xfce4-keyboard-shortcuts "$k/override" bool true
# xfwm4 only tiles with SUPER+keypad keys, which laptops lack: move the four
# half-screen tiles to the arrows. xfwm4 keeps one key per action, so the
# keypad ones go (the corner tiles keep theirs)
w=/xfwm4/custom
for dir in left right up down; do
  xfconf-query -c xfce4-keyboard-shortcuts -p "$w/<Super>KP_${dir^}" -r 2>/dev/null || true
  set_ xfce4-keyboard-shortcuts "$w/<Super>${dir^}" string "tile_${dir}_key"
done
set_ xfce4-keyboard-shortcuts "$w/override" bool true

# terminal colours to match the wallpaper; the accent is the cursor and selection
set_ xfce4-terminal /color-use-theme bool false
set_ xfce4-terminal /color-background string "#$bg"
set_ xfce4-terminal /color-foreground string "#c0caf5"
set_ xfce4-terminal /color-cursor string "#$accent"
set_ xfce4-terminal /color-selection-background string "#$accent"
set_ xfce4-terminal /color-selection string "#$bg"
set_ xfce4-terminal /font-name string "Noto Sans Mono 11"
set_ xfce4-terminal /misc-menubar-default bool false

# the panel: built on the first run (or with VIBIAN_PANEL_RESET=1), so later
# runs keep the user's own panel changes. Moving it is a property, so that
# follows VIBIAN_PANEL every time.
p=xfce4-panel
if [ "$reset_panel" = 1 ] || ! xfconf-query -c $p -p /panels >/dev/null 2>&1; then
  xfconf-query -c $p -p / -R -r 2>/dev/null || true
  set_ $p /configver int 2
  xfconf-query -c $p -p /panels -n -a -t int -s 1
  set_ $p /panels/dark-mode bool "$([ "$theme" = dark ] && echo true || echo false)"
  set_ $p /panels/panel-1/length uint 100
  set_ $p /panels/panel-1/size uint 34
  set_ $p /panels/panel-1/icon-size uint 0
  set_ $p /panels/panel-1/position-locked bool true
  # menu, window buttons, (space), tray, sound, battery, notifications, clipboard, clock, log out
  plugins=(whiskermenu tasklist separator systray pulseaudio power-manager-plugin notification-plugin xfce4-clipman-plugin clock actions)
  args=()
  for i in "${!plugins[@]}"; do
    n=$((i + 1))
    set_ $p /plugins/plugin-$n string "${plugins[$i]}"
    args+=(-t int -s $n)
  done
  xfconf-query -c $p -p /panels/panel-1/plugin-ids -n -a "${args[@]}"
  set_ $p /plugins/plugin-1/button-title string "Apps"
  set_ $p /plugins/plugin-1/show-button-title bool false
  set_ $p /plugins/plugin-1/button-icon string /usr/share/vibian/vibian-logo.png
  set_ $p /plugins/plugin-2/flat-buttons bool true
  set_ $p /plugins/plugin-2/show-handle bool false
  set_ $p /plugins/plugin-2/grouping uint 1
  set_ $p /plugins/plugin-3/expand bool true
  set_ $p /plugins/plugin-3/style uint 0
  set_ $p /plugins/plugin-4/square-icons bool true
  set_ $p /plugins/plugin-9/mode uint 2
  set_ $p /plugins/plugin-9/digital-layout uint 3
  set_ $p /plugins/plugin-9/digital-time-format string "%a %d %b  %H:%M"
  # one session button (lock, suspend, log out, shut down) instead of five
  set_ $p /plugins/plugin-10/appearance uint 1
  [ -f /usr/share/vibian/vibian-logo.png ] || xfconf-query -c $p -p /plugins/plugin-1/button-icon -r
fi
set_ $p /panels/panel-1/position string "$pos"

# the wallpaper is per monitor, by connector name, which only the running X
# session knows: a login script sets it once, then removes itself
wall=/usr/share/vibian/vibian-logo.png
mkdir -p "$data/vibian" "$cfg/autostart"
cat > "$data/vibian/xfce-apply.sh" <<EOF2
#!/bin/bash
# from the vibian playbook "xfce": sets the wallpaper on every monitor once
d=/backdrop/screen0
for m in \$(xrandr --listmonitors 2>/dev/null | awk 'NR > 1 {print \$NF}') ; do
  for w in 0 1 2 3; do
    b=\$d/monitor\$m/workspace\$w
    $([ -f "$wall" ] && echo "xfconf-query -c xfce4-desktop -p \$b/last-image -n -t string -s $wall
    xfconf-query -c xfce4-desktop -p \$b/image-style -n -t int -s 1" || echo "xfconf-query -c xfce4-desktop -p \$b/image-style -n -t int -s 0")
    xfconf-query -c xfce4-desktop -p \$b/color-style -n -t int -s 0
    xfconf-query -c xfce4-desktop -p \$b/rgba1 -r 2>/dev/null
    xfconf-query -c xfce4-desktop -p \$b/rgba1 -n -a -t double -s $(awk -v v=$((16#${bg:0:2})) 'BEGIN{printf "%.4f", v/255}') -t double -s $(awk -v v=$((16#${bg:2:2})) 'BEGIN{printf "%.4f", v/255}') -t double -s $(awk -v v=$((16#${bg:4:2})) 'BEGIN{printf "%.4f", v/255}') -t double -s 1
  done
done
xfdesktop --reload 2>/dev/null
rm -f "$cfg/autostart/vibian-xfce-apply.desktop"
touch "$data/vibian/xfce.applied"
EOF2
chmod +x "$data/vibian/xfce-apply.sh"
if [ ! -e "$data/vibian/xfce.applied" ] || [ -n "${VIBIAN_REAPPLY:-}" ]; then
  cat > "$cfg/autostart/vibian-xfce-apply.desktop" <<EOF2
[Desktop Entry]
Type=Application
Name=Vibian XFCE wallpaper (runs once)
Exec=$data/vibian/xfce-apply.sh
OnlyShowIn=XFCE;
X-XFCE-Autostart-Override=true
EOF2
fi
cat > "$cfg/autostart/vibian-xcape.desktop" <<'EOF2'
[Desktop Entry]
Type=Application
Name=SUPER opens the menu (xcape)
Exec=xcape -e "Super_L=Alt_L|F1"
OnlyShowIn=XFCE;
EOF2
# when XFCE is already running, apply now and redraw the panel
if [ -n "${DISPLAY:-}" ] && pgrep -u "$(id -u)" -x xfce4-panel >/dev/null; then
  pgrep -u "$(id -u)" -x xcape >/dev/null || setsid -f xcape -e 'Super_L=Alt_L|F1' >/dev/null 2>&1 </dev/null || true
  # xfwm4 picks up new keys live but not removed ones: restart it (windows stay)
  setsid -f xfwm4 --replace >/dev/null 2>&1 </dev/null || true
  "$data/vibian/xfce-apply.sh" >/dev/null 2>&1 || true
  xfce4-panel -r >/dev/null 2>&1 || true
fi
echo "xfce settings applied (theme=$theme panel=$panel lock=${lock}m suspend=${suspend}m)"
