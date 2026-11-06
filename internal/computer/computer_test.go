package computer

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"
)

func TestCombo(t *testing.T) {
	for in, want := range map[string][]uint16{
		"Return":       {28},
		"ctrl+l":       {29, 38},
		"Ctrl+Shift+T": {29, 42, 20},
		"super+d":      {125, 32},
		"alt+F4":       {56, 62},
		"ctrl++":       {29, 42, 13},
		"ctrl+plus":    {29, 42, 13},
		"Page_Down":    {109},
		"esc":          {1},
		"F13":          {183},
		"?":            {42, 53},
	} {
		got, err := comboCodes(in)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("comboCodes(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"hyper+x", "ctrl+", "ctrl+nosuchkey", ""} {
		if _, err := comboCodes(bad); err == nil {
			t.Errorf("comboCodes(%q) should fail", bad)
		}
	}
	if mods, key, _ := Combo("meta+enter"); !slices.Equal(mods, []string{"super"}) || key != "Return" {
		t.Errorf("Combo(meta+enter) = %v %q", mods, key)
	}
}

func TestCharKey(t *testing.T) {
	for r, want := range map[rune][2]int{'a': {30, 0}, 'A': {30, 1}, '1': {2, 0}, '!': {2, 1}, ' ': {57, 0},
		'\n': {28, 0}, '"': {40, 1}, '~': {41, 1}, 'm': {50, 0}, 'p': {25, 0}} {
		code, shift, ok := charKey(r)
		if !ok || int(code) != want[0] || shift != (want[1] == 1) {
			t.Errorf("charKey(%q) = %d %v %v, want %v", r, code, shift, ok, want)
		}
	}
	if _, _, ok := charKey('é'); ok {
		t.Error("é is not on a US keyboard")
	}
}

func TestAbsValue(t *testing.T) {
	// libinput gives the compositor value/(max+1) of the box
	for _, c := range []struct{ p, origin, size int }{{0, 0, 1920}, {959, 0, 1920}, {1919, 0, 1920}, {2000, 1920, 2560}, {-100, -1280, 3200}} {
		v := absValue(c.p, c.origin, c.size)
		back := c.origin + int(float64(v)/(absMax+1)*float64(c.size))
		if back != c.p {
			t.Errorf("%+v: value %d maps back to %d", c, v, back)
		}
	}
	if absValue(5000, 0, 1920) != absMax || absValue(-5, 0, 1920) != 0 {
		t.Error("absValue should clamp")
	}
}

func TestRegionBounds(t *testing.T) {
	if r, err := ParseRegion(" 10, 20,300,400 "); err != nil || r != (Rect{10, 20, 300, 400}) {
		t.Errorf("ParseRegion = %v %v", r, err)
	}
	if r, err := ParseRegion("10,20 300x400"); err != nil || r != (Rect{10, 20, 300, 400}) {
		t.Errorf("ParseRegion grim-style = %v %v", r, err)
	}
	for _, bad := range []string{"1,2,3", "a,b,c,d", "0,0,0,10"} {
		if _, err := ParseRegion(bad); err == nil {
			t.Errorf("ParseRegion(%q) should fail", bad)
		}
	}
	b := Bounds([]Monitor{{X: 0, Y: 0, W: 1920, H: 1080}, {X: 1920, Y: -200, W: 1280, H: 1024}})
	if b != (Rect{0, -200, 3200, 1280}) {
		t.Errorf("Bounds = %v", b)
	}
}

func TestCropScale(t *testing.T) {
	// a 200x100 HiDPI capture of a 100x50 screen: left half red, right half blue
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 100 {
				c = color.RGBA{0, 0, 255, 255}
			}
			src.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, src)
	out, err := cropScale(buf.Bytes(), Rect{0, 0, 100, 50}, Rect{50, 0, 50, 50}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(out))
	if b := img.Bounds(); b.Dx() != 25 || b.Dy() != 25 {
		t.Fatalf("size %v", b)
	}
	if r, _, bl, _ := img.At(10, 10).RGBA(); r != 0 || bl == 0 {
		t.Errorf("crop should be all blue, got %v", img.At(10, 10))
	}
	whole, _ := cropScale(buf.Bytes(), Rect{}, Rect{}, 1)
	if img, _ := png.Decode(bytes.NewReader(whole)); img.Bounds().Dx() != 200 {
		t.Errorf("no screen box: whole image at scale 1, got %v", img.Bounds())
	}
	if _, err := cropScale(buf.Bytes(), Rect{0, 0, 100, 50}, Rect{500, 500, 10, 10}, 1); err == nil {
		t.Error("off-screen region should fail")
	}
}

func TestSwayWindows(t *testing.T) {
	tree := `{"id":1,"type":"root","nodes":[{"id":2,"type":"output","name":"HEADLESS-1","nodes":[
	  {"id":3,"type":"workspace","name":"1","nodes":[
	    {"id":4,"type":"con","name":"foot","app_id":"foot","pid":10,"focused":true,"rect":{"x":0,"y":0,"width":640,"height":720},"nodes":[]},
	    {"id":5,"type":"con","name":null,"nodes":[{"id":6,"type":"con","name":"xterm","pid":11,"window_properties":{"class":"XTerm"},"rect":{"x":640,"y":0,"width":640,"height":720},"nodes":[]}]}],
	   "floating_nodes":[{"id":7,"type":"floating_con","name":"pop","app_id":"pavucontrol","pid":12,"rect":{"x":100,"y":100,"width":300,"height":200},"nodes":[]}]}]}]}`
	var root swayNode
	if err := json.Unmarshal([]byte(tree), &root); err != nil {
		t.Fatal(err)
	}
	ws := swayWindows(root)
	if len(ws) != 3 || ws[0].ID != "4" || !ws[0].Focused || ws[1].App != "XTerm" || ws[1].At != [2]int{640, 0} ||
		!ws[2].Floating || ws[2].Workspace != "1" {
		t.Errorf("swayWindows = %+v", ws)
	}
}

func TestX11Parsers(t *testing.T) {
	ws := parseWmctrl("0x01e00003  0 0    0    1280 800  kitty.kitty  box  ~/src: vim  x\n"+
		"0x00c00002 -1 0    770  1280 30   polybar.Polybar  box  bar\n"+
		"0x02200001  1 10   20   600  400  Navigator.firefox  box  Mozilla Firefox\n", 0x02200001)
	if len(ws) != 2 || ws[0].App != "kitty" || ws[0].Title != "~/src: vim x" || ws[1].Focused != true || ws[0].Focused ||
		ws[1].At != [2]int{10, 20} || ws[1].Size != [2]int{600, 400} || ws[1].Workspace != "1" {
		t.Errorf("parseWmctrl = %+v", ws)
	}
	ms := parseXrandr("Monitors: 2\n 0: +*eDP-1 1920/344x1080/194+0+0  eDP-1\n 1: +HDMI-1 2560/597x1440/336+1920+0  HDMI-1\n")
	if len(ms) != 2 || ms[1] != (Monitor{"HDMI-1", 1920, 0, 2560, 1440, 1}) {
		t.Errorf("parseXrandr = %+v", ms)
	}
}
