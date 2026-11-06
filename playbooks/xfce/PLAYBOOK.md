---
id: xfce
name: XFCE
description: A light, traditional desktop on X11: one panel with a menu, window buttons, tray and clock, floating windows, and a settings manager for everything. LightDM login screen. Fast on old or low-memory machines.
status: tested
---

# XFCE

**What it is:** XFCE 4.20, straight from the Ubuntu archive. It's the desktop Xubuntu ships: one panel with an app menu ([Whisker Menu](screenshots/menu.png)), buttons for open windows, tray icons, sound, battery, a clock and one session button (lock, suspend, log out, shut down). Windows float, snap to screen edges, and tile to half the screen with SUPER+arrows. The Settings Manager covers nearly everything, and every setting is also a command (`xfconf-query`), so the agent can change them too. It runs on X11.

The playbook installs a small app set (Xfce Terminal, Thunar, Mousepad, Ristretto, Screenshooter, Clipman) and sets:
- the Greybird theme, dark or light, with elementary icons
- the panel, at the bottom or the top
- the Vibian wallpaper
- the keyboard, the lock and suspend timeouts
- SUPER shortcuts: the menu, a terminal, the agent and half-screen tiling
- LightDM for the login screen

**Who it's for:** people on old or low-memory machines (logged in with a terminal and the agent open, the whole system used about 900 MB in our test VM), people who want a plain Windows-style desktop without KDE's size, and anyone whose apps or GPU need X11. People who want a modern-looking desktop with many settings are better off with `kde-plasma`. Keyboard-tiling fans are better off with `i3` (also X11) or `sway`.

**Needs:** any GPU X11 drives, including the generic modesetting driver and VESA. A few hundred MB of packages with `minimal`.

**Computer use:** X11 lets any program read the screen and send input, so `vibian-computer` works fully here.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Dark or light? (`VIBIAN_THEME`)
2. Panel at the bottom (like Windows) or the top? (`VIBIAN_PANEL`)
3. Just the basics, or the fuller XFCE set (xfce4-goodies: weather, system load, notes and more panel plugins, a task manager, file search)? (`VIBIAN_APPS`)
4. Keyboard layout, if not US? (`VIBIAN_KB_LAYOUT`, `VIBIAN_KB_VARIANT`)
5. Laptop: suspend after a while idle? (`VIBIAN_IDLE_SUSPEND`, minutes)

