#!/bin/bash
# niri playbook, user part: config files for niri, waybar, fuzzel and mako.
# Never run as root. Any file it replaces is kept as <file>.vibian-bak
# (only the first time). Knobs: see PLAYBOOK.md.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
hex() { [[ $1 =~ ^[0-9a-fA-F]{6}$ ]] || { echo "$2 must be 6 hex digits, got '$1'" >&2; exit 1; }; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }

term=${VIBIAN_TERMINAL:-foot}
mod=${VIBIAN_MOD:-SUPER}
gaps=${VIBIAN_GAPS:-8}
width=${VIBIAN_COLUMN_WIDTH:-0.5}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
hex "$accent" VIBIAN_ACCENT; hex "$bg" VIBIAN_BG
num "$gaps" VIBIAN_GAPS; num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $term in foot) run=(foot) ;; kitty) run=(kitty) ;; alacritty) run=(alacritty -e) ;; *) echo "VIBIAN_TERMINAL: foot, kitty or alacritty" >&2; exit 1 ;; esac
case $mod in SUPER) m=Super ;; ALT) m=Alt ;; *) echo "VIBIAN_MOD: SUPER or ALT" >&2; exit 1 ;; esac
case $width in 0.33333|0.5|0.66667|1.0) ;; *) echo "VIBIAN_COLUMN_WIDTH: 0.33333, 0.5, 0.66667 or 1.0" >&2; exit 1 ;; esac
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
kdl() { local a; for a in "$@"; do printf ' "%s"' "$a"; done; }  # kdl ARGS: quoted KDL arguments

wallpaper=/usr/share/vibian/vibian-logo.png
if [ -f "$wallpaper" ]; then swaybg=$(kdl swaybg -i "$wallpaper" -m center -c "#$bg"); else swaybg=$(kdl swaybg -c "#$bg"); fi
idle="swayidle -w"
[ "$lock" != 0 ] && idle+=" timeout $((lock * 60)) 'swaylock -f -c $bg' timeout $((lock * 60 + 30)) 'niri msg action power-off-monitors'"
[ "$suspend" != 0 ] && idle+=" timeout $((suspend * 60)) 'systemctl suspend'"
idle+=" before-sleep 'swaylock -f -c $bg'"

put "$cfg/niri/config.kdl" <<EOF2
// niri, from the vibian playbook "niri". Change anything; niri reloads this
// file as soon as you save it, and the agent reads it too. Your own settings
// go in local.kdl next to it (kept across playbook re-runs).
// Docs: https://niri-wm.github.io/niri/Configuration:-Introduction
// Check it: niri validate

input {
    keyboard {
        xkb {
            layout "$kb"
            variant "$kbv"
        }
        numlock
    }
    touchpad {
        tap
        natural-scroll
    }
    focus-follows-mouse max-scroll-amount="0%"
}

layout {
    gaps $gaps
    center-focused-column "never"
    preset-column-widths {
        proportion 0.33333
        proportion 0.5
        proportion 0.66667
    }
    default-column-width { proportion $width; }
    focus-ring {
        width 2
        active-color "#$accent"
        inactive-color "#2a2b3d"
    }
    border { off; }
    // let the wallpaper show through
    background-color "transparent"
}

// the wallpaper sits in the backdrop, so it also shows behind the overview
layer-rule {
    match namespace="^wallpaper\$"
    place-within-backdrop true
}
window-rule {
    geometry-corner-radius 8
    clip-to-geometry true
}
window-rule {
    match app-id=r#"^(pavucontrol|org\\.pulseaudio\\.pavucontrol|blueman-manager|nm-connection-editor)\$"#
    open-floating true
}

prefer-no-csd
hotkey-overlay { skip-at-startup; }
screenshot-path "~/Pictures/Screenshots/%Y-%m-%d %H-%M-%S.png"

