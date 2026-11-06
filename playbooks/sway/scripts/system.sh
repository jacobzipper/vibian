#!/bin/bash
# Sway playbook, root part: packages from the Ubuntu archive and the
# login screen. Idempotent. Knobs: VIBIAN_TERMINAL, VIBIAN_LOGIN.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
term=${VIBIAN_TERMINAL:-foot}
login=${VIBIAN_LOGIN:-greetd}
case $term in foot|kitty|alacritty) ;; *) echo "VIBIAN_TERMINAL: foot, kitty or alacritty" >&2; exit 1 ;; esac
case $login in greetd|tty) ;; *) echo "VIBIAN_LOGIN: greetd or tty" >&2; exit 1 ;; esac

pkgs=(sway swayidle swaylock xdg-desktop-portal-wlr xdg-desktop-portal-gtk lxpolkit autotiling
      waybar fuzzel mako-notifier swaybg grim slurp wl-clipboard cliphist brightnessctl playerctl
      pipewire-pulse wireplumber pavucontrol network-manager-gnome blueman
      fonts-noto-core fonts-noto-color-emoji fonts-font-awesome qt6-wayland qtwayland5 "$term")
[ "$login" = greetd ] && pkgs+=(greetd tuigreet)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"

# Sway won't start on NVIDIA's proprietary driver without this flag
session=sway
[ -e /sys/module/nvidia_drm ] && session="sway --unsupported-gpu"

# Ubuntu enables these user services for every graphical session, so they
# would also start inside Plasma or GNOME. This desktop's config starts them.
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
if [ "$login" = greetd ]; then
  # one login screen at a time: another desktop's may own display-manager.service
  for dm in sddm gdm3 lightdm; do systemctl disable -q "$dm" 2>/dev/null || true; done
  install -d /etc/greetd
  [ -f /etc/greetd/config.toml ] && [ ! -f /etc/greetd/config.toml.vibian-bak ] && cp /etc/greetd/config.toml /etc/greetd/config.toml.vibian-bak
  greeter=$(getent passwd _greetd >/dev/null && echo _greetd || echo greeter)
  cat > /etc/greetd/config.toml <<T
# Written by Vibian's sway playbook.
# vt 7 is Ubuntu's default: tty1 keeps its console (and onboarding)
[terminal]
vt = 7

[default_session]
command = "tuigreet --time --remember --asterisks --cmd '$session'"
user = "$greeter"
T
  # tuigreet's --remember needs its cache dir
  install -d -o "$greeter" -m 0755 /var/cache/tuigreet
  systemctl enable greetd.service
  systemctl set-default graphical.target
fi
echo "system part done: ${#pkgs[@]} packages, login=$login"
