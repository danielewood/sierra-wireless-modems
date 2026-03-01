// Package sysutil provides system-level utilities for root checks,
// service management, and external command execution.
package sysutil

import (
	"bytes"
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

// RunCommandLive executes a command with stdout/stderr streamed line-by-line
// through the logger. Debug-level lines from the subprocess (e.g. [Debug],
// raw protocol dumps) are routed to l.Debugf so they only appear with
// --verbose. Progress lines are shown at info level. All output is captured
// in a buffer for error reporting.
func RunCommandLive(l *log.Logger, name string, args ...string) (string, error) {
	l.Debugf("exec: %s %s", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	fw := &filterWriter{logger: l}
	cmd.Stdout = fw
	cmd.Stderr = fw

	err := cmd.Run()
	fw.flush()
	output := strings.TrimSpace(fw.buf.String())

	if err != nil {
		return output, fmt.Errorf("running %s: %w\n%s", name, err, output)
	}
	return output, nil
}

// filterWriter captures all output and routes each line through the logger.
// Lines containing "[Debug]" or raw protocol dumps ("<<<<<<", ">>>>>>") go
// to Debugf. Warning lines go to Warnf. Everything else goes to Infof.
type filterWriter struct {
	logger *log.Logger
	buf    bytes.Buffer  // captures everything for error reporting
	line   bytes.Buffer  // accumulates the current line
}

func (w *filterWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for _, b := range p {
		if b == '\n' {
			w.routeLine(w.line.String())
			w.line.Reset()
		} else {
			w.line.WriteByte(b)
		}
	}
	return len(p), nil
}

// flush outputs any remaining partial line.
func (w *filterWriter) flush() {
	if w.line.Len() > 0 {
		w.routeLine(w.line.String())
		w.line.Reset()
	}
}

// routeLine sends a single line to the appropriate log level.
func (w *filterWriter) routeLine(line string) {
	if line == "" {
		return
	}
	switch {
	case strings.Contains(line, "[Debug]"),
		strings.HasPrefix(line, "<<<<<<"),
		strings.HasPrefix(line, ">>>>>>"):
		w.logger.Debugf("  %s", line)
	case strings.Contains(line, "-Warning **"):
		// Strip the "-Warning ** " prefix for cleaner output.
		if i := strings.Index(line, "] "); i >= 0 {
			w.logger.Warnf("%s", line[i+2:])
		} else {
			w.logger.Warnf("%s", line)
		}
	case strings.HasPrefix(line, "error:"):
		w.logger.Errorf("%s", strings.TrimPrefix(line, "error: "))
	default:
		w.logger.Infof("  %s", line)
	}
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
