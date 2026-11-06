#!/bin/bash
# XFCE playbook, root part: XFCE 4.20 on X11 from the Ubuntu archive, and the
# LightDM login screen. Idempotent. Knobs: VIBIAN_APPS, VIBIAN_USER.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
apps=${VIBIAN_APPS:-minimal}
user=${VIBIAN_USER:-${SUDO_USER:-}}
case $apps in minimal|standard) ;; *) echo "VIBIAN_APPS: minimal or standard" >&2; exit 1 ;; esac

pkgs=(xserver-xorg-core xserver-xorg-input-libinput xserver-xorg-video-all x11-xserver-utils
      xfce4-session xfwm4 xfce4-panel xfdesktop4 xfce4-settings xfconf libxfce4ui-utils xfce4-appfinder
      thunar thunar-volman gvfs xfce4-terminal xfce4-notifyd xfce4-power-manager xfce4-power-manager-plugins
      xfce4-screensaver xfce4-whiskermenu-plugin xfce4-pulseaudio-plugin xfce4-screenshooter
      xfce4-clipman-plugin mousepad ristretto greybird-gtk-theme elementary-xfce-icon-theme gnome-themes-extra
      lxpolkit xdg-desktop-portal-gtk xdg-user-dirs pipewire-pulse wireplumber pavucontrol
      network-manager-gnome blueman fonts-noto-core fonts-noto-color-emoji
      lightdm lightdm-gtk-greeter dbus-x11 xcape at-spi2-core)
[ "$apps" = standard ] && pkgs+=(xfce4-goodies xfce4-taskmanager catfish)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"

# Ubuntu enables these user services for every graphical session; a Wayland
# desktop's bar would fail to start here anyway
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
# one login screen at a time: another desktop's may own display-manager.service
for dm in greetd sddm gdm3 cosmic-greeter; do systemctl disable -q "$dm" 2>/dev/null || true; done
install -d /etc/lightdm/lightdm.conf.d
cat > /etc/lightdm/lightdm.conf.d/50-vibian.conf <<'T'
# Written by Vibian's xfce playbook.
[Seat:*]
greeter-session=lightdm-gtk-greeter
user-session=xfce
T
logo=/usr/share/vibian/vibian-logo.png
[ -f /etc/lightdm/lightdm-gtk-greeter.conf ] && [ ! -f /etc/lightdm/lightdm-gtk-greeter.conf.vibian-bak ] && ! grep -q "Vibian's" /etc/lightdm/lightdm-gtk-greeter.conf && cp /etc/lightdm/lightdm-gtk-greeter.conf /etc/lightdm/lightdm-gtk-greeter.conf.vibian-bak
cat > /etc/lightdm/lightdm-gtk-greeter.conf <<T
# Written by Vibian's xfce playbook. The greeter stretches a background image
# to fill the screen, so the logo goes in the user picture instead.
[greeter]
background=#16161e
$([ -f "$logo" ] && echo "default-user-image=$logo")
theme-name=Greybird-dark
icon-theme-name=elementary-xfce-dark
font-name=Noto Sans 11
clock-format=%a %d %b  %H:%M
indicators=~host;~spacer;~clock;~spacer;~session;~power
T
# LightDM asks AccountsService for the user's last session; another
# desktop's playbook may have set it, so point it at XFCE
if [ -n "$user" ] && [ "$user" != root ] && id "$user" >/dev/null 2>&1; then
  f=/var/lib/AccountsService/users/$user
  install -d /var/lib/AccountsService/users
  if [ -f "$f" ]; then
    sed -i -e '/^Session=/d' -e '/^XSession=/d' "$f"
    grep -q '^\[User\]' "$f" || echo '[User]' >> "$f"
    sed -i '/^\[User\]/a Session=xfce\nXSession=xfce' "$f"
  else
    printf '[User]\nSession=xfce\nXSession=xfce\n' > "$f"
  fi
fi
echo /usr/sbin/lightdm > /etc/X11/default-display-manager
systemctl enable -q lightdm.service
systemctl set-default -q graphical.target
echo "system part done: ${#pkgs[@]} packages ($apps), login=lightdm"
