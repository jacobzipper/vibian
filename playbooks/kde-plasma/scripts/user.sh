#!/bin/bash
# KDE Plasma playbook, user part: keyboard, lock and suspend timeouts,
# shortcuts, and a run-once login script that applies the theme, accent and
# wallpaper (those need a running Plasma). Never run as root. Any KDE file
# it edits is first copied to <file>.vibian-bak (only the first time).
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
hex() { [[ $1 =~ ^[0-9a-fA-F]{6}$ ]] || { echo "$2 must be 6 hex digits, got '$1'" >&2; exit 1; }; }
num() { [[ $1 =~ ^[0-9]+$ ]] || { echo "$2 must be a number, got '$1'" >&2; exit 1; }; }
command -v kwriteconfig6 >/dev/null || { echo "kwriteconfig6 is missing: run system.sh first" >&2; exit 1; }

theme=${VIBIAN_THEME:-dark}
accent=${VIBIAN_ACCENT:-7aa2f7}
bg=${VIBIAN_BG:-16161e}
kb=${VIBIAN_KB_LAYOUT:-us}
kbv=${VIBIAN_KB_VARIANT:-}
lock=${VIBIAN_IDLE_LOCK:-10}
laptop=0; ls /sys/class/power_supply/BAT* >/dev/null 2>&1 && laptop=1
suspend=${VIBIAN_IDLE_SUSPEND:-$([ $laptop = 1 ] && echo 30 || echo 0)}
hex "$accent" VIBIAN_ACCENT; hex "$bg" VIBIAN_BG
num "$lock" VIBIAN_IDLE_LOCK; num "$suspend" VIBIAN_IDLE_SUSPEND
case $theme in dark) laf=org.kde.breezedark.desktop ;; light) laf=org.kde.breeze.desktop ;; *) echo "VIBIAN_THEME: dark or light" >&2; exit 1 ;; esac
[[ $kb =~ ^[a-z,]+$ && $kbv =~ ^[a-z0-9_,-]*$ ]] || { echo "bad keyboard layout" >&2; exit 1; }

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
mkdir -p "$cfg" "$data/applications" "$data/vibian" "$cfg/autostart"
written=()
# KConfig drops comments, so a marker file says "these rc files are already
# ours": only the first run keeps backups
marker=$data/vibian/kde-plasma.applied
bak() { [ ! -e "$marker" ] && [ -e "$cfg/$1" ] && [ ! -e "$cfg/$1.vibian-bak" ] && cp -p "$cfg/$1" "$cfg/$1.vibian-bak"; written+=("$cfg/$1"); return 0; }
kw() { kwriteconfig6 --file "$cfg/$1" "${@:2}"; }

bak kxkbrc
kw kxkbrc --group Layout --key Use true
kw kxkbrc --group Layout --key LayoutList "$kb"
kw kxkbrc --group Layout --key VariantList "$kbv"

bak kscreenlockerrc
if [ "$lock" = 0 ]; then
  kw kscreenlockerrc --group Daemon --key Autolock false
else
  kw kscreenlockerrc --group Daemon --key Autolock true
  kw kscreenlockerrc --group Daemon --key Timeout "$lock"
fi

bak powerdevilrc
for profile in AC Battery LowBattery; do
  if [ "$suspend" = 0 ]; then
    kw powerdevilrc --group "$profile" --group SuspendAndShutdown --key AutoSuspendAction 0
  else
    kw powerdevilrc --group "$profile" --group SuspendAndShutdown --key AutoSuspendAction 1
    kw powerdevilrc --group "$profile" --group SuspendAndShutdown --key AutoSuspendIdleTimeoutSec $((suspend * 60))
  fi
done

# SUPER+Return: Konsole, SUPER+A: the agent
cat > "$data/applications/vibian-agent.desktop" <<EOF2
[Desktop Entry]
# from the vibian playbook "kde-plasma"
Type=Application
Name=Vibian agent
Comment=Talk to this computer's agent
Exec=konsole -e vibian agent
Icon=utilities-terminal
Categories=System;
EOF2
written+=("$data/applications/vibian-agent.desktop")
bak kglobalshortcutsrc
kw kglobalshortcutsrc --group services --group org.kde.konsole.desktop --key _launch "Meta+Return	Ctrl+Alt+T"
kw kglobalshortcutsrc --group services --group vibian-agent.desktop --key _launch "Meta+A"

# Theme, accent and wallpaper need plasmashell running, so a login script
# applies them once, then removes its autostart entry.
wallpaper=/usr/share/vibian/vibian-logo.png
cat > "$data/vibian/kde-apply.sh" <<EOF2
#!/bin/bash
# from the vibian playbook "kde-plasma": run at login once, or by hand
for _ in \$(seq 30); do busctl --user status org.kde.plasmashell >/dev/null 2>&1 && break; sleep 1; done
plasma-apply-lookandfeel -a $laf
plasma-apply-colorscheme --accent-color '#$accent'
if [ -f $wallpaper ]; then
  busctl --user call org.kde.plasmashell /PlasmaShell org.kde.PlasmaShell evaluateScript s '
    desktops().forEach(function (d) {
      d.wallpaperPlugin = "org.kde.image";
      d.currentConfigGroup = ["Wallpaper", "org.kde.image", "General"];
      d.writeConfig("Image", "file://$wallpaper");
      d.writeConfig("FillMode", 6);
      d.writeConfig("Color", "#$bg");
    });'
fi
rm -f "$cfg/autostart/vibian-kde-apply.desktop"
EOF2
chmod +x "$data/vibian/kde-apply.sh"
cat > "$cfg/autostart/vibian-kde-apply.desktop" <<EOF2
[Desktop Entry]
# from the vibian playbook "kde-plasma": removes itself after one run
Type=Application
Name=Vibian look
Exec=$data/vibian/kde-apply.sh
X-KDE-autostart-phase=2
NoDisplay=true
EOF2
written+=("$data/vibian/kde-apply.sh" "$cfg/autostart/vibian-kde-apply.desktop")
# already inside Plasma: apply now too
if [ "${XDG_CURRENT_DESKTOP:-}" = KDE ] && busctl --user status org.kde.plasmashell >/dev/null 2>&1; then
  "$data/vibian/kde-apply.sh" >/dev/null 2>&1 || true
fi

touch "$marker"
printf 'wrote %s\n' "${written[@]}"
