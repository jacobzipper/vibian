---
name: packages
description: Install, remove, upgrade or find software on Vibian (apt, PPAs, Flatpak, Snap, upstream .debs and installers), fix broken dpkg/apt states, and handle mirror or DNS failures. Use for any "install X", "update the system", "apt is broken" or "where does this package come from" request.
---

# Packages on Vibian

Vibian is Ubuntu 26.04 ("resolute"). apt is the default; every apt run is wrapped in a snapper pre/post snapshot by `/etc/apt/apt.conf.d/80snapper`, so a bad install can be undone even without a ledger entry.

## Order of preference
1. **Ubuntu archive** (`apt install`). Check first: `apt-cache policy <pkg>` / `apt search <words>`.
2. **Vendor apt repo** (Docker, VS Code, Chrome, Tailscale, …): signed with a keyring in `/etc/apt/keyrings/`, declared with `signed-by=` in a deb822 `.sources` file in `/etc/apt/sources.list.d/`. Never `apt-key`.
3. **Flatpak** (Flathub) for desktop apps that need newer versions. Install `flatpak` first and add Flathub with `flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo`.
4. **PPA** only when 1 to 3 don't fit. Note it in the ledger, because PPAs are what break release upgrades.
5. **Upstream script or tarball** last. Install to `~/.local` or `/opt`, never over files dpkg owns.

## Workflow
```bash
vibian snap "install <thing>"          # optional for plain apt installs (the hook covers it), required for repos/PPAs
sudo apt-get update
sudo apt-get install -y <pkgs>
vibian log add "install <thing> for <reason>" --pkgs +a,+b --files /etc/apt/sources.list.d/x.sources
```
For removals, use `--pkgs -name`. Write the reason, because the next agent will ask "why is this here?".

## Fixing a broken state
| symptom | fix |
|---|---|
| `dpkg was interrupted` | `sudo dpkg --configure -a` |
| unmet dependencies / held broken | `sudo apt-get -f install`, then `apt-mark showhold` |
| a postinst fails | read its log in the error; `sudo dpkg --configure <pkg>` after fixing; as a last resort `vibian undo --last` |
| `Could not resolve archive.ubuntu.com` | `resolvectl status`; try `resolvectl query archive.ubuntu.com`; temporary fallback: `sudo resolvectl dns <iface> 1.1.1.1` |
| mirror is slow or down | edit `/etc/apt/sources.list.d/ubuntu.sources` (`URIs:`); ledger it |
| lock held | another apt is running (unattended-upgrades): `systemctl status unattended-upgrades`, wait. Never delete the lock files |

## Upgrades
- `sudo apt-get update && sudo apt-get full-upgrade` is fine. Snapshots cover it.
- Kernel updates: after installing, check `ls /boot` and that `update-grub` ran. On a btrfs root the snapshot submenu is refreshed by `grub-btrfsd`.
- Release upgrades (`do-release-upgrade`): ask the owner first, snapshot, and list PPAs that will be disabled.

## Gotchas
- `apt` (not `apt-get`) warns it has no stable CLI; use `apt-get` in scripts.
- Snap is installed on Ubuntu but Vibian doesn't push it. Use it if the user asks or it's the only official channel (some vendor CLIs).
- Packages that ship kernel modules (DKMS: NVIDIA, VirtualBox, v4l2loopback) need the matching `linux-headers-$(uname -r)` and, with Secure Boot on, MOK enrolment. See the `drivers-hardware` skill.
