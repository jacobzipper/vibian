---
id: headless
name: Headless
description: No desktop. The machine boots to the text console and is set up to be reached over the network, like a home server, a box in a cupboard or an old laptop with its lid shut. It gets SSH (keys or passwords), a firewall, name.local on the network, no sleep, automatic security updates, and tmux so the agent keeps working when the connection drops.
status: tested
---

# Headless

**What it is:** Vibian already boots to a terminal. This playbook makes that the plan rather than a stop on the way to a desktop:
- **No login screen.** It boots to the text console (`multi-user.target`), and any desktop's login screen is turned off. Desktops already installed stay installed, so one is a playbook away again.
- **SSH**, with root logins off. Passwords work until the user adds a key, and then they can be turned off.
- **A firewall** (ufw): everything incoming is blocked except SSH and mDNS.
- **`<hostname>.local`** on the home network (avahi), so `ssh me@homebox.local` works without knowing the IP address.
- **Never sleeps**, even with the lid shut.
- **Automatic security updates** (unattended-upgrades), with an optional nightly reboot when an update needs one.
- **tmux**, configured, and optionally started on every SSH login. The agent runs inside it and carries on if the phone or laptop on the other end disconnects.

`vibian agent` works the same over SSH as at the console: the user connects and talks to the agent there.

**Who it's for:** a home server (files, media, backups, Home Assistant, game servers), a machine that runs local AI models for other computers, a build box, or an old laptop given a second life. It also suits anyone who wants only a terminal. For talking to the agent from a phone (Discord, Telegram, a web page), see the remote-access playbook once it exists; this one is SSH only.

**Needs:** nothing beyond the base install. It works on any hardware, including machines with no screen at all after the first setup.

## Interview
Ask only what the user hasn't already said, in one short message:
1. Which computer will they connect from? If they have an SSH key there, ask for the **public** key, the one line from `~/.ssh/id_ed25519.pub`. Never ask for the private key. If they don't have one, they can make one with `ssh-keygen -t ed25519`. (`VIBIAN_SSH_KEY`)
2. Once their key works, should passwords be turned off for SSH? This is safer if the machine is reachable from the internet. (`VIBIAN_SSH_PASSWORDS`)
3. Should every SSH login open the same tmux session, so work carries on after a disconnect? (`VIBIAN_TMUX_ON_SSH`)
4. Is a reboot at night OK when an update needs one, and at what time? (`VIBIAN_AUTO_REBOOT`)

## Knobs
`VIBIAN_SSH_KEY`, `VIBIAN_TMUX_ON_SSH` and `VIBIAN_ACCENT` go to `scripts/user.sh`; the rest go to `scripts/system.sh`.

| knob | default | meaning |
|---|---|---|
| `VIBIAN_SSH` | `1` | `0`: no SSH server (only for a machine with a keyboard and screen) |
| `VIBIAN_SSH_PASSWORDS` | `yes` | `no`: keys only. system.sh refuses this until some user has a key in `~/.ssh/authorized_keys` |
| `VIBIAN_FIREWALL` | `1` | ufw: deny incoming except SSH and mDNS |
| `VIBIAN_MDNS` | `1` | answer to `<hostname>.local` (avahi) |
| `VIBIAN_SLEEP` | `never` | `never`: no suspend or hibernate, lid shut ignored. `normal`: put the defaults back |
| `VIBIAN_AUTO_UPDATES` | `1` | daily security updates (unattended-upgrades) |
| `VIBIAN_AUTO_REBOOT` | `off` | a time like `04:00`: reboot then if an update needs it |
| `VIBIAN_SSH_KEY` | none | a public key line to add to `~/.ssh/authorized_keys` |
| `VIBIAN_TMUX_ON_SSH` | `0` | `1`: SSH logins attach to the tmux session `main`, creating it if needed |
| `VIBIAN_ACCENT` | `7aa2f7` | hex colour of tmux's status line |

## Steps
1. `vibian snap "headless playbook"`.
2. Root part. Run it the way the powers setting allows:
   - `root` powers: `sudo VIBIAN_…=… bash scripts/system.sh`.
   - `scoped` powers: `vibian sudo --why "set this machine up as a headless server" -- /usr/bin/env VIBIAN_…=… bash scripts/system.sh`. The owner approves it once.
   - `confirm` powers: ask the owner to run the sudo line.

   It installs openssh-server, ufw, avahi-daemon, unattended-upgrades, tmux and htop. It turns off the login screens and boots to the console. It writes `/etc/ssh/sshd_config.d/01-vibian-headless.conf` and checks it with `sshd -t`, then reloads SSH (it never restarts it, so an SSH session survives). It allows SSH through the firewall before turning the firewall on, masks the sleep targets, and records its choices in `/etc/vibian/headless.env`.
3. User part, no sudo: `VIBIAN_SSH_KEY="ssh-ed25519 AAAA… me@laptop" bash scripts/user.sh`. It adds the key once, writes `~/.config/tmux/tmux.conf`, and adds or removes the tmux line in `~/.profile`. It also removes the lines the hyprland, sway, niri and i3 playbooks add there to start a desktop on tty1.
4. If the user wants keys only: have them log in with the key from their other computer first, then run system.sh again with `VIBIAN_SSH_PASSWORDS=no`.
5. `bash scripts/verify.sh`. It must print `VERIFY OK`.
6. `vibian log add "Headless (playbook headless)" --pkgs … --files /etc/ssh/sshd_config.d/01-vibian-headless.conf,/etc/systemd/logind.conf.d/50-vibian-headless.conf,~/.config/tmux,~/.profile --undo "…"` (closes the snapshot).
7. Tell the user how to connect: `ssh <user>@<hostname>.local`, or the IP address from `ip -brief address`. If a desktop was running, the screen keeps it until the next reboot.

## Undo
- `vibian undo <ledger-id>` puts back the files and packages from the snapshot pair.
- By hand:
  - Pick a desktop playbook, which turns its login screen back on, or run `sudo systemctl set-default graphical.target`.
  - Run `sudo ufw disable` and `sudo rm /etc/ssh/sshd_config.d/01-vibian-headless.conf && sudo systemctl reload ssh`.
  - Run system.sh with `VIBIAN_SLEEP=normal` to allow sleep again.
  - Delete the line ending `# vibian playbook headless` from `~/.profile`.

## Gotchas
- **Keys only, for real:** turn passwords off only after a key login has worked from the user's other computer. system.sh refuses when nobody has a key, but it can't tell whether the key is the right one.
- **Docker's published ports go around ufw.** Publish to `127.0.0.1:` unless the service should be reachable from the network (`dev-env` skill).
- **Opening a port:** `sudo ufw allow 8096/tcp comment jellyfin`, and log it with `vibian log add`. `sudo ufw status numbered` lists the open ports.
- **`.local` names** need mDNS on the other computer too. macOS, iOS, most Linux desktops and Windows 10 and later have it. Some guest Wi-Fi networks and VPNs block it, in which case use the IP address.
- **Reaching it from outside the home network** is out of scope: no port forwarding is set up. Tailscale is the easy route (`networking` skill).
- **A laptop with the lid shut** still has a battery to run down and a screen backlight that stays on. `VIBIAN_SLEEP=never` only stops sleeping. `power` skill: `consoleblank=60` on the kernel command line turns the console off after a minute.
- **tmux on SSH login:** detaching (Ctrl+B, then D) ends the SSH session, and the work goes on in the background. `ssh -t host bash` skips tmux for one login.
