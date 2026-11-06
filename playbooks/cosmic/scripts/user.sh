#!/bin/bash
# COSMIC playbook, user part: dark or light, tiling, wallpaper, keyboard,
# screen-off and suspend timeouts, the dock, and the SUPER+Return / SUPER+A
# shortcuts. COSMIC keeps every setting as one small RON file under
# ~/.config/cosmic/<component>/v1/<key> and picks up changes live. Never run
# as root. The first run saves ~/.config/cosmic to
# ~/.local/share/vibian/cosmic-config.vibian-bak.tar.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }

theme=${VIBIAN_THEME:-dark}
tiling=${VIBIAN_TILING:-off}
wallpaper=${VIBIAN_WALLPAPER:-vibian}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
screen_off=${VIBIAN_IDLE_SCREEN_OFF:-10}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
num "$screen_off" VIBIAN_IDLE_SCREEN_OFF; num "$suspend" VIBIAN_IDLE_SUSPEND
case $theme in dark) dark=true ;; light) dark=false ;; *) echo "VIBIAN_THEME: dark or light" >&2; exit 1 ;; esac
case $tiling in on) autotile=true ;; off) autotile=false ;; *) echo "VIBIAN_TILING: on or off" >&2; exit 1 ;; esac
case $wallpaper in
  vibian) wall=/usr/share/backgrounds/vibian/vibian-wallpaper.png ;;
  cosmic) wall=/usr/share/backgrounds/cosmic/orion_nebula_nasa_heic0601a.jpg ;;
  /*) wall=$wallpaper; [[ $wall != *\"* ]] || { echo "VIBIAN_WALLPAPER: no quotes in the path" >&2; exit 1; } ;;
  *) echo "VIBIAN_WALLPAPER: vibian, cosmic or an image's full path" >&2; exit 1 ;;
esac
[ -f "$wall" ] || { echo "no wallpaper at $wall (run system.sh first?)" >&2; exit 1; }
[[ $kb =~ ^[a-z]+$ && $kbv =~ ^[a-z0-9_-]*$ ]] || { echo "bad keyboard layout (one layout; add more in Settings)" >&2; exit 1; }

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
mkdir -p "$data/vibian" "$data/applications" "$cfg/autostart"
bak=$data/vibian/cosmic-config.vibian-bak.tar
# an empty archive means there was no ~/.config/cosmic yet
[ -e "$bak" ] || { if [ -d "$cfg/cosmic" ]; then tar -C "$cfg" -cf "$bak" cosmic; else tar -cf "$bak" -T /dev/null; fi; }

written=()
put() { # put COMPONENT KEY VALUE
  install -d "$cfg/cosmic/com.system76.$1/v1"
  printf '%s\n' "$3" > "$cfg/cosmic/com.system76.$1/v1/$2"
  written+=("$1/$2")
}
ms() { [ "$1" = 0 ] && echo None || echo "Some($(($1 * 60000)))"; }

put CosmicTheme.Mode is_dark "$dark"
put CosmicTheme.Mode auto_switch false
put CosmicComp autotile "$autotile"
put CosmicComp xkb_config "(rules: \"\", model: \"pc105\", layout: \"$kb\", variant: \"$kbv\", options: None, repeat_delay: 600, repeat_rate: 25)"
put CosmicIdle screen_off_time "$(ms "$screen_off")"
put CosmicIdle suspend_on_ac_time "$(ms "$suspend")"
put CosmicIdle suspend_on_battery_time "$(ms "$suspend")"
put CosmicBackground same-on-all true
put CosmicBackground all "(output: \"all\", source: Path(\"$wall\"), filter_by_theme: false, rotation_frequency: 3600, filter_method: Lanczos, scaling_mode: Fit((0.0863, 0.0863, 0.1176)), sampling_method: Alphanumeric)"

# SUPER+Return: COSMIC Terminal, SUPER+A: the agent (upstream's SUPER+A opens
# the app library, which stays on the dock and the panel's Applications button)
cat > "$data/applications/vibian-agent.desktop" <<'EOF2'
[Desktop Entry]
Type=Application
Name=Vibian agent
Comment=Talk to this computer's agent
Exec=cosmic-term -e vibian agent
Icon=utilities-terminal
Categories=System;
EOF2
# custom shortcuts and the dock's pins are the user's too, so a file that
# already exists gets Vibian's entries added at the top, once. In a RON map
# the last duplicate wins, so the user's own SUPER+Return or SUPER+A stays.
# add COMPONENT KEY OPENER ENTRIES MARKER: write "OPENER ENTRIES CLOSER", or
# add ENTRIES after the existing file's first OPENER
add() {
  local f=$cfg/cosmic/com.system76.$1/v1/$2 close; close=$([ "$3" = "{" ] && echo "}" || echo "]")
  if [ ! -s "$f" ] || [ "$(tr -d ' \n\t' < "$f")" = "$3$close" ]; then
    put "$1" "$2" "$(printf '%s\n%s%s' "$3" "$4" "$close")"
  elif ! grep -q "$5" "$f"; then
    O=$3 E=$4 awk '!done && i = index($0, ENVIRON["O"]) { $0 = substr($0, 1, i) "\n" ENVIRON["E"] substr($0, i + 1); done = 1 } 1' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
    written+=("$1/$2")
  fi
}
add CosmicSettings.Shortcuts custom "{" '    (modifiers: [Super], key: "Return"): Spawn("cosmic-term"),
    (modifiers: [Super], key: "a"): Spawn("cosmic-term -e vibian agent"),
' 'vibian agent'
# upstream pins Firefox and the Store, which may not be installed: pin what
# is, plus the agent
fav=
for id in firefox com.system76.CosmicFiles com.system76.CosmicTerm vibian-agent com.system76.CosmicEdit com.system76.CosmicStore com.system76.CosmicSettings; do
  for dir in "$data/applications" /usr/share/applications /var/lib/flatpak/exports/share/applications; do
    [ -f "$dir/$id.desktop" ] && { fav+="    \"$id\",
"; break; }
  done
done
if [ -s "$cfg/cosmic/com.system76.CosmicAppList/v1/favorites" ]; then fav='    "vibian-agent",
'; fi
add CosmicAppList favorites "[" "$fav" '"vibian-agent"'

# lxpolkit (installed by the sway, niri, i3 and xfce playbooks) autostarts in
# any desktop it doesn't know, and COSMIC brings its own polkit agent: two
# agents means an error dialog at every login. Hide it here, for this user.
if [ -f /etc/xdg/autostart/lxpolkit.desktop ]; then
  sed -E '/^NotShowIn=/{/COSMIC;/!s/$/COSMIC;/}' /etc/xdg/autostart/lxpolkit.desktop > "$cfg/autostart/lxpolkit.desktop"
  grep -q '^NotShowIn=.*COSMIC;' "$cfg/autostart/lxpolkit.desktop" || echo 'NotShowIn=COSMIC;' >> "$cfg/autostart/lxpolkit.desktop"
fi
echo "wrote ${#written[@]} COSMIC settings under $cfg/cosmic (backup of the old ones: $bak)"
