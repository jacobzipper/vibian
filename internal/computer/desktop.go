package computer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// capture runs a tool that writes a full-screen PNG to a file, then crops
// and scales it.
func capture(monitors func() ([]Monitor, error), r Rect, scale float64, tool string, args ...string) ([]byte, error) {
	f, err := os.CreateTemp("", "vibian-shot-*.png")
	if err != nil {
		return nil, err
	}
	f.Close()
	defer os.Remove(f.Name())
	if _, err := run(tool, append(args, f.Name())...); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(f.Name())
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("%s wrote no screenshot", tool)
	}
	var screen Rect
	if ms, err := monitors(); err == nil {
		screen = Bounds(ms)
	}
	return cropScale(data, screen, r, scale)
}

// screenFromShot is a single monitor the size of a screenshot, for desktops
// that can't list their monitors.
func screenFromShot(shot func(Rect, float64, bool) ([]byte, error)) ([]Monitor, error) {
	data, err := shot(Rect{}, 1, false)
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return []Monitor{{Name: "screen", W: cfg.Width, H: cfg.Height, Scale: 1}}, nil
}

// kde drives Plasma on Wayland: spectacle for screenshots, KWin scripts for
// windows, uinput for the pointer and keyboard (KWin has no virtual
// keyboard protocol for ordinary clients).
type kde struct{}

func (kde) Name() string { return "KDE Plasma" }

func (kde) Monitors() ([]Monitor, error) {
	var doc struct {
		Outputs []struct {
			Name          string
			Enabled       bool
			Pos           struct{ X, Y int }
			Size          struct{ Width, Height int }
			Scale         float64
			Rotation      int
			CurrentModeID string `json:"currentModeId"`
			Modes         []struct {
				ID   string
				Size struct{ Width, Height int }
			}
		}
	}
	if err := runJSON(&doc, "kscreen-doctor", "-j"); err != nil {
		return nil, err
	}
	var out []Monitor
	for _, o := range doc.Outputs {
		if !o.Enabled {
			continue
		}
		w, h := o.Size.Width, o.Size.Height
		for _, m := range o.Modes {
			if m.ID == o.CurrentModeID {
				w, h = m.Size.Width, m.Size.Height
			}
		}
		s := o.Scale
		if s <= 0 {
			s = 1
		}
		w, h = int(math.Round(float64(w)/s)), int(math.Round(float64(h)/s))
		if o.Rotation == 2 || o.Rotation == 8 { // left, right
			w, h = h, w
		}
		out = append(out, Monitor{o.Name, o.Pos.X, o.Pos.Y, w, h, s})
	}
	if len(out) == 0 {
		return nil, errors.New("kscreen-doctor shows no enabled outputs")
	}
	return out, nil
}

func (k kde) Capture(r Rect, scale float64, cursor bool) ([]byte, error) {
	args := []string{"-b", "-n", "-f"}
	if cursor {
		args = append(args, "-p")
	}
	return capture(k.Monitors, r, scale, "spectacle", append(args, "-o")...)
}

// kwinScript loads and runs a KWin script; what it print()s with the marker
// comes back from the journal.
func kwinScript(body string) (string, error) {
	b := make([]byte, 6)
	rand.Read(b)
	mark := "vibian-" + hex.EncodeToString(b)
	f, err := os.CreateTemp("", "vibian-kwin-*.js")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	fmt.Fprintf(f, "const MARK = %q;\n%s\n", mark, body)
	f.Close()
	since := time.Now().Add(-time.Second).Unix()
	gd := func(path, method string, args ...string) (string, error) {
		return run("gdbus", append([]string{"call", "--session", "--dest", "org.kde.KWin", "--object-path", path, "--method", method}, args...)...)
	}
	out, err := gd("/Scripting", "org.kde.kwin.Scripting.loadScript", f.Name(), mark)
	if err != nil {
		return "", err
	}
	id := strings.Trim(strings.TrimSpace(out), "(),int32 ")
	defer gd("/Scripting", "org.kde.kwin.Scripting.unloadScript", mark)
	if _, err := gd("/Scripting/Script"+id, "org.kde.kwin.Script.run"); err != nil {
		return "", err
	}
	for range 10 {
		time.Sleep(150 * time.Millisecond)
		log, _ := run("journalctl", "--user", "-o", "cat", "--no-pager", fmt.Sprintf("--since=@%d", since))
		for _, l := range strings.Split(log, "\n") {
			if _, after, ok := strings.Cut(l, mark+" "); ok {
				return after, nil
			}
		}
	}
	return "", errors.New("the KWin script ran but its output isn't in the user journal")
}

