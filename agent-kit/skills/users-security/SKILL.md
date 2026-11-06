---
name: users-security
description: "Users, groups, sudo and the agent's powers, SSH keys and the SSH server, the firewall, secret storage, automatic security updates and AppArmor on Vibian. Use when adding a user or group, changing what the agent may do without asking, setting up SSH access, storing an API key or password, or hardening the machine."
---

# Users and security on Vibian

## The agent's powers
The agent runs as the owner's user. `vibian powers` decides how sudo behaves for it:
- `vibian powers root` (**Everything**): `/etc/sudoers.d/vibian-agent` gives the user `NOPASSWD: ALL`. Snapshots and the ledger are the safety net.
- `vibian powers confirm` (**Ask first**): that file is removed, so sudo asks for the password, and harnesses that can ask before shell commands and edits are switched to asking.
Only the owner changes this, from their own terminal. Never edit `/etc/sudoers.d/vibian-agent` yourself, and never add NOPASSWD rules of your own. Edit sudoers only with `visudo -f`.

## Users and groups
```bash
sudo adduser <name>                       # interactive: the person types their own password
sudo usermod -aG <group> <name>           # takes effect at their next login
id <name>; getent group sudo
sudo deluser --remove-home <name>         # ask first: deletes their files
```
| group | grants |
|---|---|
| `sudo` | admin; on Vibian also networking (polkit) and the ledger |
| `docker` | root-equivalent. Say so before adding anyone |
| `video`, `render` | GPU and backlight devices for non-session use (compute, containers) |
| `input` | raw input devices: needed by some remappers, lets the user read every keystroke |
| `libvirt`, `kvm` | VMs |
| `dialout` | serial ports, microcontrollers |

## SSH
The installer sets up sshd with **password login off**: only keys work.
```bash
ssh-keygen -t ed25519 -C "<user>@<host>"        # the user's own key, for GitHub etc.
install -m 700 -d ~/.ssh; echo "<pubkey>" >> ~/.ssh/authorized_keys; chmod 600 ~/.ssh/authorized_keys
```
- sshd settings go in drop-ins, `/etc/ssh/sshd_config.d/50-vibian-*.conf`. Check with `sudo sshd -t` before `sudo systemctl reload ssh`. Reload, don't restart, while connected over ssh.
- Don't turn password login on without asking; if the machine is reachable from the internet, recommend `fail2ban` or Tailscale instead (see `networking`).
- No ssh server wanted at all: `sudo systemctl disable --now ssh.socket ssh.service`.

## Firewall
See `networking` (ufw). Default to deny incoming, open ports one at a time, and log each one.

## Secrets
- API keys and tokens go where the tool expects them (its config dir, mode 600) or into the keyring: `secret-tool store --label="<what>" service <svc> account <acct>` (`libsecret-tools` + `gnome-keyring`), or `pass` for terminal users.
- Never put a secret in the ledger, a commit, a skill, a shell history line (`HISTCONTROL=ignorespace`, leading space), a world-readable file or your reply. `vibian log add` text is plain and gets exported.
- Harness credentials (`~/.claude.json`, `~/.codex/auth.json`, `~/.config/opencode`, `~/.pi`, …) belong to their harness. Don't read or copy them.
- If you find a leaked secret (in a repo, a dotfile, a log): tell the owner what and where, and that it should be rotated. Don't print it.

## Updates
`unattended-upgrades` installs security updates daily (on by default in Ubuntu). Check with `systemctl status unattended-upgrades` and `/var/log/unattended-upgrades/`. Each run is snapshotted like any apt run. A pending reboot shows as `/var/run/reboot-required`: tell the user, don't reboot for them.

## AppArmor and Secure Boot
- `sudo aa-status`. If a profile blocks something, find the DENIED line with `journalctl -k -g apparmor`. Fix it with a local override in `/etc/apparmor.d/local/`, not by disabling AppArmor.
- `mokutil --sb-state`. Secure Boot stays on; DKMS modules need MOK enrolment (see `drivers-hardware`).

## Disk encryption
The current installer doesn't encrypt the disk (LUKS2 comes later). If the user asks about theft protection, say so plainly. Encrypting an existing install means reinstalling.
