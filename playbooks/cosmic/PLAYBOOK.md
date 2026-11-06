---
id: cosmic
name: COSMIC
description: System76's new Rust desktop from Pop!_OS: a top panel and a dock, floating windows with one-key auto-tiling, a launcher on SUPER, and its own settings app. Wayland only, with the COSMIC login screen. Comes from System76's repo.
status: tested
---

# COSMIC

**What it is:** COSMIC 1.11, the desktop System76 wrote in Rust for Pop!_OS. It has a top panel (workspaces, apps, clock, tray, sound, network, power) and a dock. Tapping SUPER opens a launcher that finds apps, windows and files. Windows float by default. One click on the panel's tiling button, SUPER+Y, or `VIBIAN_TILING=on` switches a workspace to auto-tiling like a tiling window manager's, and SUPER+arrows then move focus between tiles. It has its own apps (COSMIC Terminal, Files, Text Editor, and Store, Media Player, Image Viewer and System Monitor with `standard`) and one Settings app for everything. It runs on Wayland only.

COSMIC isn't in Ubuntu's archive, so the playbook adds System76's Pop!_OS repo for 26.04 and pins it so that Ubuntu's own packages always win (see Gotchas). It sets:
- dark or light, and auto-tiling on or off
- the Vibian wallpaper, on the desktop and on the login screen
- the keyboard, the screen-off and suspend timeouts
- the dock's pinned apps: Files, Terminal, the agent, Text Editor, Settings
- SUPER+Return for a terminal and SUPER+A for the agent
- the COSMIC login screen (cosmic-greeter)

**Who it's for:** people who want a modern, good-looking desktop that tiles when they ask it to, and Pop!_OS users who feel at home in it. In our test VM, with a terminal and the agent open, the whole system used about 1.5 GB of RAM. People who want the most settings and the widest app support are better off with `kde-plasma`. People who want a tiling window manager all the time are better off with `niri`, `hyprland` or `sway`. Old machines are better off with `xfce`.

**Needs:** a GPU with a Wayland driver: Intel, AMD, or NVIDIA's own driver with `nvidia-drm.modeset=1`. It also runs on plain framebuffers with software rendering, as in a VM. About 750 MB of packages with `minimal`.

**Computer use:** `vibian-computer` takes screenshots through COSMIC's portal. The first one makes COSMIC ask the user "Allow Apps to Take Screenshots?". After one Allow the rest are silent. Clicks, typing and keys work too. COSMIC has no window list for other programs, so use screenshots and its keys to switch windows (see the `computer-use` skill).

## Interview
Ask only what the user hasn't already said, in one short message:
1. Dark or light? (`VIBIAN_THEME`)
2. Floating windows, or auto-tiling? (`VIBIAN_TILING`; one key switches later)
3. Just the basics, or COSMIC's Store, Media Player, Image Viewer and System Monitor too? (`VIBIAN_APPS`)
4. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
5. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)

Mention that COSMIC comes from System76's repo, not Ubuntu's.

