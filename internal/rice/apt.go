package rice

import (
	"regexp"
	"strings"
	"time"
)

var aptPkg = regexp.MustCompile(`([a-z0-9][a-z0-9+.-]+)(?::[a-z0-9]+)? \(([^)]*)\)`)

// AptHistory reads /var/log/apt/history.log text (oldest first) and returns
// the packages someone asked for after since: installed and not pulled in
// as a dependency, and not removed again later.
func AptHistory(text string, since time.Time) []string {
	want := map[string]bool{}
	var at time.Time
	for _, ln := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(ln, ": ")
		if !ok {
			continue
		}
		switch k {
		case "Start-Date":
			at, _ = time.ParseInLocation("2006-01-02  15:04:05", strings.TrimSpace(v), time.Local)
		case "Install", "Reinstall", "Remove", "Purge":
			if at.Before(since) {
				continue
			}
			for _, m := range aptPkg.FindAllStringSubmatch(v, -1) {
				switch {
				case k == "Remove" || k == "Purge":
					delete(want, m[1])
				case !strings.Contains(m[2], "automatic"):
					want[m[1]] = true
				}
			}
		}
	}
	var out []string
	for p := range want {
		out = append(out, p)
	}
	return Sorted(out)
}
