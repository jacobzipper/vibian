package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacobzipper/vibian/internal/harness"
)

// fakeAgent puts a pi that prints its arguments (or sleeps) on PATH, with
// HOME and the state dir in a temp dir.
func fakeAgent(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	os.MkdirAll(bin, 0o755)
	script := "#!/bin/sh\ncase \"$2\" in sleep) exec sleep 30 ;; esac\necho \"args: $*\"\necho \"ask $VIBIAN_ASK\" >&2\n"
	if err := os.WriteFile(filepath.Join(bin, "pi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	if err := (harness.Config{Harness: "pi", Powers: harness.PowersRoot}).Save(); err != nil {
		t.Fatal(err)
	}
	return home
}

func onlyAsk(t *testing.T) ask {
	t.Helper()
	entries, _ := os.ReadDir(asksDir())
	if len(entries) != 1 {
		t.Fatalf("want one ask, got %d", len(entries))
	}
	a, err := loadAsk(entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAskWait(t *testing.T) {
	fakeAgent(t)
	if err := cmdAsk([]string{"--wait", "make", "the font bigger"}); err != nil {
		t.Fatal(err)
	}
	a := onlyAsk(t)
	if a.State != "done" || a.Agent != "pi" || a.Request != "make the font bigger" || a.Finished.IsZero() {
		t.Fatalf("ask: %+v", a)
	}
	if got := a.answer(); got != "args: -p make the font bigger" {
		t.Fatalf("answer %q", got)
	}
	errLog, _ := os.ReadFile(a.file("err.log"))
	if strings.TrimSpace(string(errLog)) != "ask "+a.ID {
		t.Fatalf("VIBIAN_ASK not passed: %q", errLog)
	}
}

// TestMain lets the test binary stand in for vibian when an ask re-runs
// itself in the background.
func TestMain(m *testing.M) {
	if os.Getenv("VIBIAN_TEST_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestAskBackgroundAndStop(t *testing.T) {
	fakeAgent(t)
	t.Setenv("VIBIAN_TEST_MAIN", "1")
	wait := func(state string) ask {
		t.Helper()
		for range 100 {
			if a := onlyAsk(t); a.State == state && (state != "running" || a.PID != 0) {
				return a
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("never %s: %+v", state, onlyAsk(t))
		return ask{}
	}
	if err := cmdAsk([]string{"sleep"}); err != nil {
		t.Fatal(err)
	}
	a := wait("running")
	time.Sleep(300 * time.Millisecond) // the agent has started
	if err := cmdAsks([]string{"stop", a.ID}); err != nil {
		t.Fatal(err)
	}
	wait("stopped")
}

func TestAsksJSON(t *testing.T) {
	fakeAgent(t)
	os.MkdirAll(filepath.Join(asksDir(), "20261106-120000"), 0o700)
	old := ask{ID: "20261106-120000", Request: "old", Agent: "pi", State: "running", Started: time.Now().Add(-2 * time.Minute)}
	old.save()
	young := ask{ID: "20261106-120001", Request: "young", Agent: "pi", State: "running", Started: time.Now()}
	os.MkdirAll(filepath.Join(asksDir(), young.ID), 0o700)
	young.save()
	if err := cmdAsks([]string{"stop", young.ID}); err == nil || !strings.Contains(err.Error(), "started") {
		t.Fatalf("stop before the runner started: %v", err)
	}
	os.RemoveAll(filepath.Join(asksDir(), young.ID))
	if err := cmdAsk([]string{"--wait", "new"}); err != nil {
		t.Fatal(err)
	}
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	err := cmdAsks([]string{"--json"})
	w.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatal(err)
	}
	var list []ask
	if err := json.NewDecoder(r).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Request != "new" || list[0].Answer != "args: -p new" {
		t.Fatalf("list: %+v", list)
	}
	// a runner that is gone reads as failed
	if list[1].State != "failed" {
		t.Fatalf("dead runner: %+v", list[1])
	}
	if _, err := loadAsk("../x"); err == nil {
		t.Fatal("loadAsk took a path")
	}
}

func TestOneShotArgs(t *testing.T) {
	for _, h := range harness.All {
		if h.OneShot == nil {
			t.Errorf("%s: no one-shot mode", h.ID)
			continue
		}
		argv := h.OneShot("hi there", h.AutoFlags)
		if argv[len(argv)-1] != "hi there" {
			t.Errorf("%s: the request must be the last argument: %q", h.ID, argv)
		}
		for _, f := range h.AutoFlags {
			if !strings.Contains(strings.Join(argv, " "), f) {
				t.Errorf("%s: lost flag %s: %q", h.ID, f, argv)
			}
		}
	}
}