## Knobs
`VIBIAN_APPS` and `VIBIAN_USER` go to `scripts/system.sh`. The rest go to `scripts/user.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_APPS` | `minimal` | `minimal`: COSMIC, Terminal, Files and Text Editor. `standard`: Store, Media Player, Image Viewer and System Monitor too |
| `VIBIAN_USER` | `$SUDO_USER` | the user whose default login session is set to COSMIC |
| `VIBIAN_THEME` | `dark` | `dark` or `light` |
| `VIBIAN_TILING` | `off` | `on`: new workspaces auto-tile. SUPER+Y flips the current one either way |
| `VIBIAN_WALLPAPER` | `vibian` | `vibian` (the logo on dark blue), `cosmic` (COSMIC's own nebula photo) or an image's full path |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout (add more later in Settings → Keyboard) |
| `VIBIAN_IDLE_SCREEN_OFF` | `10` | minutes before the screen turns off (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |

There's no accent knob: COSMIC derives its whole palette from the accent, so pick one in Settings → Appearance.

## Steps
1. `vibian snap "cosmic playbook"`.
2. Root part: System76's repo, the packages, the login screen and the user's default session. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_APPS=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install the COSMIC desktop" -- /usr/bin/env VIBIAN_APPS=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   system.sh writes `/etc/apt/keyrings/pop-os.asc`, `/etc/apt/sources.list.d/pop-os.sources` and `/etc/apt/preferences.d/pop-os.pref`. It turns off any other login screen (greetd, sddm, gdm3, lightdm) and makes cosmic-greeter the display manager, through a drop-in that gives it the `display-manager.service` alias.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`.
   - COSMIC keeps each setting in its own small file, `~/.config/cosmic/com.system76.<Component>/v1/<key>`, and picks up changes at once. user.sh writes those files.
   - The first run saves `~/.config/cosmic` to `~/.local/share/vibian/cosmic-config.vibian-bak.tar` (an empty archive if there was none).
   - It adds `~/.local/share/applications/vibian-agent.desktop` for the dock.
   - Custom shortcuts and dock pins the user already has stay. user.sh adds its two keys and the agent's pin to them once, and if the user has their own SUPER+Return or SUPER+A, theirs wins.
   - If another playbook installed lxpolkit, it adds `~/.config/autostart/lxpolkit.desktop` to keep lxpolkit out of COSMIC (see Gotchas).
4. `bash scripts/verify.sh`. It must print `VERIFY OK`.
5. `vibian log add "COSMIC desktop (playbook cosmic)" --pkgs … --files ~/.config/cosmic,/etc/apt/sources.list.d/pop-os.sources,/etc/apt/preferences.d/pop-os.pref --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start cosmic-greeter`, which switches the screen to the login. Once they're in, take a screenshot with the `vibian-computer` server (they click Allow once) and check that the panel, dock and wallpaper are there.

## Keys
| keys | does |
|---|---|
| SUPER (tap) | launcher: apps, open windows, files, calculations |
| SUPER+Return | terminal |
| SUPER+A | the agent (`vibian agent` in COSMIC Terminal) |
| SUPER+T | terminal (COSMIC's own key) |
| SUPER+F | files |
| SUPER+Q | close the window |
| SUPER+arrows, SUPER+h/j/k/l | focus left/down/up/right |
| SUPER+SHIFT+arrows | move the window |
| SUPER+Y | auto-tiling on or off for this workspace |
| SUPER+G | float or tile this window |
| SUPER+M, SUPER+F11 | maximise, fullscreen |
| SUPER+1…9, SUPER+SHIFT+1…9 | go to, or move the window to, a workspace |
| SUPER+W | workspaces overview |
| SUPER+Escape | lock |
| Alt+Tab, SUPER+Tab | switch windows |
| Print | screenshot tool |

The app library (upstream's SUPER+A) is the grid button on the dock and the panel's Applications button. Settings → Keyboard → Keyboard shortcuts changes any key, and its custom keys land in `~/.config/cosmic/com.system76.CosmicSettings.Shortcuts/v1/custom`.

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Settings: log out, delete `~/.config/cosmic`, then `tar -xf ~/.local/share/vibian/cosmic-config.vibian-bak.tar -C ~/.config`. Delete `~/.local/share/applications/vibian-agent.desktop` and `~/.config/autostart/lxpolkit.desktop`.
  - Run `sudo systemctl disable cosmic-greeter && sudo systemctl set-default multi-user.target`, then `sudo rm -r /etc/systemd/system/cosmic-greeter.service.d`.
  - Remove the packages, then the repo: `sudo rm /etc/apt/sources.list.d/pop-os.sources /etc/apt/preferences.d/pop-os.pref /etc/apt/keyrings/pop-os.asc && sudo apt update`.

## Gotchas
- **System76's repo, pinned low:** `apt.pop-os.org/release` for `resolute` also carries its own kernel, Mesa, firmware, flatpak, fwupd and more. `pop-os.pref` gives the whole repo priority 100, below Ubuntu's 500, so Ubuntu's version wins wherever both have a package and only what Ubuntu lacks (COSMIC, pop-launcher, its fonts and icons) comes from Pop. Don't raise the pin: that turns the machine into half Pop!_OS. `apt-cache policy <pkg>` shows which repo a package came from.
- **seatd goes away:** `cosmic-session` conflicts with `seatd` (COSMIC uses logind), so apt removes it. None of the other playbooks need it.
- **The login screen is on VT 1**, like GDM's and SDDM's. cosmic-greeter runs greetd with its own config, `/etc/greetd/cosmic-greeter.toml`, separate from the greetd the tiling playbooks use. If the text console is wanted back: `sudo systemctl disable --now cosmic-greeter`.
- **Login screen stuck on an empty screen:** the greeter's session reads `/etc/profile`, so anything in `/etc/profile.d` that waits for input hangs it. Vibian's onboarding hook only runs in interactive shells for that reason. `journalctl -b -u cosmic-greeter` shows what it ran.
- **Two polkit agents:** COSMIC has its own. lxpolkit, which the sway, niri, i3 and xfce playbooks install, autostarts in any desktop it doesn't list, COSMIC included, and the second agent fails with "An authentication agent already exists". user.sh copies lxpolkit's autostart file to `~/.config/autostart/` with COSMIC added to `NotShowIn`. A user created later needs user.sh run again.
- **Wallpapers with transparency** show black where they're transparent. That's why the Vibian wallpaper is the logo already laid on its background colour (`/usr/share/backgrounds/vibian/vibian-wallpaper.png`).
- **Auto-tiling is per workspace:** `VIBIAN_TILING` sets what new workspaces start with. Workspaces that are already open keep their own state until SUPER+Y or the next login.
- **Idle:** COSMIC turns the screen off but doesn't lock on idle. It locks on suspend, on SUPER+Escape and from the power menu.
- **Edit settings with care:** each file under `~/.config/cosmic` is RON, and a typo makes that component fall back to its defaults for that key. Settings shows the result at once, so check there.
- **In a VM** COSMIC works with QEMU's standard VGA or virtio-gpu, rendering in software. With virgl (virtio-vga-gl), QEMU's own screendump can't capture it, but `cosmic-screenshot` inside the guest can.
