package computer

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// wlroots is any wlroots compositor: grim for screenshots, wtype for the
// keyboard, the uinput tablet for the pointer. Hyprland, Sway and niri embed
// it and add windows and monitors from their own IPC.
type wlroots struct{}

func (wlroots) Name() string { return "wayland (wlroots)" }

func (wlroots) Capture(r Rect, scale float64, cursor bool) ([]byte, error) {
	args := []string{"-s", strconv.FormatFloat(scale, 'f', 4, 64)}
	if cursor {
		args = append(args, "-c")
	}
	if !r.Empty() {
		args = append(args, "-g", fmt.Sprintf("%d,%d %dx%d", r.X, r.Y, r.W, r.H))
	}
	out, err := run("grim", append(args, "-")...)
	return []byte(out), err
}

// Monitors without compositor IPC: one screen the size of a full screenshot.
func (w wlroots) Monitors() ([]Monitor, error) {
	shot, err := w.Capture(Rect{}, 1, false)
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(shot))
	if err != nil {
		return nil, err
	}
	return []Monitor{{Name: "screen", W: cfg.Width, H: cfg.Height, Scale: 1}}, nil
}

func (wlroots) Windows() ([]Window, error) {
	return nil, errors.New("this compositor has no window list Vibian knows; use screenshots, and the compositor's own keys to switch windows")
}

func (wlroots) Focus(string) error {
	return errors.New("this compositor has no focus command Vibian knows; click the window or use its keys")
}

// uinputKeys says to type through the uinput keyboard, as on KDE: tests
// use it to reach that path on Sway.
func uinputKeys() bool { return os.Getenv("VIBIAN_COMPUTER_KEYBOARD") == "uinput" }

func (wlroots) Type(text string) error {
	if uinputKeys() {
		return withKeyboard(func(k *Keyboard) error { return k.Type(text) })
	}
	_, err := run("wtype", append(wtypeSettle, "-d", "8", "--", text)...)
	return err
}

// wtypeSettle gives the compositor time to hand wtype's keymap to the
// focused client; without it the first key can be lost.
var wtypeSettle = []string{"-s", "120"}

func (wlroots) Key(combo string) error {
	if uinputKeys() {
		return withKeyboard(func(k *Keyboard) error { return k.Key(combo) })
	}
	mods, key, err := Combo(combo)
	if err != nil {
		return err
	}
	args := slices.Clone(wtypeSettle)
	for _, m := range mods {
		args = append(args, "-M", wtypeMod(m))
	}
	args = append(args, "-k", key)
	for i := len(mods) - 1; i >= 0; i-- {
		args = append(args, "-m", wtypeMod(mods[i]))
	}
	_, err = run("wtype", args...)
	return err
}

func wtypeMod(m string) string {
	if m == "super" {
		return "logo"
	}
	return m
}

func (wlroots) Pointer() Pointer { return nil }

// hyprland talks to Hyprland through hyprctl.
type hyprland struct{ wlroots }

func (hyprland) Name() string { return "Hyprland" }

func (hyprland) Monitors() ([]Monitor, error) {
	var ms []struct {
		Name                string
		X, Y, Width, Height int
		Scale               float64
		Transform           int
		Disabled            bool
	}
	if err := runJSON(&ms, "hyprctl", "-j", "monitors"); err != nil {
		return nil, err
	}
	var out []Monitor
	for _, m := range ms {
		if m.Disabled {
			continue
		}
		s := m.Scale
		if s <= 0 {
			s = 1
		}
		w, h := int(math.Round(float64(m.Width)/s)), int(math.Round(float64(m.Height)/s))
		if m.Transform%2 == 1 {
			w, h = h, w
		}
		out = append(out, Monitor{m.Name, m.X, m.Y, w, h, s})
	}
	return out, nil
}

// Capture refuses up front when the screens are asleep: Hyprland renders
// nothing then and grim would hang until it times out.
func (h hyprland) Capture(r Rect, scale float64, cursor bool) ([]byte, error) {
	var ms []struct {
		DpmsStatus *bool `json:"dpmsStatus"`
		Disabled   bool
	}
	if runJSON(&ms, "hyprctl", "-j", "monitors") == nil {
		asleep := 0
		for _, m := range ms {
			if m.DpmsStatus != nil && !*m.DpmsStatus {
				asleep++
			}
		}
		if asleep > 0 && asleep == len(ms) {
			return nil, errors.New("the screen is off (DPMS), so there is nothing to capture; ask the user before waking it with `hyprctl dispatch dpms on`")
		}
	}
	return h.wlroots.Capture(r, scale, cursor)
}

