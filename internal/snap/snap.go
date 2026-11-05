// Package snap wraps snapper for Vibian's root and home configs.
package snap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jacobzipper/vibian/internal/sys"
)

// Configs are the snapper configs Vibian snapshots together.
var Configs = []string{"root", "home"}

// Set maps a snapper config to a snapshot number.
type Set map[string]int

// String renders a set as "root 12, home 4".
func (s Set) String() string {
	var parts []string
	for _, c := range Configs {
		if n, ok := s[c]; ok {
			parts = append(parts, fmt.Sprintf("%s %d", c, n))
		}
	}
	return strings.Join(parts, ", ")
}

// Pair renders a pre/post pair as "root 12→13, home 4→5".
func Pair(pre, post Set) string {
	var parts []string
	for _, c := range Configs {
		a, okA := pre[c]
		b, okB := post[c]
		switch {
		case okA && okB:
			parts = append(parts, fmt.Sprintf("%s %d→%d", c, a, b))
		case okB:
			parts = append(parts, fmt.Sprintf("%s %d", c, b))
		}
	}
	return strings.Join(parts, ", ")
}

// Available reports whether snapper and the root config exist.
func Available() bool {
	_, err := os.Stat("/etc/snapper/configs/root")
	return sys.Has("snapper") && err == nil
}

func create(config string, args ...string) (int, error) {
	// ALLOW_GROUPS="sudo" in Vibian's snapper configs lets the user's own
	// account create snapshots over D-Bus, so no sudo here.
	argv := append([]string{"-c", config, "create", "--cleanup-algorithm", "number", "--print-number"}, args...)
	out, err := sys.Output("snapper", argv...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// Pre takes a "pre" snapshot of every config.
func Pre(desc string) (Set, error) {
	s := Set{}
	for _, c := range Configs {
		n, err := create(c, "--type", "pre", "-d", desc)
		if err != nil {
			return s, err
		}
		s[c] = n
	}
	return s, nil
}

// Post closes a pre snapshot set. Configs without a pre get a single snapshot.
func Post(pre Set, desc string) (Set, error) {
	s := Set{}
	for _, c := range Configs {
		var n int
		var err error
		if p, ok := pre[c]; ok {
			n, err = create(c, "--type", "post", "--pre-number", strconv.Itoa(p), "-d", desc)
		} else {
			n, err = create(c, "-d", desc)
		}
		if err != nil {
			return s, err
		}
		s[c] = n
	}
	return s, nil
}

// pendingPath is where `vibian snap` leaves its pre snapshots for the next
// `vibian log add` to close.
func pendingPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "vibian", "pending-snap.json")
}

// Pending is an open pre snapshot set.
type Pending struct {
	Why string `json:"why"`
	Pre Set    `json:"pre"`
}

// SavePending records an open pre snapshot set.
func SavePending(p Pending) error {
	path := pendingPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(p)
	return os.WriteFile(path, b, 0o644)
}

// TakePending returns and clears the open pre snapshot set, if any.
func TakePending() (*Pending, error) {
	path := pendingPath()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Pending
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, os.Remove(path)
}

// UndoChange reverts the files that changed between two snapshots.
func UndoChange(config string, from, to int) error {
	return sys.Run(sys.AsRoot("snapper", "-c", config, "undochange", fmt.Sprintf("%d..%d", from, to))...)
}

// Snapshot is one row of `snapper list`.
type Snapshot struct {
	Number int    `json:"number"`
	Type   string `json:"type"`
	Date   string `json:"date"`
	Desc   string `json:"description"`
}

// List returns a config's snapshots, oldest first.
func List(config string) ([]Snapshot, error) {
	out, err := sys.Output("snapper", "--jsonout", "-c", config, "list", "--disable-used-space")
	if err != nil {
		return nil, err
	}
	var raw map[string][]Snapshot
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, err
	}
	return raw[config], nil
}
