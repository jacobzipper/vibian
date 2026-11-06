package scoped

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRisks(t *testing.T) {
	removed := func(argv []string) (map[string]bool, error) {
		switch argv[len(argv)-1] {
		case "sudo":
			return map[string]bool{"sudo": true}, nil
		case "huge":
			m := map[string]bool{}
			for i := range 30 {
				m[string(rune('a'+i))] = false
			}
			return m, nil
		}
		return map[string]bool{"sl": false}, nil
	}
	cases := []struct {
		cmd  string
		want string // a substring of the joined risks; "" means no risks
	}{
		{"apt-get install -y sl", ""},
		{"apt remove -y sl", ""},
		{"apt purge sudo", "core packages: sudo"},
		{"apt-get autoremove huge", "removes 30 packages"},
		{"dpkg --force-all -r sl", "--force"},
		{"systemctl restart NetworkManager", ""},
		{"systemctl reboot", "reboots"},
		{"reboot", "reboots"},
		{"tee /etc/apt/sources.list.d/x.list", ""},
		{"tee /etc/fstab", "touches /etc/fstab"},
		{"sed -i s/a/b/ /etc/default/grub", "/etc/default/grub"},
		{"cp x /boot/efi/EFI/x", "touches /boot"},
		{"dd if=x.iso of=/dev/sda bs=4M", "disks"},
		{"mkfs.ext4 /dev/sdb1", "disks"},
		{"parted /dev/nvme0n1 print", "disks"},
		{"rm -rf /home/u/build", ""},
		{"rm -rf build", ""},
		{"rm -rf /tmp/x", ""},
		{"rm -rf /tmp", "outside your home"},
		{"rm -rf /opt/thing", "outside your home"},
		{"rm -r ../../../opt", ""},
		{"rm -r ../../../../../opt", "outside your home"},
		{"rm /opt/thing/file", ""},
		{"rm -rf /", "outside your home"},
		{"chown -R u /usr/local", "outside your home"},
		{"chown u /usr/local/bin/x", ""},
		{"chmod 644 /usr", "touches /usr"},
		{"ls /usr", ""},
		{"bash -c 'apt install x'", "can't be checked"},
		{"sh x.sh", "can't be checked"},
		{"python3 fix.py", "can't be checked"},
		{"env FOO=1 apt install x", "can't be checked"},
		{"find /var/log -name '*.gz' -delete", "can't be checked"},
		{"find /var/log -name x", ""},
		{"sudo ls", "who can do what"},
		{"vibian powers root", "who can do what"},
		{"usermod -aG docker u", "who can do what"},
		{"btrfs subvolume list /", ""},
		{"btrfs filesystem usage /", ""},
		{"btrfs subvolume delete /x", "btrfs changes"},
		{"snapper -c root list", ""},
		{"snapper -c root delete 3", "snapper"},
		{"grub-install /dev/sda", "boots"},
		{"update-grub", ""},
		{"cat /etc/shadow", "touches /etc/shadow"},
		{"modprobe nvidia", ""},
	}
	for _, c := range cases {
		got := strings.Join(Risks(strings.Fields(strings.ReplaceAll(c.cmd, "'", "")), "/home/u/src/a/b", "/home/u", removed), "; ")
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: risks %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestRequests(t *testing.T) {
	Dir = t.TempDir()
	argv := []string{"rm", "-rf", "/opt/x"}
	id := RequestID("u", "/home/u", argv)
	if id != RequestID("u", "/home/u", argv) || id == RequestID("u", "/", argv) {
		t.Fatal("request ids should depend on user, dir and command only")
	}
	if Take(id) {
		t.Fatal("took an approval that was never asked for")
	}
	if err := Ask(Request{ID: id, Argv: argv, Asked: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if p, _ := Pending(); len(p) != 1 || Take(id) {
		t.Fatalf("pending %v; an unapproved request must not run", p)
	}
	if _, err := Approve(id); err != nil {
		t.Fatal(err)
	}
	if p, _ := Pending(); len(p) != 0 {
		t.Fatalf("approved requests aren't pending: %v", p)
	}
	if !Take(id) || Take(id) {
		t.Fatal("an approval should work exactly once")
	}

	// stale approvals don't count
	Ask(Request{ID: id, Argv: argv})
	r, _ := Approve(id)
	r.Approved = time.Now().Add(-ApprovalTTL - time.Minute)
	save(r)
	if Take(id) {
		t.Fatal("a stale approval ran")
	}
	if _, err := os.Stat(path(id)); !os.IsNotExist(err) {
		t.Fatal("a stale approval should be dropped")
	}
	if _, err := Load("../etc/x"); err == nil {
		t.Fatal("ids with paths must be rejected")
	}
}
