package ledger

import (
	"strings"
	"testing"
)

func TestAddParseRoundTrip(t *testing.T) {
	t.Setenv("VIBIAN_LEDGER", t.TempDir())
	a, err := Add(Entry{Agent: "claude", Summary: `Hyprland "title" bars`, Snapshots: "root 311→312, home 88→89",
		Packages: []string{"+hyprland-plugins-dev"}, Files: []string{"~/.config/hypr/a.lua", "~/.config/hypr/b.lua"},
		Gotchas: []string{"rebuild per commit"}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Add(Entry{Summary: "no snapshots", Undo: "rm ~/x"})
	if a.ID != "L-0001" || b.ID != "L-0002" {
		t.Fatalf("ids %s %s", a.ID, b.ID)
	}
	got, err := Get("1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != `Hyprland "title" bars` || got.Agent != "claude" || len(got.Files) != 2 || got.Gotchas[0] != "rebuild per commit" {
		t.Fatalf("parsed %+v", got)
	}
	if p := got.Pairs(); p["root"] != [2]int{311, 312} || p["home"] != [2]int{88, 89} {
		t.Fatalf("pairs %v", p)
	}
	if !strings.Contains(got.Raw, "vibian undo L-0001") {
		t.Fatalf("default undo missing: %s", got.Raw)
	}
	if h, _ := Get("L-0002"); h.Agent != "human" || h.Undo != "rm ~/x" {
		t.Fatalf("second %+v", h)
	}
	found, _ := Search("TITLE hyprland")
	if len(found) != 1 {
		t.Fatalf("search found %d", len(found))
	}
}
