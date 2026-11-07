---
id: quickshell-desk
name: Quickshell desk
description: Hyprland with a desk built in Quickshell instead of a bar. It has a taskbar and a start menu, plus widgets on the desktop for the clock, asking the agent, the keys, the system and the agent's recent changes. Tiling underneath, familiar on top. Quickshell is built from source.
status: tested
---

# Quickshell desk

**What it is:** the `hyprland` playbook's tiling desktop, with Vibian's own desk on top, written in [Quickshell](https://quickshell.org) (QML). It replaces waybar and the fuzzel launcher with:
- a **taskbar** on every screen. It has the start button (the Vibian logo), workspace buttons, this workspace's windows (click to focus, middle-click to close), the tray, volume (scroll to change), battery and the clock.
- a **start menu** (SUPER+Space or the logo) that finds apps as you type. If you type something that isn't an app, Return asks the agent instead. Lock, log out, restart and shut down are at the bottom; the last three ask "Sure?" first.
- **widgets** on the desktop, under the windows:
  - **Ask your agent:** type a request and press Return. It opens in a terminal so the user can follow it.
  - **Recent changes:** what the agent changed lately, from `vibian history`. Click one to ask the agent what it was and how to undo it.
  - **System:** CPU, memory and disk.
  - **Keys:** the keys to know.
  - a big clock.

Everything else comes from `hyprland`: the tiling, mako, hypridle and hyprlock, swaybg, and greetd with tuigreet. The desk's QML is plain text in `~/.config/quickshell/vibian-desk/` that the agent can change, and Quickshell reloads it as soon as a file is saved.

It was inspired by the desk the agent made for its owner on the laptop where Vibian started.

**Who it's for:** people who want tiling but also a taskbar, a start menu and the agent one box away, and anyone who wants a desk they can reshape by asking for it. Quickshell can draw nearly anything, so this is the playbook to start from for a desk that looks like nobody else's. People who want plain tiling are better off with `hyprland`. People who want a classic desktop that never tiles are better off with `kde-plasma` or `xfce`.

**Needs:** what `hyprland` needs (a GPU with working KMS), plus 15 to 30 minutes to build Quickshell on a 4-core machine with 4 GB of RAM. The build is a one-off, and its compilers and headers are removed afterwards. The desk uses 200 to 400 MB of RAM (the high end with software rendering).

## Interview
Ask only what the user hasn't already said, in one short message:
1. All the widgets, or fewer? Clock, Ask your agent, Keys, System and Recent changes. (`VIBIAN_WIDGETS`)
2. Tiling, or windows that float like a normal desktop? (`VIBIAN_FLOATING`)
3. Which terminal: foot (light, default), kitty or alacritty? (`VIBIAN_TERMINAL`)
4. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
5. Any colour they like? (`VIBIAN_ACCENT`)

Mention that Quickshell is built from source, so the first run takes a while.

## Knobs
Every knob of the `hyprland` playbook works the same here (`VIBIAN_MOD`, `VIBIAN_LAYOUT`, `VIBIAN_GAPS`, `VIBIAN_BG`, `VIBIAN_WALLPAPER`, `VIBIAN_IDLE_LOCK`, `VIBIAN_IDLE_SUSPEND`, `VIBIAN_SCALE`, `VIBIAN_LOGIN`…). `VIBIAN_TERMINAL`, `VIBIAN_LOGIN` and `VIBIAN_KEEP_BUILD_DEPS` go to `scripts/system.sh`; the rest go to `scripts/user.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_WIDGETS` | `clock,ask,keys,system,changes` | the desktop widgets to show, or `none` |
| `VIBIAN_ACCENT` | `7aa2f7` | hex accent: borders, the desk's highlights |
| `VIBIAN_BG` | `16161e` | hex background colour, for the desk too |
| `VIBIAN_TERMINAL` | `foot` | where the agent opens: `foot`, `kitty` or `alacritty` |
| `VIBIAN_FLOATING` | `0` | `1`: new windows float; SUPER+T tiles one |
| `VIBIAN_KEEP_BUILD_DEPS` | `0` | `1`: keep the compilers and Qt headers after the build |

The desk's settings end up in `~/.config/quickshell/vibian-desk/desk.json` (`accent`, `background`, `terminal`, `mod`, `widgets`). Editing that file changes the desk at once, with no rerun.

The look is in `theme.json` next to it, which user.sh creates empty and never overwrites, so themes and rices put theirs there. Every key is optional: `accent`, `background`, `fg`, `dim`, `faint`, `red`, `yellow` and `green` (6 hex digits; `accent` and `background` here beat desk.json's), `font`, `mono` and `display` (font families for text, keys, and the clock and card titles), `clock` (the big clock's font, if not `display`'s), and `radius` (the cards' corner radius in px, 10 by default). For example:
```json
{ "accent": "ff79b0", "background": "16121f", "fg": "ece1f0", "display": "Dela Gothic One", "radius": 4 }
```
A wallpaper comes from the `hyprland` knob `VIBIAN_WALLPAPER`.

## Steps
1. `vibian snap "quickshell desk playbook"`.
2. Root part: the `hyprland` packages and login screen, then Quickshell. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_TERMINAL=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install Hyprland and build the Quickshell desk" -- /usr/bin/env VIBIAN_TERMINAL=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   system.sh runs `../hyprland/scripts/system.sh` first. Then it downloads Quickshell 0.3.2's source, checks it against a pinned sha256, builds it against Ubuntu's Qt 6.10 and installs it to `/usr/local` (`quickshell`, and `qs` for short). Then it removes the build packages it added, keeping the libraries Quickshell loads. Tell the user it takes a while, and keep them posted.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`. It runs the `hyprland` user.sh with the desk in place of waybar and fuzzel, then copies the desk to `~/.config/quickshell/vibian-desk/` and writes `desk.json`.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`. It runs the `hyprland` checks, then checks the Quickshell build, its libraries, the Hyprland lines that start the desk, and `desk.json`.
5. `vibian log add "Quickshell desk (playbook quickshell-desk)" --pkgs … --files ~/.config/hypr,~/.config/quickshell/vibian-desk,/usr/local/bin/quickshell --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start greetd`, which switches the screen to the login. Once they're in, take a screenshot with the `vibian-computer` server and check that the taskbar and widgets are there. If the desk is missing, run `qs -c vibian-desk` in a terminal inside the session: it prints the QML error.

## Keys (with the default `SUPER`)
The `hyprland` keys, with SUPER+Space opening the start menu:

| keys | does |
|---|---|
| SUPER+Space | start menu: type an app, or a request for the agent |
| SUPER+A | the agent (`vibian agent` in a terminal) |
| SUPER+Return | terminal |
| SUPER+Q | close window |
| SUPER+F / SUPER+T | fullscreen / float or tile this window |
| SUPER+arrows, SUPER+SHIFT+arrows | move focus, move window |
| SUPER+1…9, SUPER+SHIFT+1…9 | go to workspace, send window there |
| SUPER+V | clipboard history |
| SUPER+SHIFT+S | screenshot a region to the clipboard |
| SUPER+L | lock |
| SUPER+SHIFT+E | log out |

`qs -c vibian-desk ipc call desk start` toggles the start menu, and `qs -c vibian-desk ipc call desk ask "…"` asks the agent, for binding to other keys in `~/.config/hypr/local.conf`.

## Changing the desk
- Small things, like colours, widgets or the terminal: edit `desk.json`.
- Anything else: edit the QML. `shell.qml` lists the pieces, `Place.qml` positions the widgets, and `Desk.qml` holds the colours and helpers. Quickshell reloads on save. A broken file leaves the last working desk up and logs the error (`qs -c vibian-desk log`).
- A rerun of user.sh rewrites the `.qml` files. To keep big changes, copy the folder to `~/.config/quickshell/my-desk/`, then put `exec-once = qs -c my-desk` in `local.conf` and remove the `vibian-desk` line from `hyprland.conf`.

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Delete `~/.config/quickshell/vibian-desk` (and restore `vibian-desk.vibian-bak` if there is one).
  - Rerun the `hyprland` playbook's user.sh to get waybar and fuzzel back.
  - Run `sudo rm /usr/local/bin/quickshell /usr/local/bin/qs`.
  - Follow the `hyprland` playbook's Undo for the rest.

## Gotchas
- **Quickshell uses Qt's private APIs.** It must be rebuilt whenever Ubuntu updates Qt to a new minor version (6.10 → 6.11), or it crashes at start. If the desk vanishes after a big upgrade, run system.sh again. It rebuilds only if the installed version differs from the pinned one, so first run `sudo rm /usr/local/bin/quickshell`.
- **No crash reporter:** it needs cpptrace, which Ubuntu doesn't package, so the build turns it off.
- **Widgets sit under the windows** (the layer-shell bottom layer). A tiled window covers them; SUPER+F on an empty workspace, or another workspace, shows them again. On a screen narrower than about 1100 px only the clock and the Ask box show.
- **Typing in the Ask box** needs a click in it first: desktop widgets only take the keyboard on demand.
- **Recent changes** reads `vibian history`, so it shows what the ledger and snapper recorded. If it's empty, nothing has been logged yet.
- **waybar's config is still written** (by the hyprland user.sh), just not started. Switching back to plain Hyprland is one rerun of its user.sh.
- **In a VM** Hyprland and Quickshell need a virtio GPU and render with llvmpipe: fine for a check.
