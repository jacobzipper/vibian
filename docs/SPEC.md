# Vibian: spec and plan (draft 1, 2026-11-05)

> A Linux that boots to a blank terminal. You pick a coding agent, and the agent builds the rest:
> your desktop, your theme, your weird Bloomberg terminal. Any of it can be undone.

Status: draft 1, from a 30-question design interview. Owner: Jacob Zipper (`jacobzipper`). License: GPL-3.0.

---

## 1. Positioning

### The incumbent: Omarchy
Facts below were checked on 2026-11-05 from omarchy.org, the official manual and the 4.5 release post.
- DHH's Arch + Hyprland distro. About **46k GitHub stars** (`omacom/omarchy`), **1.35M ISO downloads in year one**, and **$23.6M pledged** to the Omacom Foundation.
- **4.0 "Quattro" (Aug 2026)** rebranded it "Beautiful, fun & agentic Linux". It ships launchers for 23 agent CLIs (`omarchy default agent <name>`) and one **experimental** "Omarchy skill" for tweaking Hyprland, the bar and themes.
- **4.5 (Nov 4, 2026)** added a floating "traditional desktop" mode.
- Snapshots are made on updates, are bootable from Limine, and **do not cover `/home` or `~/.config`**, which is where agents do most of their edits.
- No change ledger. Plugins run unsandboxed. Agents start in their don't-ask modes.

We will launch into a world where Omarchy exists, and HN's first comment will be "how is this different from Omarchy?". The answer has to fit in one line:

**Omarchy is chef's choice (omakase): DHH's desktop, with agents built in. Vibian is an empty kitchen and your own chef: no desktop until you describe one.**

| | Omarchy 4.5 | Vibian |
|---|---|---|
| First boot | DHH's Hyprland desktop | A terminal, then onboarding, then your agent interviews you |
| Desktop | One opinionated Hyprland (+ floating mode) | 10 playbooks (Hyprland, KDE, GNOME, niri, Sway, i3, XFCE, COSMIC, Quickshell, headless) or anything you describe |
| Base | Arch, rolling | Ubuntu 26.04 LTS: `ubuntu-drivers`, signed NVIDIA modules, Secure Boot friendly |
| What the agent knows | One experimental skill about Omarchy config | AGENTS.md system brief + 14 OS skills (drivers, network, power, recovery…) + computer use |
| Undo | Root snapshots on updates | btrfs snapshots of `/` **and** `/home` before every agent session and every package change, plus `vibian undo` |
| Memory across agents | none | **The ledger**: every change, file, snapshot and gotcha, in markdown any harness can read |
| Sharing | Themes and plugins (git repos) | **Whole machines**: `vibian export`, then `vibian apply github:you/rice`, replayed by *their* agent on *their* hardware |

The proof that this works already exists. Jacob's laptop went from a stock Ubuntu install to an NHK-themed rice in about a day, entirely by prompting, and became **the flagship case study**. That day included a Windows-style desk, drag-to-edge tiling, a Mullvad widget, a ComfyUI "Imagine" app and an embedded OpenClaw agent with computer use. The ledger is why a second agent (OpenClaw) could take over from the first (a remote Claude Code session).

### Audience
Devs who already use coding agents. Secondary audiences: ricers, and people curious about Linux who come from macOS.

### Tone
Playful-serious. Serious engineering (snapshots, LTS, GPL) and a playful surface (the NHK rice, a grandma mode, the "vibe" name).

---

## 2. Decisions locked by the interview

