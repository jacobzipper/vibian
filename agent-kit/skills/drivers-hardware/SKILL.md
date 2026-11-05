---
name: drivers-hardware
description: "Graphics drivers (NVIDIA incl. hybrid laptops/PRIME, AMD, Intel), Secure Boot MOK enrolment, firmware updates (fwupd), Wi-Fi/Bluetooth firmware, fingerprint readers and vendor laptop tools on Vibian. Use for \"my GPU isn't used\", \"install the NVIDIA driver\", black screens after driver changes, CUDA, firmware updates and unknown hardware."
---

# Drivers and hardware on Vibian

Start with facts: `vibian doctor`, then
```bash
lspci -nnk | grep -A3 -Ei 'vga|3d|display'    # GPUs and the driver in use
ubuntu-drivers list                            # what Ubuntu recommends
mokutil --sb-state                              # Secure Boot on/off
journalctl -b -k -p warning | tail -50         # kernel complaints this boot
```

## NVIDIA
- Install with Ubuntu's tool so it matches the kernel and signs for Secure Boot:
  ```bash
  vibian snap "nvidia driver"
  sudo ubuntu-drivers install            # or: sudo ubuntu-drivers install nvidia:<version>
  ```
  Use the `-open` kernel modules for Turing (RTX 20xx/GTX 16xx) and newer. Use the proprietary ones only for older cards.
- **Secure Boot on:** the install creates a MOK key and asks for a one-time password. On the next boot the blue **MOK Manager** screen appears, where the user picks *Enroll MOK* and types that password. Tell the user this **before** rebooting, or the module silently won't load.
- **Hybrid laptops** (Intel/AMD iGPU + NVIDIA): the default is on-demand offload. Run a program on the NVIDIA GPU with `__NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia <cmd>` (or `prime-run` if installed). `prime-select query|on-demand|nvidia|intel` switches the mode, and a switch needs a re-login.
- Power: enable `nvidia-suspend`, `nvidia-resume` and `nvidia-hibernate` (normally done by the package). For Wayland, add `nvidia-drm.modeset=1` via `/etc/default/grub.d/`, then `update-grub` (default on recent drivers).
- CUDA: `nvidia-smi` must work first. Prefer `nvidia-cuda-toolkit` from the archive, or vendor containers (`nvidia-container-toolkit`).
- **Black screen after a driver change:** boot the previous snapshot from GRUB (recovery skill), or switch to a TTY (Ctrl+Alt+F3) and run `sudo apt-get purge '^nvidia-.*' && sudo ubuntu-drivers install`, or `vibian undo --last`.

## AMD and Intel
Both work out of the box (amdgpu, i915/xe in the kernel; Mesa in userspace). Newer Mesa means a newer Ubuntu point release or the `kisak-mesa` PPA (ledger it). For Intel media decode, install `intel-media-va-driver-non-free`. For AMD ROCm, use AMD's repo; check the GPU is supported first.

## Firmware
```bash
sudo apt-get install -y fwupd
fwupdmgr refresh && fwupdmgr get-updates
fwupdmgr update             # snapshot first; BIOS updates reboot
```
For missing Wi-Fi/Bluetooth firmware (`dmesg | grep -i firmware`), the `linux-firmware` package covers most. Some need `firmware-sof-signed` (audio) or a vendor blob.

## Laptops
- Lenovo Legion / IdeaPad: `ideapad_laptop` exposes conservation mode at `/sys/bus/platform/drivers/ideapad_acpi/*/conservation_mode`. LenovoLegionLinux for fan and power modes (DKMS).
- ThinkPad: `thinkpad_acpi`; charge thresholds at `/sys/class/power_supply/BAT0/charge_control_{start,end}_threshold`.
- ASUS: `asusctl`/`supergfxctl` from the asus-linux repo.
- Fingerprint: `fprintd` + `libpam-fprintd`, then `fprintd-enroll`; check the reader is supported with `lsusb` and the libfprint list.

## Always
Write the driver version, kernel version and anything the user had to do at boot (MOK) into the ledger entry. That's the first thing the next agent needs when a kernel update breaks it.
