// Package rice is a whole machine setup as a git repo: what `vibian export`
// writes and `vibian apply` replays. It holds the manifest format, the secret
// scrub every exported file goes through, and the file copying.
package rice

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
)

// Manifest is vibian.yaml.
type Manifest struct {
	Name     string
	Vibian   string // vibian version that exported it
	Base     string // "Ubuntu 26.04"
	Exported string // date
	// Desktop is the playbook ids the setup builds on.
	Playbooks []string
	Apt       []string
	Flatpak   []string
	Snap      []string
	// Services are enabled system units; UserServices are `systemctl --user` ones.
	Services     []string
	UserServices []string
	// Dotfiles are paths relative to home, under dotfiles/; System are
	// absolute paths, under system/.
	Dotfiles []string
	System   []string
	Hardware []string
}

func (m Manifest) scalars() [][2]string {
	return [][2]string{{"name", m.Name}, {"vibian", m.Vibian}, {"base", m.Base}, {"exported", m.Exported}}
}

func (m *Manifest) lists() []struct {
	key string
	v   *[]string
} {
	return []struct {
		key string
		v   *[]string
	}{
		{"playbooks", &m.Playbooks}, {"apt", &m.Apt}, {"flatpak", &m.Flatpak}, {"snap", &m.Snap},
		{"services", &m.Services}, {"user_services", &m.UserServices},
		{"dotfiles", &m.Dotfiles}, {"system", &m.System}, {"hardware", &m.Hardware},
	}
}

// YAML renders the manifest. It only uses scalars and lists of strings, so
// ParseManifest can read it back without a YAML library.
func (m Manifest) YAML() string {
	var b strings.Builder
	b.WriteString("# A Vibian rice: `vibian apply` hands this and RICE.md to your agent.\n")
	for _, kv := range m.scalars() {
		fmt.Fprintf(&b, "%s: %s\n", kv[0], quote(kv[1]))
	}
	for _, l := range m.lists() {
		if len(*l.v) == 0 {
			fmt.Fprintf(&b, "%s: []\n", l.key)
			continue
		}
		fmt.Fprintf(&b, "%s:\n", l.key)
		for _, v := range *l.v {
			fmt.Fprintf(&b, "  - %s\n", quote(v))
		}
	}
	return b.String()
}

func quote(s string) string {
	if s == "" || strings.ContainsAny(s, ":#'\"[]{},&*!|>%@`") || strings.TrimSpace(s) != s {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var out string
		if _, err := fmt.Sscanf(s, "%q", &out); err == nil {
			return out
		}
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return s
}

// ParseManifest reads vibian.yaml. Unknown keys are ignored, so newer
// exports still apply.
func ParseManifest(text string) (Manifest, error) {
	var m Manifest
	sc := map[string]*string{"name": &m.Name, "vibian": &m.Vibian, "base": &m.Base, "exported": &m.Exported}
	ls := map[string]*[]string{}
	for _, l := range m.lists() {
		ls[l.key] = l.v
	}
	var cur *[]string
	s := bufio.NewScanner(strings.NewReader(text))
	for n := 1; s.Scan(); n++ {
		line := s.Text()
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "- ") || t == "-" {
			if cur == nil {
				return m, fmt.Errorf("vibian.yaml:%d: list item outside a list", n)
			}
			*cur = append(*cur, unquote(strings.TrimPrefix(t, "-")))
			continue
		}
		if line != t {
			return m, fmt.Errorf("vibian.yaml:%d: unexpected indentation", n)
		}
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			return m, fmt.Errorf("vibian.yaml:%d: expected key: value", n)
		}
		v = strings.TrimSpace(v)
		var unknown []string
		cur = &unknown
		if p, ok := sc[k]; ok {
			cur = nil
			*p = unquote(v)
		} else if p, ok := ls[k]; ok {
			if v != "" && v != "[]" {
				return m, fmt.Errorf("vibian.yaml:%d: %s is a list", n, k)
			}
			cur = p
		}
	}
	return m, s.Err()
}

// Sorted returns a sorted copy without duplicates or blanks.
func Sorted(v []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
