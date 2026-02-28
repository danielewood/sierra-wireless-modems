package cmd

import (
	"fmt"

	"github.com/danielewood/sierra-wireless-modems/swtool/firmware"
	"github.com/spf13/cobra"
)

var (
	flagDlFirmwareURL string
	flagDlOutputDir   string
	flagDlLegacy      bool
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download firmware from Sierra Wireless",
	Long:  `Scrapes the Sierra Wireless firmware page, downloads the latest firmware ZIP, and extracts .cwe/.nvu files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Step("Starting firmware download...")

		var ref *firmware.FirmwareRef

		switch {
		case flagDlFirmwareURL != "":
			ref = &firmware.FirmwareRef{
				URL:      flagDlFirmwareURL,
				Filename: firmware.DeriveFilename(flagDlFirmwareURL),
			}
		case flagDlLegacy:
			ref = &firmware.LegacyFirmware
			logger.Infof("Using legacy firmware: %s", ref.Filename)
		default:
			var err error
			ref, err = firmware.ScrapeLatestURL(logger)
			if err != nil {
				return fmt.Errorf("finding firmware URL: %w", err)
			}
		}

		logger.Infof("Firmware URL: %s", ref.URL)

		zipPath, err := firmware.Download(logger, ref, flagDlOutputDir)
		if err != nil {
			return fmt.Errorf("downloading firmware: %w", err)
		}

		files, err := firmware.Extract(logger, zipPath, flagDlOutputDir)
		if err != nil {
			return fmt.Errorf("extracting firmware: %w", err)
		}

		logger.Success(fmt.Sprintf("Firmware ready: %s + %s", files.CWE, files.NVU))
		return nil
	},
}

func init() {
	downloadCmd.Flags().StringVar(&flagDlFirmwareURL, "firmware-url", "", "override firmware download URL")
	downloadCmd.Flags().StringVarP(&flagDlOutputDir, "output-dir", "o", ".", "directory to save firmware files")
	downloadCmd.Flags().BoolVar(&flagDlLegacy, "legacy", false, "use legacy stable firmware")

	rootCmd.AddCommand(downloadCmd)
}
