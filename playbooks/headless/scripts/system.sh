#!/bin/bash
# Headless playbook, root part: no login screen, and the machine set up to
# be reached over the network: SSH, a firewall, mDNS, no sleep, automatic
# security updates. Idempotent. Knobs: VIBIAN_SSH, VIBIAN_SSH_PASSWORDS,
# VIBIAN_FIREWALL, VIBIAN_MDNS, VIBIAN_SLEEP, VIBIAN_AUTO_UPDATES,
# VIBIAN_AUTO_REBOOT. The choices are kept in /etc/vibian/headless.env for
# verify.sh.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "run as root (sudo bash $0)" >&2; exit 1; }
ssh=${VIBIAN_SSH:-1}
passwords=${VIBIAN_SSH_PASSWORDS:-yes}
firewall=${VIBIAN_FIREWALL:-1}
mdns=${VIBIAN_MDNS:-1}
sleep=${VIBIAN_SLEEP:-never}
updates=${VIBIAN_AUTO_UPDATES:-1}
reboot=${VIBIAN_AUTO_REBOOT:-off}
for k in SSH:$ssh FIREWALL:$firewall MDNS:$mdns AUTO_UPDATES:$updates; do
  case ${k#*:} in 0|1) ;; *) echo "VIBIAN_${k%%:*}: 0 or 1" >&2; exit 1 ;; esac
done
case $passwords in yes|no) ;; *) echo "VIBIAN_SSH_PASSWORDS: yes or no" >&2; exit 1 ;; esac
case $sleep in never|normal) ;; *) echo "VIBIAN_SLEEP: never or normal" >&2; exit 1 ;; esac
[[ $reboot =~ ^(off|([01][0-9]|2[0-3]):[0-5][0-9])$ ]] || { echo "VIBIAN_AUTO_REBOOT: off or a time like 04:00" >&2; exit 1; }
if [ "$ssh" = 1 ] && [ "$passwords" = no ]; then
  # key-only with no key anywhere would lock everyone out
  keys=$(cat /root/.ssh/authorized_keys /home/*/.ssh/authorized_keys 2>/dev/null | grep -c '^[a-z]' || true)
  [ "$keys" -gt 0 ] || { echo "VIBIAN_SSH_PASSWORDS=no needs a key in someone's ~/.ssh/authorized_keys first (user.sh with VIBIAN_SSH_KEY adds one)" >&2; exit 1; }
fi

pkgs=(tmux htop)
[ "$ssh" = 1 ] && pkgs+=(openssh-server)
[ "$firewall" = 1 ] && pkgs+=(ufw)
[ "$mdns" = 1 ] && pkgs+=(avahi-daemon libnss-mdns)
[ "$updates" = 1 ] && pkgs+=(unattended-upgrades)
export DEBIAN_FRONTEND=noninteractive
# right after first boot, an apt job (locales, unattended-upgrades) may hold the lock
apt-get -o DPkg::Lock::Timeout=300 update -q
apt-get -o DPkg::Lock::Timeout=300 install -y -q --no-install-recommends "${pkgs[@]}"

# no login screen: boot to the text console. Desktops stay installed.
for dm in greetd sddm gdm gdm3 lightdm cosmic-greeter; do systemctl disable -q "$dm" 2>/dev/null || true; done
rm -f /etc/systemd/system/display-manager.service
systemctl set-default -q multi-user.target

if [ "$ssh" = 1 ]; then
  # sshd keeps the first value it reads, and installs often carry
  # 50-cloud-init.conf with PasswordAuthentication yes: this file sorts first
  cat > /etc/ssh/sshd_config.d/01-vibian-headless.conf <<T
# Written by Vibian's headless playbook (VIBIAN_SSH_PASSWORDS=$passwords).
PermitRootLogin no
PasswordAuthentication $passwords
KbdInteractiveAuthentication no
T
  sshd -t
  systemctl enable -q ssh.socket 2>/dev/null || systemctl enable -q ssh.service
  # reload, never restart: this may be running over ssh
  systemctl reload ssh.service 2>/dev/null || true
  systemctl start ssh.socket 2>/dev/null || systemctl start ssh.service
else
  rm -f /etc/ssh/sshd_config.d/01-vibian-headless.conf
fi

if [ "$firewall" = 1 ]; then
  # ssh first, so enabling it never cuts this session
  [ "$ssh" = 1 ] && ufw allow OpenSSH >/dev/null
  [ "$mdns" = 1 ] && ufw allow 5353/udp comment mDNS >/dev/null
  ufw default deny incoming >/dev/null
  ufw default allow outgoing >/dev/null
  ufw --force enable >/dev/null
  systemctl enable -q ufw
fi

[ "$mdns" = 1 ] && systemctl enable -q --now avahi-daemon.service

targets=(sleep.target suspend.target hibernate.target hybrid-sleep.target suspend-then-hibernate.target)
if [ "$sleep" = never ]; then
  systemctl mask -q "${targets[@]}"
  install -d /etc/systemd/logind.conf.d
  cat > /etc/systemd/logind.conf.d/50-vibian-headless.conf <<T
# Written by Vibian's headless playbook: a server keeps running with the lid
# shut.
[Login]
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
IdleAction=ignore
T
else
  systemctl unmask -q "${targets[@]}"
  rm -f /etc/systemd/logind.conf.d/50-vibian-headless.conf
fi
# logind rereads its config on reload, without ending sessions
systemctl reload systemd-logind 2>/dev/null || true

if [ "$updates" = 1 ]; then
  cat > /etc/apt/apt.conf.d/20auto-upgrades <<T
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
T
  if [ "$reboot" = off ]; then
    rm -f /etc/apt/apt.conf.d/52vibian-headless
  else
    cat > /etc/apt/apt.conf.d/52vibian-headless <<T
// Written by Vibian's headless playbook: reboot after an update that needs it.
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "$reboot";
T
  fi
  systemctl enable -q --now unattended-upgrades.service
fi

install -d /etc/vibian
cat > /etc/vibian/headless.env <<T
# The headless playbook's last run, for its verify.sh.
ssh=$ssh
passwords=$passwords
firewall=$firewall
mdns=$mdns
sleep=$sleep
updates=$updates
reboot=$reboot
T
echo "system part done: ${#pkgs[@]} packages; boots to the console; ssh=$ssh passwords=$passwords firewall=$firewall mdns=$mdns sleep=$sleep updates=$updates reboot=$reboot"
