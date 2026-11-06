---
name: dev-env
description: "Set up programming on Vibian: Docker and Podman, language toolchains (Python, Node, Go, Rust, others through mise), editors (VS Code, Neovim, Zed, JetBrains), git and GitHub, and containers for other distros (distrobox). Use when the user wants to code, install a language/runtime/SDK, run containers, or set up git and SSH keys for GitHub."
---

# Dev environment on Vibian

Toolchains go in the user's home (no sudo). The system Python and Node belong to Ubuntu: never `sudo pip install` or `sudo npm install -g`.

## Languages
| language | default | notes |
|---|---|---|
| Python | `python3` (3.14) + `python3-venv`, `pipx` from the archive | PEP 668 blocks pip outside a venv. Use venvs, `pipx` for CLI tools, or `uv` (upstream: `curl -LsSf https://astral.sh/uv/install.sh \| sh`) |
| Node | archive `nodejs`/`npm` are older. For current versions use mise | for `npm i -g` without sudo: `npm config set prefix ~/.local` |
| Go | `golang-go` (1.26) | `GOPATH` defaults to `~/go`; add `~/go/bin` to PATH |
| Rust | `rustup` (archive) → `rustup default stable` | toolchains in `~/.rustup`, binaries in `~/.cargo/bin` |
| many versions / per-project | **mise** (not in the archive): `curl https://mise.run \| sh`, then `echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc` | `mise use -g node@lts python@3.13`; per project: `mise use node@22` writes `mise.toml` |
Build basics: `build-essential pkg-config`. `direnv` loads per-project env (`.envrc`).

## Containers
**Docker** from the archive (fine for most) or Docker's own repo (newest; see `packages` for the repo pattern):
```bash
sudo apt-get install -y docker.io docker-compose-v2 docker-buildx
sudo usermod -aG docker "$USER"     # root-equivalent: say so, takes effect at next login
```
- `/var/lib/docker` is its own subvolume: rollbacks and snapshots don't touch images or volumes, and `vibian undo` doesn't bring deleted ones back.
- Check the storage driver with `docker info | grep -i 'storage driver'`.
- Publish ports to `127.0.0.1:` unless the user wants them reachable from the network. Docker's published ports go around ufw.

**Podman** (`podman`, rootless, no daemon) is the alternative when the user doesn't want a root-equivalent group.
**distrobox** (`distrobox`) gives a mutable Arch/Fedora/etc. userland sharing `$HOME`, for software only packaged elsewhere (AUR, COPR): `distrobox create -n arch -i archlinux && distrobox enter arch`.

## NVIDIA in containers
`nvidia-container-toolkit` comes from NVIDIA's repo; then `sudo nvidia-ctk runtime configure --runtime=docker && sudo systemctl restart docker`. Test with `docker run --rm --gpus all ubuntu nvidia-smi`.

## Editors
| editor | install |
|---|---|
| VS Code | Microsoft's apt repo (packages.microsoft.com, package `code`); the snap also works |
| Neovim | `neovim` from the archive; for the newest, the upstream tarball in `~/.local` |
| Zed | `curl -f https://zed.dev/install.sh \| sh` (installs to `~/.local`) |
| JetBrains | JetBrains Toolbox (tarball in `~/.local`) |
| Cursor | the AppImage or .deb from cursor.com (the `cursor-agent` CLI is separate) |
GUI editors need a desktop: see the desktop playbooks.

## Git and GitHub
```bash
git config --global user.name "<Name>"; git config --global user.email "<email>"
git config --global init.defaultBranch main
ssh-keygen -t ed25519 -C "<email>"; cat ~/.ssh/id_ed25519.pub      # the user adds it on GitHub
gh auth login                                                       # the user does the browser/device flow
```
The archive `gh` is old. GitHub's apt repo (cli.github.com) has the current one. `git-lfs` comes from the archive.

## Ledger
Log repos added, groups joined (`docker`) and services enabled. Per-project installs (venvs, `node_modules`) don't need entries.
