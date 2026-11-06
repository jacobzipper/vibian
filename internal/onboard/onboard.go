// Package onboard is the first-boot TUI: network, hardware, pick an agent,
// install and log in, choose its powers, hand over.
package onboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jacobzipper/vibian/internal/doctor"
	"github.com/jacobzipper/vibian/internal/harness"
)

type step int

const (
	stepWelcome step = iota
	stepNetwork
	stepHardware
	stepAgent
	stepInstall
	stepPowers
	stepHandoff
)

var stepNames = []string{"Welcome", "Network", "Hardware", "Agent", "Install", "Powers", "Go"}

var (
	accent  = lipgloss.Color("#E8A33D") // sphinx-cat amber
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bold    = lipgloss.NewStyle().Bold(true)
	title   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	sel     = lipgloss.NewStyle().Bold(true).Foreground(accent)
	okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnSty = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	failSty = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	frame   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(1, 2)
)

const cat = `  /\_/\
 ( o.- )
  > ^ <`

// Result is what the TUI decided.
type Result struct {
	Done    bool
	Harness string
	Powers  harness.Powers
}

type checksMsg []doctor.Check
type execMsg struct {
	what string
	err  error
}

// Model is the Bubble Tea model.
type Model struct {
	step     step
	cursor   int
	checks   []doctor.Check
	checking bool
	spin     spinner.Model
	harness  harness.Harness
	busy     string
	err      error
	powers   harness.Powers
	noLogin  bool // login failed or was cancelled; s skips it
	self     string
	width    int
	Result   Result
}

// New builds the model. self is the vibian binary, re-invoked for the steps
// that need sudo (they run on the real terminal so sudo can ask).
func New(self string) Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(accent)
	cfg, _ := harness.LoadConfig()
	m := Model{spin: s, self: self, powers: harness.PowersRoot}
	if cfg.Powers != "" {
		m.powers = cfg.Powers
	}
	return m
}

func (m Model) Init() tea.Cmd { return m.spin.Tick }

func runChecks() tea.Msg { return checksMsg(doctor.Run()) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case checksMsg:
		m.checks, m.checking = msg, false
	case execMsg:
		return m.afterExec(msg)
	case tea.KeyMsg:
		if m.busy != "" {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			m.cursor++
			if max := m.options() - 1; m.cursor > max {
				m.cursor = max
			}
		case "b", "left":
			if m.step > stepWelcome {
				m.step--
				m.cursor, m.err = 0, nil
			}
		case "s":
			if m.step == stepInstall && m.noLogin {
				return m.afterExec(execMsg{"login", nil})
			}
		case "r":
			if m.step == stepNetwork || m.step == stepHardware {
				m.checking = true
				return m, runChecks
			}
		case "enter", " ":
			return m.next()
		}
	}
	return m, nil
}

func (m Model) options() int {
	switch m.step {
	case stepAgent:
		return len(harness.All)
	case stepPowers:
		return 2
	}
	return 1
}

