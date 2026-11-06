package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
		if err := setJSON(filepath.Join(home, ".gemini/settings.json"), []string{"context", "fileName"}, []string{"GEMINI.md", "AGENTS.md"}); err != nil {
			return err
		}
	}
	if err := linkSkills(home, h.SkillDirs); err != nil {
		return err
	}
	if err := h.registerMCP(home); err != nil {
		return err
	}
	return h.registerHooks(home)
}

// MCPName is the vibian-system MCP server's name in every harness, and
// ComputerMCPName the vibian-computer one's.
const (
	MCPName         = "vibian-system"
	ComputerMCPName = "vibian-computer"
)

// mcpServer is one of Vibian's MCP servers as a harness starts it.
type mcpServer struct {
	name, description string
	args              []string
}

// vibianBin is the vibian binary harnesses should run.
func vibianBin() string {
	bin := "/usr/bin/vibian"
	if _, err := os.Stat(bin); err != nil {
		if exe, err := os.Executable(); err == nil {
			bin = exe
		}
	}
	return bin
}

// mcpServers are Vibian's servers for this harness. The --agent flag stands
// in for VIBIAN_AGENT, which some harnesses don't pass to MCP servers.
func (h Harness) mcpServers() []mcpServer {
	return []mcpServer{
		{MCPName, "Vibian's system tools: doctor, hardware, the ledger and snapshots", []string{"mcp", "--agent", h.ID}},
		{ComputerMCPName, "Computer use: screenshots, windows, mouse and keyboard on this machine's desktop", []string{"mcp", "computer", "--agent", h.ID}},
	}
}

// registerMCP adds Vibian's servers to the harness's user-level MCP config,
// next to the user's own servers. opencode gets them from
// /etc/opencode/opencode.json, which vibian-base ships.
func (h Harness) registerMCP(home string) error {
	bin := vibianBin()
	if h.ID == "codex" {
		return codexMCP(filepath.Join(home, ".codex/config.toml"), bin, h.mcpServers())
	}
	for _, s := range h.mcpServers() {
		if err := h.registerServer(home, bin, s); err != nil {
			return err
		}
	}
	return nil
}

