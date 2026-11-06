package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

// Every harness either asks by default, has a way to make it ask, or says
// why it can't; and root mode never carries confirm-only settings.
func TestPowersModes(t *testing.T) {
	for _, h := range All {
		argv, env := h.LaunchArgs(PowersRoot)
		for _, f := range h.ConfirmFlags {
			if slices.Contains(argv, f) {
				t.Errorf("%s: root mode passes confirm flag %s", h.ID, f)
			}
		}
		if len(env) != len(h.LoginEnv) {
			t.Errorf("%s: root mode has confirm env %v", h.ID, env)
		}
		argv, env = h.LaunchArgs(PowersConfirm)
		for _, f := range h.AutoFlags {
			if slices.Contains(argv, f) {
				t.Errorf("%s: confirm mode passes no-prompts flag %s", h.ID, f)
			}
		}
		for _, kv := range env {
			if v, ok := strings.CutPrefix(kv, "OPENCODE_CONFIG_CONTENT="); ok && !json.Valid([]byte(v)) {
				t.Errorf("%s: OPENCODE_CONFIG_CONTENT isn't JSON", h.ID)
			}
		}
		if h.PowersCmd != nil && slices.Equal(h.PowersCmd(PowersRoot), h.PowersCmd(PowersConfirm)) {
			t.Errorf("%s: PowersCmd is the same for both modes", h.ID)
		}
	}
}

