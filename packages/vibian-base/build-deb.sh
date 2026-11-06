#!/bin/bash
# build-deb.sh OUT_DIR — build vibian-base, what every Vibian install
# carries: the vibian CLI and onboarding TUI, the agent kit (AGENTS.md and
# skills), dracut's snapshot-boot overlay and GRUB defaults. Needs Go 1.24+.
set -euo pipefail
VERSION=0.3.0
here=$(cd "$(dirname "$0")" && pwd)
out=${1:?usage: build-deb.sh OUT_DIR}
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

pkg=$work/pkg
cp -a "$here/root" "$pkg"
repo=$here/../..
mkdir -p "$pkg/usr/bin" "$pkg/usr/share/vibian"
(cd "$repo" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$pkg/usr/bin/vibian" ./cmd/vibian)
cp -a "$repo/agent-kit" "$pkg/usr/share/vibian/agent"
mkdir -p "$pkg/DEBIAN"
cat > "$pkg/DEBIAN/control" <<CTL
Package: vibian-base
Version: $VERSION
Architecture: amd64
Maintainer: Vibian <noreply@github.com>
Depends: btrfs-progs, dracut, dracut-core, git, grub-btrfs, snapper, sudo
Section: admin
Priority: optional
Homepage: https://github.com/jacobzipper/vibian
Description: Vibian base system
 The vibian CLI (onboarding, agent launcher, ledger, snapshots, undo,
 doctor), the harness-agnostic agent kit, and boot-time support for
 btrfs snapshots: a RAM overlay when a read-only snapshot is booted
 from GRUB, and a visible boot menu.
CTL
printf '%s\n' /etc/default/grub.d/90-vibian.cfg /etc/dracut.conf.d/90-vibian.conf /etc/profile.d/vibian-onboard.sh /etc/update-motd.d/00-vibian > "$pkg/DEBIAN/conffiles"
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
SH
chmod 755 "$pkg/DEBIAN/preinst" "$pkg/DEBIAN/postrm"
cat > "$pkg/DEBIAN/postinst" <<'SH'
#!/bin/sh
set -e
if [ "$1" = configure ]; then
  # the ledger: any sudo-group member (the agent's user) may append
  install -d -m 2775 -g sudo /var/lib/vibian/ledger
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
