package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every shipped playbook needs frontmatter, the sections the skill relies
// on, and executable scripts that verify.sh can judge.
func TestShippedPlaybooks(t *testing.T) {
	t.Setenv("VIBIAN_PLAYBOOKS", "../../playbooks")
	books, err := loadPlaybooks()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range books {
		if p.Name == "" || p.Description == "" {
			t.Errorf("%s: needs name and description", p.ID)
		}
		if p.Status != "tested" && p.Status != "untested" {
			t.Errorf("%s: status %q, want tested or untested", p.ID, p.Status)
		}
		b, _ := os.ReadFile(filepath.Join(p.Dir, "PLAYBOOK.md"))
		for _, s := range []string{"## Interview", "## Knobs", "## Steps", "## Undo", "VERIFY OK"} {
			if !strings.Contains(string(b), s) {
				t.Errorf("%s: PLAYBOOK.md lacks %q", p.ID, s)
			}
		}
		for _, s := range []string{"system.sh", "user.sh", "verify.sh"} {
			fi, err := os.Stat(filepath.Join(p.Dir, "scripts", s))
			if err != nil || fi.Mode()&0o111 == 0 {
				t.Errorf("%s: scripts/%s missing or not executable", p.ID, s)
			}
		}
	}
}
