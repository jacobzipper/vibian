package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jacobzipper/vibian/internal/harness"
	"github.com/jacobzipper/vibian/internal/snap"
)

// turnDebounce is the least time between two kept turn snapshots. Turns in
// between fold into the next pair (and the session's own pair).
const turnDebounce = 5 * time.Minute

// cmdHook is what harness hooks run around each user turn. It never fails the
// turn: errors go to stderr, stdout stays empty (some harnesses feed hook
// stdout to the model), and the exit status is always 0.
func cmdHook(args []string) error {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	agent := fs.String("agent", os.Getenv("VIBIAN_AGENT"), "harness id")
	fs.Parse(args[1:])
	// an ask (vibian ask) has its own snapshot pair
	if !snap.Available() || os.Getenv("VIBIAN_ASK") != "" {
		return nil
	}
	h, _ := harness.Get(*agent)
	var err error
	switch ev {
	case "turn-start":
		err = turnStart(h)
	case "turn-end":
		_, err = turnEnd(h, snap.LoadTurn())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vibian hook:", err)
	}
	return nil
}

func turnDesc(h harness.Harness) string {
	if h.Name == "" {
		return "agent turn"
	}
	return "agent turn: " + h.Name
}

// turnStart opens a pre snapshot unless one is still open from a turn that
// changed nothing, or the last kept pair is too recent.
func turnStart(h harness.Harness) error {
	t := snap.LoadTurn()
	if t.Pre != nil && !t.Clean {
		// the harness has no end-of-turn hook (or it didn't fire)
		var err error
		if t, err = turnEnd(h, t); err != nil {
			return err
		}
	}
	if t.Pre != nil {
		t.Clean = false
		return snap.SaveTurn(t)
	}
	if time.Since(t.Kept) < turnDebounce {
		return nil
	}
	t.Desc = turnDesc(h)
	pre, err := snap.Pre(t.Desc)
	if err != nil {
		return err
	}
	t.Pre = pre
	return snap.SaveTurn(t)
}

// turnEnd closes the open pre snapshot. A pair that changed something is
// kept; otherwise the post snapshot is deleted and the pre stays open for the
// next turn, so quiet turns cost nothing.
func turnEnd(h harness.Harness, t snap.Turn) (snap.Turn, error) {
	if t.Pre == nil || t.Clean {
		return t, nil
	}
	desc := t.Desc
	if desc == "" {
		desc = turnDesc(h)
	}
	post, err := snap.Post(t.Pre, desc)
	if err != nil {
		return t, err
	}
	home, _ := os.UserHomeDir()
	if changed(t.Pre, post, sessionNoise(h, home)) {
		t = snap.Turn{Kept: time.Now()}
	} else {
		if err := snap.Delete(post); err != nil {
			return t, err
		}
		t.Clean = true
	}
	return t, snap.SaveTurn(t)
}

// endTurns runs when a session ends: it closes an open turn and drops a
// leftover pre snapshot, which the session's own pair covers.
func endTurns(h harness.Harness) {
	t, err := turnEnd(h, snap.LoadTurn())
	if err == nil && t.Pre != nil {
		err = snap.Delete(t.Pre)
		t.Pre, t.Desc, t.Clean = nil, "", false
		if err == nil {
			err = snap.SaveTurn(t)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vibian: turn snapshots:", err)
	}
}
