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
#   @var_lib_vibian  /var/lib/vibian  (the ledger: undo and rollback never rewind it)
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
  "@var_lib_vibian:/var/lib/vibian"
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
# the ledger is shared by the user's agents: group sudo may append
install -d -m 2775 -g "$(chroot "$top/@" getent group sudo | cut -d: -f3)" "$top/@var_lib_vibian/ledger"

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

# Vibian's own packages (grub-btrfs isn't in the Ubuntu archive)
pool=$(dirname "$0")/pool
if compgen -G "$pool/*.deb" >/dev/null; then
  mkdir -p "$r/tmp/vibian-pool"
  cp "$pool"/*.deb "$r/tmp/vibian-pool/"
  chroot "$r" sh -c 'dpkg -i /tmp/vibian-pool/*.deb'
  rm -rf "$r/tmp/vibian-pool"
fi

# tty1 logs straight in to onboarding until it's done (vibian onboard removes this)
if [ -x "$r/usr/lib/vibian/tty1-autologin" ]; then
  mkdir -p "$r/etc/systemd/system/getty@tty1.service.d"
  cat > "$r/etc/systemd/system/getty@tty1.service.d/vibian-onboard.conf" <<'EOF'
[Service]
ExecStart=
ExecStart=-/usr/lib/vibian/tty1-autologin %I $TERM
EOF
fi

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

# The pre-snapshot copy at the top level is now dead weight. The installer
# still has /boot/efi and /cdrom mounted on top-level directories, so those
# two stay behind (empty) and rm reports them busy.
find "$top" -mindepth 1 -maxdepth 1 ! -name '@*' -exec rm -rf {} + || true
umount "$top"

# Subiquity copies its own logs to $T/var/log/installer after late-commands.
# Point that path at @var_log so they land in the installed system.
mkdir -p "$T/var/log"
mount -o "subvol=/@var_log,$opts" "$dev" "$T/var/log"
mkdir -p "$T/var/log/installer"
cp "$log" "$T/var/log/installer/vibian-layout.log"
echo "vibian: btrfs layout done" | tee -a "$T/var/log/installer/vibian-layout.log"
