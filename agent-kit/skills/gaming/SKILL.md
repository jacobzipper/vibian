---
name: gaming
description: "Gaming on Vibian: Steam (deb) with Proton, 32-bit graphics drivers, GameMode, MangoHud, controllers and their udev rules, Lutris/Heroic for other stores, and anti-cheat limits. Use when the user wants to install Steam or play a Windows game, a game won't start or runs slowly, or a controller isn't detected."
---

# Gaming on Vibian

Needs a desktop (Steam doesn't run on a bare tty) and working GPU drivers. Check `vibian doctor` and the `drivers-hardware` skill first.

## Steam
Ubuntu's `steam-installer` (multiverse) installs Valve's launcher, which then updates itself into `~/.steam`. Steam is 32-bit in parts, so i386 has to be enabled:
```bash
vibian snap "install steam"
sudo dpkg --add-architecture i386 && sudo apt-get update
sudo apt-get install -y steam-installer steam-devices mesa-vulkan-drivers mesa-vulkan-drivers:i386 libgl1-mesa-dri:i386
```
- **NVIDIA**: add the 32-bit driver libraries matching the installed driver: `dpkg -l 'libnvidia-gl-*'` shows the version, then `sudo apt-get install libnvidia-gl-<ver>:i386`. A missing i386 GL library is the usual "Steam opens a black window" cause.
- The Flatpak (`com.valvesoftware.Steam`) is the alternative when i386 multiarch isn't wanted. Pick one, not both.
- `steam-devices` adds the udev rules for Steam controllers, the Steam Deck, Index and many gamepads.

## Proton
Steam → Settings → Compatibility → "Enable Steam Play for all other titles". Per game: Properties → Compatibility.
- Before saying a game works, look it up on protondb.com (the user can check, or you fetch it).
- Proton-GE and other custom builds go in `~/.steam/root/compatibilitytools.d/` (ProtonUp-Qt from Flathub manages them). Restart Steam after adding one.
- Anti-cheat: games whose developers haven't enabled EAC/BattlEye for Proton, and kernel anti-cheat (Valorant, most competitive shooters), **won't run**. Say so instead of trying fixes.

## Performance
```bash
sudo apt-get install -y gamemode mangohud
```
Launch options per game: `gamemoderun mangohud %command%`. `gamemoded -t` tests GameMode. MangoHud's config is `~/.config/MangoHud/MangoHud.conf` (fps_limit, toggle key).
- Hybrid-GPU laptops: make sure the game runs on the dGPU (`__NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia %command%` on NVIDIA; `DRI_PRIME=1 %command%` on AMD), and check with MangoHud's GPU line.
- Set `powerprofilesctl set performance` while playing (see `power`).
- Shader pre-caching uses a lot of disk; it can be turned off in Steam's Downloads settings.

## Controllers
| controller | how |
|---|---|
| Xbox over USB, PlayStation 4/5, Switch Pro | kernel drivers built in (`xpad`, `hid-playstation`, `hid-nintendo`) plus `steam-devices` |
| Xbox over Bluetooth | pair with `bluetoothctl` (see `audio-bluetooth`). If buttons are wrong or it keeps disconnecting, `xpadneo` (DKMS, upstream: needs headers and MOK with Secure Boot) |
| 8BitDo and others | try its "X-input" mode (often a button held at power-on) |
Test with `evtest` or Steam's controller settings.

## Other stores
Heroic (Epic, GOG, Amazon) and Lutris: install from Flathub (`com.heroicgameslauncher.hgl`, `net.lutris.Lutris`). Lutris's archive deb is often old.

## Disk
Steam libraries are big and sit in `~/.steam` / `~/.local/share/Steam`, which `/home` snapshots cover. Hourly snapshots keep deleted games on disk for up to a week. For big libraries, suggest a separate library folder on a subvolume outside snapper (same recipe as models in `local-ai`), added in Steam → Storage.
