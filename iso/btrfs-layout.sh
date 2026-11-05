#!/bin/bash
# btrfs-layout.sh TARGET — turn curtin's fresh single-subvolume btrfs install
# (mounted at TARGET, usually /target) into Vibian's snapshot layout, then set
# up snapper for / and /home. Runs from autoinstall late-commands.
#
#   @            /                  @snapshots       /.snapshots
#   @home        /home              @home_snapshots  /home/.snapshots
#   @var_log     /var/log           @var_cache       /var/cache
#   @var_lib_docker  /var/lib/docker
#   @var_lib_libvirt /var/lib/libvirt
#
# Curtin can't create subvolumes itself (spike S1), hence this script.
set -euxo pipefail
log=/run/vibian-layout.log
exec > >(tee -a "$log") 2>&1

T=${1:-/target}
[ "$(findmnt -no FSTYPE "$T")" = btrfs ] || { echo "vibian: / is not btrfs, leaving layout alone"; exit 0; }
dev=$(findmnt -no SOURCE "$T" | sed 's/\[.*\]$//')
uuid=$(blkid -s UUID -o value "$dev")
esp=$(findmnt -no SOURCE "$T/boot/efi" || true)
opts=noatime,compress=zstd:1

top=/run/vibian-top
mkdir -p "$top"
mount -o subvolid=5 "$dev" "$top"

btrfs subvolume snapshot "$top" "$top/@"

# subvolume -> mount point; existing contents move across with reflinks
subvols=(
  "@home:/home"
  "@var_log:/var/log"
  "@var_cache:/var/cache"
  "@var_lib_docker:/var/lib/docker"
  "@var_lib_libvirt:/var/lib/libvirt"
  "@snapshots:/.snapshots"
  "@home_snapshots:/home/.snapshots"
)
for entry in "${subvols[@]}"; do
  sv=${entry%%:*} mp=${entry#*:}
  btrfs subvolume create "$top/$sv"
  if [ -d "$top/@$mp" ]; then
    cp -a --reflink=always "$top/@$mp/." "$top/$sv/"
    find "$top/@$mp" -mindepth 1 -delete
  fi
  mkdir -p "$top/@$mp"
done
# @home_snapshots is mounted inside /home, so its mount point lives in @home
mkdir -p "$top/@home/.snapshots"

# fstab: replace curtin's root line, keep everything else (ESP, swap)
fstab="$top/@/etc/fstab"
awk '$2 != "/"' "$fstab" > "$fstab.new"
{
  echo "UUID=$uuid / btrfs subvol=/@,$opts 0 0"
  for entry in "${subvols[@]}"; do
    echo "UUID=$uuid ${entry#*:} btrfs subvol=/${entry%%:*},$opts 0 0"
  done
} >> "$fstab.new"
mv "$fstab.new" "$fstab"

btrfs subvolume set-default "$top/@"

# Re-run the boot setup from inside the new root so GRUB learns the /@ prefix.
r=/run/vibian-root
mkdir -p "$r"
mount -o "subvol=/@,$opts" "$dev" "$r"
mount -o "subvol=/@home,$opts" "$dev" "$r/home"
if [ -n "$esp" ]; then mount "$esp" "$r/boot/efi"; fi
for d in dev dev/pts proc sys run; do mount --bind "/$d" "$r/$d"; done
if [ -d /sys/firmware/efi/efivars ]; then mount --bind /sys/firmware/efi/efivars "$r/sys/firmware/efi/efivars"; fi

chroot "$r" update-grub
if [ -n "$esp" ] && [ -f "$r/boot/efi/EFI/ubuntu/grub.cfg" ]; then
  # signed GRUB reads this stub from the ESP to find /boot/grub
  cat > "$r/boot/efi/EFI/ubuntu/grub.cfg" <<EOF
search.fs_uuid $uuid root
set prefix=(\$root)'/@/boot/grub'
configfile \$prefix/grub.cfg
EOF
fi

# snapper refuses to create a config over an existing .snapshots, so let it
# make its nested subvolume, then swap in ours.
if chroot "$r" sh -c 'command -v snapper' >/dev/null; then
  for cfg in root:/ home:/home; do
    name=${cfg%%:*} path=${cfg#*:}
    snapdir="$r${path%/}/.snapshots"
    rmdir "$snapdir"
    chroot "$r" snapper --no-dbus -c "$name" create-config "$path"
    btrfs subvolume delete "$snapdir"
    mkdir "$snapdir"
    chroot "$r" sed -i \
      -e 's/^ALLOW_GROUPS=.*/ALLOW_GROUPS="sudo"/' \
      -e 's/^NUMBER_LIMIT=.*/NUMBER_LIMIT="50"/' \
      -e 's/^TIMELINE_LIMIT_HOURLY=.*/TIMELINE_LIMIT_HOURLY="5"/' \
      -e 's/^TIMELINE_LIMIT_DAILY=.*/TIMELINE_LIMIT_DAILY="7"/' \
      "/etc/snapper/configs/$name"
  done
  chmod 750 "$r/.snapshots" "$r/home/.snapshots"
fi

for d in sys/firmware/efi/efivars run proc dev/pts dev sys boot/efi home; do
  if mountpoint -q "$r/$d"; then umount "$r/$d"; fi
done
umount "$r"

# The pre-snapshot copy at the top level is now dead weight.
find "$top" -mindepth 1 -maxdepth 1 ! -name '@*' -exec rm -rf {} +
echo "vibian: btrfs layout done"
mkdir -p "$top/@var_log/installer" && cp "$log" "$top/@var_log/installer/vibian-layout.log"
umount "$top"
