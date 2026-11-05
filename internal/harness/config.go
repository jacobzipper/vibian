package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Powers is the agent's privilege mode.
type Powers string

const (
	// PowersRoot: passwordless sudo and the harness's no-prompts mode.
	// Snapshots and the ledger are the safety net.
	PowersRoot Powers = "root"
	// PowersConfirm: sudo asks for your password; the harness asks before acting.
	PowersConfirm Powers = "confirm"
)

// Config is the user's agent choice, in ~/.config/vibian/agent.json.
type Config struct {
	Harness string `json:"harness"`
	Powers  Powers `json:"powers"`
}

func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "vibian", "agent.json")
}

// LoadConfig reads the agent config. A missing file is not an error.
func LoadConfig() (Config, error) {
	c := Config{Powers: PowersRoot}
	b, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// Save writes the agent config.
func (c Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(configPath(), append(b, '\n'), 0o644)
}
