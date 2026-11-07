#!/bin/bash
# Headless playbook, user part: an SSH key to log in with, a tmux config,
# and (if asked) tmux on every SSH login so the agent keeps running when the
# connection drops. Never run as root. A tmux config that wasn't Vibian's is
# kept as tmux.conf.vibian-bak (only the first time). Knobs: VIBIAN_SSH_KEY,
# VIBIAN_TMUX_ON_SSH, VIBIAN_ACCENT.
set -euo pipefail
[ "$(id -u)" != 0 ] || { echo "run as your user, not root" >&2; exit 1; }
key=${VIBIAN_SSH_KEY:-}
attach=${VIBIAN_TMUX_ON_SSH:-0}
accent=${VIBIAN_ACCENT:-7aa2f7}
[[ $accent =~ ^[0-9a-fA-F]{6}$ ]] || { echo "VIBIAN_ACCENT must be 6 hex digits, got '$accent'" >&2; exit 1; }
case $attach in 0|1) ;; *) echo "VIBIAN_TMUX_ON_SSH: 0 or 1" >&2; exit 1 ;; esac
if [ -n "$key" ]; then
  [[ $key =~ ^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com)\ [A-Za-z0-9+/]+=*(\ [^[:cntrl:]]*)?$ ]] ||
    { echo "VIBIAN_SSH_KEY: one public key line, like 'ssh-ed25519 AAAA… me@laptop' (the .pub file, never the private key)" >&2; exit 1; }
fi

if [ -n "$key" ]; then
  install -d -m 700 ~/.ssh
  touch ~/.ssh/authorized_keys
  chmod 600 ~/.ssh/authorized_keys
  # the same key (type and body) only once, whatever its comment
  body=$(printf '%s' "$key" | cut -d' ' -f1,2)
  if grep -qF "$body" ~/.ssh/authorized_keys; then
    echo "the key is already in ~/.ssh/authorized_keys"
  else
    printf '%s\n' "$key" >> ~/.ssh/authorized_keys
    echo "added the key to ~/.ssh/authorized_keys"
  fi
fi

cfg=${XDG_CONFIG_HOME:-$HOME/.config}
conf=$cfg/tmux/tmux.conf
mkdir -p "$cfg/tmux"
if [ -f "$conf" ] && ! grep -q 'vibian playbook headless' "$conf" && [ ! -e "$conf.vibian-bak" ]; then
  cp "$conf" "$conf.vibian-bak"
fi
cat > "$conf" <<T
# Written by the vibian playbook headless; user.sh rewrites this file.
# Your own settings go in ~/.config/tmux/local.conf.
set -g mouse on
set -g history-limit 50000
set -g base-index 1
setw -g pane-base-index 1
set -g renumber-windows on
set -g default-terminal "tmux-256color"
set -g set-titles on
set -g status-style "bg=default,fg=#a9b1d6"
set -g status-left "#[fg=#$accent,bold] #h #[default] "
set -g status-left-length 30
set -g status-right "%a %H:%M "
setw -g window-status-current-style "fg=#$accent,bold"
set -g pane-active-border-style "fg=#$accent"
if-shell "test -f ~/.config/tmux/local.conf" "source-file ~/.config/tmux/local.conf"
T

# tmux on SSH login: one session, "main", that survives a dropped connection.
# If tmux can't start (a client without a terminal type), the login carries
# on as a plain shell. The lines other playbooks add to start their desktop
# on tty1 go too.
line='[ -n "${SSH_TTY:-}" ] && [ -z "${TMUX:-}" ] && [ "${TERM:-dumb}" != dumb ] && command -v tmux >/dev/null && tmux new-session -A -s main && exit # vibian playbook headless'
touch ~/.profile
grep -vE '# vibian playbook (headless|hyprland|sway|niri|i3)$' ~/.profile > ~/.profile.vibian-tmp || true
[ "$attach" = 1 ] && printf '%s\n' "$line" >> ~/.profile.vibian-tmp
cat ~/.profile.vibian-tmp > ~/.profile
rm -f ~/.profile.vibian-tmp
echo "wrote $conf; tmux on ssh login: $([ "$attach" = 1 ] && echo on || echo off)"
