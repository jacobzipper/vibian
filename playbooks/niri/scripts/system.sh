#!/bin/bash
# niri playbook, root part. niri is not in Ubuntu 26.04's archive, so this
# builds the upstream release from source with Ubuntu's own Rust toolchain:
# the source and its vendored crates are pinned by sha256, and the build runs
# offline. xwayland-satellite (X11 apps) is built the same way from its
# Cargo.lock. The rest (bar, launcher, lock, login screen) comes from the
# archive. Idempotent: a rerun skips builds whose version is already
# installed. Knobs: VIBIAN_TERMINAL, VIBIAN_LOGIN, VIBIAN_XWAYLAND,
# VIBIAN_KEEP_BUILD_DEPS.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
term=${VIBIAN_TERMINAL:-foot}
login=${VIBIAN_LOGIN:-greetd}
xwl=${VIBIAN_XWAYLAND:-1}
keep=${VIBIAN_KEEP_BUILD_DEPS:-0}
case $term in foot|kitty|alacritty) ;; *) echo "VIBIAN_TERMINAL: foot, kitty or alacritty" >&2; exit 1 ;; esac
case $login in greetd|tty) ;; *) echo "VIBIAN_LOGIN: greetd or tty" >&2; exit 1 ;; esac
case $xwl$keep in [01][01]) ;; *) echo "VIBIAN_XWAYLAND and VIBIAN_KEEP_BUILD_DEPS are 0 or 1" >&2; exit 1 ;; esac

NIRI_VER=26.04
NIRI_SRC=https://github.com/YaLTeR/niri/archive/refs/tags/v$NIRI_VER.tar.gz
NIRI_SRC_SHA=134c602d8e0d53413a52d6cd58f9ce7e79a07d03288ee0a51ba1abd5db1b1ad9
NIRI_VENDOR=https://github.com/YaLTeR/niri/releases/download/v$NIRI_VER/niri-$NIRI_VER-vendored-dependencies.tar.xz
NIRI_VENDOR_SHA=407c525cd78710e72455c48741756857ca47dc32910a42f82cd40992a34beed6
XWLS_VER=0.8.3
XWLS_SRC=https://github.com/Supreeeme/xwayland-satellite/archive/refs/tags/v$XWLS_VER.tar.gz
XWLS_SRC_SHA=8b32873d7d740143ce9efa3cfb43c17d0176b89d2e8ffaa497db709f154fc478

pkgs=(swayidle swaylock xdg-desktop-portal-gnome xdg-desktop-portal-gtk gnome-keyring lxpolkit
      waybar fuzzel mako-notifier swaybg wl-clipboard cliphist brightnessctl playerctl
      pipewire-pulse wireplumber pavucontrol network-manager-gnome blueman
      fonts-noto-core fonts-noto-color-emoji fonts-font-awesome qt6-wayland qtwayland5 "$term")
[ "$xwl" = 1 ] && pkgs+=(xwayland)
[ "$login" = greetd ] && pkgs+=(greetd tuigreet)
build=(rustc cargo clang gcc pkg-config curl xz-utils libudev-dev libgbm-dev libxkbcommon-dev
       libegl-dev libwayland-dev libinput-dev libdbus-1-dev libsystemd-dev libseat-dev
       libpipewire-0.3-dev libpango1.0-dev libdisplay-info-dev)
[ "$xwl" = 1 ] && build+=(libxcb1-dev libxcb-cursor-dev)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"

have() { "$1" --version 2>/dev/null | grep -qF " $2"; }
need_niri=1; have /usr/local/bin/niri "$NIRI_VER" && need_niri=0
# xwayland-satellite has no --version: a stamp file records what was built
stamp=/usr/local/share/vibian/xwayland-satellite.version
need_xwls=0; [ "$xwl" = 1 ] && ! { [ -x /usr/local/bin/xwayland-satellite ] && grep -qx "$XWLS_VER" "$stamp" 2>/dev/null; } && need_xwls=1
if [ $need_niri = 1 ] || [ $need_xwls = 1 ]; then
  # remember what the build installs (with its dependencies), to remove
  # exactly that afterwards
  installed() { dpkg-query -W -f '${db:Status-Abbrev} ${Package}\n' | awk '$1 == "ii" {print $2}' | sort -u; }
  before=$(installed)
  apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${build[@]}"
  new=$(comm -13 <(echo "$before") <(installed))
  work=$(mktemp -d /var/tmp/vibian-niri.XXXXXX)
  trap 'rm -rf "$work"' EXIT
  export CARGO_HOME=$work/cargo CARGO_TARGET_DIR=$work/target
  fetch() {  # fetch URL SHA256 FILE
    curl -fsSL --retry 3 -o "$3" "$1"
    echo "$2  $3" | sha256sum -c --quiet - || { echo "checksum mismatch for $1" >&2; exit 1; }
  }
  jobs=$(nproc); mem=$(awk '/MemTotal/ {print int($2 / 1048576)}' /proc/meminfo)
  # about 1 GB per rustc job; fewer jobs beats swapping
  [ "$mem" -lt "$jobs" ] && jobs=$(( mem > 0 ? mem : 1 ))
  if [ $need_niri = 1 ]; then
    echo "building niri $NIRI_VER from source ($jobs jobs; this takes 5 to 20 minutes)"
    fetch "$NIRI_SRC" "$NIRI_SRC_SHA" "$work/niri.tgz"
    fetch "$NIRI_VENDOR" "$NIRI_VENDOR_SHA" "$work/vendor.txz"
    tar xzf "$work/niri.tgz" -C "$work"
    src=$work/niri-$NIRI_VER
    tar xJf "$work/vendor.txz" -C "$src"
    mkdir -p "$src/.cargo"
    cat > "$src/.cargo/config.toml" <<'T'
