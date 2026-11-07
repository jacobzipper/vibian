# Installing Vibian

> [!WARNING]
> Vibian is pre-alpha. Install it on a spare machine or in a VM, not on a computer you rely on.
> **The installer erases a whole disk.** With more than one disk it asks which, and it asks once
> more before it starts. Unplug any drive you care about anyway.

## What you need

- A 64-bit PC (**amd64**) or a 64-bit ARM machine (**arm64**) that boots with **UEFI**. Legacy BIOS boot isn't supported yet.
- 4 GB of RAM and a 25 GB disk at the very least. 8 GB and 64 GB are more comfortable once a desktop is on it.
- A network connection during the install and onboarding: your agent is installed from its official source.
- A USB stick of 4 GB or more, or a VM.

Secure Boot can stay on: the ISO keeps Ubuntu's signed boot chain.

| ISO | For | Tested |
|---|---|---|
| `vibian-<version>-amd64.iso` | Intel and AMD PCs and laptops, x86 VMs | Every change: install plus the whole [VM test](../tests/vm/headless-install.sh) under KVM |
| `vibian-<version>-arm64.iso` | ARM servers and workstations with UEFI, ARM VMs (UTM or Parallels on Apple Silicon, QEMU) | Install, btrfs layout, snapshots, undo and the CLI in an emulated QEMU VM. The desktop playbooks aren't tested on arm64 yet |

## 1. Download

Open the [latest release](https://github.com/jacobzipper/vibian/releases). GitHub caps release files at 2 GiB, so each ISO comes in parts. Download every `.part` file for your architecture and `SHA256SUMS`, then join the parts and check the result:

```bash
cat vibian-0.7.0-amd64.iso.part* > vibian-0.7.0-amd64.iso
sha256sum -c --ignore-missing SHA256SUMS
```

On macOS, use `shasum -a 256 -c --ignore-missing SHA256SUMS`. On Windows, join the parts in a Command Prompt:

```bat
copy /b vibian-0.7.0-amd64.iso.part1 + vibian-0.7.0-amd64.iso.part2 vibian-0.7.0-amd64.iso
certutil -hashfile vibian-0.7.0-amd64.iso SHA256
```

Releases after 0.7.0 also carry a signed build provenance attestation. With the [GitHub CLI](https://cli.github.com) you can check that the ISO was built by this repo's release workflow from a tagged commit:

```bash
gh attestation verify vibian-<version>-amd64.iso -R jacobzipper/vibian
```

## 2. Write it to a USB stick

Any of these work: [balenaEtcher](https://etcher.balena.io), [Ventoy](https://www.ventoy.net), [Rufus](https://rufus.ie) (pick DD mode when it asks), or `dd` on Linux:

```bash
sudo dd if=vibian-0.7.0-amd64.iso of=/dev/sdX bs=4M status=progress oflag=sync   # sdX is the stick, not your disk
```

In a VM, attach the ISO as a CD and turn on EFI (VirtualBox: *System → Enable EFI*; virt-manager: *Firmware: UEFI*).

## 3. Install

1. Boot from the stick in UEFI mode and pick **Install Vibian**. If the screen stays black, reboot and pick **Install Vibian (safe graphics)**.
2. **Disk:** if the machine has more than one disk of 25 GB or more, a blue menu lists them with their size, model and what's on them now. Pick the one to erase, or **Power off**. With a single disk this step is skipped. On a serial console, or to skip the menu, add `vibian.disk=/dev/nvme0n1` (your disk) to the boot entry's kernel line.
3. **Network:** wired connections come up on their own. For wifi, pick the wireless card and join your network.
4. **Profile:** your name, the computer's name, a username and a password. The password is for `sudo`; you choose later how much of it the agent gets.
5. **Confirm destructive action:** **Continue** erases the disk and installs. **No** goes back.
6. The install takes 10 to 30 minutes, depending on your connection. Pick **Reboot now** when it's done and remove the stick.

## 4. First boot

Vibian logs you in on the first console and starts onboarding: wifi if you're not online, then your agent (Claude Code, Codex, Gemini CLI, opencode, OpenClaw, pi or Cursor), its login and how much power it gets. From there, tell it what you want: *"give me a desktop"* is a good start. See [How it works](../README.md#how-it-works).

## Building the ISO yourself

The release ISOs are built by [`.github/workflows/release.yml`](../.github/workflows/release.yml). To build one locally you need `xorriso`, Go 1.24+ and the matching Ubuntu 26.04.1 Server ISO:

```bash
curl -LO https://releases.ubuntu.com/26.04.1/ubuntu-26.04.1-live-server-amd64.iso
bash iso/build.sh ubuntu-26.04.1-live-server-amd64.iso vibian-amd64.iso
# arm64, built on any machine:
VIBIAN_ARCH=arm64 bash iso/build.sh ubuntu-26.04.1-live-server-arm64.iso vibian-arm64.iso
```

`build.sh` checks the Ubuntu ISO's checksum, then adds the Vibian boot menu, `autoinstall.yaml` and the `/vibian` payload (`vibian-base` and `grub-btrfs`). Everything else on the ISO is Ubuntu's, byte for byte.
