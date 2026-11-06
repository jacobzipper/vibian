package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jacobzipper/vibian/internal/doctor"
	"github.com/jacobzipper/vibian/internal/ledger"
	"github.com/jacobzipper/vibian/internal/rice"
	"github.com/jacobzipper/vibian/internal/sys"
)

// riceDotfiles are what a rice usually is: desktop, bar, launcher,
// notifications, terminal, shell, prompt and editor configs. Only the ones
// that exist are exported.
var riceDotfiles = []string{
	".config/hypr", ".config/waybar", ".config/sway", ".config/i3", ".config/niri", ".config/quickshell", ".config/eww",
	".config/xfce4", ".config/cosmic",
	".config/picom", ".config/polybar", ".config/rofi", ".config/wofi", ".config/fuzzel", ".config/walker", ".config/dunst",
	".config/mako", ".config/swaync", ".config/kitty", ".config/alacritty", ".config/foot", ".config/wezterm", ".config/ghostty",
	".config/fish", ".config/nvim", ".config/helix", ".config/zed/settings.json", ".config/fastfetch", ".config/btop",
	".config/gtk-3.0/settings.ini", ".config/gtk-4.0/settings.ini", ".config/qt5ct", ".config/qt6ct", ".config/Kvantum",
	".config/starship.toml", ".config/tmux", ".tmux.conf", ".config/systemd/user", ".config/environment.d",
	".zshrc", ".bashrc", ".config/wallpapers", ".local/share/wallpapers",
}

// keep survives a re-export: the human and agent's own writing and extras.
var riceKeep = map[string]bool{".git": true, "RICE.md": true, "README.md": true, "scripts": true, "screenshots": true}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	name := fs.String("name", "", "the rice's name (default: hostname)")
	var add listFlag
	fs.Var(&add, "add", "another file or directory to include (repeat)")
	publish := fs.Bool("publish", false, "push it to a new public GitHub repo (needs gh and gitleaks)")
	yes := fs.Bool("yes", false, "don't ask")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "vibian-rice")
	if len(pos) == 1 {
		dir, _ = filepath.Abs(pos[0])
	} else if len(pos) > 1 {
		return errors.New("usage: vibian export [DIR] [--name N] [--add PATH] [--publish] [--yes]")
	}
	if *name == "" {
		// a re-export keeps its name
		if old, err := rice.ParseManifest(sys.ReadFile(filepath.Join(dir, "vibian.yaml"))); err == nil && old.Name != "" {
			*name = old.Name
		} else {
			*name, _ = os.Hostname()
		}
	}
	if *publish && !sys.Has("gitleaks") {
		return errors.New("--publish needs gitleaks for its last secret check: sudo apt install gitleaks")
	}
	if err := prepareRiceDir(dir); err != nil {
		return err
	}
	r, err := exportRice(dir, home, *name, add)
	if err != nil {
		return err
	}
	fmt.Print(r.report(dir))
	if sys.Has("gitleaks") {
		if err := gitleaks(dir); err != nil {
			return err
		}
		fmt.Println("gitleaks: no secrets found")
	} else {
		fmt.Println("warning: gitleaks isn't installed, so only Vibian's own scrub ran. Install it before sharing: sudo apt install gitleaks")
	}
	if err := riceCommit(dir); err != nil {
		return err
	}
	fmt.Printf("\nExported to %s (a git repo). Describe the setup in RICE.md, then share it with: vibian export --publish\n", dir)
	if *publish {
		return publishRice(dir, *yes)
	}
	return nil
}

type exportReport struct {
	m       rice.Manifest
	copied  rice.Copied
	private int
}

func (r exportReport) report(dir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d apt, %d flatpak, %d snap packages; %d services; %d files\n",
		r.m.Name, len(r.m.Apt), len(r.m.Flatpak), len(r.m.Snap), len(r.m.Services)+len(r.m.UserServices), len(r.copied.Files))
	var red []string
	for f, n := range r.copied.Redacted {
		red = append(red, fmt.Sprintf("  %s (%d)", f, n))
	}
	sort.Strings(red)
	if len(red) > 0 {
		fmt.Fprintf(&b, "secrets redacted:\n%s\n", strings.Join(red, "\n"))
	}
	if r.private > 0 {
		fmt.Fprintf(&b, "left out %d private paths (keys, credentials, browser profiles, history)\n", r.private)
	}
	for _, s := range r.copied.Skipped {
		if !strings.HasSuffix(s, ": private") {
			fmt.Fprintf(&b, "skipped %s\n", s)
		}
	}
	return b.String()
}

