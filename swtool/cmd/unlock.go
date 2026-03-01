package cmd

import (
	"fmt"
	"strings"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/keygen"
	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/spf13/cobra"
)

var flagUnlockGeneration string

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Perform AT!OPENLOCK engineering unlock on the modem",
	Long: `Automatically detects the modem generation from the ATI revision string,
retrieves the AT!OPENLOCK challenge, computes the response using the Sierra
Wireless key derivation algorithm, and sends it to unlock the modem.

The unlock enables engineering-level AT commands (AT!ENTERCND alone only
enables a subset). This is equivalent to the sierrakeygen.py -u auto-unlock.

This is a mutating command — runs in dry-run mode by default.
Pass --no-dry-run to actually send the unlock response.`,
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

		// Step 1: Detect generation from ATI response.
		resp, err := port.SendCommand("ATI")
		if err != nil {
			return fmt.Errorf("ATI command failed: %w", err)
		}

		var gen *keygen.Generation
		if flagUnlockGeneration != "" {
			gen, err = keygen.GenerationByName(flagUnlockGeneration)
			if err != nil {
				return fmt.Errorf("unknown generation %q (supported: %s)",
					flagUnlockGeneration, strings.Join(keygen.Generations(), ", "))
			}
			logger.Infof("Using specified generation: %s", gen.Name)
		} else {
			revision, model := parseATIForUnlock(resp)
			if revision == "" {
				return fmt.Errorf("could not parse revision from ATI response")
			}
			logger.Debugf("ATI revision: %s, model: %s", revision, model)

			gen, err = keygen.DetectGeneration(revision, model)
			if err != nil {
				return fmt.Errorf("detecting generation: %w", err)
			}
			logger.Infof("Detected generation: %s", gen.Name)
		}

		// Step 2: Enter engineering command mode.
		logger.Step("Entering engineering command mode...")
		if err := modem.EnterCommandMode(port); err != nil {
			return err
		}

		// Step 3: Get challenge.
		logger.Step("Requesting AT!OPENLOCK challenge...")
		challengeResp, err := port.SendCommand("AT!OPENLOCK?")
		if err != nil {
			return fmt.Errorf("AT!OPENLOCK? failed: %w", err)
		}

		challenge := parseChallenge(challengeResp)
		if challenge == "" {
			return fmt.Errorf("could not parse challenge from AT!OPENLOCK? response: %s", challengeResp)
		}
		logger.Infof("Challenge: %s", challenge)

		// Step 4: Compute response.
		response, err := keygen.Solve(challenge, gen, keygen.KeyOpenLock)
		if err != nil {
			return fmt.Errorf("computing unlock response: %w", err)
		}
		logger.Infof("Response:  %s", response)

		// Step 5: Send unlock response.
		if isDryRun() {
			fmt.Fprintf(cmd.OutOrStdout(), "AT!OPENLOCK=\"%s\"\n", response)
			logger.Warnf("DRY RUN — pass --no-dry-run to send the unlock command")
			return nil
		}

		logger.Step("Sending AT!OPENLOCK response...")
		unlockCmd := fmt.Sprintf(`AT!OPENLOCK="%s"`, response)
		_, err = port.SendCommand(unlockCmd)
		if err != nil {
			return fmt.Errorf("AT!OPENLOCK failed: %w", err)
		}

		logger.Infof("Modem unlocked successfully")
		return nil
	},
}

func init() {
	unlockCmd.Flags().StringVarP(&flagUnlockGeneration, "generation", "g", "",
		"modem generation override (e.g. MDM9x30, MDM9x50)")
	rootCmd.AddCommand(unlockCmd)
}

// parseATIForUnlock extracts the Revision and Model fields from an ATI response.
func parseATIForUnlock(resp string) (revision, model string) {
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			switch key {
			case "Revision":
				revision = val
			case "Model":
				model = val
			}
		}
	}
	return
}

// parseChallenge extracts the hex challenge string from an AT!OPENLOCK? response.
// The response typically looks like:
//
//	AT!OPENLOCK?
//	BE96CBBEE0829BCA
//	OK
func parseChallenge(resp string) string {
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" ||
			strings.HasPrefix(line, "AT!") || strings.HasPrefix(line, "!") {
			continue
		}
		// Challenge is a hex string (typically 16 hex chars = 8 bytes).
		if isHexString(line) && len(line) >= 8 {
			return line
		}
	}
	return ""
}

// isHexString reports whether s contains only hexadecimal characters.
func isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return len(s) > 0
}
