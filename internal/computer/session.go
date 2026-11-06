package computer

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// The uinput devices live as long as the MCP server: compositors need a
// moment to pick a new device up, so making one per call would be slow.
var devices struct {
	sync.Mutex
	tablet *Tablet
	kbd    *Keyboard
}

func withKeyboard(f func(*Keyboard) error) error {
	devices.Lock()
	defer devices.Unlock()
	if devices.kbd == nil {
		k, err := NewKeyboard()
		if err != nil {
			return err
		}
		devices.kbd = k
	}
	return f(devices.kbd)
}

// pointer is the desktop's own pointer, or the shared uinput tablet mapped
// onto the desktop's monitors.
func pointer(d Desktop) (Pointer, error) {
	if p := d.Pointer(); p != nil {
		return p, nil
	}
	devices.Lock()
	defer devices.Unlock()
	bounds := func() (Rect, error) {
		ms, err := d.Monitors()
		return Bounds(ms), err
	}
	if devices.tablet == nil {
		t, err := NewTablet(bounds)
		if err != nil {
			return nil, err
		}
		devices.tablet = t
	}
	devices.tablet.bounds = bounds
	return devices.tablet, nil
}

// Shot is a screenshot and how to map its pixels back to the screen.
type Shot struct {
	PNG  []byte `json:"-"`
	Note struct {
		Desktop  string    `json:"desktop"`
		Scale    float64   `json:"scale"`
		Origin   [2]int    `json:"origin"`
		ToScreen string    `json:"to_screen"`
		Cursor   *[2]int   `json:"cursor,omitempty"`
		Monitors []Monitor `json:"monitors,omitempty"`
	}
}

// maxEdge is the default size of a screenshot's long edge: models read
// about this much detail, and bigger images cost tokens.
const maxEdge = 1280

// Screenshot captures region ("x,y,w,h", empty for everything). With scale 0
// it picks one that keeps the long edge within maxEdge pixels.
func Screenshot(region string, scale float64, cursor bool) (Shot, error) {
	var s Shot
	d, err := Detect()
	if err != nil {
		return s, err
	}
	r, err := ParseRegion(region)
	if err != nil {
		return s, err
	}
	ms, _ := d.Monitors()
	area := r
	if area.Empty() {
		area = Bounds(ms)
	}
	if scale <= 0 {
		scale = 1
		if edge := max(area.W, area.H); edge > maxEdge {
			scale = float64(maxEdge) / float64(edge)
		}
	}
	scale = min(scale, 2)
	if s.PNG, err = d.Capture(r, scale, cursor); err != nil {
		return s, err
	}
	s.Note.Desktop, s.Note.Scale, s.Note.Monitors = d.Name(), round(scale), ms
	s.Note.Origin = [2]int{area.X, area.Y}
	s.Note.ToScreen = fmt.Sprintf("screen_x = image_x / %g + %d; screen_y = image_y / %g + %d", round(scale), area.X, round(scale), area.Y)
	if x, y, ok := Position(d); ok {
		s.Note.Cursor = &[2]int{x, y}
	}
	return s, nil
}

func round(f float64) float64 { return float64(int(f*10000+0.5)) / 10000 }

// Position is where the pointer is, if anyone knows.
func Position(d Desktop) (int, int, bool) {
	if c, ok := d.(interface{ Cursor() (int, int, error) }); ok {
		if x, y, err := c.Cursor(); err == nil {
			return x, y, true
		}
	}
	if p := d.Pointer(); p != nil {
		return p.Position()
	}
	devices.Lock()
	defer devices.Unlock()
	if devices.tablet != nil {
		return devices.tablet.Position()
	}
	return 0, 0, false
}

// Windows lists the desktop's monitors and windows as JSON.
func Windows() (string, error) {
	d, err := Detect()
	if err != nil {
		return "", err
	}
	ms, _ := d.Monitors()
	ws, err := d.Windows()
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(map[string]any{"desktop": d.Name(), "monitors": ms, "windows": ws}, "", " ")
	return string(b), err
}

func Focus(id string) error {
	d, err := Detect()
	if err != nil {
		return err
	}
	return d.Focus(id)
}

func Type(text string) error {
	d, err := Detect()
	if err != nil {
		return err
	}
	return d.Type(text)
}

func Key(combo string) error {
	d, err := Detect()
	if err != nil {
		return err
	}
	return d.Key(combo)
}

// At is an optional screen position.
type At struct {
	X, Y *int
}

func (a At) set() bool { return a.X != nil && a.Y != nil }

func withPointer(at At, f func(Desktop, Pointer) error) error {
	d, err := Detect()
	if err != nil {
		return err
	}
	p, err := pointer(d)
	if err != nil {
		return err
	}
	if at.set() {
		if err := p.Move(*at.X, *at.Y); err != nil {
			return err
		}
	}
	return f(d, p)
}

func Move(x, y int) error {
	return withPointer(At{&x, &y}, func(Desktop, Pointer) error { return nil })
}

// Click clicks button count times (2 double-clicks) at a, or where the
// pointer is.
func Click(a At, button string, count int) error {
	return withPointer(a, func(_ Desktop, p Pointer) error {
		for range max(1, count) {
			if err := p.Button(button, true); err != nil {
				return err
			}
			time.Sleep(40 * time.Millisecond)
			if err := p.Button(button, false); err != nil {
				return err
			}
			time.Sleep(80 * time.Millisecond)
		}
		return nil
	})
}

// Drag presses at from, moves to to in steps and releases.
func Drag(fx, fy, tx, ty int, button string) error {
	return withPointer(At{&fx, &fy}, func(_ Desktop, p Pointer) error {
		if err := p.Button(button, true); err != nil {
			return err
		}
		const steps = 12
		var err error
		time.Sleep(80 * time.Millisecond)
		for i := 1; i <= steps && err == nil; i++ {
			err = p.Move(fx+(tx-fx)*i/steps, fy+(ty-fy)*i/steps)
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(80 * time.Millisecond)
		if rerr := p.Button(button, false); err == nil {
			err = rerr
		}
		return err
	})
}

func Scroll(a At, dy, dx int) error {
	return withPointer(a, func(_ Desktop, p Pointer) error { return p.Scroll(dy, dx) })
}

// Where says where the pointer is, for tool results.
func Where() string {
	d, err := Detect()
	if err != nil {
		return ""
	}
	if x, y, ok := Position(d); ok {
		return fmt.Sprintf(" (pointer at %d,%d)", x, y)
	}
	return ""
}
