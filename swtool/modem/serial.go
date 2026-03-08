package modem

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
	"go.bug.st/serial"
)

var (
	// ErrATCommandFailed indicates the modem returned ERROR.
	ErrATCommandFailed = errors.New("AT command returned ERROR")

	// ErrATTimeout indicates the AT command response timed out.
	ErrATTimeout = errors.New("AT command response timed out")
)

// Port wraps a serial port for AT command communication.
type Port struct {
	port            serial.Port
	device          string
	log             *log.Logger
	timeout         time.Duration
	platform        Platform // Qualcomm or Intel — determines AT command set
	engineeringMode bool     // true after successful AT!ENTERCND (Qualcomm only)
}

// OpenPortForDevice opens a serial port using the device's AT port and platform.
func OpenPortForDevice(dev *Device, l *log.Logger) (*Port, error) {
	p, err := OpenPort(dev.ATPort, l)
	if err != nil {
		return nil, err
	}
	p.platform = dev.Platform
	return p, nil
}

// OpenPort opens a serial port for AT commands at 115200 8N1.
func OpenPort(device string, l *log.Logger) (*Port, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(device, mode)
	if err != nil {
		if sysutil.IsPermissionError(err) {
			return nil, fmt.Errorf("%s", sysutil.PermissionHint())
		}
		return nil, fmt.Errorf("opening serial port %s: %w", device, err)
	}

	if err := port.SetReadTimeout(2 * time.Second); err != nil {
		port.Close()
		return nil, fmt.Errorf("setting read timeout: %w", err)
	}

	p := &Port{
		port:    port,
		device:  device,
		log:     l,
		timeout: 10 * time.Second,
	}

	// Drain any buffered data
	p.drain()

	return p, nil
}

// SetTimeout overrides the default AT command response deadline.
func (p *Port) SetTimeout(d time.Duration) {
	p.timeout = d
}

// Close closes the serial port.
func (p *Port) Close() error {
	return p.port.Close()
}

// SendCommand sends an AT command and reads the response.
// Returns the response text between the echo and the OK/ERROR terminator.
func (p *Port) SendCommand(cmd string) (string, error) {
	// Purge the kernel's serial input buffer before sending. This is an
	// instant syscall (tcflush) that prevents stale data from a previous
	// slow response from contaminating this command's response. Without
	// this, readResponse can find an OK from old data and return the
	// wrong response entirely.
	p.port.ResetInputBuffer()

	p.log.Debugf("AT>>> %s", cmd)
	t0 := time.Now()

	// Write the command
	_, err := p.port.Write([]byte(cmd + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("writing to serial port: %w", err)
	}

	// Read response, discarding stale OK/ERROR that doesn't belong to us.
	response, err := p.readResponseFor(cmd)
	elapsed := time.Since(t0)
	if err != nil {
		p.log.Debugf("AT !!! %s (%s)", cmd, elapsed.Round(time.Millisecond))
		return "", fmt.Errorf("AT command %q: %w", cmd, err)
	}

	p.log.Debugf("AT<<< %s (%s)", strings.TrimSpace(response), elapsed.Round(time.Millisecond))

	// Check for ERROR response
	if containsTerminator(response, "ERROR") {
		return response, fmt.Errorf("AT command %q: %w", cmd, ErrATCommandFailed)
	}

	return response, nil
}

// SendCommandExpect sends a command and verifies the response contains
// the expected substring.
func (p *Port) SendCommandExpect(cmd, expect string) (string, error) {
	resp, err := p.SendCommand(cmd)
	if err != nil {
		return resp, err
	}
	if !strings.Contains(resp, expect) {
		return resp, fmt.Errorf("AT command %q: expected %q in response, got: %s", cmd, expect, resp)
	}
	return resp, nil
}

// readResponseFor reads from the serial port until it finds an OK or ERROR
// terminator for the given command. If a terminator is found but the response
// contains a DIFFERENT command's echo, it's stale data — discard and keep
// reading. If no echo is found at all (e.g., echo disabled for ATE1), the
// response is accepted.
func (p *Port) readResponseFor(cmd string) (string, error) {
	var buf strings.Builder
	chunk := make([]byte, 1024)
	deadline := time.Now().Add(p.timeout)

	for time.Now().Before(deadline) {
		n, err := p.port.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			content := buf.String()

			if containsTerminator(content, "OK") || containsTerminator(content, "ERROR") {
				// Best case: our command echo is present.
				if strings.Contains(content, cmd) {
					return stripPreEcho(content, cmd), nil
				}
				// Check if the response contains a DIFFERENT command's echo.
				// If so, this is definitely stale data — discard and keep reading.
				if containsForeignEcho(content, cmd) {
					p.log.Debugf("AT: discarding stale %d bytes (foreign echo, expected %q)", buf.Len(), cmd)
					buf.Reset()
					continue
				}
				// No echo at all (e.g., ATE1 when echo is off, or echo not
				// yet enabled). Accept the response as-is.
				return content, nil
			}
		}
		if err != nil {
			// Read timeout is expected — keep trying until deadline
			continue
		}
	}

	// If we got some data but no terminator, return what we have with a timeout error
	if buf.Len() > 0 {
		return buf.String(), ErrATTimeout
	}
	return "", ErrATTimeout
}

// containsForeignEcho checks if the response contains an AT command echo
// that doesn't match the command we sent. This detects stale responses
// from previous commands.
func containsForeignEcho(response, myCmd string) bool {
	for _, line := range strings.Split(response, "\r\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "AT") && line != myCmd {
			return true
		}
	}
	return false
}

// stripPreEcho removes any data that appeared before the command echo.
func stripPreEcho(response, cmd string) string {
	idx := strings.Index(response, cmd)
	if idx > 0 {
		return response[idx:]
	}
	return response
}

// containsTerminator checks if the response contains a line that is just
// the terminator word (OK or ERROR), surrounded by \r\n.
func containsTerminator(response, terminator string) bool {
	return strings.Contains(response, "\r\n"+terminator+"\r\n") ||
		strings.HasSuffix(strings.TrimSpace(response), terminator)
}

// drain reads and discards any pending data in the serial buffer.
func (p *Port) drain() {
	buf := make([]byte, 4096)
	// Set a very short timeout for draining
	p.port.SetReadTimeout(50 * time.Millisecond)
	for {
		n, _ := p.port.Read(buf)
		if n == 0 {
			break
		}
		p.log.Debugf("drain: discarded %d bytes", n)
	}
	// Restore normal timeout
	p.port.SetReadTimeout(2 * time.Second)
}
