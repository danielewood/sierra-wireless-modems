package modem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
)

var (
	// ErrNoModemFound indicates no supported modem was detected.
	ErrNoModemFound = errors.New("no EM7455/MC7455 modem found")

	// ErrMultipleModems indicates more than one modem was found.
	ErrMultipleModems = errors.New("multiple modems found; remove extras and retry")

	// ErrTimeout indicates a polling operation timed out.
	ErrTimeout = errors.New("timed out waiting for modem")
)

const sysfsUSBDevices = "/sys/bus/usb/devices"

// Detect scans sysfs for a connected EM7455/MC7455 modem.
// Returns the detected device or an error.
func Detect(l *log.Logger) (*Device, error) {
	return detectWithIDs(l, OnlineIDs)
}

// DetectBootloader scans sysfs for a modem in bootloader/QDL mode.
func DetectBootloader(l *log.Logger) (*Device, error) {
	dev, err := detectWithIDs(l, BootloaderIDs)
	if err != nil {
		return nil, err
	}
	dev.Bootloader = true
	return dev, nil
}

// WaitForModem polls for an online modem until found or timeout.
func WaitForModem(l *log.Logger, timeout time.Duration) (*Device, error) {
	return poll(l, timeout, 3*time.Second, Detect)
}

// WaitForBootloader polls for a modem in bootloader mode until found or timeout.
func WaitForBootloader(l *log.Logger, timeout time.Duration) (*Device, error) {
	return poll(l, timeout, 2*time.Second, DetectBootloader)
}

func poll(l *log.Logger, timeout, interval time.Duration, fn func(*log.Logger) (*Device, error)) (*Device, error) {
	deadline := time.Now().Add(timeout)
	spin := l.StartSpinner("Waiting for modem...")
	defer spin.Stop()
	for {
		dev, err := fn(l)
		if err == nil {
			return dev, nil
		}
		if !errors.Is(err, ErrNoModemFound) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, ErrTimeout
		}
		time.Sleep(interval)
	}
}

func detectWithIDs(l *log.Logger, knownIDs map[USBID]string) (*Device, error) {
	entries, err := os.ReadDir(sysfsUSBDevices)
	if err != nil {
		return nil, fmt.Errorf("reading sysfs: %w", err)
	}

	var found []*Device
	for _, entry := range entries {
		devPath := filepath.Join(sysfsUSBDevices, entry.Name())

		vid, err := readSysfsFile(filepath.Join(devPath, "idVendor"))
		if err != nil {
			continue
		}
		pid, err := readSysfsFile(filepath.Join(devPath, "idProduct"))
		if err != nil {
			continue
		}

		id := USBID{Vendor: vid, Product: pid}
		name, ok := knownIDs[id]
		if !ok {
			continue
		}

		dev := &Device{
			ID:        id,
			Name:      name,
			SysfsPath: devPath,
		}

		// Find AT port and CDC/QCQMI device from interfaces
		discoverDevicePaths(l, dev)

		l.Debugf("found modem: %s (%s) at %s", name, id, devPath)
		if dev.ATPort != "" {
			l.Debugf("  AT port: %s", dev.ATPort)
		}
		if dev.CDCDevice != "" {
			l.Debugf("  CDC device: %s", dev.CDCDevice)
		}

		found = append(found, dev)
	}

	switch len(found) {
	case 0:
		return nil, ErrNoModemFound
	case 1:
		return found[0], nil
	default:
		return nil, ErrMultipleModems
	}
}

// discoverDevicePaths finds the AT serial port and CDC/QCQMI device
// by walking the USB interface subdirectories in sysfs.
func discoverDevicePaths(l *log.Logger, dev *Device) {
	entries, err := os.ReadDir(dev.SysfsPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		ifacePath := filepath.Join(dev.SysfsPath, entry.Name())

		// Find the AT command interface (bInterfaceNumber = 03)
		ifaceNum, err := readSysfsFile(filepath.Join(ifacePath, "bInterfaceNumber"))
		if err != nil {
			continue
		}

		if ifaceNum == "03" {
			dev.ATPort = findTTYDevice(ifacePath)
		}

		// Look for CDC-WDM device (MBIM) or QCQMI device (QMI)
		findControlDevice(ifacePath, dev)
	}
}

// findTTYDevice looks for a ttyUSB device under an interface directory.
func findTTYDevice(ifacePath string) string {
	// Walk looking for tty subdirectories
	matches, _ := filepath.Glob(filepath.Join(ifacePath, "ttyUSB*"))
	if len(matches) > 0 {
		return "/dev/" + filepath.Base(matches[0])
	}

	// Sometimes it's nested: interface/tty/ttyUSBx
	matches, _ = filepath.Glob(filepath.Join(ifacePath, "tty", "ttyUSB*"))
	if len(matches) > 0 {
		return "/dev/" + filepath.Base(matches[0])
	}

	// Try via /sys/class/tty symlinks
	ttyEntries, err := filepath.Glob("/sys/class/tty/ttyUSB*")
	if err != nil {
		return ""
	}
	for _, ttyPath := range ttyEntries {
		link, err := os.Readlink(filepath.Join(ttyPath, "device"))
		if err != nil {
			continue
		}
		// Resolve and check if it points to our interface
		resolved, err := filepath.Abs(filepath.Join(ttyPath, "device", link))
		if err != nil {
			continue
		}
		if strings.HasPrefix(resolved, ifacePath) {
			return "/dev/" + filepath.Base(ttyPath)
		}
	}

	return ""
}

// findControlDevice looks for cdc-wdm or qcqmi devices under an interface.
func findControlDevice(ifacePath string, dev *Device) {
	// Look for usbmisc/cdc-wdm*
	matches, _ := filepath.Glob(filepath.Join(ifacePath, "usbmisc", "cdc-wdm*"))
	for _, m := range matches {
		dev.CDCDevice = "/dev/" + filepath.Base(m)
		return
	}

	// Also check net/*/usbmisc/cdc-wdm* pattern
	matches, _ = filepath.Glob(filepath.Join(ifacePath, "*", "usbmisc", "cdc-wdm*"))
	for _, m := range matches {
		dev.CDCDevice = "/dev/" + filepath.Base(m)
		return
	}

	// Look for qcqmi devices
	matches, _ = filepath.Glob(filepath.Join(ifacePath, "GobiQMI", "qcqmi*"))
	for _, m := range matches {
		dev.QCQMIDevice = "/dev/" + filepath.Base(m)
		return
	}
}

// readSysfsFile reads a single-line sysfs file and trims whitespace.
func readSysfsFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
