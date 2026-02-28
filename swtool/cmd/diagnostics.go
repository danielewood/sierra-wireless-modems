package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/danielewood/sierra-wireless-modems/swtool/qmi"
	"github.com/spf13/cobra"
)

var flagDiagJSON bool

var diagCmd = &cobra.Command{
	Use:     "diagnostics",
	Aliases: []string{"diag"},
	Short:   "Run health checks on the modem and report problems",
	Long: `Runs a series of diagnostic checks against the modem and reports results
as PASS, WARN, or FAIL with plain-language explanations.

Checks firmware image slots, firmware preference consistency, power state,
SIM status, network registration, USB identity, and USB composition.`,
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

		// JSON path: collect all data first, then output.
		if flagDiagJSON {
			info, err := modem.GetInfo(port)
			if err != nil {
				return fmt.Errorf("reading modem info: %w", err)
			}
			var qmiMode string
			device, mbim := controlDevice(dev)
			if device != "" {
				if mode, err := qmi.GetOperatingMode(logger, device, mbim); err == nil {
					qmiMode = mode
				}
			}
			return printDiagJSON(dev, info, runChecks(info, qmiMode))
		}

		w := os.Stdout

		// Print header immediately, before any AT commands, so the user has
		// visual confirmation the command started.
		fmt.Fprintln(w)
		title := fmt.Sprintf("🩺 Modem Diagnostics: %s (%s)", dev.Name, dev.ID)
		fmt.Fprintln(w, title)
		fmt.Fprintln(w, strings.Repeat("━", cellWidth(title)))
		fmt.Fprintln(w)

		// Start animated checklist before data collection so spinners are
		// visible during the multi-second AT command sequence.
		cl := startChecklist(w, []string{
			checkDisplayName("firmware_images"),
			checkDisplayName("firmware_preference"),
			checkDisplayName("power_state"),
			checkDisplayName("pcoffen"),
			checkDisplayName("sim_status"),
			checkDisplayName("network_registration"),
			checkDisplayName("usb_identity"),
			checkDisplayName("usb_composition"),
		})

		// Fire QMI query in parallel with AT commands — they use different
		// kernel interfaces so there's no contention.
		var qmiMode string
		qmiDone := make(chan struct{})
		go func() {
			defer close(qmiDone)
			device, mbim := controlDevice(dev)
			if device == "" {
				return
			}
			if mode, err := qmi.GetOperatingMode(logger, device, mbim); err == nil {
				qmiMode = mode
			}
		}()

		// Stream check results as each AT command group completes.
		// send# → check resolved (matches GetInfoStreaming doc comment):
		//   #1 → power_state (index 2) + pcoffen (index 3)
		//   #2 → sim_status (index 4)
		//   #3 → network_registration (index 5)
		//   #4 → firmware_preference (index 1)
		//   #5 → usb_identity (index 6)
		//   #6 → usb_composition (index 7)
		//   #7 → firmware_images (index 0)
		//   power_state may be updated again after <-qmiDone if QMI adds info
		var finalInfo *modem.Info
		sendNum := 0
		modem.GetInfoStreaming(port, func(info *modem.Info) {
			sendNum++
			finalInfo = info
			switch sendNum {
			case 1: // AT!PCINFO? + AT!PCOFFEN? done — power state ready.
				pwr := checkPowerState(info, "") // QMI not available yet; use AT data
				cl.Resolve(2, diagBadge(pwr.Status), pwr.Summary)
				if c := checkPCOFFEN(info); c != nil {
					cl.Resolve(3, diagBadge(c.Status), c.Summary)
				} else {
					cl.Resolve(3, diagBadge(statusPass), "PCOFFEN=2 (best setting)")
				}
				cl.SetVoters(info.Power.LPMVoters)
			case 2: // AT+CPIN? done (or skipped in LPM) — sim_status ready.
				sim := checkSIMStatus(info)
				cl.Resolve(4, diagBadge(sim.Status), sim.Summary)
			case 3: // CREG/CEREG/CGREG done — network_registration ready.
				reg := checkNetworkRegistration(info)
				cl.Resolve(5, diagBadge(reg.Status), reg.Summary)
			case 4: // AT!IMPREF? done — firmware_preference ready.
				fwp := checkFirmwarePreference(info)
				cl.Resolve(1, diagBadge(fwp.Status), fwp.Summary)
			case 5: // USBVID+USBPID done — usb_identity ready.
				uid := checkUSBIdentity(info)
				cl.Resolve(6, diagBadge(uid.Status), uid.Summary)
			case 6: // AT!USBCOMP? done — usb_composition ready.
				uco := checkUSBComposition(info)
				cl.Resolve(7, diagBadge(uco.Status), uco.Summary)
			case 7: // AT!IMAGE? done — firmware_images ready.
				fw := checkFirmwareImages(info)
				cl.Resolve(0, diagBadge(fw.Status), fw.Summary)
			}
		})

		// Wait for QMI, then update power_state if QMI provides additional info
		// (e.g. when AT!PCINFO? returned empty but QMI has a mode).
		<-qmiDone
		if finalInfo != nil && (finalInfo.Power.State == "" || qmiMode != "") {
			pwr := checkPowerState(finalInfo, qmiMode)
			cl.Resolve(2, diagBadge(pwr.Status), pwr.Summary)
		}

		cl.Finish()

		if finalInfo == nil {
			return fmt.Errorf("no modem data received")
		}

		// noAnimate (piped/quiet): voter table and summary are not part of the live area.
		if cl.noAnimate {
			printLPMVoterTable(w, finalInfo.Power.LPMVoters)
			checks := runChecks(finalInfo, qmiMode)
			printDiagSummary(w, checks)
		}

		return nil
	},
}

