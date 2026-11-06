package rice

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestManifestRoundTrip(t *testing.T) {
	m := Manifest{
		Name: "nhk", Vibian: "0.5.0", Base: "Ubuntu 26.04", Exported: "2026-11-06",
		Playbooks: []string{"hyprland"}, Apt: []string{"hyprland", "waybar"},
		Services: []string{"ollama.service"}, Dotfiles: []string{".config/hypr", ".zshrc"},
		Hardware: []string{"GPU: NVIDIA Corporation GA104 [GeForce RTX 3070]", "CPU: AMD Ryzen 7 5800H: 16 threads"},
	}
	got, err := ParseManifest(m.YAML())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Fatalf("round trip:\n%s\n%+v", m.YAML(), got)
	}
	if _, err := ParseManifest("apt: hyprland\n"); err == nil {
		t.Error("scalar for a list parsed")
	}
	got, err = ParseManifest("name: x\nfuture_key:\n  - a\napt:\n  - 'it''s'\n")
	if err != nil || got.Name != "x" || len(got.Apt) != 1 || got.Apt[0] != "it's" {
		t.Errorf("lenient parse: %+v %v", got, err)
	}
}

func TestDenied(t *testing.T) {
	for _, p := range []string{".ssh/id_ed25519", ".ssh", ".config/gh/hosts.yml", ".env", "proj/.env.local",
		".config/hypr/secret.json", "/etc/NetworkManager/system-connections/home.nmconnection", ".mozilla/firefox/x/cookies.sqlite",
		".codex/auth.json", ".local/share/fish/fish_history", "/etc/wireguard/wg0.conf", "keys/server.pem", ".config/mullvad/account"} {
		if !Denied(p) {
			t.Errorf("%s not denied", p)
		}
	}
	for _, p := range []string{".config/hypr/hyprland.conf", ".zshrc", ".config/waybar/config.jsonc", "/etc/default/grub", ".config/kitty/kitty.conf"} {
		if Denied(p) {
			t.Errorf("%s denied", p)
		}
	}
}

func TestScrub(t *testing.T) {
	keep := []string{
		"bind = SUPER, Return, exec, kitty",
		`author = "Jacob"`,
		"bypass = true",
		"export PATH=$HOME/.local/bin:$PATH",
		"export OPENAI_API_KEY=$(pass show openai)",
		"password_command = pass",
	}
	for _, s := range keep {
		if got, n := Scrub(s); n != 0 || got != s {
			t.Errorf("changed %q → %q", s, got)
		}
	}
	gone := map[string]string{
		"export GITHUB_TOKEN=ghp_abcdefghijklmnopqrstuvwxyz0123456789AB":                     "ghp_",
		`"api_key": "abc123def456ghi"`:                                                       "abc123",
		"password = hunter2hunter2":                                                          "hunter2",
		"ANTHROPIC_API_KEY=sk-ant-api03-aaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                       "sk-ant",
		"account 1234 5678 9012 3456":                                                        "5678",
		"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaA\n-----END OPENSSH PRIVATE KEY-----": "b3Bl",
		"slack = xoxb-1234567890-abcdefghij":                                                 "xoxb",
	}
	for s, frag := range gone {
		got, n := Scrub(s)
		if n == 0 || strings.Contains(got, frag) || !strings.Contains(got, Redacted) {
			t.Errorf("Scrub(%q) = %q (%d)", s, got, n)
		}
		if again, m := Scrub(got); m != 0 || again != got {
			t.Errorf("Scrub not idempotent on %q: %q", got, again)
		}
	}
}

func TestCopy(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	w := func(rel, s string) {
		p := filepath.Join(src, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	w("hypr/hyprland.conf", "bind = SUPER, Q, exec, kitty\n")
	w("hypr/env.conf", "env = OPENAI_API_KEY,x\nexport HF_TOKEN=hf_abcdefghijklmnopqrstuvwxyz012345\n")
	w("hypr/.env", "SECRET=1\n")
	w("hypr/node_modules/x/index.js", "x\n")
	w("hypr/blob.bin", "a\x00b")
	w("hypr/wall.png", "\x89PNG\x00")
	w("hypr/hyprland.conf~", "old\n")
	w("hypr/hyprland.conf.orig", "old\n")
	os.WriteFile(filepath.Join(src, "hypr/big.conf"), make([]byte, MaxFile+1), 0o644)
	os.Symlink("hyprland.conf", filepath.Join(src, "hypr/link.conf"))

	var c Copied
	if err := Copy(filepath.Join(src, "hypr"), filepath.Join(dst, "hypr"), ".config/hypr", &c); err != nil {
		t.Fatal(err)
	}
	want := []string{".config/hypr/env.conf", ".config/hypr/hyprland.conf", ".config/hypr/link.conf", ".config/hypr/wall.png"}
	if got := Sorted(c.Files); !reflect.DeepEqual(got, want) {
		t.Errorf("files %v", got)
	}
	if c.Redacted[".config/hypr/env.conf"] != 1 {
		t.Errorf("redacted %v", c.Redacted)
	}
	b, _ := os.ReadFile(filepath.Join(dst, "hypr/env.conf"))
	if strings.Contains(string(b), "hf_abc") {
		t.Errorf("secret copied:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(dst, "hypr/.env")); err == nil {
		t.Error(".env copied")
	}
}

func TestAptHistory(t *testing.T) {
	log := `
Start-Date: 2026-10-01  09:00:00
Commandline: curtin in-target -- apt-get install ubuntu-server-minimal
Install: ubuntu-server-minimal:amd64 (1.5), libc6:amd64 (2.42, automatic)
End-Date: 2026-10-01  09:05:00

Start-Date: 2026-11-01  10:00:00
Commandline: apt install hyprland waybar htop
Requested-By: vibian (1000)
Install: hyprland:amd64 (0.45-1), libhyprlang2:amd64 (0.6, automatic), waybar:amd64 (0.12), htop:amd64 (3.4)
End-Date: 2026-11-01  10:01:00

Start-Date: 2026-11-02  10:00:00
Commandline: apt remove htop
Remove: htop:amd64 (3.4)
End-Date: 2026-11-02  10:00:05
`
	since := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	if got := AptHistory(log, since); !reflect.DeepEqual(got, []string{"hyprland", "waybar"}) {
		t.Errorf("got %v", got)
	}
}
