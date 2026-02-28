package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
	"github.com/spf13/cobra"
)

const udevRulePath = "/etc/udev/rules.d/99-sierra-wireless.rules"

const udevRuleContent = `# Sierra Wireless EM7455/MC7455/EM7565 modem access for dialout group
# Installed by: swtool setup

# Serial ports (ttyUSB)
SUBSYSTEM=="tty", ATTRS{idVendor}=="1199", MODE="0660", GROUP="dialout"
SUBSYSTEM=="tty", ATTRS{idVendor}=="413c", MODE="0660", GROUP="dialout"

# CDC-WDM devices (MBIM)
SUBSYSTEM=="usb", ATTRS{idVendor}=="1199", MODE="0660", GROUP="dialout"
SUBSYSTEM=="usb", ATTRS{idVendor}=="413c", MODE="0660", GROUP="dialout"

# QMI control devices
SUBSYSTEM=="usbmisc", ATTRS{idVendor}=="1199", MODE="0660", GROUP="dialout"
SUBSYSTEM=="usbmisc", ATTRS{idVendor}=="413c", MODE="0660", GROUP="dialout"
`

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure system for non-root modem access",
	Long: `Creates udev rules and adds your user to the dialout group so that
read-only commands (info, at) work without sudo.

Requires root. After setup, log out and back in for group changes to take effect.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sysutil.RequireRoot(); err != nil {
			return fmt.Errorf("setup requires root: run with sudo")
		}

		// Determine the real user (not root)
		realUser := os.Getenv("SUDO_USER")
		if realUser == "" {
			realUser = os.Getenv("USER")
		}
		if realUser == "" || realUser == "root" {
			logger.Warnf("Could not determine non-root user. Run with: sudo swtool setup")
		}

		// Step 1: Create udev rule
		logger.Step("Creating udev rule...")
		if err := os.WriteFile(udevRulePath, []byte(udevRuleContent), 0644); err != nil {
			return fmt.Errorf("writing udev rule %s: %w", udevRulePath, err)
		}
		logger.Infof("  Created %s", udevRulePath)

		// Step 2: Reload udev rules
		logger.Step("Reloading udev rules...")
		if out, err := sysutil.RunCommand(logger, "udevadm", "control", "--reload-rules"); err != nil {
			logger.Warnf("udevadm reload: %v\n%s", err, out)
		}
		if out, err := sysutil.RunCommand(logger, "udevadm", "trigger"); err != nil {
			logger.Warnf("udevadm trigger: %v\n%s", err, out)
		}

		// Step 3: Add user to dialout group
		if realUser != "" && realUser != "root" {
			logger.Step(fmt.Sprintf("Adding %s to dialout group...", realUser))
			if out, err := sysutil.RunCommand(logger, "usermod", "-aG", "dialout", realUser); err != nil {
				// Check if already in group
				if !strings.Contains(out, "already a member") {
					logger.Warnf("usermod: %v\n%s", err, out)
				}
			}
			logger.Infof("  Added %s to dialout group", realUser)
		}

		logger.Success("Setup complete!")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Log out and back in for group changes to take effect.")
		fmt.Fprintln(os.Stderr, "  Then run: swtool info  (no sudo needed)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Note: flash and configure still require sudo.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