func init() {
	diagCmd.Flags().BoolVar(&flagDiagJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(diagCmd)
}

// checkStatus constants for diagnostic results.
const (
	statusPass = "pass"
	statusWarn = "warn"
	statusFail = "fail"
)

// Check holds the result of a single diagnostic check.
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

// runChecks executes all diagnostic checks and returns the results.
// Always returns exactly 8 checks in a stable order.
func runChecks(info *modem.Info, qmiMode string) []Check {
	var checks []Check
	checks = append(checks, checkFirmwareImages(info))
	checks = append(checks, checkFirmwarePreference(info))
	checks = append(checks, checkPowerState(info, qmiMode))
	if c := checkPCOFFEN(info); c != nil {
		checks = append(checks, *c)
	} else {
		checks = append(checks, Check{Name: "pcoffen", Status: statusPass, Summary: "PCOFFEN=2 (best setting)"})
	}
	checks = append(checks, checkSIMStatus(info))
	checks = append(checks, checkNetworkRegistration(info))
	checks = append(checks, checkUSBIdentity(info))
	checks = append(checks, checkUSBComposition(info))
	return checks
}

// checkFirmwareImages verifies firmware image slots are healthy.
func checkFirmwareImages(info *modem.Info) Check {
	slots := info.Images.Firmware
	if len(slots) == 0 {
		return Check{
			Name:    "firmware_images",
			Status:  statusWarn,
			Summary: "no firmware image data available",
		}
	}

	var goodCount, emptyCount, badCount int
	var failuresInActive bool
	for _, s := range slots {
		switch strings.ToUpper(s.Status) {
		case "GOOD":
			goodCount++
		case "EMPTY":
			emptyCount++
		case "BAD":
			badCount++
		}
	}

	// Check if active slot has failures.
	activeSlot := fmt.Sprintf("%d", info.Images.ActiveSlot)
	for _, s := range slots {
		if s.Slot == activeSlot && s.Failures > 0 {
			failuresInActive = true
		}
	}

	if emptyCount == len(slots) {
		return Check{
			Name:    "firmware_images",
			Status:  statusFail,
			Summary: "all firmware slots are empty",
			Detail:  "no firmware loaded, modem cannot operate — flash firmware with 'swtool flash'",
		}
	}

	if badCount > 0 {
		return Check{
			Name:    "firmware_images",
			Status:  statusFail,
			Summary: fmt.Sprintf("%d/%d slots BAD", badCount, len(slots)),
			Detail:  "corrupt firmware slot detected, reflash recommended",
		}
	}

	if failuresInActive {
		return Check{
			Name:    "firmware_images",
			Status:  statusWarn,
			Summary: fmt.Sprintf("%d/%d slots in use, active slot %d has failures", goodCount, len(slots), info.Images.ActiveSlot),
			Detail:  "active firmware slot has recorded failures — monitor for stability issues",
		}
	}

	return Check{
		Name:    "firmware_images",
		Status:  statusPass,
		Summary: fmt.Sprintf("%d/%d slots in use, active slot %d (GOOD)", goodCount, len(slots), info.Images.ActiveSlot),
	}
}

// checkFirmwarePreference verifies preferred and current firmware match.
func checkFirmwarePreference(info *modem.Info) Check {
	pref := info.Firmware.Preferred
	cur := info.Firmware.Current

	// If both are empty, we can't check.
	if pref.Version == "" && cur.Version == "" {
		return Check{
			Name:    "firmware_preference",
			Status:  statusWarn,
			Summary: "no firmware version data available",
		}
	}

	if pref.Version != "" && cur.Version != "" && pref.Version != cur.Version {
		return Check{
			Name:    "firmware_preference",
			Status:  statusFail,
			Summary: fmt.Sprintf("version mismatch: preferred %s but running %s", pref.Version, cur.Version),
			Detail:  "firmware version mismatch causes the modem to enter low-power mode — reflash or update preference",
		}
	}

	if pref.CarrierName != "" && cur.CarrierName != "" && pref.CarrierName != cur.CarrierName {
		return Check{
			Name:    "firmware_preference",
			Status:  statusFail,
			Summary: fmt.Sprintf("carrier mismatch: preferred %s but running %s", pref.CarrierName, cur.CarrierName),
			Detail:  "carrier mismatch causes the modem to enter low-power mode — reflash or update preference",
		}
	}

	version := cur.Version
	if version == "" {
		version = pref.Version
	}
	carrier := cur.CarrierName
	if carrier == "" {
		carrier = pref.CarrierName
	}

	return Check{
		Name:    "firmware_preference",
		Status:  statusPass,
		Summary: fmt.Sprintf("%s %s (preferred = current)", version, carrier),
	}
}

// lpmVoterDescriptions maps LPM voter names to human-readable explanations.
var lpmVoterDescriptions = map[string]string{
	"FOTA":      "Firmware-over-the-air update",
	"User":      "User-initiated (AT+CFUN=0 or QMI set-offline)",
	"BIOS":      "BIOS power management",
	"OMADM":     "OMA-DM client power management",
	"Temp":      "Thermal protection (overheating)",
	"Volt":      "Voltage out of range",
	"W_DISABLE": "Hardware RF kill / BIOS disable pin (fix: PCOFFEN=2)",
	"IMSWITCH":  "Image selection mismatch (PRI version mismatch)",
	"LWM2M":     "LWM2M client power management",
}

// lpmVoterOrder defines a consistent display order for LPM voters.
var lpmVoterOrder = []string{
	"FOTA", "User", "BIOS", "OMADM", "Temp", "Volt", "W_DISABLE", "IMSWITCH", "LWM2M",
}

// formatLPMVoters builds a detail string listing all LPM voters with descriptions.
// Active voters (value != 0) are highlighted with their description.
func formatLPMVoters(voters map[string]int) string {
	if len(voters) == 0 {
		return ""
	}

	var lines []string
	for _, name := range lpmVoterOrder {
		val, ok := voters[name]
		if !ok {
			continue
		}
		desc := lpmVoterDescriptions[name]
		if val != 0 {
			lines = append(lines, fmt.Sprintf("  %-14s %d  ** %s", name, val, desc))
		} else {
			lines = append(lines, fmt.Sprintf("  %-14s %d", name, val))
		}
	}
	// Include any voters not in the known order.
	for name, val := range voters {
		found := false
		for _, known := range lpmVoterOrder {
			if name == known {
				found = true
				break
			}
		}
		if !found {
			if val != 0 {
				lines = append(lines, fmt.Sprintf("  %-14s %d  ** unknown voter", name, val))
			} else {
				lines = append(lines, fmt.Sprintf("  %-14s %d", name, val))
			}
		}
	}

	return "LPM voters:\n" + strings.Join(lines, "\n")
}

// checkPowerState verifies the modem is online and not stuck in low-power mode.
func checkPowerState(info *modem.Info, qmiMode string) Check {
	state := info.Power.State

	// Use QMI mode as fallback/supplement.
	if state == "" && qmiMode != "" {
		state = qmiMode
	}

	if state == "" {
		return Check{
			Name:    "power_state",
			Status:  statusWarn,
			Summary: "power state unknown",
		}
	}

	voterDetail := formatLPMVoters(info.Power.LPMVoters)

	isLowPower := strings.Contains(strings.ToLower(state), "low") ||
		strings.EqualFold(state, "offline")

	if isLowPower {
		var activeCount int
		for _, val := range info.Power.LPMVoters {
			if val != 0 {
				activeCount++
			}
		}

		summary := state
		if activeCount > 0 {
			summary = fmt.Sprintf("%s — %d active LPM voter(s)", state, activeCount)
		}

		detail := voterDetail
		if detail == "" {
			detail = "modem is in low-power mode but no active LPM voters detected"
		}

		return Check{
			Name:    "power_state",
			Status:  statusFail,
			Summary: summary,
			Detail:  detail,
		}
	}

	return Check{
		Name:    "power_state",
		Status:  statusPass,
		Summary: state,
		Detail:  voterDetail,
	}
}

// checkPCOFFEN returns a WARN check if PCOFFEN is not set to 2 (ignore W_DISABLE).
// Returns nil if no separate PCOFFEN check is needed (power state already covers it).
func checkPCOFFEN(info *modem.Info) *Check {
	// Only emit this check if power state is not already failing due to W_DISABLE.
	// PCOFFEN is a preventive check — relevant even when modem is online.
	if info.Power.PCOFFEN == 2 {
		return nil // Best setting, no check needed.
	}

	return &Check{
		Name:    "pcoffen",
		Status:  statusWarn,
		Summary: fmt.Sprintf("PCOFFEN=%d — W_DISABLE pin can force low-power", info.Power.PCOFFEN),
		Detail:  "set AT!PCOFFEN=2 to ignore W_DISABLE pin and prevent unexpected low-power mode",
	}
}

// checkSIMStatus verifies a SIM card is present and ready.
func checkSIMStatus(info *modem.Info) Check {
	status := info.SIM.Status
	if status == modem.SIMStatusLPM {
		return Check{
			Name:    "sim_status",
			Status:  statusWarn,
			Summary: "unavailable — modem is in low-power mode",
			Detail:  "SIM queries are skipped while the modem is offline; bring the modem online first",
		}
	}
	if status == "" {
		return Check{
			Name:    "sim_status",
			Status:  statusFail,
			Summary: "SIM status unknown (no response from AT+CPIN?)",
			Detail:  "SIM not detected — check that a SIM card is inserted",
		}
	}

	upper := strings.ToUpper(status)

	if strings.Contains(upper, "NOT INSERTED") || strings.Contains(upper, "NOT READY") {
		return Check{
			Name:    "sim_status",
			Status:  statusFail,
			Summary: status,
			Detail:  "SIM not detected — check that a SIM card is properly inserted",
		}
	}

	if strings.Contains(upper, "PIN") {
		return Check{
			Name:    "sim_status",
			Status:  statusWarn,
			Summary: status,
			Detail:  "SIM requires PIN unlock before the modem can register on a network",
		}
	}

	if strings.Contains(upper, "PUK") {
		return Check{
			Name:    "sim_status",
			Status:  statusWarn,
			Summary: status,
			Detail:  "SIM is PUK-locked — contact your carrier for the PUK code",
		}
	}

	if strings.Contains(upper, "READY") {
		return Check{
			Name:    "sim_status",
			Status:  statusPass,
			Summary: "READY",
		}
	}

	return Check{
		Name:    "sim_status",
		Status:  statusWarn,
		Summary: status,
		Detail:  "unexpected SIM status",
	}
}

// checkNetworkRegistration verifies LTE (EPS) registration status.
func checkNetworkRegistration(info *modem.Info) Check {
	eps := info.Registration.EPS

	if eps.Status == "" {
		// Fall back to CS or GPRS if EPS is empty.
		if info.Registration.CS.Status != "" {
			eps = info.Registration.CS
		} else if info.Registration.GPRS.Status != "" {
			eps = info.Registration.GPRS
		}
	}

	if eps.Status == "" {
		return Check{
			Name:    "network_registration",
			Status:  statusWarn,
			Summary: "registration status unknown",
		}
	}

	upper := strings.ToUpper(eps.Status)

	switch {
	case strings.Contains(upper, "HOME") || strings.Contains(upper, "ROAMING"):
		return Check{
			Name:    "network_registration",
			Status:  statusPass,
			Summary: eps.Status,
		}
	case strings.Contains(upper, "SEARCHING") || strings.Contains(upper, "NOT REGISTERED, SEARCHING"):
		return Check{
			Name:    "network_registration",
			Status:  statusWarn,
			Summary: eps.Status,
			Detail:  "modem is searching for a network — this may take a moment",
		}
	case strings.Contains(upper, "DENIED"):
		return Check{
			Name:    "network_registration",
			Status:  statusWarn,
			Summary: eps.Status,
			Detail:  "registration denied by network — check SIM activation and APN settings",
		}
	default:
		return Check{
			Name:    "network_registration",
			Status:  statusFail,
			Summary: eps.Status,
			Detail:  "not registered and not searching — check antenna, SIM, and modem power state",
		}
	}
}

// checkUSBIdentity looks up the modem's VID:PID in known vendor profiles.
func checkUSBIdentity(info *modem.Info) Check {
	vid := strings.ToUpper(info.USB.VID)
	pidApp := strings.ToUpper(info.USB.PID.App)

	if vid == "" || pidApp == "" {
		return Check{
			Name:    "usb_identity",
			Status:  statusWarn,
			Summary: "USB identity data unavailable",
		}
	}

	for name, profile := range modem.Vendors {
		if strings.EqualFold(vid, profile.VID) && strings.EqualFold(pidApp, profile.PIDApp) {
			pidInfo := fmt.Sprintf("VID=%s, PID=%s", vid, pidApp)
			if info.USB.PID.Boot != "" {
				pidInfo = fmt.Sprintf("VID=%s, PID=%s/%s", vid, pidApp, strings.ToUpper(info.USB.PID.Boot))
			}
			return Check{
				Name:    "usb_identity",
				Status:  statusPass,
				Summary: fmt.Sprintf("%s (%s)", name, pidInfo),
			}
		}
	}

	return Check{
		Name:    "usb_identity",
		Status:  statusWarn,
		Summary: fmt.Sprintf("unknown USB identity %s:%s", vid, pidApp),
		Detail:  "VID:PID does not match any known Sierra Wireless vendor profile",
	}
}

// checkUSBComposition reports the current USB composition mode.
func checkUSBComposition(info *modem.Info) Check {
	comp := info.USB.Composition
	if comp.Bitmask == "" && len(comp.Interfaces) == 0 {
		return Check{
			Name:    "usb_composition",
			Status:  statusWarn,
			Summary: "USB composition data unavailable",
		}
	}

	interfaces := ""
	if len(comp.Interfaces) > 0 {
		interfaces = strings.Join(comp.Interfaces, ", ")
	}

	// Check against known compositions.
	switch comp.Bitmask {
	case "0000100D":
		summary := "MBIM"
		if interfaces != "" {
			summary = fmt.Sprintf("MBIM (%s)", interfaces)
		}
		return Check{
			Name:    "usb_composition",
			Status:  statusPass,
			Summary: summary,
		}
	case "0000010D":
		summary := "QMI"
		if interfaces != "" {
			summary = fmt.Sprintf("QMI (%s)", interfaces)
		}
		return Check{
			Name:    "usb_composition",
			Status:  statusPass,
			Summary: summary,
		}
	default:
		summary := comp.Bitmask
		if interfaces != "" {
			summary = fmt.Sprintf("%s (%s)", comp.Bitmask, interfaces)
		}
		return Check{
			Name:    "usb_composition",
			Status:  statusWarn,
			Summary: summary,
			Detail:  "USB composition does not match known MBIM (0000100D) or QMI (0000010D) modes",
		}
	}
}

// diagResult is the JSON output structure for diagnostics.
type diagResult struct {
	Device string  `json:"device"`
	USBID  string  `json:"usb_id"`
	Checks []Check `json:"checks"`
}

// printDiagJSON outputs diagnostic results as structured JSON.
func printDiagJSON(dev *modem.Device, info *modem.Info, checks []Check) error {
	result := diagResult{
		Device: fmt.Sprintf("%s %s", info.Identity.Manufacturer, info.Identity.Model),
		USBID:  dev.ID.String(),
		Checks: checks,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// ══════════════════════════════════════════════════════════════════════
// Display constants and helpers
// ══════════════════════════════════════════════════════════════════════

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorYellow = "\033[0;33m"
	colorGreen  = "\033[0;32m"
	colorCyan   = "\033[0;36m"
	colorDim    = "\033[2m"
)

// checkDisplayNames maps check names to human-readable display names.
var checkDisplayNames = map[string]string{
	"firmware_images":      "Firmware Images",
	"firmware_preference":  "Firmware Preference",
	"power_state":          "Power State",
	"pcoffen":              "PCOFFEN (W_DISABLE Override)",
	"sim_status":           "SIM Status",
	"network_registration": "Network Registration",
	"usb_identity":         "USB Identity",
	"usb_composition":      "USB Composition",
}

// checkDisplayName returns the human-readable display name for a check.
func checkDisplayName(name string) string {
	if dn, ok := checkDisplayNames[name]; ok {
		return dn
	}
	return strings.ReplaceAll(name, "_", " ")
}

// diagBadge returns the emoji badge for a diagnostic check status.
func diagBadge(status string) string {
	switch status {
	case statusPass:
		return "✅"
	case statusWarn:
		return "⚠️ "
	case statusFail:
		return "❌"
	default:
		return "  "
	}
}

// ══════════════════════════════════════════════════════════════════════
// Diagnostic text output
// ══════════════════════════════════════════════════════════════════════

// printDiagSummary shows a final pass/warn/fail count with emoji.
func printDiagSummary(w *os.File, checks []Check) {
	var pass, warn, fail int
	for _, c := range checks {
		switch c.Status {
		case statusPass:
			pass++
		case statusWarn:
			warn++
		case statusFail:
			fail++
		}
	}

	fmt.Fprintln(w)
	var parts []string
	if pass > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", pass))
	}
	if warn > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warn))
	}
	if fail > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", fail))
	}

	if fail > 0 {
		fmt.Fprintf(w, "📊 %s\n", strings.Join(parts, ", "))
	} else if warn > 0 {
		fmt.Fprintf(w, "📊 %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Fprintln(w, "🎉 All checks passed!")
	}
}

