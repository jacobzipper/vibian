---
name: audio-bluetooth
description: "Sound and Bluetooth on Vibian: PipeWire and WirePlumber (outputs, inputs, volume, default device, HDMI/USB/Bluetooth audio, no-sound fixes) and BlueZ (pairing headphones, mice, keyboards and controllers). Use when there is no sound, the wrong device plays, the mic doesn't work, or the user wants to pair or fix a Bluetooth device."
---

# Audio and Bluetooth on Vibian

The server base has no sound stack. A desktop playbook installs it; on a bare tty install it with:
```bash
sudo apt-get install -y pipewire-audio wireplumber alsa-utils pulseaudio-utils bluez
```
`pipewire-audio` brings pipewire-pulse, pipewire-alsa and the Bluetooth plugin (`libspa-0.2-bluetooth`). They run as **user** services, started by the login session.

## Look first
```bash
systemctl --user status pipewire pipewire-pulse wireplumber
wpctl status                       # sinks (outputs), sources (inputs), default marked with *
aplay -l; arecord -l               # what ALSA/the kernel sees
pactl info | grep -i server        # "PulseAudio (on PipeWire …)" means apps are routed right
```

## Everyday
```bash
wpctl set-default <id>                         # id from wpctl status
wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%+      # or 0.4 for 40%; add -l 1.0 to cap
wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
wpctl set-volume @DEFAULT_AUDIO_SOURCE@ 80%    # mic
pactl list cards short; pactl set-card-profile <card> <profile>   # HDMI vs analog, BT A2DP vs headset
speaker-test -c 2 -t wav -l 1                  # test tone; tell the user first
```

## No sound
| symptom | check / fix |
|---|---|
| `wpctl status` shows only "Dummy Output" | the kernel has no card: `aplay -l`, `dmesg \| grep -iE 'sof\|snd\|hda'`. Intel laptops need SOF firmware (`linux-firmware`, `firmware-sof-signed`), then reboot |
| card exists but nothing plays | wrong default or muted: `wpctl status`, `alsamixer` (F6 picks the card, M unmutes) |
| HDMI/DisplayPort silent | switch the card profile to the HDMI output; monitors with no speakers still show up as sinks |
| crackling, dropouts | raise the quantum: `~/.config/pipewire/pipewire.conf.d/10-quantum.conf` with `context.properties = { default.clock.min-quantum = 1024 }`, then restart the services |
| only root hears sound | audio is per user; never run players with sudo |

Restart the stack (cuts audio for a second): `systemctl --user restart wireplumber pipewire pipewire-pulse`.

WirePlumber 0.5 is configured with SPA-JSON `.conf` drop-ins in `~/.config/wireplumber/wireplumber.conf.d/`. The old Lua scripts from 0.4-era guides do nothing.

## Bluetooth
```bash
systemctl status bluetooth; rfkill list           # rfkill unblock bluetooth
bluetoothctl power on
bluetoothctl --timeout 20 scan on                 # put the device in pairing mode first
bluetoothctl pair <MAC> && bluetoothctl trust <MAC> && bluetoothctl connect <MAC>
bluetoothctl devices Connected; bluetoothctl remove <MAC>   # forget
```
- Headphones: the A2DP profile has good sound but no mic. Using the mic switches to HFP/HSP, which sounds like a phone call. That's Bluetooth, not a bug. Switch with `pactl set-card-profile bluez_card.<MAC_> a2dp-sink` or a `headset-head-unit` profile.
- Codecs (LDAC, aptX, AAC) come with `libspa-0.2-bluetooth`; `pactl list cards` shows what was negotiated.
- Pairing keeps failing: `bluetoothctl remove <MAC>`, forget the computer on the device too, then pair again. Keyboards may ask you to type a PIN on them: tell the user.
- Dual-boot with Windows: pairing keys differ per OS, so the device has to be paired again after switching.
- Game controllers: see the `gaming` skill (udev rules, xpadneo).

## Ledger
Write config file changes (`~/.config/pipewire`, `~/.config/wireplumber`, `/etc/bluetooth/main.conf`) to the ledger. Pairing a device doesn't need an entry.
