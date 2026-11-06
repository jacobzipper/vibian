#!/bin/bash
# scoped-powers.sh: runs inside the test VM as the vibian user (password
# vibian, powers=root to start) and walks powers=scoped: plain commands run
# and are logged, risky ones wait for the owner's `sudo vibian approve`, the
# agent can't approve itself, and an OK works once. Ends back on powers=root.
# Prints key=value lines for headless-install.sh to check.
set -u
pw=vibian
owner() { echo "$pw" | sudo -S -p '' "$@"; }
asks() { vibian sudo --why "$@" 2>&1 >/dev/null | sed -n 's/.*request \([0-9a-f]\{8\}\).*/\1/p' | head -1; }

vibian powers scoped >/dev/null; echo "set=$? mode=$(vibian powers)"
sudo -k; sudo -n true 2>/dev/null && echo plain-sudo=yes || echo plain-sudo=no
vibian sudo --why "scoped test: mark" -- touch /etc/vibian-scoped-test; echo "plain=$? owner=$(stat -c %U /etc/vibian-scoped-test)"
vibian sudo -- touch /etc/x 2>/dev/null; echo "nowhy=$?"
vibian sudo --why t -- false; echo "false=$?"
vibian sudo --why "scoped test: setup" -- mkdir -p /opt/scoped-test; echo "mkdir=$?"
vibian sudo --why "scoped test: clean up" -- rm -rf /opt/scoped-test 2>/dev/null; echo "risky=$?"
id=$(asks "scoped test: clean up" -- rm -rf /opt/scoped-test); echo "id=${id:-none}"
sudo -n vibian approve --yes "$id" >/dev/null 2>&1; echo "self-approve=$?"
vibian sudo --why sneaky -- vibian approve --yes "$id" >/dev/null 2>&1; echo "sudo-approve=$?"
sudo -n /usr/bin/vibian sudo-root --why x --agent y -- rm -rf /opt/scoped-test >/dev/null 2>&1; echo "direct=$?"
vibian sudo --why t -- bash -c id >/dev/null 2>&1; echo "wrapper=$?"
cp /usr/bin/id ~/myid; vibian sudo --why t -- ~/myid >/dev/null 2>&1; echo "own-program=$?"
echo "listed=$(owner vibian approve | grep -c "$id")"
owner vibian approve --yes "$id" >/dev/null; echo "approve=$?"
vibian sudo --why "scoped test: clean up" -- rm -rf /opt/scoped-test; echo "rerun=$? gone=$(test -e /opt/scoped-test && echo no || echo yes)"
vibian sudo --why "scoped test: clean up" -- rm -rf /opt/scoped-test 2>/dev/null; echo "once=$?"
id2=$(asks t -- bash -c id); owner vibian approve --deny "$id2" >/dev/null; echo "deny=$? left=$(owner vibian approve | grep -c "$id2")"
vibian log search "scoped test" > /tmp/scoped-ledger.txt
echo "ledger-cmd=$(grep -c 'command: `rm -rf /opt/scoped-test`' /tmp/scoped-ledger.txt) ledger-ok=$(grep -c 'approved by the owner' /tmp/scoped-ledger.txt) ledger-snap=$(grep -A6 'clean up' /tmp/scoped-ledger.txt | grep -c 'snapshots: root')"
owner vibian sudo-root --why x --agent y -- true >/dev/null 2>&1; owner rm -f /etc/vibian-scoped-test
owner vibian powers root >/dev/null && vibian powers root >/dev/null; echo "restored=$(sudo -n true && vibian powers)"
