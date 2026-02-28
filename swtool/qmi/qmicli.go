// Package qmi provides QMI modem operations via native protocol or qmicli fallback.
package qmi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/qmux"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
)

// ErrQMINotInstalled indicates that qmi-firmware-update is missing.
// qmicli is no longer required (native QMI is preferred).
var ErrQMINotInstalled = fmt.Errorf("qmi-firmware-update not installed (needed for firmware flashing)")

// CheckInstalled verifies that qmi-firmware-update is in PATH.
// qmicli is optional — native QMI is used when available.
func CheckInstalled() error {
	if err := sysutil.CheckBinaryExists("qmi-firmware-update"); err != nil {
		return ErrQMINotInstalled
	}
	return nil
}

// hasQMICLI returns true if qmicli is available in PATH.
func hasQMICLI() bool {
	return sysutil.CheckBinaryExists("qmicli") == nil
}

// SetUSBComposition sets the modem USB composition.
// compositionID is the numeric value (e.g. "8" for MBIM, "6" for QMI).
// cdcDevice is the CDC-WDM device path (e.g. "/dev/cdc-wdm0").
func SetUSBComposition(l *log.Logger, cdcDevice, compositionID string) error {
	l.Step(fmt.Sprintf("Setting USB composition to %s...", compositionID))

	// Try native QMI first.
	conn, err := qmux.Open(cdcDevice)
	if err == nil {
		defer conn.Close()
		id, parseErr := strconv.Atoi(compositionID)
		if parseErr != nil {
			return fmt.Errorf("invalid composition ID %q: %w", compositionID, parseErr)
		}
		if err := qmux.SetUSBComposition(conn, uint8(id)); err != nil {
			return fmt.Errorf("setting USB composition: %w", err)
		}
		return nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return fmt.Errorf("setting USB composition: native QMI failed and qmicli not available")
	}

	_, err = sysutil.RunCommand(l, "qmicli",
		"--device-open-mbim", "-p",
		"-d", cdcDevice,
		fmt.Sprintf("--dms-swi-set-usb-composition=%s", compositionID),
	)
	if err != nil {
		return fmt.Errorf("setting USB composition: %w", err)
	}
	return nil
}

// ResetViaQMI sends a DMS operating mode reset.
// device is the control device path (e.g. "/dev/cdc-wdm0" or "/dev/qcqmi0").
// mbim should be true when using a CDC-WDM device.
func ResetViaQMI(l *log.Logger, device string, mbim bool) error {
	l.Step("Resetting modem...")

	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		if err := qmux.SetOperatingMode(conn, qmux.ModeReset); err != nil {
			return fmt.Errorf("resetting modem: %w", err)
		}
		return nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return fmt.Errorf("resetting modem: native QMI failed and qmicli not available")
	}

	args := []string{"-p", "-d", device, "--dms-set-operating-mode=reset"}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	_, err = sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return fmt.Errorf("resetting modem via qmicli: %w", err)
	}
	return nil
}

// SetOnline sets the modem operating mode to online.
// Use this to bring a modem out of low-power or offline mode.
func SetOnline(l *log.Logger, device string, mbim bool) error {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		if err := qmux.SetOperatingMode(conn, qmux.ModeOnline); err != nil {
			return fmt.Errorf("setting modem online: %w", err)
		}
		return nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return fmt.Errorf("setting modem online: native QMI failed and qmicli not available")
	}

	args := []string{"-p", "-d", device, "--dms-set-operating-mode=online"}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	_, err = sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return fmt.Errorf("setting modem online: %w", err)
	}
	return nil
}