func (m Model) next() (tea.Model, tea.Cmd) {
	m.err = nil
	switch m.step {
	case stepWelcome:
		m.step, m.checking = stepNetwork, true
		return m, runChecks
	case stepNetwork:
		m.step = stepHardware
	case stepHardware:
		m.step, m.cursor = stepAgent, 0
	case stepAgent:
		m.harness = harness.All[m.cursor]
		m.step = stepInstall
		if m.harness.Installed() {
			return m.login()
		}
	case stepInstall:
		if !m.harness.Installed() {
			m.busy = "install"
			return m, tea.ExecProcess(announce(m.harness.InstallCmd(), "Installing "+m.harness.Name+" with its official installer"), func(err error) tea.Msg { return execMsg{"install", err} })
		}
		return m.login()
	case stepPowers:
		m.powers = []harness.Powers{harness.PowersRoot, harness.PowersConfirm}[m.cursor]
		m.busy = "powers"
		c := exec.Command(m.self, "powers", string(m.powers))
		return m, tea.ExecProcess(announce(c, "Setting the agent's powers (sudo may ask for your password)"), func(err error) tea.Msg { return execMsg{"powers", err} })
	case stepHandoff:
		m.Result = Result{Done: true, Harness: m.harness.ID, Powers: m.powers}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) login() (tea.Model, tea.Cmd) {
	c := m.harness.LoginCmd()
	if c == nil {
		return m.afterExec(execMsg{"login", nil})
	}
	m.busy = "login"
	return m, tea.ExecProcess(announce(c, "Logging in to "+m.harness.Name+". Open the link on your phone if it shows one"), func(err error) tea.Msg { return execMsg{"login", err} })
}

func (m Model) afterExec(msg execMsg) (tea.Model, tea.Cmd) {
	m.busy = ""
	m.noLogin = false
	if msg.err != nil && msg.what == "login" {
		// cancelling a provider picker isn't fatal: most harnesses can log
		// in from inside the session too
		m.noLogin = true
		m.err = fmt.Errorf("login didn't finish (%v). enter: try again  ·  s: skip, log in later inside %s  ·  b: back", msg.err, m.harness.Name)
		return m, nil
	}
	if msg.err != nil {
		m.err = fmt.Errorf("%s failed: %v. Enter retries, b goes back", msg.what, msg.err)
		if msg.what == "install" || msg.what == "login" {
			m.step = stepInstall
		}
		return m, nil
	}
	switch msg.what {
	case "install":
		return m.login()
	case "login":
		if err := m.harness.Configure(); err != nil {
			m.err = err
			return m, nil
		}
		cfg, _ := harness.LoadConfig()
		cfg.Harness = m.harness.ID
		cfg.Save()
		m.step, m.cursor = stepPowers, 0
	case "powers":
		m.step = stepHandoff
	}
	return m, nil
}

// announce prints a header before a command takes over the terminal.
func announce(c *exec.Cmd, what string) *exec.Cmd {
	script := `printf '\033[H\033[2J\033[1;33m▸ %s\033[0m\n\n' "$1"; shift; "$@"; rc=$?; ` +
		`[ $rc -eq 0 ] || { printf '\n\033[31mexit %s\033[0m. Press enter to go back.' $rc; read -r _; }; exit $rc`
	wrapped := exec.Command("sh", append([]string{"-c", script, "sh", what, c.Path}, c.Args[1:]...)...)
	wrapped.Env, wrapped.Dir = c.Env, c.Dir
	return wrapped
}

func (m Model) View() string {
	var b strings.Builder
	// progress line
	var crumbs []string
	for i, n := range stepNames {
		switch {
		case step(i) == m.step:
			crumbs = append(crumbs, sel.Render(n))
		case step(i) < m.step:
			crumbs = append(crumbs, okStyle.Render(n))
		default:
			crumbs = append(crumbs, dim.Render(n))
		}
	}
	b.WriteString(strings.Join(crumbs, dim.Render(" › ")) + "\n\n")

	switch m.step {
	case stepWelcome:
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, title.Render(cat), "   ",
			bold.Render("Welcome to Vibian.")+"\n\n"+
				"This computer is run by a coding agent you pick.\n"+
				"It installs things, fixes things and builds your desktop,\n"+
				"and every change it makes is snapshotted and written down,\n"+
				"so any of it can be undone.") + "\n\n")
		b.WriteString(dim.Render("enter: start  ·  q: quit to a shell (run `vibian onboard` later)"))
	case stepNetwork:
		b.WriteString(bold.Render("Network") + "\n\n")
		b.WriteString(m.checkList("network", "dns"))
		if !m.checking && !m.ok("dns") {
			b.WriteString("\nThe agent needs the internet. Plug in a cable, or set up wifi with\n" +
				"`sudo nmtui` (if installed) or netplan, then press r.\n")
		}
		b.WriteString("\n" + dim.Render("enter: continue  ·  r: check again  ·  q: quit"))
	case stepHardware:
		b.WriteString(bold.Render("Hardware and system") + "\n\n")
		b.WriteString(m.checkList("boot", "secure-boot", "rootfs", "snapper", "grub-btrfs", "gpu", "battery", "disk", "units"))
		b.WriteString("\n" + dim.Render("Anything amber or red is the agent's first job.") + "\n")
		b.WriteString("\n" + dim.Render("enter: continue  ·  r: check again  ·  b: back"))
	case stepAgent:
		b.WriteString(bold.Render("Pick your agent") + "\n\n")
		for i, h := range harness.All {
			line := fmt.Sprintf("%-12s %-10s %s", h.Name, h.Vendor, dim.Render(h.Licence))
			if h.Installed() {
				line += okStyle.Render("  installed")
			}
			if i == m.cursor {
				b.WriteString(sel.Render("› ") + sel.Render(line) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
		b.WriteString("\n" + dim.Render("Proprietary agents are installed from their vendor, never shipped on the ISO.") + "\n")
		b.WriteString("\n" + dim.Render("↑/↓: choose  ·  enter: pick  ·  b: back"))
	case stepInstall:
		b.WriteString(bold.Render("Install "+m.harness.Name) + "\n\n")
		if m.busy != "" {
			b.WriteString(m.spin.View() + " " + m.busy + "…\n")
		} else {
			installed := m.harness.Installed()
			if installed {
				b.WriteString(okStyle.Render("✓ installed") + "\n")
			} else {
				b.WriteString("This runs the official installer:\n\n  " + dim.Render(m.harness.Install) + "\n")
			}
			if m.harness.LoginCmd() == nil {
				hint := m.harness.LoginHint
				if hint == "" {
					hint = "You'll log in the first time it starts."
				}
				b.WriteString("\n" + hint + "\n")
			} else {
				b.WriteString("\nThen its login (" + strings.Join(m.harness.Login, " ") + "). There's no browser yet,\nso open the link it shows on your phone.\n")
				if m.harness.LoginHint != "" {
					b.WriteString(dim.Render(m.harness.LoginHint) + "\n")
				}
			}
			if installed {
				b.WriteString("\n" + dim.Render("enter: log in  ·  b: back"))
			} else {
				b.WriteString("\n" + dim.Render("enter: install  ·  b: back"))
			}
		}
	case stepPowers:
		b.WriteString(bold.Render("How much can "+m.harness.Name+" do?") + "\n\n")
		opts := []struct{ name, desc string }{
			{"Everything (recommended)", "Passwordless sudo and no permission prompts.\nSnapshots and the ledger mean any change can be undone."},
			{"Ask first", "sudo asks for your password and the agent asks before acting."},
		}
		for i, o := range opts {
			if i == m.cursor {
				b.WriteString(sel.Render("› "+o.name) + "\n")
			} else {
				b.WriteString("  " + o.name + "\n")
			}
			b.WriteString(dim.Render(indent(o.desc, "    ")) + "\n\n")
		}
		if m.harness.NoConfirm != "" {
			b.WriteString(warnSty.Render(m.harness.NoConfirm) + "\n\n")
		}
		b.WriteString(dim.Render("Change it later with `vibian powers`.  ↑/↓  ·  enter: choose"))
	case stepHandoff:
		b.WriteString(bold.Render("All set.") + "\n\n")
		b.WriteString(fmt.Sprintf("Agent:   %s\nPowers:  %s\n\n", m.harness.Name, m.powers))
		b.WriteString("Press enter and " + m.harness.Name + " takes over. Start it again any time\nwith `vibian agent`.\n\n")
		if m.harness.LoginCmd() == nil && m.harness.LoginHint != "" {
			b.WriteString(warnSty.Render(m.harness.LoginHint) + "\n\n")
		}
		b.WriteString("Useful commands:\n\n")
		b.WriteString(dim.Render("  vibian log        what changed, and by whom\n  vibian undo --last\n  vibian doctor") + "\n\n")
		b.WriteString(dim.Render("enter: hand over  ·  b: back"))
	}
	if m.err != nil {
		b.WriteString("\n\n" + failSty.Render(m.err.Error()))
	}
	w := 76
	if m.width > 0 && m.width-4 < w {
		w = m.width - 4
	}
	return frame.Width(w).Render(b.String()) + "\n"
}

func indent(s, pre string) string {
	return pre + strings.ReplaceAll(s, "\n", "\n"+pre)
}

func (m Model) ok(id string) bool {
	for _, c := range m.checks {
		if c.ID == id {
			return c.Status == doctor.OK
		}
	}
	return false
}

func (m Model) checkList(ids ...string) string {
	if m.checking {
		return m.spin.View() + " checking…\n"
	}
	var b strings.Builder
	for _, id := range ids {
		for _, c := range m.checks {
			if c.ID != id {
				continue
			}
			mark := map[doctor.Status]string{
				doctor.OK: okStyle.Render("✓"), doctor.Warn: warnSty.Render("!"),
				doctor.Fail: failSty.Render("✗"), doctor.Info: dim.Render("·"),
			}[c.Status]
			fmt.Fprintf(&b, "%s %-16s %s\n", mark, c.Name, c.Detail)
			if c.Fix != "" && c.Status != doctor.OK {
				b.WriteString(dim.Render("    "+c.Fix) + "\n")
			}
		}
	}
	return b.String()
}

// Run starts the TUI on the terminal.
func Run() (Result, error) {
	self, err := os.Executable()
	if err != nil {
		self = "vibian"
	}
	final, err := tea.NewProgram(New(self), tea.WithAltScreen()).Run()
	if err != nil {
		return Result{}, err
	}
	return final.(Model).Result, nil
}
