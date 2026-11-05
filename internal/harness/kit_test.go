package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Harnesses parse SKILL.md frontmatter as YAML and drop a skill whose
// frontmatter doesn't parse (pi rejected an unquoted "Vibian: resolution").
func TestKitSkillFrontmatter(t *testing.T) {
	files, _ := filepath.Glob("../../agent-kit/skills/*/SKILL.md")
	if len(files) == 0 {
		t.Fatal("no skills found")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.SplitN(string(b), "---\n", 3)
		if len(parts) != 3 || parts[0] != "" {
			t.Errorf("%s: no --- frontmatter at the top", f)
			continue
		}
		keys := map[string]string{}
		for _, ln := range strings.Split(strings.TrimSpace(parts[1]), "\n") {
			k, v, ok := strings.Cut(ln, ": ")
			if !ok {
				t.Errorf("%s: frontmatter line isn't `key: value`: %q", f, ln)
				continue
			}
			quoted := strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`)
			if !quoted && (strings.Contains(v, ": ") || strings.Contains(v, " #")) {
				t.Errorf("%s: %s needs quotes (contains ': ' or ' #')", f, k)
			}
			keys[k] = v
		}
		if keys["name"] != filepath.Base(filepath.Dir(f)) {
			t.Errorf("%s: name %q doesn't match its directory", f, keys["name"])
		}
		if keys["description"] == "" {
			t.Errorf("%s: no description", f)
		}
	}
}
