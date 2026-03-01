package cmd

import (
	"fmt"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/firmware"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/qmux"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/danielewood/sierra-wireless-modems/swtool/qmi"
	"github.com/spf13/cobra"
)

var (
	flagFirmwareURL  string
	flagLegacy       bool
	flagSkipDownload bool
	flagSkipFlash    bool
)

var flashCmd = &cobra.Command{
	Use:   "flash",
	Short: "Full flash workflow: download + flash",
	Long: `Downloads the latest firmware from Sierra Wireless and flashes it onto the modem.

Use 'swtool configure' to change USB identity, bands, or other modem settings.
This is the default command when swtool is run without a subcommand.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()

		logger.Step(fmt.Sprintf("Starting flash workflow for %s (%s)", dev.Name, dev.ID))

		if isDryRun() {
			return flashDryRun(dev)
		}

		// Pre-flight checks
		if err := sysutil.RequireRoot(); err != nil {
			return err
		}
		if !flagSkipFlash {
			if err := qmi.CheckInstalled(); err != nil {
				return err
			}
		}

		// Step 1: Stop ModemManager
		sysutil.StopModemManager(logger)
		defer sysutil.StartModemManager(logger)

		// Step 2: Download firmware
		var fwFiles *firmware.FirmwareFiles

		if !flagSkipDownload {
			var ref *firmware.FirmwareRef

			switch {
			case flagFirmwareURL != "":
				ref = &firmware.FirmwareRef{
					URL:      flagFirmwareURL,
					Filename: firmware.DeriveFilename(flagFirmwareURL),
				}
			case flagLegacy:
				ref = &firmware.LegacyFirmware
				logger.Infof("Using legacy firmware: %s", ref.Filename)
			default:
				var err error
				ref, err = firmware.ScrapeLatestURL(logger)
				if err != nil {
					return fmt.Errorf("finding firmware URL: %w", err)
				}
			}

			zipPath, err := firmware.Download(logger, ref, ".")
			if err != nil {
				return fmt.Errorf("downloading firmware: %w", err)
			}

			fwFiles, err = firmware.Extract(logger, zipPath, ".")
			if err != nil {
				return fmt.Errorf("extracting firmware: %w", err)
			}
		} else {
			// Find existing firmware files in current directory
			var err error
			fwFiles, err = firmware.FindLocalFiles(".")
			if err != nil {
				return fmt.Errorf("finding local firmware files: %w", err)
			}
			logger.Infof("Using existing firmware: %s + %s", fwFiles.CWE, fwFiles.NVU)
		}

		// Step 3: Flash firmware
		if !flagSkipFlash {
			if dev.Bootloader {
				// Modem is already in QDL/bootloader mode — flash directly
				// without setting firmware preference (no QMI available).
				if err := qmi.FlashFirmwareDownloadMode(logger, dev.ID.String(), fwFiles.CWE, fwFiles.NVU); err != nil {
					return fmt.Errorf("flashing firmware: %w", err)
				}
			} else {
				// Extract carrier name from NVU — must match exact case
				// or the IMSWITCH LPM voter holds the modem in low-power.
				priID, err := firmware.ExtractPRIID(fwFiles.NVU)
				if err != nil {
					return fmt.Errorf("extracting PRI ID from NVU: %w", err)
				}
				logger.Debugf("NVU carrier: %s, PRI: %s rev %s", priID.Carrier, priID.PartNumber, priID.Revision)

				device, mbim := controlDevice(dev)
				if device == "" {
					return fmt.Errorf("no QMI control device found — cannot flash firmware")
				}

				if err := qmi.FlashFirmware(logger, device, mbim, priID.Carrier, fwFiles.CWE, fwFiles.NVU); err != nil {
					return fmt.Errorf("flashing firmware: %w", err)
				}
			}

			logger.Success("Firmware flashed successfully")

			// Wait for modem to come back and verify it's online
			logger.Step("Waiting for modem to reboot after flash...")
			if _, err := waitForModemReady(180 * time.Second); err != nil {
				return fmt.Errorf("modem not ready after flash: %w", err)
			}
		}

		logger.Success("Flash workflow complete!")
		return nil
	},
}

// flashDryRun shows what would happen without executing anything.
func flashDryRun(dev *modem.Device) error {
	logger.Infof("DRY RUN — pass --no-dry-run to execute")
	logger.Infof("")

	// Read current firmware version for context
	if dev.ATPort != "" {
		port, err := modem.OpenPort(dev.ATPort, logger)
		if err == nil {
			info := modem.GetConfigSummary(port)
			port.Close()
			if info != nil {
				logger.Infof("Current state:")
				if info.Firmware.Current.Version != "" {
					logger.Infof("  Firmware:  %s", info.Firmware.Current.Version)
				}
				if info.Firmware.Current.CarrierName != "" {
					logger.Infof("  Carrier:   %s", info.Firmware.Current.CarrierName)
				}
				logger.Infof("")
			}
		}
	}

	step := 1

	logger.Infof("  %d. Stop ModemManager", step)
	step++

	logger.Infof("  %d. Download firmware", step)
	if !flagSkipDownload {
		switch {
		case flagFirmwareURL != "":
			logger.Infof("     Source: %s", flagFirmwareURL)
		case flagLegacy:
			logger.Infof("     Source: legacy (%s)", firmware.LegacyFirmware.Filename)
		default:
			logger.Infof("     Source: latest from Sierra Wireless")
		}
		logger.Infof("     Extract .cwe and .nvu files")
	} else {
		logger.Infof("     Use existing local firmware files (--skip-download)")
	}
	step++

	if !flagSkipFlash {
		if dev.Bootloader {
			logger.Infof("  %d. Flash firmware via qmi-firmware-update --update-download", step)
			logger.Infof("     Modem is in bootloader mode — flashing directly")
			logger.Infof("     Wait for modem to come online")
		} else {
			logger.Infof("  %d. Flash firmware via qmi-firmware-update --update", step)
			logger.Infof("     Set firmware preference (version + carrier from NVU)")
			logger.Infof("     Reset to bootloader → flash → reboot")
			logger.Infof("     Wait for modem to come online")
		}
		step++
	}

	logger.Infof("  %d. Restart ModemManager", step)
	return nil
}

// waitForModemReady waits for the modem to appear in sysfs and reach online
// operating mode. Handles USB re-enumeration by re-detecting the modem when
// control device paths go stale.
//
// Recovery sequence for low-power lockup:
//  1. Try QMI set-operating-mode=online
//  2. If that fails (InvalidTransition), set firmware preference via AT commands
//     (IMPREF/GOBIIMPREF) — after a fresh flash the preference may not match
//     the new firmware, keeping the modem in low-power.
//  3. QMI reset and wait for modem to reappear.
func waitForModemReady(timeout time.Duration) (*modem.Device, error) {
	time.Sleep(3 * time.Second) // let modem start rebooting
	deadline := time.Now().Add(timeout)
	spin := logger.StartSpinner("Waiting for modem to come back online...")
	defer spin.Stop()

	var (
		dev         *modem.Device
		lastMode    string
		lastErr     string
		setOnlineAt time.Time // when we last tried QMI SetOnline
	)

	for {
		if time.Now().After(deadline) {
			if lastMode != "" {
				return dev, fmt.Errorf("timed out waiting for modem (last mode: %s)", lastMode)
			}
			return dev, fmt.Errorf("timed out waiting for modem to appear")
		}

		// (Re-)detect modem in sysfs — device paths can change during USB re-enumeration
		detected, err := modem.Detect(logger)
		if err != nil {
			spin.Update("Waiting for modem to appear in sysfs...")
			time.Sleep(3 * time.Second)
			continue
		}
		dev = detected

		// Check for QMI control device
		device, mbim := controlDevice(dev)
		if device == "" {
			spin.Update("Waiting for QMI control device...")
			time.Sleep(2 * time.Second)
			continue
		}

		// Query operating mode via QMI
		mode, err := qmi.GetOperatingMode(logger, device, mbim)
		if err != nil {
			errMsg := err.Error()
			if errMsg != lastErr {
				logger.Debugf("operating mode check: %v", err)
				lastErr = errMsg
			}
			time.Sleep(2 * time.Second)
			continue
		}

		if mode == "online" {
			return dev, nil
		}

		if mode != lastMode {
			spin.Update(fmt.Sprintf("Waiting for modem (mode: %s)...", mode))
			lastMode = mode
		}

		// Actively bring modem out of low-power/offline. Retry every 10s
		// in case the first attempt gets InvalidTransition while modem is
		// still initializing.
		if (mode == "low-power" || mode == "offline") && time.Since(setOnlineAt) > 10*time.Second {
			logger.Debugf("modem in %s mode, attempting QMI set-online", mode)
			if setErr := qmi.SetOnline(logger, device, mbim); setErr != nil {
				logger.Debugf("QMI set-online failed: %v", setErr)
			}
			setOnlineAt = time.Now()
		}

		time.Sleep(2 * time.Second)
	}
}

// controlDevice returns the QMI control device path and whether it uses MBIM.
// For cdc-wdm devices, the transport is determined by checking the sysfs
// driver: cdc_mbim → MBIM, qmi_wwan → raw QMI. This is needed so external
// tools (qmi-firmware-update, qmicli) get the correct --device-open-mbim flag.
func controlDevice(dev *modem.Device) (string, bool) {
	if dev.CDCDevice != "" {
		return dev.CDCDevice, qmux.IsMBIMDevice(dev.CDCDevice)
	}
	if dev.QCQMIDevice != "" {
		return dev.QCQMIDevice, false
	}
	return "", false
}

func init() {
	flashCmd.Flags().StringVar(&flagFirmwareURL, "firmware-url", "", "override firmware download URL")
	flashCmd.Flags().BoolVar(&flagLegacy, "legacy", false, "use legacy stable firmware")
	flashCmd.Flags().BoolVar(&flagSkipDownload, "skip-download", false, "skip firmware download")
	flashCmd.Flags().BoolVar(&flagSkipFlash, "skip-flash", false, "skip firmware flash")

	rootCmd.AddCommand(flashCmd)
}

func completeUSBMode(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"mbim\tMBIM mode (diag, nmea, modem, mbim)",
		"qmi\tQMI mode (diag, nmea, modem, rmnet0)",
	}, cobra.ShellCompDirectiveNoFileComp
}

func completeUSBSpeed(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"2.0\tHigh Speed USB 2.0",
		"3.0\tSuperSpeed USB 3.0",
	}, cobra.ShellCompDirectiveNoFileComp
}

func completeBands(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"lte\tLTE bands only",
		"all\tAll bands (LTE + 3G + 2G)",
	}, cobra.ShellCompDirectiveNoFileComp
}

func completeFastEnum(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"0\tDisable fast enumeration",
		"1\tFast enum on cold boot only",
		"2\tFast enum on warm boot only (recommended)",
		"3\tFast enum on warm and cold boot",
	}, cobra.ShellCompDirectiveNoFileComp
}
