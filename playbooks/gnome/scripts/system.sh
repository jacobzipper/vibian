#!/bin/bash
# GNOME playbook, root part: GNOME 50 from the Ubuntu archive and the GDM
# login screen. Idempotent.
# Knobs: VIBIAN_STYLE (vanilla or ubuntu), VIBIAN_APPS (minimal or standard),
# VIBIAN_USER (whose default session to set; defaults to $SUDO_USER).
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
style=${VIBIAN_STYLE:-vanilla}
apps=${VIBIAN_APPS:-minimal}
user=${VIBIAN_USER:-${SUDO_USER:-}}
case $style in vanilla) session=gnome; base=(gnome-session) ;; ubuntu) session=ubuntu; base=(ubuntu-session yaru-theme-gtk yaru-theme-icon) ;; *) echo "VIBIAN_STYLE: vanilla or ubuntu" >&2; exit 1 ;; esac
case $apps in minimal|standard) ;; *) echo "VIBIAN_APPS: minimal or standard" >&2; exit 1 ;; esac

pkgs=("${base[@]}" gdm3 gnome-shell gnome-control-center gnome-settings-daemon gnome-keyring
      xdg-desktop-portal-gnome xdg-user-dirs-gtk dconf-cli network-manager
      ptyxis nautilus gnome-text-editor loupe gnome-system-monitor gnome-disk-utility
      pipewire-pulse wireplumber fonts-noto-core fonts-noto-color-emoji)
[ "$apps" = standard ] && pkgs+=(gnome-tweaks gnome-shell-extension-manager gnome-calculator gnome-software)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
if [ "$apps" = minimal ]; then
  apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"
else
  apt-get -o DPkg::Lock::Timeout=300 install -y -q "${pkgs[@]}"
fi

# one login screen at a time: another desktop's may own display-manager.service
for dm in greetd sddm lightdm; do systemctl disable -q "$dm" 2>/dev/null || true; done
# a tiling desktop's bar, notifications and idle daemon would start in GNOME
# too (Ubuntu enables them for every graphical session); its config starts them
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
echo /usr/sbin/gdm3 > /etc/X11/default-display-manager 2>/dev/null || true
systemctl enable -q gdm3.service
systemctl set-default -q graphical.target

# preselect this session for the user: GDM otherwise offers the last-used
# one, which may be another desktop's
if [ -n "$user" ] && id "$user" >/dev/null 2>&1; then
  f=/var/lib/AccountsService/users/$user
  install -d -m 700 /var/lib/AccountsService/users
  [ -f "$f" ] || printf '[User]\nSystemAccount=false\n' > "$f"
  grep -q '^\[User\]' "$f" || printf '[User]\n' >> "$f"
  for k in Session XSession; do
    if grep -q "^$k=" "$f"; then sed -i "s/^$k=.*/$k=$session/" "$f"; else sed -i "/^\[User\]/a $k=$session" "$f"; fi
  done
fi
echo "system part done: ${#pkgs[@]} packages ($style, $apps), login=gdm3, session=$session${user:+ for $user}"