func (kde) Windows() ([]Window, error) {
	out, err := kwinScript(`const out = [];
for (const w of workspace.windowList()) {
  if (!w.normalWindow || w.skipTaskbar) continue;
  const g = w.frameGeometry;
  out.push({id: String(w.internalId), app: w.resourceClass, title: w.caption,
    workspace: (w.desktops || []).map(d => d.name).join(","),
    at: [Math.round(g.x), Math.round(g.y)], size: [Math.round(g.width), Math.round(g.height)],
    focused: w === workspace.activeWindow});
}
print(MARK + " " + JSON.stringify(out));`)
	if err != nil {
		return nil, err
	}
	var ws []Window
	if err := json.Unmarshal([]byte(out), &ws); err != nil {
		return nil, fmt.Errorf("KWin window list: %v", err)
	}
	return ws, nil
}

func (kde) Focus(id string) error {
	out, err := kwinScript(fmt.Sprintf(`let ok = false;
for (const w of workspace.windowList()) {
  if (String(w.internalId) === %q) { workspace.activeWindow = w; ok = workspace.activeWindow === w; }
}
print(MARK + " " + ok);`, id))
	if err != nil {
		return err
	}
	if out != "true" {
		return fmt.Errorf("no window %s, or it would not take focus", id)
	}
	return nil
}

func (kde) Type(text string) error {
	return withKeyboard(func(k *Keyboard) error { return k.Type(text) })
}
func (kde) Key(combo string) error {
	return withKeyboard(func(k *Keyboard) error { return k.Key(combo) })
}
func (kde) Pointer() Pointer { return nil }

// gnome is best effort: GNOME on Wayland keeps window lists and silent
// screenshots to itself and its extensions.
type gnome struct{}

func (gnome) Name() string { return "GNOME (best effort)" }

func (g gnome) Monitors() ([]Monitor, error) { return screenFromShot(g.Capture) }

func (g gnome) Capture(r Rect, scale float64, cursor bool) ([]byte, error) {
	args := []string{}
	if cursor {
		args = append(args, "-p")
	}
	shot, err := capture(func() ([]Monitor, error) { return nil, errors.New("unknown") }, r, scale, "gnome-screenshot", append(args, "-f")...)
	if err != nil {
		return nil, fmt.Errorf("%v (GNOME on Wayland only lets trusted apps take screenshots without asking; a Wayland session of Hyprland, Sway, niri or KDE works fully)", err)
	}
	return shot, nil
}

func (gnome) Windows() ([]Window, error) {
	return nil, errors.New("GNOME doesn't tell other programs about its windows; use screenshots and super/alt+tab")
}

func (gnome) Focus(string) error {
	return errors.New("GNOME doesn't let other programs focus windows; click the window or use alt+tab")
}

func (gnome) Type(text string) error {
	return withKeyboard(func(k *Keyboard) error { return k.Type(text) })
}
func (gnome) Key(combo string) error {
	return withKeyboard(func(k *Keyboard) error { return k.Key(combo) })
}
func (gnome) Pointer() Pointer { return nil }

// x11 drives an X server with maim, wmctrl and xdotool.
type x11 struct{}

func (x11) Name() string { return "X11" }

var xrandrMonitor = regexp.MustCompile(`(\d+)/\d+x(\d+)/\d+\+(-?\d+)\+(-?\d+)\s+(\S+)\s*$`)

func (x11) Monitors() ([]Monitor, error) {
	out, err := run("xrandr", "--listmonitors")
	if err != nil {
		return nil, err
	}
	return parseXrandr(out), nil
}