// ══════════════════════════════════════════════════════════════════════
// LPM voter table
// ══════════════════════════════════════════════════════════════════════

// printLPMVoterTable renders a bordered table of all LPM voters with 🟢/🔴 indicators.
func printLPMVoterTable(w *os.File, voters map[string]int) {
	if len(voters) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  📡 Low Power Mode Voters:")

	tbl := newTable("Voter", "", "Description")
	for _, name := range lpmVoterOrder {
		val, ok := voters[name]
		if !ok {
			continue
		}
		indicator := "🟢"
		if val != 0 {
			indicator = "🔴"
		}
		tbl.row(name, indicator, lpmVoterDescriptions[name])
	}
	// Include unknown voters not in the standard order.
	for name, val := range voters {
		known := false
		for _, k := range lpmVoterOrder {
			if name == k {
				known = true
				break
			}
		}
		if !known {
			indicator := "🟢"
			if val != 0 {
				indicator = "🔴"
			}
			tbl.row(name, indicator, "(unknown voter)")
		}
	}

	tbl.render(w, "  ")
}

// ══════════════════════════════════════════════════════════════════════
// Animated checklist display (gh CLI style)
// ══════════════════════════════════════════════════════════════════════

// Spinner frames (braille pattern, same as internal/log).
var clSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// voterColW0/1/2 are fixed column widths for the LPM voter table.
// Widths are determined by the widest value in each column across all known voters.
//
//   Col 0: "W_DISABLE" = 9 chars (widest voter name)
//   Col 1: emoji indicator (🟢/🔴) = 2 terminal cells
//   Col 2: "Hardware RF kill / BIOS disable pin (fix: PCOFFEN=2)" = 52 chars
const (
	voterColW0 = 9
	voterColW1 = 2
	voterColW2 = 52

	// voterSectionLines is the fixed number of lines occupied by the voter section:
	// blank + heading + top border + header + divider + 9 data rows + bottom border.
	voterSectionLines = 1 + 1 + 1 + 1 + 1 + 9 + 1
)

