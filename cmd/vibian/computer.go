package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jacobzipper/vibian/internal/computer"
	"github.com/jacobzipper/vibian/internal/mcp"
)

const computerInstructions = `Computer use for this machine's desktop (Hyprland, Sway, niri, KDE, X11; GNOME best effort). ` +
	`Look before you act: screenshot, then click or type, then screenshot again to check. Coordinates are ` +
	`screen pixels; a screenshot's note says how to map image pixels back. Prefer windows/focus_window and ` +
	`keyboard shortcuts to hunting with the mouse. Ask the owner before anything that sends, buys, deletes ` +
	`or logs in, and never type passwords or secrets you were not given for that purpose.`

var (
	intProp  = func(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }
	numProp  = func(d string) map[string]any { return map[string]any{"type": "number", "description": d} }
	boolProp = func(d string) map[string]any { return map[string]any{"type": "boolean", "description": d} }
	btnProp  = map[string]any{"type": "string", "enum": []string{"left", "right", "middle"}, "description": "mouse button (default left)"}
)

// point is an optional x, y in tool arguments.
type point struct{ X, Y *int }

func (p point) at() computer.At { return computer.At{X: p.X, Y: p.Y} }

func button(b string) string {
	if b == "" {
		return "left"
	}
	return b
}

func computerTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "screenshot",
			Description: "Capture the screen, or a region \"x,y,w,h\" in screen pixels. The image is scaled (by default so the long edge is at most 1280 px); " +
				"the note with it says how to turn image pixels into screen pixels for click and friends, which desktop this is and its monitors.",
			ReadOnly: true,
			Schema: schema(map[string]any{
				"region": strProp(`"x,y,w,h" in screen pixels; omit for every monitor`),
				"scale":  numProp("image pixels per screen pixel (0.1-2); omit to fit 1280 px"),
				"cursor": boolProp("draw the pointer (default true)"),
			}),
			Content: func(a json.RawMessage) ([]mcp.Content, error) {
				p := struct {
					Region string
					Scale  float64
					Cursor *bool
				}{}
				json.Unmarshal(a, &p)
				shot, err := computer.Screenshot(p.Region, p.Scale, p.Cursor == nil || *p.Cursor)
				if err != nil {
					return nil, err
				}
				note, _ := json.Marshal(shot.Note)
				return []mcp.Content{mcp.Image(shot.PNG), mcp.Text(string(note))}, nil
			},
		},
		{
			Name:        "windows",
			Description: "List monitors and open windows: id, app, title, workspace, position, size, floating, focused. Use an id with focus_window.",
			ReadOnly:    true,
			Call:        func(json.RawMessage) (string, error) { return computer.Windows() },
		},
		{
			Name:        "focus_window",
			Description: "Focus a window (switching to its workspace) by its id from `windows`.",
			Schema:      schema(map[string]any{"id": strProp("window id from `windows`")}, "id"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ ID string }
				json.Unmarshal(a, &p)
				if p.ID == "" {
					return "", errors.New("id is required")
				}
				return "focused " + p.ID, computer.Focus(p.ID)
			},
		},
		{
			Name:        "move_mouse",
			Description: "Move the pointer to screen pixel (x, y), e.g. to hover a menu or tooltip.",
			Schema:      schema(map[string]any{"x": intProp("screen x"), "y": intProp("screen y")}, "x", "y"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ X, Y int }
				json.Unmarshal(a, &p)
				return fmt.Sprintf("pointer at %d,%d", p.X, p.Y), computer.Move(p.X, p.Y)
			},
		},
		{
			Name:        "click",
			Description: "Click at screen pixel (x, y), or where the pointer is when they are left out. count 2 double-clicks.",
			Schema: schema(map[string]any{
				"x": intProp("screen x"), "y": intProp("screen y"), "button": btnProp,
				"count": intProp("clicks (default 1)"),
			}),
			Call: func(a json.RawMessage) (string, error) {
				var p struct {
					point
					Button string
					Count  int
				}
				json.Unmarshal(a, &p)
				if err := computer.Click(p.at(), button(p.Button), p.Count); err != nil {
					return "", err
				}
				return fmt.Sprintf("clicked %s x%d%s", button(p.Button), max(1, p.Count), computer.Where()), nil
			},
		},
		{
			Name:        "drag",
			Description: "Press at (x1, y1), move to (x2, y2) and release: drag windows, select text, move sliders.",
			Schema: schema(map[string]any{
				"x1": intProp("start x"), "y1": intProp("start y"), "x2": intProp("end x"), "y2": intProp("end y"), "button": btnProp,
			}, "x1", "y1", "x2", "y2"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct {
					X1, Y1, X2, Y2 int
					Button         string
				}
				json.Unmarshal(a, &p)
				if err := computer.Drag(p.X1, p.Y1, p.X2, p.Y2, button(p.Button)); err != nil {
					return "", err
				}
				return fmt.Sprintf("dragged to %d,%d", p.X2, p.Y2), nil
			},
		},
		{
			Name:        "scroll",
			Description: "Scroll by wheel notches at (x, y) or where the pointer is. dy > 0 scrolls down, dx > 0 scrolls right.",
			Schema: schema(map[string]any{
				"dy": intProp("notches down (negative: up); default 3"), "dx": intProp("notches right (negative: left)"),
				"x": intProp("screen x"), "y": intProp("screen y"),
			}),
			Call: func(a json.RawMessage) (string, error) {
				p := struct {
					point
					DY, DX *int
				}{}
				json.Unmarshal(a, &p)
				dy, dx := 3, 0
				if p.DY != nil {
					dy = *p.DY
				}
				if p.DX != nil {
					dx = *p.DX
					if p.DY == nil {
						dy = 0
					}
				}
				return fmt.Sprintf("scrolled %d down, %d right", dy, dx), computer.Scroll(p.at(), dy, dx)
			},
		},
		{
			Name:        "type_text",
			Description: "Type text into the focused window. Use `key` for Enter and shortcuts.",
			Schema:      schema(map[string]any{"text": strProp("what to type")}, "text"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ Text string }
				json.Unmarshal(a, &p)
				return fmt.Sprintf("typed %d characters", len([]rune(p.Text))), computer.Type(p.Text)
			},
		},
		{
			Name: "key",
			Description: "Press a key or combo: \"Return\", \"ctrl+l\", \"ctrl+shift+t\", \"alt+F4\", \"super+d\", \"Escape\". " +
				"Keys are XKB names (Return, Tab, BackSpace, Left, Page_Down, F5, a, 1 …).",
			Schema: schema(map[string]any{"combo": strProp("key or combo")}, "combo"),
			Call: func(a json.RawMessage) (string, error) {
				var p struct{ Combo string }
				json.Unmarshal(a, &p)
				if strings.TrimSpace(p.Combo) == "" {
					return "", errors.New("combo is required")
				}
				return "pressed " + p.Combo, computer.Key(p.Combo)
			},
		},
	}
}
