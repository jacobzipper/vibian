---
name: recovery
description: "Undo changes and recover a broken Vibian system with snapper snapshots, the ledger, boot-to-snapshot from GRUB, rollback, initramfs/GRUB repair and chroot from a live USB. Use when something broke after a change, the machine won't boot, the user says \"undo that\" or \"go back to how it was\", or before doing anything to boot, kernel, fstab or initramfs."
---

# Recovery on Vibian

## The layers (cheapest first)
| problem | tool |
|---|---|
| one change went wrong | `vibian undo <ledger-id>` (or `--last`) |
| several changes, or no ledger entry | `snapper -c root list` → `sudo snapper -c root undochange A..B` (same for `-c home`) |
| system boots but is broken badly | `vibian undo --to <snapshot>`, then reboot |
| system won't boot | GRUB → **Vibian snapshots** → pick a snapshot → kernel; then `vibian undo --to <n>` and reboot |
| GRUB itself is gone | live USB → chroot (below) |

## Reading what happened
```bash
vibian log tail 20                         # what agents/humans did
vibian log search nvidia                    # find the entry
snapper -c root list | tail -20            # apt pre/post pairs and agent sessions
sudo snapper -c root status 41..42         # files changed between two snapshots
sudo snapper -c root diff 41..42 /etc/default/grub
```
`vibian undo <id>` prints how many paths it will revert and asks first. Pass `--yes` only when the owner already agreed.

## Booting a snapshot
- GRUB shows its menu for 3 seconds (10 after a failed boot). **Vibian snapshots** is the last entry. Each snapshot has its own kernels; pick the newest kernel the snapshot has.
- That boot runs with `rd.overlay=1`: the snapshot is read-only underneath, and writes go to RAM and vanish at reboot. `vibian doctor` says "running from a snapshot".
- To keep it: `vibian undo --to <n>` (n is in `/proc/cmdline`, `@snapshots/<n>/snapshot`) then reboot. The broken root is set aside as `@rollback-<date>` and kept for 7 days (`vibian-rollback-cleanup.service` deletes older ones at boot). From a normal boot a snapshot of it is taken too, and the ledger entry names it.

## Things that don't roll back
`/home` (has its own `home` config), `/var/log`, `/var/cache`, `/var/lib/docker`, `/var/lib/libvirt` and `/var/lib/vibian` (the ledger) are separate subvolumes. Tell the user that a root rollback won't touch their files or containers.

## Repairs
- **initramfs** (Vibian uses dracut, not initramfs-tools): `sudo dracut --force --kver "$(uname -r)"` (repeat with each `--kver` in `/boot/vmlinuz-*`; `--regenerate-all` fails if a removed kernel left `/usr/lib/modules/VERSION` behind). The `overlayfs` dracut module must stay included (`/etc/dracut.conf.d/90-vibian.conf`), or snapshot boot breaks.
- **GRUB config**: `sudo update-grub`. The ESP stub `/boot/efi/EFI/ubuntu/grub.cfg` must point at `/@/boot/grub`.
- **fstab**: root is `subvol=/@`. Never change it to a snapshot path; use `vibian undo --to`.
- **chroot from a live USB** (Ubuntu or Vibian ISO, "Try"):
  ```bash
  sudo mount -o subvol=@ /dev/<root> /mnt
  sudo mount -o subvol=@home /dev/<root> /mnt/home
  sudo mount /dev/<esp> /mnt/boot/efi
  for d in dev dev/pts proc sys run; do sudo mount --bind /$d /mnt/$d; done
  sudo chroot /mnt
  ```
  Or replace the root from the live USB:
  ```bash
  sudo mount -o subvolid=5 /dev/<root> /mnt
  sudo mv /mnt/@ /mnt/@broken
  sudo btrfs subvolume snapshot /mnt/@snapshots/<n>/snapshot /mnt/@
  ```

## Always
- Before touching kernel, initramfs, GRUB, fstab or drivers: `vibian snap "why"` and tell the owner the recovery path (the GRUB snapshots menu) **before** you start.
- After any recovery, write a ledger entry with a `--gotcha` explaining what broke, so it doesn't happen again.
