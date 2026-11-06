#!/bin/bash
# Sway playbook, user part: config files for Sway, waybar, fuzzel and mako.
# Never run as root. Any file it replaces is kept as <file>.vibian-bak
# (only the first time). Knobs: see PLAYBOOK.md.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
hex() { [[ $1 =~ ^[0-9a-fA-F]{6}$ ]] || { echo "$2 must be 6 hex digits, got '$1'" >&2; exit 1; }; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }

term=${VIBIAN_TERMINAL:-foot}
mod=${VIBIAN_MOD:-SUPER}
floating=${VIBIAN_FLOATING:-0}
autotiling=${VIBIAN_AUTOTILING:-1}
gaps=${VIBIAN_GAPS:-6}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
scale=${VIBIAN_SCALE:-1}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
hex "$accent" VIBIAN_ACCENT; hex "$bg" VIBIAN_BG
num "$gaps" VIBIAN_GAPS; num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $term in foot) run="foot" ;; kitty) run="kitty" ;; alacritty) run="alacritty -e" ;; *) echo "VIBIAN_TERMINAL: foot, kitty or alacritty" >&2; exit 1 ;; esac
case $mod in SUPER) m=Mod4 ;; ALT) m=Mod1 ;; *) echo "VIBIAN_MOD: SUPER or ALT" >&2; exit 1 ;; esac
case $floating$autotiling in [01][01]) ;; *) echo "VIBIAN_FLOATING and VIBIAN_AUTOTILING are 0 or 1" >&2; exit 1 ;; esac
[[ $scale =~ ^[0-9]+(\.[0-9]+)?$ ]] || { echo "VIBIAN_SCALE: a number" >&2; exit 1; }
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
if [ -f "$wallpaper" ]; then bgline="output * bg $wallpaper center #$bg"; else bgline="output * bg #$bg solid_color"; fi
extra=""
[ "$floating" = 1 ] && extra+=$'for_window [app_id=".*"] floating enable\nfor_window [class=".*"] floating enable\n'
[ "$autotiling" = 1 ] && extra+=$'exec autotiling\n'
idle="exec swayidle -w"
[ "$lock" != 0 ] && idle+=" timeout $((lock * 60)) 'swaylock -f -c $bg' timeout $((lock * 60 + 30)) 'swaymsg \"output * power off\"' resume 'swaymsg \"output * power on\"'"
[ "$suspend" != 0 ] && idle+=" timeout $((suspend * 60)) 'systemctl suspend'"
idle+=" before-sleep 'swaylock -f -c $bg'"

put "$cfg/sway/config" <<EOF2
# Sway, from the vibian playbook "sway". Change anything; the agent reads
# this file too. Your own settings go in ~/.config/sway/config.d/.
# Docs: man 5 sway, https://github.com/swaywm/sway/wiki
set \$mod $m
set \$term $run
set \$menu fuzzel

$bgline
output * scale $scale
input type:keyboard {
    xkb_layout $kb
    xkb_variant "$kbv"
}
input type:touchpad {
    tap enabled
    natural_scroll enabled
}

default_border pixel 2
default_floating_border pixel 2
gaps inner $((gaps / 2))
gaps outer $((gaps - gaps / 2))
# class                 border   bg       text     indicator child_border
client.focused          #$accent #$accent #$bg     #$accent  #$accent
client.unfocused        #2a2b3d  #$bg     #a9b1d6  #2a2b3d   #2a2b3d
client.focused_inactive #2a2b3d  #$bg     #a9b1d6  #2a2b3d   #2a2b3d
floating_modifier \$mod normal
focus_follows_mouse yes

exec dbus-update-activation-environment --systemd WAYLAND_DISPLAY XDG_CURRENT_DESKTOP=sway
exec mako
exec lxpolkit
exec nm-applet --indicator
exec wl-paste --watch cliphist store
$idle
$extra
bar {
    swaybar_command waybar
}

bindsym \$mod+Return exec \$term
bindsym \$mod+a exec \$term vibian agent
bindsym \$mod+space exec \$menu
bindsym \$mod+q kill
bindsym \$mod+f fullscreen toggle
bindsym \$mod+t floating toggle
bindsym \$mod+w layout tabbed
bindsym \$mod+e layout toggle split
bindsym \$mod+b splith
bindsym \$mod+n splitv
bindsym \$mod+l exec swaylock -f -c $bg
bindsym \$mod+Shift+s exec grim -g "\$(slurp)" - | wl-copy
bindsym \$mod+v exec cliphist list | fuzzel --dmenu | cliphist decode | wl-copy
bindsym \$mod+Shift+c reload
bindsym \$mod+Shift+e exec swaymsg exit
bindsym \$mod+Left focus left
bindsym \$mod+Right focus right
bindsym \$mod+Up focus up
bindsym \$mod+Down focus down
bindsym \$mod+Shift+Left move left
bindsym \$mod+Shift+Right move right
bindsym \$mod+Shift+Up move up
bindsym \$mod+Shift+Down move down
mode "resize" {
    bindsym Left resize shrink width 20px
    bindsym Right resize grow width 20px
    bindsym Up resize shrink height 20px
    bindsym Down resize grow height 20px
    bindsym Escape mode "default"
    bindsym Return mode "default"
}
bindsym \$mod+r mode "resize"