// checklist displays an animated list of items with braille spinners
// that transition to emoji badges as each result is determined.
// The live area also includes the LPM voter table and a summary line,
// all rendered together so they appear immediately and update in place.
type checklist struct {
	w          *os.File
	items      []clItem
	maxName    int
	voters     map[string]int // nil = still loading; set by SetVoters
	votersDone bool           // true after SetVoters is called
	liveLines  int            // total lines in the live area (fixed at startup)
	mu         sync.Mutex
	frame      int
	stop       chan struct{}
	done       chan struct{}
	noAnimate  bool
}

type clItem struct {
	name     string
	resolved bool
	badge    string
	status   string // statusPass / statusWarn / statusFail
	summary  string
}

// startChecklist prints the full initial live area (checklist rows + LPM voter
// table with spinner placeholders + summary line) and begins animation.
// Falls back to non-animated output when w is not a terminal.
func startChecklist(w *os.File, names []string) *checklist {
	cl := &checklist{
		w:     w,
		items: make([]clItem, len(names)),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}

	for i, name := range names {
		cl.items[i] = clItem{name: name}
		if len(name) > cl.maxName {
			cl.maxName = len(name)
		}
	}

	// Skip animation if not a terminal or quiet mode.
	if !isTerminal(w) || flagQuiet {
		cl.noAnimate = true
		close(cl.done)
		return cl
	}

	// liveLines = checklist rows + voter section + blank line + summary line.
	cl.liveLines = len(cl.items) + voterSectionLines + 1 + 1

	frame0 := clSpinnerFrames[0]
	ind0 := frame0 + " " // 2-cell loading placeholder: spinner + space

	// Print checklist rows.
	for _, item := range cl.items {
		fmt.Fprintf(w, "  %s %-*s\n", frame0, cl.maxName+2, item.name)
	}

	// Blank line before voter section.
	fmt.Fprintln(w)

	// Voter section heading.
	fmt.Fprintln(w, "  \U0001f4e1 Low Power Mode Voters:")

	// Voter table — top border.
	fmt.Fprintf(w, "  \u250c%s\u252c%s\u252c%s\u2510\n",
		strings.Repeat("\u2500", voterColW0+2),
		strings.Repeat("\u2500", voterColW1+2),
		strings.Repeat("\u2500", voterColW2+2))

	// Voter table — header row (indicator column has no label).
	fmt.Fprintf(w, "  \u2502 %-*s \u2502 %-*s \u2502 %-*s \u2502\n",
		voterColW0, "Voter",
		voterColW1, "",
		voterColW2, "Description")

	// Voter table — divider.
	fmt.Fprintf(w, "  \u251c%s\u253c%s\u253c%s\u2524\n",
		strings.Repeat("\u2500", voterColW0+2),
		strings.Repeat("\u2500", voterColW1+2),
		strings.Repeat("\u2500", voterColW2+2))

	// Voter table — data rows with spinner placeholders.
	for _, name := range lpmVoterOrder {
		desc := lpmVoterDescriptions[name]
		fmt.Fprintf(w, "  \u2502 %-*s \u2502 %s \u2502 %-*s \u2502\n",
			voterColW0, name, ind0, voterColW2, desc)
	}

	// Voter table — bottom border.
	fmt.Fprintf(w, "  \u2514%s\u2534%s\u2534%s\u2518\n",
		strings.Repeat("\u2500", voterColW0+2),
		strings.Repeat("\u2500", voterColW1+2),
		strings.Repeat("\u2500", voterColW2+2))

	// Blank line before summary.
	fmt.Fprintln(w)

	// Summary line.
	fmt.Fprintf(w, "  %s checking...\n", frame0)

	go cl.animate()

	return cl
}

