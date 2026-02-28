package sysutil

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
)

// RequireRoot checks that the program is running as root (uid 0).
func RequireRoot() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("this tool must be run as root (sudo)")
	}
	return nil
}

// IsPermissionError returns true if err is a permission denied error.
func IsPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES)
}

// PermissionHint returns actionable guidance for fixing serial port permissions.
func PermissionHint() string {
	return `permission denied opening serial port

Fix this by running:  sudo swtool setup

Or manually:
  sudo usermod -aG dialout $USER
  sudo tee /etc/udev/rules.d/99-sierra-wireless.rules <<'EOF'
  SUBSYSTEM=="tty", ATTRS{idVendor}=="1199", MODE="0660", GROUP="dialout"
  SUBSYSTEM=="tty", ATTRS{idVendor}=="413c", MODE="0660", GROUP="dialout"
  SUBSYSTEM=="usb", ATTRS{idVendor}=="1199", MODE="0660", GROUP="dialout"
  SUBSYSTEM=="usb", ATTRS{idVendor}=="413c", MODE="0660", GROUP="dialout"
  EOF
  sudo udevadm control --reload-rules && sudo udevadm trigger
  # Log out and back in for group change to take effect`
}

// modemManagerInstalled checks if ModemManager is known to systemd.
func modemManagerInstalled() bool {
	// "systemctl list-unit-files ModemManager.service" exits 0 even if MM
	// isn't installed, but produces no output.  A simpler check: see if
	// the unit file path resolves.
	_, err := RunCommandSilent("systemctl", "cat", "ModemManager.service")
	return err == nil
}

// StopModemManager stops and disables ModemManager via systemctl.
// Does nothing if ModemManager is not installed.
func StopModemManager(l *log.Logger) error {
	if !modemManagerInstalled() {
		l.Debugf("ModemManager not installed, skipping stop")
		return nil
	}
	l.Step("Stopping ModemManager...")
	RunCommand(l, "systemctl", "stop", "ModemManager")
	RunCommand(l, "systemctl", "disable", "ModemManager")
	return nil
}

// StartModemManager re-enables and starts ModemManager.
// Does nothing if ModemManager is not installed.
func StartModemManager(l *log.Logger) {
	if !modemManagerInstalled() {
		return
	}
	l.Step("Restarting ModemManager...")
	RunCommand(l, "systemctl", "enable", "ModemManager")
	RunCommand(l, "systemctl", "start", "ModemManager")
}
