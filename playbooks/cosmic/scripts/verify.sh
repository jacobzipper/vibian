#!/bin/bash
# COSMIC playbook check: the Pop repo and its pin, packages, the login screen
# and the user's settings. Prints one line per check and VERIFY OK or
# VERIFY FAILED.
set -u
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
data=${XDG_DATA_HOME:-$HOME/.local/share}
c=$cfg/cosmic/com.system76
bad=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; bad=1; fi; }
check "Pop repo signed by its key" "grep -q 'Signed-By: /etc/apt/keyrings/pop-os.asc' /etc/apt/sources.list.d/pop-os.sources && test -s /etc/apt/keyrings/pop-os.asc"
check "Pop repo pinned below Ubuntu" "apt-cache policy | grep -A1 ' 100 http://apt.pop-os.org/release' | grep -q 'o=pop-os-release\\|origin apt.pop-os.org'"
# the installed version's source is the line after ***
check "kernel and Mesa still Ubuntu's" "! apt-cache policy linux-image-\$(uname -r) libgl1-mesa-dri | grep -A1 '^ \\*\\*\\*' | grep -q pop-os"
for p in cosmic-session cosmic-comp cosmic-panel cosmic-settings cosmic-greeter cosmic-term cosmic-files xdg-desktop-portal-cosmic; do
  check "package $p" "dpkg-query -W -f '\${Status}' $p | grep -q 'install ok installed'"
done
check "a COSMIC Wayland session" "test -f /usr/share/wayland-sessions/cosmic.desktop"
check "cosmic-greeter is the login screen" "readlink -f /etc/systemd/system/display-manager.service | grep -q cosmic-greeter"
check "boots to graphical.target" "test \$(systemctl get-default) = graphical.target"
check "no tiling-desktop bar in COSMIC" "! systemctl --global is-enabled -q waybar.service"
check "the login screen doesn't wait for onboarding" "grep -q 'case \$- in \\*i\\*' /etc/profile.d/vibian-onboard.sh"
check "theme set" "grep -qx 'true\\|false' $c.CosmicTheme.Mode/v1/is_dark"
check "wallpaper set" "grep -q 'source: Path(\"/' $c.CosmicBackground/v1/all"
check "keyboard set" "grep -q 'layout: \"[a-z]' $c.CosmicComp/v1/xkb_config"
check "idle timeouts set" "test -s $c.CosmicIdle/v1/screen_off_time && test -s $c.CosmicIdle/v1/suspend_on_ac_time"
check "SUPER+Return opens a terminal" "grep -q '\"Return\"): Spawn(\"cosmic-term\")' $c.CosmicSettings.Shortcuts/v1/custom"
check "SUPER+A runs the agent" "grep -q '\"a\"): Spawn(\"cosmic-term -e vibian agent\")' $c.CosmicSettings.Shortcuts/v1/custom"
check "agent pinned to the dock" "grep -q vibian-agent $c.CosmicAppList/v1/favorites && test -f $data/applications/vibian-agent.desktop"
check "one polkit agent (no lxpolkit in COSMIC)" "test ! -f /etc/xdg/autostart/lxpolkit.desktop || grep -q '^NotShowIn=.*COSMIC;' $cfg/autostart/lxpolkit.desktop"
check "old settings backed up" "tar -tf $data/vibian/cosmic-config.vibian-bak.tar"
[ $bad = 0 ] && echo "VERIFY OK" || echo "VERIFY FAILED"
exit $bad