bindsym --locked XF86AudioRaiseVolume exec wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%+
bindsym --locked XF86AudioLowerVolume exec wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-
bindsym --locked XF86AudioMute exec wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
bindsym --locked XF86MonBrightnessUp exec brightnessctl set 5%+
bindsym --locked XF86MonBrightnessDown exec brightnessctl set 5%-
bindsym --locked XF86AudioPlay exec playerctl play-pause
bindsym --locked XF86AudioNext exec playerctl next
bindsym --locked XF86AudioPrev exec playerctl previous
EOF2
for i in 1 2 3 4 5 6 7 8 9; do
  printf 'bindsym $mod+%s workspace number %s\nbindsym $mod+Shift+%s move container to workspace number %s\n' $i $i $i $i
done >> "$cfg/sway/config"
printf '\n# your own settings: kept across playbook re-runs\ninclude ~/.config/sway/config.d/*\n' >> "$cfg/sway/config"
mkdir -p "$cfg/sway/config.d"

put "$cfg/waybar/config.jsonc" <<EOF2
// from the vibian playbook "sway"
{
  "layer": "top",
  "height": 30,
  "modules-left": ["sway/workspaces", "sway/mode", "sway/window"],
  "modules-center": ["clock"],
  "modules-right": ["pulseaudio", "network", "battery", "tray"],
  "sway/window": { "max-length": 60 },
  "clock": { "format": "{:%a %d %b  %H:%M}", "tooltip-format": "<tt>{calendar}</tt>" },
  "pulseaudio": { "format": "{icon} {volume}%", "format-muted": " muted", "format-icons": { "default": ["", "", ""] }, "on-click": "pavucontrol" },
  "network": { "format-wifi": " {essid}", "format-ethernet": " wired", "format-disconnected": "offline", "on-click": "$run nmtui" },
  "battery": { "format": "{icon} {capacity}%", "format-icons": ["", "", "", "", ""], "states": { "warning": 25, "critical": 10 } },
  "tray": { "spacing": 8 }
}
EOF2

put "$cfg/waybar/style.css" <<EOF2
/* from the vibian playbook "sway" */
* { font-family: "Noto Sans", "FontAwesome";  /* Ubuntu ships Font Awesome 4.7 */ font-size: 13px; }
window#waybar { background: alpha(#$bg, 0.85); color: #c0caf5; }
#workspaces button { color: #c0caf5; padding: 0 6px; border-radius: 6px; }
#workspaces button.focused { background: #$accent; color: #$bg; }
#mode { background: #f7768e; color: #$bg; padding: 0 8px; }
#clock, #pulseaudio, #network, #battery, #tray, #window { padding: 0 10px; }
#battery.critical { color: #f7768e; }
EOF2

put "$cfg/fuzzel/fuzzel.ini" <<EOF2
# from the vibian playbook "sway"
[main]
terminal=$run
font=Noto Sans:size=12
width=40
[colors]
background=${bg}ee
text=c0caf5ff
selection=${accent}ff
selection-text=${bg}ff
border=${accent}ff
[border]
radius=8
EOF2

put "$cfg/mako/config" <<EOF2
# from the vibian playbook "sway"
font=Noto Sans 11
background-color=#${bg}ee
text-color=#c0caf5
border-color=#$accent
border-radius=8
default-timeout=6000
EOF2

if [ "${VIBIAN_LOGIN:-greetd}" = tty ]; then
  # start Sway when logging in on tty1
  flag=""; [ -e /sys/module/nvidia_drm ] && flag=" --unsupported-gpu"
  line="[ \"\$(tty)\" = /dev/tty1 ] && [ -z \"\${WAYLAND_DISPLAY:-}\" ] && exec sway$flag # vibian playbook sway"
  grep -qF 'vibian playbook sway' "$HOME/.profile" 2>/dev/null || echo "$line" >> "$HOME/.profile"
  written+=("$HOME/.profile")
fi

printf 'wrote %s\n' "${written[@]}"
