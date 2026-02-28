// Package sysutil provides system-level utilities for root checks,
// service management, and external command execution.
package sysutil

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
)

// RunCommand executes an external command and returns combined stdout+stderr.
// The command and output are logged at debug level.
func RunCommand(l *log.Logger, name string, args ...string) (string, error) {
	l.Debugf("exec: %s %s", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if output != "" {
		l.Debugf("output: %s", output)
	}

	if err != nil {
		return output, fmt.Errorf("running %s: %w\n%s", name, err, output)
	}
	return output, nil
}

// RunCommandSilent executes a command without logging. Returns stdout+stderr.
func RunCommandSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("running %s: %w", name, err)
	}
	return output, nil
}

// CheckBinaryExists verifies that a required binary is in PATH.
func CheckBinaryExists(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w", name, err)
	}
	return nil
}
