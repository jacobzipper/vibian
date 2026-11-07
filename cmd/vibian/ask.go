package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/jacobzipper/vibian/internal/harness"
)

// An ask is one request run by the agent in the background, with no
// session: the Ask overlay sends them, and `vibian asks` lists them. Each
// lives in ~/.local/state/vibian/asks/<id>/ as ask.json (this struct),
// out.log (the answer) and err.log.
type ask struct {
	ID       string    `json:"id"`
	Request  string    `json:"request"`
	Agent    string    `json:"agent"`
	State    string    `json:"state"` // running, done, failed, stopped
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished,omitzero"`
	PID      int       `json:"pid,omitempty"`
	Ledger   string    `json:"ledger,omitempty"` // the stub entry, if the ask changed files without one
	Answer   string    `json:"answer,omitempty"` // filled by `vibian asks --json`
}

func asksDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local/state")
	}
	return filepath.Join(dir, "vibian", "asks")
}

func loadAsk(id string) (ask, error) {
	var a ask
	if id == "" || strings.ContainsAny(id, "/.") {
		return a, fmt.Errorf("no ask %q", id)
	}
	b, err := os.ReadFile(filepath.Join(asksDir(), id, "ask.json"))
	if os.IsNotExist(err) {
		return a, fmt.Errorf("no ask %q (vibian asks lists them)", id)
	}
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return a, err
	}
	// a runner that died without writing its result, or never started
	if a.State == "running" {
		if a.PID > 0 && syscall.Kill(a.PID, 0) != nil || a.PID == 0 && time.Since(a.Started) > time.Minute {
			a.State = "failed"
		}
	}
	return a, nil
}

func (a ask) save() error {
	b, _ := json.MarshalIndent(a, "", "  ")
	p := filepath.Join(asksDir(), a.ID, "ask.json")
	if err := os.WriteFile(p+".tmp", append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

func (a ask) file(name string) string { return filepath.Join(asksDir(), a.ID, name) }

// answer is what the agent printed, or its errors if it printed nothing.
func (a ask) answer() string {
	b, _ := os.ReadFile(a.file("out.log"))
	if s := strings.TrimSpace(string(b)); s != "" {
		return s
	}
	b, _ = os.ReadFile(a.file("err.log"))
	return strings.TrimSpace(string(b))
}

// cmdAsk starts a request in the background and prints its id.
//
//	vibian ask "request"          in the background; a notification says when it's done
//	vibian ask --wait "request"   in the foreground, printing the answer
func cmdAsk(args []string) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	wait := fs.Bool("wait", false, "run in the foreground and print the answer")
	runID := fs.String("run", "", "") // internal: the background runner
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if *runID != "" {
		return runAsk(*runID, false)
	}
	req := strings.TrimSpace(strings.Join(pos, " "))
	if req == "" {
		return errors.New(`usage: vibian ask [--wait] "what you'd like done"`)
	}
	cfg, err := harness.LoadConfig()
	if err != nil {
		return err
	}
	h, err := askHarness(cfg)
	if err != nil {
		return err
	}
	a := ask{Request: req, Agent: h.ID, State: "running", Started: time.Now()}
	if err := os.MkdirAll(asksDir(), 0o700); err != nil {
		return err
	}
	for i := 0; ; i++ {
		a.ID = a.Started.Format("20060102-150405")
		if i > 0 {
			a.ID += fmt.Sprintf("-%d", i)
		}
		if err := os.Mkdir(filepath.Join(asksDir(), a.ID), 0o700); err == nil {
			break
		} else if !os.IsExist(err) {
			return err
		}
	}
	if err := a.save(); err != nil {
		return err
	}
	if *wait {
		return runAsk(a.ID, true)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	// its own session, so closing the overlay or terminal doesn't stop it
	c := exec.Command(self, "ask", "--run", a.ID)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return err
	}
	// the runner records its own pid: saving here could undo its result
	c.Process.Release()
	fmt.Println(a.ID)
	return nil
}

func askHarness(cfg harness.Config) (harness.Harness, error) {
	if cfg.Harness == "" {
		return harness.Harness{}, errors.New("no agent picked yet: run `vibian onboard` or `vibian agent set <harness>`")
	}
	if os.Geteuid() == 0 {
		return harness.Harness{}, errors.New("ask as your own user, not root")
	}
	h, err := harness.Get(cfg.Harness)
	if err != nil {
		return h, err
	}
	if h.OneShot == nil {
		return h, fmt.Errorf("%s has no one-shot mode: use `vibian agent`", h.Name)
	}
	if _, ok := h.Path(); !ok {
		return h, fmt.Errorf("%s isn't installed: vibian agent set %s", h.Name, h.ID)
	}
	return h, nil
}

// runAsk runs the agent on one ask between a snapshot pair and records the
// result. In the background it ends with a desktop notification.
func runAsk(id string, foreground bool) error {
	a, err := loadAsk(id)
	if err != nil {
		return err
	}
	cfg, err := harness.LoadConfig()
	if err != nil {
		return err
	}
	h, err := harness.Get(a.Agent)
	if err != nil {
		return err
	}
	bin, _ := h.Path()
	flags, env := h.LaunchArgs(cfg.Powers)
	out, err := os.Create(a.file("out.log"))
	if err != nil {
		return err
	}
	defer out.Close()
	errLog, err := os.Create(a.file("err.log"))
	if err != nil {
		return err
	}
	defer errLog.Close()

	home, _ := os.UserHomeDir()
	c := exec.Command(bin, h.OneShot(a.Request, flags)...)
	c.Dir = home
	// VIBIAN_ASK: the ask has its own snapshot pair, so turn hooks skip it
	c.Env = harness.Env(append(env, "VIBIAN_AGENT="+h.ID, "VIBIAN_ASK="+a.ID)...)
	c.Stdout, c.Stderr = out, errLog
	if foreground {
		c.Stdout = io.MultiWriter(out, os.Stdout)
		c.Stderr = io.MultiWriter(errLog, os.Stderr)
	} else {
		// `vibian asks stop` signals the whole session: the agent stops,
		// this runner stays to record it
		signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM, syscall.SIGHUP)
	}
	a.PID = os.Getpid()
	a.save()
	short := a.Request
	if len([]rune(short)) > 60 {
		short = string([]rune(short)[:59]) + "…"
	}
	a.Ledger, err = inSnapshots(h, "ask: "+short, "ask ended without a ledger entry (auto-recorded): "+short, false, c.Run)
	a.Finished = time.Now()
	a.State = "done"
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit) && exit.ExitCode() < 0:
		a.State = "stopped"
	case err != nil:
		a.State = "failed"
		fmt.Fprintln(errLog, "vibian:", err)
	}
	a.PID = 0
	if err := a.save(); err != nil {
		return err
	}
	if !foreground {
		notifyAsk(a)
	}
	return nil
}