// The vibian-system server lands next to the user's own servers and settings,
// and running Configure again changes nothing.
func TestRegisterMCP(t *testing.T) {
	home, kit := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VIBIAN_KIT", kit)
	os.WriteFile(filepath.Join(kit, "AGENTS.md"), []byte("# brief\n"), 0o644)
	write := func(rel, s string) {
		p := filepath.Join(home, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	write(".claude.json", `{"numStartups": 3, "mcpServers": {"mine": {"command": "x"}}}`)
	write(".gemini/settings.json", `{"context": {"other": true}, "mcpServers": {"mine": {"command": "x"}}}`)
	write(".codex/config.toml", "model = \"gpt-5\"\n\n[mcp_servers.mine]\ncommand = \"x\"\n")

	for _, f := range []struct{ id, file string }{
		{"claude", ".claude.json"}, {"gemini", ".gemini/settings.json"},
		{"cursor", ".cursor/mcp.json"}, {"pi", ".pi/agent/mcp.json"},
	} {
		h, _ := Get(f.id)
		for i := 0; i < 2; i++ {
			if err := h.Configure(); err != nil {
				t.Fatal(f.id, err)
			}
		}
		var cfg struct {
			NumStartups int                       `json:"numStartups"`
			Context     map[string]any            `json:"context"`
			Servers     map[string]map[string]any `json:"mcpServers"`
		}
		b, _ := os.ReadFile(filepath.Join(home, f.file))
		if err := json.Unmarshal(b, &cfg); err != nil {
			t.Fatal(f.id, err)
		}
		s := cfg.Servers[MCPName]
		if args, _ := s["args"].([]any); len(args) != 3 || args[2] != f.id || s["command"] == "" {
			t.Errorf("%s: server %v", f.id, s)
		}
		c := cfg.Servers[ComputerMCPName]
		if args, _ := c["args"].([]any); len(args) != 4 || args[1] != "computer" || args[3] != f.id {
			t.Errorf("%s: computer server %v", f.id, c)
		}
		switch f.id {
		case "claude":
			if cfg.NumStartups != 3 || cfg.Servers["mine"] == nil {
				t.Errorf("claude: lost user settings:\n%s", b)
			}
		case "gemini":
			if cfg.Context["other"] != true || cfg.Context["fileName"] == nil || cfg.Servers["mine"] == nil {
				t.Errorf("gemini: lost user settings:\n%s", b)
			}
		}
	}

	codex, _ := Get("codex")
	codex.Configure()
	codex.Configure()
	b, _ := os.ReadFile(filepath.Join(home, ".codex/config.toml"))
	s := string(b)
	if strings.Count(s, "mcp_servers."+MCPName+" = {") != 1 || !strings.HasPrefix(s, "# vibian:begin") ||
		!strings.Contains(s, `args = ["mcp", "--agent", "codex"]`) || !strings.Contains(s, `args = ["mcp", "computer", "--agent", "codex"]`) || !strings.Contains(s, "[mcp_servers.mine]") || !strings.Contains(s, `model = "gpt-5"`) {
		t.Fatalf("codex config.toml:\n%s", s)
	}
}

func TestOpencodeSystemConfig(t *testing.T) {
	b, err := os.ReadFile("../../packages/vibian-base/root/etc/opencode/opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		MCP struct {
			Servers map[string]struct {
				Type    string   `json:"type"`
				Command []string `json:"command"`
			} `json:"servers"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	s := cfg.MCP.Servers[MCPName]
	if s.Type != "local" || !slices.Equal(s.Command, []string{"/usr/bin/vibian", "mcp", "--agent", "opencode"}) {
		t.Fatalf("opencode server %+v", s)
	}
	c := cfg.MCP.Servers[ComputerMCPName]
	if c.Type != "local" || !slices.Equal(c.Command, []string{"/usr/bin/vibian", "mcp", "computer", "--agent", "opencode"}) {
		t.Fatalf("opencode computer server %+v", c)
	}
}

func TestRegisterHooks(t *testing.T) {
	home, kit := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VIBIAN_KIT", kit)
	os.WriteFile(filepath.Join(kit, "AGENTS.md"), []byte("# brief\n"), 0o644)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude/settings.json"),
		[]byte(`{"model": "opus", "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "notify-send done"}]}]}}`), 0o644)

	for _, f := range []struct{ id, file, start, end string }{
		{"claude", ".claude/settings.json", "UserPromptSubmit", "Stop"},
		{"gemini", ".gemini/settings.json", "BeforeAgent", "AfterAgent"},
		{"cursor", ".cursor/hooks.json", "beforeSubmitPrompt", "stop"},
	} {
		h, _ := Get(f.id)
		for i := 0; i < 2; i++ {
			if err := h.Configure(); err != nil {
				t.Fatal(f.id, err)
			}
		}
		b, _ := os.ReadFile(filepath.Join(home, f.file))
		var cfg struct {
			Model   string           `json:"model"`
			Version int              `json:"version"`
			Hooks   map[string][]any `json:"hooks"`
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			t.Fatal(f.id, err)
		}
		for k, ev := range map[string]string{f.start: "turn-start", f.end: "turn-end"} {
			mine := 0
			for _, e := range cfg.Hooks[k] {
				j, _ := json.Marshal(e)
				if strings.Contains(string(j), "hook "+ev+" --agent "+f.id) {
					mine++
				}
			}
			if mine != 1 {
				t.Errorf("%s: %d vibian %s hooks:\n%s", f.id, mine, k, b)
			}
		}
		switch f.id {
		case "claude":
			if cfg.Model != "opus" || len(cfg.Hooks["Stop"]) != 2 || !strings.Contains(string(b), "notify-send done") {
				t.Errorf("claude: lost user settings:\n%s", b)
			}
		case "cursor":
			if cfg.Version != 1 {
				t.Errorf("cursor: version %d", cfg.Version)
			}
		}
	}

	for _, f := range []struct{ id, file string }{
		{"pi", ".pi/agent/extensions/vibian.ts"},
		{"opencode", ".config/opencode/plugins/vibian.ts"},
	} {
		h, _ := Get(f.id)
		if err := h.Configure(); err != nil {
			t.Fatal(f.id, err)
		}
		b, _ := os.ReadFile(filepath.Join(home, f.file))
		if s := string(b); !strings.Contains(s, `"turn-start"`) || !strings.Contains(s, `"turn-end"`) || !strings.Contains(s, `"--agent", "`+f.id+`"`) {
			t.Errorf("%s plugin:\n%s", f.id, s)
		}
	}
}

func TestCodexManagedHooks(t *testing.T) {
	b, err := os.ReadFile("../../packages/vibian-base/root/etc/codex/requirements.toml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"[[hooks.UserPromptSubmit]]", "[[hooks.Stop]]", "hook turn-start --agent codex", "hook turn-end --agent codex"} {
		if !strings.Contains(s, want) {
			t.Errorf("requirements.toml lacks %q", want)
		}
	}
}
