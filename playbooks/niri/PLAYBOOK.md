---
id: niri
name: niri
description: A scrolling tiling Wayland desktop. Windows sit side by side on an endless strip you scroll through, so opening one never squeezes the others. Built from the pinned upstream release, with a bar, launcher, notifications, idle lock and a graphical login.
status: tested
---

# niri

**What it is:** niri is a scrollable-tiling Wayland compositor. Each workspace is a strip of columns that runs off the right edge of the screen. A new window opens as a new column at its usual size, and the others keep theirs: you scroll the strip instead of shrinking everything. Workspaces stack vertically, per monitor. SUPER+O zooms out to an overview of all of them. Animations are smooth but brief, and the config reloads the moment you save it.

The playbook sets up a complete desktop:
- niri 26.04 and xwayland-satellite 0.8.3 (X11 apps), built from source (see below)
- waybar for the bar, fuzzel to launch apps, mako for notifications
- swayidle and swaylock to lock the screen when idle
- greetd and tuigreet for a terminal-style login screen

Every piece is a plain text file the agent can change later.

**Built from source:** niri isn't in Ubuntu 26.04's archive (it arrives in 26.10). system.sh downloads the upstream v26.04 source and its vendored Rust crates and checks both against sha256 hashes pinned in the script. It then builds offline with Ubuntu's own `rustc`. xwayland-satellite is fetched the same way and built with `cargo --locked`. The binaries go to `/usr/local`. Afterwards, the compilers and `-dev` packages it installed are removed again (`VIBIAN_KEEP_BUILD_DEPS=1` keeps them). The build takes 5 to 20 minutes and needs about 3 GB of free disk while it runs. It uses fewer jobs on machines with little RAM. No PPA or third-party repository is added. Updates come from rerunning the playbook after the pinned version in system.sh is bumped.

**Who it's for:** keyboard people who like tiling but hate how it squeezes windows, people with wide or ultrawide screens, and Hyprland or Sway users who want something calmer to use. Someone who wants windows to stay where they drop them is better off with `kde-plasma` or `gnome`.

**Needs:** a GPU with working KMS. Intel and AMD just work, and so does NVIDIA's proprietary driver (535+). niri refuses software rendering, so a VM needs 3D acceleration (virgl), not just a virtual GPU. The build needs network access to github.com and crates.io.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Which terminal: foot (light, default), kitty (ligatures, images) or alacritty? (`VIBIAN_TERMINAL`)
2. How wide should new windows open: a third, half (default), two thirds or the full screen? (`VIBIAN_COLUMN_WIDTH`)
3. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
4. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)
5. Any colour they like? (`VIBIAN_ACCENT`). Everything else about the look comes later, as a rice.

Tell them the first run compiles niri, so it takes a while.

## Knobs
`VIBIAN_TERMINAL`, `VIBIAN_LOGIN`, `VIBIAN_XWAYLAND` and `VIBIAN_KEEP_BUILD_DEPS` go to `scripts/system.sh`. The others go to `scripts/user.sh` (`VIBIAN_TERMINAL` and `VIBIAN_LOGIN` too).