type hyprClient struct {
	Address, Class, Title string
	Mapped, Hidden        bool
	Floating              bool
	At, Size              [2]int
	Workspace             struct{ Name string }
}

func (hyprland) Windows() ([]Window, error) {
	var cs []hyprClient
	if err := runJSON(&cs, "hyprctl", "-j", "clients"); err != nil {
		return nil, err
	}
	active := hyprActive()
	var out []Window
	for _, c := range cs {
		if !c.Mapped || c.Hidden {
			continue
		}
		out = append(out, Window{ID: c.Address, App: c.Class, Title: c.Title, Workspace: c.Workspace.Name,
			At: c.At, Size: c.Size, Floating: c.Floating, Focused: c.Address == active})
	}
	return out, nil
}

func hyprActive() string {
	var a struct{ Address string }
	runJSON(&a, "hyprctl", "-j", "activewindow")
	return a.Address
}

func (hyprland) Focus(id string) error {
	run("hyprctl", "dispatch", "focuswindow", "address:"+id)
	time.Sleep(200 * time.Millisecond)
	if hyprActive() == id {
		return nil
	}
	// Lua-configured Hyprland dispatches through hl.dispatch
	run("hyprctl", "eval", fmt.Sprintf(`for _, w in ipairs(hl.get_windows() or {}) do if w.address == %q then hl.dispatch(hl.dsp.focus({window = w})) end end`, id))
	time.Sleep(200 * time.Millisecond)
	if a := hyprActive(); a != id {
		return fmt.Errorf("focus did not stick (the active window is %s)", a)
	}
	return nil
}

// Cursor is where Hyprland says the pointer is.
func (hyprland) Cursor() (int, int, error) {
	out, err := run("hyprctl", "cursorpos")
	if err != nil {
		return 0, 0, err
	}
	var x, y float64
	if _, err := fmt.Sscanf(strings.ReplaceAll(out, ",", " "), "%f %f", &x, &y); err != nil {
		return 0, 0, fmt.Errorf("hyprctl cursorpos: %q", out)
	}
	return int(x), int(y), nil
}

// sway talks to Sway through swaymsg, and moves the pointer with Sway's own
// seat commands, so it needs no uinput.
type sway struct{ wlroots }

func (sway) Name() string { return "Sway" }

func (sway) Monitors() ([]Monitor, error) {
	var outs []struct {
		Name   string
		Active bool
		Scale  float64
		Rect   struct{ X, Y, Width, Height int }
	}
	if err := runJSON(&outs, "swaymsg", "-r", "-t", "get_outputs"); err != nil {
		return nil, err
	}
	var out []Monitor
	for _, o := range outs {
		if o.Active {
			out = append(out, Monitor{o.Name, o.Rect.X, o.Rect.Y, o.Rect.Width, o.Rect.Height, o.Scale})
		}
	}
	return out, nil
}

type swayNode struct {
	ID               int
	Type, Name       string
	AppID            string `json:"app_id"`
	Pid              int
	Focused          bool
	Rect             struct{ X, Y, Width, Height int }
	WindowProperties struct{ Class string } `json:"window_properties"`
	Nodes            []swayNode
	FloatingNodes    []swayNode `json:"floating_nodes"`
}

func swayWindows(root swayNode) []Window {
	var out []Window
	var walk func(n swayNode, ws string, floating bool)
	walk = func(n swayNode, ws string, floating bool) {
		if n.Type == "workspace" {
			ws = n.Name
		}
		if (n.Type == "con" || n.Type == "floating_con") && n.Pid > 0 && len(n.Nodes) == 0 {
			app := n.AppID
			if app == "" {
				app = n.WindowProperties.Class
			}
			out = append(out, Window{ID: strconv.Itoa(n.ID), App: app, Title: n.Name, Workspace: ws,
				At: [2]int{n.Rect.X, n.Rect.Y}, Size: [2]int{n.Rect.Width, n.Rect.Height}, Floating: floating, Focused: n.Focused})
		}
		for _, c := range n.Nodes {
			walk(c, ws, floating)
		}
		for _, c := range n.FloatingNodes {
			walk(c, ws, true)
		}
	}
	walk(root, "", false)
	return out
}

