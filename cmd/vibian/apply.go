package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jacobzipper/vibian/internal/harness"
	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/rice"
	"github.com/jacobzipper/vibian/internal/snap"
	"github.com/jacobzipper/vibian/internal/sys"
)

func cmdApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	scripts := fs.Bool("scripts-only", false, "apply it without the agent: files, packages, services, then scripts/*.sh")
	dry := fs.Bool("dry-run", false, "only show what it would change")
	yes := fs.Bool("yes", false, "don't ask (with --scripts-only)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: vibian apply <github:user/repo | git url | dir> [--dry-run] [--scripts-only] [--yes]")
	}
	dir, err := fetchRice(pos[0])
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, "vibian.yaml"))
	if err != nil {
		return fmt.Errorf("%s has no vibian.yaml; is it a Vibian rice?", pos[0])
	}
	m, err := rice.ParseManifest(string(b))
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	p := planApply(dir, home, m)
	fmt.Print(p.String())
	if *dry {
		return nil
	}
	if !*scripts {
		cfg, err := harness.LoadConfig()
		if err != nil {
			return err
		}
		return runAgent(cfg, applyPrompt(dir, pos[0], p))
	}
	q := "Apply it?"
	if len(p.scripts) > 0 {
		q = "Apply it, including running its scripts as you (they can use sudo)?"
	}
	if !*yes && !confirm(q) {
		return errors.New("nothing changed")
	}
	return applyScripts(dir, home, p)
}

var ghShort = regexp.MustCompile(`^(?:github:)?([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)$`)

// fetchRice returns a local directory holding the rice, cloning or updating
// it under ~/.local/share/vibian/rices.
func fetchRice(src string) (string, error) {
	if fi, err := os.Stat(src); err == nil && fi.IsDir() {
		return filepath.Abs(src)
	}
	url := src
	if m := ghShort.FindStringSubmatch(src); m != nil {
		url = "https://github.com/" + m[1] + ".git"
	} else if !strings.Contains(src, "://") && !strings.HasPrefix(src, "git@") {
		return "", fmt.Errorf("%s: not a directory, github:user/repo or git URL", src)
	}
	home, _ := os.UserHomeDir()
	name := strings.TrimSuffix(filepath.Base(strings.TrimSuffix(url, "/")), ".git")
	if m := ghShort.FindStringSubmatch(src); m != nil {
		name = strings.ReplaceAll(m[1], "/", "-")
	}
	dir := filepath.Join(home, ".local/share/vibian/rices", name)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		if _, err := sys.Output("git", "-C", dir, "pull", "-q", "--ff-only"); err != nil {
			return "", err
		}
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if _, err := sys.Output("git", "clone", "-q", "--depth", "1", url, dir); err != nil {
		return "", err
	}
	return dir, nil
}

// applyPlan is what applying a rice would change here.
type applyPlan struct {
	m                    rice.Manifest
	apt                  []string // not installed yet
	newFiles, overwrites []string // "~/x" and "/etc/x"
	scripts              []string
}

func planApply(dir, home string, m rice.Manifest) applyPlan {
	p := applyPlan{m: m}
	if len(m.Apt) > 0 {
		out, _ := sys.Output("dpkg-query", append([]string{"-W", "-f", "${Package} ${db:Status-Status}\n"}, m.Apt...)...)
		have := map[string]bool{}
		for _, ln := range strings.Split(out, "\n") {
			if pk, st, ok := strings.Cut(ln, " "); ok && st == "installed" {
				have[pk] = true
			}
		}
		for _, pk := range m.Apt {
			if !have[pk] {
				p.apt = append(p.apt, pk)
			}
		}
	}
	walk := func(root, dst, shown string) {
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			target, label := filepath.Join(dst, rel), filepath.Join(shown, rel)
			cur, err := os.ReadFile(target)
			switch {
			case err != nil && os.IsNotExist(err):
				p.newFiles = append(p.newFiles, label)
			case d.Type()&fs.ModeSymlink != 0:
				if l, _ := os.Readlink(path); l != readlink(target) {
					p.overwrites = append(p.overwrites, label)
				}
			default:
				if b, _ := os.ReadFile(path); err != nil || string(b) != string(cur) {
					p.overwrites = append(p.overwrites, label)
				}
			}
			return nil
		})
	}
	walk(filepath.Join(dir, "dotfiles"), home, "~")
	walk(filepath.Join(dir, "system"), "/", "/")
	p.scripts, _ = filepath.Glob(filepath.Join(dir, "scripts", "*.sh"))
	return p
}

func readlink(p string) string {
	l, _ := os.Readlink(p)
	return l
}

