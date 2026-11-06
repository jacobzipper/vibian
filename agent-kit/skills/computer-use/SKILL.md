---
name: computer-use
description: "See and drive this machine's desktop with the vibian-computer MCP tools: screenshot, windows, focus_window, click, drag, scroll, type_text, key. Use when a task needs a GUI (checking how a rice looks, clicking through a settings app, testing a widget or bar, filling a form in a desktop app), when the user says 'look at my screen' or 'click', or to verify a desktop change actually worked."
---

# Computer use

`vibian-computer` is an MCP server (`vibian mcp computer`) that `vibian agent` registers in every harness next to `vibian-system`. It works on the desktop of the user who is logged in at the machine, even when you were started from a TTY or over SSH: it finds the session's sockets itself.

| desktop | screenshots | windows / focus | pointer | keyboard |
|---|---|---|---|---|
| Hyprland | grim | hyprctl | uinput tablet | wtype |
| Sway | grim | swaymsg | Sway's own `seat cursor` | wtype |
| niri | grim | `niri msg` | uinput tablet | wtype |
| other wlroots (river, labwc, Wayfire…) | grim | none: use keys | uinput tablet | wtype |
| COSMIC | cosmic-screenshot (the user allows it once) | none: use keys | uinput tablet | wtype |
| KDE Plasma (Wayland) | spectacle | KWin scripts | uinput tablet | uinput (US ASCII) |
| X11 (any WM) | maim, or ImageMagick `import` | wmctrl + xdotool | xdotool | xdotool |
| GNOME (best effort) | gnome-screenshot, often refused on Wayland | none | uinput tablet | uinput (US ASCII) |

A tool that needs something missing says which package to install (`grim`, `wtype`, `xdotool`, `wmctrl`, `maim`, `spectacle`, `cosmic-screenshot`). The playbooks install them with each desktop. `/dev/uinput` belongs to whoever is logged in at the screen (Vibian's udev rule `70-vibian-uinput.rules`, the same idea as Steam's controller rule).

## The loop
1. `windows` first. It is cheap and gives you the monitors, every window with its position, and which one is focused. `focus_window` beats hunting with the mouse.
2. `screenshot` to see. By default it is scaled so the long edge is 1280 px. Pass `region` (`"x,y,w,h"` in screen pixels) with `scale` 1 to read small text. The note that comes with every image gives `to_screen`: **screen = image / scale + origin**. Do the maths; don't click image coordinates.
3. Act: `click`, `drag`, `scroll`, `type_text`, `key`. Prefer the keyboard. Shortcuts and typed commands are exact, and a click is a guess at a pixel.
4. `screenshot` again to check it worked before you say it did. UIs animate, so take another if the first catches a transition.

## Rules
- **Ask first** before anything that sends, posts, buys, deletes, signs in, accepts terms or changes an account. Show the user what you're about to submit.
- Never type a password, token or recovery code unless the user gave it to you for that field just now. Don't read secrets off the screen into the chat or the ledger.
- Stay in the windows the task needs. Don't open the user's mail, chats or browser tabs to "look around".
- If the user is actively using the machine (the focused window changes between your screenshots, or the pointer moves), stop and ask before taking over the mouse.
- Don't use computer use for what a config file or CLI does better. Edit `hyprland.conf`, then reload and screenshot to check. Don't click through a settings GUI to do the same thing.
- GUI actions aren't snapshotted. If you changed system settings through a GUI, `ledger_add` what you changed and where.

## Desktop notes
- **Hyprland**: window ids are addresses (`0x55d…`). `hyprctl dispatch` covers workspaces, moves and layouts more reliably than dragging.
- **Sway**: ids are con ids. `swaymsg '[app_id=foot] focus'` and friends work too.
- **niri**: windows have no screen position in its IPC, so take a screenshot to find them. `niri msg action` covers columns and workspaces.
- **KDE**: the keyboard fallback types US ASCII only. For other text, `wl-copy 'text'`, then `key ctrl+v`. Window ids are KWin UUIDs.
- **COSMIC**: the first screenshot makes COSMIC ask the user "Allow Apps to Take Screenshots?". Tell them to click Allow before you take it. If they denied it, they can change it in Settings. There's no window list: tap SUPER for the launcher (it finds open windows too), or use Alt+Tab and SUPER+arrows.
- **X11**: ids are hex window ids from wmctrl. Panels and docks (desktop -1) are left out.
- **GNOME** keeps silent screenshots and window lists to itself and its extensions. Say so, and suggest Hyprland/Sway/KDE/COSMIC if the user needs real computer use.
- Multiple monitors share one coordinate space (the `monitors` list in `windows`). Negative positions are fine.

## Troubleshooting
| symptom | fix |
|---|---|
| `no graphical session found` | Nobody is logged into a desktop, or it runs as another user. Ask the user to log in. |
| `can't open /dev/uinput` | The user isn't logged in at the machine's own screen (uaccess). Check with `getfacl /dev/uinput`. |
| the first typed letter goes missing | Rare with wtype. Retype, or type into a focused field after a click. |
| clicks land in the wrong place | You forgot the screenshot's scale or origin, or the monitor layout changed. Call `windows` and redo the maths. |
| screenshots are black on NVIDIA | Old driver without screencopy support: check `drivers-hardware`. |