func (sway) Windows() ([]Window, error) {
	var root swayNode
	if err := runJSON(&root, "swaymsg", "-r", "-t", "get_tree"); err != nil {
		return nil, err
	}
	return swayWindows(root), nil
}

func (sway) Focus(id string) error {
	if _, err := strconv.Atoi(id); err != nil {
		return fmt.Errorf("sway window ids are numbers, got %q", id)
	}
	_, err := run("swaymsg", fmt.Sprintf("[con_id=%s] focus", id))
	return err
}

func (sway) Pointer() Pointer {
	if os.Getenv("VIBIAN_COMPUTER_POINTER") == "uinput" {
		return nil
	}
	return &swayPtr
}

// swayPtr remembers where it put the pointer: Sway can't say.
var swayPtr swayPointer

type swayPointer struct {
	x, y   int
	placed bool
}

func (p *swayPointer) Move(x, y int) error {
	if _, err := run("swaymsg", "seat", "*", "cursor", "set", strconv.Itoa(x), strconv.Itoa(y)); err != nil {
		return err
	}
	p.x, p.y, p.placed = x, y, true
	return nil
}

func (p *swayPointer) Button(name string, down bool) error {
	b, ok := map[string]string{"left": "button1", "middle": "button2", "right": "button3"}[name]
	if !ok {
		return fmt.Errorf("unknown button %q (left, right or middle)", name)
	}
	act := "release"
	if down {
		act = "press"
	}
	_, err := run("swaymsg", "seat", "*", "cursor", act, b)
	return err
}

func (p *swayPointer) Scroll(dy, dx int) error {
	// buttons 4-7 are the wheel: up, down, left, right
	notch := func(n int, neg, pos string) error {
		b := pos
		if n < 0 {
			b = neg
		}
		for range abs(n) {
			if _, err := run("swaymsg", "seat", "*", "cursor", "press", b); err != nil {
				return err
			}
			run("swaymsg", "seat", "*", "cursor", "release", b)
		}
		return nil
	}
	if err := notch(dy, "button4", "button5"); err != nil {
		return err
	}
	return notch(dx, "button6", "button7")
}

func (p *swayPointer) Position() (int, int, bool) { return p.x, p.y, p.placed }

// niri talks to niri through `niri msg`.
type niri struct{ wlroots }

func (niri) Name() string { return "niri" }

func (niri) Monitors() ([]Monitor, error) {
	var outs map[string]struct {
		Logical *struct {
			X, Y, Width, Height int
			Scale               float64
		}
	}
	if err := runJSON(&outs, "niri", "msg", "--json", "outputs"); err != nil {
		return nil, err
	}
	var out []Monitor
	for name, o := range outs {
		if l := o.Logical; l != nil {
			out = append(out, Monitor{name, l.X, l.Y, l.Width, l.Height, l.Scale})
		}
	}
	return out, nil
}

func (niri) Windows() ([]Window, error) {
	var ws []struct {
		ID          int
		Title       string
		AppID       string `json:"app_id"`
		WorkspaceID *int   `json:"workspace_id"`
		IsFocused   bool   `json:"is_focused"`
		IsFloating  bool   `json:"is_floating"`
		Layout      *struct {
			WindowSize [2]float64 `json:"window_size"`
		}
	}
	if err := runJSON(&ws, "niri", "msg", "--json", "windows"); err != nil {
		return nil, err
	}
	var out []Window
	for _, w := range ws {
		win := Window{ID: strconv.Itoa(w.ID), App: w.AppID, Title: w.Title, Floating: w.IsFloating, Focused: w.IsFocused}
		if w.WorkspaceID != nil {
			win.Workspace = strconv.Itoa(*w.WorkspaceID)
		}
		if w.Layout != nil {
			win.Size = [2]int{int(w.Layout.WindowSize[0]), int(w.Layout.WindowSize[1])}
		}
		out = append(out, win)
	}
	return out, nil
}

func (niri) Focus(id string) error {
	_, err := run("niri", "msg", "action", "focus-window", "--id", id)
	return err
}