| Area | Decision |
|---|---|
| Name | **Vibian**  |
| Deliverable | **Our own ISO from day one** |
| Base | Ubuntu Server 26.04 LTS. Canonical's IP policy means a modified ISO must be fully rebranded as Vibian |
| First boot | Autologin on tty1, straight into the onboarding TUI |
| Harnesses (v1) | Claude Code, Codex CLI, Gemini CLI, opencode, OpenClaw, pi, Cursor CLI. Local Ollama path flagged *experimental* |
| Hardware (v1) | NVIDIA hybrid laptop, NVIDIA desktop, AMD GPU, Intel iGPU, VMs (QEMU/virt-manager, VirtualBox) |
| Privileges | Picked at onboarding. Default: full root + snapshots |
| Rollback | btrfs + snapper, grub-btrfs boot-to-snapshot |
| Ledger | Core. Markdown, with a small CLI to append and search |
| Agent format | Harness-agnostic: AGENTS.md + Agent Skills (SKILL.md) + MCP, with a thin adapter per harness |
| Base skills | All 14 (list in section 4.4) |
| Computer use | Core MCP server, Wayland (Hyprland/Sway/niri/KDE) + X11 |
| Ask overlay / assistant panel | Optional component offered at onboarding |
| Remote access (phone/Discord/TG) | Playbook, not core |
| Export / replay a whole machine | **Headline feature** |
| Telemetry | None |
| Playbooks | Markdown guide + tested reference scripts |
| Desktop playbooks (v1) | Hyprland, KDE Plasma, GNOME, niri, Sway, i3, XFCE, COSMIC, Quickshell widget desk, headless |
| Desktop onboarding | Interview + gallery, with a skip |
| NHK | Flagship case study + an installable rice |
| README | Short VM-recorded videos/GIFs + the exact prompts |
| Actions | On, hardened |
| Testing | QEMU/KVM VM matrix, agent-driven GUI tests, USB-stick installs for real NVIDIA hardware |

---

## 3. What the user sees

```
[USB boot] → Vibian installer (autoinstall; asks disk, encryption, user, wifi)
   → reboot → GRUB (Vibian theme, snapshot submenu) → Plymouth
   → tty1 autologin → `vibian onboard` TUI:
        1. Network        (wifi picker, or skip if wired)
        2. Hardware check (GPU found → driver installed? Secure Boot? battery? → green ticks)
        3. Pick your agent (7 harnesses + "local (experimental)")
        4. Install + log in (official installer; device-code or browser-on-phone login)
        5. Agent powers   (root+snapshots [default] / confirm-destructive / scoped sudo)
        6. Extras         (Ask overlay + assistant panel? computer use?)
        7. Hand-off       → launches the agent with the first-run prompt
   → the agent: "Hi. I run this machine. Want a desktop? Here's the gallery…"
        → interview (or "skip, I'm a power user")
        → picks a playbook, adapts it, builds it, screenshots it, logs it to the ledger
   → reboot into your desktop. Your agent hotkey works from now on.
```

The demo moment is the first 90 seconds after hand-off: a black terminal turns into a themed desktop with nothing but prompts.

---

## 4. Architecture

### 4.1 ISO build
- **Input:** the official `ubuntu-26.04.x-live-server-amd64.iso`.
- **Remaster** (xorriso + squashfs-tools, no loop mounts, so it can run in a node container or on CI):
  - `autoinstall.yaml` as an interactive autoinstall: storage, identity and network are asked; everything else is preset.
  - Preset includes: `drivers: install: true`, `codecs: install: true`, `oem: install: auto`.
  - A local apt pool on the ISO with the `vibian-*` .debs. Late-commands install them into the target, and they are also published to a signed apt repo so updates work.
  - Rebranded GRUB menu, splash and isolinux.
- **Packages:**

  | package | contents |
  |---|---|
  | `vibian-base` | the CLI, ledger, snapper configs, apt hooks, onboarding TUI, tty1 autologin drop-in (removed after onboarding) |
  | `vibian-branding` | os-release (`ID=vibian`, `ID_LIKE=ubuntu debian`), Plymouth, GRUB theme, MOTD. Replaces Ubuntu marks, per Canonical's IP policy |
  | `vibian-agent-kit` | AGENTS.md, skills, MCP servers, harness adapters |
  | `vibian-playbooks` | desktop playbooks + reference scripts |
  | `vibian-extras-ask` | optional: Ask overlay + assistant panel |

- **Builds:** reproducible from `iso/remaster.sh` on any Linux box (no KVM or root mounts needed), and in hardened GitHub Actions.

### 4.2 Disk layout and snapshots
- btrfs subvolumes:
  - `@` → `/`
  - `@home` → `/home`
  - `@snapshots` → `/.snapshots`
  - `@home_snapshots`
  - `@var_log`, `@var_cache`, `@var_lib_docker`, `@var_lib_libvirt`: excluded from root snapshots, so rollbacks don't eat logs or VMs
