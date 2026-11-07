#!/bin/bash
# Quickshell desk playbook, root part. The desk runs on Hyprland, so this
# first runs the hyprland playbook's system.sh (packages, greetd login).
# Quickshell is not in Ubuntu 26.04's archive, so it is built from the
# upstream release (source pinned by sha256) against Ubuntu's Qt 6.10, and
# installed to /usr/local. The build dependencies are removed afterwards.
# Idempotent: a rerun skips the build if this version is installed. Knobs:
# VIBIAN_TERMINAL, VIBIAN_LOGIN (passed to hyprland), VIBIAN_KEEP_BUILD_DEPS.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
here=$(cd "$(dirname "$0")/.." && pwd)
keep=${VIBIAN_KEEP_BUILD_DEPS:-0}
case $keep in 0|1) ;; *) echo "VIBIAN_KEEP_BUILD_DEPS is 0 or 1" >&2; exit 1 ;; esac

QS_VER=0.3.2
QS_SRC=https://github.com/quickshell-mirror/quickshell/archive/refs/tags/v$QS_VER.tar.gz
QS_SRC_SHA=946a27e2a276917a03c1079e5c60971af84ece73088c3661dee455d92eb420f7

bash "$here/../hyprland/scripts/system.sh"

# what the desk's QML loads at run time, and the fonts and icons it draws with
pkgs=(qml6-module-qtquick qml6-module-qtquick-layouts qml6-module-qtquick-controls
      qml6-module-qtquick-window qml6-module-qtqml-workerscript qml6-module-qtquick-templates
      qt6-svg-plugins qt6-wayland libjemalloc2 upower papirus-icon-theme fonts-inter jq)
build=(cmake ninja-build g++ pkg-config curl qt6-base-dev qt6-base-private-dev
       qt6-declarative-dev qt6-declarative-private-dev qt6-shadertools-dev qt6-wayland-dev
       qt6-wayland-private-dev libcli11-dev libjemalloc-dev spirv-tools libdrm-dev libgbm-dev
       libvulkan-dev libegl-dev libwayland-dev wayland-protocols libpipewire-0.3-dev libpam0g-dev
       libpolkit-agent-1-dev libglib2.0-dev libxcb1-dev)
export DEBIAN_FRONTEND=noninteractive
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"

if ! /usr/local/bin/quickshell --version 2>/dev/null | grep -qF "Quickshell $QS_VER "; then
  # remember what the build installs (with its dependencies), to remove
  # exactly that afterwards
  installed() { dpkg-query -W -f '${db:Status-Abbrev} ${Package}\n' | awk '$1 == "ii" {print $2}' | sort -u; }
  before=$(installed)
  apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${build[@]}"
  new=$(comm -13 <(echo "$before") <(installed))
  work=$(mktemp -d /var/tmp/vibian-quickshell.XXXXXX)
  trap 'rm -rf "$work"' EXIT
  curl -fsSL --retry 3 -o "$work/qs.tgz" "$QS_SRC"
  echo "$QS_SRC_SHA  $work/qs.tgz" | sha256sum -c --quiet - || { echo "checksum mismatch for $QS_SRC" >&2; exit 1; }
  tar xzf "$work/qs.tgz" -C "$work"
  jobs=$(nproc); mem=$(awk '/MemTotal/ {print int($2 / 1048576)}' /proc/meminfo)
  # about 1 GB per C++ job; fewer jobs beats swapping
  [ "$mem" -lt "$jobs" ] && jobs=$(( mem > 0 ? mem : 1 ))
  echo "building Quickshell $QS_VER from source ($jobs jobs; this takes 15 to 30 minutes)"
  # no crash handler: it needs cpptrace, which Ubuntu doesn't package
  cmake -S "$work/quickshell-$QS_VER" -B "$work/build" -G Ninja -DCMAKE_BUILD_TYPE=RelWithDebInfo \
    -DCMAKE_INSTALL_PREFIX=/usr/local -DDISTRIBUTOR="Vibian quickshell-desk playbook" \
    -DCRASH_HANDLER=OFF -DBUILD_TESTING=OFF -DINSTALL_QML_PREFIX=lib/qt6/qml
  cmake --build "$work/build" -j "$jobs"
  cmake --install "$work/build" --strip
  # keep the shared libraries quickshell loads when the -dev packages go
  libs=$(ldd /usr/local/bin/quickshell | awk '$3 ~ /^\// {print $3}' | sort -u)
  runtime=$(for l in $libs; do dpkg -S "$(readlink -f "$l")" 2>/dev/null || dpkg -S "$l" 2>/dev/null; done | cut -d: -f1 | sort -u)
  if [ "$keep" = 0 ] && [ -n "$new" ]; then
    # only what the build brought in and nothing needs now
    apt-mark auto $new >/dev/null
    [ -n "$runtime" ] && apt-mark manual $runtime >/dev/null
    gone=$(apt-get -s autoremove | awk '/^Remv / {sub(/:.*/, "", $2); print $2}' | sort -u | comm -12 - <(echo "$new"))
    [ -n "$gone" ] && apt-get -o DPkg::Lock::Timeout=300 purge -y -q $gone
  elif [ -n "$runtime" ]; then
    apt-mark manual $runtime >/dev/null
  fi
fi
/usr/local/bin/quickshell --version >/dev/null
echo "system part done: $(/usr/local/bin/quickshell --version | head -n 1), ${#pkgs[@]} packages for the desk"
