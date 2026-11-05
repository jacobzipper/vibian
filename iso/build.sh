#!/bin/bash
# build.sh UBUNTU_ISO OUT_ISO [--unattended]
#
# Remaster the Ubuntu Server live ISO into a Vibian installer: new boot menu,
# /autoinstall.yaml and the /vibian payload. Boot records are replayed from the
# source ISO, so BIOS + UEFI + Secure Boot keep working.
#
# --unattended is for VM tests only: no confirmation before wiping the largest
# disk, and a throwaway login (vibian / vibian).
# VIBIAN_CMDLINE_EXTRA adds kernel arguments (e.g. console=ttyS0,115200).
set -euo pipefail

UBUNTU_SHA256=cc8a95cde20f6ced61a322420de00f10cc3c90ced545daa46cb9c1a117f1d927
here=$(cd "$(dirname "$0")" && pwd)
src=${1:?usage: build.sh UBUNTU_ISO OUT_ISO [--unattended]}
out=${2:?usage: build.sh UBUNTU_ISO OUT_ISO [--unattended]}
unattended=${3:-}

if [ -z "${VIBIAN_SKIP_SHA:-}" ]; then
  echo "$UBUNTU_SHA256  $src" | sha256sum -c --quiet -
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/vibian"
cp "$here/btrfs-layout.sh" "$work/vibian/"
cp "$here/autoinstall.yaml" "$work/autoinstall.yaml"

cmdline=""
if [ "$unattended" = --unattended ]; then
  cmdline="autoinstall"
  python3 - "$work/autoinstall.yaml" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read()
s = s.replace("  interactive-sections:\n    - network\n    - identity\n", "")
s = s.replace("  version: 1\n", """  version: 1
  identity:
    hostname: vibian-test
    username: vibian
    realname: Vibian Test
    # "vibian"
    password: "$6$vibian.test$DDgOhvYVgoZYQdI/Ch1.ir.3Rr4SK05bPKN7XZwqdGz5A/dB3WXRr1pk9BB13Tyd6v/MNgcmloYheDmXTxV.1/"
  ssh:
    install-server: true
    allow-pw: true
""", 1)
# drop the release ssh block now that the test one replaced it
head, sep, tail = s.partition("  ssh:\n    install-server: true\n    allow-pw: false\n")
open(p, "w").write(head + tail)
PY
fi
cmdline="$cmdline ${VIBIAN_CMDLINE_EXTRA:-}"
sed "s|@CMDLINE@|${cmdline# }|" "$here/grub.cfg" > "$work/grub.cfg"

# keep casper's integrity check honest
xorriso -osirrox on -indev "$src" -extract /md5sum.txt "$work/md5sum.txt" 2>/dev/null
chmod u+w "$work/md5sum.txt"
grep -vE ' \./(boot/grub/grub\.cfg|autoinstall\.yaml|vibian/.*)$' "$work/md5sum.txt" > "$work/md5.new"
(cd "$work" && md5sum grub.cfg | sed 's| grub.cfg| ./boot/grub/grub.cfg|'
 md5sum autoinstall.yaml | sed 's| autoinstall.yaml| ./autoinstall.yaml|'
 find vibian -type f -exec md5sum {} + | sed 's| vibian/| ./vibian/|') >> "$work/md5.new"

rm -f "$out"
xorriso -indev "$src" -outdev "$out" \
  -volid "Vibian 26.04 amd64" \
  -map "$work/grub.cfg" /boot/grub/grub.cfg \
  -map "$work/autoinstall.yaml" /autoinstall.yaml \
  -map "$work/vibian" /vibian \
  -map "$work/md5.new" /md5sum.txt \
  -boot_image any replay
sha256sum "$out"