// GetOperatingMode queries the DMS operating mode.
// Returns the mode string: "online", "offline", "low-power", "reset", etc.
func GetOperatingMode(l *log.Logger, device string, mbim bool) (string, error) {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		mode, err := qmux.GetOperatingMode(conn)
		if err != nil {
			return "", fmt.Errorf("getting operating mode: %w", err)
		}
		return qmux.ModeString(mode), nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return "", fmt.Errorf("getting operating mode: native QMI failed and qmicli not available")
	}

	args := []string{"-p", "-d", device, "--dms-get-operating-mode"}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	out, err := sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return "", fmt.Errorf("getting operating mode: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		key, val := parseQMIKV(strings.TrimSpace(line))
		if key == "Mode" {
			return val, nil
		}
	}
	return "", fmt.Errorf("no operating mode in output: %s", out)
}

// SetFirmwarePreference updates the modem's firmware preference via QMI DMS.
// This tells the modem which firmware version/carrier/config to boot with.
// After a flash, if the preference doesn't match the installed firmware, the
// modem stays in low-power mode ("fw version mismatch" in AT!IMPREF?).
//
// carrier must match the modem's case exactly (e.g. "GENERIC" not "Generic").
func SetFirmwarePreference(l *log.Logger, device string, mbim bool, fwVersion, configVersion, carrier string) error {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		if err := qmux.SetFirmwarePreference(conn, fwVersion, configVersion, carrier); err != nil {
			return fmt.Errorf("setting firmware preference: %w", err)
		}
		return nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return fmt.Errorf("setting firmware preference: native QMI failed and qmicli not available")
	}

	pref := fmt.Sprintf("firmware-version=%s,config-version=%s,carrier=%s",
		fwVersion, configVersion, carrier)
	args := []string{"-p", "-d", device,
		fmt.Sprintf("--dms-set-firmware-preference=%s", pref),
	}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	_, err = sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return fmt.Errorf("setting firmware preference: %w", err)
	}
	return nil
}

// ListStoredImages queries the DMS stored firmware images.
// Returns the list of all modem and PRI image slots.
func ListStoredImages(l *log.Logger, device string, mbim bool) ([]StoredImage, error) {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		nativeImages, err := qmux.ListStoredImagesEx(conn)
		if err != nil {
			return nil, fmt.Errorf("listing stored images: %w", err)
		}
		// Convert internal types to public types.
		result := make([]StoredImage, len(nativeImages))
		for i, img := range nativeImages {
			result[i] = StoredImage{
				Type:         qmux.ImageTypeString(img.Type),
				Slot:         int(img.Slot),
				UniqueID:     img.UniqueID,
				BuildID:      img.BuildID,
				StorageIndex: int(img.StorageIndex),
				FailureCount: int(img.FailureCount),
				Current:      img.Current,
			}
		}
		return result, nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return nil, fmt.Errorf("listing stored images: native QMI failed and qmicli not available")
	}

	args := []string{"-p", "-d", device, "--dms-list-stored-images"}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	out, err := sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return nil, fmt.Errorf("listing stored images: %w", err)
	}
	return parseStoredImages(out), nil
}

// GetFirmwarePreference queries the DMS firmware preference.
// Returns the currently configured firmware preference images.
func GetFirmwarePreference(l *log.Logger, device string, mbim bool) ([]FirmwarePreferenceImage, error) {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		nativePrefs, err := qmux.GetFirmwarePreference(conn)
		if err != nil {
			return nil, fmt.Errorf("getting firmware preference: %w", err)
		}
		result := make([]FirmwarePreferenceImage, len(nativePrefs))
		for i, p := range nativePrefs {
			result[i] = FirmwarePreferenceImage{
				Type:     qmux.ImageTypeString(p.Type),
				UniqueID: p.UniqueID,
				BuildID:  p.BuildID,
			}
		}
		return result, nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return nil, fmt.Errorf("getting firmware preference: native QMI failed and qmicli not available")
	}

	args := []string{"-p", "-d", device, "--dms-get-firmware-preference"}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	out, err := sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return nil, fmt.Errorf("getting firmware preference: %w", err)
	}
	return parseFirmwarePreference(out), nil
}