spawn-at-startup$swaybg
spawn-at-startup "waybar"
spawn-at-startup "mako"
spawn-at-startup "lxpolkit"
spawn-at-startup "nm-applet" "--indicator"
spawn-sh-at-startup "wl-paste --watch cliphist store"
spawn-sh-at-startup "$idle"

binds {
    $m+Shift+Slash { show-hotkey-overlay; }
    $m+Return hotkey-overlay-title="Terminal" { spawn$(kdl "${run[0]}"); }
    $m+A hotkey-overlay-title="The agent" { spawn$(kdl "${run[@]}" vibian agent); }
    $m+Space hotkey-overlay-title="Apps: fuzzel" { spawn "fuzzel"; }
    $m+D { spawn "fuzzel"; }
    $m+L hotkey-overlay-title="Lock" { spawn$(kdl swaylock -f -c "$bg"); }
    $m+V hotkey-overlay-title="Clipboard history" { spawn-sh "cliphist list | fuzzel --dmenu | cliphist decode | wl-copy"; }
    $m+O repeat=false { toggle-overview; }
    $m+Q repeat=false { close-window; }

    $m+Left  { focus-column-left; }
    $m+Right { focus-column-right; }
    $m+Up    { focus-window-or-workspace-up; }
    $m+Down  { focus-window-or-workspace-down; }
    $m+Shift+Left  { move-column-left; }
    $m+Shift+Right { move-column-right; }
    $m+Shift+Up    { move-window-up-or-to-workspace-up; }
    $m+Shift+Down  { move-window-down-or-to-workspace-down; }
    $m+Home { focus-column-first; }
    $m+End  { focus-column-last; }
    $m+Ctrl+Left  { focus-monitor-left; }
    $m+Ctrl+Right { focus-monitor-right; }
    $m+Ctrl+Shift+Left  { move-column-to-monitor-left; }
    $m+Ctrl+Shift+Right { move-column-to-monitor-right; }

    $m+Page_Down { focus-workspace-down; }
    $m+Page_Up   { focus-workspace-up; }
    $m+Shift+Page_Down { move-column-to-workspace-down; }
    $m+Shift+Page_Up   { move-column-to-workspace-up; }
    $m+WheelScrollDown cooldown-ms=150 { focus-workspace-down; }
    $m+WheelScrollUp   cooldown-ms=150 { focus-workspace-up; }
    $m+WheelScrollRight { focus-column-right; }
    $m+WheelScrollLeft  { focus-column-left; }
    $m+Shift+WheelScrollDown { focus-column-right; }
    $m+Shift+WheelScrollUp   { focus-column-left; }

    $m+BracketLeft  { consume-or-expel-window-left; }
    $m+BracketRight { consume-or-expel-window-right; }
    $m+Comma  { consume-window-into-column; }
    $m+Period { expel-window-from-column; }
    $m+R { switch-preset-column-width; }
    $m+Shift+R { reset-window-height; }
    $m+F { maximize-column; }
    $m+Shift+F { fullscreen-window; }
    $m+C { center-column; }
    $m+Minus { set-column-width "-10%"; }
    $m+Equal { set-column-width "+10%"; }
    $m+Shift+Minus { set-window-height "-10%"; }
    $m+Shift+Equal { set-window-height "+10%"; }
    $m+T { toggle-window-floating; }
    $m+Shift+T { switch-focus-between-floating-and-tiling; }
    $m+W { toggle-column-tabbed-display; }

    Print { screenshot; }
    $m+Shift+S { screenshot; }
    Ctrl+Print { screenshot-screen; }
    Alt+Print { screenshot-window; }
    $m+Escape allow-inhibiting=false { toggle-keyboard-shortcuts-inhibit; }
    $m+Shift+P { power-off-monitors; }
    $m+Shift+E { quit; }
    Ctrl+Alt+Delete { quit; }

    XF86AudioRaiseVolume allow-when-locked=true { spawn-sh "wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%+"; }
    XF86AudioLowerVolume allow-when-locked=true { spawn-sh "wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-"; }
    XF86AudioMute allow-when-locked=true { spawn-sh "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"; }
    XF86AudioMicMute allow-when-locked=true { spawn-sh "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"; }
    XF86MonBrightnessUp allow-when-locked=true { spawn "brightnessctl" "set" "5%+"; }
    XF86MonBrightnessDown allow-when-locked=true { spawn "brightnessctl" "set" "5%-"; }
    XF86AudioPlay allow-when-locked=true { spawn "playerctl" "play-pause"; }
    XF86AudioNext allow-when-locked=true { spawn "playerctl" "next"; }
    XF86AudioPrev allow-when-locked=true { spawn "playerctl" "previous"; }
EOF2
for i in 1 2 3 4 5 6 7 8 9; do
  printf '    %s+%s { focus-workspace %s; }\n    %s+Shift+%s { move-column-to-workspace %s; }\n' "$m" $i $i "$m" $i $i
done >> "$cfg/niri/config.kdl"
printf '}\n\n// your own settings: they override the ones above\ninclude "local.kdl"\n' >> "$cfg/niri/config.kdl"
[ -e "$cfg/niri/local.kdl" ] || printf '// Your own niri settings. The vibian playbook never touches this file.\n' > "$cfg/niri/local.kdl"

put "$cfg/waybar/config.jsonc" <<EOF2
// from the vibian playbook "niri"
{
  "layer": "top",
  "height": 30,
  "modules-left": ["niri/workspaces", "niri/window"],
  "modules-center": ["clock"],
  "modules-right": ["pulseaudio", "network", "battery", "tray"],
  "niri/window": { "max-length": 60 },
  "clock": { "format": "{:%a %d %b  %H:%M}", "tooltip-format": "<tt>{calendar}</tt>" },
  "pulseaudio": { "format": "{icon} {volume}%", "format-muted": " muted", "format-icons": { "default": ["", "", ""] }, "on-click": "pavucontrol" },
  "network": { "format-wifi": " {essid}", "format-ethernet": " wired", "format-disconnected": "offline", "on-click": "${run[*]} nmtui" },
  "battery": { "format": "{icon} {capacity}%", "format-icons": ["", "", "", "", ""], "states": { "warning": 25, "critical": 10 } },
  "tray": { "spacing": 8 }
}
EOF2

put "$cfg/waybar/style.css" <<EOF2
/* from the vibian playbook "niri" */
* { font-family: "Noto Sans", "FontAwesome";  /* Ubuntu ships Font Awesome 4.7 */ font-size: 13px; }
window#waybar { background: alpha(#$bg, 0.85); color: #c0caf5; }
#workspaces button { color: #c0caf5; padding: 0 6px; border-radius: 6px; }
#workspaces button.active { background: #$accent; color: #$bg; }
#clock, #pulseaudio, #network, #battery, #tray, #window { padding: 0 10px; }
#battery.critical { color: #f7768e; }
EOF2

put "$cfg/fuzzel/fuzzel.ini" <<EOF2
# from the vibian playbook "niri"
[main]
terminal=${run[*]}
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
# from the vibian playbook "niri"
font=Noto Sans 11
background-color=#${bg}ee
text-color=#c0caf5
border-color=#$accent
border-radius=8
default-timeout=6000
EOF2

if [ "${VIBIAN_LOGIN:-greetd}" = tty ]; then
  # start niri when logging in on tty1
  line="[ \"\$(tty)\" = /dev/tty1 ] && [ -z \"\${WAYLAND_DISPLAY:-}\" ] && exec niri-session # vibian playbook niri"
  grep -qF 'vibian playbook niri' "$HOME/.profile" 2>/dev/null || echo "$line" >> "$HOME/.profile"
  written+=("$HOME/.profile")
fi

printf 'wrote %s\n' "${written[@]}"
