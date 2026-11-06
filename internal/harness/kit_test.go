package harness

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every shipped skill must load in every harness: Agent Skills frontmatter
// with a name matching its directory and a description saying when to use it,
// and AGENTS.md must list it.
func TestKitSkills(t *testing.T) {
	kit := filepath.Join("..", "..", "agent-kit")
	brief, err := os.ReadFile(filepath.Join(kit, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	dirs, _ := filepath.Glob(filepath.Join(kit, "skills", "*"))
	if len(dirs) == 0 {
		t.Fatal("no skills")
	}
	front := regexp.MustCompile(`(?s)\A---\nname: ([a-z0-9-]+)\ndescription: "((?:[^"\\\n]|\\.)+)"\n---\n`)
	for _, d := range dirs {
		id := filepath.Base(d)
		b, err := os.ReadFile(filepath.Join(d, "SKILL.md"))
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		m := front.FindSubmatch(b)
		switch {
		case m == nil:
			t.Errorf("%s: SKILL.md must start with name/description frontmatter", id)
		case string(m[1]) != id:
			t.Errorf("%s: name is %q", id, m[1])
		case len(m[2]) > 1024 || !strings.Contains(string(m[2]), "Use "):
			t.Errorf("%s: description must be under 1024 chars and say when to use it", id)
		}
		if !strings.Contains(string(brief), "`"+id+"`") {
			t.Errorf("%s: not listed in AGENTS.md", id)
		}
	}
}