func parseXrandr(out string) []Monitor {
	var ms []Monitor
	for _, l := range strings.Split(out, "\n") {
		if m := xrandrMonitor.FindStringSubmatch(l); m != nil {
			n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
			ms = append(ms, Monitor{m[5], n(3), n(4), n(1), n(2), 1})
		}
	}
	return ms
}

func (x x11) Capture(r Rect, scale float64, cursor bool) ([]byte, error) {
	if _, err := exec.LookPath("maim"); err != nil {
		return capture(x.Monitors, r, scale, "import", "-window", "root")
	}
	args := []string{"-f", "png"}
	if !cursor {
		args = append(args, "-u")
	}
	return capture(x.Monitors, r, scale, "maim", args...)
}

func (x11) Windows() ([]Window, error) {
	out, err := run("wmctrl", "-lGx")
	if err != nil {
		return nil, err
	}
	active := -1
	if a, err := run("xdotool", "getactivewindow"); err == nil {
		active, _ = strconv.Atoi(strings.TrimSpace(a))
	}
	return parseWmctrl(out, active), nil
}

// parseWmctrl reads `wmctrl -lGx`: id, desktop, x, y, w, h, class, host, title.
func parseWmctrl(out string, active int) []Window {
	var ws []Window
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) < 8 || f[1] == "-1" { // -1: sticky panels and docks
			continue
		}
		n := func(i int) int { v, _ := strconv.Atoi(f[i]); return v }
		id, _ := strconv.ParseInt(f[0], 0, 64)
		title := ""
		if len(f) > 8 {
			title = strings.Join(f[8:], " ")
		}
		app := f[6]
		if _, class, ok := strings.Cut(app, "."); ok {
			app = class
		}
		ws = append(ws, Window{ID: f[0], App: app, Title: title, Workspace: f[1],
			At: [2]int{n(2), n(3)}, Size: [2]int{n(4), n(5)}, Focused: int(id) == active})
	}
	return ws
}

func (x11) Focus(id string) error {
	_, err := run("wmctrl", "-ia", id)
	return err
}

func (x11) Type(text string) error {
	_, err := run("xdotool", "type", "--delay", "8", "--", text)
	return err
}

func (x11) Key(combo string) error {
	mods, key, err := Combo(combo)
	if err != nil {
		return err
	}
	for i, m := range mods {
		if m == "altgr" {
			mods[i] = "ISO_Level3_Shift"
		}
	}
	_, err = run("xdotool", "key", "--", strings.Join(append(mods, key), "+"))
	return err
}

func (x11) Pointer() Pointer { return xdoPointer{} }

type xdoPointer struct{}

func (xdoPointer) Move(x, y int) error {
	_, err := run("xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y))
	return err
}

func (xdoPointer) Button(name string, down bool) error {
	b, ok := map[string]string{"left": "1", "middle": "2", "right": "3"}[name]
	if !ok {
		return fmt.Errorf("unknown button %q (left, right or middle)", name)
	}
	act := "mouseup"
	if down {
		act = "mousedown"
	}
	_, err := run("xdotool", act, b)
	return err
}

func (xdoPointer) Scroll(dy, dx int) error {
	click := func(n int, neg, pos string) error {
		if n == 0 {
			return nil
		}
		b := pos
		if n < 0 {
			b = neg
		}
		_, err := run("xdotool", "click", "--repeat", strconv.Itoa(abs(n)), "--delay", "30", b)
		return err
	}
	if err := click(dy, "4", "5"); err != nil {
		return err
	}
	return click(dx, "6", "7")
}

func (xdoPointer) Position() (int, int, bool) {
	out, err := run("xdotool", "getmouselocation", "--shell")
	if err != nil {
		return 0, 0, false
	}
	var x, y int
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(l, "X="); ok {
			x, _ = strconv.Atoi(v)
		}
		if v, ok := strings.CutPrefix(l, "Y="); ok {
			y, _ = strconv.Atoi(v)
		}
	}
	return x, y, true
}
