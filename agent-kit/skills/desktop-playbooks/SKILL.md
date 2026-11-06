---
name: desktop-playbooks
description: "Set up a desktop on this machine from Vibian's playbook gallery (Hyprland and more): pick one with the user, run its short interview, then its scripts, verify and log it. Use when the user wants a desktop, a GUI, a window manager or a 'pretty' setup, asks what desktops there are, or wants to switch, change or remove one. Vibian boots to a terminal until then."
---

# Desktop playbooks

Vibian boots to a terminal. A playbook turns it into a desktop: packages, a login screen, and config files the user can read. Each one lives in `/usr/share/vibian/playbooks/<id>/`:

- `PLAYBOOK.md`: what it is, who it's for, the interview, the knobs, the steps, keys, undo and gotchas.
- `scripts/system.sh`: the root part (packages, login screen). Idempotent.
- `scripts/user.sh`: the user part (dotfiles). Never run as root. It keeps any file it replaces as `<file>.vibian-bak`.
- `scripts/verify.sh`: checks everything and ends with `VERIFY OK` or `VERIFY FAILED`.

The scripts take their choices from `VIBIAN_*` environment variables (the knobs), so the same script serves every answer.

## Picking one
1. `vibian playbooks` lists them with a status. `tested` means Vibian's VM tests run it on every change. `untested` means it was written carefully but nobody has booted it yet, so say so.
2. If the user hasn't said what they want, ask in one message what the computer is for (coding, gaming, a laptop on battery, a media box, a server) and whether they like tiling or floating windows. Then suggest one or two playbooks, saying why in a sentence each.
3. Check the hardware before you promise anything: `vibian doctor` and the `hardware` MCP tool. NVIDIA on Wayland needs the proprietary driver first (`drivers-hardware` skill). A laptop wants idle suspend and brightness keys.

## Running one
1. `vibian playbooks show <id>` and read all of it.
2. Ask the interview questions in **one** message. Offer the defaults so "defaults are fine" is a full answer. Map the answers to the knobs.
3. Follow the playbook's Steps exactly: `vibian snap`, then `system.sh` with root (how depends on `vibian powers`: see AGENTS.md), then `user.sh`, then `verify.sh`.
4. If `verify.sh` fails, fix the cause and run it again. Don't log a desktop that doesn't verify.
5. `vibian log add` with the packages, the files and the undo line from the playbook.
6. Tell the user how to get in (reboot, or log out of tty1), the five keys they need first, and how to undo. Once they're in, check with `vibian-computer` (`computer-use` skill): a screenshot showing the bar and wallpaper is the real test.

## Changing it later
- Re-run `user.sh` with different knobs to change the accent, gaps, terminal or keyboard. It only rewrites its own files.
- The user's own settings go where the playbook says, and re-runs never touch them:
  - Hyprland: `~/.config/hypr/local.conf`.
  - Sway: `~/.config/sway/config.d/`.
  - KDE Plasma: System Settings, or `kwriteconfig6`; never rewrite Plasma's rc files whole.
- Switching desktops: run the new playbook. Its `system.sh` disables the other login screens, so only one is ever enabled. Remove the old desktop only if the user asks, with `vibian undo <ledger-id>` or the playbook's Undo section.

## No playbook fits
Do it by hand the same way: archive packages first, configs in `~/.config` with comments, a check you can run, a ledger entry. If it works well, offer to save it as a playbook (same layout, `status: untested`) under `~/.local/share/vibian/playbooks/<id>/` and point the user at `vibian export` to share it.

## Gotchas
- Don't start a compositor inside the user's SSH or TTY session to "test" it. `verify.sh` checks configs without a screen.
- Don't restart the display manager while the user is logged in graphically without saying so first: it ends their session.
- A desktop's own versions move fast. Prefer the Ubuntu archive; a PPA is opt-in, logged with the reason.
