package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/snap"
	"github.com/jacobzipper/vibian/internal/sys"
)

func cmdUndo(args []string) error {
	fs := flag.NewFlagSet("undo", flag.ContinueOnError)
	last := fs.Bool("last", false, "undo the newest ledger entry that has snapshots")
	pair := fs.String("pair", "", "undo the snapshot pair starting at root snapshot N (see `vibian history`)")
	to := fs.Int("to", 0, "make root snapshot N the running system (takes effect at reboot)")
	yes := fs.Bool("yes", false, "don't ask")
	cleanup := fs.Bool("cleanup", false, "delete the root subvolumes earlier rollbacks set aside (run at boot)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	switch {
	case *cleanup:
		return rollbackCleanup()
	case *to > 0:
		return rollback(*to, *yes)
	case *pair != "":
		return undoPair(*pair, *yes)
	case *last:
		entries, err := ledger.All()
		if err != nil {
			return err
		}
		for i := len(entries) - 1; i >= 0; i-- {
			if len(entries[i].Pairs()) > 0 {
				return undoEntry(entries[i], *yes)
			}
		}
		return errors.New("no ledger entry has snapshots to undo")
	case len(pos) == 1:
		e, err := ledger.Get(pos[0])
		if err != nil {
			return err
		}
		return undoEntry(e, *yes)
	}
	return errors.New("usage: vibian undo <ledger-id> | --last | --pair <n> | --to <snapshot>")
}

func confirm(q string) bool {
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		// an agent's shell: a prompt would just read EOF
		fmt.Println(q + " No terminal to ask on: run it again with --yes once the owner has agreed.")
		return false
	}
	fmt.Print(q + " [y/N] ")
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y")
}

// undoEntry reverts the files an entry's snapshot pairs say changed. The undo
// is itself snapshotted and logged, so it can be undone too.
func undoEntry(e ledger.Entry, yes bool) error {
	pairs := e.Pairs()
	if len(pairs) == 0 {
		if e.Undo != "" {
			return fmt.Errorf("%s has no snapshots; its undo note says: %s", e.ID, e.Undo)
		}
		return fmt.Errorf("%s has no snapshots to undo", e.ID)
	}
	fmt.Printf("%s · %s · %q\nsnapshots: %s\n", e.ID, e.Agent, e.Summary, e.Snapshots)
	for _, c := range snap.Configs {
		if p, ok := pairs[c]; ok {
			out, _ := sys.RootOutput("snapper", "-c", c, "status", fmt.Sprintf("%d..%d", p[0], p[1]))
			n := 0
			if out != "" {
				n = len(strings.Split(out, "\n"))
			}
			fmt.Printf("  %s: %d paths changed\n", c, n)
		}
	}
	for _, g := range e.Gotchas {
		fmt.Println("  gotcha:", g)
	}
	if !yes && !confirm("Put those files back the way they were before "+e.ID+"?") {
		return errors.New("nothing changed")
	}
	why := "undo " + e.ID
	pre, err := snap.Pre(why)
	if err != nil {
		return err
	}
	for _, c := range snap.Configs {
		if p, ok := pairs[c]; ok {
			if err := snap.UndoChange(c, p[0], p[1]); err != nil {
				return fmt.Errorf("%s: %w", c, err)
			}
		}
	}
	post, err := snap.Post(pre, why)
	if err != nil {
		return err
	}
	u, err := ledger.Add(ledger.Entry{Agent: agentName(), Summary: fmt.Sprintf("undo %s (%s)", e.ID, e.Summary), Snapshots: snap.Pair(pre, post)})
	if err != nil {
		return err
	}
	fmt.Printf("Undone. Recorded as %s; `vibian undo %s` puts it back.\n", u.ID, u.ID)
	fmt.Println("Running services keep old files open: restart them, or reboot if the change touched the kernel or drivers.")
	if units := orphanUnits(); len(units) > 0 {
		fmt.Printf("Still running though the undo removed them: %s. They're gone after a reboot, or stop them now with `sudo systemctl stop %s`.\n",
			strings.Join(units, ", "), strings.Join(units, " "))
	}
	return nil
}

// orphanUnits reloads systemd and lists units that are still active but whose
// unit files no longer exist, such as a display manager the undo removed.
// They aren't stopped here: that could end the owner's session.
func orphanUnits() []string {
	if _, err := sys.RootOutput("systemctl", "daemon-reload"); err != nil {
		return nil
	}
	out, _ := sys.Output("systemctl", "list-units", "--all", "--state=active", "--plain", "--no-legend", "--no-pager")
	return parseOrphans(out)
}

func parseOrphans(listUnits string) []string {
	var units []string
	for _, l := range strings.Split(listUnits, "\n") {
		f := strings.Fields(l)
		if len(f) >= 3 && f[1] == "not-found" && f[2] == "active" && !strings.HasSuffix(f[0], ".scope") {
			units = append(units, f[0])
		}
	}
	return units
}

