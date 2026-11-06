package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jacobzipper/vibian/internal/doctor"
	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/mcp"
	"github.com/jacobzipper/vibian/internal/sys"
)

const mcpInstructions = `Vibian's own tools: machine health (doctor), a hardware inventory, ` +
	`and the ledger of every change made to this computer. Read the ledger before changing ` +
	`something (ledger_search), call snapshot before risky changes, and ledger_add when done.`

// cmdMCP serves the vibian-system MCP server on stdio, or with `computer`
// the vibian-computer one. Harnesses start them with --agent <id> because
// some of them don't pass VIBIAN_AGENT through.
func cmdMCP(args []string) error {
	s := mcp.Server{Name: "vibian", Version: version, Instructions: mcpInstructions, Tools: systemTools()}
	if len(args) > 0 && args[0] == "computer" {
		s = mcp.Server{Name: "vibian-computer", Version: version, Instructions: computerInstructions, Tools: computerTools()}
		args = args[1:]
	}
	if len(args) == 2 && args[0] == "--agent" {
		os.Setenv("VIBIAN_AGENT", args[1])
	}
	return s.Serve(os.Stdin, os.Stdout)
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var (
	strProp  = func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	listProp = func(d string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": d}
	}
)

func systemTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "doctor",
			Description: "Health of this machine: boot mode, Secure Boot, btrfs root, snapshots, GPU driver, battery, disk, network, internet and failed units. Each check has a status (ok/warn/fail/info) and, when not ok, a fix.",
			ReadOnly:    true,
			Call: func(json.RawMessage) (string, error) {
				b, err := json.MarshalIndent(doctor.Run(), "", "  ")
				return string(b), err
			},
		},
		{
			Name:        "hardware",
			Description: "Hardware inventory: machine model and firmware, CPU, memory, GPUs with their drivers, network, audio and USB devices, disks and batteries.",
			ReadOnly:    true,
			Call:        func(json.RawMessage) (string, error) { return hardware(), nil },
		},
		{
			Name:        "ledger_search",
			Description: "Search the ledger (every change agents and the owner made to this machine) for words, e.g. a package, a path or a symptom. Returns matching entries with their files, packages and gotchas.",
			ReadOnly:    true,
			Schema:      schema(map[string]any{"query": strProp("words to look for")}, "query"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ Query string }
				json.Unmarshal(a, &p)
				found, err := ledger.Search(p.Query)
				if err != nil || len(found) == 0 {
					return "no matching entries", err
				}
				return joinEntries(found), nil
			},
		},
		{
			Name:        "ledger_tail",
			Description: "The newest ledger entries (default 10): what changed on this machine lately, who changed it and how to undo it.",
			ReadOnly:    true,
			Schema:      schema(map[string]any{"count": map[string]any{"type": "integer", "description": "how many entries"}}),
			Call: func(a json.RawMessage) (string, error) {
				p := struct{ Count int }{10}
				json.Unmarshal(a, &p)
				all, err := ledger.All()
				if err != nil || len(all) == 0 {
					return "the ledger is empty", err
				}
				if p.Count > 0 && len(all) > p.Count {
					all = all[len(all)-p.Count:]
				}
				return joinEntries(all), nil
			},
		},
		{
			Name:        "ledger_show",
			Description: "One ledger entry by id (e.g. L-0012).",
			ReadOnly:    true,
			Schema:      schema(map[string]any{"id": strProp("ledger id, like L-0012")}, "id"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ ID string }
				json.Unmarshal(a, &p)
				e, err := ledger.Get(p.ID)
				return e.Raw, err
			},
		},
		{
			Name:        "snapshot",
			Description: "Snapshot root and home before a risky change (packages, drivers, boot, /etc, several dotfiles). The next ledger_add closes the pair, so that change can be undone with `vibian undo <id>`.",
			Schema:      schema(map[string]any{"why": strProp("what you're about to change")}, "why"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ Why string }
				json.Unmarshal(a, &p)
				if strings.TrimSpace(p.Why) == "" {
					return "", errors.New("why is required")
				}
				return self("snap", p.Why)
			},
		},
		{
			Name:        "ledger_add",
			Description: "Record a change you made in the ledger, with the files and packages it touched and anything the next agent should know. Closes the snapshot taken by `snapshot`. Never put secrets in it.",
			Schema: schema(map[string]any{
				"summary":  strProp("what you did and why, one line"),
				"files":    listProp("paths you changed"),
				"packages": listProp("packages, +added or -removed"),
				"gotchas":  listProp("things the next agent should know"),
				"undo":     strProp("how to undo it, if not just `vibian undo <id>`"),
			}, "summary"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct {
					Summary                  string
					Files, Packages, Gotchas []string
					Undo                     string
				}
				json.Unmarshal(a, &p)
				if strings.TrimSpace(p.Summary) == "" {
					return "", errors.New("summary is required")
				}
				args := []string{"log", "add", p.Summary}
				for _, f := range p.Files {
					args = append(args, "--files", f)
				}
				for _, k := range p.Packages {
					args = append(args, "--pkgs", k)
				}
				for _, g := range p.Gotchas {
					args = append(args, "--gotcha", g)
				}
				if p.Undo != "" {
					args = append(args, "--undo", p.Undo)
				}
				return self(args...)
			},
		},
	}
}