// renderAll redraws the entire live area from the current baseline.
// Caller must hold cl.mu.
func (cl *checklist) renderAll() {
	frame := clSpinnerFrames[cl.frame%len(clSpinnerFrames)]
	ind := frame + " " // 2-cell loading placeholder: spinner + space
	pfx := "\r\033[K"

	fmt.Fprintf(cl.w, "\033[%dA", cl.liveLines) // move cursor to top of live area

	// Checklist rows.
	for _, item := range cl.items {
		if item.resolved {
			fmt.Fprintf(cl.w, "%s  %s %-*s  %s\n", pfx, item.badge, cl.maxName+2, item.name, item.summary)
		} else {
			fmt.Fprintf(cl.w, "%s  %s %-*s\n", pfx, frame, cl.maxName+2, item.name)
		}
	}

	// Blank line before voter section.
	fmt.Fprintf(cl.w, "%s\n", pfx)

	// Voter section heading.
	fmt.Fprintf(cl.w, "%s  \U0001f4e1 Low Power Mode Voters:\n", pfx)

	// Voter table — top border.
	fmt.Fprintf(cl.w, "%s  \u250c%s\u252c%s\u252c%s\u2510\n", pfx,
		strings.Repeat("\u2500", voterColW0+2),
		strings.Repeat("\u2500", voterColW1+2),
		strings.Repeat("\u2500", voterColW2+2))

	// Voter table — header row.
	fmt.Fprintf(cl.w, "%s  \u2502 %-*s \u2502 %-*s \u2502 %-*s \u2502\n", pfx,
		voterColW0, "Voter",
		voterColW1, "",
		voterColW2, "Description")

	// Voter table — divider.
	fmt.Fprintf(cl.w, "%s  \u251c%s\u253c%s\u253c%s\u2524\n", pfx,
		strings.Repeat("\u2500", voterColW0+2),
		strings.Repeat("\u2500", voterColW1+2),
		strings.Repeat("\u2500", voterColW2+2))

	// Voter table — data rows.
	for _, name := range lpmVoterOrder {
		desc := lpmVoterDescriptions[name]
		var rowInd string
		if !cl.votersDone {
			rowInd = ind // spinner + space (loading)
		} else if val, ok := cl.voters[name]; !ok {
			rowInd = "\u2500 " // "─ " — name not in map, 2 cells
		} else if val != 0 {
			rowInd = "\U0001f534" // 🔴
		} else {
			rowInd = "\U0001f7e2" // 🟢
		}
		fmt.Fprintf(cl.w, "%s  \u2502 %-*s \u2502 %s \u2502 %-*s \u2502\n", pfx,
			voterColW0, name, rowInd, voterColW2, desc)
	}

	// Voter table — bottom border.
	fmt.Fprintf(cl.w, "%s  \u2514%s\u2534%s\u2534%s\u2518\n", pfx,
		strings.Repeat("\u2500", voterColW0+2),
		strings.Repeat("\u2500", voterColW1+2),
		strings.Repeat("\u2500", voterColW2+2))

	// Blank line before summary.
	fmt.Fprintf(cl.w, "%s\n", pfx)

	// Summary line.
	fmt.Fprintf(cl.w, "%s%s\n", pfx, cl.summaryText(frame))
}

