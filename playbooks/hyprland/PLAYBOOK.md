---
id: hyprland
name: Hyprland
description: A tiling Wayland desktop that animates, with a bar, launcher, notifications, idle lock and a graphical login. Keyboard first, fully scriptable.
status: tested
---

# Hyprland

**What it is:** Hyprland tiles windows automatically: a new window splits the space, and the keyboard moves everything. The playbook sets up a complete desktop:
- waybar for the bar
- fuzzel to launch apps
- mako for notifications
- hypridle and hyprlock to lock the screen when idle
- swaybg for the wallpaper
- greetd and tuigreet for a terminal-style login screen

Every piece is a plain text file the agent can change later.

**Who it's for:** people who live in terminals and editors, want the keyboard to do everything, and like to tweak. It's a poor fit for someone who wants windows to stay where they drop them; offer `kde-plasma` or `gnome` instead, or Hyprland's floating mode (knob `VIBIAN_FLOATING`).

**Needs:** a GPU with working KMS. Intel, AMD and NVIDIA (proprietary driver, 535+) all work; NVIDIA gets its env vars set automatically. A VM needs virtio-gpu or another DRM device.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Tiling, or windows that float like a normal desktop? (`VIBIAN_FLOATING`)
2. Which terminal: foot (light, default), kitty (ligatures, images) or alacritty? (`VIBIAN_TERMINAL`)
3. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
4. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)
5. Any colour they like? (`VIBIAN_ACCENT`). Everything else about the look comes later, as a rice.

## Knobs
Environment variables for `scripts/user.sh` (and `VIBIAN_LOGIN` for `scripts/system.sh`):

| knob | default | meaning |
|---|---|---|
| `VIBIAN_TERMINAL` | `foot` | `foot`, `kitty` or `alacritty` (system.sh installs it too) |
| `VIBIAN_MOD` | `SUPER` | the main modifier |
| `VIBIAN_FLOATING` | `0` | `1`: new windows float; SUPER+T tiles one |
| `VIBIAN_LAYOUT` | `dwindle` | `dwindle` (spiral splits) or `master` (one big window + a stack) |
| `VIBIAN_GAPS` | `6` | outer gap in px; inner is half |
| `VIBIAN_ACCENT` | `7aa2f7` | hex accent: borders, bar highlight, launcher selection |
| `VIBIAN_BG` | `16161e` | hex background colour |
| `VIBIAN_WALLPAPER` | none (the Vibian logo) | absolute path of a wallpaper image, also blurred behind the lock screen |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the lock screen (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |
| `VIBIAN_SCALE` | `auto` | monitor scale (`1`, `1.5`, `2` …) |
| `VIBIAN_LOGIN` | `greetd` | `greetd` (a login screen on VT 7, Ubuntu's usual place) or `tty` (log in on tty1, Hyprland starts from the shell) |

## Steps
1. `vibian snap "hyprland playbook"`.
2. Root part: packages from the Ubuntu archive, and the login screen. With `root` powers run `sudo VIBIAN_TERMINAL=… VIBIAN_LOGIN=… bash scripts/system.sh`. With `scoped` powers, run `vibian sudo --why "install the Hyprland desktop" -- /usr/bin/env VIBIAN_TERMINAL=… bash scripts/system.sh`; the owner approves it once. With `confirm` powers, ask the owner to run the sudo line.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`. It writes `~/.config/{hypr,waybar,fuzzel,mako}` and moves any file it would replace to `<file>.vibian-bak`.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`. It checks the packages, `Hyprland --verify-config`, the waybar JSON and the login setup.
5. `vibian log add "Hyprland desktop (playbook hyprland)" --pkgs … --files ~/.config/hypr,… --undo "…"` (closes the snapshot).
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
| SUPER+L | lock |
| SUPER+SHIFT+S | screenshot a region to the clipboard |
| SUPER+V | clipboard history |
| SUPER+SHIFT+E | log out |
| SUPER+drag, SUPER+right-drag | move, resize |

## Newer Hyprland
The archive's Hyprland (0.53 in 26.04) is what's tested. The `cppiber/hyprland` PPA carries the newest release. Newer versions keep reading `hyprland.conf`, but options and rule syntax move between releases. Add the PPA only if the user asks for a feature the archive lacks, say why in the ledger, and run `verify.sh` straight after.

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Delete `~/.config/{hypr,waybar,fuzzel,mako}` and restore the `.vibian-bak` files.
  - Run `sudo systemctl disable greetd && sudo systemctl set-default multi-user.target`.
  - Remove the packages.

## Gotchas
- **NVIDIA:** user.sh writes `env = LIBVA_DRIVER_NAME,nvidia` and the rest only when `lspci` shows an NVIDIA GPU *and* the proprietary module is loaded. With nouveau, Hyprland runs but is slow; suggest the `drivers-hardware` skill.
- **Black screen after login:** read `~/.local/share/hyprland/hyprland.log` or the `$XDG_RUNTIME_DIR/hypr/*/hyprland.log`. It's almost always a config error (verify.sh catches those) or a missing DRM device.
- **No login screen after reboot:** `systemctl status greetd`. tuigreet's command is in `/etc/greetd/config.toml`.
- **Start it with `start-hyprland`,** not `Hyprland`: since 0.53 the wrapper restarts it safely after a crash, and Hyprland warns without it.
- **A `#` in hyprland.conf starts a comment,** even inside a command. Write colours without it (`swaybg -c 16161e`) or double it (`##`).
- **Don't point greetd at VT 1.** Ubuntu's getty (and Vibian's onboarding) own tty1, and the greeter dies there in a restart loop.
- **In a VM** Hyprland needs a virtio GPU (`-vga none -device virtio-vga` in QEMU). It renders with llvmpipe: fine for a check, slow to use.
