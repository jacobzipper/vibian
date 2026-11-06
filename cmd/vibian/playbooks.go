package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// playbookDir is where vibian-base installs the desktop playbooks.
var playbookDir = "/usr/share/vibian/playbooks"

type playbook struct {
	ID, Name, Description, Status, Dir string
}

// loadPlaybooks reads the frontmatter of every <dir>/*/PLAYBOOK.md.
func loadPlaybooks() ([]playbook, error) {
	if d := os.Getenv("VIBIAN_PLAYBOOKS"); d != "" {
		playbookDir = d
	}
	files, _ := filepath.Glob(filepath.Join(playbookDir, "*", "PLAYBOOK.md"))
	if len(files) == 0 {
		return nil, fmt.Errorf("no playbooks in %s (is vibian-base installed?)", playbookDir)
	}
	var out []playbook
	for _, f := range files {
		p, err := readPlaybook(f)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func readPlaybook(file string) (playbook, error) {
	p := playbook{ID: filepath.Base(filepath.Dir(file)), Dir: filepath.Dir(file)}
	f, err := os.Open(file)
	if err != nil {
		return p, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || sc.Text() != "---" {
		return p, fmt.Errorf("%s: no frontmatter", file)
	}
	for sc.Scan() && sc.Text() != "---" {
		k, v, _ := strings.Cut(sc.Text(), ":")
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "name":
			p.Name = v
		case "description":
			p.Description = v
		case "status":
			p.Status = v
		}
	}
	return p, nil
}

// cmdPlaybooks is `vibian playbooks [list|show <id>]`: the gallery of
// desktops the agent can set up.
func cmdPlaybooks(args []string) error {
	books, err := loadPlaybooks()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		for _, p := range books {
			fmt.Printf("%-16s %-8s %s\n", p.ID, p.Status, p.Description)
		}
		fmt.Println("\nRead one with `vibian playbooks show <id>`.")
		return nil
	}
	if args[0] != "show" || len(args) != 2 {
		return errors.New("usage: vibian playbooks [list|show <id>]")
	}
	for _, p := range books {
		if p.ID == args[1] {
			b, err := os.ReadFile(filepath.Join(p.Dir, "PLAYBOOK.md"))
			if err != nil {
				return err
			}
			fmt.Printf("<!-- playbook %s: scripts are in %s/scripts -->\n%s", p.ID, p.Dir, b)
			return nil
		}
	}
	return fmt.Errorf("no playbook %q: see `vibian playbooks`", args[1])
}