## Knobs
`VIBIAN_APPS` and `VIBIAN_USER` go to `scripts/system.sh`. The rest go to `scripts/user.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_APPS` | `minimal` | `minimal`: XFCE and its core apps. `standard`: xfce4-goodies, a task manager and Catfish too |
| `VIBIAN_USER` | `$SUDO_USER` | the user whose default login session is set to XFCE |
| `VIBIAN_THEME` | `dark` | `dark` (Greybird-dark) or `light` (Greybird) |
| `VIBIAN_PANEL` | `bottom` | `bottom` or `top` |
| `VIBIAN_PANEL_RESET` | `0` | `1`: rebuild the panel even if the user changed it |
| `VIBIAN_ACCENT` | `7aa2f7` | hex colour for the terminal cursor and selection (Greybird's own blue stays) |
| `VIBIAN_BG` | `16161e` | hex colour around the wallpaper and behind the terminal |
| `VIBIAN_KB_LAYOUT` / `VIBIAN_KB_VARIANT` | `us` / empty | XKB layout |
| `VIBIAN_IDLE_LOCK` | `10` | minutes before the lock screen (0: never) |
| `VIBIAN_IDLE_SUSPEND` | `0` on desktops, `30` on laptops | minutes before suspend (0: never; the power manager's minimum is 15) |

## Steps
1. `vibian snap "xfce playbook"`.
2. Root part: the packages, the login screen and the user's default session. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_APPS=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install the XFCE desktop" -- /usr/bin/env VIBIAN_APPS=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   system.sh turns off any other login screen (greetd, sddm, gdm3) and makes LightDM the display manager.
3. User part, no sudo: `VIBIAN_…=… bash scripts/user.sh`.
   - It sets everything with `xfconf-query`. When XFCE is running, the changes show at once. Over SSH it uses the user's D-Bus session, or starts its own.
   - The first run copies `~/.config/xfce4/xfconf/xfce-perchannel-xml/` to `xfce-perchannel-xml.vibian-bak`.
   - The panel is built only on the first run, so later runs keep the user's panel changes. `VIBIAN_PANEL_RESET=1` rebuilds it.
   - The wallpaper is set per monitor, so a run-once login script does it: `~/.local/share/vibian/xfce-apply.sh`.
   - A lone SUPER tap opens the menu through `xcape`, started from `~/.config/autostart/vibian-xcape.desktop` (see Gotchas).
   - When XFCE is running, it also restarts xfwm4 so the new keys take effect. Windows stay open.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`.
5. `vibian log add "XFCE desktop (playbook xfce)" --pkgs … --files ~/.config/xfce4 --undo "…"` (closes the snapshot).
6. Tell the user to reboot, or to run `sudo systemctl start lightdm`, which switches the screen to the login. Once they're in, take a screenshot with the `vibian-computer` server and check that the panel and wallpaper are there.

## Keys
| keys | does |
|---|---|
| SUPER (tap), Alt+F1 | app menu (type to search) |
| SUPER+Return | terminal |
| SUPER+A | the agent (`vibian agent` in a terminal) |
| SUPER+E | files (Thunar) |
| SUPER+L, Ctrl+Alt+L | lock |
| SUPER+arrows | tile the window to that half of the screen |
| Alt+Tab | switch windows |
| Print | screenshot tool |
| SUPER+SHIFT+S | screenshot a region to the clipboard |
| Ctrl+Alt+Del | log out, restart or shut down |

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Settings: log out, then replace `~/.config/xfce4/xfconf/xfce-perchannel-xml` with `xfce-perchannel-xml.vibian-bak` (or delete it for XFCE's defaults). Delete `~/.local/share/vibian/xfce-apply.sh`, `~/.config/autostart/vibian-xfce-apply.desktop` and `~/.config/autostart/vibian-xcape.desktop`.
  - Run `sudo systemctl disable lightdm && sudo systemctl set-default multi-user.target`, then `sudo rm /etc/lightdm/lightdm.conf.d/50-vibian.conf`.
  - Remove the packages.

## Gotchas
- **Change settings with `xfconf-query` or the Settings Manager**, not by editing the XML files: xfconfd keeps them in memory and writes over your edits. To find a property, run `xfconf-query -m -c <channel>` (it prints changes) while you flip it in the Settings Manager; `xfconf-query -l` lists the channels.
- **SUPER key:** don't bind `Super_L` itself in Keyboard settings. That grabs the key, and then xfwm4 never sees SUPER+arrow or any other SUPER combo it owns. The playbook runs `xcape -e 'Super_L=Alt_L|F1'` instead: a lone tap sends Alt+F1, and Alt+F1 opens the menu. If the menu stops opening on SUPER, check that `xcape` is running.
- **One key per window action:** xfwm4 keeps a single shortcut per action, so the playbook removes the SUPER+keypad bindings for the four half-screen tiles (the corner tiles keep theirs). Adding a second key for an action in Window Manager settings silently replaces the first. xfwm4 picks up new keys live but needs `xfwm4 --replace` to drop old ones.
- **Panel changed by hand and now broken:** `VIBIAN_PANEL_RESET=1 bash scripts/user.sh` rebuilds it, then `xfce4-panel -r`.
- **Wallpaper missing on a new monitor:** XFCE stores it per connector. Run `~/.local/share/vibian/xfce-apply.sh` again, or set it in Desktop settings.
- **No login screen after reboot:** `systemctl status lightdm`, then `/var/log/lightdm/`. LightDM is on VT 7, and the text console stays on tty1.
- **Switching from another desktop:** system.sh disables greetd, SDDM and GDM, and the other playbooks disable LightDM in the same way. It also turns off the waybar, mako and hypridle user services that Ubuntu enables for every session. i3 and XFCE share LightDM: the session menu on the login screen picks between them.
- **Screen sharing and screenshots work everywhere:** X11 lets any program read the screen. That's also why `vibian-computer` works fully here, but it means any app you run can see every window.
- **Wayland:** XFCE 4.20's Wayland session is experimental, so the playbook doesn't set it up.
- **In a VM** X11 works with QEMU's standard VGA or virtio-gpu.
