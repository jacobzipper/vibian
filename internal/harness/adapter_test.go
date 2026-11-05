package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureIsIdempotentAndKeepsUserText(t *testing.T) {
	home, kit := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VIBIAN_KIT", kit)
	os.MkdirAll(filepath.Join(kit, "skills", "recovery"), 0o755)
	os.WriteFile(filepath.Join(kit, "skills", "recovery", "SKILL.md"), []byte("---\nname: recovery\n---\n"), 0o644)
	os.WriteFile(filepath.Join(kit, "AGENTS.md"), []byte("# Vibian brief v1\n"), 0o644)

	codex, _ := Get("codex")
	user := filepath.Join(home, ".codex/AGENTS.md")
	os.MkdirAll(filepath.Dir(user), 0o755)
	os.WriteFile(user, []byte("my own rules\n"), 0o644)
	for i := 0; i < 2; i++ {
		if err := codex.Configure(); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(kit, "AGENTS.md"), []byte("# Vibian brief v2\n"), 0o644)
	codex.Configure()
	b, _ := os.ReadFile(user)
	s := string(b)
	if strings.Count(s, beginMark) != 1 || !strings.Contains(s, "v2") || strings.Contains(s, "v1") || !strings.Contains(s, "my own rules") {
		t.Fatalf("instructions:\n%s", s)
	}
	if dst, err := os.Readlink(filepath.Join(home, ".agents/skills/recovery")); err != nil || dst != filepath.Join(kit, "skills", "recovery") {
		t.Fatalf("skill link %q %v", dst, err)
	}

	claude, _ := Get("claude")
	claude.Configure()
	b, _ = os.ReadFile(filepath.Join(home, ".claude/CLAUDE.md"))
	if !strings.Contains(string(b), "@"+filepath.Join(kit, "AGENTS.md")) {
		t.Fatalf("claude import missing:\n%s", b)
	}
}
