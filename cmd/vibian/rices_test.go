package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacobzipper/vibian/internal/rice"
)

// Every built-in rice needs a manifest that parses, a RICE.md, the
// playbooks it builds on, the dotfiles it lists and executable scripts.
func TestBuiltinRices(t *testing.T) {
	t.Setenv("VIBIAN_RICES", "../../rices")
	t.Setenv("VIBIAN_PLAYBOOKS", "../../playbooks")
	books, err := loadPlaybooks()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range books {
		have[p.ID] = true
	}
	dirs, _ := filepath.Glob("../../rices/*/vibian.yaml")
	if len(dirs) == 0 {
		t.Fatal("no built-in rices")
	}
	for _, f := range dirs {
		name := filepath.Base(filepath.Dir(f))
		dir, err := fetchRice(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b, _ := os.ReadFile(filepath.Join(dir, "vibian.yaml"))
		m, err := rice.ParseManifest(string(b))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if m.Name != name {
			t.Errorf("%s: name %q, want the directory's", name, m.Name)
		}
		for _, p := range m.Playbooks {
			if !have[p] {
				t.Errorf("%s: no playbook %q", name, p)
			}
		}
		for _, d := range m.Dotfiles {
			if _, err := os.Stat(filepath.Join(dir, "dotfiles", d)); err != nil {
				t.Errorf("%s: lists %s, which isn't in dotfiles/", name, d)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "RICE.md")); err != nil {
			t.Errorf("%s: no RICE.md", name)
		}
		scripts, _ := filepath.Glob(filepath.Join(dir, "scripts", "*.sh"))
		for _, s := range scripts {
			if fi, _ := os.Stat(s); fi.Mode()&0o111 == 0 {
				t.Errorf("%s: %s isn't executable", name, filepath.Base(s))
			}
		}
	}
	if _, err := fetchRice("no-such-rice"); err == nil || !strings.Contains(err.Error(), "built-in rices: ") {
		t.Errorf("unknown rice: %v, want an error listing the built-in ones", err)
	}
}
