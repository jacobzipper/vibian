---
id: ask-overlay
name: Ask overlay
description: An extra for any desktop. SUPER+/ opens a box to type a request for the agent from anywhere. The agent works on it in the background and a notification says when it's done; the box lists the recent requests and their answers.
status: tested
---

# Ask overlay

**What it is:** a box that pops up over whatever is on screen when the user presses SUPER+/. They type a request, like "make the text bigger" or "why is the fan loud", and press Return. The box can close straight away: the agent works on it in the background, and a notification shows the end of its answer when it's done. Opening the box again lists the last few requests with ✓ (done), … (working), ✗ (failed) or ■ (stopped); clicking one shows the whole answer, and a running one can be stopped.

It is a small GTK 4 window (`vibian-ask`, [screenshot](screenshots/overlay.png)). On Hyprland, Sway, niri, KDE and COSMIC it sits on top of everything (layer shell); on GNOME and X11 desktops it's a normal window without a title bar (and floats in i3).

Behind it is `vibian ask "…"`, which works without the overlay too, in any terminal or over SSH. It runs the user's agent in its one-shot mode (`claude -p`, `codex exec`, `gemini -p`, `opencode run`, `openclaw agent --local`, `pi -p`, `agent -p` for Cursor) with the same powers as `vibian agent`, between a snapshot pair. If the request changed files and the agent wrote no ledger entry, vibian writes one. `vibian asks` lists the requests and `vibian asks ID` shows one.

**Who it's for:** anyone on a desktop who wants the agent one key away without a terminal, for quick jobs and questions. Long conversations are still better in `vibian agent` (SUPER+A).

**Needs:** a desktop set up by one of the other playbooks (its user.sh binds the key in each one it finds). A request only works once the agent has been logged in through `vibian agent`.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Is SUPER+/ fine as the key? If not, which one? (Change the bind by hand afterwards; see Gotchas.)
2. Should the box match the desktop's colours? Use the accent and background the desktop playbook used. (`VIBIAN_ACCENT`, `VIBIAN_BG`)

## Knobs
All go to `scripts/user.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_ACCENT` | `7aa2f7` | hex colour of the border and highlights |
| `VIBIAN_BG` | `16161e` | hex background colour |

## Steps
1. `vibian snap "ask overlay playbook"`.
2. Root part. Run it the way the powers setting allows:
   - `root` powers: `sudo bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "install the Ask overlay" -- /usr/bin/env bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   It installs python3-gi, GTK 4's and gtk4-layer-shell's GObject bindings, gtk4-layer-shell itself and libnotify-bin, then copies the overlay to `/usr/local/bin/vibian-ask` and `/usr/local/lib/vibian/ask-overlay.py`.
3. User part, no sudo: `VIBIAN_ACCENT=… bash scripts/user.sh`. It binds SUPER+/ in every desktop a playbook set up for this user, and adds "Ask your agent" to the app menu:
   - Hyprland and the Quickshell desk: one line in `~/.config/hypr/local.conf`.
   - Sway and i3: `~/.config/sway/config.d/vibian-ask`, `~/.config/i3/config.d/vibian-ask`.
   - niri: a block in `~/.config/niri/local.kdl`.
   - XFCE: `xfce4-keyboard-shortcuts`. KDE Plasma: `kglobalshortcutsrc`. GNOME: a custom keybinding in dconf. COSMIC: one line in its custom shortcuts.
4. `bash scripts/verify.sh`. It must print `VERIFY OK`.
5. `vibian log add "Ask overlay (playbook ask-overlay)" --pkgs … --files /usr/local/bin/vibian-ask,/usr/local/lib/vibian/ask-overlay.py,~/.local/share/applications/vibian-ask.desktop,… --undo "…"` (closes the snapshot).
6. Reload the desktop's config so the key works now: `hyprctl reload`, `swaymsg reload`, `i3-msg reload`. niri, XFCE, GNOME and COSMIC pick it up by themselves; KDE Plasma at the next login. Then have the user press SUPER+/ and ask something small, like "what's my IP address?".

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Delete the lines and blocks marked `vibian playbook ask-overlay` (Hyprland, niri, COSMIC), the `vibian-ask` files in Sway's and i3's `config.d`, and `~/.local/share/applications/vibian-ask.desktop`.
  - XFCE: `xfconf-query -c xfce4-keyboard-shortcuts -p '/commands/custom/<Super>slash' -r`.
  - GNOME: remove `vibian-ask` from the `custom-keybindings` list in dconf.
  - Run `sudo rm -r /usr/local/bin/vibian-ask /usr/local/lib/vibian`.

## Gotchas
- **Requests run without a window to approve things in.** With `confirm` powers the agent has nobody to ask, so it skips or refuses whatever needs approval and says so in its answer. Root and scoped powers work as usual (`vibian sudo` still waits for the owner).
- **Not logged in yet:** the answer is the harness's login error. Log in once with `vibian agent`.
- **The notification** needs a notification daemon: mako (Hyprland, Sway, niri), dunst (i3), or the desktop's own. Without one, the overlay still shows the answer.
- **A different key:** edit the bind the playbook wrote, keeping the marker comment so a re-run replaces it. In KDE and GNOME use the keyboard settings.
- **gtk4-layer-shell must load before libwayland**, which is why `vibian-ask` is a small shell script that preloads it. Running `python3 /usr/local/lib/vibian/ask-overlay.py` directly gives a plain window instead of an overlay.
- **Two requests at once** both run. Each gets its own snapshot pair, and the pairs may overlap.
