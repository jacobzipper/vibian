// Package sys holds small helpers for running system commands.
package sys

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Output runs a command and returns its trimmed stdout. stderr is folded into
// the error so callers can show it.
func Output(name string, args ...string) (string, error) {
	var out, errb bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), fmt.Errorf("%s: %s", name, msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// AsRoot prefixes a command with sudo unless we already are root.
func AsRoot(name string, args ...string) []string {
	if os.Geteuid() == 0 {
		return append([]string{name}, args...)
	}
	return append([]string{"sudo", name}, args...)
}

// RootOutput is Output run through AsRoot.
func RootOutput(name string, args ...string) (string, error) {
	argv := AsRoot(name, args...)
	return Output(argv[0], argv[1:]...)
}

// Run runs a command attached to the terminal.
func Run(argv ...string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// Has reports whether a command is on PATH.
func Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ReadFile returns a file's trimmed contents, or "" if it can't be read.
func ReadFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
