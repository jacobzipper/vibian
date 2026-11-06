<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/vibian-logo-dark.png">
    <img src="docs/assets/vibian-logo-light.png" alt="Vibian's mascot, a winking sphinx cat" width="160">
  </picture>
</p>

# Vibian

**A Linux that boots to a blank terminal. You pick a coding agent, and the agent builds the rest.**

Vibian is an Ubuntu 26.04 LTS–based distro for people who already live in Claude Code, Codex, opencode, OpenClaw, pi, Gemini CLI or Cursor. Install it and you get a terminal and an onboarding screen. Pick your agent, log in, and from then on the agent runs your machine. It sets up drivers, wifi and power. Then it interviews you and builds a desktop: Hyprland, KDE, GNOME, niri, Sway, i3, XFCE, COSMIC, a widget desk, or something nobody has made before.

Root for an LLM sounds insane, so Vibian is built around undo:

- **Snapshots of `/` and `/home`** (btrfs + snapper) before every agent session, every turn and every package change. They're bootable from GRUB if something goes very wrong.
- **The ledger:** every change the agent makes (files, packages, snapshots, how to undo it, gotchas learned) goes into plain markdown. Any agent can read it, so you can switch harnesses and the next agent knows your machine.
- **`vibian undo`** reverts a single ledger entry, or the whole session.

And because the ledger knows how your machine was built, **you can share it**:

```sh
vibian export --publish                    # your whole setup → a git repo
vibian apply github:someone/their-rice     # their setup, rebuilt by your agent, adapted to your hardware
```

## Status

🚧 **Pre-alpha, built in public.** Nothing to install yet. The design is in [`docs/SPEC.md`](docs/SPEC.md).

| | |
|---|---|
| Spec | ✅ draft 1 |
| Harness adapter research | ✅ [`docs/research/harness-adapters.md`](docs/research/harness-adapters.md) |
| ISO remaster pipeline | ✅ boots and installs unattended in a KVM VM (UEFI) |
| btrfs + snapper autoinstall layout | ✅ boots from `@`, apt snapshots work ([test](tests/vm/headless-install.sh)) |
| grub-btrfs snapshot boot menu | ✅ packaged ([`packages/grub-btrfs`](packages/grub-btrfs)), menu fills in as snapshots are taken |
| Booting a snapshot | ✅ pick it in GRUB → the system comes up as it was, in a RAM overlay ([`packages/vibian-base`](packages/vibian-base)) |
| Onboarding TUI, `vibian` CLI, ledger | ✅ first boot lands in onboarding on tty1; `vibian snap/log/undo/doctor` and the git-backed ledger pass the VM test ([`cmd/vibian`](cmd/vibian)) |
| Rollback | ✅ `vibian undo --to <n>`, even from a snapshot boot: the next boot runs the snapshot as `@`, and the old root is kept for 7 days |
| Harness install from onboarding (7 agents) | 🚧 pi and opencode installed and handed over from onboarding in the VM, driven by keyboard ([`tests/vm/drive.sh`](tests/vm/drive.sh)). All 7 official installers run unattended in the VM and every flag Vibian passes exists in their `--help`. Logging in to a real account is next |
| 4 of 14 OS skills | ✅ packages, drivers-hardware, recovery, displays ([`agent-kit`](agent-kit)) |
| Remaining 10 OS skills + computer-use MCP | ⏳ |
| 10 desktop playbooks | ⏳ |
| Examples (Bloomberg desk, startup mission control, grandma mode) | ⏳ |

## Why not Omarchy?

Omarchy is great if you want DHH's desktop. Vibian is for people who want *their own*. It starts with no desktop at all, runs on Ubuntu LTS so NVIDIA cards and Secure Boot just work, snapshots `/home` as well as `/`, and lets you share whole machines rather than just themes. There's a longer comparison in the [spec](docs/SPEC.md#1-positioning).

## Origin

Vibian started as one laptop. A stock Ubuntu install was turned into a fully custom /g/-style anime rice in about a day, entirely by prompting coding agents. That build had a Windows-style desk, drag-to-edge tiling, a VPN widget, a local image-gen app and an embedded agent with computer use. A second agent then picked up where the first left off by reading its change log. Vibian turns that into an OS.

## License

[GPL-3.0](LICENSE). Agent harnesses are installed from their official sources at onboarding and are never bundled; several are proprietary.
