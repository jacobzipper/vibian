---
name: services
description: "systemd on Vibian: system and user services, timers instead of cron, overrides, transient units, the journal, boot time and failed units. Use when something should run in the background, at boot, at login or on a schedule, when a service fails or won't start, or when reading logs."
---

# Services on Vibian

Anything that should keep running gets a systemd unit, not `nohup`, `&`, `screen` or `@reboot` cron lines. A unit restarts on failure, logs to the journal and can be undone by deleting one file.

## Look first
```bash
systemctl --failed; systemctl --user --failed
systemctl status <unit>                       # also: systemctl cat <unit> (file + drop-ins)
journalctl -u <unit> -b --no-pager | tail -50
journalctl -b -p err --no-pager | tail -50    # every error this boot
journalctl -b -1 -p warning                   # last boot (after a crash)
systemd-analyze blame | head; systemd-analyze critical-chain
```
`vibian doctor` reports failed units too.

## System or user?
| needs | kind | files |
|---|---|---|
| root, hardware, starts before login | system | `/etc/systemd/system/<name>.service` |
| the user's files, their desktop, their audio | user | `~/.config/systemd/user/<name>.service` |
| a user service that keeps running after logout or starts at boot | user, plus `sudo loginctl enable-linger <user>` | |

## A service
```ini
# ~/.config/systemd/user/devserver.service
[Unit]
Description=Dev server for ~/proj

[Service]
WorkingDirectory=%h/proj
ExecStart=/usr/bin/npm run dev
Restart=on-failure
Environment=PORT=3000

[Install]
WantedBy=default.target
```
```bash
systemctl --user daemon-reload && systemctl --user enable --now devserver
```
System services use `WantedBy=multi-user.target`. Give them `User=` unless they really need root, and harden them with `ProtectSystem=strict`, `ProtectHome=read-only` and `NoNewPrivileges=yes` where possible.

## Timers (instead of cron)
```ini
# backup.timer, next to backup.service (Type=oneshot)
[Timer]
OnCalendar=daily          # or Mon..Fri 09:00, *:0/15
Persistent=true           # runs at boot if a run was missed while off
RandomizedDelaySec=10m
[Install]
WantedBy=timers.target
```
`systemd-analyze calendar "Mon..Fri 09:00"` shows when that fires; `systemctl list-timers` shows what's scheduled.

## Changing a packaged unit
Never edit `/usr/lib/systemd/system/*`: an upgrade overwrites it. Write a drop-in:
```bash
sudo mkdir -p /etc/systemd/system/<unit>.d
printf '[Service]\nEnvironment=FOO=1\n' | sudo tee /etc/systemd/system/<unit>.d/50-vibian.conf
sudo systemctl daemon-reload && sudo systemctl restart <unit>
```
(`systemctl edit` does the same but opens an editor, which you can't drive.) To clear a list setting like `ExecStart`, set it empty first: `ExecStart=` then the new line.

## One-off background jobs
`systemd-run --user --unit=<name> <cmd>` runs a command as a transient unit, so its output stays in `journalctl --user -u <name>`. Use it for long builds or downloads that shouldn't die with your shell. `systemd-run --user --on-active=30m <cmd>` runs a command later.

## Don't
- Don't restart `systemd-logind`, the display manager, `NetworkManager` or `dbus` while the user is working: say what will happen first.
- Don't `mask` units you don't understand. `disable --now` is reversible, and a masked unit confuses the next agent.
- Ledger every unit, timer and drop-in you add, with its path, so `vibian undo` and the next agent can find it.
