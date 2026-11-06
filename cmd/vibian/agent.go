package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/jacobzipper/vibian/internal/harness"
	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/snap"
	"github.com/jacobzipper/vibian/internal/sys"
)

// FirstPrompt is what the agent hears when onboarding hands over.
const FirstPrompt = `Hi! I just installed Vibian and picked you as this machine's agent. ` +
	`Introduce yourself in two sentences, run ` + "`vibian doctor`" + `, and tell me what you found ` +
	`and what you'd fix. Then ask what I'd like this computer to be.`

func cmdAgent(args []string) error {
	sub := "run"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	cfg, err := harness.LoadConfig()
	if err != nil {
		return err
	}
	switch sub {
	case "list":
		for _, h := range harness.All {
			mark := " "
			if h.ID == cfg.Harness {
				mark = "*"
			}
			state := "not installed"
			if h.Installed() {
				state = "installed"
			}
			fmt.Printf("%s %-9s %-12s %-11s %s\n", mark, h.ID, h.Name, h.Licence, state)
		}
	case "set":
		if len(args) != 1 {
			return errors.New("usage: vibian agent set <harness>")
		}
		h, err := harness.Get(args[0])
		if err != nil {
			return err
		}
		if !h.Installed() {
			fmt.Printf("Installing %s with its official installer…\n", h.Name)
			if err := run(h.InstallCmd()); err != nil {
				return err
			}
		}
		if err := h.Configure(); err != nil {
			return err
		}
		cfg.Harness = h.ID
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Your agent is now %s. Start it with: vibian agent\n", h.Name)
	case "login":
		h, err := harness.Get(cfg.Harness)
		if err != nil {
			return err
		}
		if c := h.LoginCmd(); c != nil {
			return run(c)
		}
		fmt.Printf("%s logs in the first time you start it: vibian agent\n", h.Name)
	case "run":
		fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
		prompt := fs.String("prompt", "", "first message")
		if _, err := parse(fs, args); err != nil {
			return err
		}
		return runAgent(cfg, *prompt)
	default:
		return fmt.Errorf("unknown: vibian agent %s", sub)
	}
	return nil
}

func run(c *exec.Cmd) error {
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

// runAgent launches the harness between a pre and post snapshot. If the
// session changed files but nobody wrote a ledger entry, it writes a stub.
func runAgent(cfg harness.Config, prompt string) error {
	if cfg.Harness == "" {
		return errors.New("no agent picked yet: run `vibian onboard` or `vibian agent set <harness>`")
	}
	if os.Geteuid() == 0 {
		return errors.New("run your agent as your own user, not root (it gets sudo)")
	}
	h, err := harness.Get(cfg.Harness)
	if err != nil {
		return err
	}
	bin, ok := h.Path()
	if !ok {
		return fmt.Errorf("%s isn't installed: vibian agent set %s", h.Name, h.ID)
	}
	if err := h.Configure(); err != nil {
		fmt.Fprintln(os.Stderr, "vibian: couldn't update the agent's config:", err)
	}

	if h.PowersCmd != nil && cfg.Applied[h.ID] != cfg.Powers {
		c := exec.Command(bin, h.PowersCmd(cfg.Powers)...)
		c.Env = harness.Env()
		if out, err := c.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "vibian: couldn't set %s's approvals to %s: %v\n%s", h.Name, cfg.Powers, err, out)
		} else {
			if cfg.Applied == nil {
				cfg.Applied = map[string]harness.Powers{}
			}
			cfg.Applied[h.ID] = cfg.Powers
			cfg.Save()
		}
	}

	argv, env := h.LaunchArgs(cfg.Powers)
	if prompt != "" && h.PromptArgs != nil {
		argv = append(argv, h.PromptArgs(prompt)...)
	}

	before, _ := ledger.All()
	var pre snap.Set
	if snap.Available() {
		pre, err = snap.Pre("agent session: " + h.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vibian: no session snapshot:", err)
		}
	}

	home, _ := os.UserHomeDir()
	c := exec.Command(bin, argv...)
	c.Dir = home
	c.Env = harness.Env(append(env, "VIBIAN_AGENT="+h.ID)...)
	runErr := run(c)

	if pre != nil {
		endTurns(h)
		post, err := snap.Post(pre, "agent session: "+h.Name)
		if err == nil && changed(pre, post, sessionNoise(h, home)) {
			after, _ := ledger.All()
			if len(after) == len(before) {
				e, _ := ledger.Add(ledger.Entry{
					Agent:     h.ID,
					Summary:   "session ended without a ledger entry (auto-recorded)",
					Snapshots: snap.Pair(pre, post),
					Gotchas:   []string{"written by `vibian agent`: see `snapper -c root status` on the pair for what changed"},
				})
				fmt.Printf("\nvibian: the session changed files without a ledger entry; recorded it as %s.\n", e.ID)
			}
		}
	}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) {
		return nil // the harness's own exit status isn't ours to report
	}
	return runErr
}

