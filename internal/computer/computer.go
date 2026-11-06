// Package computer drives the graphical session for the vibian-computer MCP
// server: screenshots, windows, the pointer and the keyboard.
//
// Every desktop gets a backend. wlroots compositors (Hyprland, Sway, niri and
// the rest) take screenshots with grim and type with wtype; COSMIC is the
// same but for screenshots, which go through its portal; KDE uses
// spectacle and KWin scripts; X11 uses maim, wmctrl and xdotool. Where the
// compositor has no way to place the pointer and press buttons, a uinput
// device that behaves like a VM's USB tablet does it, so absolute positions
// work on any compositor.
package computer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Rect is an area in screen (logical, layout) pixels.
type Rect struct{ X, Y, W, H int }

func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Monitor is one output in layout coordinates.
type Monitor struct {
	Name  string  `json:"name"`
	X     int     `json:"x"`
	Y     int     `json:"y"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	Scale float64 `json:"scale"`
}

// Window is one toplevel window. ID is what Focus takes.
type Window struct {
	ID        string `json:"id"`
	App       string `json:"app"`
	Title     string `json:"title"`
	Workspace string `json:"workspace,omitempty"`
	At        [2]int `json:"at"`
	Size      [2]int `json:"size"`
	Floating  bool   `json:"floating,omitempty"`
	Focused   bool   `json:"focused,omitempty"`
}

// Desktop is a backend for one kind of graphical session.
type Desktop interface {
	Name() string
	Monitors() ([]Monitor, error)
	Windows() ([]Window, error)
	Focus(id string) error
	// Capture returns a PNG of r (everything when r is empty), with each
	// layout pixel scaled by scale.
	Capture(r Rect, scale float64, cursor bool) ([]byte, error)
	Type(text string) error
	Key(combo string) error
	// Pointer is the backend's own pointer, or nil to use uinput.
	Pointer() Pointer
}

// Pointer places the pointer and presses buttons.
type Pointer interface {
	Move(x, y int) error
	Button(name string, down bool) error
	// Scroll turns the wheel by notches; dy > 0 scrolls down, dx > 0 right.
	Scroll(dy, dx int) error
	// Position is where the pointer is, when the desktop can say.
	Position() (x, y int, ok bool)
}

// ErrNoSession means there is no graphical session to drive.
var ErrNoSession = errors.New("no graphical session found: computer use needs a desktop (Wayland or X11) running for this user; on a bare TTY there is nothing to see")

// Detect finds the running desktop. It fills in WAYLAND_DISPLAY, DISPLAY and
// the compositor sockets when the agent was started outside the session
// (from a TTY, over SSH, or by a service started before the desktop).
func Detect() (Desktop, error) {
	fixEnv()
	procs := ownProcesses()
	wayland := os.Getenv("WAYLAND_DISPLAY") != ""
	switch {
	case procs["Hyprland"] || procs["hyprland"]:
		return hyprland{}, nil
	case procs["sway"]:
		return sway{}, nil
	case procs["niri"]:
		return niri{}, nil
	case procs["cosmic-comp"]:
		return cosmic{}, nil
	case procs["kwin_wayland"]:
		return kde{}, nil
	case procs["gnome-shell"] && wayland:
		return gnome{}, nil
	case wayland:
		return wlroots{}, nil
	case os.Getenv("DISPLAY") != "":
		return x11{}, nil
	}
	return nil, ErrNoSession
}

// fixEnv points this process at the user's session, preferring what the
// environment already says when it still exists.
func fixEnv() {
	run := os.Getenv("XDG_RUNTIME_DIR")
	if run == "" {
		run = fmt.Sprintf("/run/user/%d", os.Getuid())
		os.Setenv("XDG_RUNTIME_DIR", run)
	}
	// portals (COSMIC's screenshots) and qdbus6 need the session bus
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" && exists(filepath.Join(run, "bus")) {
		os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(run, "bus"))
	}
	if w := os.Getenv("WAYLAND_DISPLAY"); w == "" || !exists(filepath.Join(run, w)) {
		if socks := newest(filepath.Join(run, "wayland-*"), ".lock"); len(socks) > 0 {
			os.Setenv("WAYLAND_DISPLAY", filepath.Base(socks[0]))
		} else {
			os.Unsetenv("WAYLAND_DISPLAY")
		}
	}
	if s := newest(filepath.Join(run, "hypr/*/.socket.sock"), ""); len(s) > 0 {
		sig := filepath.Base(filepath.Dir(s[0]))
		if cur := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"); cur == "" || !exists(filepath.Join(run, "hypr", cur, ".socket.sock")) {
			os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", sig)
		}
	}
	if s := os.Getenv("SWAYSOCK"); s == "" || !exists(s) {
		if socks := newest(filepath.Join(run, fmt.Sprintf("sway-ipc.%d.*.sock", os.Getuid())), ""); len(socks) > 0 {
			os.Setenv("SWAYSOCK", socks[0])
		}
	}
	if s := os.Getenv("NIRI_SOCKET"); s == "" || !exists(s) {
		if socks := newest(filepath.Join(run, "niri.*.sock"), ""); len(socks) > 0 {
			os.Setenv("NIRI_SOCKET", socks[0])
		}
	}
	if d := os.Getenv("DISPLAY"); d == "" || !exists("/tmp/.X11-unix/X"+strings.TrimPrefix(strings.SplitN(d, ".", 2)[0], ":")) {
		if xs := newest("/tmp/.X11-unix/X*", ""); len(xs) > 0 {
			os.Setenv("DISPLAY", ":"+strings.TrimPrefix(filepath.Base(xs[len(xs)-1]), "X"))
		} else {
			os.Unsetenv("DISPLAY")
		}
	}
	if os.Getenv("DISPLAY") != "" && os.Getenv("XAUTHORITY") == "" {
		home, _ := os.UserHomeDir()
		for _, p := range append(newest(filepath.Join(run, ".mutter-Xwaylandauth.*"), ""),
			append(newest(filepath.Join(run, "xauth_*"), ""), filepath.Join(home, ".Xauthority"))...) {
			if exists(p) {
				os.Setenv("XAUTHORITY", p)
				break
			}
		}
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// newest globs pattern, newest first, skipping names ending in skip.
func newest(pattern, skip string) []string {
	ms, _ := filepath.Glob(pattern)
	ms = slices.DeleteFunc(ms, func(m string) bool { return skip != "" && strings.HasSuffix(m, skip) })
	mtime := func(p string) int64 {
		if st, err := os.Stat(p); err == nil {
			return st.ModTime().UnixNano()
		}
		return 0
	}
	slices.SortStableFunc(ms, func(a, b string) int { return int(mtime(b) - mtime(a)) })
	return ms
}

// ownProcesses is the set of command names this user is running.
func ownProcesses() map[string]bool {
	out := map[string]bool{}
	ds, _ := os.ReadDir("/proc")
	uid := uint32(os.Getuid())
	for _, d := range ds {
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		if st, ok := info.Sys().(*syscall.Stat_t); !ok || st.Uid != uid {
			continue
		}
		if b, err := os.ReadFile("/proc/" + d.Name() + "/comm"); err == nil {
			out[strings.TrimSpace(string(b))] = true
		}
	}
	return out
}

// run runs a desktop tool, naming the package when it isn't installed.
func run(name string, args ...string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", fmt.Errorf("%s is not installed (sudo apt install %s)", name, pkgOf(name))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(out))
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s did not finish in 15s (is the screen asleep or locked?)", name)
		}
		return "", fmt.Errorf("%s failed: %v: %s", name, err, msg)
	}
	return string(out), nil
}

func runJSON(v any, name string, args ...string) error {
	out, err := run(name, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(out), v); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	return nil
}

func pkgOf(tool string) string {
	switch tool {
	case "hyprctl":
		return "hyprland"
	case "swaymsg":
		return "sway"
	case "qdbus6", "gdbus":
		return "libglib2.0-bin"
	case "kscreen-doctor":
		return "libkscreen-bin"
	case "xrandr":
		return "x11-xserver-utils"
	case "journalctl":
		return "systemd"
	}
	return tool
}

// Bounds is the box around all monitors.
func Bounds(ms []Monitor) Rect {
	if len(ms) == 0 {
		return Rect{}
	}
	x0, y0, x1, y1 := ms[0].X, ms[0].Y, ms[0].X+ms[0].W, ms[0].Y+ms[0].H
	for _, m := range ms[1:] {
		x0, y0 = min(x0, m.X), min(y0, m.Y)
		x1, y1 = max(x1, m.X+m.W), max(y1, m.Y+m.H)
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// ParseRegion reads "x,y,w,h".
func ParseRegion(s string) (Rect, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Rect{}, nil
	}
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == 'x' })
	if len(f) != 4 {
		return Rect{}, fmt.Errorf("region %q: want x,y,w,h", s)
	}
	var n [4]int
	for i, v := range f {
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Rect{}, fmt.Errorf("region %q: %v", s, err)
		}
		n[i] = int(x)
	}
	r := Rect{n[0], n[1], n[2], n[3]}
	if r.Empty() {
		return Rect{}, fmt.Errorf("region %q is empty", s)
	}
	return r, nil
}

// Combo splits a key combo like "ctrl+shift+t" into modifiers (ctrl, shift,
// alt, super, altgr) and the key.
func Combo(combo string) (mods []string, key string, err error) {
	parts := strings.Split(strings.TrimSpace(combo), "+")
	if n := len(parts); n > 2 && parts[n-1] == "" && parts[n-2] == "" { // "ctrl++"
		parts = append(parts[:len(parts)-2], "plus")
	}
	for _, p := range parts[:len(parts)-1] {
		m, ok := map[string]string{"ctrl": "ctrl", "control": "ctrl", "shift": "shift", "alt": "alt",
			"super": "super", "meta": "super", "win": "super", "logo": "super", "cmd": "super", "altgr": "altgr"}[strings.ToLower(strings.TrimSpace(p))]
		if !ok {
			return nil, "", fmt.Errorf("unknown modifier %q in %q (use ctrl, shift, alt, super, altgr)", p, combo)
		}
		mods = append(mods, m)
	}
	key = strings.TrimSpace(parts[len(parts)-1])
	if key == "" {
		return nil, "", fmt.Errorf("no key in %q", combo)
	}
	if k, ok := keyAliases[strings.ToLower(key)]; ok {
		key = k
	}
	return mods, key, nil
}

// keyAliases maps common names to XKB keysyms.
var keyAliases = map[string]string{
	"enter": "Return", "return": "Return", "esc": "Escape", "escape": "Escape", "tab": "Tab",
	"backspace": "BackSpace", "del": "Delete", "delete": "Delete", "ins": "Insert", "insert": "Insert",
	"home": "Home", "end": "End", "pageup": "Page_Up", "pgup": "Page_Up", "page_up": "Page_Up", "prior": "Page_Up",
	"pagedown": "Page_Down", "pgdn": "Page_Down", "page_down": "Page_Down", "next": "Page_Down",
	"left": "Left", "right": "Right", "up": "Up", "down": "Down", "space": "space", "spacebar": "space",
	"print": "Print", "menu": "Menu", "plus": "plus", "minus": "minus",
}
