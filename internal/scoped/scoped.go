// Package scoped is the policy behind powers=scoped: the agent has no sudo
// of its own and asks for root one command at a time through `vibian sudo`.
// Ordinary commands run and are logged; risky ones wait for the owner to
// approve them with their own password (`sudo vibian approve`).
//
// This guards against an agent's mistakes, not against a hostile agent: it
// runs as the owner's user, so it can still change anything that user owns.
package scoped

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Risks says why a root command needs the owner's OK; empty means it can
// run. argv is the command, cwd and home are the caller's. removed lists the
// packages an apt or dpkg removal would take with it, plus whether each is
// core (essential, required or important, or on Vibian's own list).
func Risks(argv []string, cwd, home string, removed func(argv []string) (map[string]bool, error)) []string {
	if len(argv) == 0 {
		return []string{"no command"}
	}
	var risks []string
	add := func(r string) {
		if !slices.Contains(risks, r) {
			risks = append(risks, r)
		}
	}
	name := filepath.Base(argv[0])
	args := argv[1:]

	switch {
	case wrappers[name] || strings.HasPrefix(name, "python") || strings.HasPrefix(name, "perl") ||
		(name == "find" && slices.ContainsFunc(args, func(a string) bool {
			return a == "-delete" || strings.HasPrefix(a, "-exec") || strings.HasPrefix(a, "-ok")
		})):
		add(name + " runs other commands or code that can't be checked; run each command directly")
	case privilege[name]:
		add(name + " changes who can do what")
	case disk[name] || strings.HasPrefix(name, "mkfs") || strings.HasPrefix(name, "fsck"):
		add(name + " works on disks or partitions")
	case boot[name]:
		add(name + " changes how the machine boots")
	case power[name]:
		add(name + " reboots or powers off the machine")
	case name == "systemctl" && len(args) > 0 && power[args[0]]:
		add("systemctl " + args[0] + " reboots or powers off the machine")
	case name == "btrfs" && !btrfsReadOnly(args):
		add("btrfs changes filesystems or subvolumes")
	case name == "snapper" && len(args) > 0 && slices.ContainsFunc(args, func(a string) bool {
		return a == "delete" || a == "rollback" || a == "undochange" || a == "delete-config"
	}):
		add("snapper deletes or rolls back snapshots")
	}

	if name == "rm" || ((name == "chmod" || name == "chown" || name == "chgrp" || name == "setfacl") && recursive(args)) {
		for _, p := range operands(args) {
			if !strings.HasPrefix(p, "/") {
				p = filepath.Join(cwd, p)
			}
			p = filepath.Clean(p)
			if name == "rm" && !recursive(args) {
				continue
			}
			if !scratch(p, home) {
				add(fmt.Sprintf("%s -r on %s, outside your home and the temp dirs", name, p))
			}
		}
	}

	if (name == "apt" || name == "apt-get" || name == "aptitude" || name == "dpkg") && removes(name, args) {
		if slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--force") }) {
			add(name + " with --force")
		}
		pkgs, err := removed(argv)
		switch {
		case err != nil:
			add(fmt.Sprintf("couldn't check what %s would remove: %v", name, err))
		case len(pkgs) > 25:
			add(fmt.Sprintf("removes %d packages", len(pkgs)))
		default:
			var core []string
			for p, isCore := range pkgs {
				if isCore {
					core = append(core, p)
				}
			}
			sort.Strings(core)
			if len(core) > 0 {
				add("removes core packages: " + strings.Join(core, ", "))
			}
		}
	}

	for _, a := range args {
		if _, v, ok := strings.Cut(a, "="); ok && strings.HasPrefix(v, "/") {
			a = v // of=/dev/sda, --target=/boot/efi
		}
		if p := protected(a, modifies[name]); p != "" {
			add("touches " + p)
		}
	}
	return risks
}

