// Package harness knows how to install, log in to, configure and launch each
// supported agent harness. Facts come from docs/research/harness-adapters.md.
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Harness is one agent CLI Vibian can drive.
type Harness struct {
	ID      string
	Name    string
	Vendor  string
	Licence string
	Binary  string
	// Install is a shell command run as the user. Proprietary harnesses are
	// never shipped on the ISO; this runs each one's official installer.
	Install string
	// Login runs after install. Empty means the harness logs in on first launch.
	Login []string
	// LoginHint tells the user how to log in when Login is empty.
	LoginHint string
	// LoginEnv is set for both Login and the first launch (no browser yet).
	LoginEnv []string
	// Instructions is the user-level instructions file, relative to $HOME.
	// Claude Code gets an @import line instead of a copy of AGENTS.md.
	Instructions string
	// SkillDirs are where skills are linked, relative to $HOME.
	SkillDirs []string
	// AutoFlags put the harness in its no-prompts mode (powers = root).
	AutoFlags []string
	// ConfirmFlags and ConfirmEnv make it ask before acting (powers =
	// confirm) when its default mode doesn't.
	ConfirmFlags []string
	ConfirmEnv   []string
	// PowersCmd sets an approval policy the harness stores itself. It runs
	// at launch whenever powers changed since it last ran.
	PowersCmd func(p Powers) []string
	// NoConfirm says what "ask first" can't do for a harness that has no
	// approval prompts.
	NoConfirm string
	// PromptArgs starts an interactive session with a first message. Nil when
	// a first message would fail because login happens inside the session.
	PromptArgs func(prompt string) []string
	// OneShot runs one request with no session and exits: the arguments
	// after the binary, given the powers flags from LaunchArgs.
	OneShot func(prompt string, flags []string) []string
	// StateDirs hold the harness's own sessions, caches and databases,
	// relative to $HOME. Changes there don't count as the session changing
	// the system.
	StateDirs []string
	// Unverified marks paths taken from docs but not yet run in a VM.
	Unverified bool
}

var agentsSkills = ".agents/skills"

// All is the onboarding order.
var All = []Harness{
	{
		ID: "claude", Name: "Claude Code", Vendor: "Anthropic", Licence: "proprietary", Binary: "claude",
		Install:      "curl -fsSL https://claude.ai/install.sh | bash",
		LoginHint:    "Claude Code asks you to log in when it starts. Open the link it shows on your phone.",
		Instructions: ".claude/CLAUDE.md",
		SkillDirs:    []string{".claude/skills"},
		StateDirs:    []string{".claude", ".claude.json"},
		AutoFlags:    []string{"--dangerously-skip-permissions"},
		PromptArgs:   func(p string) []string { return []string{p} },
		OneShot:      func(p string, f []string) []string { return append(f, "-p", p) },
	},
	{
		ID: "codex", Name: "Codex CLI", Vendor: "OpenAI", Licence: "Apache-2.0", Binary: "codex",
		Install:      "curl -fsSL https://chatgpt.com/codex/install.sh | sh",
		Login:        []string{"codex", "login", "--device-auth"},
		Instructions: ".codex/AGENTS.md",
		SkillDirs:    []string{agentsSkills},
		StateDirs:    []string{".codex"},
		AutoFlags:    []string{"--dangerously-bypass-approvals-and-sandbox"},
		PromptArgs:   func(p string) []string { return []string{p} },
		// exec refuses to run outside a git repo without the flag
		OneShot: func(p string, f []string) []string {
			return append(append([]string{"exec", "--skip-git-repo-check"}, f...), p)
		},
	},
	{
		ID: "gemini", Name: "Gemini CLI", Vendor: "Google", Licence: "Apache-2.0", Binary: "gemini",
		Install:      "command -v npm >/dev/null || sudo apt-get install -y nodejs npm; npm install -g --prefix \"$HOME/.local\" @google/gemini-cli",
		LoginEnv:     []string{"NO_BROWSER=true"},
		Instructions: ".gemini/GEMINI.md",
		SkillDirs:    []string{agentsSkills},
		StateDirs:    []string{".gemini"},
		AutoFlags:    []string{"--approval-mode=yolo"},
		PromptArgs:   func(p string) []string { return []string{"-i", p} },
		OneShot:      func(p string, f []string) []string { return append(f, "-p", p) },
	},
	{
		ID: "opencode", Name: "opencode", Vendor: "Anomaly", Licence: "MIT", Binary: "opencode",
		Install:      "curl -fsSL https://opencode.ai/v2/install | bash",
		Login:        []string{"opencode", "auth", "login"},
		LoginHint:    "opencode has free models built in, so logging in to a provider is optional (Esc, then s, skips it).",
		Instructions: ".config/opencode/AGENTS.md",
		SkillDirs:    []string{agentsSkills},
		StateDirs:    []string{".local/share/opencode", ".local/state/opencode", ".opencode"},
		AutoFlags:    []string{"--auto"},
		// Its default agent allows every tool. The rules are appended after
		// the user's opencode.jsonc, so they win over its allow rules, and
		// --standalone keeps them out of the shared background server,
		// which would otherwise keep the env of whoever started it.
		ConfirmFlags: []string{"--standalone"},
		ConfirmEnv:   []string{`OPENCODE_CONFIG_CONTENT={"permissions":[{"action":"shell","resource":"*","effect":"ask"},{"action":"edit","resource":"*","effect":"ask"}]}`},
		PromptArgs:   func(p string) []string { return []string{"--prompt", p} },
		OneShot:      func(p string, f []string) []string { return append(append([]string{"run"}, f...), p) },
	},
	{
		ID: "openclaw", Name: "OpenClaw", Vendor: "OpenClaw", Licence: "MIT", Binary: "openclaw",
		Install:      "curl -fsSL https://openclaw.ai/install.sh | bash -s -- --no-onboard",
		Login:        []string{"openclaw", "onboard"},
		Instructions: ".openclaw/workspace/AGENTS.md",
		SkillDirs:    []string{agentsSkills},
		StateDirs:    []string{".openclaw"},
		PromptArgs:   func(p string) []string { return []string{"tui", "--message", p} },
		// --local: the embedded agent; without it, the turn goes through a
		// gateway that needs its own credentials
		OneShot: func(p string, f []string) []string {
			return append(append([]string{"agent", "--local"}, f...), "--message", p)
		},
		// Its default runs commands without asking; both presets work
		// offline and persist in ~/.openclaw/state.
		PowersCmd: func(p Powers) []string {
			if p == PowersConfirm {
				return []string{"exec-policy", "preset", "cautious"}
			}
			return []string{"exec-policy", "preset", "yolo"}
		},
		// install and flags checked in the VM; the TUI needs the Gateway that
		// `openclaw onboard` sets up, untested without a model account
		Unverified: true,
	},
	{
		ID: "pi", Name: "pi", Vendor: "Earendil", Licence: "MIT", Binary: "pi",
		// stdout through cat: the installer ends with "Start pi now? [Y/n]"
		// only when stdout is a terminal, and vibian starts pi itself after
		// configuring it. Its other prompts read /dev/tty, so they still work.
		Install:      "curl -fsSL https://pi.dev/install.sh | sh | cat",
		LoginHint:    "pi starts without a model: type /login and pick a provider, then say hi.",
		Instructions: ".pi/agent/AGENTS.md",
		SkillDirs:    []string{agentsSkills},
		StateDirs:    []string{".pi"},
		NoConfirm:    "pi has no approval prompts: it runs commands and edits files without asking. Only sudo asks for your password.",
		OneShot:      func(p string, f []string) []string { return append(f, "-p", p) },
	},
	{
		ID: "cursor", Name: "Cursor CLI", Vendor: "Anysphere", Licence: "proprietary", Binary: "agent",
		Install:  "curl -fsS https://cursor.com/install | bash",
		Login:    []string{"agent", "login"},
		LoginEnv: []string{"NO_OPEN_BROWSER=1"},
		// Cursor has no user-level instructions file; it reads AGENTS.md from
		// the working directory, and the launcher starts it in $HOME.
		Instructions: "AGENTS.md",
		SkillDirs:    []string{agentsSkills},
		StateDirs:    []string{".cursor", ".local/share/cursor-agent"},
		AutoFlags:    []string{"--force"},
		PromptArgs:   func(p string) []string { return []string{p} },
		OneShot:      func(p string, f []string) []string { return append(f, "-p", p) },
	},
}