// prepareRiceDir makes dir ready for an export: new, empty, or an earlier
// export whose generated parts get replaced.
func prepareRiceDir(dir string) error {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if len(ents) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, "vibian.yaml")); err != nil {
		return fmt.Errorf("%s isn't empty and isn't an earlier export; pick another directory", dir)
	}
	for _, e := range ents {
		if !riceKeep[e.Name()] {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func exportRice(dir, home, name string, add []string) (exportReport, error) {
	r := exportReport{m: rice.Manifest{Name: name, Vibian: version, Base: osName(), Exported: time.Now().Format("2006-01-02")}}
	entries, _ := ledger.All()

	r.m.Apt = exportedPackages(entries)
	r.m.Playbooks = desktops()
	if sys.Has("flatpak") {
		out, _ := sys.Output("flatpak", "list", "--app", "--columns=application")
		r.m.Flatpak = rice.Sorted(strings.Split(out, "\n"))
	}
	r.m.Snap = snaps()
	var units []string
	r.m.Services, units = services(false)
	r.m.UserServices, _ = services(true)
	r.m.Hardware = riceHardware()

	// what to copy: path relative to home for dotfiles, absolute for system
	var dot, system []string
	for _, d := range riceDotfiles {
		if d == ".bashrc" && sys.ReadFile(filepath.Join(home, d)) == sys.ReadFile("/etc/skel/.bashrc") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(home, d)); err == nil {
			dot = append(dot, d)
		}
	}
	system = append(system, units...)
	system = append(system, aptSources()...)
	place := func(p string, asked bool) error {
		p = strings.Trim(strings.TrimSpace(p), "`")
		if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
		if !filepath.IsAbs(p) {
			if !asked {
				return nil // a ledger note, not a path
			}
			p, _ = filepath.Abs(p)
		}
		p = filepath.Clean(p)
		if _, err := os.Lstat(p); err != nil {
			if asked {
				return err
			}
			return nil
		}
		switch {
		case strings.HasPrefix(p, home+"/"):
			dot = append(dot, strings.TrimPrefix(p, home+"/"))
		case strings.HasPrefix(p, "/etc/systemd/system/snap"):
			// snapd writes these; `snap install` brings them back
		case strings.HasPrefix(p, "/etc/") || strings.HasPrefix(p, "/usr/local/"):
			system = append(system, p)
		case asked:
			return fmt.Errorf("--add %s: only files under your home, /etc and /usr/local can go in a rice", p)
		}
		return nil
	}
	for _, e := range entries {
		for _, f := range e.Files {
			place(f, false)
		}
	}
	for _, a := range add {
		if err := place(a, true); err != nil {
			return r, err
		}
	}
	r.m.Dotfiles, r.m.System = outermost(dot), outermost(system)
	for _, d := range r.m.Dotfiles {
		if err := rice.Copy(filepath.Join(home, d), filepath.Join(dir, "dotfiles", d), d, &r.copied); err != nil {
			return r, err
		}
	}
	for _, s := range r.m.System {
		if err := rice.Copy(s, filepath.Join(dir, "system", s), s, &r.copied); err != nil {
			return r, err
		}
	}
	for _, s := range r.copied.Skipped {
		if strings.HasSuffix(s, ": private") {
			r.private++
		}
	}
	// a denied path asked for by name is dropped from the manifest too
	r.m.Dotfiles = existing(r.m.Dotfiles, filepath.Join(dir, "dotfiles"))
	r.m.System = existing(r.m.System, filepath.Join(dir, "system"))

	if b, err := os.ReadFile(filepath.Join(ledger.Dir(), "ledger.md")); err == nil {
		s, n := rice.Scrub(string(b))
		if n > 0 {
			r.copied.Redacted["ledger/ledger.md"] = n
		}
		os.MkdirAll(filepath.Join(dir, "ledger"), 0o755)
		os.WriteFile(filepath.Join(dir, "ledger", "ledger.md"), []byte(s), 0o644)
	}
	if err := os.WriteFile(filepath.Join(dir, "vibian.yaml"), []byte(r.m.YAML()), 0o644); err != nil {
		return r, err
	}
	if _, err := os.Stat(filepath.Join(dir, "RICE.md")); err != nil {
		os.WriteFile(filepath.Join(dir, "RICE.md"), []byte(riceDoc(r.m, entries)), 0o644)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		os.WriteFile(filepath.Join(dir, "README.md"), []byte(fmt.Sprintf(
			"# %s\n\nA [Vibian](https://github.com/jacobzipper/vibian) rice. What it is and how it fits together: [RICE.md](RICE.md).\n\n"+
				"```\nvibian apply github:OWNER/REPO\n```\n\nYour agent applies it and asks before overwriting your own files. `vibian apply --scripts-only` does it without an agent.\n", r.m.Name)), 0o644)
	}
	screenshot(dir)
	return r, nil
}

// outermost drops paths inside other paths in the list.
func outermost(paths []string) []string {
	paths = rice.Sorted(paths)
	var out []string
	for _, p := range paths {
		if len(out) > 0 && strings.HasPrefix(p, out[len(out)-1]+"/") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// existing keeps the paths that made it into the export.
func existing(paths []string, root string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Lstat(filepath.Join(root, p)); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func osName() string {
	for _, ln := range strings.Split(sys.ReadFile("/etc/os-release"), "\n") {
		if v, ok := strings.CutPrefix(ln, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// exportedPackages are the apt packages someone installed after the OS
// install, by hand or through the ledger, that are still installed by hand.
func exportedPackages(entries []ledger.Entry) []string {
	var since time.Time
	if fi, err := os.Stat("/var/log/installer"); err == nil {
		since = fi.ModTime()
	}
	pkgs := rice.AptHistory(aptHistory(), since)
	for _, e := range entries {
		for _, p := range e.Packages {
			if p = strings.TrimSpace(p); strings.HasPrefix(p, "-") {
				continue
			}
			pkgs = append(pkgs, strings.TrimPrefix(p, "+"))
		}
	}
	manual := map[string]bool{}
	out, err := sys.Output("apt-mark", "showmanual")
	for _, p := range strings.Split(out, "\n") {
		manual[strings.TrimSpace(p)] = true
	}
	var keep []string
	for _, p := range rice.Sorted(pkgs) {
		if (err != nil || manual[p]) && !strings.HasPrefix(p, "vibian") {
			keep = append(keep, p)
		}
	}
	return keep
}

// aptHistory is every apt history log, oldest first.
func aptHistory() string {
	logs, _ := filepath.Glob("/var/log/apt/history.log.*")
	num := func(p string) int {
		n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "history.log."), ".gz"))
		return n
	}
	sort.Slice(logs, func(i, j int) bool { return num(logs[i]) > num(logs[j]) })
	var b strings.Builder
	for _, p := range append(logs, "/var/log/apt/history.log") {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		var rd io.Reader = f
		if strings.HasSuffix(p, ".gz") {
			if z, err := gzip.NewReader(f); err == nil {
				rd = z
			}
		}
		io.Copy(&b, rd)
		f.Close()
		b.WriteString("\n")
	}
	return b.String()
}

// desktopPkgs maps a package each desktop playbook installs to the
// playbook's ID. niri isn't in the archive: its playbook builds it into
// /usr/local/bin.
var desktopPkgs = map[string]string{"hyprland": "hyprland", "sway": "sway", "gnome-shell": "gnome",
	"plasma-desktop": "kde-plasma", "i3-wm": "i3", "xfce4-session": "xfce", "cosmic-session": "cosmic"}

// desktops are the desktop playbooks the installed packages point at.
func desktops() []string {
	var pkgs []string
	for p := range desktopPkgs {
		pkgs = append(pkgs, p)
	}
	out, _ := sys.Output("dpkg-query", append([]string{"-W", "-f", "${Package} ${db:Status-Status}\n"}, pkgs...)...)
	var ids []string
	for _, ln := range strings.Split(out, "\n") {
		if p, st, ok := strings.Cut(ln, " "); ok && st == "installed" {
			ids = append(ids, desktopPkgs[p])
		}
	}
	if _, err := os.Stat("/usr/local/bin/niri"); err == nil {
		ids = append(ids, "niri")
	}
	return rice.Sorted(ids)
}

func snaps() []string {
	if !sys.Has("snap") {
		return nil
	}
	out, _ := sys.Output("snap", "list", "--color=never")
	var names []string
	for i, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if i == 0 || len(f) < 2 {
			continue
		}
		notes := f[len(f)-1]
		if strings.Contains(notes, "base") || strings.Contains(notes, "snapd") || strings.Contains(notes, "core") || f[0] == "snapd" || f[0] == "bare" || strings.HasPrefix(f[0], "gnome-") || strings.HasPrefix(f[0], "gtk-common") {
			continue
		}
		names = append(names, f[0])
	}
	return rice.Sorted(names)
}

// services are the units someone enabled that aren't on by default, and the
// unit files of custom ones, which go in system/.
func services(user bool) (names, files []string) {
	argv := []string{"list-unit-files", "--type=service,timer,socket,path", "--state=enabled", "--no-legend", "--plain"}
	dir := "/etc/systemd/system"
	if user {
		argv = append([]string{"--user"}, argv...)
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config/systemd/user")
	}
	out, _ := sys.Output("systemctl", argv...)
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		// snaps bring their own units
		if len(f) < 2 || strings.HasPrefix(f[0], "vibian") || strings.HasPrefix(f[0], "snap.") || strings.HasPrefix(f[0], "snap-") {
			continue
		}
		custom := false
		if fi, err := os.Lstat(filepath.Join(dir, f[0])); err == nil && fi.Mode().IsRegular() {
			custom = true
			files = append(files, filepath.Join(dir, f[0]))
		}
		if custom || (len(f) >= 3 && f[2] == "disabled") {
			names = append(names, f[0])
		}
	}
	return rice.Sorted(names), files
}

var signedBy = regexp.MustCompile(`(?m)(?:Signed-By:\s*|signed-by=)(/[^\s\]]+)`)

// aptSources are third-party apt sources and the keys they're signed with.
func aptSources() []string {
	var out []string
	srcs, _ := filepath.Glob("/etc/apt/sources.list.d/*")
	for _, s := range srcs {
		if b := filepath.Base(s); b == "ubuntu.sources" || !(strings.HasSuffix(b, ".sources") || strings.HasSuffix(b, ".list")) {
			continue
		}
		out = append(out, s)
		for _, m := range signedBy.FindAllStringSubmatch(sys.ReadFile(s), -1) {
			if _, err := os.Stat(m[1]); err == nil && rice.PublicKey(m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// riceHardware is what the rice was made on, so an agent applying it knows
// what to adapt.
func riceHardware() []string {
	var hw []string
	cpu := ""
	for _, ln := range strings.Split(sys.ReadFile("/proc/cpuinfo"), "\n") {
		if v, ok := strings.CutPrefix(ln, "model name"); ok {
			cpu = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), ":"))
			break
		}
	}
	if cpu != "" {
		n, _ := sys.Output("nproc")
		hw = append(hw, fmt.Sprintf("CPU: %s (%s threads)", cpu, n))
	}
	for _, ln := range strings.Split(sys.ReadFile("/proc/meminfo"), "\n") {
		if v, ok := strings.CutPrefix(ln, "MemTotal:"); ok {
			kb, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(v), " kB"))
			hw = append(hw, fmt.Sprintf("RAM: %d GiB", (kb+(1<<19))>>20))
		}
	}
	for _, g := range doctor.GPUs() {
		hw = append(hw, fmt.Sprintf("GPU: %s %s (driver: %s)", g.Vendor, g.Model, orNone(g.Driver)))
	}
	return hw
}

func screenshot(dir string) {
	shots := filepath.Join(dir, "screenshots")
	if ents, _ := os.ReadDir(shots); len(ents) > 0 || os.Getenv("WAYLAND_DISPLAY") == "" || !sys.Has("grim") {
		return
	}
	os.MkdirAll(shots, 0o755)
	sys.Output("grim", filepath.Join(shots, "desktop.png"))
}

func riceDoc(m rice.Manifest, entries []ledger.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", m.Name)
	b.WriteString("<!-- Say what this setup is for and what it looks like, in a few lines. Put screenshots in screenshots/. -->\n\n")
	b.WriteString("## Applying it (for agents)\n\n")
	b.WriteString("Follow the `export-apply` skill. In short:\n\n")
	b.WriteString("1. Read `vibian.yaml`: packages, services, and the files under `dotfiles/` (relative to home) and `system/` (absolute paths).\n")
	b.WriteString("2. `vibian snap \"apply rice " + m.Name + "\"` first.\n")
	b.WriteString("3. Install what's missing. Packages that don't exist on this release: find the replacement or say so.\n")
	b.WriteString("4. Ask before overwriting any of the user's existing files. Keep the user's own settings where they don't conflict.\n")
	b.WriteString("5. Adapt hardware-specific lines (monitor names and modes, GPU env vars, keyboard layout) to this machine: `hardware` says what it came from.\n")
	b.WriteString("6. Run `scripts/*.sh` in order only after showing the user what they do.\n")
	b.WriteString("7. `vibian log add \"apply rice " + m.Name + "\" --pkgs … --files …`.\n\n")
	if len(m.Hardware) > 0 {
		b.WriteString("## Hardware it came from\n\n")
		for _, h := range m.Hardware {
			fmt.Fprintf(&b, "- %s\n", h)
		}
		b.WriteString("\n")
	}
	var sums []string
	for _, e := range entries {
		if !strings.Contains(e.Summary, "auto-recorded") {
			s, _ := rice.Scrub(e.Summary)
			sums = append(sums, fmt.Sprintf("- %s %s", e.Time.Format("2006-01-02"), s))
		}
	}
	if len(sums) > 0 {
		b.WriteString("## How it was built\n\nFrom the ledger (full log in `ledger/ledger.md`):\n\n")
		b.WriteString(strings.Join(sums, "\n") + "\n")
	}
	return b.String()
}

// gitleaks is the last gate: a second, independent secret scanner over
// everything that would be committed.
func gitleaks(dir string) error {
	rep, err := os.CreateTemp("", "vibian-gitleaks-*.json")
	if err != nil {
		return err
	}
	rep.Close()
	defer os.Remove(rep.Name())
	c := exec.Command("gitleaks", "detect", "--no-git", "--no-banner", "--redact", "--exit-code", "1",
		"--source", dir, "--report-format", "json", "--report-path", rep.Name())
	out, err := c.CombinedOutput()
	if err == nil {
		return nil
	}
	var found []struct {
		File      string
		StartLine int
		RuleID    string
	}
	if b, rerr := os.ReadFile(rep.Name()); rerr == nil && json.Unmarshal(b, &found) == nil && len(found) > 0 {
		fmt.Println("gitleaks found possible secrets:")
		for _, f := range found {
			rel, _ := filepath.Rel(dir, f.File)
			fmt.Printf("  %s:%d (%s)\n", rel, f.StartLine, f.RuleID)
		}
		return errors.New("not committed. Remove or fix those lines (or leave the file out), then run vibian export again")
	}
	return fmt.Errorf("gitleaks: %v\n%s", err, out)
}

func riceCommit(dir string) error {
	git := func(args ...string) (string, error) {
		return sys.Output("git", append([]string{"-C", dir}, args...)...)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if _, err := git("init", "-q", "-b", "main"); err != nil {
			return err
		}
	}
	if _, err := git("add", "-A"); err != nil {
		return err
	}
	if _, err := git("diff", "--cached", "--quiet"); err == nil {
		return nil // nothing changed since the last export
	}
	var id []string
	if n, _ := sys.Output("git", "config", "user.name"); n == "" {
		id = append(id, "-c", "user.name=Vibian export")
	}
	if e, _ := sys.Output("git", "config", "user.email"); e == "" {
		id = append(id, "-c", "user.email=vibian@localhost")
	}
	_, err := git(append(id, "commit", "-q", "-m", "vibian export "+time.Now().Format("2006-01-02 15:04"))...)
	return err
}

func publishRice(dir string, yes bool) error {
	if !sys.Has("gh") {
		return errors.New("publishing needs the GitHub CLI: sudo apt install gh && gh auth login")
	}
	if remote, _ := sys.Output("git", "-C", dir, "remote", "get-url", "origin"); remote != "" {
		if !yes && !confirm("Push this export to "+remote+"?") {
			return errors.New("not pushed")
		}
		return sys.Run("git", "-C", dir, "push", "-q", "origin", "HEAD")
	}
	repo := filepath.Base(dir)
	files, _ := sys.Output("git", "-C", dir, "ls-files")
	n := len(strings.Split(files, "\n"))
	fmt.Printf("This creates a public GitHub repo %q with %d files. Look first: git -C %s show --stat\n", repo, n, dir)
	if !yes && !confirm("Publish it?") {
		return errors.New("not published")
	}
	if err := sys.Run("gh", "repo", "create", repo, "--public", "--source", dir, "--push", "--description", "A Vibian rice"); err != nil {
		return err
	}
	c := exec.Command("gh", "repo", "edit", "--add-topic", "vibian-rice")
	c.Dir = dir
	c.Run()
	c = exec.Command("gh", "repo", "view", "--json", "url", "-q", ".url")
	c.Dir = dir
	b, _ := c.Output()
	url := strings.TrimSpace(string(b))
	fmt.Printf("Published: %s\nAnyone can apply it with: vibian apply %s\n", url, strings.TrimPrefix(url, "https://github.com/"))
	return nil
}
