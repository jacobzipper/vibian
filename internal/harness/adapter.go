package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KitDir holds AGENTS.md and skills/. VIBIAN_KIT overrides it for development.
func KitDir() string {
	if d := os.Getenv("VIBIAN_KIT"); d != "" {
		return d
	}
	return "/usr/share/vibian/agent"
}

const (
	beginMark = "<!-- vibian:begin (managed by `vibian agent`, edits inside are overwritten) -->"
	endMark   = "<!-- vibian:end -->"
)

// Configure links the system brief and skills into the harness's user-level
// config. It is idempotent and runs before every launch, so package updates
// to the kit reach the agent without a manual step.
func (h Harness) Configure() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	brief := filepath.Join(KitDir(), "AGENTS.md")
	var block string
	if h.ID == "claude" {
		// Claude Code has no user-level AGENTS.md; CLAUDE.md imports it.
		block = "@" + brief
	} else {
		b, err := os.ReadFile(brief)
		if err != nil {
			return fmt.Errorf("agent kit missing: %w", err)
		}
		block = strings.TrimSpace(string(b))
	}
	if err := writeBlock(filepath.Join(home, h.Instructions), block); err != nil {
		return err
	}
	if h.ID == "gemini" {
		// GEMINI.md is enough, but keep AGENTS.md readable from project dirs too.
		if err := mergeJSON(filepath.Join(home, ".gemini/settings.json"), map[string]any{
			"context": map[string]any{"fileName": []string{"GEMINI.md", "AGENTS.md"}},
		}); err != nil {
			return err
		}
	}
	return linkSkills(home, h.SkillDirs)
}

// writeBlock puts block between Vibian's markers in path, keeping whatever
// else the user wrote there.
func writeBlock(path, block string) error {
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		// never write through a symlink into somebody else's file
		os.Remove(path)
	}
	managed := beginMark + "\n" + block + "\n" + endMark
	text := string(old)
	if i := strings.Index(text, beginMark); i >= 0 {
		if j := strings.Index(text[i:], endMark); j >= 0 {
			text = text[:i] + managed + text[i+j+len(endMark):]
		} else {
			text = text[:i] + managed
		}
	} else if strings.TrimSpace(text) == "" {
		text = managed + "\n"
	} else {
		text = managed + "\n\n" + text
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// linkSkills symlinks every kit skill into each skills dir. A user's own
// skill with the same name wins: we never replace a real directory.
func linkSkills(home string, dirs []string) error {
	skills, _ := filepath.Glob(filepath.Join(KitDir(), "skills", "*", "SKILL.md"))
	for _, d := range dirs {
		dst := filepath.Join(home, d)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		for _, s := range skills {
			src := filepath.Dir(s)
			link := filepath.Join(dst, filepath.Base(src))
			if st, err := os.Lstat(link); err == nil {
				if st.Mode()&os.ModeSymlink == 0 {
					continue
				}
				os.Remove(link)
			}
			if err := os.Symlink(src, link); err != nil {
				return err
			}
		}
	}
	return nil
}

// mergeJSON sets keys in a JSON settings file, keeping the rest.
func mergeJSON(path string, set map[string]any) error {
	cur := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &cur); err != nil {
			return fmt.Errorf("%s is not plain JSON, leaving it alone: %w", path, err)
		}
	}
	for k, v := range set {
		cur[k] = v
	}
	b, _ := json.MarshalIndent(cur, "", "  ")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
