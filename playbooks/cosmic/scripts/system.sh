#!/bin/bash
# COSMIC playbook, root part. COSMIC isn't in Ubuntu 26.04's archive, so this
# adds System76's Pop!_OS repo for resolute, signed by the key shipped next
# to this script (files/pop-os.asc, fingerprint 63C4 6DF0 140D 7389 6142
# 9F4E 204D D8AE C33A 7AFF). The repo also carries its own kernel, Mesa,
# firmware and more, so it's pinned below Ubuntu: Ubuntu's version wins
# wherever both have a package, and only what Ubuntu lacks (COSMIC itself,
# pop-launcher, fonts, icons) comes from Pop. Then the COSMIC greeter.
# Idempotent. Knobs: VIBIAN_APPS, VIBIAN_USER.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
apps=${VIBIAN_APPS:-minimal}
user=${VIBIAN_USER:-${SUDO_USER:-}}
case $apps in minimal|standard) ;; *) echo "VIBIAN_APPS: minimal or standard" >&2; exit 1 ;; esac
here=$(cd "$(dirname "$0")/.." && pwd)

install -Dm644 "$here/files/pop-os.asc" /etc/apt/keyrings/pop-os.asc
cat > /etc/apt/sources.list.d/pop-os.sources <<'T'
# Written by Vibian's cosmic playbook: COSMIC from System76. See
# /etc/apt/preferences.d/pop-os.pref for why it never replaces Ubuntu packages.
Types: deb
URIs: http://apt.pop-os.org/release
Suites: resolute
Components: main
Signed-By: /etc/apt/keyrings/pop-os.asc
T
cat > /etc/apt/preferences.d/pop-os.pref <<'T'
# Written by Vibian's cosmic playbook. Ubuntu's archive is at 500, so
# Ubuntu's kernel, Mesa, firmware and the rest always win; packages only Pop
# has still install and update from here.
Package: *
Pin: origin apt.pop-os.org
Pin-Priority: 100
T

pkgs=(cosmic-session cosmic-term cosmic-files cosmic-edit cosmic-wallpapers
      cosmic-screenshot wtype pipewire-pulse wireplumber xdg-user-dirs fonts-noto-core fonts-noto-color-emoji)
[ "$apps" = standard ] && pkgs+=(cosmic-store cosmic-player cosmic-viewer cosmic-monitor)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
# cosmic-session conflicts with seatd (COSMIC uses logind), so apt removes it
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"

# Ubuntu enables these user services for every graphical session; COSMIC has
# its own panel, notifications and idle handling
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
# one login screen at a time: another desktop's may own display-manager.service
for dm in greetd sddm gdm3 lightdm; do systemctl disable -q "$dm" 2>/dev/null || true; done
# the package leaves its display-manager alias commented out; this drop-in
# gives it one, so enable and disable manage it like any other login screen
install -d /etc/systemd/system/cosmic-greeter.service.d
cat > /etc/systemd/system/cosmic-greeter.service.d/vibian.conf <<'T'
# Written by Vibian's cosmic playbook.
[Install]
Alias=display-manager.service
T
systemctl daemon-reload
# the greeter offers the user's last session; another desktop's playbook may
# have set it in AccountsService, so point it at COSMIC
if [ -n "$user" ] && [ "$user" != root ] && id "$user" >/dev/null 2>&1; then
  f=/var/lib/AccountsService/users/$user
  install -d /var/lib/AccountsService/users
  if [ -f "$f" ]; then
    sed -i -e '/^Session=/d' -e '/^XSession=/d' "$f"
    grep -q '^\[User\]' "$f" || echo '[User]' >> "$f"
    sed -i '/^\[User\]/a Session=cosmic' "$f"
  else
    printf '[User]\nSession=cosmic\n' > "$f"
  fi
fi
# cosmic-bg draws transparent pixels black, so the wallpaper is the logo
# already laid on Vibian's background colour
install -Dm644 "$here/files/vibian-wallpaper.png" /usr/share/backgrounds/vibian/vibian-wallpaper.png
echo /usr/bin/cosmic-greeter > /etc/X11/default-display-manager
systemctl enable -q cosmic-greeter.service
systemctl set-default -q graphical.target
echo "system part done: ${#pkgs[@]} packages ($apps), login=cosmic-greeter"
