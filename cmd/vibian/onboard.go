package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jacobzipper/vibian/internal/harness"
	"github.com/jacobzipper/vibian/internal/onboard"
	"github.com/jacobzipper/vibian/internal/sys"
)

// autologin is the tty1 drop-in the installer writes; onboarding removes it.
const autologin = "/etc/systemd/system/getty@tty1.service.d/vibian-onboard.conf"

func onboardedMarker() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "vibian", "onboarded")
}

func cmdOnboard(args []string) error {
	if len(args) > 0 && args[0] == "--if-needed" {
		// called from the tty1 login profile
		if _, err := os.Stat(onboardedMarker()); err == nil {
			return nil
		}
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("run onboarding as your own user, not root")
	}
	res, err := onboard.Run()
	if err != nil {
		return err
	}
	if !res.Done {
		fmt.Println("Onboarding paused. Pick up where you left off with: vibian onboard")
		return nil
	}
	os.MkdirAll(filepath.Dir(onboardedMarker()), 0o755)
	os.WriteFile(onboardedMarker(), nil, 0o644)
	if _, err := os.Stat(autologin); err == nil {
		if _, err := sys.RootOutput("rm", "-f", autologin); err != nil {
			fmt.Fprintln(os.Stderr, "vibian: couldn't turn off tty1 autologin:", err)
		}
	}
	cfg, err := harness.LoadConfig()
	if err != nil {
		return err
	}
	return runAgent(cfg, FirstPrompt)
}