// notifyAsk shows the end of the answer as a desktop notification, when
// there is a desktop to show it.
func notifyAsk(a ask) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return
	}
	title := "Your agent is done"
	if a.State != "done" {
		title = "Your agent couldn't finish"
	}
	body := a.answer()
	if r := []rune(body); len(r) > 300 {
		body = "…" + string(r[len(r)-299:])
	}
	if body == "" {
		body = a.Request
	}
	icon := "/usr/share/vibian/vibian-logo.png"
	exec.Command("notify-send", "--app-name=Vibian", "--icon="+icon, title, body).Run()
}

// cmdAsks lists recent asks, shows one, or stops one.
//
//	vibian asks [-n N] [--json]   newest first
//	vibian asks ID [--json]       one ask with its answer
//	vibian asks stop ID
func cmdAsks(args []string) error {
	fs := flag.NewFlagSet("asks", flag.ContinueOnError)
	n := fs.Int("n", 20, "how many")
	asJSON := fs.Bool("json", false, "JSON, with answers")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 2 && pos[0] == "stop" {
		a, err := loadAsk(pos[1])
		if err != nil {
			return err
		}
		if a.State != "running" {
			return fmt.Errorf("ask %s is %s", a.ID, a.State)
		}
		if a.PID <= 0 {
			return fmt.Errorf("ask %s hasn't started yet: try again in a moment", a.ID)
		}
		// the runner's session: the agent and anything it started
		return syscall.Kill(-a.PID, syscall.SIGTERM)
	}
	if len(pos) == 1 {
		a, err := loadAsk(pos[0])
		if err != nil {
			return err
		}
		a.Answer = a.answer()
		if *asJSON {
			b, _ := json.MarshalIndent(a, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("%s  %s  %s\n> %s\n\n%s\n", a.ID, a.Agent, a.State, a.Request, a.Answer)
		if a.Ledger != "" {
			fmt.Printf("\nIt changed files without a ledger entry; vibian recorded it as %s.\n", a.Ledger)
		}
		return nil
	}
	if len(pos) > 0 {
		return errors.New("usage: vibian asks [-n N] [--json] [ID | stop ID]")
	}
	entries, _ := os.ReadDir(asksDir())
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	var list []ask
	for _, id := range ids {
		if len(list) == *n {
			break
		}
		if a, err := loadAsk(id); err == nil {
			if *asJSON {
				a.Answer = a.answer()
			}
			list = append(list, a)
		}
	}
	if *asJSON {
		if list == nil {
			list = []ask{}
		}
		b, _ := json.MarshalIndent(list, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	if len(list) == 0 {
		fmt.Println(`no asks yet: vibian ask "…", or the Ask overlay`)
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, a := range list {
		req := a.Request
		if r := []rune(req); len(r) > 60 {
			req = string(r[:59]) + "…"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.ID, a.Agent, a.State, req)
	}
	return tw.Flush()
}
