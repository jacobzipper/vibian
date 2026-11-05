// Package doctor checks the health of a Vibian machine. Every check is
// read-only and fast enough to run on each onboarding screen refresh.
package doctor

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jacobzipper/vibian/internal/sys"
)

// Status of a check.
type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
	Info Status = "info"
)

// Check is one line of the doctor report.
type Check struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// Groups orders the checks for display.
var Groups = []struct {
	Name   string
	Checks []func() Check
}{
	{"System", []func() Check{osRelease, bootMode, secureBoot, failedUnits}},
	{"Snapshots", []func() Check{rootFS, snapperConfigs, grubBtrfs}},
	{"Network", []func() Check{network, dns}},
	{"Hardware", []func() Check{gpu, battery, disk}},
}

// Run runs every check.
func Run() []Check {
	var out []Check
	for _, g := range Groups {
		for _, c := range g.Checks {
			out = append(out, c())
		}
	}
	return out
}

func osRelease() Check {
	c := Check{ID: "os", Name: "OS", Status: Info}
	for _, ln := range strings.Split(sys.ReadFile("/etc/os-release"), "\n") {
		if v, ok := strings.CutPrefix(ln, "PRETTY_NAME="); ok {
			c.Detail = strings.Trim(v, `"`)
		}
	}
	if k, err := sys.Output("uname", "-r"); err == nil {
		c.Detail += ", kernel " + k
	}
	return c
}

func bootMode() Check {
	if _, err := os.Stat("/sys/firmware/efi"); err == nil {
		return Check{ID: "boot", Name: "Boot mode", Status: OK, Detail: "UEFI"}
	}
	return Check{ID: "boot", Name: "Boot mode", Status: Warn, Detail: "legacy BIOS", Fix: "Vibian is tested on UEFI; switch the firmware to UEFI mode if you can"}
}

func secureBoot() Check {
	c := Check{ID: "secure-boot", Name: "Secure Boot", Status: Info}
	if !sys.Has("mokutil") {
		c.Detail = "unknown (mokutil not installed)"
		return c
	}
	out, _ := sys.Output("mokutil", "--sb-state")
	switch {
	case strings.Contains(out, "enabled"):
		c.Detail = "on: out-of-tree drivers (NVIDIA) need a MOK key, which ubuntu-drivers sets up"
	case strings.Contains(out, "disabled"):
		c.Detail = "off"
	default:
		c.Detail = "not supported"
	}
	return c
}

func failedUnits() Check {
	c := Check{ID: "units", Name: "Services", Status: OK, Detail: "nothing failed"}
	out, err := sys.Output("systemctl", "--failed", "--no-legend", "--plain")
	if err != nil {
		c.Status, c.Detail = Info, "systemd not reachable"
		return c
	}
	var names []string
	for _, ln := range strings.Split(out, "\n") {
		if f := strings.Fields(ln); len(f) > 0 {
			names = append(names, f[0])
		}
	}
	if len(names) > 0 {
		c.Status, c.Detail = Warn, strings.Join(names, ", ")+" failed"
		c.Fix = "journalctl -b -u " + names[0]
	}
	return c
}

// overlayRoot reports whether we booted a snapshot (RAM overlay root).
func overlayRoot() bool {
	return strings.Contains(sys.ReadFile("/proc/cmdline"), "rd.overlay")
}

func rootFS() Check {
	c := Check{ID: "rootfs", Name: "Root filesystem"}
	fstype, _ := sys.Output("findmnt", "-no", "FSTYPE", "/")
	opts, _ := sys.Output("findmnt", "-no", "OPTIONS", "/")
	switch {
	case overlayRoot():
		c.Status, c.Detail = Warn, "running from a snapshot; changes vanish at reboot"
		c.Fix = "vibian undo --to <snapshot> makes this snapshot the system"
	case fstype == "btrfs" && strings.Contains(opts, "subvol=/@"):
		c.Status, c.Detail = OK, "btrfs, subvolume @"
	case fstype == "btrfs":
		c.Status, c.Detail = Warn, "btrfs, but not on the @ subvolume"
	default:
		c.Status, c.Detail = Fail, fstype+": snapshots need btrfs"
	}
	return c
}

func snapperConfigs() Check {
	c := Check{ID: "snapper", Name: "Snapshots"}
	var missing []string
	for _, cfg := range []string{"root", "home"} {
		if _, err := os.Stat(filepath.Join("/etc/snapper/configs", cfg)); err != nil {
			missing = append(missing, cfg)
		}
	}
	if !sys.Has("snapper") || len(missing) > 0 {
		c.Status, c.Detail = Fail, "snapper config missing: "+strings.Join(missing, ", ")
		return c
	}
	c.Status, c.Detail = OK, "root and home configs"
	if _, err := os.Stat("/etc/apt/apt.conf.d/80snapper"); err == nil {
		c.Detail += ", apt pre/post hook"
	}
	return c
}

func grubBtrfs() Check {
	c := Check{ID: "grub-btrfs", Name: "Boot to snapshot"}
	if out, _ := sys.Output("systemctl", "is-active", "grub-btrfsd"); out == "active" {
		c.Status, c.Detail = OK, "GRUB lists snapshots (grub-btrfsd running)"
		return c
	}
	c.Status, c.Detail, c.Fix = Warn, "grub-btrfsd is not running", "sudo systemctl enable --now grub-btrfsd"
	return c
}

