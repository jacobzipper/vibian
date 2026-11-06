---
name: power
description: "Battery life, power profiles, suspend and hibernate, lid and power-button behaviour, idle, charge limits and laptop vendor tools (Lenovo Legion/IdeaPad/ThinkPad, ASUS, Framework) on Vibian. Use when the user asks about battery, heat, fan noise, sleep not working, what closing the lid does, or limiting charge to protect the battery."
---

# Power on Vibian

## Look first
```bash
upower -d | grep -E 'model|state|percentage|energy-rate|capacity'   # capacity < 80% = worn battery
cat /sys/class/power_supply/BAT*/charge_control_end_threshold 2>/dev/null
powerprofilesctl                       # current profile and who provides it
systemctl is-active tuned power-profiles-daemon tlp thermald 2>/dev/null
cat /sys/power/mem_sleep               # [s2idle] or [deep]
sudo powertop --time=10 --html=/tmp/powertop.html   # biggest consumers (apt install powertop)
```

## Power profiles
Ubuntu 26.04 provides profiles with **tuned** and `tuned-ppd` (the power-profiles-daemon API on top of tuned). The older `power-profiles-daemon` is still in the archive. Only one of tuned-ppd, power-profiles-daemon or **TLP** may run: they fight.
```bash
sudo apt-get install -y tuned tuned-ppd       # if neither is installed
powerprofilesctl set power-saver|balanced|performance
tuned-adm list; tuned-adm active              # finer control
```
- TLP only when the user wants its fine control: it conflicts with tuned-ppd and power-profiles-daemon, so apt removes them. Ledger that.
- Intel laptops that run hot: `thermald`.

## Suspend
```bash
systemctl suspend
journalctl -b -1 -g 'suspend|resume|PM:' --no-pager | tail -40    # after a failed resume
```
- Wakes up by itself: `cat /proc/acpi/wakeup`, then disable the culprit (often `XHC` USB or the lid) with a small systemd unit or udev rule, not by hand each boot.
- Battery drains while suspended on s2idle-only machines: check for a BIOS "Linux"/S3 sleep option before trying `mem_sleep_default=deep` (kernel cmdline in `/etc/default/grub.d/`, then `update-grub`; snapshot first).
- **Hibernate** is off on purpose. Vibian installs without swap, and the kernel locks hibernation down while Secure Boot is on. Explain that before trying to set it up.

## Lid, power button, idle
logind decides what the lid and the power button do. Use a drop-in, never edit `logind.conf`:
```ini
# /etc/systemd/logind.conf.d/50-vibian-lid.conf
[Login]
HandleLidSwitch=suspend
HandleLidSwitchExternalPower=suspend
HandleLidSwitchDocked=ignore
HandlePowerKey=suspend
```
Apply with `sudo systemctl kill -s HUP systemd-logind`. **Don't restart systemd-logind**: that ends every session, including the user's desktop.

Idle dimming, locking and suspend-after-idle belong to the desktop (hypridle, swayidle, GNOME/KDE settings): see the desktop playbook in use.

## Charge limits (battery health)
Generic, works on many ThinkPads, ASUS, Framework, Dell and Huawei laptops:
```bash
echo 80 | sudo tee /sys/class/power_supply/BAT0/charge_control_end_threshold
```
It resets at boot. Persist it with a udev rule:
```
# /etc/udev/rules.d/90-vibian-charge-limit.rules
ACTION=="add", SUBSYSTEM=="power_supply", KERNEL=="BAT0", ATTR{charge_control_end_threshold}="80"
```
- **Lenovo Legion/IdeaPad**: no threshold file. Conservation mode caps at about 60-80% depending on the model: `echo 1 | sudo tee /sys/bus/platform/drivers/ideapad_acpi/VPC2004:*/conservation_mode`. Legion fan/performance modes follow `powerprofilesctl` through the `platform_profile` driver.
- **ThinkPad**: `charge_control_start_threshold` exists too (start 75, stop 80 is a common pair).
- **ASUS**: `charge_control_end_threshold` works. `asusctl`/`supergfxctl` (fan curves, keyboard LEDs, GPU modes) come from the asus-linux project, not the Ubuntu archive. Ask before adding that repo.
- **Framework**: the threshold works on recent BIOSes; `framework_tool` is upstream only.

## Ledger
Write every logind drop-in, udev rule, kernel parameter and TLP/tuned switch to the ledger, with the measured before/after (`upower` energy-rate) when it was about battery life.
