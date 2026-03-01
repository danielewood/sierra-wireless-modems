package qmux

import (
	"fmt"
	"os"
	"path/filepath"
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
// For /dev/cdc-wdm* devices, it checks sysfs to determine whether the
// device is driven by qmi_wwan (raw QMI) or cdc_mbim (MBIM tunneling).
// For /dev/qcqmi* devices, raw QMI is always used.
func OpenTransport(devicePath string) (Transport, error) {
	if strings.Contains(devicePath, "cdc-wdm") {
		if isMBIMDevice(devicePath) {
			return openMBIMTransport(devicePath)
		}
		return OpenRawTransport(devicePath)
	}
	return OpenRawTransport(devicePath)
}

// isMBIMDevice checks sysfs to determine if a cdc-wdm device uses the
// cdc_mbim driver (MBIM) vs qmi_wwan (raw QMI).
// Falls back to assuming MBIM if the driver can't be determined.
func isMBIMDevice(devicePath string) bool {
	// Extract device name: "/dev/cdc-wdm0" → "cdc-wdm0"
	devName := filepath.Base(devicePath)
	driverLink, err := os.Readlink(filepath.Join("/sys/class/usbmisc", devName, "device/driver"))
	if err != nil {
		// Can't determine driver — default to MBIM (the more common case).
		return true
	}
	driver := filepath.Base(driverLink)
	return driver != "qmi_wwan"
}

// rawTransport sends/receives QMUX frames directly on /dev/qcqmi* devices.
type rawTransport struct {
	f *os.File
}

// OpenRawTransport opens a raw QMI transport for the given device path.
// Use this when the device speaks raw QMUX (e.g. /dev/cdc-wdm0 in QMI mode
// or /dev/qcqmi* devices).
func OpenRawTransport(devicePath string) (*rawTransport, error) {
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