// btrfsTop mounts the top-level btrfs volume and returns its path.
func btrfsTop() (string, func(), error) {
	src, err := sys.Output("findmnt", "-no", "SOURCE", "/.snapshots")
	if err != nil {
		return "", nil, errors.New("/.snapshots isn't mounted; this doesn't look like a Vibian btrfs layout")
	}
	dev := regexp.MustCompile(`\[.*\]$`).ReplaceAllString(src, "")
	top := "/run/vibian-top"
	if _, err := sys.RootOutput("mkdir", "-p", top); err != nil {
		return "", nil, err
	}
	if _, err := sys.RootOutput("mount", "-o", "subvolid=5", dev, top); err != nil {
		return "", nil, err
	}
	return top, func() { sys.RootOutput("umount", top) }, nil
}

// rollback swaps @ for a writable copy of a root snapshot. The current @ is
// snapshotted first, then renamed aside and deleted after keepDays.
func rollback(n int, yes bool) error {
	var target *snap.Snapshot
	snaps, err := snap.List("root")
	for i := range snaps {
		if snaps[i].Number == n {
			target = &snaps[i]
		}
	}
	if err != nil {
		// snapper can't open the root config when / is a snapshot overlay;
		// read the snapshot's metadata directly
		xml, xerr := sys.RootOutput("cat", fmt.Sprintf("/.snapshots/%d/info.xml", n))
		if xerr != nil {
			return err
		}
		tag := func(t string) string {
			m := regexp.MustCompile("<" + t + ">(.*?)</" + t + ">").FindStringSubmatch(xml)
			if m == nil {
				return ""
			}
			return m[1]
		}
		target = &snap.Snapshot{Number: n, Date: tag("date"), Desc: tag("description")}
	}
	if target == nil {
		return fmt.Errorf("no root snapshot %d (see `snapper -c root list`)", n)
	}
	fmt.Printf("Root snapshot %d · %s · %s\n", n, target.Date, target.Desc)
	fmt.Println("After a reboot, / will be exactly as it was then. /home, logs, caches and the ledger stay as they are now.")
	if !yes && !confirm("Roll back?") {
		return errors.New("nothing changed")
	}
	// Booted into a snapshot, / is an overlay snapper can't snapshot; the
	// set-aside root is the only copy of the old system then, so cleanup
	// keeps it for a week either way.
	overlay := strings.Contains(sys.ReadFile("/proc/cmdline"), "rd.overlay")
	var before snap.Set
	if !overlay {
		if before, err = snap.Post(nil, fmt.Sprintf("before rollback to %d", n)); err != nil {
			return err
		}
	}
	top, done, err := btrfsTop()
	if err != nil {
		return err
	}
	defer done()
	aside := fmt.Sprintf("@rollback-%s", time.Now().Format("20060102-150405"))
	steps := [][]string{
		{"mv", filepath.Join(top, "@"), filepath.Join(top, aside)},
		{"btrfs", "subvolume", "snapshot", filepath.Join(top, "@snapshots", strconv.Itoa(n), "snapshot"), filepath.Join(top, "@")},
	}
	for _, s := range steps {
		if _, err := sys.RootOutput(s[0], s[1:]...); err != nil {
			if s[0] == "btrfs" {
				// put the old root back so the next boot still works
				sys.RootOutput("mv", filepath.Join(top, aside), filepath.Join(top, "@"))
			}
			return err
		}
	}
	id, _ := sys.RootOutput("btrfs", "subvolume", "show", filepath.Join(top, "@"))
	for _, ln := range strings.Split(id, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(ln), "Subvolume ID:"); ok {
			sys.RootOutput("btrfs", "subvolume", "set-default", strings.TrimSpace(v), top)
		}
	}
	entry := ledger.Entry{
		Agent:   agentName(),
		Summary: fmt.Sprintf("roll back / to snapshot %d (%s)", n, target.Desc),
		Undo:    fmt.Sprintf("the old root is kept as %s for %d days: mv it back to @ from a live USB", aside, keepDays),
	}
	if !overlay {
		entry.Snapshots = "root " + strconv.Itoa(before["root"]) + " (the system before the rollback)"
		entry.Undo = fmt.Sprintf("`vibian undo --to %d`", before["root"])
	}
	e, err := ledger.Add(entry)
	if err != nil {
		return err
	}
	fmt.Printf("Done (%s). Reboot to finish: sudo reboot\n", e.ID)
	return nil
}

const keepDays = 7

// rollbackCleanup deletes @rollback-* subvolumes older than keepDays. Runs at
// boot, when they're no longer mounted.
func rollbackCleanup() error {
	top, done, err := btrfsTop()
	if err != nil {
		return err
	}
	defer done()
	old, _ := filepath.Glob(filepath.Join(top, "@rollback-*"))
	for _, o := range old {
		t, err := time.ParseInLocation("20060102-150405", strings.TrimPrefix(filepath.Base(o), "@rollback-"), time.Local)
		if err != nil || time.Since(t) < keepDays*24*time.Hour {
			continue
		}
		if _, err := sys.RootOutput("btrfs", "subvolume", "delete", "--recursive", o); err != nil {
			fmt.Fprintln(os.Stderr, "vibian:", err)
		}
	}
	return nil
}
