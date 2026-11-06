package rice

import (
	"path/filepath"
	"regexp"
	"strings"
)

// denied are paths that never leave the machine, whatever they contain:
// keys, credentials, browser profiles, password stores, VPN accounts and
// harness logins. Patterns match a path relative to home (or an absolute
// system path) by its components.
var denied = []string{
	".ssh", ".gnupg", ".password-store", ".local/share/keyrings", ".pki",
	".mozilla", ".config/google-chrome", ".config/chromium", ".config/BraveSoftware", ".config/vivaldi",
	".config/microsoft-edge", ".librewolf", ".zen", ".thunderbird",
	".aws", ".azure", ".config/gcloud", ".kube", ".docker/config.json", ".config/gh/hosts.yml",
	".netrc", ".git-credentials", ".npmrc", ".pypirc", ".cargo/credentials", ".cargo/credentials.toml",
	".claude", ".claude.json", ".codex/auth.json", ".gemini/oauth_creds.json", ".gemini/google_accounts.json",
	".cursor/cli-config.json", ".config/cursor", ".pi/agent/auth.json", ".openclaw", ".local/share/opencode",
	".config/mullvad", ".config/Mullvad VPN", ".local/state/vibian", ".config/vibian",
	".bash_history", ".zsh_history", ".python_history", ".lesshst", ".local/share/fish/fish_history",
	".local/share/recently-used.xbel", ".config/Signal", ".config/discord", ".config/Slack",
	"/etc/NetworkManager/system-connections", "/etc/shadow", "/etc/gshadow", "/etc/ssh", "/etc/wireguard",
	"/etc/mullvad-vpn", "/etc/sudoers", "/etc/sudoers.d", "/etc/ssl/private", "/etc/netplan",
}

// deniedNames match a file's base name anywhere.
var deniedNames = regexp.MustCompile(`(?i)^(\.env(\..*)?|.*\.(pem|key|p12|pfx|kdbx|keystore|jks|ovpn|gpg|asc)|id_(rsa|dsa|ecdsa|ed25519)(\.pub)?|credentials(\.json)?|secrets?(\.(json|ya?ml|toml))?|.*token.*|auth\.json|cookies(\.sqlite)?|wg\d*\.conf)$`)

// Denied reports whether a path (relative to home, or absolute) must not be
// exported.
func Denied(path string) bool {
	p := filepath.Clean(path)
	if PublicKey(p) {
		return false
	}
	for _, d := range denied {
		if p == d || strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return deniedNames.MatchString(filepath.Base(p))
}

// PublicKey reports whether a path is an apt signing key: public, binary,
// and needed for the rice's third-party sources.
func PublicKey(path string) bool {
	for _, d := range []string{"/etc/apt/keyrings/", "/etc/apt/trusted.gpg.d/", "/usr/share/keyrings/"} {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// secretRes find secrets inside files that are otherwise fine to share. A
// match is replaced, not the whole file dropped.
var secretRes = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY( BLOCK)?-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY( BLOCK)?-----`),
	regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{50,}\b`),
	regexp.MustCompile(`\bsk-(ant-|proj-)?[A-Za-z0-9_\-]{20,}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
	regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}\b`),
	regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), // JWT
	// a Mullvad account number is 16 digits, often grouped in fours
	regexp.MustCompile(`\b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?\d{4}\b`),
}

// keyAssign catches `password = …`, `api_key: …`, `export FOO_TOKEN=…`. The
// keyword has to be a whole word of the key, so `author` and `bypass` don't
// count.
var keyAssign = regexp.MustCompile(`(?i)((?:^|[\s"'{,])(?:export\s+)?(?:[A-Za-z0-9_.-]*[_.-])?(?:pass(?:word|wd|phrase)?|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|psk|auth)(?:[_.-][A-Za-z0-9_.-]*)?["']?\s*[:=]\s*)(["']?)([^\s"'#,}]{6,})`)

// Redacted is what a secret is replaced with.
const Redacted = "<redacted by vibian export>"

// Scrub replaces secrets in text. It returns the new text and how many it
// replaced.
func Scrub(text string) (string, int) {
	n := 0
	for _, re := range secretRes {
		text = re.ReplaceAllStringFunc(text, func(string) string { n++; return Redacted })
	}
	text = keyAssign.ReplaceAllStringFunc(text, func(m string) string {
		sub := keyAssign.FindStringSubmatch(m)
		if sub[3] == Redacted || strings.HasPrefix(sub[3], "$") || strings.HasPrefix(sub[3], "<") {
			return m // already redacted, or a variable reference
		}
		n++
		return sub[1] + sub[2] + Redacted
	})
	return text, n
}
