#!/bin/bash
# GNOME playbook, user part: theme, accent, wallpaper, keyboard, lock and
# suspend timeouts, and the SUPER+Return / SUPER+A shortcuts, all through
# dconf. Never run as root. The first run saves `dconf dump /` to
# ~/.local/share/vibian/gnome-dconf.vibian-bak.ini.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }
command -v dconf >/dev/null || { echo "dconf is missing: run system.sh first" >&2; exit 1; }

theme=${VIBIAN_THEME:-dark}
accent=${VIBIAN_ACCENT:-blue}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $theme in dark) scheme=prefer-dark ;; light) scheme=default ;; *) echo "VIBIAN_THEME: dark or light" >&2; exit 1 ;; esac
case $accent in blue|teal|green|yellow|orange|red|pink|purple|slate|brown) ;; *) echo "VIBIAN_ACCENT: one of GNOME's accents: blue teal green yellow orange red pink purple slate brown" >&2; exit 1 ;; esac
[[ $bg =~ ^[0-9a-fA-F]{6}$ ]] || { echo "VIBIAN_BG must be 6 hex digits, got '$bg'" >&2; exit 1; }
[[ $kb =~ ^[a-z]+$ && $kbv =~ ^[a-z0-9_-]*$ ]] || { echo "bad keyboard layout (one layout; add more in Settings)" >&2; exit 1; }

data=${XDG_DATA_HOME:-$HOME/.local/share}
mkdir -p "$data/vibian"
# dconf writes go through the dconf service on the session bus; over SSH or
# a TTY there is none, so start a private one
if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then exec dbus-run-session -- bash "$0" "$@"; fi

bak=$data/vibian/gnome-dconf.vibian-bak.ini
[ -e "$bak" ] || { echo "# dconf dump / before the vibian playbook \"gnome\" (nothing below: nothing was set)"; dconf dump /; } > "$bak"

sleep_type=nothing; [ "$suspend" = 0 ] || sleep_type=suspend
kbsrc="('xkb', '$kb${kbv:++$kbv}')"
wall=/usr/share/vibian/vibian-logo.png
keys=/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings
ini=$data/vibian/gnome.dconf.ini
cat > "$ini" <<EOF2
# from the vibian playbook "gnome": loaded with dconf load /
[org/gnome/desktop/interface]
color-scheme='$scheme'
accent-color='$accent'

[org/gnome/desktop/background]
picture-uri='file://$wall'
picture-uri-dark='file://$wall'
picture-options='centered'
primary-color='#$bg'
color-shading-type='solid'

[org/gnome/desktop/screensaver]
picture-uri='file://$wall'
picture-options='centered'
primary-color='#$bg'
lock-enabled=$([ "$lock" = 0 ] && echo false || echo true)
lock-delay=uint32 0

[org/gnome/desktop/session]
idle-delay=uint32 $((lock * 60))

[org/gnome/settings-daemon/plugins/power]
sleep-inactive-ac-type='$sleep_type'
sleep-inactive-ac-timeout=$((suspend * 60))
sleep-inactive-battery-type='$sleep_type'
sleep-inactive-battery-timeout=$((suspend * 60))

[org/gnome/desktop/input-sources]
sources=[$kbsrc]

[org/gnome/mutter]
edge-tiling=true
dynamic-workspaces=true

[org/gnome/shell/keybindings]
toggle-application-view=@as []

[org/gnome/settings-daemon/plugins/media-keys]
custom-keybindings=['$keys/vibian-terminal/', '$keys/vibian-agent/']

[org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/vibian-terminal]
name='Terminal'
command='ptyxis --new-window'
binding='<Super>Return'

[org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/vibian-agent]
name='Vibian agent'
command='ptyxis --new-window -- vibian agent'
binding='<Super>a'
EOF2
dconf load / < "$ini"
echo "wrote dconf from $ini (backup of the old settings: $bak)"
