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

		// Build settings only for flags that were explicitly passed.
		var cfg modem.ConfigureSettings
		changed := cmd.Flags().Changed

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
			cfg.SelRat, cfg.Band = resolveBands(flagCfgBands)
		}
		if changed("usb-speed") {
			cfg.USBSpeed = resolveUSBSpeed(flagCfgUSBSpeed)
			cfg.SetUSBSpeed = true
		}
		if changed("fast-enum") {
			cfg.FastEnumEN = flagCfgFastEnum
			cfg.SetFastEnum = true
		}

		cmds := cfg.ATCommands()
		if len(cmds) == 0 {
			return fmt.Errorf("no settings to apply — pass at least one flag (--vendor, --mode, --bands, --usb-speed, --fast-enum)")
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
			logger.Infof("  AT!RESET")
			return nil
		}

		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			return err
		}
		defer port.Close()

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
