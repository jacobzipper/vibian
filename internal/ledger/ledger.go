// Package ledger is Vibian's change log: one markdown file that every agent
// and human appends to, kept in git so it has its own history.
package ledger

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Dir is the ledger directory. VIBIAN_LEDGER overrides it (tests, dev boxes).
func Dir() string {
	if d := os.Getenv("VIBIAN_LEDGER"); d != "" {
		return d
	}
	return "/var/lib/vibian/ledger"
}

func file() string { return filepath.Join(Dir(), "ledger.md") }

// Entry is one ledger entry.
type Entry struct {
	ID        string
	Time      time.Time
	Agent     string
	Summary   string
	Snapshots string // "root 12→13, home 4→5"
	Packages  []string
	Files     []string
	Undo      string
	Command   string // a root command run through `vibian sudo`
	Result    string // how that command ended
	Gotchas   []string
	Raw       string
}

// Markdown renders an entry in the ledger format.
func (e Entry) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s · %s · %s · %q\n", e.ID, e.Time.Format("2006-01-02 15:04"), e.Agent, e.Summary)
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "- %s: %s\n", k, v)
		}
	}
	line("snapshots", e.Snapshots)
	line("packages", strings.Join(e.Packages, ", "))
	line("files", strings.Join(e.Files, ", "))
	if e.Command != "" {
		line("command", "`"+e.Command+"`")
	}
	line("result", e.Result)
	if e.Undo != "" {
		line("undo", e.Undo)
	} else if e.Snapshots != "" {
		line("undo", "`vibian undo "+e.ID+"`")
	}
	for _, g := range e.Gotchas {
		line("gotcha", g)
	}
	return b.String()
}

const header = "# Vibian ledger\n\nEvery change to this machine, newest last. Agents append with `vibian log add`; undo with `vibian undo <id>`.\n"

// Add appends an entry, giving it the next id, and commits it.
func Add(e Entry) (Entry, error) {
	if err := os.MkdirAll(Dir(), 0o2775); err != nil {
		return e, err
	}
	lock, err := os.OpenFile(filepath.Join(Dir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o664)
	if err != nil {
		return e, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return e, err
	}

	entries, err := All()
	if err != nil {
		return e, err
	}
	e.ID = fmt.Sprintf("L-%04d", len(entries)+1)
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.Agent == "" {
		e.Agent = "human"
	}

	f, err := os.OpenFile(file(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o664)
	if err != nil {
		return e, err
	}
	if st, _ := f.Stat(); st != nil && st.Size() == 0 {
		f.WriteString(header)
	}
	_, err = f.WriteString("\n" + e.Markdown())
	f.Close()
	if err != nil {
		return e, err
	}
	commit(e.ID + " " + e.Summary)
	return e, nil
}

// commit records the change in the ledger's git history. Failures are
// ignored: the markdown file is the source of truth, git is a bonus.
func commit(msg string) {
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	git := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", Dir(), "-c", "user.name=vibian", "-c", "user.email=vibian@localhost", "-c", "safe.directory=*"}, args...)...)
		return cmd.Run()
	}
	if _, err := os.Stat(filepath.Join(Dir(), ".git")); err != nil {
		if git("init", "-q", "--shared=group") != nil {
			return
		}
	}
	if git("add", "ledger.md") == nil {
		git("commit", "-q", "-m", msg)
	}
}

var headRe = regexp.MustCompile(`^## (L-\d+) · (\d{4}-\d\d-\d\d \d\d:\d\d) · ([^·]+) · "(.*)"$`)

// All parses every entry, oldest first.
func All() ([]Entry, error) {
	b, err := os.ReadFile(file())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	var cur *Entry
	for _, ln := range strings.Split(string(b), "\n") {
		if m := headRe.FindStringSubmatch(ln); m != nil {
			if cur != nil {
				out = append(out, *cur)
			}
			t, _ := time.ParseInLocation("2006-01-02 15:04", m[2], time.Local)
			summary, err := strconv.Unquote(`"` + m[4] + `"`)
			if err != nil {
				summary = m[4]
			}
			cur = &Entry{ID: m[1], Time: t, Agent: strings.TrimSpace(m[3]), Summary: summary}
		}
		if cur == nil {
			continue
		}
		cur.Raw += ln + "\n"
		k, v, ok := strings.Cut(strings.TrimPrefix(ln, "- "), ": ")
		if !ok || !strings.HasPrefix(ln, "- ") {
			continue
		}
		switch k {
		case "snapshots":
			cur.Snapshots = v
		case "packages":
			cur.Packages = splitList(v)
		case "files":
			cur.Files = splitList(v)
		case "undo":
			cur.Undo = v
		case "command":
			cur.Command = strings.TrimSuffix(strings.TrimPrefix(v, "`"), "`")
		case "result":
			cur.Result = v
		case "gotcha":
			cur.Gotchas = append(cur.Gotchas, v)
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	for i := range out {
		out[i].Raw = strings.TrimRight(out[i].Raw, "\n")
	}
	return out, nil
}

func splitList(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ", ") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Get finds an entry by id ("L-0042" or "42").
func Get(id string) (Entry, error) {
	if n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(id), "L-")); err == nil {
		id = fmt.Sprintf("L-%04d", n)
	}
	entries, err := All()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("no ledger entry %s", id)
}

// Search returns entries whose text contains every word of q (any case).
func Search(q string) ([]Entry, error) {
	entries, err := All()
	if err != nil {
		return nil, err
	}
	words := strings.Fields(strings.ToLower(q))
	var out []Entry
outer:
	for _, e := range entries {
		text := strings.ToLower(e.Raw)
		for _, w := range words {
			if !strings.Contains(text, w) {
				continue outer
			}
		}
		out = append(out, e)
	}
	return out, nil
}

var pairRe = regexp.MustCompile(`(\w+) (\d+)→(\d+)`)

// Pairs returns the pre→post snapshot pairs an entry recorded, per config.
func (e Entry) Pairs() map[string][2]int {
	out := map[string][2]int{}
	for _, m := range pairRe.FindAllStringSubmatch(e.Snapshots, -1) {
		a, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		out[m[1]] = [2]int{a, b}
	}
	return out
}
