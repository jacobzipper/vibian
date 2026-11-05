#!/bin/bash
# build-deb.sh OUT_DIR — package upstream grub-btrfs as a .deb.
#
# Ubuntu 26.04 and Debian don't ship grub-btrfs, so Vibian builds it from a
# pinned upstream release. It adds a "snapshots" submenu to GRUB, and
# grub-btrfsd refreshes that menu whenever snapper creates or deletes a snapshot.
set -euo pipefail

VERSION=4.14
SHA256=4fe2f8a49b39e3e561fc51bbc5a0f81a7222c1e9ef8672e014e0ba3bedad20c6
URL=https://github.com/Antynea/grub-btrfs/archive/refs/tags/v$VERSION.tar.gz
REV=1vibian3

out=${1:?usage: build-deb.sh OUT_DIR}
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

curl -fsSL -o "$work/src.tar.gz" "$URL"
echo "$SHA256  $work/src.tar.gz" | sha256sum -c --quiet -
tar -xzf "$work/src.tar.gz" -C "$work"

pkg=$work/pkg
make -s -C "$work/grub-btrfs-$VERSION" install DESTDIR="$pkg" \
  SYSTEMD=true OPENRC=false GRUB_UPDATE_EXCLUDE=true INSTALL_DOCS=true >/dev/null
rm -rf "$pkg/usr/lib/initcpio"  # Arch-only overlay hook
# Snapshots are read-only: have dracut boot them under a RAM overlay
# (vibian-base adds dracut's overlayfs module).
sed -i 's|^#GRUB_BTRFS_SNAPSHOT_KERNEL_PARAMETERS=.*|GRUB_BTRFS_SNAPSHOT_KERNEL_PARAMETERS="rd.overlay=1"|' \
  "$pkg/etc/default/grub-btrfs/config"
# grub-btrfs names the submenu after /etc/os-release, which still says Ubuntu
sed -i 's|^#GRUB_BTRFS_SUBMENUNAME=.*|GRUB_BTRFS_SUBMENUNAME="Vibian snapshots"|' \
  "$pkg/etc/default/grub-btrfs/config"
grep -q '^GRUB_BTRFS_SNAPSHOT_KERNEL_PARAMETERS="rd.overlay=1"' "$pkg/etc/default/grub-btrfs/config"
grep -q '^GRUB_BTRFS_SUBMENUNAME="Vibian snapshots"' "$pkg/etc/default/grub-btrfs/config"

mkdir -p "$pkg/DEBIAN"
cat > "$pkg/DEBIAN/control" <<CTL
Package: grub-btrfs
Version: $VERSION-$REV
Architecture: all
Maintainer: Vibian <noreply@github.com>
Depends: bash, btrfs-progs, grub-common, inotify-tools
Section: admin
Priority: optional
Homepage: https://github.com/Antynea/grub-btrfs
Description: GRUB submenu for booting btrfs snapshots
 Adds a GRUB submenu listing btrfs snapshots (snapper, timeshift, yabsnap)
 and a daemon, grub-btrfsd, that regenerates it when snapshots change.
 Built by Vibian from the upstream v$VERSION release.
CTL
echo /etc/default/grub-btrfs/config > "$pkg/DEBIAN/conffiles"
cat > "$pkg/DEBIAN/postinst" <<'SH'
#!/bin/sh
set -e
if [ "$1" = configure ]; then
  if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
    systemctl enable --now grub-btrfsd.service || true
  else
    systemctl enable grub-btrfsd.service || true
  fi
  if [ -e /boot/grub/grub.cfg ] && command -v update-grub >/dev/null; then
    update-grub || true
  fi
fi
SH
cat > "$pkg/DEBIAN/prerm" <<'SH'
#!/bin/sh
set -e
if [ "$1" = remove ]; then
  systemctl disable --now grub-btrfsd.service 2>/dev/null || true
fi
SH
chmod 755 "$pkg/DEBIAN/postinst" "$pkg/DEBIAN/prerm"

deb="$out/grub-btrfs_${VERSION}-${REV}_all.deb"
dpkg-deb --root-owner-group -Zxz --build "$pkg" "$deb" >/dev/null
echo "$deb"