// summaryText returns the live summary line based on current checklist state.
// Shows progress while checks are pending; final result when all are resolved.
func (cl *checklist) summaryText(frame string) string {
	var pass, warn, fail, pending int
	for _, item := range cl.items {
		if !item.resolved {
			pending++
			continue
		}
		switch item.status {
		case statusPass:
			pass++
		case statusWarn:
			warn++
		case statusFail:
			fail++
		}
	}

	if pending > 0 {
		var parts []string
		if pass > 0 {
			parts = append(parts, fmt.Sprintf("%d passed", pass))
		}
		if warn > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", warn))
		}
		if fail > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", fail))
		}
		parts = append(parts, fmt.Sprintf("%d checking...", pending))
		return fmt.Sprintf("  %s %s", frame, strings.Join(parts, ", "))
	}

	var parts []string
	if pass > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", pass))
	}
	if warn > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warn))
	}
	if fail > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", fail))
	}
	if fail > 0 || warn > 0 {
		return fmt.Sprintf("  \U0001f4ca %s", strings.Join(parts, ", "))
	}
	return "  \U0001f389 All checks passed!"
}

// badgeStatus maps a diagBadge emoji back to its status constant.
func badgeStatus(badge string) string {
	switch badge {
	case "\u2705": // ✅
		return statusPass
	case "\u26a0\ufe0f ": // ⚠️ (trailing space compensates for terminal width variance)
		return statusWarn
	case "\u274c": // ❌
		return statusFail
	default:
		return ""
	}
}

