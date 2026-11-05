---
name: displays
description: "Monitors on Vibian: resolution, refresh rate, scaling/HiDPI, multi-monitor layout, rotation, mirroring and fractional scaling, for Hyprland, Sway, niri, KDE Plasma, GNOME, X11 desktops and the bare console. Use for \"my screen is blurry/tiny/wrong resolution\", \"add my second monitor\", \"set 144Hz\" or \"the laptop screen should be on the left\"."
---

# Displays on Vibian

First find out **what's drawing the screen**:
```bash
echo "$XDG_SESSION_TYPE $XDG_CURRENT_DESKTOP"     # wayland Hyprland / x11 XFCE / tty
loginctl show-session $(loginctl | awk '/seat0/{print $1; exit}') -p Type -p Desktop
```
If you run over ssh or from a TTY those variables are empty. Check the session of the user on seat0, and export `WAYLAND_DISPLAY`/`DISPLAY` from `/run/user/<uid>/` before calling desktop tools.

Then list outputs with the matching tool, and **always keep a way back**: change at runtime first, confirm with the user that it looks right, then persist.

| desktop | list | set at runtime | persist in |
|---|---|---|---|
| Hyprland | `hyprctl monitors all` | `hyprctl keyword monitor DP-1,2560x1440@144,0x0,1.25` | `~/.config/hypr/hyprland.conf` (`monitor=` lines) |
| Sway | `swaymsg -t get_outputs` | `swaymsg output DP-1 mode 2560x1440@144Hz pos 0 0 scale 1.25` | `~/.config/sway/config` |
| niri | `niri msg outputs` | (edit config; niri reloads it live) | `~/.config/niri/config.kdl` `output "DP-1" { mode "2560x1440@144"; scale 1.25; position x=0 y=0; }` |
| KDE Plasma | `kscreen-doctor -o` | `kscreen-doctor output.DP-1.mode.2560x1440@144 output.DP-1.scale.1.25` | saved by KScreen automatically |
| GNOME | `gnome-monitor-config list` (if installed) | Settings → Displays; scripted: `gdctl` (GNOME 48+) | `~/.config/monitors.xml` |
| X11 (i3, XFCE) | `xrandr` | `xrandr --output DP-1 --mode 2560x1440 --rate 144 --pos 0x0` | i3: `exec_always xrandr …`; XFCE: `xfce4-display-settings` profile |
| console (tty) | `cat /sys/class/drm/*/modes` | kernel arg `video=DP-1:2560x1440@60` | `/etc/default/grub.d/` then `update-grub` |

## Scaling rules of thumb
- ~14" 1080p: 1.0 to 1.25. 14 to 16" 1440p/1600p: 1.25 to 1.5. 4K at 27": 1.5. 4K at 15": 2.0.
- Fractional scaling makes XWayland apps blurry. Hyprland: `xwayland { force_zero_scaling = true }` plus app-level scaling (`GDK_SCALE`, `QT_SCALE_FACTOR`, Electron `--ozone-platform=wayland`). KDE handles it per app.
- Positions are in **logical** pixels on Wayland (after scaling). On X11 they're physical.

## Multi-monitor checklist
1. List outputs, connectors (`DP-1`, `HDMI-A-1`, `eDP-1`) and preferred modes.
2. Ask where each screen physically sits (left/right/above). Don't guess.
3. Set mode, scale and position for each. Make positions line up (e.g. laptop 2560/1.6 = 1600 logical wide → external at x=1600).
4. Pick the primary or workspace assignment if the desktop supports it.
5. With the lid closed and docked: the desktop's lid switch handling, plus `HandleLidSwitchDocked=ignore` in `/etc/systemd/logind.conf.d/`.

## Gotchas
- High refresh over HDMI often needs HDMI 2.0+ cables. Prefer DisplayPort.
- NVIDIA + Wayland needs `nvidia-drm.modeset=1` (see `drivers-hardware`).
- If the screen goes black after a change, Hyprland/Sway configs reload on save: revert the line. KDE/GNOME revert by themselves after a timeout unless confirmed.
- Ledger the final config lines, not just "set up monitors". The export feature replays them on other machines.
