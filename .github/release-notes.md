**Pre-alpha.** Install it on a spare machine or in a VM, not on a computer you rely on.

| Architecture | Runs on | Tested |
|---|---|---|
| **amd64** | 64-bit Intel and AMD PCs, UEFI | Every change: full install and the whole test suite in a KVM VM |
| **arm64** | 64-bit ARM with UEFI: servers, workstations, VMs (UTM, Parallels, QEMU) | Install, snapshots, undo and the CLI in an emulated VM. The desktop playbooks aren't tested on arm64 yet |

### Download

GitHub caps release files at 2 GiB, so each ISO comes in parts. Download every part for your architecture plus `SHA256SUMS`, then join and check them:

```bash
cat vibian-@VERSION@-amd64.iso.part* > vibian-@VERSION@-amd64.iso
sha256sum -c --ignore-missing SHA256SUMS
```

On Windows: `copy /b vibian-@VERSION@-amd64.iso.part1 + vibian-@VERSION@-amd64.iso.part2 vibian-@VERSION@-amd64.iso`

### Install

1. Write the ISO to a USB stick with [balenaEtcher](https://etcher.balena.io), Ventoy, Rufus (DD mode) or `dd`, and boot it in UEFI mode.
2. Pick **Install Vibian**. The installer asks for your network and your user, then asks you to confirm. **Continue erases the largest disk in the machine**, and you can't pick another one yet, so unplug any drive you care about.
3. Reboot into Vibian. Onboarding starts on its own and asks which coding agent you want.

Full guide: [docs/INSTALL.md](https://github.com/jacobzipper/vibian/blob/v@VERSION@/docs/INSTALL.md)
