package computer

import (
	"fmt"
	"strings"
)

// keyCodes maps modifiers and lower-cased XKB keysyms to evdev key codes
// on a US layout.
var keyCodes = map[string]uint16{
	"ctrl": 29, "shift": 42, "alt": 56, "super": 125, "altgr": 100,
	"escape": 1, "minus": 12, "equal": 13, "backspace": 14, "tab": 15,
	"bracketleft": 26, "bracketright": 27, "return": 28, "semicolon": 39,
	"apostrophe": 40, "grave": 41, "backslash": 43, "comma": 51, "period": 52,
	"slash": 53, "space": 57, "caps_lock": 58, "print": 99, "home": 102,
	"up": 103, "page_up": 104, "left": 105, "right": 106, "end": 107,
	"down": 108, "page_down": 109, "insert": 110, "delete": 111, "menu": 127,
	"xf86audiomute": 113, "xf86audiolowervolume": 114, "xf86audioraisevolume": 115,
}

func init() {
	for i, c := range "1234567890" {
		keyCodes[string(c)] = uint16(2 + i)
	}
	for i, row := range []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"} {
		start := []uint16{16, 30, 44}[i]
		for j, c := range row {
			keyCodes[string(c)] = start + uint16(j)
		}
	}
	for i := range 10 {
		keyCodes[fmt.Sprintf("f%d", i+1)] = uint16(59 + i)
	}
	keyCodes["f11"], keyCodes["f12"] = 87, 88
	for i := 13; i <= 24; i++ {
		keyCodes[fmt.Sprintf("f%d", i)] = uint16(183 + i - 13)
	}
}

// shifted is what each key types with shift held, on a US layout.
var shifted = map[rune]rune{
	'!': '1', '@': '2', '#': '3', '$': '4', '%': '5', '^': '6', '&': '7', '*': '8', '(': '9', ')': '0',
	'_': '-', '+': '=', '{': '[', '}': ']', '|': '\\', ':': ';', '"': '\'', '~': '`', '<': ',', '>': '.', '?': '/',
}

var plainChars = map[rune]string{
	'-': "minus", '=': "equal", '[': "bracketleft", ']': "bracketright", ';': "semicolon",
	'\'': "apostrophe", '`': "grave", '\\': "backslash", ',': "comma", '.': "period", '/': "slash",
	' ': "space", '\n': "return", '\t': "tab",
}

// charKey is the key (and whether shift is needed) that types r.
func charKey(r rune) (code uint16, shift bool, ok bool) {
	if base, ok := shifted[r]; ok {
		code, _, ok := charKey(base)
		return code, true, ok
	}
	if r >= 'A' && r <= 'Z' {
		code, ok := keyCodes[strings.ToLower(string(r))]
		return code, true, ok
	}
	name, ok := plainChars[r]
	if !ok {
		name = string(r)
	}
	code, ok = keyCodes[name]
	return code, false, ok
}

// comboCodes turns "ctrl+shift+t" into key codes, modifiers first.
func comboCodes(combo string) ([]uint16, error) {
	mods, key, err := Combo(combo)
	if err != nil {
		return nil, err
	}
	var codes []uint16
	for _, m := range mods {
		codes = append(codes, keyCodes[m])
	}
	if strings.ToLower(key) == "plus" {
		key = "+"
	}
	code, ok := keyCodes[strings.ToLower(key)]
	if !ok {
		if c, shift, isChar := charKey([]rune(key)[0]); isChar && len([]rune(key)) == 1 {
			if shift {
				codes = append(codes, keyCodes["shift"])
			}
			code, ok = c, true
		}
	}
	if !ok {
		return nil, fmt.Errorf("unknown key %q in %q (use XKB names: Return, Tab, Escape, BackSpace, Left, Page_Down, F5, a, 1 …)", key, combo)
	}
	return append(codes, code), nil
}