func (h Harness) registerServer(home, bin string, s mcpServer) error {
	stdio := map[string]any{"command": bin, "args": s.args}
	switch h.ID {
	case "claude":
		return setJSON(filepath.Join(home, ".claude.json"), []string{"mcpServers", s.name},
			map[string]any{"type": "stdio", "command": bin, "args": s.args, "env": map[string]any{}})
	case "gemini":
		return setJSON(filepath.Join(home, ".gemini/settings.json"), []string{"mcpServers", s.name}, stdio)
	case "cursor":
		return setJSON(filepath.Join(home, ".cursor/mcp.json"), []string{"mcpServers", s.name}, stdio)
	case "pi":
		stdio["description"] = s.description
		return setJSON(filepath.Join(home, ".pi/agent/mcp.json"), []string{"mcpServers", s.name}, stdio)
	case "openclaw":
		// openclaw.json is JSON5, so let openclaw edit it, once
		if strings.Contains(string(readFile(filepath.Join(home, ".openclaw/openclaw.json"))), `"`+s.name+`"`) {
			return nil
		}
		oc, ok := h.Path()
		if !ok {
			return nil
		}
		b, _ := json.Marshal(stdio)
		cmd := exec.Command(oc, "mcp", "set", s.name, string(b))
		cmd.Env = Env()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("openclaw mcp set: %v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// hookMark is in every hook command Vibian adds (`<bin> hook turn-start …`),
// so it can find its own whatever the binary is called.
const hookMark = " hook turn-"

// registerHooks makes the harness run `vibian hook turn-start` and
// `turn-end` around each user turn. Codex gets its hooks from
// /etc/codex/requirements.toml, which vibian-base ships: Codex runs managed
// hooks without asking the user to trust them first.
func (h Harness) registerHooks(home string) error {
	bin := vibianBin()
	cmd := func(ev string) string { return bin + " hook " + ev + " --agent " + h.ID }
	switch h.ID {
	case "claude":
		path := filepath.Join(home, ".claude/settings.json")
		for ev, k := range map[string]string{"turn-start": "UserPromptSubmit", "turn-end": "Stop"} {
			e := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": cmd(ev), "timeout": 30}}}
			if err := addHook(path, []string{"hooks", k}, e); err != nil {
				return err
			}
		}
	case "gemini":
		path := filepath.Join(home, ".gemini/settings.json")
		for ev, k := range map[string]string{"turn-start": "BeforeAgent", "turn-end": "AfterAgent"} {
			e := map[string]any{"hooks": []any{map[string]any{"name": "vibian-" + ev, "type": "command", "command": cmd(ev), "timeout": 30000}}}
			if err := addHook(path, []string{"hooks", k}, e); err != nil {
				return err
			}
		}
	case "cursor":
		path := filepath.Join(home, ".cursor/hooks.json")
		if err := setJSON(path, []string{"version"}, 1); err != nil {
			return err
		}
		for ev, k := range map[string]string{"turn-start": "beforeSubmitPrompt", "turn-end": "stop"} {
			if err := addHook(path, []string{"hooks", k}, map[string]any{"command": cmd(ev), "timeout": 30}); err != nil {
				return err
			}
		}
	case "pi":
		return writeIfChanged(filepath.Join(home, ".pi/agent/extensions/vibian.ts"), piExtension(bin))
	case "opencode":
		return writeIfChanged(filepath.Join(home, ".config/opencode/plugins/vibian.ts"), opencodePlugin(bin))
	case "openclaw":
		return h.openclawPlugin(home)
	}
	return nil
}

// opencodePlugin snapshots around each prompt. The "prompt" hook runs once
// per user message; a busy period ends with one session.execution.* event.
// Several plugin instances may see the same event, which is fine: turn-end
// does nothing the second time.
func opencodePlugin(bin string) string {
	return `// Managed by ` + "`vibian agent`" + `; edits are overwritten. Per-turn snapshots.
import { execFile } from "node:child_process"

const hook = (ev: string) =>
  new Promise<void>((done) =>
    execFile(` + fmt.Sprintf("%q", bin) + `, ["hook", ev, "--agent", "opencode"], { timeout: 30000 }, () => done()),
  )

export default {
  id: "vibian",
  async setup(ctx: any) {
    await ctx.session.hook("prompt", () => hook("turn-start"))
    const stop = new AbortController()
    const types = ["session.execution.succeeded", "session.execution.failed", "session.execution.interrupted"]
    void (async () => {
      for await (const _ of ctx.event.subscribe({ signal: stop.signal, types })) await hook("turn-end")
    })().catch(() => {})
    return () => stop.abort()
  },
}
`
}

// openclawPluginDir is the turn-snapshot plugin vibian-base ships.
const openclawPluginDir = "/usr/lib/vibian/openclaw"

// openclawPlugin links the plugin into OpenClaw once. OpenClaw only lets
// plugins it doesn't ship see agent turns with allowConversationAccess.
func (h Harness) openclawPlugin(home string) error {
	if _, err := os.Stat(openclawPluginDir); err != nil {
		return nil
	}
	if strings.Contains(string(readFile(filepath.Join(home, ".openclaw/openclaw.json"))), openclawPluginDir) {
		return nil
	}
	oc, ok := h.Path()
	if !ok {
		return nil
	}
	for _, args := range [][]string{
		{"plugins", "install", "--link", openclawPluginDir, "--force", "--accept-capabilities"},
		{"plugins", "enable", "vibian"},
		{"config", "set", "plugins.entries.vibian.hooks.allowConversationAccess", "true"},
	} {
		cmd := exec.Command(oc, args...)
		cmd.Env = Env()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("openclaw %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// piExtension snapshots around each prompt: before_agent_start fires once per
// user message and agent_settled once pi is idle again.
func piExtension(bin string) string {
	return `// Managed by ` + "`vibian agent`" + `; edits are overwritten. Per-turn snapshots.
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const hook = (pi: ExtensionAPI, ev: string) =>
  pi.exec(` + fmt.Sprintf("%q", bin) + `, ["hook", ev, "--agent", "pi"], { timeout: 30000 }).catch(() => {});

export default function (pi: ExtensionAPI) {
  pi.on("before_agent_start", async (e) => {
    if (e.messageType === "user") await hook(pi, "turn-start");
  });
  pi.on("agent_settled", async () => {
    await hook(pi, "turn-end");
  });
}
`
}

func writeIfChanged(path, text string) error {
	if string(readFile(path)) == text {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func readFile(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

// codexMCP puts the server at the top of config.toml as a root-level dotted
// key, so it never swallows the user's keys the way a [table] block would.
func codexMCP(path, bin string, servers []mcpServer) error {
	text := string(readFile(path))
	var lines []string
	for _, s := range servers {
		lines = append(lines, fmt.Sprintf("mcp_servers.%s = { command = %q, args = %s }", s.name, bin, tomlList(s.args)))
	}
	if strings.Contains(text, "\n[mcp_servers]") || strings.HasPrefix(text, "[mcp_servers]") {
		// a plain [mcp_servers] table can't be mixed with dotted keys
		return fmt.Errorf("%s has an [mcp_servers] table; add Vibian's servers there by hand:\n%s", path,
			strings.ReplaceAll(strings.Join(lines, "\n"), "mcp_servers.", ""))
	}
	return writeMarked(path, "# vibian:begin (managed by `vibian agent`, edits inside are overwritten)", "# vibian:end", strings.Join(lines, "\n"))
}

func tomlList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = fmt.Sprintf("%q", x)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// writeBlock puts block between Vibian's markers in path, keeping whatever
// else the user wrote there.
func writeBlock(path, block string) error {
	return writeMarked(path, beginMark, endMark, block)
}

func writeMarked(path, beginMark, endMark, block string) error {
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

// setJSON sets the value at a key path in a JSON settings file, keeping
// everything else.
func setJSON(path string, keys []string, v any) error {
	return editJSON(path, func(root map[string]any) {
		m := dig(root, keys[:len(keys)-1])
		m[keys[len(keys)-1]] = v
	})
}

// addHook puts entry in the list at a key path, replacing an earlier Vibian
// entry and keeping the user's own.
func addHook(path string, keys []string, entry any) error {
	return editJSON(path, func(root map[string]any) {
		m := dig(root, keys[:len(keys)-1])
		k := keys[len(keys)-1]
		old, _ := m[k].([]any)
		list := []any{}
		for _, e := range old {
			if b, _ := json.Marshal(e); !strings.Contains(string(b), hookMark) {
				list = append(list, e)
			}
		}
		m[k] = append(list, entry)
	})
}

// dig returns the object at a key path, creating it as needed.
func dig(m map[string]any, keys []string) map[string]any {
	for _, k := range keys {
		next, ok := m[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[k] = next
		}
		m = next
	}
	return m
}

// editJSON applies edit to a JSON settings file. It leaves the file alone
// when nothing changes.
func editJSON(path string, edit func(map[string]any)) error {
	cur := map[string]any{}
	old := readFile(path)
	if len(strings.TrimSpace(string(old))) > 0 {
		if err := json.Unmarshal(old, &cur); err != nil {
			return fmt.Errorf("%s is not plain JSON, leaving it alone: %w", path, err)
		}
	}
	before, _ := json.MarshalIndent(cur, "", "  ")
	edit(cur)
	b, _ := json.MarshalIndent(cur, "", "  ")
	if len(old) > 0 && string(before) == string(b) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// new files are private: ~/.claude.json, for one, holds account details
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
