package main

import (
	"flag"
	"reflect"
	"testing"
)

// Paths and packages split on commas; gotchas are sentences and must not.
func TestLogAddFlags(t *testing.T) {
	var files listFlag
	var gotchas textFlag
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.Var(&files, "files", "")
	fs.Var(&gotchas, "gotcha", "")
	err := fs.Parse([]string{
		"--files", "~/.config/hypr, ~/.local/bin/tape",
		"--gotcha", "Defaults: foot, US layout, tiling.",
		"--gotcha", "Quotes are ~15 min delayed.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"~/.config/hypr", "~/.local/bin/tape"}; !reflect.DeepEqual([]string(files), want) {
		t.Errorf("files = %q, want %q", files, want)
	}
	if want := []string{"Defaults: foot, US layout, tiling.", "Quotes are ~15 min delayed."}; !reflect.DeepEqual([]string(gotchas), want) {
		t.Errorf("gotchas = %q, want %q", gotchas, want)
	}
}
