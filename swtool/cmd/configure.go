package cmd

import (
	"fmt"

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
Use --mode to switch between MBIM and QMI USB composition.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()

		if dev.ATPort == "" {
			return fmt.Errorf("no AT port found for modem %s", dev.Name)
		}

		// Validate vendor flag if set
		var vendor *modem.VendorProfile
		if flagCfgVendor != "" {
			v, ok := modem.Vendors[flagCfgVendor]
			if !ok {
				return fmt.Errorf("unknown vendor %q (valid: sierra, dell, lenovo)", flagCfgVendor)
			}
			vendor = &v
		}

		comp := resolveUSBComposition(flagCfgMode)
		selrat, band := resolveBands(flagCfgBands)
		usbSpeed := resolveUSBSpeed(flagCfgUSBSpeed)

		logger.Step(fmt.Sprintf("Configuring %s (%s)...", dev.Name, dev.ID))
		if vendor != nil {
			logger.Infof("  Vendor:      %s (VID=%s, PID=%s/%s)", flagCfgVendor, vendor.VID, vendor.PIDApp, vendor.PIDBoot)
		}
		logger.Infof("  USB mode:    %s", comp.Description)
		logger.Infof("  USB speed:   %s", flagCfgUSBSpeed)
		logger.Infof("  Bands:       %s (SELRAT=%s, BAND=%s)", flagCfgBands, selrat, band)
		logger.Infof("  Fast enum:   %d", flagCfgFastEnum)

		if isDryRun() {
			logger.Infof("")
			logger.Infof("DRY RUN — pass --no-dry-run to execute")
			logger.Infof("")
			logger.Infof("Would apply the following AT commands:")
			logger.Infof("  AT!USBCOMP=%s", comp.ATValue)
			if vendor != nil {
				logger.Infof("  AT!USBVID=%s", vendor.VID)
				logger.Infof("  AT!USBPID=%s,%s", vendor.PIDApp, vendor.PIDBoot)
				logger.Infof("  AT!USBPRODUCT=\"%s\"", vendor.Product)
			}
			logger.Infof("  AT!SELRAT=%s", selrat)
			logger.Infof("  AT!BAND=%s", band)
			logger.Infof("  AT!CUSTOM=\"FASTENUMEN\",%d", flagCfgFastEnum)
			logger.Infof("  AT!USBSPEED=%d", usbSpeed)
			logger.Infof("  AT!RESET")
			return nil
		}

		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			return err
		}
		defer port.Close()

		cfg := modem.ConfigureSettings{
			USBComp:       comp.ATValue,
			SelRat:        selrat,
			Band:          band,
			FastEnumEN:    flagCfgFastEnum,
			USBSpeed:      usbSpeed,
			PRIIDCustomer: "Generic-Laptop",
		}

		if vendor != nil {
			cfg.USBVID = vendor.VID
			cfg.USBPID = fmt.Sprintf("%s,%s", vendor.PIDApp, vendor.PIDBoot)
			cfg.USBProduct = vendor.Product
		}

		if err := modem.ApplySettings(port, cfg); err != nil {
			return fmt.Errorf("applying settings: %w", err)
		}

		logger.Success("Settings applied. Modem is resetting...")
		return nil
	},
}

func init() {
	configureCmd.Flags().StringVar(&flagCfgVendor, "vendor", "", "modem vendor identity (sierra, dell, lenovo)")
	configureCmd.Flags().StringVar(&flagCfgMode, "mode", "mbim", "USB composition mode (mbim, qmi)")
	configureCmd.Flags().StringVar(&flagCfgUSBSpeed, "usb-speed", "2.0", "USB interface speed")
	configureCmd.Flags().StringVar(&flagCfgBands, "bands", "lte", "band selection")
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

// resolveBands converts the user-facing flag value to SELRAT and BAND AT values.
func resolveBands(bands string) (selrat, band string) {
	switch bands {
	case "all":
		return "00", "00"
	default:
		return "06", "09"
	}
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
