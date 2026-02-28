package qmi

import (
	"fmt"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
)

// FlashFirmware runs the full firmware update process using
// qmi-firmware-update --update. This handles the complete lifecycle:
// set firmware preference → reset to bootloader → flash → reboot.
//
// For 9x15/9x30 devices (EM7455/MC7455), --update sets the firmware
// preference via QMI DMS before resetting to bootloader mode. This is
// critical: without updating the preference, the modem boots into low-power
// mode due to a firmware version mismatch.
//
// The carrier string must match the exact case from the NVU binary (e.g.
// "GENERIC", not "Generic"). qmi-firmware-update auto-detects carrier
// from filenames using title case, but the modem's firmware preference
// uses the case from the NVU. A case mismatch in AT!IMPREF causes the
// IMSWITCH LPM voter to hold the modem in low-power mode.
//
// device is the cdc-wdm path (e.g. "/dev/cdc-wdm0").
func FlashFirmware(l *log.Logger, device, carrier, cweFile, nvuFile string) error {
	l.Step(fmt.Sprintf("Flashing firmware: %s + %s (carrier: %s) ...", cweFile, nvuFile, carrier))

	_, err := sysutil.RunCommand(l, "qmi-firmware-update",
		"--update",
		"--cdc-wdm", device,
		"--device-open-mbim",
		"-p",
		"--carrier", carrier,
		cweFile, nvuFile,
	)
	if err != nil {
		return fmt.Errorf("flashing firmware: %w", err)
	}

	return nil
}

// FlashFirmwareDownloadMode flashes firmware onto a modem already in QDL
// bootloader mode using --update-download. Use FlashFirmware for normal
// operation; this is only needed for manual recovery.
func FlashFirmwareDownloadMode(l *log.Logger, vidpid, cweFile, nvuFile string) error {
	l.Step(fmt.Sprintf("Flashing firmware (download mode): %s + %s ...", cweFile, nvuFile))

	_, err := sysutil.RunCommand(l, "qmi-firmware-update",
		"--update-download",
		"-d", vidpid,
		cweFile, nvuFile,
	)
	if err != nil {
		return fmt.Errorf("flashing firmware: %w", err)
	}

	return nil
}