| knob | default | meaning |
|---|---|---|
| `VIBIAN_TERMINAL` | `foot` | `foot`, `kitty` or `alacritty` |
| `VIBIAN_LOGIN` | `greetd` | `greetd` (a login screen on VT 7, Ubuntu's usual place) or `tty` (log in on tty1 and niri starts from the shell) |
| `VIBIAN_XWAYLAND` | `1` | `1`: build xwayland-satellite, so X11 apps (Steam, older Java and Electron apps) run. `0`: Wayland apps only |
| `VIBIAN_KEEP_BUILD_DEPS` | `0` | `1`: keep Rust and the `-dev` packages after the build |
| `VIBIAN_MOD` | `SUPER` | the main modifier: `SUPER` or `ALT` |
| `VIBIAN_COLUMN_WIDTH` | `0.5` | width of a new window, as a fraction of the screen: `0.33333`, `0.5`, `0.66667` or `1.0` |
| `VIBIAN_GAPS` | `8` | gap between windows in px |
| `VIBIAN_ACCENT` | `7aa2f7` | hex accent: focus ring, bar highlight, launcher selection |
| `VIBIAN_BG` | `16161e` | hex background colour |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the lock screen (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never) |

## Steps
1. `vibian snap "niri playbook"`.
2. Root part: the build, the packages and the login screen. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_TERMINAL=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "build and install the niri desktop" -- /usr/bin/env VIBIAN_TERMINAL=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   It runs for a long time, so run it the way long jobs run (a managed job, or tell the user it will take a while). A rerun skips the build when the pinned version is already installed. system.sh turns off any other login screen (sddm, gdm3, lightdm).
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`. It writes `~/.config/{niri,waybar,fuzzel,mako}` and moves any file it would replace to `<file>.vibian-bak`. It also creates `~/.config/niri/local.kdl` once and never touches it again.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`. It checks the build, the session files, `niri validate`, the waybar JSON and the login setup.
5. `vibian log add "niri desktop (playbook niri)" --pkgs … --files /usr/local/bin/niri,/usr/local/bin/niri-session,/usr/local/bin/xwayland-satellite,~/.config/niri,… --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start greetd`, which switches the screen to the login. Once they're in, take a screenshot with the `vibian-computer` server and check that the bar and wallpaper are there.

## Keys (with the default `SUPER`)
| keys | does |
|---|---|
| SUPER+Return | terminal |
| SUPER+A | the agent (`vibian agent` in a terminal) |
| SUPER+Space | app launcher |
| SUPER+Q | close window |
| SUPER+O | overview: every workspace, zoomed out |
| SUPER+Left / Right | scroll to the column on the left / right |
| SUPER+Up / Down | window above / below, then the workspace above / below |
| SUPER+SHIFT+arrows | move the column, or the window up or down |
| SUPER+Page Up / Page Down, SUPER+wheel | workspace up / down |
| SUPER+1…9, SUPER+SHIFT+1…9 | go to workspace, send the column there |
| SUPER+R | cycle the column width: a third, half, two thirds |
| SUPER+Minus / SUPER+Equal | column narrower / wider |
| SUPER+F / SUPER+SHIFT+F | maximise the column / fullscreen |
| SUPER+C | centre the column |
| SUPER+[ / SUPER+] | pull the window into the column on the left / right, or push it out |
| SUPER+W | show the column's windows as tabs |
| SUPER+T | float or tile this window |
| SUPER+L | lock |
| Print, SUPER+SHIFT+S | screenshot: pick a region, Enter to copy and save |
| SUPER+V | clipboard history |
| SUPER+SHIFT+/ | every key, on screen |
| SUPER+SHIFT+E | log out |

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Delete `~/.config/{niri,waybar,fuzzel,mako}` and restore the `.vibian-bak` files.
  - Run `sudo systemctl disable greetd && sudo systemctl set-default multi-user.target`.
  - Run `sudo rm /usr/local/bin/{niri,niri-session,xwayland-satellite} /usr/local/lib/systemd/user/{niri.service,niri-shutdown.target} /usr/local/share/wayland-sessions/niri.desktop /etc/xdg/xdg-desktop-portal/niri-portals.conf`.
  - Remove the packages.

## Gotchas
- **Your own settings go in `~/.config/niri/local.kdl`.** It's included at the end of the main config, so anything there wins, and re-running the playbook never touches it.
- **Config mistakes don't break the session:** niri keeps the last good config and shows an error banner. `niri validate` prints the exact line. Run it after every edit.
- **Monitor scale and layout:** niri picks a scale itself. To change it, run `niri msg outputs` for the names, then add an `output "eDP-1" { scale 1.5; }` block to `local.kdl`.
- **X11 apps** start xwayland-satellite on demand: niri sets `DISPLAY` and launches it when the first X11 client connects. With `VIBIAN_XWAYLAND=0` they simply fail to open.
- **The build failed:** it is all in system.sh's output. Usually that means no network, a full disk, or the machine ran out of memory. On 4 GB or less, close other apps and rerun it; it uses fewer jobs automatically. Nothing was installed if the build failed.
- **Upgrading:** bump `NIRI_VER` and the two hashes at the top of system.sh, then rerun it. Once Ubuntu ships niri (26.10), the playbook can switch to the archive package.
- **Screen sharing** in browsers goes through `xdg-desktop-portal-gnome`, which niri supports. The portal choice is in `/etc/xdg/xdg-desktop-portal/niri-portals.conf`.
- **Black screen after login:** read `journalctl --user -b -u niri`. To rule out the login screen, log in on a text console and run `niri-session`.
- **In a VM** niri needs 3D acceleration: it skips software renderers such as llvmpipe and leaves the screen black (`journalctl --user -u niri` says "software EGL renderers are skipped"). In QEMU use `-device virtio-vga-gl -display gtk,gl=on` (or `-display egl-headless`); in virt-manager, turn on 3D acceleration for the virtio video device. tuigreet still shows without it, since it runs on the text console.
