---
id: sway
name: Sway
description: A calm, stable tiling Wayland desktop, i3-compatible, with a bar, launcher, notifications, idle lock and a graphical login. No animations, very little to break.
status: tested
---

# Sway

**What it is:** Sway is the i3 window manager rebuilt for Wayland. Windows tile in rows and columns you control, and workspaces switch instantly. It changes slowly and rarely breaks on upgrade. The playbook sets up a complete desktop:
- waybar for the bar
- fuzzel to launch apps
- mako for notifications
- swayidle and swaylock to lock the screen when idle
- autotiling, so new windows split the longer side like Hyprland's spiral (optional)
- greetd and tuigreet for a terminal-style login screen

Every piece is a plain text file the agent can change later.

**Who it's for:** keyboard people who want a tiling desktop that stays out of the way: i3 users moving to Wayland, older or low-power machines, anyone who found Hyprland too flashy or too fast-moving. Someone who wants windows to stay where they drop them is better off with `kde-plasma` or `gnome`.

**Needs:** a GPU with working KMS. Intel and AMD just work. NVIDIA needs the proprietary driver (535+); system.sh then starts Sway with `--unsupported-gpu`. A VM needs virtio-gpu or another DRM device.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Tiling, or windows that float like a normal desktop? (`VIBIAN_FLOATING`)
2. Which terminal: foot (light, default), kitty (ligatures, images) or alacritty? (`VIBIAN_TERMINAL`)
3. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
4. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)
5. Any colour they like? (`VIBIAN_ACCENT`). Everything else about the look comes later, as a rice.

## Knobs
Environment variables for `scripts/user.sh` (and `VIBIAN_TERMINAL`, `VIBIAN_LOGIN` for `scripts/system.sh`):

| knob | default | meaning |
|---|---|---|
| `VIBIAN_TERMINAL` | `foot` | `foot`, `kitty` or `alacritty` (system.sh installs it too) |
| `VIBIAN_MOD` | `SUPER` | the main modifier: `SUPER` or `ALT` |
| `VIBIAN_FLOATING` | `0` | `1`: new windows float; SUPER+T tiles one |
| `VIBIAN_AUTOTILING` | `1` | `1`: alternate splits automatically; `0`: plain i3 behaviour (SUPER+B / SUPER+N pick the split) |
| `VIBIAN_GAPS` | `6` | outer gap in px; inner is half |
| `VIBIAN_ACCENT` | `7aa2f7` | hex accent: borders, bar highlight, launcher selection |
| `VIBIAN_BG` | `16161e` | hex background colour |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the lock screen (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |
| `VIBIAN_SCALE` | `1` | output scale (`1`, `1.5`, `2` …) |
| `VIBIAN_LOGIN` | `greetd` | `greetd` (a login screen on VT 7, Ubuntu's usual place) or `tty` (log in on tty1, Sway starts from the shell) |

## Steps
1. `vibian snap "sway playbook"`.
2. Root part: packages from the Ubuntu archive, and the login screen. With `root` powers run `sudo VIBIAN_TERMINAL=… VIBIAN_LOGIN=… bash scripts/system.sh`. With `scoped` powers, run `vibian sudo --why "install the Sway desktop" -- /usr/bin/env VIBIAN_TERMINAL=… bash scripts/system.sh`; the owner approves it once. With `confirm` powers, ask the owner to run the sudo line.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`. It writes `~/.config/{sway,waybar,fuzzel,mako}` and moves any file it would replace to `<file>.vibian-bak`.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`. It checks the packages, `sway --validate` (headless, so it works over SSH), the waybar JSON and the login setup.
5. `vibian log add "Sway desktop (playbook sway)" --pkgs … --files ~/.config/sway,… --undo "…"` (closes the snapshot).
6. Tell the user to reboot (or `sudo systemctl start greetd`, which switches the screen to the login). Once they're in, take a screenshot with the `vibian-computer` server and check the bar and wallpaper are there.

## Keys (with the default `SUPER`)
| keys | does |
|---|---|
| SUPER+Return | terminal |
| SUPER+A | the agent (`vibian agent` in a terminal) |
| SUPER+Space | app launcher |
| SUPER+Q | close window |
| SUPER+F / SUPER+T | fullscreen / float or tile this window |
| SUPER+arrows, SUPER+SHIFT+arrows | move focus, move window |
| SUPER+1…9, SUPER+SHIFT+1…9 | go to workspace, send window there |
| SUPER+W / SUPER+E | tabbed / split layout for this container |
| SUPER+B / SUPER+N | next window splits horizontally / vertically |
| SUPER+R | resize mode (arrows, then Escape) |
| SUPER+L | lock |
| SUPER+SHIFT+S | screenshot a region to the clipboard |
| SUPER+V | clipboard history |
| SUPER+SHIFT+C | reload the config |
| SUPER+SHIFT+E | log out |
| SUPER+drag, SUPER+right-drag | move, resize a floating window |

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Delete `~/.config/{sway,waybar,fuzzel,mako}` and restore the `.vibian-bak` files.
  - Run `sudo systemctl disable greetd && sudo systemctl set-default multi-user.target`.
  - Remove the packages.

## Gotchas
- **Your own settings go in `~/.config/sway/config.d/`.** The main config includes every file there, and re-running the playbook never touches them.
- **NVIDIA:** Sway refuses to start on the proprietary driver without `--unsupported-gpu`. system.sh adds it to the login command when the `nvidia` module is loaded. With nouveau it starts but is slow; suggest the `drivers-hardware` skill.
- **Black screen after login:** run `sway --validate` (verify.sh does), then read `journalctl --user -b` or start `sway -d 2>~/sway.log` from a TTY.
- **No login screen after reboot:** `systemctl status greetd`. tuigreet's command is in `/etc/greetd/config.toml`. Don't point greetd at VT 1: the console and Vibian's onboarding live there.
- **Screen sharing** in browsers goes through `xdg-desktop-portal-wlr`, which system.sh installs. Sway's config exports the session to it on start.
- **In a VM** Sway needs a virtio GPU (`-vga none -device virtio-vga` in QEMU). It renders with llvmpipe: fine for a check, slow to use.
