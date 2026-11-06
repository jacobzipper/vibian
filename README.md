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
| Agent powers | ✅ "Everything" runs each agent in its no-prompts mode with passwordless sudo. "Ask first" makes every agent ask before commands and edits (opencode through permission rules, OpenClaw through its exec policy). pi has no approval prompts, and onboarding says so. "Scoped" takes sudo away: the agent runs root commands one at a time with `vibian sudo --why … -- cmd`, each is logged, and risky ones (disks, boot, sudoers, removing core packages, `rm -r` outside home) wait for you to run `sudo vibian approve <id>` |
| Vibian login banner | ✅ replaces Ubuntu's welcome and "minimized" text on tty1 and ssh, moved aside so removing `vibian-base` brings them back |
| 15 OS skills | ✅ packages, drivers-hardware, displays, networking, audio-bluetooth, power, users-security, services, recovery, computer-use, gaming, local-ai, dev-env, export-apply, desktop-playbooks ([`agent-kit`](agent-kit)) |
| Wifi in onboarding | ✅ NetworkManager; pick a network and type its password on the Network step |
| `vibian-system` MCP server | ✅ `vibian mcp`: doctor, hardware, ledger and snapshot tools, registered in all 7 harnesses' configs |
| Per-turn snapshots | ✅ `vibian hook` keeps a root+home pair for each agent turn that changed something (at most one per 5 min); hooks or plugins for all 7 harnesses |
| `vibian history` | ✅ ledger entries and snapshot pairs in one list; `vibian undo --pair <n>` for changes nobody logged |
| Export and apply | ✅ `vibian export` writes the setup as a git repo: packages, dotfiles, apt sources, services, and the ledger. A deny-list, a scrub and gitleaks keep secrets out. `--publish` puts it on GitHub. `vibian apply` hands it to your agent with a plan; `--scripts-only` replays it as-is |
| `vibian-computer` MCP server | ✅ `vibian mcp computer`: screenshot, windows, focus, click/drag/scroll, type, keys on Hyprland, Sway, niri, other wlroots, KDE and X11 (GNOME best effort), registered in all 7 harnesses. VM-tested on Sway (native and uinput input) and X11; Hyprland is checked read-only on real hardware; niri and KDE are untested |
| Desktop playbooks | 🚧 `vibian playbooks` lists them and the `desktop-playbooks` skill walks the agent through one: a short interview, a root script, a user script, and a verify script that must print `VERIFY OK`. ✅ Three, all from the Ubuntu archive. [Hyprland](playbooks/hyprland/PLAYBOOK.md) and [Sway](playbooks/sway/PLAYBOOK.md) each come with a tuigreet login, waybar, fuzzel and mako. [KDE Plasma](playbooks/kde-plasma/PLAYBOOK.md) 6.6 comes with SDDM on its Wayland greeter, Breeze dark or light in your accent colour, and a small app set. The VM test runs each playbook on every change, and each was booted to the desktop by hand ([Hyprland](playbooks/hyprland/screenshots/desktop.png), [Sway](playbooks/sway/screenshots/desktop.png), [KDE](playbooks/kde-plasma/screenshots/desktop.png)). The other 7 are next |
| Examples (Bloomberg desk, startup mission control, grandma mode) | ⏳ |

## Why not Omarchy?

Omarchy is great if you want DHH's desktop. Vibian is for people who want *their own*. It starts with no desktop at all, runs on Ubuntu LTS so NVIDIA cards and Secure Boot just work, snapshots `/home` as well as `/`, and lets you share whole machines rather than just themes. There's a longer comparison in the [spec](docs/SPEC.md#1-positioning).

## Origin

Vibian started as one laptop. A stock Ubuntu install was turned into a fully custom /g/-style anime rice in about a day, entirely by prompting coding agents. That build had a Windows-style desk, drag-to-edge tiling, a VPN widget, a local image-gen app and an embedded agent with computer use. A second agent then picked up where the first left off by reading its change log. Vibian turns that into an OS.

## License

[GPL-3.0](LICENSE). Agent harnesses are installed from their official sources at onboarding and are never bundled; several are proprietary.
