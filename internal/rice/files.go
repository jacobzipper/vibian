package rice

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxFile is the biggest file an export copies. Rices are text; anything
// bigger is a wallpaper pack, a font cache or a mistake.
const MaxFile = 1 << 20

// skipDirs are never worth exporting.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "__pycache__": true, ".venv": true, "venv": true, "target": true, "cache": true, "Cache": true}

// Copied is what Copy did with one source path.
type Copied struct {
	Files    []string // relative to the copy's root
	Redacted map[string]int
	Skipped  []string // "path: why"
}

// Copy copies src (a file or a directory) to dst, dropping denied paths,
// binaries and big files, and scrubbing secrets from what's left. rel is the
// path Denied sees: relative to home for dotfiles, absolute for system files.
func Copy(src, dst, rel string, c *Copied) error {
	if c.Redacted == nil {
		c.Redacted = map[string]int{}
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			c.Skipped = append(c.Skipped, fmt.Sprintf("%s: %v", p, err))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		sub, _ := filepath.Rel(src, p)
		r := filepath.Join(rel, sub)
		if Denied(r) {
			c.Skipped = append(c.Skipped, r+": private")
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != src && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		out := filepath.Join(dst, sub)
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return nil
			}
			os.MkdirAll(filepath.Dir(out), 0o755)
			os.Remove(out)
			c.Files = append(c.Files, r)
			return os.Symlink(target, out)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if leftover.MatchString(d.Name()) {
			c.Skipped = append(c.Skipped, r+": backup copy")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > MaxFile {
			c.Skipped = append(c.Skipped, r+": over 1 MiB")
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			c.Skipped = append(c.Skipped, fmt.Sprintf("%s: %v", r, err))
			return nil
		}
		if !text(b) {
			// images (wallpapers, icons) and apt keys are part of a rice;
			// other binaries aren't
			if !image(p) && !PublicKey(r) {
				c.Skipped = append(c.Skipped, r+": binary")
				return nil
			}
		} else {
			s, n := Scrub(string(b))
			if n > 0 {
				c.Redacted[r] = n
			}
			b = []byte(s)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		c.Files = append(c.Files, r)
		return os.WriteFile(out, b, info.Mode().Perm()&0o755|0o600)
	})
}

// leftover matches backup copies editors, dpkg and installers leave behind.
var leftover = regexp.MustCompile(`(~|\.(orig|save|bak|old|swp|dpkg-(old|dist|new|bak)|ucf-(old|dist|new)|distUpgrade))$`)

func text(b []byte) bool {
	if len(b) > 8192 {
		b = b[:8192]
	}
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	// a cut in the middle of a rune is fine
	for i := 0; i < 4 && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	return utf8.Valid(b)
}

func image(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".ico":
		return true
	}
	return false
}