- Optional LUKS2 (asked at install).
- snapper configs `root` and `home`.
  - Timeline snapshots: hourly ×6, daily ×7, weekly ×4.
  - The apt hook `/etc/apt/apt.conf.d/80vibian-snapper` takes a pre/post pair around every dpkg run.
- `grub-btrfs` + `grub-btrfsd` put a "Vibian snapshots" submenu in GRUB. Booting a snapshot is read-only, and `vibian undo --to <id>` makes it permanent.
- **Agent snapshots** happen three ways:
  1. `vibian agent` (the launcher every harness goes through) takes a pre snapshot at session start and a post snapshot at exit.
  2. Every harness also runs `vibian hook turn-start` / `turn-end` around each user turn (hooks, plugins or extensions that `vibian agent` writes; Codex reads managed hooks from `/etc/codex/requirements.toml`). A turn that changed nothing outside the harness's own state keeps no snapshot: its post is deleted and the pre carries over to the next turn. Kept pairs are at least 5 minutes apart, so snapshots don't pile up. State is in `~/.local/state/vibian/turn.json`, and the session end closes whatever is open.
  3. Skills tell the agent to run `vibian snap "<why>"` before anything risky.
- **Spike S1 (first):** subiquity/curtin may not create btrfs subvolumes natively. Option (a) is a curtin storage config plus late-commands that create the subvolumes and move the data. Option (b) is our own tiny installer step that runs before subiquity. Decide after a VM test. **Result (2026-11-05): option (a).** `iso/btrfs-layout.sh` snapshots the fresh install into `@`, splits out the other subvolumes with reflink copies, rewrites fstab and the ESP GRUB stub, and creates the snapper configs. A KVM install from the remastered ISO passes `tests/vm/headless-install.sh`: the system boots from `/@`, and apt makes pre/post snapshots. `grub-btrfs` is in neither Ubuntu 26.04 nor Debian, so `packages/grub-btrfs/build-deb.sh` builds a .deb from a pinned upstream release. The ISO carries it in `/vibian/pool`, and the VM test checks that `grub-btrfsd` runs and that the snapshot submenu fills in. **Booting a snapshot** works the same way on 26.04, which uses dracut (not initramfs-tools): `vibian-base` adds dracut's `overlayfs` module and grub-btrfs adds `rd.overlay=1` to snapshot entries, so the read-only snapshot boots with a tmpfs upper layer. Drop-ins skip snapper-boot, the snapper timers and GRUB's boot-success units on such boots, so the system reports `running`. The VM test takes a snapshot, breaks the system, picks the snapshot in the GRUB menu with arrow keys via the QEMU monitor, and checks the pre-break state is back.

### 4.3 The `vibian` CLI
One Go binary (also hosting the Bubble Tea onboarding TUI, so the demo looks great).

```
vibian onboard                     # the first-boot TUI; re-runnable
vibian agent [run|set <harness>|list|login]   # launcher: snapshots, adapters, env
vibian log add "summary" [--files …] [--pkgs …] [--undo "…"] [--gotcha "…"]
vibian log [search <q>|show <id>|tail]
vibian snap "why"                  # root+home snapshot pair, recorded in the ledger
vibian undo [<ledger-id>|--last|--pair <n>|--to <snapshot>]   # snapper undochange / rollback
vibian history [-n N] [--json]     # ledger entries joined to their snapshots, plus unlogged pairs
vibian doctor                      # GPU/driver/SB/firmware/network/audio/snapper health, JSON too
vibian export [DIR] [--add PATH] [--publish]   # machine → git repo (see 4.7)
vibian apply <github:user/repo|path> [--dry-run|--scripts-only]   # hand a rice to your agent to replay
vibian playbooks [list|show <id>]
vibian mcp [computer] [--agent X]  # MCP servers on stdio: vibian-system, vibian-computer
vibian powers [root|confirm|scoped]    # change the privilege mode later
vibian sudo --why "…" -- cmd …     # scoped powers: one root command, logged; risky ones need…
vibian approve [<id>|--deny <id>]  # …the owner's OK (their own sudo password)
```

