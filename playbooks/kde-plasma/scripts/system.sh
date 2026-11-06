#!/bin/bash
# KDE Plasma playbook, root part: Plasma 6 from the Ubuntu archive and the
# SDDM login screen on its Wayland greeter (no X server needed).
# Idempotent. Knob: VIBIAN_APPS (minimal or standard).
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
apps=${VIBIAN_APPS:-minimal}
case $apps in minimal|standard) ;; *) echo "VIBIAN_APPS: minimal or standard" >&2; exit 1 ;; esac

pkgs=(kde-plasma-desktop plasma-session-wayland kwin-wayland sddm sddm-theme-breeze breeze
      plasma-nm plasma-pa powerdevil bluedevil kscreen kde-config-screenlocker xdg-desktop-portal-kde
      konsole dolphin kde-spectacle ark kate libkf6config-bin pipewire-pulse wireplumber
      fonts-noto-core fonts-noto-color-emoji)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
if [ "$apps" = minimal ]; then
  apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"
else
  apt-get -o DPkg::Lock::Timeout=300 install -y -q "${pkgs[@]}"
fi

# one login screen at a time: another desktop's may own display-manager.service
for dm in greetd gdm3 lightdm cosmic-greeter; do systemctl disable -q "$dm" 2>/dev/null || true; done
# a tiling desktop's bar, notifications and idle daemon would start in Plasma
# too (Ubuntu enables them for every graphical session); its config starts them
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
install -d /etc/sddm.conf.d
cat > /etc/sddm.conf.d/10-vibian.conf <<'T'
# Written by Vibian's kde-plasma playbook: the greeter runs on KWin
# (Wayland), so no X server is needed.
[General]
DisplayServer=wayland
GreeterEnvironment=QT_WAYLAND_SHELL_INTEGRATION=layer-shell

[Wayland]
CompositorCommand=kwin_wayland --drm --no-lockscreen --no-global-shortcuts --locale1

[Theme]
Current=breeze
T
# preselect Plasma (Wayland): SDDM otherwise offers the last-used or first
# session, which may be another desktop's
if id sddm >/dev/null 2>&1; then
  install -d -o sddm -g sddm /var/lib/sddm
  kwriteconfig6 --file /var/lib/sddm/state.conf --group Last --key Session /usr/share/wayland-sessions/plasma.desktop
  chown sddm:sddm /var/lib/sddm/state.conf
fi
echo /usr/bin/sddm > /etc/X11/default-display-manager 2>/dev/null || true
systemctl enable -q sddm.service
systemctl set-default -q graphical.target
echo "system part done: ${#pkgs[@]} packages ($apps), login=sddm"
