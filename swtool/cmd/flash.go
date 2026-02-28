package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/firmware"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/danielewood/sierra-wireless-modems/swtool/qmi"
	"github.com/spf13/cobra"
)

var (
	flagUSBMode       string
	flagUSBSpeed      string
	flagBands         string
	flagFastEnum      int
	flagFirmwareURL   string
	flagLegacy        bool
	flagSkipDownload  bool
	flagSkipFlash     bool
	flagSkipConfigure bool
)

var flashCmd = &cobra.Command{
	Use:   "flash",
	Short: "Full flash workflow: download + flash + configure",
	Long: `Downloads the latest firmware from Sierra Wireless, flashes it onto the modem,
and configures the modem with generic VID/PID, band settings, and USB mode.

This is the default command when swtool is run without a subcommand.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()
		comp := resolveUSBComposition(flagUSBMode)
		selrat, band := resolveBands(flagBands)
		usbSpeed := resolveUSBSpeed(flagUSBSpeed)

		logger.Step(fmt.Sprintf("Starting flash workflow for %s (%s)", dev.Name, dev.ID))

		if isDryRun() {
			return flashDryRun(dev, comp, selrat, band, usbSpeed)
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
			// Extract PRI ID from .nvu for later configuration.
			// The carrier name must be passed to qmi-firmware-update with
			// exact case matching the NVU binary — a case mismatch causes
			// the IMSWITCH LPM voter to hold the modem in low-power mode.
			priID, err := firmware.ExtractPRIID(fwFiles.NVU)
			if err != nil {
				return fmt.Errorf("extracting PRI ID from NVU: %w", err)
			}
			logger.Debugf("NVU carrier: %s, PRI: %s rev %s", priID.Carrier, priID.PartNumber, priID.Revision)

			// Pre-flash: set power management and clear firmware images.
			// PCOFFEN=2 and FASTENUMEN survive the flash and prevent the modem
			// from getting stuck in low-power mode after rebooting.
			if dev.ATPort == "" {
				return fmt.Errorf("no AT port found — cannot prepare modem for flash")
			}
			port, err := modem.OpenPort(dev.ATPort, logger)
			if err != nil {
				return fmt.Errorf("opening AT port for pre-flash setup: %w", err)
			}
			if err := modem.PreFlashSetup(port, flagFastEnum); err != nil {
				port.Close()
				return fmt.Errorf("pre-flash setup: %w", err)
			}
			port.Close()

			// Flash firmware using --update, which handles the full lifecycle:
			// set firmware preference → reset to bootloader → flash → reboot.
			// This is critical for 9x30 devices — without updating the firmware
			// preference, the modem boots into low-power after flash due to
			// firmware version mismatch.
			device, _ := controlDevice(dev)
			if device == "" {
				return fmt.Errorf("no QMI control device found — cannot flash firmware")
			}

			if err := qmi.FlashFirmware(logger, device, priID.Carrier, fwFiles.CWE, fwFiles.NVU); err != nil {
				return fmt.Errorf("flashing firmware: %w", err)
			}

			logger.Success("Firmware flashed successfully")

			// Wait for modem to come back and verify it's online
			logger.Step("Waiting for modem to reboot after flash...")
			dev, err = waitForModemReady(180 * time.Second)
			if err != nil {
				return fmt.Errorf("modem not ready after flash: %w", err)
			}

			// Step 5: Configure modem settings
			if !flagSkipConfigure {
				if dev.ATPort == "" {
					return fmt.Errorf("no AT port found — cannot configure modem")
				}

				port, err := modem.OpenPort(dev.ATPort, logger)
				if err != nil {
					return fmt.Errorf("opening AT port for configuration: %w", err)
				}

				cfg := modem.ConfigureSettings{
					USBComp:       comp.ATValue,
					USBVID:        modem.Vendors["sierra"].VID,
					USBPID:        fmt.Sprintf("%s,%s", modem.Vendors["sierra"].PIDApp, modem.Vendors["sierra"].PIDBoot),
					USBProduct:    modem.Vendors["sierra"].Product,
					PRIIDCustomer: "Generic-Laptop",
					SelRat:        selrat,
					Band:          band,
					FastEnumEN:    flagFastEnum,
					USBSpeed:      usbSpeed,
				}

				if priID != nil {
					cfg.PRIIDPartNum = priID.PartNumber
					cfg.PRIIDRev = priID.Revision
				}

				if err := modem.ApplySettings(port, cfg); err != nil {
					return fmt.Errorf("applying settings: %w", err)
				}
				port.Close()

				// Reset via QMI to apply settings
				device, mbim := controlDevice(dev)
				if device != "" {
					logger.Step("Resetting modem to apply settings...")
					if err := qmi.ResetViaQMI(logger, device, mbim); err != nil {
						logger.Warnf("QMI reset failed: %v (settings may need manual reboot)", err)
					} else {
						dev, err = waitForModemReady(120 * time.Second)
						if err != nil {
							logger.Warnf("Modem did not come back online after configure: %v", err)
						}
					}
				}
			}
		}

		logger.Success("Flash workflow complete!")
		return nil
	},
}

// flashDryRun shows what would happen without executing anything.
// Reads current modem settings to show before → after for each change.
func flashDryRun(dev *modem.Device, comp modem.USBComposition, selrat, band string, usbSpeed int) error {
	logger.Infof("DRY RUN — pass --no-dry-run to execute")
	logger.Infof("")

	// Read current settings for before/after comparison (lightweight — ~8 AT commands)
	var info *modem.Info
	if dev.ATPort != "" {
		port, err := modem.OpenPort(dev.ATPort, logger)
		if err == nil {
			info = modem.GetConfigSummary(port)
			port.Close()
		}
	}

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
		logger.Infof("  %d. Pre-flash setup", step)
		logger.Infof("     Set PCOFFEN=2 (ignore W_DISABLE pin — prevents low-power lockup)")
		logger.Infof("     Set FASTENUMEN=%d (%s)", flagFastEnum, fastEnumDescription(flagFastEnum))
		step++

		logger.Infof("  %d. Flash firmware via qmi-firmware-update --update", step)
		logger.Infof("     Set firmware preference (version + carrier GENERIC)")
		logger.Infof("     Reset to bootloader → flash → reboot")
		logger.Infof("     Wait for modem to come online")
		step++
	}

	if !flagSkipConfigure {
		logger.Infof("  %d. Configure modem settings", step)

		// Identity — make modem appear as generic Sierra Wireless EM7455
		logger.Infof("")
		v := modem.Vendors["sierra"]
		logger.Infof("     USB identity (appear as generic Sierra Wireless EM7455):")
		if info != nil {
			showChange("       VID", info.USB.VID, v.VID)
			showChange("       PID", info.USB.PID.App+","+info.USB.PID.Boot, v.PIDApp+","+v.PIDBoot)
			showChange("       Product", info.USB.Product, v.Product)
			// ConfigType may include description like "1 (Generic)" — strip to just the number
			cfgType := strings.SplitN(info.USB.Composition.ConfigType, " ", 2)[0]
			curComp := fmt.Sprintf("%d,%s,%s", info.USB.Composition.ConfigIndex,
				cfgType, info.USB.Composition.Bitmask)
			showChange("       USBCOMP", curComp, comp.ATValue)
		} else {
			logger.Infof("       VID → 1199, PID → 9071,9070, Product → EM7455")
			logger.Infof("       USBCOMP → %s", comp.ATValue)
		}

		// Carrier preference — unlock all carriers
		logger.Infof("")
		logger.Infof("     Carrier preference (unlock from OEM carrier lock):")
		if info != nil {
			showChange("       Carrier", info.Firmware.Current.CarrierName, "GENERIC")
		} else {
			logger.Infof("       Carrier → GENERIC")
		}

		// Network — RAT and band selection
		selratName := selratDescription(selrat)
		bandName := bandDescription(band)
		logger.Infof("")
		logger.Infof("     Network (RAT and band selection):")
		if info != nil {
			curSelrat := fmt.Sprintf("%02d", info.Network.RATSelection.Index)
			if info.Network.RATSelection.Name != "" {
				curSelrat += " (" + info.Network.RATSelection.Name + ")"
			}
			showChange("       SELRAT", curSelrat, selrat+" ("+selratName+")")

			curBand := fmt.Sprintf("%02d", info.Network.CurrentBand.Index)
			if info.Network.CurrentBand.Name != "" {
				curBand += " (" + info.Network.CurrentBand.Name + ")"
			}
			showChange("       BAND", curBand, band+" ("+bandName+")")
		} else {
			logger.Infof("       SELRAT → %s (%s)", selrat, selratName)
			logger.Infof("       BAND → %s (%s)", band, bandName)
		}

		// Power management
		logger.Infof("")
		logger.Infof("     Power management:")
		if info != nil {
			curFastEnum := customHexToDecimal(info.Custom["FASTENUMEN"])
			showChange("       FASTENUMEN", curFastEnum, fmt.Sprintf("%d — %s", flagFastEnum, fastEnumDescription(flagFastEnum)))
			showChange("       PCOFFEN", fmt.Sprintf("%d", info.Power.PCOFFEN), "2 — ignore W_DISABLE pin")
		} else {
			logger.Infof("       FASTENUMEN → %d — %s", flagFastEnum, fastEnumDescription(flagFastEnum))
			logger.Infof("       PCOFFEN → 2 — ignore W_DISABLE pin")
		}

		// USB speed
		targetSpeed := usbSpeedName(usbSpeed)
		logger.Infof("")
		logger.Infof("     USB interface speed:")
		if info != nil && info.USB.Speed.Current != "" {
			showChange("       USBSPEED", info.USB.Speed.Current, targetSpeed)
		} else {
			logger.Infof("       USBSPEED → %s", targetSpeed)
		}

		logger.Infof("")
		step++
	}

	logger.Infof("  %d. Restart ModemManager", step)
	return nil
}

// customHexToDecimal converts a hex custom value like "0x02" to decimal "2".
// Returns "(not set)" if the value is empty or unparseable.
func customHexToDecimal(hex string) string {
	hex = strings.TrimSpace(hex)
	if hex == "" {
		return "(not set)"
	}
	trimmed := strings.TrimPrefix(hex, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	var val int64
	if _, err := fmt.Sscanf(trimmed, "%x", &val); err == nil {
		return fmt.Sprintf("%d", val)
	}
	return hex
}

// selratDescription returns a human-readable description for a SELRAT value.
func selratDescription(selrat string) string {
	switch selrat {
	case "00":
		return "automatic"
	case "01":
		return "GSM only"
	case "02":
		return "WCDMA only"
	case "06":
		return "LTE only"
	default:
		return "unknown"
	}
}

// bandDescription returns a human-readable description for a BAND index.
func bandDescription(band string) string {
	switch band {
	case "00":
		return "all bands"
	case "09":
		return "LTE all"
	default:
		return "custom"
	}
}

// fastEnumDescription returns a human-readable description for FASTENUMEN.
func fastEnumDescription(val int) string {
	switch val {
	case 0:
		return "disabled"
	case 1:
		return "cold boot only"
	case 2:
		return "warm boot only"
	case 3:
		return "warm and cold boot"
	default:
		return "unknown"
	}
}

// usbSpeedName returns the human-readable name for a USBSPEED AT value.
func usbSpeedName(speed int) string {
	if speed == 1 {
		return "Super-Speed"
	}
	return "High-Speed"
}

// showChange logs a before → after line, highlighting when values differ.
func showChange(label, before, after string) {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	if before == after {
		logger.Infof("%s: %s (no change)", label, after)
	} else {
		logger.Infof("%s: %s → %s", label, before, after)
	}
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
func controlDevice(dev *modem.Device) (string, bool) {
	if dev.CDCDevice != "" {
		return dev.CDCDevice, true
	}
	if dev.QCQMIDevice != "" {
		return dev.QCQMIDevice, false
	}
	return "", false
}

func init() {
	flashCmd.Flags().StringVar(&flagUSBMode, "usb-mode", "mbim", "USB composition mode")
	flashCmd.Flags().StringVar(&flagUSBSpeed, "usb-speed", "2.0", "USB interface speed")
	flashCmd.Flags().StringVar(&flagBands, "bands", "lte", "band selection")
	flashCmd.Flags().IntVar(&flagFastEnum, "fast-enum", 2, "fast enumeration mode (0-3)")
	flashCmd.Flags().StringVar(&flagFirmwareURL, "firmware-url", "", "override firmware download URL")
	flashCmd.Flags().BoolVar(&flagLegacy, "legacy", false, "use legacy stable firmware")
	flashCmd.Flags().BoolVar(&flagSkipDownload, "skip-download", false, "skip firmware download")
	flashCmd.Flags().BoolVar(&flagSkipFlash, "skip-flash", false, "skip firmware flash")
	flashCmd.Flags().BoolVar(&flagSkipConfigure, "skip-configure", false, "skip modem configuration")

	// Register completions for enum flags
	flashCmd.RegisterFlagCompletionFunc("usb-mode", completeUSBMode)
	flashCmd.RegisterFlagCompletionFunc("usb-speed", completeUSBSpeed)
	flashCmd.RegisterFlagCompletionFunc("bands", completeBands)
	flashCmd.RegisterFlagCompletionFunc("fast-enum", completeFastEnum)

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
