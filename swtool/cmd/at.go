package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/spf13/cobra"
)

var (
	flagATInteractive bool
	flagATTimeout     time.Duration
)

// dangerousATCommands lists AT command prefixes that mutate modem state.
// Query forms (=? and ?) are always safe and excluded by isDangerousAT.
var dangerousATCommands = []string{
	// Modem control
	"AT!RESET",
	"AT!BOOTHOLD",
	"AT&F",
	"AT+CFUN=",
	// USB identity
	"AT!USBCOMP=",
	"AT!USBVID=",
	"AT!USBPID=",
	"AT!USBPRODUCT=",
	"AT!USBSPEED=",
	// Firmware & carrier
	"AT!IMAGE=",
	"AT!IMPREF=",
	"AT!GOBIIMPREF=",
	"AT!PRIID=",
	"AT!CUSTOM=",
	"AT!ENTERCND=",
	// Network
	"AT!SELRAT=",
	"AT!BAND=",
	"AT!LTECA=",
	// Power
	"AT!PCOFFEN=",
	// GPS
	"AT!GPSFIX=",
	// Factory reset
	"AT!RMARESET=",
	"AT!NVRESTORE=",
	// PDP / APN
	"AT+CGDCONT=",
	// SMS
	"AT+CMGS=",
	// AirVantage OTA
	"AT+WDSC=",
	"AT+WDSS=",
}

// isDangerousAT returns true if cmd matches a dangerous AT command prefix.
// Query forms ending in =? or ? are always considered safe.
func isDangerousAT(cmd string) bool {
	upper := strings.ToUpper(strings.TrimSpace(cmd))

	// Query forms are always safe
	if strings.HasSuffix(upper, "=?") || (strings.HasSuffix(upper, "?") && !strings.Contains(upper, "=")) {
		return false
	}

	for _, prefix := range dangerousATCommands {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

var atCmd = &cobra.Command{
	Use:   "at [commands...]",
	Short: "Send raw AT command(s) to the modem",
	Long: `Send one or more AT commands directly to the modem and print responses.

Examples:
  swtool at "ATI"
  swtool at "AT!BAND?" "AT!IMAGE?"
  swtool at --interactive`,
	Args: cobra.ArbitraryArgs,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return modem.CommonATCommands, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()

		if dev.ATPort == "" {
			return fmt.Errorf("no AT port found for modem %s", dev.Name)
		}

		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			return err
		}
		defer port.Close()

		if flagATTimeout > 0 {
			port.SetTimeout(flagATTimeout)
		}

		if flagATInteractive {
			return runInteractive(port)
		}

		if len(args) == 0 {
			return fmt.Errorf("provide AT commands as arguments, or use --interactive")
		}

		// Fix shell-escaped '!' (bash history expansion: \! → !)
		for i, atcmd := range args {
			args[i] = strings.ReplaceAll(atcmd, `\!`, "!")
		}

		// Auto-enter engineering mode if any command uses AT! prefix
		for _, atcmd := range args {
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(atcmd)), "AT!") {
				modem.EnterCommandMode(port)
				break
			}
		}

		for _, atcmd := range args {
			if isDryRun() && isDangerousAT(atcmd) {
				logger.Warnf("DRY RUN — blocked dangerous command: %s", atcmd)
				logger.Infof("  Pass --no-dry-run to execute mutating commands")
				continue
			}
			resp, err := port.SendCommand(atcmd)
			if err != nil {
				logger.Errorf("%s: %v", atcmd, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "%s\n", strings.TrimSpace(resp))
		}

		return nil
	},
}

func init() {
	atCmd.Flags().BoolVarP(&flagATInteractive, "interactive", "i", false, "interactive REPL mode")
	atCmd.Flags().DurationVarP(&flagATTimeout, "timeout", "t", 0, "AT command timeout (e.g. 60s, 2m); default 10s")
	rootCmd.AddCommand(atCmd)
}

func runInteractive(port *modem.Port) error {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Fprintln(os.Stderr, "Interactive AT mode. Type 'quit' or Ctrl-D to exit.")
	if isDryRun() {
		fmt.Fprintln(os.Stderr, "  DRY RUN active — dangerous commands will be blocked.")
		fmt.Fprintln(os.Stderr, "  Restart with --no-dry-run to send all commands.")
	}
	fmt.Fprintln(os.Stderr, "")

	for {
		fmt.Fprint(os.Stderr, "AT> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "quit") || strings.EqualFold(line, "exit") {
			break
		}

		if isDryRun() && isDangerousAT(line) {
			fmt.Fprintf(os.Stderr, "  BLOCKED (dry-run): %s\n", line)
			continue
		}

		resp, err := port.SendCommand(line)
		if err != nil {
			logger.Errorf("%v", err)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s\n", strings.TrimSpace(resp))
	}

	fmt.Fprintln(os.Stderr, "\nExiting.")
	return nil
}
