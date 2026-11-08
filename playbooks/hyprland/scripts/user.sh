#!/bin/bash
# Hyprland playbook, user part: config files for Hyprland, waybar, fuzzel,
# mako, hypridle and hyprlock. Never run as root. Any file it replaces is
# kept as <file>.vibian-bak (only the first time). Knobs: see PLAYBOOK.md.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
hex() { [[ $1 =~ ^[0-9a-fA-F]{6}$ ]] || { echo "$2 must be 6 hex digits, got '$1'" >&2; exit 1; }; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }

term=${VIBIAN_TERMINAL:-foot}
mod=${VIBIAN_MOD:-SUPER}
floating=${VIBIAN_FLOATING:-0}
layout=${VIBIAN_LAYOUT:-dwindle}
gaps=${VIBIAN_GAPS:-6}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
scale=${VIBIAN_SCALE:-auto}
wall=${VIBIAN_WALLPAPER:-}
# set by the quickshell-desk playbook: its desk replaces waybar and the
# SUPER+Space launcher
desk=${VIBIAN_DESK:-waybar}
case $desk in waybar) bar=waybar; menu=fuzzel ;; quickshell) bar="qs -c vibian-desk"; menu="qs -c vibian-desk ipc call desk start" ;; *) echo "VIBIAN_DESK: waybar or quickshell" >&2; exit 1 ;; esac
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
hex "$accent" VIBIAN_ACCENT; hex "$bg" VIBIAN_BG
num "$gaps" VIBIAN_GAPS; num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $term in foot) run="foot" ;; kitty) run="kitty" ;; alacritty) run="alacritty -e" ;; *) echo "VIBIAN_TERMINAL: foot, kitty or alacritty" >&2; exit 1 ;; esac
case $layout in dwindle|master) ;; *) echo "VIBIAN_LAYOUT: dwindle or master" >&2; exit 1 ;; esac
case $mod in SUPER|ALT|CTRL|SUPER_SHIFT|SUPER_ALT) ;; *) echo "VIBIAN_MOD: SUPER, ALT, CTRL, SUPER_SHIFT or SUPER_ALT" >&2; exit 1 ;; esac
[[ $scale = auto || $scale =~ ^[0-9]+(\.[0-9]+)?$ ]] || { echo "VIBIAN_SCALE: auto or a number" >&2; exit 1; }
[[ $kb =~ ^[a-z,]+$ && $kbv =~ ^[a-z0-9_,-]*$ ]] || { echo "bad keyboard layout" >&2; exit 1; }
[ -z "$wall" ] || [[ $wall = /* && $wall != *[[:space:]\#\$]* && -f $wall ]] ||
  { echo "VIBIAN_WALLPAPER: the absolute path of an image file, with no spaces, # or \$ (got '$wall')" >&2; exit 1; }

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

nvidia=""
if lspci 2>/dev/null | grep -qi 'vga.*nvidia\|3d.*nvidia' && [ -e /sys/module/nvidia_drm ]; then
  nvidia='env = LIBVA_DRIVER_NAME,nvidia
env = __GLX_VENDOR_LIBRARY_NAME,nvidia
env = NVD_BACKEND,direct'
fi
logo=/usr/share/vibian/vibian-logo.png
swaybg="swaybg -c $bg"  # no '#': it starts a comment in hyprland.conf
if [ -n "$wall" ]; then swaybg="$swaybg -i $wall -m fill"
elif [ -f "$logo" ]; then swaybg="$swaybg -i $logo -m center"
fi
lockbg="color = rgb($bg)"
[ -n "$wall" ] && lockbg="color = rgb($bg)
    path = $wall
    blur_passes = 2"
float_rule=""
[ "$floating" = 1 ] && float_rule='windowrule = float on, match:class .*'

put "$cfg/hypr/hyprland.conf" <<EOF2
# Hyprland, from the vibian playbook "hyprland". Change anything; the
# agent reads this file too. Docs: https://wiki.hypr.land/Configuring/
monitor = , preferred, auto, $scale

\$mod = $mod
\$terminal = $run
\$menu = $menu

$nvidia
env = XCURSOR_SIZE,24
env = QT_QPA_PLATFORM,wayland;xcb

exec-once = $swaybg
exec-once = $bar
exec-once = mako
exec-once = hypridle
exec-once = systemctl --user start hyprpolkitagent
exec-once = nm-applet --indicator
exec-once = wl-paste --watch cliphist store

general {
    gaps_in = $((gaps / 2))
    gaps_out = $gaps
    border_size = 2
    col.active_border = rgb($accent)
    col.inactive_border = rgba(ffffff22)
    layout = $layout
    resize_on_border = true
}

decoration {
    rounding = 8
    blur {
        enabled = true
        size = 4
        passes = 2
    }
}

animations {
    enabled = true
}

input {
    kb_layout = $kb
    kb_variant = $kbv
    follow_mouse = 1
    touchpad {
        natural_scroll = true
        tap-to-click = true
    }
}

dwindle {
    preserve_split = true
}

misc {
    disable_hyprland_logo = true
    disable_hyprland_guiutils_check = true  # not in the 26.04 archive
    force_default_wallpaper = 0
    background_color = rgb($bg)
}

$float_rule

bind = \$mod, Return, exec, \$terminal
bind = \$mod, A, exec, \$terminal vibian agent
bind = \$mod, Space, exec, \$menu
bind = \$mod, Q, killactive
bind = \$mod, F, fullscreen
bind = \$mod, T, togglefloating
bind = \$mod, L, exec, loginctl lock-session
bind = \$mod SHIFT, E, exit
bind = \$mod SHIFT, S, exec, grim -g "\$(slurp)" - | wl-copy
bind = \$mod, V, exec, cliphist list | fuzzel --dmenu | cliphist decode | wl-copy
bind = \$mod, left, movefocus, l
bind = \$mod, right, movefocus, r
bind = \$mod, up, movefocus, u
bind = \$mod, down, movefocus, d
bind = \$mod SHIFT, left, movewindow, l
bind = \$mod SHIFT, right, movewindow, r
bind = \$mod SHIFT, up, movewindow, u
bind = \$mod SHIFT, down, movewindow, d
EOF2
for i in 1 2 3 4 5 6 7 8 9; do
  printf 'bind = $mod, %s, workspace, %s\nbind = $mod SHIFT, %s, movetoworkspace, %s\n' $i $i $i $i
done >> "$cfg/hypr/hyprland.conf"
cat >> "$cfg/hypr/hyprland.conf" <<'EOF2'
bind = $mod, mouse_down, workspace, e+1
bind = $mod, mouse_up, workspace, e-1
bindm = $mod, mouse:272, movewindow
bindm = $mod, mouse:273, resizewindow

bindel = , XF86AudioRaiseVolume, exec, wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%+
bindel = , XF86AudioLowerVolume, exec, wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-
bindl = , XF86AudioMute, exec, wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
bindel = , XF86MonBrightnessUp, exec, brightnessctl set 5%+
bindel = , XF86MonBrightnessDown, exec, brightnessctl set 5%-
bindl = , XF86AudioPlay, exec, playerctl play-pause
bindl = , XF86AudioNext, exec, playerctl next
bindl = , XF86AudioPrev, exec, playerctl previous

# your own settings: kept across playbook re-runs
source = ~/.config/hypr/local.conf
EOF2
[ -e "$cfg/hypr/local.conf" ] || printf '# Your Hyprland settings. The playbook never overwrites this file.\n' > "$cfg/hypr/local.conf"

listeners=""
[ "$lock" != 0 ] && listeners+="
listener {
    timeout = $((lock * 60))
    on-timeout = loginctl lock-session
}
listener {
    timeout = $((lock * 60 + 30))
    on-timeout = hyprctl dispatch dpms off
    on-resume = hyprctl dispatch dpms on
}"
[ "$suspend" != 0 ] && listeners+="
listener {
    timeout = $((suspend * 60))
    on-timeout = systemctl suspend
}"
put "$cfg/hypr/hypridle.conf" <<EOF2
# from the vibian playbook "hyprland"
general {
    lock_cmd = pidof hyprlock || hyprlock
    before_sleep_cmd = loginctl lock-session
    after_sleep_cmd = hyprctl dispatch dpms on
}
$listeners
EOF2

put "$cfg/hypr/hyprlock.conf" <<EOF2
# from the vibian playbook "hyprland"
general {
    hide_cursor = true
}
background {
    $lockbg
}
label {
    text = \$TIME
    font_size = 72
    color = rgb(c0caf5)
    position = 0, 120
    halign = center
    valign = center
}
input-field {
    size = 320, 50
    outline_thickness = 2
    outer_color = rgb($accent)
    inner_color = rgb($bg)
    font_color = rgb(c0caf5)
    placeholder_text = password
    position = 0, -40
    halign = center
    valign = center
}
EOF2

put "$cfg/waybar/config.jsonc" <<EOF2
// from the vibian playbook "hyprland"
{
  "layer": "bottom",
  "height": 30,
  "modules-left": ["hyprland/workspaces", "hyprland/window"],
  "modules-center": ["clock"],
  "modules-right": ["pulseaudio", "network", "battery", "tray"],
  "hyprland/window": { "max-length": 60 },
  "clock": { "format": "{:%a %d %b  %H:%M}", "tooltip-format": "<tt>{calendar}</tt>" },
  "pulseaudio": { "format": "{icon} {volume}%", "format-muted": " muted", "format-icons": { "default": ["", "", ""] }, "on-click": "pavucontrol" },
  "network": { "format-wifi": " {essid}", "format-ethernet": " wired", "format-disconnected": "offline", "on-click": "$run nmtui" },
  "battery": { "format": "{icon} {capacity}%", "format-icons": ["", "", "", "", ""], "states": { "warning": 25, "critical": 10 } },
  "tray": { "spacing": 8 }
}
EOF2

put "$cfg/waybar/style.css" <<EOF2
/* from the vibian playbook "hyprland" */
* { font-family: "Noto Sans", "FontAwesome";  /* Ubuntu ships Font Awesome 4.7 */ font-size: 13px; }
window#waybar { background: alpha(#$bg, 0.85); color: #c0caf5; }
#workspaces button { color: #c0caf5; padding: 0 6px; border-radius: 6px; }
#workspaces button.active { background: #$accent; color: #$bg; }
#clock, #pulseaudio, #network, #battery, #tray, #window { padding: 0 10px; }
#battery.critical { color: #f7768e; }
EOF2

put "$cfg/fuzzel/fuzzel.ini" <<EOF2
# from the vibian playbook "hyprland"
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
# from the vibian playbook "hyprland"
font=Noto Sans 11
background-color=#${bg}ee
text-color=#c0caf5
border-color=#$accent
border-radius=8
default-timeout=6000
EOF2

if [ "${VIBIAN_LOGIN:-greetd}" = tty ]; then
  # start Hyprland when logging in on tty1
  line='[ "$(tty)" = /dev/tty1 ] && [ -z "${WAYLAND_DISPLAY:-}" ] && exec start-hyprland # vibian playbook hyprland'
  grep -qF 'vibian playbook hyprland' "$HOME/.profile" 2>/dev/null || echo "$line" >> "$HOME/.profile"
  written+=("$HOME/.profile")
fi

printf 'wrote %s\n' "${written[@]}"
