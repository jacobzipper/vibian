---
id: kde-plasma
name: KDE Plasma
description: A full, familiar desktop (taskbar, start menu, floating windows, system settings for everything), with the SDDM login screen. The safe pick for people coming from Windows.
status: tested
---

# KDE Plasma

**What it is:** Plasma 6 is KDE's desktop, straight from the Ubuntu archive. It has:
- a taskbar with a start menu, a clock and tray icons for network, sound, battery and Bluetooth
- windows that float and snap to screen edges
- System Settings, which covers nearly everything
- SDDM for the login screen, running its Wayland greeter, so no X server is needed

The playbook installs a small set of apps (Konsole, Dolphin, Spectacle, Ark, Kate) and no full KDE suite. It sets the keyboard, the lock and suspend timeouts, two shortcuts, Breeze dark or light with your accent colour, and the Vibian wallpaper.

**Who it's for:** anyone who wants a normal desktop that works with the mouse: people coming from Windows, shared family computers, and people who change settings by clicking. Keyboard-first users who want tiling are better off with `hyprland` or `sway`. KWin can still tile: drag a window with Shift held to drop it into a tile, or use SUPER+arrows.

**Needs:** any GPU with KMS. Intel, AMD and NVIDIA (proprietary driver 535+) all work on Wayland. A VM needs virtio-gpu or another DRM device. About 1.2 GB of packages with `minimal`, and around 2 GB with `standard`.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Dark or light? (`VIBIAN_THEME`)
2. Just the basics, or the fuller KDE app set (KDE Connect, a document viewer, and Ubuntu's other recommended apps)? (`VIBIAN_APPS`)
3. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
4. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)
5. Any colour they like? (`VIBIAN_ACCENT`)

## Knobs
These are environment variables for `scripts/user.sh`. `VIBIAN_APPS` goes to `scripts/system.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_APPS` | `minimal` | `minimal`: Plasma and five apps, no recommends. `standard`: Ubuntu's recommended set too |
| `VIBIAN_THEME` | `dark` | `dark` (Breeze Dark) or `light` (Breeze) |
| `VIBIAN_ACCENT` | `7aa2f7` | hex accent colour |
| `VIBIAN_BG` | `16161e` | hex colour around the wallpaper |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the lock screen (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |

## Steps
1. `vibian snap "kde-plasma playbook"`.
2. Root part: the packages and the login screen. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_APPS=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install the KDE Plasma desktop" -- /usr/bin/env VIBIAN_APPS=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   system.sh turns off any other login screen (greetd, gdm3, lightdm) and makes SDDM the display manager.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`.
   - It edits `~/.config/{kxkbrc,kscreenlockerrc,powerdevilrc,kglobalshortcutsrc}` with `kwriteconfig6`. Before the first edit, it copies each file to `<file>.vibian-bak`.
   - It also adds `~/.local/share/applications/vibian-agent.desktop` and a run-once login script, `~/.local/share/vibian/kde-apply.sh`. That script applies the theme, accent and wallpaper; it needs Plasma running.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`.
5. `vibian log add "KDE Plasma desktop (playbook kde-plasma)" --pkgs … --files ~/.config/kxkbrc,… --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start sddm`, which switches the screen to the login. Pick the **Plasma (Wayland)** session. Once they're in, take a screenshot with the `vibian-computer` server and check that the panel and wallpaper are there.

## Keys
| keys | does |
|---|---|
| SUPER | start menu (type to search) |
| SUPER+Return, Ctrl+Alt+T | Konsole |
| SUPER+A | the agent (`vibian agent` in Konsole) |
| SUPER+arrows | snap the window to a half, quarter or maximised |
| Shift+drag | drop the window into a tile (SUPER+T edits the tile layout) |
| Alt+Tab | switch windows |
| SUPER+L | lock |
| Print | Spectacle screenshot |
| SUPER+V | clipboard history |
| Ctrl+Alt+Del | log out |

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Restore the `.vibian-bak` files. Delete `~/.local/share/applications/vibian-agent.desktop`, `~/.local/share/vibian/kde-apply.sh`, `~/.local/share/vibian/kde-plasma.applied` and `~/.config/autostart/vibian-kde-apply.desktop`.
  - Run `sudo systemctl disable sddm && sudo systemctl set-default multi-user.target`, then `sudo rm /etc/sddm.conf.d/10-vibian.conf`.
  - Remove the packages.

## Gotchas
- **Change settings in System Settings or with `kwriteconfig6`**, not by rewriting the rc files: Plasma rewrites them while running. To see what a setting is called, change it in the GUI, then `grep -r` the key in `~/.config`.
- **Theme or wallpaper not applied:** the login script runs once, then deletes its autostart entry. Run `~/.local/share/vibian/kde-apply.sh` again from inside Plasma.
- **Shortcuts:** `kglobalshortcutsrc` is read when KWin starts, so new shortcuts work after the next login.
- **Black screen or a bounce back to SDDM:** pick the Wayland session, then read `journalctl --user -b -u plasma-kwin_wayland` and `journalctl -b -u sddm`. To rule out the Wayland greeter, delete `/etc/sddm.conf.d/10-vibian.conf` (SDDM then uses X11).
- **SDDM takes VT 1**, as Ubuntu sets it up, so the text console moves: Ctrl+Alt+F2 gives a login prompt.
- **Switching from Hyprland or Sway:** system.sh disables greetd. The Hyprland and Sway playbooks disable SDDM in the same way, so only one login screen is ever enabled. It also turns off the waybar, mako and hypridle user services that Ubuntu enables for every session, which would otherwise draw a second bar over Plasma.
- **In a VM**, Plasma needs a virtio GPU (`-vga none -device virtio-vga` in QEMU). It renders with llvmpipe, which is slow but usable.
