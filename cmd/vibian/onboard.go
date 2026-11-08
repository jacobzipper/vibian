package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
		waitFirstBoot()
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
	// the powers step leaves sudo's prompt on screen; start the agent on a clean one
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Print("\033[H\033[2J")
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

// waitFirstBoot holds onboarding until cloud-init's first boot is over. It
// prints the host keys on the console a few seconds after the login prompt
// would appear, which scribbles over the onboarding screen on tty1.
func waitFirstBoot() {
	if _, err := os.Stat("/run/cloud-init/result.json"); err == nil {
		return
	}
	if _, err := exec.LookPath("cloud-init"); err != nil {
		return
	}
	fmt.Println("Finishing the first boot…")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	exec.CommandContext(ctx, "cloud-init", "status", "--wait").Run()
}
