package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/danielewood/sierra-wireless-modems/swtool/qmi"
	"github.com/spf13/cobra"
)

var flagRepairJSON bool

var repairCmd = &cobra.Command{
	Use:     "repair",
	Aliases: []string{"fix"},
	Short:   "Detect and fix common modem problems automatically",
	Long: `Runs the same diagnostic checks as 'swtool diagnostics', then applies
known fixes for any problems found.

Fixable conditions:
  - PCOFFEN not set to 2 (W_DISABLE pin can force low-power)
  - Firmware preference carrier mismatch (causes low-power lockup)
  - Modem stuck in low-power or offline mode

Non-fixable conditions are reported with guidance:
  - Firmware image issues → run 'swtool flash'
  - SIM not inserted/locked → physical action or PIN required
  - Network registration → usually resolves after power state fix
  - USB identity/composition → run 'swtool configure'

This is a mutating command — runs in dry-run mode by default.
Pass --no-dry-run to actually apply fixes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()

		if dev.ATPort == "" {
			return fmt.Errorf("no AT port found for modem %s", dev.Name)
		}

		logger.Step("Running modem repair...")

		// Collect modem info.
		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			return err
		}

		info, err := modem.GetInfo(port)
		if err != nil {
			port.Close()
			return fmt.Errorf("reading modem info: %w", err)
		}
		port.Close()

		// Get QMI operating mode and stored image data.
		var qmiMode string
		var storedImages []qmi.StoredImage
		var fwPref []qmi.FirmwarePreferenceImage
		device, mbim := controlDevice(dev)
		if device != "" {
			mode, err := qmi.GetOperatingMode(logger, device, mbim)
			if err != nil {
				logger.Debugf("QMI operating mode unavailable: %v", err)
			} else {
				qmiMode = mode
			}

			if imgs, err := qmi.ListStoredImages(logger, device, mbim); err != nil {
				logger.Debugf("QMI stored images unavailable: %v", err)
			} else {
				storedImages = imgs
			}

			if pref, err := qmi.GetFirmwarePreference(logger, device, mbim); err != nil {
				logger.Debugf("QMI firmware preference unavailable: %v", err)
			} else {
				fwPref = pref
			}
		}

		// Run diagnostic checks.
		checks := runChecks(info, qmiMode)

		// Build repair plan from checks.
		plan := buildRepairPlan(checks, info, qmiMode, storedImages, fwPref)

		// If everything passes, nothing to do.
		if allPass(plan) {
			if flagRepairJSON {
				return printRepairJSON(dev, info, plan)
			}
			printRepairText(dev, info, plan)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "🎉 Nothing to repair — all checks passed!")
			return nil
		}

		// Dry-run: show what would be fixed.
		if isDryRun() {
			if flagRepairJSON {
				return printRepairJSON(dev, info, plan)
			}
			fmt.Fprintln(os.Stderr, "🔍 DRY RUN — would apply these repairs (pass --no-dry-run to execute):")
			fmt.Fprintln(os.Stderr)
			printRepairText(dev, info, plan)
			return nil
		}

		// Execute repairs.
		plan = executeRepairs(dev, info, plan, storedImages)

		// Brief pause for modem to stabilize after fixes.
		time.Sleep(2 * time.Second)

		// Re-check to verify fixes took effect.
		port, err = modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			logger.Warnf("Could not re-read modem info for verification: %v", err)
		} else {
			newInfo, err := modem.GetInfo(port)
			port.Close()
			if err != nil {
				logger.Warnf("Could not re-read modem info for verification: %v", err)
			} else {
				// Refresh QMI mode.
				newQMIMode := qmiMode
				if device != "" {
					if mode, err := qmi.GetOperatingMode(logger, device, mbim); err == nil {
						newQMIMode = mode
					}
				}

				newChecks := runChecks(newInfo, newQMIMode)
				plan = verifyRepairs(plan, newChecks)
				info = newInfo
			}
		}

		if flagRepairJSON {
			return printRepairJSON(dev, info, plan)
		}
		printRepairText(dev, info, plan)
		return nil
	},
}

func init() {
	repairCmd.Flags().BoolVar(&flagRepairJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(repairCmd)
}

// repairStatus constants for repair results.
const (
	repairPass    = "pass"
	repairFixed   = "fixed"
	repairSkipped = "skipped"
	repairFailed  = "failed"
	repairWouldFix = "would_fix"
)

// RepairResult holds the outcome of a single repair action.
type RepairResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

// skipGuidance maps check names to user guidance for non-fixable conditions.
var skipGuidance = map[string]string{
	"firmware_images":      "run 'swtool flash' to reflash firmware",
	"sim_status":           "",  // dynamic based on status
	"network_registration": "should resolve after power state fix",
	"usb_identity":         "run 'swtool configure --vendor' to change USB identity",
	"usb_composition":      "run 'swtool configure --mode' to change USB composition",
}

// buildRepairPlan converts diagnostic checks into repair results with planned actions.
func buildRepairPlan(checks []Check, info *modem.Info, qmiMode string, storedImages []qmi.StoredImage, fwPref []qmi.FirmwarePreferenceImage) []RepairResult {
	var results []RepairResult

	// Determine if QMI firmware preference fix is available.
	// We need stored images so we can derive the correct values.
	hasQMIData := len(storedImages) > 0

	for _, c := range checks {
		switch c.Name {
		case "pcoffen":
			if c.Status != statusPass {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairWouldFix,
					Summary: fmt.Sprintf("Would send AT!PCOFFEN=2 (currently %d)", info.Power.PCOFFEN),
				})
			} else {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairPass,
					Summary: c.Summary,
				})
			}

		case "firmware_preference":
			if c.Status == statusFail && hasQMIData {
				// QMI can fix any mismatch (version or carrier) by deriving
				// the correct values from stored images.
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairWouldFix,
					Summary: "Would set firmware preference via QMI (from stored images)",
				})
			} else if c.Status == statusFail && strings.Contains(c.Summary, "carrier mismatch") {
				// AT fallback: can only fix carrier mismatch with hardcoded GENERIC.
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairWouldFix,
					Summary: `Would send AT!IMPREF="GENERIC" + AT!GOBIIMPREF="GENERIC" (AT fallback)`,
				})
			} else if c.Status == statusFail || c.Status == statusWarn {
				// Version mismatch without QMI data — not auto-fixable via AT
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairSkipped,
					Summary: c.Summary,
					Detail:  "run 'swtool flash' to reflash with matching firmware",
				})
			} else {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairPass,
					Summary: c.Summary,
				})
			}

		case "power_state":
			isLowPower := c.Status == statusFail && (strings.Contains(strings.ToLower(c.Summary), "low") ||
				strings.Contains(strings.ToLower(c.Summary), "offline"))
			if isLowPower {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairWouldFix,
					Summary: "Would bring modem online via QMI SetOnline",
				})
			} else if c.Status != statusPass {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairSkipped,
					Summary: c.Summary,
					Detail:  c.Detail,
				})
			} else {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairPass,
					Summary: c.Summary,
				})
			}

		case "sim_status":
			if c.Status != statusPass {
				detail := c.Detail
				if detail == "" {
					upper := strings.ToUpper(c.Summary)
					if strings.Contains(upper, "NOT INSERTED") || strings.Contains(upper, "NOT READY") {
						detail = "insert SIM card"
					} else if strings.Contains(upper, "PIN") {
						detail = "unlock SIM with PIN"
					} else if strings.Contains(upper, "PUK") {
						detail = "contact carrier for PUK code"
					}
				}
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairSkipped,
					Summary: c.Summary,
					Detail:  detail,
				})
			} else {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairPass,
					Summary: c.Summary,
				})
			}

		default:
			// firmware_images, network_registration, usb_identity, usb_composition
			if c.Status != statusPass {
				detail := c.Detail
				if guidance, ok := skipGuidance[c.Name]; ok && guidance != "" && detail == "" {
					detail = guidance
				}
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairSkipped,
					Summary: c.Summary,
					Detail:  detail,
				})
			} else {
				results = append(results, RepairResult{
					Name:    c.Name,
					Status:  repairPass,
					Summary: c.Summary,
				})
			}
		}
	}

	return results
}

// allPass returns true if every result is a pass.
func allPass(results []RepairResult) bool {
	for _, r := range results {
		if r.Status != repairPass {
			return false
		}
	}
	return true
}

// executeRepairs applies fixes in the correct order: PCOFFEN first, then
// firmware preference, then bring online.
func executeRepairs(dev *modem.Device, info *modem.Info, plan []RepairResult, storedImages []qmi.StoredImage) []RepairResult {
	device, mbim := controlDevice(dev)

	// Determine which fixes are needed.
	needsPCOFFEN := false
	needsFWPref := false
	needsOnline := false
	for _, r := range plan {
		if r.Status != repairWouldFix {
			continue
		}
		switch r.Name {
		case "pcoffen":
			needsPCOFFEN = true
		case "firmware_preference":
			needsFWPref = true
		case "power_state":
			needsOnline = true
		}
	}

	// Fix 1: PCOFFEN — always AT-only (Sierra proprietary).
	if needsPCOFFEN {
		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			logger.Warnf("Could not open AT port for PCOFFEN fix: %v", err)
			markPlanEntry(plan, "pcoffen", repairFailed, fmt.Sprintf("could not open AT port: %v", err))
		} else {
			if err := modem.EnterCommandMode(port); err != nil {
				logger.Warnf("Could not enter engineering mode: %v", err)
				markPlanEntry(plan, "pcoffen", repairFailed, fmt.Sprintf("could not enter engineering mode: %v", err))
			} else {
				applyPCOFFENFix(port, plan, info)
			}
			port.Close()
		}
	}

	// Fix 2: Firmware preference — try QMI first, fall back to AT.
	if needsFWPref {
		if device != "" && len(storedImages) > 0 {
			applyFWPrefFixQMI(plan, device, mbim, storedImages)
		} else {
			// AT fallback: hardcoded GENERIC.
			port, err := modem.OpenPort(dev.ATPort, logger)
			if err != nil {
				logger.Warnf("Could not open AT port for firmware preference fix: %v", err)
				markPlanEntry(plan, "firmware_preference", repairFailed, fmt.Sprintf("could not open AT port: %v", err))
			} else {
				if err := modem.EnterCommandMode(port); err != nil {
					logger.Warnf("Could not enter engineering mode: %v", err)
					markPlanEntry(plan, "firmware_preference", repairFailed, fmt.Sprintf("could not enter engineering mode: %v", err))
				} else {
					applyFWPrefFix(port, plan)
				}
				port.Close()
			}
		}
	}

	// Fix 3: Bring modem online via QMI.
	if needsOnline {
		applyOnlineFix(plan, device, mbim)
	}

	return plan
}

// applyPCOFFENFix sends AT!PCOFFEN=2 and updates the plan entry.
func applyPCOFFENFix(port *modem.Port, plan []RepairResult, info *modem.Info) {
	logger.Step("Setting PCOFFEN=2...")
	_, err := port.SendCommand("AT!PCOFFEN=2")
	for i := range plan {
		if plan[i].Name != "pcoffen" {
			continue
		}
		if err != nil {
			plan[i].Status = repairFailed
			plan[i].Summary = fmt.Sprintf("AT!PCOFFEN=2 failed: %v", err)
		} else {
			plan[i].Status = repairFixed
			plan[i].Summary = fmt.Sprintf("Set PCOFFEN=2 (was %d)", info.Power.PCOFFEN)
		}
	}
}

// applyFWPrefFix sends AT!IMPREF="GENERIC" and AT!GOBIIMPREF="GENERIC".
func applyFWPrefFix(port *modem.Port, plan []RepairResult) {
	logger.Step(`Setting firmware preference to GENERIC...`)
	var firstErr error
	for _, cmd := range []string{`AT!IMPREF="GENERIC"`, `AT!GOBIIMPREF="GENERIC"`} {
		if _, err := port.SendCommand(cmd); err != nil {
			logger.Debugf("%s failed: %v", cmd, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	for i := range plan {
		if plan[i].Name != "firmware_preference" {
			continue
		}
		if firstErr != nil {
			plan[i].Status = repairFailed
			plan[i].Summary = fmt.Sprintf("set firmware preference failed: %v", firstErr)
		} else {
			plan[i].Status = repairFixed
			plan[i].Summary = "Set firmware preference to GENERIC"
		}
	}
}

// applyOnlineFix brings the modem online via QMI SetOnline.
func applyOnlineFix(plan []RepairResult, device string, mbim bool) {
	for i := range plan {
		if plan[i].Name != "power_state" {
			continue
		}
		if device == "" {
			plan[i].Status = repairFailed
			plan[i].Summary = "no QMI control device found"
			return
		}
		logger.Step("Bringing modem online via QMI...")
		if err := qmi.SetOnline(logger, device, mbim); err != nil {
			plan[i].Status = repairFailed
			plan[i].Summary = fmt.Sprintf("QMI SetOnline failed: %v", err)
		} else {
			plan[i].Status = repairFixed
			plan[i].Summary = "Brought modem online via QMI"
		}
	}
}

// applyFWPrefFixQMI sets firmware preference via QMI using stored image data.
// It finds the current PRI image, extracts the firmware version, carrier, and
// config version, then calls SetFirmwarePreference.
func applyFWPrefFixQMI(plan []RepairResult, device string, mbim bool, storedImages []qmi.StoredImage) {
	// Find the current PRI image.
	var currentPRI *qmi.StoredImage
	for i := range storedImages {
		if storedImages[i].Type == "pri" && storedImages[i].Current {
			currentPRI = &storedImages[i]
			break
		}
	}

	if currentPRI == nil {
		// No current PRI found — fall through to failure.
		markPlanEntry(plan, "firmware_preference", repairFailed, "no current PRI image found in stored images")
		return
	}

	fwVersion, carrier := qmi.ParsePRIBuildID(currentPRI.BuildID)
	if fwVersion == "" || carrier == "" {
		markPlanEntry(plan, "firmware_preference", repairFailed,
			fmt.Sprintf("could not parse PRI build ID %q", currentPRI.BuildID))
		return
	}

	configVersion := currentPRI.UniqueID

	logger.Step(fmt.Sprintf("Setting firmware preference via QMI: fw=%s, config=%s, carrier=%s",
		fwVersion, configVersion, carrier))

	err := qmi.SetFirmwarePreference(logger, device, mbim, fwVersion, configVersion, carrier)
	for i := range plan {
		if plan[i].Name != "firmware_preference" {
			continue
		}
		if err != nil {
			plan[i].Status = repairFailed
			plan[i].Summary = fmt.Sprintf("QMI set firmware preference failed: %v", err)
		} else {
			plan[i].Status = repairFixed
			plan[i].Summary = fmt.Sprintf("Set firmware preference via QMI: %s %s", fwVersion, carrier)
		}
	}
}

// markPlanEntry marks a specific repair plan entry with the given status and summary.
func markPlanEntry(plan []RepairResult, name, status, summary string) {
	for i := range plan {
		if plan[i].Name == name {
			plan[i].Status = status
			plan[i].Summary = summary
			return
		}
	}
}

// verifyRepairs cross-checks repair results against fresh diagnostic checks.
// If a "fixed" item still fails in the new checks, it gets marked as "failed".
func verifyRepairs(plan []RepairResult, newChecks []Check) []RepairResult {
	checkMap := make(map[string]Check, len(newChecks))
	for _, c := range newChecks {
		checkMap[c.Name] = c
	}

	for i := range plan {
		if plan[i].Status != repairFixed {
			continue
		}
		if newCheck, ok := checkMap[plan[i].Name]; ok {
			if newCheck.Status == statusFail {
				plan[i].Status = repairFailed
				plan[i].Detail = fmt.Sprintf("fix applied but check still fails: %s", newCheck.Summary)
			}
		}
	}

	return plan
}

// repairResult is the JSON output structure for repair.
type repairResult struct {
	Device  string         `json:"device"`
	USBID   string         `json:"usb_id"`
	DryRun  bool           `json:"dry_run"`
	Results []RepairResult `json:"results"`
}

// printRepairJSON outputs repair results as structured JSON.
func printRepairJSON(dev *modem.Device, info *modem.Info, results []RepairResult) error {
	result := repairResult{
		Device:  fmt.Sprintf("%s %s", info.Identity.Manufacturer, info.Identity.Model),
		USBID:   dev.ID.String(),
		DryRun:  isDryRun(),
		Results: results,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// repairBadge returns the emoji badge for a repair result status.
func repairBadge(status string) string {
	switch status {
	case repairPass:
		return "✅"
	case repairFixed:
		return "🔧"
	case repairSkipped:
		return "⏭️ "
	case repairFailed:
		return "❌"
	case repairWouldFix:
		return "🔮"
	default:
		return "  "
	}
}

// printRepairText outputs emoji-badged repair results to stdout.
func printRepairText(dev *modem.Device, info *modem.Info, results []RepairResult) {
	w := os.Stdout

	fmt.Fprintln(w)
	title := fmt.Sprintf("🔧 Modem Repair: %s %s (%s)",
		info.Identity.Manufacturer, info.Identity.Model, dev.ID)
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, strings.Repeat("━", cellWidth(title)))
	fmt.Fprintln(w)

	// Find max display name width for alignment.
	maxName := 0
	for _, r := range results {
		if n := len(checkDisplayName(r.Name)); n > maxName {
			maxName = n
		}
	}

	for _, r := range results {
		badge := repairBadge(r.Status)
		name := checkDisplayName(r.Name)
		fmt.Fprintf(w, "  %s %-*s  %s\n", badge, maxName+2, name, r.Summary)

		if r.Detail != "" {
			for _, line := range strings.Split(r.Detail, "\n") {
				// Indent detail below the summary, aligned with result text.
				// Badge(2) + space(1) + name(maxName+2) + space(2) = maxName+7
				fmt.Fprintf(w, "  %*s  %s\n", maxName+5, "↳", strings.TrimSpace(line))
			}
		}
	}

	// Summary line.
	printRepairSummary(w, results)
}

// printRepairSummary shows a final count of pass/fixed/skipped/failed.
func printRepairSummary(w *os.File, results []RepairResult) {
	var pass, fixed, skipped, failed, wouldFix int
	for _, r := range results {
		switch r.Status {
		case repairPass:
			pass++
		case repairFixed:
			fixed++
		case repairSkipped:
			skipped++
		case repairFailed:
			failed++
		case repairWouldFix:
			wouldFix++
		}
	}

	fmt.Fprintln(w)
	var parts []string
	if pass > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", pass))
	}
	if fixed > 0 {
		parts = append(parts, fmt.Sprintf("%d fixed", fixed))
	}
	if wouldFix > 0 {
		parts = append(parts, fmt.Sprintf("%d would fix", wouldFix))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}

	if failed > 0 {
		fmt.Fprintf(w, "📊 %s\n", strings.Join(parts, ", "))
	} else if fixed > 0 || wouldFix > 0 {
		fmt.Fprintf(w, "📊 %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Fprintf(w, "🎉 %s\n", strings.Join(parts, ", "))
	}
}
