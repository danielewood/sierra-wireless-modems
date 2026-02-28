// Package cmd implements the cobra CLI commands for swtool.
package cmd

import (
	"os"
	"path/filepath"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	flagVerbose  bool
	flagQuiet    bool
	flagDevice   string
	flagNoDryRun bool

	// Shared state populated by PersistentPreRun
	logger        *log.Logger
	detectedModem *modem.Device
)

// isDryRun returns true when the tool should simulate rather than execute.
// All mutating commands default to dry-run; pass --no-dry-run to execute.
func isDryRun() bool {
	return !flagNoDryRun
}

// rootCmd is the base command when called without subcommands.
var rootCmd = &cobra.Command{
	Use:   "swtool",
	Short: "Sierra Wireless EM7455/MC7455 modem management tool",
	Long: `Autoflash detects, configures, and flashes Sierra Wireless EM7455/MC7455/EM7565
modems. It replaces the legacy bash script with a single binary that handles modem
detection, AT command communication, firmware downloading, and flashing.

Running without a subcommand is equivalent to 'swtool flash'.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logger = log.New(flagVerbose, flagQuiet)

		// Commands that don't need a modem
		if skipModemDetection(cmd) {
			return nil
		}

		logger.Step("Searching for EM7455/MC7455 modem...")

		dev, err := modem.Detect(logger)
		if err != nil {
			return err
		}

		// Allow device override
		if flagDevice != "" {
			dev.ATPort = flagDevice
		}

		logger.Infof("Found %s (%s)", dev.Name, dev.ID)
		if dev.ATPort != "" {
			logger.Debugf("AT port: %s", dev.ATPort)
		}
		if dev.CDCDevice != "" {
			logger.Debugf("CDC device: %s", dev.CDCDevice)
		}

		detectedModem = dev
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default: delegate to flash command
		return flashCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().StringVar(&flagDevice, "device", "", "override AT port path (e.g. /dev/ttyUSB2)")
	rootCmd.PersistentFlags().BoolVar(&flagNoDryRun, "no-dry-run", false, "actually execute changes (default is dry-run)")

	// Dynamic completion for --device
	rootCmd.RegisterFlagCompletionFunc("device", completeDevicePaths)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// skipModemDetection returns true for commands that don't need a modem.
func skipModemDetection(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "completion", "help", "download", "setup":
		return true
	}
	return false
}

// completeDevicePaths provides tab completion for --device flag.
func completeDevicePaths(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var paths []string

	// Scan for ttyUSB devices
	matches, _ := filepath.Glob("/dev/ttyUSB*")
	paths = append(paths, matches...)

	// Scan for CDC-WDM devices
	matches, _ = filepath.Glob("/dev/cdc-wdm*")
	paths = append(paths, matches...)

	// Scan for QCQMI devices
	matches, _ = filepath.Glob("/dev/qcqmi*")
	paths = append(paths, matches...)

	return paths, cobra.ShellCompDirectiveNoFileComp
}

// requireModem returns the detected modem or exits with an error.
func requireModem() *modem.Device {
	if detectedModem == nil {
		logger.Errorf("no modem detected")
		os.Exit(1)
	}
	return detectedModem
}
