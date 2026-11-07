#!/bin/bash
# Ask overlay playbook, root part: GTK 4 and gtk4-layer-shell for the
# overlay, notify-send for "done" notifications, and the overlay itself in
# /usr/local. Idempotent. No knobs.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
here=$(cd "$(dirname "$0")/.." && pwd)
# the layer-shell bindings don't depend on the library itself
pkgs=(python3-gi gir1.2-gtk-4.0 gir1.2-gtk4layershell-1.0 libgtk4-layer-shell0 libnotify-bin)
export DEBIAN_FRONTEND=noninteractive
apt-get -o DPkg::Lock::Timeout=300 update -q
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"
install -D -m 644 "$here/files/ask-overlay.py" /usr/local/lib/vibian/ask-overlay.py
install -D -m 755 "$here/files/vibian-ask" /usr/local/bin/vibian-ask
echo "system part done: ${#pkgs[@]} packages; /usr/local/bin/vibian-ask"