// Get finds a harness by id.
func Get(id string) (Harness, error) {
	for _, h := range All {
		if h.ID == id {
			return h, nil
		}
	}
	var ids []string
	for _, h := range All {
		ids = append(ids, h.ID)
	}
	return Harness{}, fmt.Errorf("unknown harness %q (one of: %s)", id, strings.Join(ids, ", "))
}

// Path returns the harness binary, looking in ~/.local/bin too (where most
// official installers put it, and which isn't on PATH until the next login).
func (h Harness) Path() (string, bool) {
	if p, err := exec.LookPath(h.Binary); err == nil {
		return p, true
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{".local/bin", ".opencode/bin", ".npm-global/bin"} {
		p := filepath.Join(home, dir, h.Binary)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

// Installed reports whether the harness binary exists.
func (h Harness) Installed() bool {
	_, ok := h.Path()
	return ok
}

// Env is the environment harness commands run with.
func Env(extra ...string) []string {
	home, _ := os.UserHomeDir()
	env := os.Environ()
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + filepath.Join(home, ".local/bin") + ":" + strings.TrimPrefix(kv, "PATH=")
		}
	}
	return append(env, extra...)
}

// InstallCmd returns the command that runs the official installer.
func (h Harness) InstallCmd() *exec.Cmd {
	cmd := exec.Command("bash", "-o", "pipefail", "-c", h.Install)
	cmd.Env = Env()
	return cmd
}

// LaunchArgs are the flags and extra env for a session with powers p.
func (h Harness) LaunchArgs(p Powers) (argv, env []string) {
	env = append(env, h.LoginEnv...)
	if p == PowersConfirm {
		return append(argv, h.ConfirmFlags...), append(env, h.ConfirmEnv...)
	}
	return append(argv, h.AutoFlags...), env
}

// LoginCmd returns the login command, or nil if login happens on first launch.
func (h Harness) LoginCmd() *exec.Cmd {
	if len(h.Login) == 0 {
		return nil
	}
	bin, ok := h.Path()
	if !ok {
		bin = h.Login[0]
	}
	cmd := exec.Command(bin, h.Login[1:]...)
	cmd.Env = Env(h.LoginEnv...)
	return cmd
}
