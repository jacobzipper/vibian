#!/bin/bash
# build-deb.sh OUT_DIR — build vibian-base, what every Vibian install
# carries: the vibian CLI and onboarding TUI, the agent kit (AGENTS.md and
# skills), dracut's snapshot-boot overlay and GRUB defaults. Needs Go 1.24+.
set -euo pipefail
VERSION=0.2.0
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
printf '%s\n' /etc/default/grub.d/90-vibian.cfg /etc/dracut.conf.d/90-vibian.conf /etc/profile.d/vibian-onboard.sh > "$pkg/DEBIAN/conffiles"
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
