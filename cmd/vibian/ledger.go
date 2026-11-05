package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/snap"
)

// listFlag collects a repeatable, comma-separable flag.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ", ") }
func (l *listFlag) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*l = append(*l, s)
		}
	}
	return nil
}

// parse lets flags come after positional args ("vibian log add "x" --files a").
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func agentName() string {
	if a := os.Getenv("VIBIAN_AGENT"); a != "" {
		return a
	}
	return "human"
}

func cmdSnap(args []string) error {
	if len(args) != 1 {
		return errors.New(`usage: vibian snap "why"`)
	}
	if !snap.Available() {
		return errors.New("snapper isn't set up on this machine")
	}
	// An unclosed snap from earlier gets closed first so pairs never nest.
	if p, _ := snap.TakePending(); p != nil {
		if post, err := snap.Post(p.Pre, p.Why); err == nil {
			e, _ := ledger.Add(ledger.Entry{Agent: agentName(), Summary: p.Why + " (closed by the next snap)", Snapshots: snap.Pair(p.Pre, post)})
			fmt.Printf("closed the earlier snap %q as %s\n", p.Why, e.ID)
		}
	}
	pre, err := snap.Pre(args[0])
	if err != nil {
		return err
	}
	if err := snap.SavePending(snap.Pending{Why: args[0], Pre: pre}); err != nil {
		return err
	}
	fmt.Printf("snapshot %s. When you're done, record it: vibian log add \"what changed\"\n", pre)
	return nil
}

func cmdLog(args []string) error {
	sub := "tail"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "add":
		return logAdd(args)
	case "tail":
		n := 10
		if len(args) > 0 {
			fmt.Sscan(args[0], &n)
		}
		entries, err := ledger.All()
		if err != nil {
			return err
		}
		if len(entries) > n {
			entries = entries[len(entries)-n:]
		}
		if len(entries) == 0 {
			fmt.Println("The ledger is empty. Changes recorded with `vibian log add` show up here.")
		}
		for _, e := range entries {
			fmt.Println(e.Raw + "\n")
		}
	case "show":
		if len(args) != 1 {
			return errors.New("usage: vibian log show <id>")
		}
		e, err := ledger.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Println(e.Raw)
	case "search":
		found, err := ledger.Search(strings.Join(args, " "))
		if err != nil {
			return err
		}
		for _, e := range found {
			fmt.Println(e.Raw + "\n")
		}
		if len(found) == 0 {
			return errors.New("no matching entries")
		}
	default:
		return fmt.Errorf("unknown: vibian log %s", sub)
	}
	return nil
}

func logAdd(args []string) error {
	fs := flag.NewFlagSet("log add", flag.ContinueOnError)
	var files, pkgs, gotchas listFlag
	fs.Var(&files, "files", "changed paths (repeat or comma-separate)")
	fs.Var(&pkgs, "pkgs", "packages: +added, -removed")
	fs.Var(&gotchas, "gotcha", "something the next agent should know")
	undo := fs.String("undo", "", "how to undo it, if not just `vibian undo <id>`")
	noSnap := fs.Bool("no-snap", false, "don't close the pending snap or take one")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New(`usage: vibian log add "summary" [--files a,b] [--pkgs +x,-y] [--undo "…"] [--gotcha "…"]`)
	}
	e := ledger.Entry{Agent: agentName(), Summary: pos[0], Files: files, Packages: pkgs, Gotchas: gotchas, Undo: *undo}
	if !*noSnap && snap.Available() {
		p, err := snap.TakePending()
		if err != nil {
			return err
		}
		if p != nil {
			post, err := snap.Post(p.Pre, pos[0])
			if err != nil {
				return err
			}
			e.Snapshots = snap.Pair(p.Pre, post)
		}
	}
	e, err = ledger.Add(e)
	if err != nil {
		return err
	}
	fmt.Printf("%s recorded", e.ID)
	if e.Snapshots != "" {
		fmt.Printf(" (snapshots %s; undo with `vibian undo %s`)", e.Snapshots, e.ID)
	}
	fmt.Println()
	return nil
}