[source.crates-io]
replace-with = "vendored-sources"
[source."git+https://github.com/Smithay/smithay.git?rev=ff5fa7df392cecfba049ffed55cdaa4e98a8e7ef"]
git = "https://github.com/Smithay/smithay.git"
rev = "ff5fa7df392cecfba049ffed55cdaa4e98a8e7ef"
replace-with = "vendored-sources"
[source.vendored-sources]
directory = "vendor"
T
    (cd "$src" && cargo build --release --frozen -j "$jobs")
    install -m 0755 "$CARGO_TARGET_DIR/release/niri" "$src/resources/niri-session" /usr/local/bin/
    install -d /usr/local/lib/systemd/user /usr/local/share/wayland-sessions /etc/xdg/xdg-desktop-portal
    sed 's|^ExecStart=niri |ExecStart=/usr/local/bin/niri |' "$src/resources/niri.service" > /usr/local/lib/systemd/user/niri.service
    install -m 0644 "$src/resources/niri-shutdown.target" /usr/local/lib/systemd/user/
    install -m 0644 "$src/resources/niri.desktop" /usr/local/share/wayland-sessions/
    install -m 0644 "$src/resources/niri-portals.conf" /etc/xdg/xdg-desktop-portal/
  fi
  if [ $need_xwls = 1 ]; then
    echo "building xwayland-satellite $XWLS_VER from source"
    fetch "$XWLS_SRC" "$XWLS_SRC_SHA" "$work/xwls.tgz"
    tar xzf "$work/xwls.tgz" -C "$work"
    # crates are fetched from crates.io and checked against Cargo.lock
    (cd "$work/xwayland-satellite-$XWLS_VER" && cargo fetch --locked && cargo build --release --frozen -j "$jobs" -F systemd)
    install -m 0755 "$CARGO_TARGET_DIR/release/xwayland-satellite" /usr/local/bin/
    install -d "$(dirname "$stamp")" && echo "$XWLS_VER" > "$stamp"
  fi
  # keep the shared libraries the binaries load when the -dev packages go
  bins=(/usr/local/bin/niri); [ -x /usr/local/bin/xwayland-satellite ] && bins+=(/usr/local/bin/xwayland-satellite)
  libs=$(ldd "${bins[@]}" | awk '$3 ~ /^\// {print $3}' | sort -u)
  runtime=$(for l in $libs; do dpkg -S "$(readlink -f "$l")" 2>/dev/null || dpkg -S "$l" 2>/dev/null; done | cut -d: -f1 | sort -u)
  if [ "$keep" = 0 ] && [ -n "$new" ]; then
    # only what the build brought in and nothing needs now; packages that
    # were already unneeded before are left alone
    apt-mark auto $new >/dev/null
    [ -n "$runtime" ] && apt-mark manual $runtime >/dev/null
    gone=$(apt-get -s autoremove | awk '/^Remv / {sub(/:.*/, "", $2); print $2}' | sort -u | comm -12 - <(echo "$new"))
    [ -n "$gone" ] && apt-get -o DPkg::Lock::Timeout=300 purge -y -q $gone
  elif [ -n "$runtime" ]; then
    apt-mark manual $runtime >/dev/null
  fi
fi

# Ubuntu enables these user services for every graphical session, so they
# would also start inside Plasma or GNOME. This desktop's config starts them.
systemctl --global disable -q waybar.service mako.service hypridle.service hyprpolkitagent.service 2>/dev/null || true
if [ "$login" = greetd ]; then
  # one login screen at a time: another desktop's may own display-manager.service
  for dm in sddm gdm3 lightdm cosmic-greeter; do systemctl disable -q "$dm" 2>/dev/null || true; done
  install -d /etc/greetd
  [ -f /etc/greetd/config.toml ] && [ ! -f /etc/greetd/config.toml.vibian-bak ] && cp /etc/greetd/config.toml /etc/greetd/config.toml.vibian-bak
  greeter=$(getent passwd _greetd >/dev/null && echo _greetd || echo greeter)
  cat > /etc/greetd/config.toml <<T
# Written by Vibian's niri playbook.
# vt 7 is Ubuntu's default: tty1 keeps its console (and onboarding)
[terminal]
vt = 7

[default_session]
command = "tuigreet --time --remember --asterisks --cmd niri-session"
user = "$greeter"
T
  # tuigreet's --remember needs its cache dir
  install -d -o "$greeter" -m 0755 /var/cache/tuigreet
  systemctl enable greetd.service
  systemctl set-default graphical.target
fi
echo "system part done: niri $(/usr/local/bin/niri --version | awk '{print $2}'), ${#pkgs[@]} packages, login=$login"
