// Package wifi lists and joins wifi networks through NetworkManager (nmcli).
package wifi

import (
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/jacobzipper/vibian/internal/sys"
)

// Network is one visible wifi network.
type Network struct {
	SSID     string
	Signal   int    // 0-100
	Security string // "" for open networks
	InUse    bool
}

// Device returns the wifi device to join networks with, or "" if
// NetworkManager isn't running or sees no wifi device.
func Device() string {
	if !sys.Has("nmcli") {
		return ""
	}
	out, err := sys.Output("nmcli", "-t", "-f", "DEVICE,TYPE,STATE", "device")
	if err != nil {
		return ""
	}
	return pickDevice(out)
}

// pickDevice prefers an idle wifi device, so a second radio that's busy (a
// hotspot, say) isn't taken over.
func pickDevice(out string) string {
	first := ""
	for _, line := range strings.Split(out, "\n") {
		f := fields(line)
		if len(f) != 3 || f[1] != "wifi" {
			continue
		}
		if f[2] == "disconnected" {
			return f[0]
		}
		if first == "" {
			first = f[0]
		}
	}
	return first
}

// Scan asks NetworkManager for a fresh scan on dev and returns what it sees.
func Scan(dev string) ([]Network, error) {
	out, err := sys.Output("nmcli", "-t", "-f", "IN-USE,SIGNAL,SECURITY,SSID", "device", "wifi", "list", "ifname", dev, "--rescan", "yes")
	if err != nil {
		return nil, err
	}
	return Parse(out), nil
}

// ConnectCmd joins a network on dev; --ask prompts for the password on the
// terminal.
func ConnectCmd(dev, ssid string) *exec.Cmd {
	return exec.Command("nmcli", "--ask", "device", "wifi", "connect", ssid, "ifname", dev)
}

// Parse reads `nmcli -t -f IN-USE,SIGNAL,SECURITY,SSID device wifi list`.
// Hidden networks are dropped, each SSID is kept once (strongest access
// point), and the result is sorted by signal.
func Parse(out string) []Network {
	best := map[string]Network{}
	for _, line := range strings.Split(out, "\n") {
		f := fields(line)
		if len(f) != 4 || f[3] == "" {
			continue
		}
		sig, _ := strconv.Atoi(f[1])
		sec := f[2]
		if sec == "--" {
			sec = ""
		}
		n := Network{SSID: f[3], Signal: sig, Security: sec, InUse: f[0] == "*"}
		if old, ok := best[n.SSID]; ok {
			n.InUse = n.InUse || old.InUse
			if old.Signal > n.Signal {
				n.Signal, n.Security = old.Signal, old.Security
			}
		}
		best[n.SSID] = n
	}
	nets := make([]Network, 0, len(best))
	for _, n := range best {
		nets = append(nets, n)
	}
	sort.Slice(nets, func(i, j int) bool {
		if nets[i].Signal != nets[j].Signal {
			return nets[i].Signal > nets[j].Signal
		}
		return nets[i].SSID < nets[j].SSID
	})
	return nets
}

// fields splits an nmcli terse line on ':', honouring its \: and \\ escapes.
func fields(line string) []string {
	var f []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
		case c == ':':
			f = append(f, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(f, cur.String())
}
