#!/bin/bash
# build-deb.sh OUT_DIR — build vibian-base, what every Vibian install
# carries: the vibian CLI and onboarding TUI, the agent kit (AGENTS.md and
# skills), dracut's snapshot-boot overlay and GRUB defaults. Needs Go 1.24+.
set -euo pipefail
VERSION=0.6.0
here=$(cd "$(dirname "$0")" && pwd)
out=${1:?usage: build-deb.sh OUT_DIR}
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

pkg=$work/pkg
cp -a "$here/root" "$pkg"
chmod 600 "$pkg/etc/netplan/01-vibian-networkmanager.yaml" # netplan warns on readable configs
repo=$here/../..
mkdir -p "$pkg/usr/bin" "$pkg/usr/share/vibian"
(cd "$repo" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$pkg/usr/bin/vibian" ./cmd/vibian)
cp -a "$repo/agent-kit" "$pkg/usr/share/vibian/agent"
cp -a "$repo/playbooks" "$pkg/usr/share/vibian/playbooks"
cp "$repo/docs/assets/vibian-logo-dark.png" "$pkg/usr/share/vibian/vibian-logo.png"
mkdir -p "$pkg/DEBIAN"
cat > "$pkg/DEBIAN/control" <<CTL
Package: vibian-base
Version: $VERSION
Architecture: amd64
Maintainer: Vibian <noreply@github.com>
Depends: btrfs-progs, dracut, dracut-core, git, grub-btrfs, network-manager, polkitd, snapper, sudo
Recommends: gitleaks
Section: admin
Priority: optional
Homepage: https://github.com/jacobzipper/vibian
Description: Vibian base system
 The vibian CLI (onboarding, agent launcher, ledger, snapshots, undo,
 doctor), the harness-agnostic agent kit, and boot-time support for
 btrfs snapshots: a RAM overlay when a read-only snapshot is booted
 from GRUB, and a visible boot menu. NetworkManager runs networking so
 wifi can be joined from onboarding.
CTL
printf '%s\n' /etc/default/grub.d/90-vibian.cfg /etc/dracut.conf.d/90-vibian.conf /etc/profile.d/vibian-onboard.sh /etc/update-motd.d/00-vibian /etc/netplan/01-vibian-networkmanager.yaml /etc/opencode/opencode.json /etc/codex/requirements.toml > "$pkg/DEBIAN/conffiles"
# Ubuntu's login banner ("Welcome to Ubuntu", "This system has been
# minimized") is moved aside, not edited, so removing vibian-base puts it
# back. /etc/issue{,.net} are replaced; they aren't conffiles here because
# dpkg-divert can't move the originals back over a conffile left behind.
diverted="/etc/issue /etc/issue.net /etc/update-motd.d/00-header /etc/update-motd.d/10-help-text /etc/update-motd.d/50-motd-news /etc/update-motd.d/60-unminimize"
cat > "$pkg/DEBIAN/preinst" <<SH
#!/bin/sh
set -e
if [ "\$1" = install ] || [ "\$1" = upgrade ]; then
  for f in $diverted; do
    dpkg-divert --package vibian-base --add --rename --divert "\$f.ubuntu" "\$f" >/dev/null
  done
fi
SH
cat > "$pkg/DEBIAN/postrm" <<SH
#!/bin/sh
set -e
if [ "\$1" = remove ] || [ "\$1" = abort-install ]; then
  for f in $diverted; do
    dpkg-divert --package vibian-base --remove --rename --divert "\$f.ubuntu" "\$f" >/dev/null
  done
fi
if [ "\$1" = remove ]; then
  # without our netplan file, networkd renders the network again
  systemctl enable systemd-networkd.service systemd-networkd-wait-online.service >/dev/null 2>&1 || true
fi
SH
chmod 755 "$pkg/DEBIAN/preinst" "$pkg/DEBIAN/postrm"
cat > "$pkg/DEBIAN/postinst" <<'SH'
#!/bin/sh
set -e
if [ "$1" = configure ]; then
  # the ledger: any sudo-group member (the agent's user) may append
  install -d -m 2775 -g sudo /var/lib/vibian/ledger
  # NetworkManager takes over from networkd at the next boot. Nothing runs
  # `netplan apply` here: that could drop the connection apt is using.
  # networkd is left with nothing to manage, and its wait-online would hold
  # every boot for two minutes. Only once, so a user who turns it back on keeps it.
  if [ -z "$2" ] || dpkg --compare-versions "$2" lt 0.4.0; then
    systemctl disable systemd-networkd-wait-online.service systemd-networkd.service systemd-networkd.socket >/dev/null 2>&1 || true
  fi
  # /dev/uinput to the user at the seat (computer use), without a reboot
  udevadm control --reload-rules >/dev/null 2>&1 && udevadm trigger --sysname-match=uinput >/dev/null 2>&1 || true
  dracut --regenerate-all --force
  if [ -e /boot/grub/grub.cfg ] && command -v update-grub >/dev/null; then
    update-grub || true
  fi
fi
SH
chmod 755 "$pkg/DEBIAN/postinst"

deb="$out/vibian-base_${VERSION}_amd64.deb"
dpkg-deb --root-owner-group -Zxz --build "$pkg" "$deb" >/dev/null
echo "$deb"