func (cl *checklist) animate() {
	defer close(cl.done)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cl.stop:
			return
		case <-ticker.C:
			cl.mu.Lock()
			cl.frame++
			cl.renderAll()
			cl.mu.Unlock()
		}
	}
}

// Resolve updates a checklist item with its final badge and summary,
// immediately redrawing the full live area so the badge appears at once.
func (cl *checklist) Resolve(index int, badge, summary string) {
	if cl.noAnimate {
		fmt.Fprintf(cl.w, "  %s %-*s  %s\n", badge, cl.maxName+2, cl.items[index].name, summary)
		return
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	cl.items[index].resolved = true
	cl.items[index].badge = badge
	cl.items[index].status = badgeStatus(badge)
	cl.items[index].summary = summary

	cl.renderAll()
}

// SetVoters updates the LPM voter data and immediately redraws the live area.
// For the noAnimate path this is a no-op; voter data is printed separately after Finish.
func (cl *checklist) SetVoters(voters map[string]int) {
	if cl.noAnimate {
		return
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.voters = voters
	cl.votersDone = true
	cl.renderAll()
}

// Finish stops the animation and performs a final redraw to freeze the display.
func (cl *checklist) Finish() {
	if cl.noAnimate {
		return
	}
	close(cl.stop)
	<-cl.done
	// Final redraw: freeze spinner frames at their last position.
	cl.mu.Lock()
	cl.renderAll()
	cl.mu.Unlock()
}

// isTerminal reports whether f is connected to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ══════════════════════════════════════════════════════════════════════
// Box-drawing table builder
// ══════════════════════════════════════════════════════════════════════

// table renders bordered tables with box-drawing characters.
type table struct {
	headers []string
	rows    [][]string
	ncols   int
}

func newTable(headers ...string) *table {
	return &table{headers: headers, ncols: len(headers)}
}

func (t *table) row(cells ...string) {
	for len(cells) < t.ncols {
		cells = append(cells, "")
	}
	t.rows = append(t.rows, cells[:t.ncols])
}

func (t *table) render(w *os.File, indent string) {
	widths := t.computeWidths()
	t.drawBorder(w, widths, indent, "┌", "┬", "┐")
	if t.hasHeaders() {
		t.drawRow(w, t.headers, widths, indent)
		t.drawBorder(w, widths, indent, "├", "┼", "┤")
	}
	for _, row := range t.rows {
		t.drawRow(w, row, widths, indent)
	}
	t.drawBorder(w, widths, indent, "└", "┴", "┘")
}

// renderString returns the bordered table as a string, suitable for embedding
// inside a bubbletea View().
func (t *table) renderString(indent string) string {
	var b strings.Builder
	widths := t.computeWidths()
	t.drawBorderStr(&b, widths, indent, "┌", "┬", "┐")
	if t.hasHeaders() {
		t.drawRowStr(&b, t.headers, widths, indent)
		t.drawBorderStr(&b, widths, indent, "├", "┼", "┤")
	}
	for _, row := range t.rows {
		t.drawRowStr(&b, row, widths, indent)
	}
	t.drawBorderStr(&b, widths, indent, "└", "┴", "┘")
	return b.String()
}

func (t *table) drawBorderStr(b *strings.Builder, widths []int, indent, left, mid, right string) {
	b.WriteString(indent)
	b.WriteString(left)
	for i, width := range widths {
		b.WriteString(strings.Repeat("─", width+2))
		if i < len(widths)-1 {
			b.WriteString(mid)
		}
	}
	b.WriteString(right)
	b.WriteString("\n")
}

func (t *table) drawRowStr(b *strings.Builder, cells []string, widths []int, indent string) {
	b.WriteString(indent)
	b.WriteString("│")
	for i, width := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		pad := max(0, width-cellWidth(cell))
		fmt.Fprintf(b, " %s%s │", cell, strings.Repeat(" ", pad))
	}
	b.WriteString("\n")
}

func (t *table) hasHeaders() bool {
	for _, h := range t.headers {
		if h != "" {
			return true
		}
	}
	return false
}

func (t *table) computeWidths() []int {
	widths := make([]int, t.ncols)
	for i, h := range t.headers {
		if cw := cellWidth(h); cw > widths[i] {
			widths[i] = cw
		}
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) {
				if cw := cellWidth(cell); cw > widths[i] {
					widths[i] = cw
				}
			}
		}
	}
	return widths
}