var (
	wrappers = set("sh", "bash", "dash", "zsh", "fish", "ksh", "busybox", "env", "xargs", "nohup", "nice",
		"ionice", "timeout", "stdbuf", "chroot", "systemd-run", "setsid", "flock", "strace", "watch",
		"script", "nsenter", "unshare", "ruby", "node", "nodejs", "lua", "php", "awk", "gawk", "mawk",
		"make", "tclsh", "expect", "parallel", "runuser", "sg", "chrt", "taskset", "eval", "exec", "time")
	privilege = set("sudo", "su", "pkexec", "doas", "visudo", "vipw", "vigr", "passwd", "chpasswd",
		"usermod", "userdel", "deluser", "groupdel", "delgroup", "gpasswd", "chsh", "chage", "vibian")
	disk = set("fdisk", "sfdisk", "cfdisk", "gdisk", "sgdisk", "cgdisk", "parted", "mke2fs", "mkswap",
		"wipefs", "dd", "shred", "blkdiscard", "cryptsetup", "lvremove", "vgremove", "pvremove",
		"lvreduce", "lvresize", "resize2fs", "tune2fs", "e2fsck", "zpool", "zfs", "nvme", "hdparm",
		"flashrom", "fwupdmgr", "fwupdtool", "losetup", "mdadm", "sgdisk", "ddrescue", "badblocks")
	boot  = set("grub-install", "efibootmgr", "bootctl", "kernelstub", "mokutil", "sbctl", "grub-reboot", "grub-set-default", "update-grub2")
	power = set("reboot", "poweroff", "halt", "shutdown", "kexec", "isolate", "rescue", "emergency", "kexec-load")
)

func set(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func btrfsReadOnly(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		switch a {
		case "filesystem", "fi", "subvolume", "sub", "device", "dev", "scrub", "qgroup", "property", "inspect-internal":
			continue
		case "show", "df", "du", "usage", "list", "get", "stats", "status", "find-new", "get-default", "logical-resolve", "inode-resolve":
			return true
		}
		return false
	}
	return true // bare `btrfs` or `btrfs --help`
}

func recursive(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--recursive" || (strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR")) {
			return true
		}
	}
	return false
}

