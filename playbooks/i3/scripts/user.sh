#!/bin/bash
# i3 playbook, user part: config files for i3, i3status, rofi, dunst and
# picom. Never run as root. Any file it replaces is kept as <file>.vibian-bak
# (only the first time). Knobs: see PLAYBOOK.md.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
hex() { [[ $1 =~ ^[0-9a-fA-F]{6}$ ]] || { echo "$2 must be 6 hex digits, got '$1'" >&2; exit 1; }; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }

term=${VIBIAN_TERMINAL:-alacritty}
mod=${VIBIAN_MOD:-SUPER}
floating=${VIBIAN_FLOATING:-0}
autotiling=${VIBIAN_AUTOTILING:-1}
gaps=${VIBIAN_GAPS:-6}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
hex "$accent" VIBIAN_ACCENT; hex "$bg" VIBIAN_BG
num "$gaps" VIBIAN_GAPS; num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $term in alacritty) run="alacritty -e" ;; kitty) run="kitty" ;; xterm) run="xterm -e" ;; *) echo "VIBIAN_TERMINAL: alacritty, kitty or xterm" >&2; exit 1 ;; esac
case $mod in SUPER) m=Mod4 ;; ALT) m=Mod1 ;; *) echo "VIBIAN_MOD: SUPER or ALT" >&2; exit 1 ;; esac
case $floating$autotiling in [01][01]) ;; *) echo "VIBIAN_FLOATING and VIBIAN_AUTOTILING are 0 or 1" >&2; exit 1 ;; esac
[[ $kb =~ ^[a-z,]+$ && $kbv =~ ^[a-z0-9_,-]*$ ]] || { echo "bad keyboard layout" >&2; exit 1; }

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
written=()
put() {  # put FILE: write stdin to FILE, keeping a first-time backup
  local f=$1
  mkdir -p "$(dirname "$f")"
  if [ -e "$f" ] && [ ! -e "$f.vibian-bak" ] && ! grep -q 'vibian playbook' "$f" 2>/dev/null; then
    mv "$f" "$f.vibian-bak"
  fi
  cat > "$f"
  written+=("$f")
}

wallpaper=/usr/share/vibian/vibian-logo.png
if [ -f "$wallpaper" ]; then wp="feh --no-fehbg --bg-center --image-bg '#$bg' $wallpaper"; else wp="xsetroot -solid '#$bg'"; fi
extra=""
[ "$floating" = 1 ] && extra+=$'for_window [class=".*"] floating enable\n'
[ "$autotiling" = 1 ] && extra+=$'exec --no-startup-id autotiling\n'
kbopt=""; [ -n "$kbv" ] && kbopt=" -variant $kbv"
# xss-lock locks on the screensaver timeout and before suspend; xautolock
# suspends after a longer idle time
idle="exec --no-startup-id xset s $((lock * 60)) 30 dpms 0 0 $((lock * 60 + 60))"
[ "$lock" = 0 ] && idle="exec --no-startup-id xset s off -dpms"
idle+=$'\n'"exec --no-startup-id xss-lock --transfer-sleep-lock -- i3lock -n -c $bg"
[ "$suspend" != 0 ] && idle+=$'\n'"exec --no-startup-id xautolock -time $suspend -locker 'systemctl suspend' -detectsleep"

put "$cfg/i3/config" <<EOF2
# i3, from the vibian playbook "i3". Change anything; the agent reads this
# file too. Your own settings go in ~/.config/i3/config.d/.
# Docs: https://i3wm.org/docs/userguide.html  Check it: i3 -C
set \$mod $m
set \$term $run
set \$bg #$bg
set \$accent #$accent

font pango:Noto Sans 10
default_border pixel 2
default_floating_border pixel 2
gaps inner $((gaps / 2))
gaps outer $((gaps - gaps / 2))
# class                 border   bg       text     indicator child_border
client.focused          \$accent \$accent \$bg     \$accent  \$accent
client.unfocused        #2a2b3d  \$bg     #a9b1d6  #2a2b3d   #2a2b3d
client.focused_inactive #2a2b3d  \$bg     #a9b1d6  #2a2b3d   #2a2b3d
floating_modifier \$mod
focus_follows_mouse yes

exec --no-startup-id setxkbmap -layout $kb$kbopt
exec --no-startup-id $wp
exec --no-startup-id picom -b
exec --no-startup-id dunst
exec --no-startup-id lxpolkit
exec --no-startup-id nm-applet
exec --no-startup-id dex --autostart --environment i3
$idle
$extra
bar {
    status_command i3status
    position top
    tray_output primary
    font pango:Noto Sans, FontAwesome 10
    colors {
        background \$bg
        statusline #c0caf5
        separator #2a2b3d
        #                  border   bg       text
        focused_workspace  \$accent \$accent \$bg
        active_workspace   #2a2b3d  #2a2b3d  #c0caf5
        inactive_workspace \$bg     \$bg     #c0caf5
        urgent_workspace   #f7768e  #f7768e  \$bg
    }
}

