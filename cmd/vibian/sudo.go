package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/scoped"
	"github.com/jacobzipper/vibian/internal/snap"
	"github.com/jacobzipper/vibian/internal/sys"
)

// exitCode ends the process with a code once main has printed nothing.
type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit %d", int(e)) }

// needsOK is the exit code of a risky command waiting for the owner.
const needsOK = 3

// selfBin is the path the scoped sudoers rule names.
func selfBin() string {
	if _, err := os.Stat("/usr/bin/vibian"); err == nil {
		return "/usr/bin/vibian"
	}
	exe, _ := os.Executable()
	return exe
}

// cmdSudo is `vibian sudo --why "reason" -- cmd args…`: the agent's way to
// run one command as root under powers=scoped.
func cmdSudo(args []string) error {
	why, agent := "", os.Getenv("VIBIAN_AGENT")
	for len(args) > 0 {
		a := args[0]
		switch {
		case a == "--":
			args = args[1:]
			goto done
		case a == "--why" && len(args) > 1:
			why, args = args[1], args[2:]
		case strings.HasPrefix(a, "--why="):
			why, args = strings.TrimPrefix(a, "--why="), args[1:]
		case a == "--agent" && len(args) > 1:
			agent, args = args[1], args[2:]
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %s (usage: vibian sudo --why \"reason\" -- command args…)", a)
		default:
			goto done
		}
	}
done:
	if len(args) == 0 {
		return errors.New(`usage: vibian sudo --why "reason" -- command args…`)
	}
	if strings.TrimSpace(why) == "" {
		return errors.New(`say why: vibian sudo --why "install the GPU driver" -- apt-get install …`)
	}
	if agent == "" {
		agent = "agent"
	}
	if os.Geteuid() == 0 {
		return errors.New("vibian sudo is for the agent's user; you're already root")
	}
	bin := selfBin()
	root := append([]string{bin, "sudo-root", "--why", why, "--agent", agent, "--"}, args...)
	if exec.Command("sudo", append([]string{"-n", "-l"}, root...)...).Run() != nil {
		return errors.New("vibian sudo isn't set up: it works when the agent's powers are `scoped` (check with `vibian powers`). Under `confirm`, ask the owner to run the command")
	}
	cmd := exec.Command("sudo", append([]string{"-n"}, root...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return exitCode(ee.ExitCode())
		}
		return err
	}
	return nil
}

// cmdSudoRoot runs as root through the scoped sudoers rule, so it trusts
// nothing but its exact argument shape:
// sudo-root --why R --agent A -- cmd args…
func cmdSudoRoot(args []string) error {
	if os.Geteuid() != 0 || os.Getenv("SUDO_USER") == "" {
		return errors.New("sudo-root is run by `vibian sudo`")
	}
	if len(args) < 6 || args[0] != "--why" || args[2] != "--agent" || args[4] != "--" {
		return errors.New(`usage: vibian sudo --why "reason" -- command args…`)
	}
	why, agent, argv := args[1], args[3], args[5:]
	name := os.Getenv("SUDO_USER")
	u, err := user.Lookup(name)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("%s: not found", argv[0])
	}
	path, _ = filepath.Abs(path)
	risks := scoped.Risks(argv, cwd, u.HomeDir, removedPackages)
	if real, err := filepath.EvalSymlinks(path); err == nil && filepath.Base(real) != filepath.Base(argv[0]) {
		// `sh` is dash, `awk` is mawk: check the real program too
		for _, r := range scoped.Risks(append([]string{real}, argv[1:]...), cwd, u.HomeDir, removedPackages) {
			if !slices.Contains(risks, r) {
				risks = append(risks, r)
			}
		}
	}
	if !systemProgram(path) {
		risks = append(risks, path+" isn't a system program (root-owned, in /usr or /bin)")
	}

	id := scoped.RequestID(name, cwd, argv)
	approved := false
	if len(risks) > 0 {
		if !scoped.Take(id) {
			r := scoped.Request{ID: id, User: name, Agent: agent, Why: why, Argv: argv, Cwd: cwd, Risks: risks, Asked: time.Now()}
			if err := scoped.Ask(r); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "vibian sudo: this needs the owner's OK first (request %s):\n", id)
			for _, r := range risks {
				fmt.Fprintf(os.Stderr, "  - %s\n", r)
			}
			fmt.Fprintf(os.Stderr, "Ask them to run `sudo vibian approve %s` in their own terminal, then run this same command again.\n", id)
			return exitCode(needsOK)
		}
		approved = true
	}

	var pre snap.Set
	if approved && snap.Available() {
		pre, _ = snap.Pre("vibian sudo: " + why)
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Args[0] = argv[0]
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			return runErr
		}
		code = ee.ExitCode()
	}

	e := ledger.Entry{Agent: agent, Summary: why, Command: shellJoin(argv), Result: fmt.Sprintf("exit %d", code)}
	if approved {
		e.Result += ", approved by the owner (" + strings.Join(risks, "; ") + ")"
		if len(pre) > 0 {
			if post, err := snap.Post(pre, "vibian sudo: "+why); err == nil {
				e.Snapshots = snap.Pair(pre, post)
			}
		}
	}
	if e, err := ledger.Add(e); err != nil {
		fmt.Fprintf(os.Stderr, "vibian sudo: couldn't write the ledger: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "vibian sudo: logged as %s\n", e.ID)
	}
	if code != 0 {
		return exitCode(code)
	}
	return nil
}

