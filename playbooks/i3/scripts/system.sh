#!/bin/bash
# i3 playbook, root part: i3 on X11 from the Ubuntu archive, and the LightDM
# login screen. Idempotent. Knobs: VIBIAN_TERMINAL, VIBIAN_LOGIN, VIBIAN_USER.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
term=${VIBIAN_TERMINAL:-alacritty}
login=${VIBIAN_LOGIN:-lightdm}
user=${VIBIAN_USER:-${SUDO_USER:-}}
case $term in alacritty|kitty|xterm) ;; *) echo "VIBIAN_TERMINAL: alacritty, kitty or xterm (foot is Wayland-only)" >&2; exit 1 ;; esac
case $login in lightdm|tty) ;; *) echo "VIBIAN_LOGIN: lightdm or tty" >&2; exit 1 ;; esac

pkgs=(xserver-xorg-core xserver-xorg-input-libinput xserver-xorg-video-all x11-xserver-utils xinit
      i3-wm i3status i3lock xss-lock xautolock rofi dunst picom feh maim xclip xdotool autotiling dex arandr
      lxpolkit xdg-desktop-portal-gtk brightnessctl playerctl pipewire-pulse wireplumber pavucontrol
      network-manager-gnome blueman fonts-noto-core fonts-noto-color-emoji fonts-font-awesome "$term")
[ "$login" = lightdm ] && pkgs+=(lightdm lightdm-gtk-greeter gnome-themes-extra)
# startx as a normal user needs the Xorg wrapper
[ "$login" = tty ] && pkgs+=(xserver-xorg-legacy)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
# A display manager's postinst starts it on the spot, which would pull the
# screen out from under the install. Hold just the login screens until apt is
# done; they come up at the next boot (or on `systemctl start`).
held=
if [ ! -e /usr/sbin/policy-rc.d ]; then
  printf '#!/bin/sh\nfor a; do case $a in greetd*|gdm*|sddm*|lightdm*|cosmic-greeter*|display-manager*) exit 101 ;; esac; done\n' > /usr/sbin/policy-rc.d
  chmod 755 /usr/sbin/policy-rc.d; held=1; trap 'rm -f /usr/sbin/policy-rc.d' EXIT
fi
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"
[ -z "$held" ] || { rm -f /usr/sbin/policy-rc.d; trap - EXIT; }

# Ubuntu enables these user services for every graphical session; a Wayland
# desktop's bar would fail to start here anyway
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
if [ "$login" = lightdm ]; then
  # one login screen at a time: another desktop's may own display-manager.service
  for dm in greetd sddm gdm3 cosmic-greeter; do systemctl disable -q "$dm" 2>/dev/null || true; done
  install -d /etc/lightdm/lightdm.conf.d
  cat > /etc/lightdm/lightdm.conf.d/50-vibian.conf <<'T'
# Written by Vibian's i3 playbook.
[Seat:*]
greeter-session=lightdm-gtk-greeter
user-session=i3
T
  logo=/usr/share/vibian/vibian-logo.png
  [ -f /etc/lightdm/lightdm-gtk-greeter.conf ] && [ ! -f /etc/lightdm/lightdm-gtk-greeter.conf.vibian-bak ] && cp /etc/lightdm/lightdm-gtk-greeter.conf /etc/lightdm/lightdm-gtk-greeter.conf.vibian-bak
  cat > /etc/lightdm/lightdm-gtk-greeter.conf <<T
# Written by Vibian's i3 playbook. The greeter stretches a background image
# to fill the screen, so the logo goes in the user picture instead.
[greeter]
background=#16161e
$([ -f "$logo" ] && echo "default-user-image=$logo")
theme-name=Adwaita-dark
font-name=Noto Sans 11
clock-format=%a %d %b  %H:%M
indicators=~host;~spacer;~clock;~spacer;~session;~power
T
  # LightDM asks AccountsService for the user's last session; another
  # desktop's playbook may have set it, so point it at i3
  if [ -n "$user" ] && id "$user" >/dev/null 2>&1 && [ "$user" != root ]; then
    f=/var/lib/AccountsService/users/$user
    install -d /var/lib/AccountsService/users
    if [ -f "$f" ]; then
      sed -i -e '/^Session=/d' -e '/^XSession=/d' "$f"
      grep -q '^\[User\]' "$f" || echo '[User]' >> "$f"
      sed -i '/^\[User\]/a Session=i3\nXSession=i3' "$f"
    else
      printf '[User]\nSession=i3\nXSession=i3\n' > "$f"
    fi
  fi
  echo /usr/sbin/lightdm > /etc/X11/default-display-manager
  systemctl enable -q lightdm.service
  systemctl set-default -q graphical.target
fi
echo "system part done: ${#pkgs[@]} packages, login=$login"
