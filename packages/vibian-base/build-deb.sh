#!/bin/bash
# build-deb.sh OUT_DIR — build vibian-base, the glue every Vibian install
# carries: dracut's snapshot-boot overlay and GRUB defaults.
set -euo pipefail
VERSION=0.1.2
here=$(cd "$(dirname "$0")" && pwd)
out=${1:?usage: build-deb.sh OUT_DIR}
mkdir -p "$out"
out=$(cd "$out" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

pkg=$work/pkg
cp -a "$here/root" "$pkg"
mkdir -p "$pkg/DEBIAN"
cat > "$pkg/DEBIAN/control" <<CTL
Package: vibian-base
Version: $VERSION
Architecture: all
Maintainer: Vibian <noreply@github.com>
Depends: btrfs-progs, dracut, dracut-core, grub-btrfs, snapper
Section: admin
Priority: optional
Homepage: https://github.com/jacobzipper/vibian
Description: Vibian base system glue
 Boot-time support for Vibian's btrfs snapshots: a RAM overlay when a
 read-only snapshot is booted from GRUB, and a visible boot menu.
CTL
printf '%s\n' /etc/default/grub.d/90-vibian.cfg /etc/dracut.conf.d/90-vibian.conf > "$pkg/DEBIAN/conffiles"
cat > "$pkg/DEBIAN/postinst" <<'SH'
#!/bin/sh
set -e
if [ "$1" = configure ]; then
  dracut --regenerate-all --force
  if [ -e /boot/grub/grub.cfg ] && command -v update-grub >/dev/null; then
    update-grub || true
  fi
fi
SH
chmod 755 "$pkg/DEBIAN/postinst"

deb="$out/vibian-base_${VERSION}_all.deb"
dpkg-deb --root-owner-group -Zxz --build "$pkg" "$deb" >/dev/null
echo "$deb"