### 4.4 Agent kit (harness-agnostic)
- **System brief:**
  - `/usr/share/vibian/agent/AGENTS.md`: what Vibian is, the rules (log everything, snapshot before risk, screenshot before clicking, never touch the user's windows while they type), where things live, and how to undo.
  - It is linked into each harness's *user-level* instructions, so the agent knows it's the OS agent from any directory.
- **Skills** (Agent Skills / SKILL.md format; each skill = SKILL.md + scripts/):

  | # | skill | covers |
  |---|---|---|
  | 1 | `packages` | apt, flatpak, PPAs, unattended-upgrades, DNS-failure fallback |
  | 2 | `drivers-hardware` | `ubuntu-drivers`, NVIDIA (PRIME offload, power, Secure Boot MOK), firmware (fwupd), fingerprint |
  | 3 | `displays` | scaling, HiDPI, multi-monitor, refresh rate, per-compositor syntax |
  | 4 | `networking` | NetworkManager, wifi, VPN (WireGuard/Tailscale/Mullvad), DNS, ufw |
  | 5 | `audio-bluetooth` | PipeWire/WirePlumber, BlueZ |
  | 6 | `power` | power-profiles-daemon/TLP, suspend, lid, idle, vendor tools (Legion, ThinkPad, ASUS) |
  | 7 | `users-security` | users, groups, SSH keys, secret storage, sudo modes |
  | 8 | `services` | systemd units, user units, timers, journal |
  | 9 | `recovery` | snapper, rollback, GRUB/initramfs repair, chroot from a live USB, the ledger |
  | 10 | `computer-use` | how to use the MCP below safely |
  | 11 | `gaming` | Steam (deb), Proton, gamemode, MangoHud, controllers |
  | 12 | `local-ai` | Ollama, ComfyUI, sharing the GPU between them |
  | 13 | `dev-env` | Docker, mise, languages, editors |
  | 14 | `export-apply` | dotfiles tracking, export, and replaying someone else's rice |

  Plus `desktop-playbooks`, a meta-skill that knows the gallery and how to interview the user.
- **MCP servers:**
  - `vibian-computer` (`vibian mcp computer`): a generalisation of the laptop's `nhk-computer`. Tools: screenshot (region/scale, returned as an image plus a note mapping image pixels to screen coordinates), windows, focus_window, move_mouse, click/drag/scroll, type_text, key. Backends: Hyprland (hyprctl, grim, wtype), Sway (its own seat pointer), niri, other wlroots (grim, wtype), KDE (spectacle, KWin scripts), X11 (maim, wmctrl, xdotool), GNOME best effort. Where the desktop has no pointer or keyboard injection, it creates a uinput tablet and keyboard; a udev `uaccess` rule gives `/dev/uinput` to the user at the seat. `vibian agent` registers it next to `vibian-system`.
    - Backends: wlroots (Hyprland/Sway/niri) via grim + virtual pointer + wtype; KDE via the KWin scripting/portal screenshot; X11 via xdotool + maim; a uinput fallback for buttons and the wheel.
    - GNOME Wayland is the hard one: screenshots go through the portal, input through uinput (ydotool-style).
  - `vibian-system` (small, `vibian mcp`): `doctor` JSON, the hardware inventory, ledger search/tail/show, plus `snapshot` and `ledger_add` so the three rules work as tool calls. Structured context even weak local models can use. `vibian agent` registers it in every harness (opencode reads it from `/etc/opencode/opencode.json`).
- **Harness adapters** (installed by `vibian agent set <h>`). Verified against each harness's docs and source on 2026-11-05; details and sources are in `docs/research/harness-adapters.md`.

  | harness | licence | user instructions | skills dir | MCP | per-turn hooks (pre → post) | headless login |
  |---|---|---|---|---|---|---|
  | Claude Code | proprietary | `~/.claude/CLAUDE.md` → `@/usr/share/vibian/agent/AGENTS.md` | `~/.claude/skills` | `claude mcp add -s user` | `UserPromptSubmit` → `Stop` | `claude setup-token` / API key |
  | Codex CLI | Apache-2.0 | `~/.codex/AGENTS.md` | `~/.agents/skills` | `codex mcp add` | `UserPromptSubmit` → `Stop` (managed, in `/etc/codex/requirements.toml`) | `codex login --device-auth` / API key |
  | Gemini CLI | Apache-2.0 | `~/.gemini/GEMINI.md` + `context.fileName` | `~/.agents/skills` | `gemini mcp add --scope user` | `BeforeAgent` → `AfterAgent` (trusted folders only) | `NO_BROWSER=true` paste-code / API key |
  | opencode (v2) | MIT | `~/.config/opencode/AGENTS.md` | `~/.agents/skills` | `opencode mcp add --global` | plugin `session.hook("prompt")` → `session.execution.*` event (`~/.config/opencode/plugins/vibian.ts`) | `opencode auth login` |
  | OpenClaw | MIT | `~/.openclaw/workspace/AGENTS.md` | `~/.agents/skills` | `openclaw mcp add` | plugin `before_agent_run` → `agent_end` (`/usr/lib/vibian/openclaw`, linked once, needs `allowConversationAccess`) | paste-code / setup-token / API key |
  | pi | MIT | `~/.pi/agent/AGENTS.md` | `~/.agents/skills` | `pi mcp add` | extension `before_agent_start` (user messages) → `agent_settled` | `/login` paste-back / API key |
  | Cursor CLI | proprietary | none on disk → launcher passes the brief | `~/.agents/skills` | `~/.cursor/mcp.json` | `beforeSubmitPrompt` → `stop` (CLI coverage partial) | `NO_OPEN_BROWSER=1 agent login` / API key |
  | local (exp.) | — | via opencode or pi + Ollama | same | same | same | none |

  Rules that fall out of this:
  - The agent always runs as the **normal user with sudo, never as root**. Claude Code refuses its skip-permissions mode as root, and it's the right model anyway.
  - Skills are installed to `~/.agents/skills` (6 harnesses) plus `~/.claude/skills`.
  - Codex snapshot hooks are shipped as *managed* hooks, so they run without an interactive trust prompt.
  - opencode has no post-turn hook, so `vibian agent` takes the session-end snapshot.

- **Proprietary harnesses are never put in the ISO.** Onboarding runs each one's official installer, then its login, using device codes or "open this URL on your phone", since there's no browser yet.

### 4.5 Privilege modes (picked at onboarding, default `root`)
- `root`: NOPASSWD sudo for the user, and the agent runs in its harness's auto mode. The safety net is snapshots plus the ledger.
- `confirm`: sudo works, but a polkit-style desktop or terminal prompt fires for a destructive list: disk tools, `rm -rf` outside home, purging core packages, bootloader and fstab edits.
- `scoped`: the agent gets `vibian sudo --why "<reason>" -- <cmd>` instead of sudo. Everything is logged to the ledger, and the risky list needs a human OK.
  - The only sudoers rule is `user ALL=(root) NOPASSWD: /usr/bin/vibian sudo-root *`; the root side accepts exactly `sudo-root --why R --agent A -- cmd…`.
  - Risky: disk and partition tools, bootloader and Secure Boot tools, reboot/poweroff, user and sudo changes (including `vibian` itself), `btrfs`/`snapper` writes, `rm -r` or recursive chmod/chown outside home and the temp dirs, any argument under fstab, crypttab, GRUB, /boot, sudoers, shadow, PAM, snapper or Vibian's own state, block devices, apt/dpkg removals that take an essential/required/important or Vibian-core package or more than 25 packages, `--force`, and anything that runs code it can't see (shells, interpreters, env, xargs, `find -exec`) or a program outside the root-owned system dirs.
  - A risky command exits 3 and leaves a request in /run/vibian/sudo. The owner runs `sudo vibian approve <id>` with their own password, and the OK is good for one run of that exact command, from that directory, within 30 minutes. Approved runs get a snapshot pair.
  - This catches an agent's mistakes. It is not a sandbox: the agent still runs as the owner's user.

### 4.6 The ledger
- **Storage:** `/var/lib/vibian/ledger/` as a git repo, with `ledger.md` (append-only entries) and `entries/<id>.md` for long ones. The markdown is the source of truth: any harness or human can `grep` it.
- **Entry format:**
  ```markdown
  ## L-0142 · 2026-11-05 14:02 · claude-code · "Hyprland title bars"
  - snapshots: root 311→312, home 88→89
  - packages: +hyprland-plugins-dev
  - files: ~/.config/hypr/UserConfigs/nhk_bars.lua (new), …
  - undo: `vibian undo L-0142` (or: remove line 12 of user_settings.lua; hyprctl reload)
  - gotcha: hyprbars must be rebuilt per Hyprland commit; apt hook does it
  ```
- The skills require a ledger entry for every change. At session end, `vibian agent` warns if snapshots changed with no matching entry, and auto-writes a stub entry from the snapshot diff.
- This is the laptop lesson made into a product: the ledger is what let a second agent take over from the first.

### 4.7 Export and apply ("a rice you install with one prompt")
- `vibian export [DIR]` (default `~/vibian-rice`) writes a git repo. Re-running it refreshes the generated parts and keeps `RICE.md`, `scripts/` and `screenshots/`:
  - `vibian.yaml`: name, base, playbook ids (detected from installed desktops), apt/flatpak/snap packages, enabled services that aren't on by default, the dotfile and system paths, and the hardware it was made on. It's a flat YAML subset that the CLI reads and writes without a library.
    - **Apt packages:** apt history after the install plus the ledger's `pkgs`, kept only if they're still marked manual.
  - `dotfiles/`: the usual rice configs that exist (compositor, bar, launcher, notifications, terminal, shell, prompt, editor, GTK/Qt theme, user units), every home path in the ledger's `files`, and `--add` paths. Files over 1 MiB and binaries other than images are skipped.
  - `system/`: third-party apt sources and their keys, custom unit files, and the ledger's `/etc` and `/usr/local` paths
  - `scripts/`
  - `RICE.md`: a prompt-ready narrative for an agent: what this setup is and how to rebuild it, adapting to different monitors and GPUs. The export writes the agent steps, the hardware and the ledger summaries; the user and their agent write the top.
  - `ledger/`: scrubbed
  - `screenshots/`: `grim` takes one on wlroots if there isn't one
- Before anything is committed, three layers run:
  1. A **deny-list** drops paths: keys, `.env*`, ssh/gnupg, browser profiles, cloud and harness logins, shell history, VPN accounts, NetworkManager connections.
  2. A **scrub** redacts tokens, `password =`/`*_TOKEN=` values, private key blocks and 16-digit (Mullvad-style) account numbers in the files that stay.
  3. **gitleaks** scans the result and blocks the commit on any finding.
  - `--publish` requires gitleaks. It asks, then `gh repo create --public` with topic `vibian-rice`.
- `vibian apply github:jacobzipper/nhk-rice` clones it to `~/.local/share/vibian/rices/` and prints a plan: packages to install, files new and overwritten, services, scripts, and the source hardware. Then it starts *your* agent with the plan and the `export-apply` skill, so the replay adapts to *your* hardware and asks before overwriting your files.
  - `--dry-run` only prints the plan.
  - `--scripts-only` is the deterministic mode for identical machines. It copies system files and dotfiles, installs packages, enables services and runs `scripts/*.sh`, all between one snapshot pair with one ledger entry.
- **Gallery:** the website's gallery is a static page built from GitHub topic `vibian-rice`. No backend needed. This is the growth loop: every shared rice is an ad for Vibian.

### 4.8 Desktop playbooks
- Each playbook is `playbooks/<id>/PLAYBOOK.md` (what it is, who it's for, the interview questions to ask, the adaptation knobs, steps, keys, undo, gotchas), plus `scripts/` and `screenshots/`.
  - Frontmatter: `id`, `name`, `description`, `status` (`tested`: the VM test runs it; `untested`: written, not yet booted).
  - `scripts/system.sh` (root: packages, login screen), `scripts/user.sh` (dotfiles; refuses root; keeps a first-time `.vibian-bak` of any file it replaces) and `scripts/verify.sh` (prints `VERIFY OK` or `VERIFY FAILED`). All idempotent. The knobs are `VIBIAN_*` environment variables, so one script serves every interview answer.
  - They ship in `vibian-base` under `/usr/share/vibian/playbooks/`. `vibian playbooks` lists them; `vibian playbooks show <id>` prints one.
  - The `desktop-playbooks` skill: pick one with the user, ask the interview in one message, run the steps by `vibian powers`, verify, log, then check the result with a `vibian-computer` screenshot.
  - Archive packages by default. A PPA (e.g. newer Hyprland) is opt-in and logged with the reason.
  - A desktop missing from the archive (niri in 26.04) is built from its upstream release. The source and vendored crates are pinned by sha256, the build runs offline with the archive's Rust, it installs to `/usr/local`, and the build dependencies are removed afterwards.
  - One login screen at a time: each `system.sh` disables the other display managers (greetd, sddm, gdm3, lightdm) before enabling its own. The tiling playbooks also turn off Ubuntu's global user units for waybar, mako, hypridle and hyprpolkitagent, which would otherwise start inside Plasma or GNOME; their configs start these themselves. SDDM and GDM preselect the playbook's session (SDDM's state file, AccountsService).
- v1 set: hyprland · kde-plasma · gnome · niri · sway · i3 · xfce · cosmic · quickshell-desk (the generalised NHK desk) · headless.
- Themes are rices on top of playbooks. **`nhk`** is the first one: the flagship case study, scrubbed of personal bits.
- **Remote-access playbook:** talk to your OS agent from your phone (OpenClaw channels, Tailscale ssh, a web UI).

### 4.9 Optional component: Ask overlay and assistant panel
A generalisation of the laptop's SUPER+/ "Ask N.H.K." and `nhk-claw`: a hotkey dialog, requests that run as background tasks, a desktop notification on finish, and a panel listing conversations. Harness-agnostic: it talks to `vibian ask`, which runs the agent's one-shot mode in the background between a snapshot pair (`vibian asks` lists the requests and answers). Built as a small GTK4/layer-shell app, so it works on any compositor rather than only Quickshell. **Done (2026-11-06)** as the `ask-overlay` playbook, an extra for any desktop playbook; the request list in the overlay is the panel.

---

## 5. Repo layout (`jacobzipper/vibian`, GPL-3.0)
```
cmd/vibian/            Go CLI entry points (onboard, agent, powers, snap, log, undo, doctor)
internal/              onboard (Bubble Tea TUI), harness (the 7 adapters: install, login,
                       instructions, skills, flags), ledger, snap, doctor, sys
agent-kit/AGENTS.md    system brief
agent-kit/skills/<14 skills>/SKILL.md + scripts/
agent-kit/mcp/computer/  agent-kit/mcp/system/
playbooks/<10 desktops>/   rices/nhk/
iso/                   build.sh (remaster), autoinstall.yaml, btrfs-layout.sh, branding/
packages/              vibian-base, grub-btrfs (build-deb.sh each)
tests/vm/              QEMU tests: headless-install.sh (autoinstall → first boot → CLI →
                       snapshot boot → rollback), drive.sh (keyboard walkthroughs)
examples/<6 examples>/ prompts + recordings + exported rice
docs/                  SPEC.md (this), CONTRIBUTING (how to write a playbook), case-study-nhk.md
.github/               hardened workflows + CODEOWNERS
```

**Actions hardening** (the September worm is why):
- Default `permissions: {}` at the top of every workflow, granting `contents: read` per job.
- Every action pinned by commit SHA.
- **No repository secrets at all.** Releases are signed and uploaded by a maintainer, outside Actions.
- `CODEOWNERS` requires Jacob's review on `.github/**`. No `pull_request_target`.
- Dependabot only for SHA bumps.

---

## 6. README examples (headline 3, flagship, gallery)
Jacob picked 6. Proposed split:

| slot | example | why |
|---|---|---|
| **Flagship case study** | /g/ NHK rice in a day | It really happened, with receipts. Its own section, plus the installable `nhk` rice |
| Headline 1 | **Bloomberg-terminal desk** | Biggest visual "wait, what": amber on black, tiled tickers, news and charts, keyboard-driven |
| Headline 2 | **Startup mission control** | HN's own audience: chat apps, CI/deploys, uptime, revenue, health metrics on one screen |
| Headline 3 | **Grandma / kid mode** | The emotional, shareable one: big buttons, locked down, the agent maintains it remotely |
| Gallery | Arcade / couch-gaming kiosk | Boots straight into a controller-driven game shelf |
| Gallery | Voice-first desktop | Local whisper wake word that drives the agent |

Every example comes with the **exact prompt(s)**, a 15–30 s clip recorded in a VM (sped up, with the prompt overlaid), and an exported rice you can `vibian apply`.

---

## 7. Testing
- **VM matrix** (QEMU/KVM + libvirt): UEFI and BIOS, Secure Boot on (OVMF secboot) and off, virtio-gpu (virgl) for Wayland compositors, and a VirtualBox smoke run.
- **Automated smoke test** (`tests/vm/run.sh`):
  - Boot the ISO headless with a test autoinstall seed and wait for first boot.
  - Assert: the snapper configs, the grub-btrfs submenu, `vibian doctor` passes, onboarding starts on tty1, and a `vibian undo` round trip works.
  - Screenshots come from QMP `screendump`.
- **Agent-driven tests:** an agent with the `vibian-computer` MCP drives the VM window. It runs onboarding, picks a harness and runs each playbook, and records clips for the README.
- **Real hardware:** VMs can't exercise GPU drivers, so the NVIDIA hybrid and desktop paths are tested with USB-stick installs on real machines.

## 8. Roadmap (aggressive; adjust after the spikes)
| milestone | target | done when |
|---|---|---|
| **M0 spikes** | days 1–3 | Repo up. S1 btrfs layout via autoinstall decided. S2 remaster pipeline boots a rebranded ISO in a VM. S3 the 7 adapter paths are verified. S4 libvirt works on the laptop and OpenClaw can drive a VM window |
| **M1 boots to agent** | week 1–2 | ISO → install → tty1 onboarding → harness installed and logged in → agent with AGENTS.md + 4 core skills (packages, drivers, recovery, displays) + `vibian log/snap/undo/doctor` |
| **M2 kit complete** | week 2–3 | All 14 skills, `vibian-computer` on wlroots/KDE/X11 (GNOME best-effort), privilege modes, `export`/`apply` + secret scrub |
| **M3 desktops** | week 3–4 | 10 playbooks with tested scripts, the `nhk` rice, the Ask overlay, the gallery interview at onboarding |
| **M4 show** | week 4–5 | 6 examples recorded, README, website + gallery, the NHK case study, a private beta (~20 agent-using devs) |
| **Launch** | week 6 | See section 9 |

---

## 9. Launch
Built in public. The launch plan is to show demo videos of real prompts, have a gallery of shareable rices, then post a Show HN and on Reddit once the 3 headline examples and the NHK case study are recorded.

## 10. Risks
- **Omarchy moves fast** (23 agents, 4.5 shipped yesterday). If they add a ledger and `/home` snapshots, our edge narrows to blank canvas + Ubuntu/NVIDIA + whole-machine sharing. So ship fast and own the "build your own" and sharing story.
- **btrfs via subiquity** may fight us (spike S1). Fallback: our own install step.
- **GNOME Wayland computer use** is restricted. Ship it best-effort and document it.
- **Harness churn:** 7 CLIs change paths and flags monthly. Adapters are data in `internal/harness/harness.go`. Keep them tiny and add a CI check that installs each one weekly in a VM (the manual version of that check passed on 2026-11-06).
- **Agent logins with no browser** at first boot: device codes work for most; worst case, a phone QR code.

## 11. Resolved after the interview
- Built in public from day one.
- README split as in section 6.
- Mascot: a **sphinx cat**.
- Pace: quality first, but no idle time.

Sources: omarchy.org (home, manual/ai, news 4.5); codetocloud.io Omarchy 4 review; heise.de on the Omacom Foundation; gradually.ai / skila.ai OpenClaw stats; GitHub API star counts (2026-11-05); dev.to Show HN and AFFiNE guides; boilingsteam.com / The Register on CachyOS; lavx on Bazzite growth.
