package main

import (
	"testing"
	"time"

	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/snap"
)

func TestTimeline(t *testing.T) {
	s := func(n, pre int, typ, date, desc string) snap.Snapshot {
		return snap.Snapshot{Number: n, Pre: pre, Type: typ, Date: "2026-11-06 " + date, Desc: desc}
	}
	lists := map[string][]snap.Snapshot{
		"root": {
			s(1, 0, "single", "05:00:00", "boot"),
			s(2, 0, "pre", "05:01:00", "install htop"),
			s(3, 2, "post", "05:02:00", "install htop"),
			s(4, 0, "pre", "05:10:00", "agent turn: pi"),
			s(5, 4, "post", "05:11:00", "agent turn: pi"),
			s(6, 0, "pre", "05:20:00", "apt"),
			s(7, 6, "post", "05:20:30", "apt"),
			s(8, 0, "pre", "05:30:00", "agent turn: pi"), // still open
		},
		"home": {
			s(10, 0, "pre", "05:01:00", "install htop"),
			s(11, 10, "post", "05:02:00", "install htop"),
			s(12, 0, "pre", "05:10:00", "agent turn: pi"),
			s(13, 12, "post", "05:11:01", "agent turn: pi"),
		},
	}
	at, _ := time.ParseInLocation(snapperDate, "2026-11-06 05:02:00", time.Local)
	entries := []ledger.Entry{
		{ID: "L-0001", Time: at, Agent: "pi", Summary: "installed htop", Snapshots: "root 2→3, home 10→11"},
		{ID: "L-0002", Time: at.Add(time.Hour), Agent: "pi", Summary: "noted the GPU", Undo: "nothing to undo"},
	}
	got := timeline(lists, entries)
	want := []struct{ what, snaps, undo string }{
		{"installed htop", "root 2→3, home 10→11", "vibian undo L-0001"},
		{"agent turn: pi", "root 4→5, home 12→13", "vibian undo --pair 4"},
		{"apt", "root 6→7", "vibian undo --pair 6"},
		{"noted the GPU", "", "nothing to undo"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows: %+v", len(got), got)
	}
	for i, w := range want {
		if g := got[i]; g.What != w.what || g.snapshots() != w.snaps || g.Undo != w.undo {
			t.Errorf("row %d: %q %q %q, want %+v", i, g.What, g.snapshots(), g.Undo, w)
		}
	}
}
