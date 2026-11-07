---
name: export-apply
description: "Share this machine's setup as a rice repo (`vibian export`) and rebuild someone else's rice here (`vibian apply`), adapting it to this hardware. Use when the user wants to share, back up or publish their setup or dotfiles, asks to install or copy someone's rice, theme or desktop from a GitHub repo, or when `vibian apply` hands you a RICE.md."
---

# Export and apply rices

A rice is a git repo:

| path | what |
|---|---|
| `vibian.yaml` | name, base, desktop playbooks, apt/flatpak/snap packages, enabled services, the `dotfiles` and `system` paths, the hardware it was made on |
| `dotfiles/` | files relative to home (`dotfiles/.config/hypr/…` → `~/.config/hypr/…`) |
| `system/` | absolute paths (`system/etc/…` → `/etc/…`): third-party apt sources and keys, custom units, files the ledger recorded |
| `scripts/*.sh` | optional extra steps, run in name order |
| `RICE.md` | what the setup is, and notes for the agent applying it |
| `ledger/ledger.md` | the scrubbed ledger: how the machine was built, step by step |
| `screenshots/` | what it looks like |

## Exporting
```bash
vibian export                      # → ~/vibian-rice (re-run to refresh; RICE.md, scripts/ and screenshots/ are kept)
vibian export ~/rices/mine --name mine --add ~/.local/bin/my-widget
vibian export --publish            # public GitHub repo with topic vibian-rice (needs gh login and gitleaks)
```
- It picks up the usual rice configs that exist (hypr, waybar, kitty, nvim, fish, starship, …), every home, `/etc` or `/usr/local` path in the ledger's `files`, and `--add` paths. Packages come from apt history after the install plus the ledger's `pkgs`, kept only if still installed by hand.
- **Secrets never leave.** The deny-list drops keys, `.env*`, ssh/gnupg, browser profiles, cloud and harness logins, shell history, VPN accounts and NetworkManager connections. Then a scrub redacts tokens, `password =`/`*_TOKEN=` values, private key blocks and 16-digit account numbers in the files that stay. Then gitleaks scans the whole export and refuses to commit if it finds anything.
- If gitleaks flags a line, fix the source file (move the secret to `~/.config/environment.d/` or a secret store, and reference it) rather than editing the export. Never commit around the check.
- After exporting, **write RICE.md's top section with the user**: what it's for, the look, key bindings worth knowing. Take a screenshot (`grim screenshots/desktop.png` on wlroots) if there isn't one. Show the user `git -C ~/vibian-rice show --stat` before `--publish`. Publishing is public: always ask first.

## Applying
Rices that ship with Vibian are applied by name (`vibian apply nhk`, `vibian apply tape`; `vibian apply` with no argument lists them). `vibian apply github:user/repo` clones the rice to `~/.local/share/vibian/rices/`, prints a plan (packages to install, files new and overwritten, services, scripts, the source hardware) and starts you with that plan. Then:
1. Read `RICE.md`, `vibian.yaml`, and skim `ledger/ledger.md` for gotchas the author hit.
2. `vibian snap "apply rice <name>"`.
3. **Packages.** `apt-get install` what's missing. A package missing from this release: look for the renamed one (`apt search`), a flatpak, or the author's ledger entry for where it came from. Add third-party sources from `system/etc/apt` only after telling the user which ones.
4. **Files.** New files: copy them in. Files the user already has: show the difference (`diff -u ~/.config/x dotfiles/.config/x`) and ask. Merge where you can (keep their keybinds, take the theme). Back up what you replace to `~/.local/share/vibian/rices/<name>.backup/`.
5. **Adapt to this machine.** The `hardware` list says what it was made on. Check `vibian doctor` and the `hardware` MCP tool for what's here, then fix:
   - monitor names, modes and scale (`hyprctl monitors`, `wlr-randr`, `swaymsg -t get_outputs`)
   - GPU env vars: drop NVIDIA-only ones (`LIBVA_DRIVER_NAME=nvidia`, `__GLX_VENDOR_LIBRARY_NAME`, `GBM_BACKEND`) on Intel/AMD, and add them on NVIDIA
   - keyboard layout, battery/backlight widgets on desktops, and hard-coded `/home/<author>` paths
   - `<redacted by vibian export>` markers: those were the author's secrets. Ask the user for theirs or drop the line
6. **Services.** `systemctl enable --now` the `services`, and `systemctl --user enable --now` the `user_services`.
7. **Scripts.** Read each `scripts/*.sh`, tell the user what it does, and run it only with their OK.
8. One `vibian log add "apply rice <name>" --pkgs +a,+b --files ~/.config/hypr,…`, with gotchas for anything you adapted. Tell the user how to log in to the new desktop and that `vibian undo <id>` takes it all back.

`vibian apply --dry-run` only prints the plan. `vibian apply --scripts-only` copies everything as-is without an agent: fine for identical machines, wrong for different monitors or GPUs.