func (p applyPlan) String() string {
	var b strings.Builder
	list := func(v []string, max int) string {
		if len(v) > max {
			return strings.Join(v[:max], ", ") + fmt.Sprintf(", … (%d more)", len(v)-max)
		}
		return strings.Join(v, ", ")
	}
	m := p.m
	fmt.Fprintf(&b, "Rice %s (exported %s from Vibian %s on %s)\n", m.Name, m.Exported, m.Vibian, m.Base)
	if len(m.Playbooks) > 0 {
		fmt.Fprintf(&b, "  desktop: %s\n", strings.Join(m.Playbooks, ", "))
	}
	fmt.Fprintf(&b, "  apt: %d to install", len(p.apt))
	if len(p.apt) > 0 {
		fmt.Fprintf(&b, " (%s)", list(p.apt, 12))
	}
	b.WriteString("\n")
	if len(m.Flatpak)+len(m.Snap) > 0 {
		fmt.Fprintf(&b, "  flatpak: %s  snap: %s\n", list(m.Flatpak, 8), list(m.Snap, 8))
	}
	fmt.Fprintf(&b, "  files: %d new, %d overwritten", len(p.newFiles), len(p.overwrites))
	if len(p.overwrites) > 0 {
		fmt.Fprintf(&b, " (%s)", list(p.overwrites, 8))
	}
	b.WriteString("\n")
	if s := append(append([]string{}, m.Services...), m.UserServices...); len(s) > 0 {
		fmt.Fprintf(&b, "  services: %s\n", list(s, 8))
	}
	for _, s := range p.scripts {
		fmt.Fprintf(&b, "  script: %s (runs as you, can use sudo)\n", filepath.Base(s))
	}
	if len(m.Hardware) > 0 {
		fmt.Fprintf(&b, "  made on: %s\n", strings.Join(m.Hardware, "; "))
	}
	return b.String()
}

func applyPrompt(dir, src string, p applyPlan) string {
	return fmt.Sprintf("Apply the Vibian rice in %s (from %s) to this machine. Read the export-apply skill first, then its RICE.md and vibian.yaml. "+
		"Here's what `vibian apply` worked out:\n\n%s\n"+
		"Ask me before overwriting files I already have or running its scripts, adapt hardware-specific settings to this machine, "+
		"and record one ledger entry when you're done.", dir, src, p.String())
}

// applyScripts applies a rice without an agent, between one snapshot pair.
func applyScripts(dir, home string, p applyPlan) error {
	m := p.m
	why := "apply rice " + m.Name
	var pre snap.Set
	var err error
	if snap.Available() {
		if pre, err = snap.Pre(why); err != nil {
			return err
		}
	} else {
		fmt.Println("warning: no snapper here, so this can't be undone with vibian undo")
	}
	var problems []string
	step := func(what string, argv ...string) {
		fmt.Println("→", what)
		if err := sys.Run(argv...); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", what, err))
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "system")); err == nil {
		step("system files", sys.AsRoot("cp", "-rT", "--no-preserve=ownership", filepath.Join(dir, "system"), "/")...)
		if _, err := os.Stat(filepath.Join(dir, "system/etc/apt")); err == nil {
			step("apt update", sys.AsRoot("apt-get", "update", "-q")...)
		}
	}
	var avail, missing []string
	for _, pk := range p.apt {
		if _, err := sys.Output("apt-cache", "show", "--no-all-versions", pk); err == nil {
			avail = append(avail, pk)
		} else {
			missing = append(missing, pk)
		}
	}
	if len(missing) > 0 {
		problems = append(problems, "not in this machine's apt sources: "+strings.Join(missing, ", "))
	}
	if len(avail) > 0 {
		step("apt install", sys.AsRoot("apt-get", append([]string{"install", "-y", "-q"}, avail...)...)...)
	}
	if len(m.Flatpak) > 0 {
		if sys.Has("flatpak") {
			sys.Output("flatpak", "remote-add", "--user", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
			step("flatpak install", append([]string{"flatpak", "install", "--user", "-y", "--noninteractive", "flathub"}, m.Flatpak...)...)
		} else {
			problems = append(problems, "flatpak isn't installed: skipped "+strings.Join(m.Flatpak, ", "))
		}
	}
	for _, s := range m.Snap {
		step("snap install "+s, sys.AsRoot("snap", "install", s)...)
	}
	if _, err := os.Stat(filepath.Join(dir, "dotfiles")); err == nil {
		step("dotfiles", "cp", "-rT", filepath.Join(dir, "dotfiles"), home)
	}
	if len(m.Services) > 0 {
		sys.RootOutput("systemctl", "daemon-reload")
		step("services", sys.AsRoot("systemctl", append([]string{"enable", "--now"}, m.Services...)...)...)
	}
	if len(m.UserServices) > 0 {
		sys.Output("systemctl", "--user", "daemon-reload")
		step("user services", append([]string{"systemctl", "--user", "enable", "--now"}, m.UserServices...)...)
	}
	for _, s := range p.scripts {
		fmt.Println("→", filepath.Base(s))
		c := exec.Command("bash", s)
		c.Dir = dir
		c.Env = append(os.Environ(), "RICE_DIR="+dir)
		if err := run(c); err != nil {
			problems = append(problems, fmt.Sprintf("scripts/%s: %v", filepath.Base(s), err))
		}
	}

	e := ledger.Entry{Agent: agentName(), Summary: why + " (vibian apply --scripts-only)", Gotchas: problems}
	for _, pk := range avail {
		e.Packages = append(e.Packages, "+"+pk)
	}
	for _, d := range m.Dotfiles {
		e.Files = append(e.Files, "~/"+d)
	}
	e.Files = append(e.Files, m.System...)
	if pre != nil {
		post, err := snap.Post(pre, why)
		if err != nil {
			return err
		}
		e.Snapshots = snap.Pair(pre, post)
	}
	e, err = ledger.Add(e)
	if err != nil {
		return err
	}
	for _, pr := range problems {
		fmt.Println("problem:", pr)
	}
	fmt.Printf("Applied %s, recorded as %s.", m.Name, e.ID)
	if e.Snapshots != "" {
		fmt.Printf(" `vibian undo %s` takes it back.", e.ID)
	}
	fmt.Println("\nLog out and back in (or reboot) to start the new desktop.")
	return nil
}