func (t *table) drawBorder(w *os.File, widths []int, indent, left, mid, right string) {
	fmt.Fprint(w, indent, left)
	for i, width := range widths {
		fmt.Fprint(w, strings.Repeat("─", width+2))
		if i < len(widths)-1 {
			fmt.Fprint(w, mid)
		}
	}
	fmt.Fprintln(w, right)
}

func (t *table) drawRow(w *os.File, cells []string, widths []int, indent string) {
	fmt.Fprint(w, indent, "│")
	for i, width := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		pad := width - cellWidth(cell)
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(w, " %s%s │", cell, strings.Repeat(" ", pad))
	}
	fmt.Fprintln(w)
}

// cellWidth returns the visual width of a string in terminal cells.
// Handles ANSI escape sequences (zero width) and wide Unicode/emoji (2 cells).
func cellWidth(s string) int {
	w := 0
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		// Variation selector 16 — invisible, used in emoji sequences.
		if r == 0xFE0F {
			continue
		}
		if isWideRune(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// isWideRune reports whether a rune occupies 2 terminal cells.
func isWideRune(r rune) bool {
	return (r >= 0x2300 && r <= 0x23FF) || // Misc technical (⏭ etc.)
		(r >= 0x2600 && r <= 0x27BF) || // Misc symbols + Dingbats (⚠✅❌ etc.)
		(r >= 0x2B50 && r <= 0x2B55) || // Stars
		(r >= 0x1F000 && r <= 0x1FAFF) // All emoji blocks (🔧🟢🔴🩺 etc.)
}

// truncateToWidth truncates s so its visible width fits within maxWidth cells.
// ANSI escape sequences pass through without counting toward width.
func truncateToWidth(s string, maxWidth int) string {
	var b strings.Builder
	w := 0
	inEsc := false
	hadEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			hadEsc = true
			b.WriteRune(r)
			continue
		}
		if inEsc {
			b.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == 0xFE0F {
			b.WriteRune(r)
			continue
		}
		rw := 1
		if isWideRune(r) {
			rw = 2
		}
		if w+rw > maxWidth {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	// Only reset styling if the string actually contained ANSI escapes.
	if hadEsc {
		if inEsc {
			b.WriteString("m") // close malformed escape
		}
		b.WriteString("\033[0m")
	}
	return b.String()
}
