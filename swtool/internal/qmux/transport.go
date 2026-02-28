package qmux

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Transport abstracts the underlying device I/O for QMI communication.
// Raw QMI devices (/dev/qcqmi*) send QMUX frames directly.
// MBIM devices (/dev/cdc-wdm*) wrap QMUX in MBIM COMMAND messages.
type Transport interface {
	// Send writes a QMUX frame to the device.
	Send(frame []byte) error

	// Receive reads the next QMUX frame from the device.
	Receive() ([]byte, error)

	// Close releases the device.
	Close() error
}

// DefaultReadTimeout is the read timeout for device I/O.
const DefaultReadTimeout = 5 * time.Second

// OpenTransport opens a QMI transport for the given device path.
// It auto-detects MBIM (/dev/cdc-wdm*) vs raw QMI (/dev/qcqmi*) based on
// the device path name.
func OpenTransport(devicePath string) (Transport, error) {
	if strings.Contains(devicePath, "cdc-wdm") {
		return openMBIMTransport(devicePath)
	}
	return openRawTransport(devicePath)
}

// rawTransport sends/receives QMUX frames directly on /dev/qcqmi* devices.
type rawTransport struct {
	f *os.File
}

func openRawTransport(devicePath string) (*rawTransport, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening raw QMI device %s: %w", devicePath, err)
	}
	return &rawTransport{f: f}, nil
}

func (t *rawTransport) Send(frame []byte) error {
	_, err := t.f.Write(frame)
	if err != nil {
		return fmt.Errorf("writing to QMI device: %w", err)
	}
	return nil
}

func (t *rawTransport) Receive() ([]byte, error) {
	// QMI frames are at most 4096 bytes (standard USB control transfer size).
	buf := make([]byte, 4096)

	if err := t.f.SetReadDeadline(time.Now().Add(DefaultReadTimeout)); err != nil {
		// Not all file types support deadlines; fall through to blocking read.
	}

	n, err := t.f.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("reading from QMI device: %w", err)
	}
	return buf[:n], nil
}

func (t *rawTransport) Close() error {
	return t.f.Close()
}