bindsym \$mod+Return exec $term
bindsym \$mod+a exec \$term vibian agent
bindsym \$mod+space exec rofi -show drun
bindsym \$mod+Tab exec rofi -show window
bindsym \$mod+q kill
bindsym \$mod+f fullscreen toggle
bindsym \$mod+t floating toggle
bindsym \$mod+w layout tabbed
bindsym \$mod+e layout toggle split
bindsym \$mod+b split h
bindsym \$mod+n split v
bindsym \$mod+l exec --no-startup-id loginctl lock-session
bindsym \$mod+Shift+s exec --no-startup-id maim -s -u | xclip -selection clipboard -t image/png
bindsym Print exec --no-startup-id sh -c 'mkdir -p ~/Pictures/Screenshots && maim -u ~/Pictures/Screenshots/\$(date +%F-%H%M%S).png'
bindsym \$mod+Shift+c reload
bindsym \$mod+Shift+r restart
bindsym \$mod+Shift+e exec i3-msg exit
bindsym \$mod+Left focus left
bindsym \$mod+Right focus right
bindsym \$mod+Up focus up
bindsym \$mod+Down focus down
bindsym \$mod+Shift+Left move left
bindsym \$mod+Shift+Right move right
bindsym \$mod+Shift+Up move up
bindsym \$mod+Shift+Down move down
mode "resize" {
    bindsym Left resize shrink width 20 px or 2 ppt
    bindsym Right resize grow width 20 px or 2 ppt
    bindsym Up resize shrink height 20 px or 2 ppt
    bindsym Down resize grow height 20 px or 2 ppt
    bindsym Escape mode "default"
    bindsym Return mode "default"
}
bindsym \$mod+r mode "resize"

bindsym XF86AudioRaiseVolume exec --no-startup-id wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%+
bindsym XF86AudioLowerVolume exec --no-startup-id wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-
bindsym XF86AudioMute exec --no-startup-id wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
bindsym XF86MonBrightnessUp exec --no-startup-id brightnessctl set 5%+
bindsym XF86MonBrightnessDown exec --no-startup-id brightnessctl set 5%-
bindsym XF86AudioPlay exec --no-startup-id playerctl play-pause
bindsym XF86AudioNext exec --no-startup-id playerctl next
bindsym XF86AudioPrev exec --no-startup-id playerctl previous
EOF2
for i in 1 2 3 4 5 6 7 8 9; do
  printf 'bindsym $mod+%s workspace number %s\nbindsym $mod+Shift+%s move container to workspace number %s\n' $i $i $i $i
done >> "$cfg/i3/config"
printf '\n# your own settings: kept across playbook re-runs\ninclude ~/.config/i3/config.d/*\n' >> "$cfg/i3/config"
mkdir -p "$cfg/i3/config.d"

put "$cfg/i3status/config" <<EOF2
# from the vibian playbook "i3"
general {
    colors = true
    color_good = "#9ece6a"
    color_degraded = "#e0af68"
    color_bad = "#f7768e"
    interval = 5
}
order += "volume master"
order += "wireless _first_"
order += "ethernet _first_"
order += "battery all"
order += "tztime local"
volume master { format = " %volume" format_muted = " muted" device = "pulse" }
wireless _first_ { format_up = " %essid" format_down = "" }
ethernet _first_ { format_up = " wired" format_down = "" }
battery all { format = "%status %percentage" format_down = "" status_chr = "" status_bat = "" status_full = "" low_threshold = 15 }
tztime local { format = "%a %d %b  %H:%M" }
EOF2

put "$cfg/rofi/config.rasi" <<EOF2
/* from the vibian playbook "i3" */
configuration { modi: "drun,run,window"; show-icons: true; terminal: "$term"; font: "Noto Sans 12"; }
* { bg: #${bg}ee; fg: #c0caf5; accent: #$accent; background-color: transparent; text-color: @fg; }
window { background-color: @bg; border: 2px; border-color: @accent; border-radius: 8px; width: 40%; padding: 12px; }
inputbar { padding: 6px; children: [prompt, entry]; spacing: 8px; }
prompt { text-color: @accent; }
listview { lines: 10; scrollbar: false; }
element { padding: 4px 6px; border-radius: 6px; }
element selected { background-color: @accent; text-color: #$bg; }
element-text, element-icon { background-color: inherit; text-color: inherit; }
EOF2

put "$cfg/dunst/dunstrc" <<EOF2
# from the vibian playbook "i3"
[global]
font = Noto Sans 11
background = "#${bg}ee"
foreground = "#c0caf5"
frame_color = "#$accent"
frame_width = 2
corner_radius = 8
offset = (12, 40)
[urgency_low]
timeout = 4
[urgency_normal]
timeout = 6
[urgency_critical]
timeout = 0
frame_color = "#f7768e"
EOF2

put "$cfg/picom/picom.conf" <<EOF2
# from the vibian playbook "i3": no tearing, soft corners, quick fades
backend = "xrender";
vsync = true;
corner-radius = 8;
rounded-corners-exclude = [ "class_g = 'i3bar'", "class_g = 'Rofi'" ];
fading = true;
fade-delta = 4;
shadow = false;
EOF2

if [ "${VIBIAN_LOGIN:-lightdm}" = tty ]; then
  # start i3 when logging in on tty1
  printf '# from the vibian playbook "i3"\nexec i3\n' > "$HOME/.xinitrc"
  written+=("$HOME/.xinitrc")
  line='[ "$(tty)" = /dev/tty1 ] && [ -z "${DISPLAY:-}" ] && exec startx # vibian playbook i3'
  grep -qF 'vibian playbook i3' "$HOME/.profile" 2>/dev/null || echo "$line" >> "$HOME/.profile"
  written+=("$HOME/.profile")
fi

printf 'wrote %s\n' "${written[@]}"
