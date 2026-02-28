// Package log provides a colored slog-based logger for CLI output.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[0;36m"
	colorRed    = "\033[0;31m"
	colorYellow = "\033[0;33m"
	colorGreen  = "\033[0;32m"
	colorGray   = "\033[0;37m"
)

// Logger wraps slog with colored step output matching the original script style.
type Logger struct {
	*slog.Logger
	verbose bool
	quiet   bool

	mu      sync.Mutex
	spinner *Spinner // active spinner, if any
}

// New creates a logger. If verbose, debug messages are shown.
// If quiet, only errors are shown.
func New(verbose, quiet bool) *Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	if quiet {
		level = slog.LevelError
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	return &Logger{
		Logger:  slog.New(handler),
		verbose: verbose,
		quiet:   quiet,
	}
}

// clearSpinnerLine erases the spinner from the terminal so other output
// can print cleanly. The spinner will redraw on its next tick.
func (l *Logger) clearSpinnerLine() {
	if l.spinner != nil {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}
}

// Step prints a cyan-colored step header to stderr, matching the bash script's style.
func (l *Logger) Step(msg string) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	l.clearSpinnerLine()
	l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s---%s %s\n", colorCyan, colorReset, msg)
}

// Success prints a green-colored success message to stderr.
func (l *Logger) Success(msg string) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	l.clearSpinnerLine()
	l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s%s%s\n", colorGreen, msg, colorReset)
}

// Errorf prints a red-colored error message to stderr.
func (l *Logger) Errorf(format string, args ...any) {
	l.mu.Lock()
	l.clearSpinnerLine()
	l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%sERROR: %s%s\n", colorRed, fmt.Sprintf(format, args...), colorReset)
}

// Warnf prints a yellow-colored warning message to stderr.
func (l *Logger) Warnf(format string, args ...any) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	l.clearSpinnerLine()
	l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%sWARN: %s%s\n", colorYellow, fmt.Sprintf(format, args...), colorReset)
}

// Infof prints an info message to stderr.
func (l *Logger) Infof(format string, args ...any) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	l.clearSpinnerLine()
	l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s\n", fmt.Sprintf(format, args...))
}

// Debugf prints a debug message to stderr (only in verbose mode).
func (l *Logger) Debugf(format string, args ...any) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	l.clearSpinnerLine()
	l.mu.Unlock()
	fmt.Fprintf(os.Stderr, "%s%s%s\n", colorGray, fmt.Sprintf(format, args...), colorReset)
}

// Writer returns an io.Writer that writes to stderr at info level.
// Useful for passing to external command output.
func (l *Logger) Writer() io.Writer {
	if l.quiet {
		return io.Discard
	}
	return os.Stderr
}

// Spinner displays an animated braille spinner with a message on stderr.
type Spinner struct {
	mu    sync.Mutex
	msg   string
	start time.Time
	stop  chan struct{}
	done  chan struct{}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner begins an animated spinner with the given message.
// Returns a Spinner that must be stopped with Stop().
func (l *Logger) StartSpinner(msg string) *Spinner {
	s := &Spinner{
		msg:   msg,
		start: time.Now(),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	if l.quiet {
		close(s.done)
		return s
	}

	l.mu.Lock()
	l.spinner = s
	l.mu.Unlock()

	go func() {
		defer close(s.done)
		frame := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				l.mu.Lock()
				l.spinner = nil
				l.mu.Unlock()
				return
			case <-ticker.C:
				s.mu.Lock()
				msg := s.msg
				s.mu.Unlock()
				elapsed := int(time.Since(s.start).Seconds())
				fmt.Fprintf(os.Stderr, "\r\033[K%s%s%s %s %s(%ds)%s",
					colorCyan, spinnerFrames[frame%len(spinnerFrames)], colorReset,
					msg,
					colorGray, elapsed, colorReset)
				frame++
			}
		}
	}()

	return s
}

// Update changes the spinner message while it's running.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	select {
	case <-s.stop:
		return // already stopped
	default:
		close(s.stop)
	}
	<-s.done
}