func joinEntries(es []ledger.Entry) string {
	var b strings.Builder
	for _, e := range es {
		b.WriteString(e.Raw + "\n\n")
	}
	return strings.TrimSpace(b.String())
}

// self runs this binary for commands that print and track pending snapshots
// the same way the CLI does.
func self(args ...string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(exe, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func hardware() string {
	var b strings.Builder
	section := func(title, body string) {
		if body = strings.TrimSpace(body); body != "" {
			fmt.Fprintf(&b, "## %s\n%s\n\n", title, body)
		}
	}
	dmi := func(k string) string { return sys.ReadFile("/sys/class/dmi/id/" + k) }
	section("Machine", fmt.Sprintf("%s %s %s\nfirmware: %s %s", dmi("sys_vendor"), dmi("product_name"), dmi("product_version"), dmi("bios_vendor"), dmi("bios_version")))
	if out, err := sys.Output("lscpu"); err == nil {
		var keep []string
		for _, l := range strings.Split(out, "\n") {
			for _, k := range []string{"Model name:", "CPU(s):", "Thread(s) per core:", "Virtualization:"} {
				if strings.HasPrefix(strings.TrimSpace(l), k) {
					keep = append(keep, strings.Join(strings.Fields(l), " "))
				}
			}
		}
		section("CPU", strings.Join(keep, "\n"))
	}
	if out, err := sys.Output("free", "-h"); err == nil {
		section("Memory", out)
	}
	var gpus []string
	for _, g := range doctor.GPUs() {
		gpus = append(gpus, fmt.Sprintf("%s %s (driver: %s)", g.Vendor, g.Model, orNone(g.Driver)))
	}
	section("GPUs", strings.Join(gpus, "\n"))
	if out, err := sys.Output("lspci", "-k"); err == nil {
		section("PCI devices", pciSummary(out))
	}
	if out, err := sys.Output("lsusb"); err == nil {
		section("USB", out)
	}
	if out, err := sys.Output("lsblk", "-o", "NAME,SIZE,TYPE,FSTYPE,MOUNTPOINTS", "-e", "7"); err == nil {
		section("Disks", out)
	}
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	var bl []string
	for _, p := range bats {
		r := func(k string) string { return sys.ReadFile(filepath.Join(p, k)) }
		line := fmt.Sprintf("%s: %s%% %s", filepath.Base(p), r("capacity"), strings.ToLower(r("status")))
		if full, design := r("energy_full"), r("energy_full_design"); full != "" && design != "" {
			line += fmt.Sprintf(", health %s/%s µWh", full, design)
		}
		if t := r("charge_control_end_threshold"); t != "" {
			line += ", charge limit " + t + "%"
		}
		bl = append(bl, line)
	}
	section("Batteries", strings.Join(bl, "\n"))
	if out, err := sys.Output("nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device"); err == nil {
		section("Network devices (nmcli)", out)
	}
	return strings.TrimSpace(b.String())
}

// pciSummary lists `lspci -k` devices with the kernel driver in use,
// skipping bridges.
func pciSummary(out string) string {
	var keep []string
	var dev string
	flush := func(driver string) {
		if dev != "" {
			keep = append(keep, dev+" (driver: "+orNone(driver)+")")
		}
		dev = ""
	}
	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, "\t") {
			flush("")
			lower := strings.ToLower(l)
			if strings.Contains(lower, "bridge") {
				continue
			}
			dev = l
			continue
		}
		if d, ok := strings.CutPrefix(strings.TrimSpace(l), "Kernel driver in use: "); ok && dev != "" {
			flush(d)
		}
	}
	flush("")
	return strings.Join(keep, "\n")
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
