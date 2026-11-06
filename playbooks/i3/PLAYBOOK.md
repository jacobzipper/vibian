---
id: i3
name: i3
description: The classic keyboard tiling window manager, on X11. Light, predictable and decades-stable, with a bar, launcher, notifications, idle lock and the LightDM login screen. Runs on old GPUs and anything Wayland struggles with.
status: tested
---

# i3

**What it is:** i3 is the tiling window manager that Sway copied. Windows tile in rows and columns you control, workspaces switch instantly, and the config is one readable text file. It runs on X11, not Wayland, so it works where Wayland doesn't: old or odd GPUs, remote desktop tools, screen recorders and games that expect X11, and NVIDIA cards on drivers older than 535.

The playbook sets up a complete desktop from the Ubuntu archive:
- i3bar with i3status for the bar
- rofi to launch apps and switch windows
- dunst for notifications
- picom for tear-free drawing and rounded corners
- xss-lock and i3lock to lock the screen when idle, plus xautolock to suspend
- autotiling, so new windows split the longer side (optional)
- LightDM for the login screen

Every piece is a plain text file the agent can change later.

**Who it's for:** i3 users, people on old hardware, and anyone whose apps or GPU need X11. Someone who wants Wayland's smoothness and per-monitor scaling is better off with `sway`, which uses the same keys. Someone who wants floating windows is better off with `xfce` or `kde-plasma`.

**Needs:** any GPU X11 drives. That includes the generic modesetting driver and VESA, so even very old machines work. About 250 MB of packages.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Tiling, or windows that float like a normal desktop? (`VIBIAN_FLOATING`)
2. Which terminal: alacritty (default), kitty, or plain xterm? (`VIBIAN_TERMINAL`)
3. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
4. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)
5. Any colour they like? (`VIBIAN_ACCENT`). Everything else about the look comes later, as a rice.

## Knobs
`VIBIAN_TERMINAL`, `VIBIAN_LOGIN` and `VIBIAN_USER` go to `scripts/system.sh`. The others go to `scripts/user.sh` (`VIBIAN_TERMINAL` and `VIBIAN_LOGIN` too).

| knob | default | meaning |
|---|---|---|
| `VIBIAN_TERMINAL` | `alacritty` | `alacritty`, `kitty` or `xterm`. foot is Wayland-only |
| `VIBIAN_LOGIN` | `lightdm` | `lightdm` (a login screen on VT 7) or `tty` (log in on tty1, and `startx` runs i3) |
| `VIBIAN_USER` | `$SUDO_USER` | the user whose default login session is set to i3 |
| `VIBIAN_MOD` | `SUPER` | the main modifier: `SUPER` or `ALT` |
| `VIBIAN_FLOATING` | `0` | `1`: new windows float; SUPER+T tiles one |
| `VIBIAN_AUTOTILING` | `1` | `1`: alternate splits automatically; `0`: plain i3 behaviour (SUPER+B / SUPER+N pick the split) |
| `VIBIAN_GAPS` | `6` | outer gap in px; inner is half |
| `VIBIAN_ACCENT` | `7aa2f7` | hex accent: borders, bar highlight, launcher selection |
| `VIBIAN_BG` | `16161e` | hex background colour |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the lock screen (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |

## Steps
1. `vibian snap "i3 playbook"`.
2. Root part: packages from the Ubuntu archive, and the login screen. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_TERMINAL=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install the i3 desktop" -- /usr/bin/env VIBIAN_TERMINAL=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   system.sh turns off any other login screen (greetd, sddm, gdm3) and makes LightDM the display manager.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`. It writes `~/.config/{i3,i3status,rofi,dunst,picom}` and moves any file it would replace to `<file>.vibian-bak`.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`. It checks the packages, `i3 -C` (a config check that needs no screen), the other configs and the login setup.
5. `vibian log add "i3 desktop (playbook i3)" --pkgs … --files ~/.config/i3,… --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start lightdm`, which switches the screen to the login. Once they're in, take a screenshot with the `vibian-computer` server and check that the bar and wallpaper are there.

## Keys (with the default `SUPER`)
| keys | does |
|---|---|
| SUPER+Return | terminal |
| SUPER+A | the agent (`vibian agent` in a terminal) |
| SUPER+Space | app launcher |
| SUPER+Tab | switch to any window by name |
| SUPER+Q | close window |
| SUPER+F / SUPER+T | fullscreen / float or tile this window |
| SUPER+arrows, SUPER+SHIFT+arrows | move focus, move window |
| SUPER+1…9, SUPER+SHIFT+1…9 | go to workspace, send window there |
| SUPER+W / SUPER+E | tabbed / split layout for this container |
| SUPER+B / SUPER+N | next window splits horizontally / vertically |
| SUPER+R | resize mode (arrows, then Escape) |
| SUPER+L | lock |
| SUPER+SHIFT+S | screenshot a region to the clipboard |
| Print | screenshot the screen to `~/Pictures/Screenshots` |
| SUPER+SHIFT+C / SUPER+SHIFT+R | reload the config / restart i3 in place |
| SUPER+SHIFT+E | log out |
| SUPER+drag, SUPER+right-drag | move, resize a floating window |

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Delete `~/.config/{i3,i3status,rofi,dunst,picom}` and restore the `.vibian-bak` files.
  - Run `sudo systemctl disable lightdm && sudo systemctl set-default multi-user.target`, then `sudo rm /etc/lightdm/lightdm.conf.d/50-vibian.conf`.
  - Remove the packages.

## Gotchas
- **Your own settings go in `~/.config/i3/config.d/`.** The main config includes every file there, and re-running the playbook never touches them.
- **Coming from Sway:** the keys are the same, and Sway's config mostly works in i3. The differences are `exec --no-startup-id`, `xrandr` for monitors (or `arandr`, which is installed) and `setxkbmap` for the keyboard.
- **Monitors and scaling:** set them with `arandr` and save the `xrandr` line it shows into `config.d/`. X11 has one scale for every screen: for HiDPI, put `Xft.dpi: 192` in `~/.Xresources`.
- **Tearing or a laggy terminal:** picom draws the screen. Try `backend = "glx"` in `~/.config/picom/picom.conf`, or remove the `picom` line from the config to test without it.
- **No login screen after reboot:** `systemctl status lightdm`, then `/var/log/lightdm/`. LightDM is on VT 7, and the text console stays on tty1.
- **Screen sharing and screenshots work everywhere:** X11 lets any program read the screen. That's also why `vibian-computer` works fully here. But it means any app you run can see every window, so prefer Wayland desktops on shared machines.
- **In a VM** X11 works with QEMU's standard VGA or virtio-gpu.