// SelectStoredImage selects the active firmware image slot pair.
// modemSlot and priSlot are 0-based slot indices.
func SelectStoredImage(l *log.Logger, device string, mbim bool, modemSlot, priSlot int) error {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		images, err := qmux.ListStoredImages(conn)
		if err != nil {
			return fmt.Errorf("selecting stored image: %w", err)
		}
		if err := qmux.SelectStoredImage(conn, images, modemSlot, priSlot); err != nil {
			return fmt.Errorf("selecting stored image: %w", err)
		}
		return nil
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return fmt.Errorf("selecting stored image: native QMI failed and qmicli not available")
	}

	sel := fmt.Sprintf("modem=%d,pri=%d", modemSlot, priSlot)
	args := []string{"-p", "-d", device,
		fmt.Sprintf("--dms-select-stored-image=%s", sel),
	}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	_, err = sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return fmt.Errorf("selecting stored image: %w", err)
	}
	return nil
}

// DeleteStoredImage deletes a stored firmware image slot.
// imageType is "modem" or "pri", slot is the 0-based index.
func DeleteStoredImage(l *log.Logger, device string, mbim bool, imageType string, slot int) error {
	conn, err := qmux.Open(device)
	if err == nil {
		defer conn.Close()
		images, listErr := qmux.ListStoredImages(conn)
		if listErr != nil {
			return fmt.Errorf("deleting stored image: %w", listErr)
		}
		imgTypeByte, typeErr := qmux.ImageTypeFromString(imageType)
		if typeErr != nil {
			return fmt.Errorf("deleting stored image: %w", typeErr)
		}
		for _, img := range images {
			if img.Type == imgTypeByte && int(img.Slot) == slot {
				if err := qmux.DeleteStoredImage(conn, img); err != nil {
					return fmt.Errorf("deleting stored image: %w", err)
				}
				return nil
			}
		}
		return fmt.Errorf("deleting stored image: %s slot %d not found", imageType, slot)
	}
	l.Debugf("native QMI unavailable, falling back to qmicli: %v", err)

	if !hasQMICLI() {
		return fmt.Errorf("deleting stored image: native QMI failed and qmicli not available")
	}

	sel := fmt.Sprintf("%s=%d", imageType, slot)
	args := []string{"-p", "-d", device,
		fmt.Sprintf("--dms-delete-stored-image=%s", sel),
	}
	if mbim {
		args = append([]string{"--device-open-mbim"}, args...)
	}
	_, err = sysutil.RunCommand(l, "qmicli", args...)
	if err != nil {
		return fmt.Errorf("deleting stored image: %w", err)
	}
	return nil
}

// WaitForOnline polls the modem operating mode until it reports "online"
// or the timeout expires. Use this after a reset to verify the modem is
// ready for commands.
func WaitForOnline(l *log.Logger, device string, mbim bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	spin := l.StartSpinner("Waiting for modem to come online...")
	defer spin.Stop()
	var lastMode string
	triedSetOnline := false
	for {
		mode, err := GetOperatingMode(l, device, mbim)
		if err == nil && mode == "online" {
			return nil
		}
		if err != nil {
			l.Debugf("operating mode check: %v", err)
		} else if mode != lastMode {
			spin.Update(fmt.Sprintf("Waiting for modem to come online (mode: %s)...", mode))
			lastMode = mode
		}

		// If modem is stuck in low-power or offline, actively bring it online
		if err == nil && !triedSetOnline && (mode == "low-power" || mode == "offline") {
			l.Debugf("modem in %s mode, attempting to set online", mode)
			if setErr := SetOnline(l, device, mbim); setErr != nil {
				l.Debugf("set online failed: %v", setErr)
			}
			triedSetOnline = true
		}

		if time.Now().After(deadline) {
			if lastMode != "" {
				return fmt.Errorf("timed out waiting for modem to come online (last mode: %s)", lastMode)
			}
			return fmt.Errorf("timed out waiting for modem to come online")
		}
		time.Sleep(2 * time.Second)
	}
}
