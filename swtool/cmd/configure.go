package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/spf13/cobra"
)

var (
	flagCfgMode     string
	flagCfgUSBSpeed string
	flagCfgBands    string
	flagCfgFastEnum int
	flagCfgVendor   string
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Apply modem settings (vendor identity, USB mode, bands)",
	Long: `Configures the modem via AT commands without re-flashing firmware.
Use --vendor to change the modem's USB identity (dell, lenovo, sierra).
Use --mode to switch between MBIM and QMI USB composition.
Use --bands to set RAT mode and band selection.

Band selection examples:
  --bands lte          LTE only, all LTE bands
  --bands all          All RATs (2G+3G+4G), all bands
  --bands 3g           3G only
  --bands lte:3,7,20   LTE only, restrict to bands 3, 7, and 20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()

		if dev.ATPort == "" {
			return fmt.Errorf("no AT port found for modem %s", dev.Name)
		}

		// Build settings only for flags that were explicitly passed.
		cfg := modem.ConfigureSettings{
			Platform: dev.Platform,
			XACTMode: -1,
		}
		changed := cmd.Flags().Changed

		if dev.Platform == modem.PlatformIntel {
			// Intel: only --bands is supported for configure
			if changed("vendor") || changed("mode") || changed("usb-speed") || changed("fast-enum") {
				return fmt.Errorf("--vendor, --mode, --usb-speed, and --fast-enum are not supported on Intel modems; use --bands")
			}
			if changed("bands") {
				mode, bands, err := resolveXACTBands(flagCfgBands)
				if err != nil {
					return err
				}
				cfg.XACTMode = mode
				cfg.XACTBands = bands
				cfg.SetXACT = true
			}
		} else {
			// Qualcomm
			if changed("vendor") {
				v, ok := modem.Vendors[flagCfgVendor]
				if !ok {
					return fmt.Errorf("unknown vendor %q (valid: sierra, dell, lenovo)", flagCfgVendor)
				}
				cfg.USBVID = v.VID
				cfg.USBPID = fmt.Sprintf("%s,%s", v.PIDApp, v.PIDBoot)
				cfg.USBProduct = v.Product
			}
			if changed("mode") {
				cfg.USBComp = resolveUSBComposition(flagCfgMode).ATValue
			}
			if changed("bands") {
				cfg.SelRat, cfg.Band = resolveBandsQualcomm(flagCfgBands)
			}
			if changed("usb-speed") {
				cfg.USBSpeed = resolveUSBSpeed(flagCfgUSBSpeed)
				cfg.SetUSBSpeed = true
			}
			if changed("fast-enum") {
				cfg.FastEnumEN = flagCfgFastEnum
				cfg.SetFastEnum = true
			}
		}

		cmds := cfg.ATCommands()
		if len(cmds) == 0 {
			return fmt.Errorf("no settings to apply — pass at least one flag (--bands, --vendor, --mode, --usb-speed, --fast-enum)")
		}

		logger.Step(fmt.Sprintf("Configuring %s (%s)...", dev.Name, dev.ID))

		if isDryRun() {
			logger.Infof("")
			logger.Infof("DRY RUN — pass --no-dry-run to execute")
			logger.Infof("")
			logger.Infof("Would apply the following AT commands:")
			for _, c := range cmds {
				logger.Infof("  %s", c)
			}
			if dev.Platform != modem.PlatformIntel {
				logger.Infof("  AT!RESET")
			}
			return nil
		}

		port, err := modem.OpenPortForDevice(dev, logger)
		if err != nil {
			return err
		}
		defer port.Close()

		if err := modem.ApplySettings(port, cfg); err != nil {
			return fmt.Errorf("applying settings: %w", err)
		}

		logger.Success("Settings applied.")
		if dev.Platform != modem.PlatformIntel {
			logger.Infof("Resetting modem...")
			modem.ResetModem(port)
		}
		return nil
	},
}

func init() {
	configureCmd.Flags().StringVar(&flagCfgVendor, "vendor", "", "modem vendor identity (sierra, dell, lenovo)")
	configureCmd.Flags().StringVar(&flagCfgMode, "mode", "mbim", "USB composition mode (mbim, qmi)")
	configureCmd.Flags().StringVar(&flagCfgUSBSpeed, "usb-speed", "2.0", "USB interface speed")
	configureCmd.Flags().StringVar(&flagCfgBands, "bands", "", "band selection (lte, all, 3g, lte:3,7,20)")
	configureCmd.Flags().IntVar(&flagCfgFastEnum, "fast-enum", 2, "fast enumeration mode (0-3)")

	configureCmd.RegisterFlagCompletionFunc("vendor", completeVendor)
	configureCmd.RegisterFlagCompletionFunc("mode", completeUSBMode)
	configureCmd.RegisterFlagCompletionFunc("usb-speed", completeUSBSpeed)
	configureCmd.RegisterFlagCompletionFunc("bands", completeBands)
	configureCmd.RegisterFlagCompletionFunc("fast-enum", completeFastEnum)

	rootCmd.AddCommand(configureCmd)
}

func completeVendor(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"sierra\tSierra Wireless EM7455", "dell\tDell DW5811e", "lenovo\tLenovo EM7455"}, cobra.ShellCompDirectiveNoFileComp
}

// resolveUSBComposition converts the user-facing flag value to a USBComposition.
func resolveUSBComposition(mode string) modem.USBComposition {
	switch mode {
	case "qmi":
		return modem.CompQMI
	default:
		return modem.CompMBIM
	}
}

// resolveBandsQualcomm converts the user-facing flag value to SELRAT and BAND AT values
// for Qualcomm-based modems (AT!SELRAT + AT!BAND).
func resolveBandsQualcomm(bands string) (selrat, band string) {
	switch bands {
	case "all":
		return "00", "00"
	default:
		return "06", "09"
	}
}

// resolveXACTBands parses the --bands flag for Intel XMM modems and returns
// the AT+XACT mode and band list.
//
// Supported formats:
//
//	"lte"         → mode 2, no band list (all LTE bands)
//	"all"         → mode 6, no band list (all bands)
//	"3g"          → mode 1, no band list (all 3G bands)
//	"2g"          → mode 0, no band list (all 2G bands)
//	"lte:3,7,20"  → mode 2, bands "103,107,120"
//	"3g:1,2,5"    → mode 1, bands "1,2,5"
//	"all:1,2,101" → mode 6, raw band numbers passed through
func resolveXACTBands(spec string) (mode int, bands string, err error) {
	rat, bandSpec, _ := strings.Cut(spec, ":")

	switch strings.ToLower(rat) {
	case "lte", "4g":
		mode = 2
	case "3g", "umts", "wcdma":
		mode = 1
	case "2g", "gsm":
		mode = 0
	case "all":
		mode = 6
	default:
		return 0, "", fmt.Errorf("unknown band preset %q (valid: lte, 3g, 2g, all; e.g. --bands lte:3,7,20)", rat)
	}

	if bandSpec == "" {
		return mode, "", nil
	}

	// Parse band numbers and convert to AT+XACT encoding
	var parts []string
	for _, s := range strings.Split(bandSpec, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return 0, "", fmt.Errorf("invalid band number %q in %q", s, spec)
		}
		switch {
		case mode == 2 || mode == 6:
			// LTE bands: user specifies band number (e.g. 3), encode as 100+N
			if n < 100 {
				n += 100
			}
			parts = append(parts, strconv.Itoa(n))
		default:
			// 3G/2G bands: pass through as-is
			parts = append(parts, strconv.Itoa(n))
		}
	}

	return mode, strings.Join(parts, ","), nil
}

// resolveUSBSpeed converts the user-facing flag value to the AT!USBSPEED value.
func resolveUSBSpeed(speed string) int {
	switch speed {
	case "3.0":
		return 1
	default:
		return 0
	}
}