// changed reports whether anything outside noise differs between the pairs.
func changed(pre, post snap.Set, noise []string) bool {
	for _, c := range snap.Configs {
		a, okA := pre[c]
		b, okB := post[c]
		if !okA || !okB {
			continue
		}
		out, err := sys.Output("snapper", "-c", c, "status", fmt.Sprintf("%d..%d", a, b))
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(out, "\n") {
			// "c..... /path"
			if _, path, ok := strings.Cut(strings.TrimSpace(ln), " "); ok && !under(path, noise) {
				return true
			}
		}
	}
	return false
}

// sessionNoise is what every session changes without anyone asking: the
// harness's own state, Vibian's, caches, systemd's private tmp dirs, and the GRUB
// snapshot menu grub-btrfsd rewrites when the session snapshot appears.
func sessionNoise(h harness.Harness, home string) []string {
	noise := []string{"/tmp", "/var/tmp", filepath.Join(home, ".cache"), filepath.Join(home, ".config/vibian"), filepath.Join(home, ".local/state/vibian"),
		"/boot/grub/grub-btrfs.cfg", "/boot/grub/grub-btrfs.cfg.bkp", "/boot/grub/grub-btrfs.new"}
	for _, d := range h.StateDirs {
		noise = append(noise, filepath.Join(home, d))
	}
	return noise
}

// under reports whether path is in one of dirs, or is a parent of one: the
// first session creates ~/.local/state just to hold Vibian's state, and
// anything else put there shows up as its own path.
func under(path string, dirs []string) bool {
	for _, d := range dirs {
		if path == d || strings.HasPrefix(path, d+"/") || strings.HasPrefix(d, path+"/") {
			return true
		}
	}
	return false
}

func cmdPowers(args []string) error {
	cfg, err := harness.LoadConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Println(cfg.Powers)
		return nil
	}
	p := harness.Powers(args[0])
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if os.Geteuid() == 0 {
		// `sudo vibian powers root` means the user who ran sudo
		if name = os.Getenv("SUDO_USER"); name == "" {
			return errors.New("run this as the agent's user, not root")
		}
		if err := applyPowers(name, p); err != nil {
			return err
		}
		fmt.Println("powers:", p, "for", name)
		return nil
	}
	if err := applyPowers(name, p); err != nil {
		return err
	}
	cfg.Powers = p
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Println("powers:", p)
	if h, err := harness.Get(cfg.Harness); err == nil && p == harness.PowersConfirm && h.NoConfirm != "" {
		fmt.Println(h.NoConfirm)
	}
	return nil
}

// applyPowers writes or removes the agent's sudoers drop-in.
func applyPowers(username string, p harness.Powers) error {
	const file = "/etc/sudoers.d/vibian-agent"
	switch p {
	case harness.PowersRoot:
		rule := fmt.Sprintf("# vibian powers=root: snapshots and the ledger are the safety net\n%s ALL=(ALL) NOPASSWD: ALL\n", username)
		tmp, err := os.CreateTemp("", "vibian-sudoers")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		tmp.WriteString(rule)
		tmp.Close()
		if _, err := sys.Output("visudo", "-cqf", tmp.Name()); err != nil {
			return err
		}
		_, err = sys.RootOutput("install", "-m", "0440", "-o", "root", "-g", "root", tmp.Name(), file)
		return err
	case harness.PowersConfirm:
		_, err := sys.RootOutput("rm", "-f", file)
		return err
	}
	return fmt.Errorf("unknown powers %q (root or confirm; scoped comes in M2)", p)
}
