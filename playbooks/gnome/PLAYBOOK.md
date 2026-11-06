---
id: gnome
name: GNOME
description: The calm, polished desktop Ubuntu ships: one top bar, an overview for windows and apps, and few settings to fuss with. Upstream GNOME 50 or Ubuntu's version with the dock, plus the GDM login screen.
status: tested
---

# GNOME

**What it is:** GNOME 50, straight from the Ubuntu archive. There's one top bar. The SUPER key opens the overview, which shows your windows, workspaces and an app search. Settings is simple and covers the common things. You pick one of two styles:
- `vanilla`: upstream GNOME (Adwaita, no dock). It looks the way GNOME's own docs and screenshots do.
- `ubuntu`: Ubuntu's session with the dock on the left, tray icons and the Yaru theme. It's what stock Ubuntu Desktop looks like.

The playbook installs GDM for the login screen and a small app set: Ptyxis (terminal), Files, Text Editor, Image Viewer, System Monitor and Disks. It sets dark or light, your accent, the Vibian wallpaper, the keyboard, the lock and suspend timeouts, and two shortcuts.

**Who it's for:** people who want a clean desktop that stays out of the way and mostly works without setup: Mac users, Ubuntu Desktop users, and anyone who would rather not choose between hundreds of settings. People who want to tweak everything are better off with `kde-plasma`. Keyboard-tiling fans are better off with `hyprland`, `sway` or `niri`.

**Needs:** any GPU with KMS. Intel, AMD and NVIDIA (proprietary driver 535+) all run Wayland. A VM needs a virtual GPU (virtio-gpu or QEMU's standard VGA). About 1 GB of packages with `minimal`.

**Computer use:** GNOME doesn't let other programs take silent screenshots or list windows, so `vibian-computer` only works partly here (see the `computer-use` skill). Tell the user before they pick GNOME if they want the agent to drive their desktop.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Plain GNOME, or Ubuntu's version with the dock on the left? (`VIBIAN_STYLE`)
2. Dark or light? (`VIBIAN_THEME`)
3. Accent colour: blue, teal, green, yellow, orange, red, pink, purple, slate or brown? (`VIBIAN_ACCENT`)
4. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
5. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)

## Knobs
`VIBIAN_STYLE`, `VIBIAN_APPS` and `VIBIAN_USER` go to `scripts/system.sh`. The rest go to `scripts/user.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_STYLE` | `vanilla` | `vanilla` (upstream GNOME) or `ubuntu` (dock, tray icons, Yaru) |
| `VIBIAN_APPS` | `minimal` | `minimal`: GNOME and six apps, no recommends. `standard`: recommends, plus Tweaks, Extension Manager, Calculator and Software |
| `VIBIAN_USER` | `$SUDO_USER` | the user whose default login session is set to this one |
| `VIBIAN_THEME` | `dark` | `dark` or `light` |
| `VIBIAN_ACCENT` | `blue` | one of GNOME's accents: blue, teal, green, yellow, orange, red, pink, purple, slate, brown |
| `VIBIAN_BG` | `16161e` | hex colour around the wallpaper |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout (add more later in Settings → Keyboard) |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the screen blanks and locks (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |

## Steps
1. `vibian snap "gnome playbook"`.
2. Root part: the packages, the login screen, and the user's default session. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_STYLE=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install the GNOME desktop" -- /usr/bin/env VIBIAN_STYLE=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   system.sh turns off any other login screen (greetd, sddm, lightdm) and makes GDM the display manager.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`. It writes its settings to `~/.local/share/vibian/gnome.dconf.ini` and loads them with `dconf load /`. The first run saves the old settings to `~/.local/share/vibian/gnome-dconf.vibian-bak.ini`. It works over SSH: it starts its own D-Bus session when there is none.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`.
5. `vibian log add "GNOME desktop (playbook gnome)" --pkgs … --files ~/.local/share/vibian/gnome.dconf.ini --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start gdm3`, which switches the screen to the login. Once they're in, ask them to check that the top bar and wallpaper are there. A `vibian-computer` screenshot may not work on GNOME.

## Keys
| keys | does |
|---|---|
| SUPER | the overview: windows, workspaces, type to search apps |
| SUPER+Return | terminal (Ptyxis) |
| SUPER+A | the agent (`vibian agent` in Ptyxis). It replaces GNOME's "show apps" key; SUPER then type still finds apps |
| SUPER+Left / SUPER+Right | tile the window to half the screen |
| SUPER+Up | maximise |
| SUPER+Page Up / Page Down | switch workspace |
| Alt+Tab | switch apps |
| SUPER+L | lock |
| Print | screenshot tool |
| SUPER+V | notifications and calendar |

## Undo
- `vibian undo <ledger-id>` puts back the packages from the snapshot pair.
- Settings: `dconf load / < ~/.local/share/vibian/gnome-dconf.vibian-bak.ini` restores what you had before. Keys that were unset then stay at the playbook's values. To return them to GNOME's defaults, run `dconf reset -f /org/gnome/` and `dconf reset -f /org/gnome/settings-daemon/plugins/media-keys/`.
- Login screen: `sudo systemctl disable gdm3 && sudo systemctl set-default multi-user.target`.
- Remove the packages.

## Gotchas
- **Change settings with `gsettings` or Settings**, not by editing files: GNOME keeps them in a binary dconf database. To find a key, run `dconf watch /` while you flip it in Settings.
- **Extensions** are where most GNOME breakage comes from after an upgrade. With `standard`, Extension Manager installs them from extensions.gnome.org. Log what you add so you know what to disable when the shell misbehaves (`gnome-extensions disable <uuid>`).
- **GDM takes VT 1**, like SDDM: the text console moves to Ctrl+Alt+F3.
- **GDM falls back to X11** on a few GPUs and VMs; its udev rules decide. That's fine: the GNOME session runs on X11 too. `echo $XDG_SESSION_TYPE` shows which one is in use.
- **Switching from another desktop:** system.sh disables greetd and SDDM, and the other playbooks disable GDM in the same way. It also turns off the waybar, mako and hypridle user services that Ubuntu enables for every session.
- **NVIDIA:** GDM uses Wayland on the proprietary driver only when `nvidia-drm.modeset=1` is set, which is the default since driver 545. Older drivers fall back to X11.
