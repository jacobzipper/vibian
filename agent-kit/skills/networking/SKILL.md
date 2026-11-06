---
name: networking
description: "Wifi, ethernet, VPNs (WireGuard, Tailscale, Mullvad, OpenVPN), DNS, hotspots and the firewall on Vibian, all through NetworkManager. Use when the user asks to join or forget a network, the internet or DNS is down, wifi is slow or drops, they want a VPN or a static IP, or a port needs opening."
---

# Networking on Vibian

NetworkManager runs every interface. `/etc/netplan/01-vibian-networkmanager.yaml` sets `renderer: NetworkManager`; the installer's wired config (`50-cloud-init.yaml`) shows up as the connection `netplan-<iface>`. The owner's user (the sudo group) may run `nmcli` without sudo, through `/usr/share/polkit-1/rules.d/50-vibian-networkmanager.rules`.

## Look first
```bash
nmcli general                         # state, connectivity (full / limited / portal / none)
nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device
ip -br addr; ip route
resolvectl status                     # DNS per link (systemd-resolved)
nmcli networking connectivity check   # captive portal? → "portal"
vibian doctor                         # network + internet checks
```

## Wifi
```bash
nmcli device wifi rescan; sleep 5; nmcli device wifi list
nmcli --ask device wifi connect "<SSID>"         # the user types the password, it never passes through you
nmcli device wifi connect "<SSID>" hidden yes --ask
nmcli connection show; nmcli connection delete "<name>"   # forget
nmcli connection modify "<name>" connection.autoconnect-priority 10
```
- Let the user type wifi passwords themselves (`--ask` on their terminal). If they paste one to you, use it and never repeat it, log it or put it in the ledger.
- Enterprise (eduroam, WPA2-Enterprise): `nmcli connection add type wifi ifname <wlan> con-name eduroam ssid eduroam wifi-sec.key-mgmt wpa-eap 802-1x.eap peap 802-1x.phase2-auth mschapv2 802-1x.identity <user> --ask`, plus `802-1x.ca-cert` if the institution publishes one.
- Hotspot: `nmcli device wifi hotspot ifname <wlan> ssid <name> password <pw>`; stop it with `nmcli connection down Hotspot`.
- Connections are saved as netplan files `/etc/netplan/90-NM-<uuid>.yaml` (mode 600, they hold the passwords). Don't paste them anywhere.

## When wifi misbehaves
| symptom | check / fix |
|---|---|
| no wifi device | `rfkill list` → `rfkill unblock wifi`; `dmesg \| grep -iE 'firmware\|wlan\|iwl\|ath\|rtw\|mt7'`; missing firmware → `linux-firmware`; Broadcom → `ubuntu-drivers list` (see `drivers-hardware`) |
| drops every few minutes | power saving: `/etc/NetworkManager/conf.d/wifi-powersave-off.conf` with `[connection]` / `wifi.powersave = 2`, then `sudo systemctl reload NetworkManager` |
| sees few 5/6 GHz networks | regulatory domain: `iw reg get`; set it with `sudo iw reg set <CC>`, persist with `cfg80211 ieee80211_regdom=<CC>` in `/etc/modprobe.d/` |
| connected, no internet | `nmcli networking connectivity check`; portal → open a browser (or `curl -sI http://connectivity-check.ubuntu.com`); otherwise DNS (below) |

## DNS and static addresses
```bash
nmcli connection modify "<name>" ipv4.dns "1.1.1.1 9.9.9.9" ipv4.ignore-auto-dns yes
nmcli connection modify "<name>" ipv4.method manual ipv4.addresses 192.168.1.50/24 ipv4.gateway 192.168.1.1
nmcli connection up "<name>"          # applies it; drops the link for a second
```
Revert with `ipv4.method auto ipv4.ignore-auto-dns no ipv4.dns ""`.

## VPNs
- **WireGuard**: `apt install wireguard-tools`; `nmcli connection import type wireguard file wg0.conf` (the name comes from the file). The private key is in that file: keep it mode 600 and out of the ledger.
- **Tailscale**: its own apt repo (pkgs.tailscale.com, see `packages` for the keyring/`.sources` pattern), then `sudo tailscale up`. The user opens the login URL.
- **Mullvad**: its apt repo (repository.mullvad.net), package `mullvad-vpn`; `mullvad account login` with the user typing the account number. Never print or log that number.
- **OpenVPN**: `apt install network-manager-openvpn`; `nmcli connection import type openvpn file x.ovpn`.
- A VPN that sets a default route changes what the agent can reach. Say so before you bring one up.

## Firewall
ufw is off by default. To turn it on without locking out ssh:
```bash
sudo ufw allow OpenSSH        # only if sshd is in use
sudo ufw default deny incoming && sudo ufw enable
sudo ufw allow 8080/tcp comment 'dev server'; sudo ufw status numbered
```
Log every open port in the ledger with its reason.

## Don't
- Don't `netplan apply` or restart NetworkManager while the user (or you, over ssh) depends on the link without saying so: it can drop the connection.
- Don't remove `01-vibian-networkmanager.yaml` or the polkit rule. Onboarding and the agent rely on them.
