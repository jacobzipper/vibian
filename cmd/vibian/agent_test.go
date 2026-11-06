package main

import "testing"

func TestUnder(t *testing.T) {
	noise := []string{"/tmp", "/home/u/.local/state/vibian"}
	for path, want := range map[string]bool{
		"/tmp":                                  true,
		"/tmp/x":                                true,
		"/tmpfile":                              false,
		"/home/u/.local":                        true, // created just to hold the state dir
		"/home/u/.local/state/vibian/turn.json": true,
		"/home/u/.local/bin/tool":               false,
		"/etc/hosts":                            false,
	} {
		if got := under(path, noise); got != want {
			t.Errorf("under(%q) = %v", path, got)
		}
	}
}