func network() Check {
	c := Check{ID: "network", Name: "Network"}
	out, _ := sys.Output("ip", "-o", "route", "show", "default")
	if out == "" {
		c.Status, c.Detail = Fail, "no default route"
		c.Fix = "plug in a cable, or pick a wifi network in `vibian onboard`"
		return c
	}
	f := strings.Fields(out)
	for i := range f {
		if f[i] == "dev" && i+1 < len(f) {
			c.Detail = "via " + f[i+1]
		}
	}
	c.Status = OK
	return c
}

func dns() Check {
	c := Check{ID: "dns", Name: "Internet"}
	// several hosts: some networks block or can't resolve one of them
	hosts := []string{"archive.ubuntu.com", "github.com", "pi.dev"}
	type res struct {
		host string
		err  error
	}
	done := make(chan res, len(hosts))
	for _, h := range hosts {
		go func() {
			conn, err := net.DialTimeout("tcp", h+":443", 3*time.Second)
			if err == nil {
				conn.Close()
			}
			done <- res{h, err}
		}()
	}
	var failed []string
	resolved := false
	timeout := time.After(4 * time.Second)
	for range hosts {
		select {
		case r := <-done:
			if r.err == nil {
				c.Status, c.Detail = OK, "online (reached "+r.host+")"
				return c
			}
			var dnsErr *net.DNSError
			if !errors.As(r.err, &dnsErr) {
				resolved = true
			}
			failed = append(failed, r.host)
		case <-timeout:
			c.Status, c.Detail, c.Fix = Fail, "no answer from "+strings.Join(hosts, ", "), "resolvectl status; ip route"
			return c
		}
	}
	if resolved {
		c.Status, c.Detail, c.Fix = Fail, "DNS works but "+strings.Join(failed, ", ")+" are unreachable", "ip route; is a proxy or captive portal in the way?"
		return c
	}
	c.Status, c.Detail, c.Fix = Fail, "can't resolve "+strings.Join(failed, ", "), "resolvectl status"
	return c
}

// GPU describes one display adapter.
type GPU struct {
	Vendor, Model, Driver string
}

// GPUs lists display adapters from sysfs (no lspci needed).
func GPUs() []GPU {
	var out []GPU
	devs, _ := filepath.Glob("/sys/bus/pci/devices/*")
	for _, d := range devs {
		class := sys.ReadFile(filepath.Join(d, "class"))
		if !strings.HasPrefix(class, "0x03") {
			continue
		}
		g := GPU{Vendor: vendorName(sys.ReadFile(filepath.Join(d, "vendor")))}
		if link, err := os.Readlink(filepath.Join(d, "driver")); err == nil {
			g.Driver = filepath.Base(link)
		}
		g.Model = sys.ReadFile(filepath.Join(d, "label"))
		if g.Model == "" {
			g.Model = filepath.Base(d)
		}
		out = append(out, g)
	}
	return out
}

func vendorName(id string) string {
	switch id {
	case "0x10de":
		return "NVIDIA"
	case "0x1002":
		return "AMD"
	case "0x8086":
		return "Intel"
	case "0x1af4", "0x1234", "0x15ad", "0x80ee", "0x1b36":
		return "virtual"
	}
	return id
}

func gpu() Check {
	c := Check{ID: "gpu", Name: "Graphics"}
	gs := GPUs()
	if len(gs) == 0 {
		c.Status, c.Detail = Info, "no display adapter (headless)"
		return c
	}
	c.Status = OK
	var parts []string
	for _, g := range gs {
		d := g.Driver
		if d == "" {
			d = "no driver"
			if g.Vendor == "NVIDIA" {
				c.Status, c.Fix = Fail, "sudo ubuntu-drivers install"
			} else {
				c.Status = Warn
			}
		}
		if g.Vendor == "NVIDIA" && d == "nouveau" {
			c.Status, c.Fix = Warn, "sudo ubuntu-drivers install (the NVIDIA driver is faster and supports CUDA)"
		}
		parts = append(parts, g.Vendor+" ("+d+")")
	}
	c.Detail = strings.Join(parts, " + ")
	return c
}

func battery() Check {
	c := Check{ID: "battery", Name: "Battery", Status: Info, Detail: "none (desktop or VM)"}
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(bats) == 0 {
		return c
	}
	cap := sys.ReadFile(filepath.Join(bats[0], "capacity"))
	st := strings.ToLower(sys.ReadFile(filepath.Join(bats[0], "status")))
	c.Status, c.Detail = OK, cap+"%, "+st
	return c
}

func disk() Check {
	c := Check{ID: "disk", Name: "Disk"}
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		c.Status, c.Detail = Info, err.Error()
		return c
	}
	free := float64(st.Bavail) * float64(st.Bsize)
	total := float64(st.Blocks) * float64(st.Bsize)
	c.Detail = humanBytes(free) + " free of " + humanBytes(total)
	c.Status = OK
	if free < 0.1*total || free < 5e9 {
		c.Status, c.Fix = Warn, "snapper cleanup: sudo snapper -c root cleanup number"
	}
	return c
}

func humanBytes(b float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for b >= 1000 && i < len(units)-1 {
		b /= 1000
		i++
	}
	return strconv.FormatFloat(b, 'f', 0, 64) + " " + units[i]
}
