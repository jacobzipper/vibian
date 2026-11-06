package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/snap"
)

// change is one row of `vibian history`: a ledger entry, a snapshot pair, or
// both.
type change struct {
	Time   time.Time         `json:"time"`
	Ledger string            `json:"ledger,omitempty"`
	What   string            `json:"what"`
	Agent  string            `json:"agent,omitempty"`
	Pairs  map[string][2]int `json:"snapshots,omitempty"`
	Undo   string            `json:"undo,omitempty"`
}

func (c change) snapshots() string {
	var parts []string
	for _, cfg := range snap.Configs {
		if p, ok := c.Pairs[cfg]; ok {
			parts = append(parts, fmt.Sprintf("%s %d→%d", cfg, p[0], p[1]))
		}
	}
	return strings.Join(parts, ", ")
}

const snapperDate = "2006-01-02 15:04:05"

// timeline joins ledger entries to snapshot pairs, oldest first. A pair a
// ledger entry recorded becomes that entry's row; the other pairs (agent
// sessions and turns, apt) get rows of their own. A home pair belongs to the
// root pair with the same description taken closest in time.
func timeline(lists map[string][]snap.Snapshot, entries []ledger.Entry) []change {
	type post struct {
		snap.Snapshot
		at   time.Time
		used bool
	}
	posts := map[string][]*post{}
	for cfg, l := range lists {
		for _, s := range l {
			if s.Type == "post" && s.Pre > 0 {
				at, _ := time.ParseInLocation(snapperDate, s.Date, time.Local)
				posts[cfg] = append(posts[cfg], &post{s, at, false})
			}
		}
	}
	take := func(cfg string, num int) time.Time {
		for _, p := range posts[cfg] {
			if p.Number == num {
				p.used = true
				return p.at
			}
		}
		return time.Time{}
	}
	var out []change
	for _, e := range entries {
		c := change{Time: e.Time, Ledger: e.ID, What: e.Summary, Agent: e.Agent, Pairs: e.Pairs()}
		for cfg, p := range c.Pairs {
			// the ledger only has minutes; the snapshot has seconds
			if at := take(cfg, p[1]); cfg == "root" && !at.IsZero() {
				c.Time = at
			}
		}
		if len(c.Pairs) > 0 {
			c.Undo = "vibian undo " + e.ID
		} else if e.Undo != "" {
			c.Undo = e.Undo
		}
		out = append(out, c)
	}
	for _, r := range posts["root"] {
		if r.used {
			continue
		}
		c := change{Time: r.at, What: r.Desc, Pairs: map[string][2]int{"root": {r.Pre, r.Number}}}
		var best *post
		for _, h := range posts["home"] {
			d := h.at.Sub(r.at).Abs()
			if !h.used && h.Desc == r.Desc && d < 2*time.Minute && (best == nil || d < best.at.Sub(r.at).Abs()) {
				best = h
			}
		}
		if best != nil {
			best.used = true
			c.Pairs["home"] = [2]int{best.Pre, best.Number}
		}
		c.Undo = fmt.Sprintf("vibian undo --pair %d", r.Pre)
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

func loadTimeline() ([]change, error) {
	entries, err := ledger.All()
	if err != nil {
		return nil, err
	}
	lists := map[string][]snap.Snapshot{}
	if snap.Available() {
		for _, cfg := range snap.Configs {
			if lists[cfg], err = snap.List(cfg); err != nil {
				return nil, fmt.Errorf("snapper -c %s list: %w", cfg, err)
			}
		}
	}
	return timeline(lists, entries), nil
}

// cmdHistory prints what changed on this machine, newest last.
func cmdHistory(args []string) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	n := fs.Int("n", 20, "show the last N changes (0 for all)")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	all, err := loadTimeline()
	if err != nil {
		return err
	}
	if *n > 0 && len(all) > *n {
		all = all[len(all)-*n:]
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(all)
	}
	if len(all) == 0 {
		fmt.Println("Nothing recorded yet.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tWHAT\tSNAPSHOTS\tUNDO")
	for _, c := range all {
		what := c.What
		if c.Ledger != "" {
			what = c.Ledger + " " + what
			if c.Agent != "" {
				what += " (" + c.Agent + ")"
			}
		}
		if r := []rune(what); len(r) > 60 {
			what = string(r[:59]) + "…"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Time.Format("2006-01-02 15:04"), what, c.snapshots(), c.Undo)
	}
	return w.Flush()
}

// undoPair undoes a snapshot pair no ledger entry recorded, found by its root
// pre number as `vibian history` shows it.
func undoPair(arg string, yes bool) error {
	pre, err := strconv.Atoi(arg)
	if err != nil {
		return fmt.Errorf("--pair takes the root pre snapshot number from `vibian history`, not %q", arg)
	}
	all, err := loadTimeline()
	if err != nil {
		return err
	}
	for _, c := range all {
		if p, ok := c.Pairs["root"]; ok && p[0] == pre {
			if c.Ledger != "" {
				return fmt.Errorf("that pair is %s's: vibian undo %s", c.Ledger, c.Ledger)
			}
			e := ledger.Entry{ID: fmt.Sprintf("snapshot pair %d", pre), Agent: "snapper", Summary: c.What, Snapshots: c.snapshots()}
			return undoEntry(e, yes)
		}
	}
	return errors.New("no snapshot pair starts at root " + arg + ": see `vibian history`")
}