// systemProgram reports whether path is root-owned and in a system bin dir.
func systemProgram(path string) bool {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	ok := false
	for _, d := range []string{"/usr/bin/", "/usr/sbin/", "/bin/", "/sbin/", "/usr/local/bin/", "/usr/local/sbin/", "/usr/lib/", "/usr/libexec/"} {
		if strings.HasPrefix(real, d) {
			ok = true
		}
	}
	return ok && sys.RootOwned(real)
}

// removedPackages asks apt what a removal would take with it, and which of
// those packages are core.
func removedPackages(argv []string) (map[string]bool, error) {
	var pkgs []string
	if filepath.Base(argv[0]) == "dpkg" {
		for _, a := range argv[1:] {
			if !strings.HasPrefix(a, "-") {
				pkgs = append(pkgs, strings.SplitN(a, ":", 2)[0])
			}
		}
	} else {
		sim := []string{"-s"}
		for _, a := range argv[1:] {
			if a != "-y" && a != "--yes" && a != "-q" {
				sim = append(sim, a)
			}
		}
		out, err := sys.Output("apt-get", sim...)
		if err != nil {
			return nil, err
		}
		for _, ln := range strings.Split(out, "\n") {
			if f := strings.Fields(ln); len(f) > 1 && (f[0] == "Remv" || f[0] == "Purg") {
				pkgs = append(pkgs, strings.SplitN(f[1], ":", 2)[0])
			}
		}
	}
	m := map[string]bool{}
	if len(pkgs) == 0 {
		return m, nil
	}
	out, _ := sys.Output("dpkg-query", append([]string{"-W", "-f", "${Package} ${Essential} ${Priority}\n"}, pkgs...)...)
	info := map[string]string{}
	for _, ln := range strings.Split(out, "\n") {
		if p, rest, ok := strings.Cut(ln, " "); ok {
			info[p] = rest
		}
	}
	for _, p := range pkgs {
		f := strings.Fields(info[p])
		m[p] = slices.Contains(scoped.CorePackages, p) || strings.HasPrefix(p, "linux-image") ||
			(len(f) > 0 && f[0] == "yes") || (len(f) > 1 && (f[1] == "required" || f[1] == "important"))
	}
	return m, nil
}

func shellJoin(argv []string) string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if a != "" && !strings.ContainsAny(a, " \t\n'\"\\$`;&|<>(){}[]*?!~#") {
			out[i] = a
		} else {
			out[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(out, " ")
}

// cmdApprove is the owner's side: list requests, approve or deny one. It
// needs the owner's own sudo, which the agent doesn't have.
func cmdApprove(args []string) error {
	if os.Geteuid() != 0 {
		return sys.Run(append([]string{"sudo", selfBin(), "approve"}, args...)...)
	}
	deny, yes := false, false
	var ids []string
	for _, a := range args {
		switch a {
		case "--deny", "-d":
			deny = true
		case "--yes", "-y":
			yes = true
		default:
			ids = append(ids, a)
		}
	}
	if len(ids) == 0 {
		reqs, err := scoped.Pending()
		if err != nil {
			return err
		}
		if len(reqs) == 0 {
			fmt.Println("Nothing is waiting for your OK.")
			return nil
		}
		for _, r := range reqs {
			show(r)
		}
		fmt.Println("Approve one with `sudo vibian approve <id>`, or drop it with `--deny`.")
		return nil
	}
	for _, id := range ids {
		if deny {
			if err := scoped.Deny(id); err != nil {
				return err
			}
			fmt.Println("denied", id)
			continue
		}
		r, err := scoped.Load(id)
		if err != nil {
			return err
		}
		show(r)
		if !yes && !confirm("Let the agent run this as root once, within 30 minutes?") {
			fmt.Println("not approved")
			continue
		}
		if _, err := scoped.Approve(id); err != nil {
			return err
		}
		fmt.Printf("approved %s: the agent can run it now\n", id)
	}
	return nil
}

func show(r scoped.Request) {
	fmt.Printf("%s  %s, %s ago, in %s\n", r.ID, r.Agent, time.Since(r.Asked).Round(time.Second), r.Cwd)
	fmt.Printf("  run:  %s\n  why:  %s\n", shellJoin(r.Argv), r.Why)
	for _, k := range r.Risks {
		fmt.Printf("  risk: %s\n", k)
	}
	fmt.Println()
}