// operands are the arguments that aren't options.
func operands(args []string) []string {
	var out []string
	opts := true
	for _, a := range args {
		if opts && a == "--" {
			opts = false
			continue
		}
		if opts && strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// scratch reports whether p is inside the caller's home or a temp dir (not
// the dir itself).
func scratch(p, home string) bool {
	for _, d := range []string{home, "/tmp", "/var/tmp"} {
		if d != "" && d != "/" && strings.HasPrefix(p, strings.TrimSuffix(d, "/")+"/") {
			return true
		}
	}
	return false
}

func removes(name string, args []string) bool {
	for _, a := range args {
		switch a {
		case "remove", "purge", "autoremove", "autopurge", "-r", "-P", "--remove", "--purge", "dist-upgrade", "full-upgrade":
			return true
		}
		if name == "dpkg" && strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rP") {
			return true
		}
	}
	return false
}

// protectedPaths break boot, logins or sudo when edited wrongly. Block
// devices are listed by prefix.
var protectedPaths = []string{
	"/etc/fstab", "/etc/crypttab", "/etc/default/grub", "/etc/grub.d", "/boot", "/efi",
	"/etc/sudoers", "/etc/shadow", "/etc/gshadow", "/etc/passwd", "/etc/group", "/etc/pam.d",
	"/etc/security", "/etc/snapper", "/etc/kernel", "/etc/dracut.conf", "/etc/polkit-1",
	"/etc/subuid", "/etc/subgid", "/etc/login.defs", "/var/lib/vibian", "/usr/lib/vibian",
}

var blockDevs = []string{"/dev/sd", "/dev/nvme", "/dev/vd", "/dev/hd", "/dev/xvd", "/dev/mmcblk", "/dev/mapper/", "/dev/disk/", "/dev/dm-", "/dev/md", "/dev/loop", "/dev/mem", "/dev/port"}

// modifies are commands for which naming a top-level system dir (/usr,
// /etc) is risky; anything can read them.
var modifies = set("rm", "mv", "cp", "chmod", "chown", "chgrp", "setfacl", "chattr", "ln", "rsync", "install", "truncate", "tee", "sed", "rmdir", "mount", "umount")

// protected returns the protected path arg falls under, or "". Top-level
// system dirs count only when dirs is set.
func protected(arg string, dirs bool) string {
	if !strings.HasPrefix(arg, "/") {
		return ""
	}
	p := filepath.Clean(arg)
	for _, d := range blockDevs {
		if strings.HasPrefix(p, strings.TrimSuffix(d, "/")) {
			return p
		}
	}
	for _, d := range protectedPaths {
		if p == d || strings.HasPrefix(p, d+"/") || strings.HasPrefix(p, d+".") {
			return d
		}
	}
	if !dirs {
		return ""
	}
	// the top-level system dirs themselves: `chmod -R x /usr`, `mv /etc /x`
	switch p {
	case "/", "/usr", "/etc", "/bin", "/sbin", "/lib", "/lib64", "/var", "/home", "/opt", "/srv", "/root", "/usr/bin", "/usr/lib", "/usr/sbin":
		return p
	}
	return ""
}

// CorePackages are removed only with the owner's OK, on top of everything
// apt marks essential, required or important.
var CorePackages = []string{
	"sudo", "snapper", "btrfs-progs", "grub-efi-amd64", "grub-efi-amd64-signed", "grub-efi-arm64",
	"grub-efi-arm64-signed", "grub-common", "grub2-common", "grub-btrfs", "shim-signed", "dracut", "dracut-core", "systemd", "systemd-sysv", "network-manager",
	"openssh-server", "linux-generic", "linux-image-generic", "ubuntu-minimal", "ubuntu-standard",
	"vibian-base", "git", "polkitd",
}

// Request is a root command waiting for, or holding, the owner's OK.
type Request struct {
	ID       string    `json:"id"`
	User     string    `json:"user"`
	Agent    string    `json:"agent"`
	Why      string    `json:"why"`
	Argv     []string  `json:"argv"`
	Cwd      string    `json:"cwd"`
	Risks    []string  `json:"risks"`
	Asked    time.Time `json:"asked"`
	Approved time.Time `json:"approved,omitzero"`
}

// Dir holds requests. It's on /run, so they don't survive a reboot.
var Dir = "/run/vibian/sudo"

// ApprovalTTL is how long an OK lasts before the command has to run.
const ApprovalTTL = 30 * time.Minute

// RequestID is the same for the same user, directory and command, so
// running the command again after the OK finds it.
func RequestID(user, cwd string, argv []string) string {
	h := sha256.New()
	for _, s := range append([]string{user, cwd}, argv...) {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}

func path(id string) string { return filepath.Join(Dir, id+".json") }

// Ask records r as pending, unless it's already there.
func Ask(r Request) error {
	if err := os.MkdirAll(Dir, 0o700); err != nil {
		return err
	}
	if old, err := Load(r.ID); err == nil && old.Approved.IsZero() {
		return nil
	}
	return save(r)
}

func save(r Request) error {
	b, _ := json.MarshalIndent(r, "", "  ")
	tmp := path(r.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path(r.ID))
}

// Load reads a request.
func Load(id string) (Request, error) {
	var r Request
	if strings.ContainsAny(id, "/.") || id == "" {
		return r, fmt.Errorf("bad request id %q", id)
	}
	b, err := os.ReadFile(path(id))
	if errors.Is(err, os.ErrNotExist) {
		return r, fmt.Errorf("no request %s (it ran, was denied, or the machine rebooted)", id)
	}
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}

// Pending lists requests waiting for an OK, oldest first.
func Pending() ([]Request, error) {
	files, _ := filepath.Glob(filepath.Join(Dir, "*.json"))
	var out []Request
	for _, f := range files {
		r, err := Load(strings.TrimSuffix(filepath.Base(f), ".json"))
		if err == nil && r.Approved.IsZero() {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Asked.Before(out[j].Asked) })
	return out, nil
}

// Approve marks a request approved.
func Approve(id string) (Request, error) {
	r, err := Load(id)
	if err != nil {
		return r, err
	}
	r.Approved = time.Now()
	return r, save(r)
}

// Deny drops a request.
func Deny(id string) error {
	if _, err := Load(id); err != nil {
		return err
	}
	return os.Remove(path(id))
}

// Take consumes an approval for id: true if it was approved within
// ApprovalTTL. A used or stale approval is removed either way.
func Take(id string) bool {
	r, err := Load(id)
	if err != nil || r.Approved.IsZero() {
		return false
	}
	os.Remove(path(id))
	return time.Since(r.Approved) < ApprovalTTL
}
