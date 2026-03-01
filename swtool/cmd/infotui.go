package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
)

// pollResultMsg carries one partial (or final) result from the streaming poll.
type pollResultMsg struct {
	info *modem.Info
	err  error
}

// streamUpdateMsg wraps a pollResultMsg plus the channel to read the next update.
type streamUpdateMsg struct {
	result pollResultMsg
	ch     <-chan pollResultMsg
}

// pollDoneMsg indicates the streaming poll channel has closed.
type pollDoneMsg struct{}

// initMsg triggers the first poll on program start (handled in Update so
// state changes are preserved — Init() has a value receiver).
type initMsg struct{}

// tickMsg triggers the next poll.
type tickMsg time.Time

// spinnerTickMsg triggers a spinner frame advance during polling.
type spinnerTickMsg struct{}

// tuiModel is the bubbletea model for the info watch dashboard.
type tuiModel struct {
	dev      *modem.Device
	interval time.Duration
	section  string // optional section filter (e.g. "signal")
	jsonDump bool   // if true, dump JSON on exit

	info    *modem.Info // merged display info (sticky values)
	rawInfo *modem.Info // last raw poll result (for JSON dump)
	lastErr error       // last poll error
	lastOK  time.Time   // time of last successful poll
	polling bool        // true while a poll goroutine is in flight

	// Per-field staleness: key is the display label, value is
	// the number of consecutive polls where this field was not refreshed.
	fieldAges map[string]int

	// Fields whose age was reset during the current poll cycle.
	// Populated during streaming updates; fields NOT in this set
	// get aged when the cycle completes (pollDoneMsg).
	cycleUpdated map[string]bool

	// QMI cache — shared across poll cycles. Signal info is always fresh;
	// serving system, system info, and cell location are cached.
	qmiCache *qmiCache

	width        int
	height       int
	scroll       int // vertical scroll offset (lines)
	spinnerFrame int // braille spinner frame counter

	quitting bool
}

// runInfoTUI launches the bubbletea TUI for live modem info.
// If section is non-empty, only that section is polled and displayed.
func runInfoTUI(dev *modem.Device, interval time.Duration, jsonOnExit bool, section string) error {
	if !isTerminal(os.Stdin) {
		return fmt.Errorf("--watch requires an interactive terminal")
	}

	m := tuiModel{
		dev:          dev,
		interval:     interval,
		section:      section,
		fieldAges:    make(map[string]int),
		cycleUpdated: make(map[string]bool),
		qmiCache:     &qmiCache{},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	final := result.(tuiModel)
	if final.jsonDump {
		// Dump the merged view — matches what the user sees on screen.
		// rawInfo is only the last streaming snapshot, which may be
		// incomplete if the poll was still in-flight when 'j' was pressed.
		info := final.info
		if info != nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(info)
		}
	}

	return nil
}

// isTerminal is defined in diagnostics.go

func (m tuiModel) Init() tea.Cmd {
	return func() tea.Msg { return initMsg{} }
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "j":
			m.quitting = true
			m.jsonDump = true
			return m, tea.Quit
		case "r":
			// Only start a poll if one isn't already running.
			if !m.polling {
				m.polling = true
				m.cycleUpdated = make(map[string]bool)
				ch := startStreamPoll(m.dev, m.qmiCache, m.section)
				return m, tea.Batch(waitStreamUpdate(ch), spinnerTick())
			}
		case "up", "k":
			m.scroll = max(0, m.scroll-1)
		case "down":
			m.scroll++
		case "pgup":
			m.scroll = max(0, m.scroll-m.height/2)
		case "pgdown":
			m.scroll += m.height / 2
		case "home":
			m.scroll = 0
		case "end":
			m.scroll = 999999 // clamped below
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinnerTickMsg:
		if m.polling {
			m.spinnerFrame++
			return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
		}

	case initMsg:
		m.polling = true
		ch := startStreamPoll(m.dev, m.qmiCache, m.section)
		return m, tea.Batch(waitStreamUpdate(ch), tickAfter(m.interval), spinnerTick())

	case streamUpdateMsg:
		if msg.result.err != nil {
			m.lastErr = msg.result.err
		} else {
			m.rawInfo = msg.result.info
			newFields := extractFields(msg.result.info)
			for k, nv := range newFields {
				if nv != "" {
					m.fieldAges[k] = 0
					m.cycleUpdated[k] = true
				}
			}
			m.info = mergeInfo(m.info, msg.result.info)
			m.lastErr = nil
			m.lastOK = time.Now()
		}
		return m, waitStreamUpdate(msg.ch)

	case pollDoneMsg:
		m.polling = false
		// Age fields that weren't updated during this poll cycle
		for k := range m.fieldAges {
			if !m.cycleUpdated[k] {
				m.fieldAges[k]++
			}
		}
		m.cycleUpdated = make(map[string]bool)

	case tickMsg:
		if m.polling {
			return m, tickAfter(m.interval)
		}
		m.polling = true
		m.cycleUpdated = make(map[string]bool)
		ch := startStreamPoll(m.dev, m.qmiCache, m.section)
		return m, tea.Batch(waitStreamUpdate(ch), tickAfter(m.interval), spinnerTick())
	}

	// Clamp scroll after every update — prevents getting stuck when content
	// length changes between renders (View has a value receiver so can't fix it there).
	m.clampScroll()

	return m, nil
}

// clampScroll ensures scroll offset is within valid bounds for current content.
func (m *tuiModel) clampScroll() {
	if m.info == nil || m.width == 0 || m.height == 0 {
		m.scroll = 0
		return
	}
	content := m.renderContent()
	lines := strings.Count(content, "\n") + 1
	fixedLines := 3 // header + badge bar + footer
	viewHeight := max(m.height-fixedLines, 1)
	maxScroll := max(0, lines-viewHeight)
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
}

func (m tuiModel) View() string {
	if m.quitting {
		return ""
	}
	if m.width < 20 || m.height < 5 {
		return "Terminal too small."
	}
	if m.info == nil && m.lastErr == nil {
		frame := clSpinnerFrames[m.spinnerFrame%len(clSpinnerFrames)]
		return fmt.Sprintf(" %s Loading modem info...", frame)
	}

	// Header line 1: title with spinner
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	titleSection := ""
	if m.section != "" {
		titleSection = " " + m.section
	}
	title := fmt.Sprintf(" 🩺 swtool info%s  %s", titleSection, m.lastOK.Format("15:04:05"))
	if m.polling {
		frame := clSpinnerFrames[m.spinnerFrame%len(clSpinnerFrames)]
		title += fmt.Sprintf("  %s updating", frame)
	}
	header := titleStyle.Width(m.width).Render(title)

	// Header line 2: health badge bar (hidden in section-filtered mode)
	var badgeLine string
	if m.section == "" {
		if checks := m.computeChecks(); len(checks) > 0 {
			var badges []string
			for _, c := range checks {
				badges = append(badges, fmt.Sprintf("%s %s", diagBadge(c.Status), checkDisplayName(c.Name)))
			}
			badgeLine = " " + strings.Join(badges, "  ")
		}
	}

	// Error line (fixed)
	var errLine string
	if m.lastErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
		errLine = errStyle.Render(fmt.Sprintf("  ❌ %v", m.lastErr))
	}

	// Scrollable content
	content := m.renderContent()
	lines := strings.Split(content, "\n")

	fixedLines := 3 // header + badge bar + footer
	if errLine != "" {
		fixedLines++
	}
	viewHeight := max(m.height-fixedLines, 1)

	end := min(m.scroll+viewHeight, len(lines))
	start := min(m.scroll, max(0, len(lines)-1))
	visible := strings.Join(lines[start:end], "\n")

	// Footer with scroll indicator
	footerStyle := lipgloss.NewStyle().Faint(true)
	scrollHint := ""
	maxScroll := max(1, len(lines)-viewHeight)
	if len(lines) > viewHeight {
		pct := m.scroll * 100 / maxScroll
		scrollHint = fmt.Sprintf("  %d%%", pct)
	}
	footer := footerStyle.Render(fmt.Sprintf("  [q]uit  [j]son  [r]efresh  [↑↓]scroll%s", scrollHint))

	parts := []string{header, badgeLine}
	if errLine != "" {
		parts = append(parts, errLine)
	}
	parts = append(parts, visible, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// minTwoColWidth is the minimum terminal width to use two-column layout.
const minTwoColWidth = 90

// renderContent renders all modem data in bordered panels, using two columns
// when the terminal is wide enough.
func (m tuiModel) renderContent() string {
	if m.info == nil {
		return "  (no data)"
	}

	info := m.info
	a := m.fieldAges
	checks := m.computeChecks()

	// Panel width for two-column vs single-column layout.
	// Section-filtered mode always uses full width.
	panelW := m.width
	if m.section == "" && m.width >= minTwoColWidth {
		panelW = m.width/2 - 1
	}
	labelW := min(20, max(12, panelW/3))

	identity := renderPanel("Identity", "", panelW, func(b *strings.Builder) {
		pf(b, "Manufacturer", info.Identity.Manufacturer, a["Manufacturer"], labelW)
		pf(b, "Model", info.Identity.Model, a["Model"], labelW)
		pf(b, "Revision", info.Identity.Revision, a["Revision"], labelW)
		pf(b, "HW Rev", info.Identity.HardwareRev, a["HW Rev"], labelW)
		pf(b, "IMEI", info.Identity.IMEI, a["IMEI"], labelW)
		pf(b, "MEID", info.Identity.MEID, a["MEID"], labelW)
		pf(b, "IMEI SV", info.Identity.IMEISV, a["IMEI SV"], labelW)
		pf(b, "FSN", info.Identity.FSN, a["FSN"], labelW)
	})

	sim := renderPanel("SIM", sectionBadge(checks, "sim_status"), panelW, func(b *strings.Builder) {
		pf(b, "Status", info.SIM.Status, a["SIM"], labelW)
		pf(b, "IMSI", info.SIM.IMSI, a["IMSI"], labelW)
		pf(b, "ICCID", info.SIM.ICCID, a["ICCID"], labelW)
	})

	signal := renderPanel("Signal", "", panelW, func(b *strings.Builder) {
		d := info.Diagnostics
		shown := make(map[string]bool)
		if d.RSSI != 0 {
			bars := renderBars(d.SignalBars, 5)
			pf(b, "RSSI", fmt.Sprintf("%s %d dBm", bars, d.RSSI), a["RSSI"], labelW)
			shown["RSSI (dBm)"] = true // bars already show RSSI
		}
		importantKeys := []string{
			"System mode", "PS state", "LTE band", "LTE bw",
			"LTE Rx chan", "LTE Tx chan", "EMM state", "RRC state",
			"RSSI (dBm)",
			"Tx Power", "TAC", "Cell ID", "Current Time",
		}
		for _, key := range importantKeys {
			if shown[key] {
				continue
			}
			if v, ok := d.GStatus[key]; ok {
				pf(b, key, formatGStatusValue(key, v), a[key], labelW)
				shown[key] = true
			}
		}
		if url := cellLookupURL(d.GStatus); url != "" {
			pf(b, "Cell Lookup", url, 0, labelW)
		}
		// Skip serving cell keys — shown in cell detail section below.
		for k := range servingCellGStatusKeys {
			shown[k] = true
		}
		var remaining []string
		for k := range d.GStatus {
			if !shown[k] {
				remaining = append(remaining, k)
			}
		}
		slices.Sort(remaining)
		for _, key := range remaining {
			pf(b, key, formatGStatusValue(key, d.GStatus[key]), a[key], labelW)
		}
		// Serving cell + PCC diversity + neighbor cells.
		renderCellTableStyled(b, info, a)
	})

	firmware := renderPanel("Firmware", sectionBadge(checks, "firmware_preference"), panelW, func(b *strings.Builder) {
		f := info.Firmware
		pf(b, "Cur FW", f.Current.Version, a["FW"], labelW)
		pf(b, "Cur Carrier", f.Current.CarrierName, a["Carrier"], labelW)
		pf(b, "Cur Config", f.Current.ConfigName, a["Config"], labelW)
		pf(b, "Pref FW", f.Preferred.Version, a["Pref FW"], labelW)
		pf(b, "Pref Carrier", f.Preferred.CarrierName, a["Pref Carrier"], labelW)
		pf(b, "Pref Config", f.Preferred.ConfigName, a["Pref Config"], labelW)
	})

	priID := renderPanel("PRI ID", "", panelW, func(b *strings.Builder) {
		f := info.Firmware
		pf(b, "Part Number", f.PRIID.PartNumber, a["PRI Part"], labelW)
		pf(b, "Revision", f.PRIID.Revision, a["PRI Rev"], labelW)
		pf(b, "Customer", f.PRIID.Customer, a["PRI Customer"], labelW)
		pf(b, "Carrier PRI", f.PRIID.CarrierPRI, a["Carrier PRI"], labelW)
	})

	network := renderPanel("Network", sectionBadge(checks, "network_registration"), panelW, func(b *strings.Builder) {
		if op := info.Operator; op != "" && op != "0" {
			pf(b, "Operator", op, a["Operator"], labelW)
		}
		pf(b, "EPS", info.Registration.EPS.Status, a["EPS"], labelW)
		pf(b, "CS", info.Registration.CS.Status, a["CS"], labelW)
		pf(b, "GPRS", info.Registration.GPRS.Status, a["GPRS"], labelW)
		if info.Network.RATSelection.Name != "" {
			pf(b, "RAT", fmt.Sprintf("%02d (%s)", info.Network.RATSelection.Index, info.Network.RATSelection.Name), a["RAT"], labelW)
		}
		if info.Network.CurrentBand.Name != "" {
			pf(b, "Current Band", fmt.Sprintf("%02d - %s", info.Network.CurrentBand.Index, info.Network.CurrentBand.Name), a["Current Band"], labelW)
		}
		for i, apn := range info.APNs {
			pf(b, fmt.Sprintf("APN %d", i+1), apn, a["APN"], labelW)
		}
	})

	usb := renderPanel("USB", sectionBadge(checks, "usb_composition"), panelW, func(b *strings.Builder) {
		u := info.USB
		pf(b, "VID", u.VID, a["VID"], labelW)
		pf(b, "PID App", u.PID.App, a["PID"], labelW)
		pf(b, "PID Boot", u.PID.Boot, a["PID Boot"], labelW)
		pf(b, "Product", u.Product, a["Product"], labelW)
		usbMode := u.Composition.Bitmask
		if len(u.Composition.Interfaces) > 0 {
			usbMode = joinStrings(u.Composition.Interfaces, ", ")
		}
		pf(b, "Composition", usbMode, a["USB Mode"], labelW)
		pf(b, "Config Type", u.Composition.ConfigType, a["Config Type"], labelW)
		pf(b, "Speed (Cur)", u.Speed.Current, a["Speed"], labelW)
		pf(b, "Speed (Max)", u.Speed.Supported, a["Speed Max"], labelW)
	})

	power := renderPanel("Power", sectionBadge(checks, "power_state"), panelW, func(b *strings.Builder) {
		pf(b, "State", info.Power.State, a["Power"], labelW)
		if info.Power.PCOFFEN != 0 {
			pf(b, "PCOFFEN", fmt.Sprintf("%d", info.Power.PCOFFEN), a["PCOFFEN"], labelW)
		}
		pf(b, "LPM Persist", info.Power.LPMPersistence, a["LPM Persist"], labelW)
		if len(info.Power.LPMVoters) > 0 {
			fmt.Fprintln(b, "LPM Voters:")
			for _, name := range lpmVoterOrder {
				val, ok := info.Power.LPMVoters[name]
				if !ok {
					continue
				}
				indicator := "🟢"
				if val != 0 {
					indicator = "🔴"
				}
				fmt.Fprintf(b, " %s %-12s %s\n", indicator, name, lpmVoterDescriptions[name])
			}
		}
	})

	var custom string
	if len(info.Custom) > 0 {
		custom = renderPanel("Custom Settings", "", panelW, func(b *strings.Builder) {
			var keys []string
			for k := range info.Custom {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				pf(b, k, info.Custom[k], a["custom:"+k], labelW)
			}
		})
	}

	gps := renderPanel("GPS", "", panelW, func(b *strings.Builder) {
		g := info.GPS
		pf(b, "Session", g.SessionStatus, a["GPS Session"], labelW)
		pf(b, "Fix Status", g.FixStatus, a["GPS"], labelW)
		pf(b, "Fix Type", g.FixType, a["GPS Fix Type"], labelW)
		pf(b, "TTFF (sec)", g.TTFF, a["GPS TTFF"], labelW)
		pf(b, "Latitude", g.Latitude, a["Latitude"], labelW)
		pf(b, "Longitude", g.Longitude, a["Longitude"], labelW)
		pf(b, "Altitude (m)", g.Altitude, a["Altitude"], labelW)
		pf(b, "HEPE (m)", g.HEPE, a["HEPE"], labelW)
		pf(b, "HDOP", g.HDOP, a["HDOP"], labelW)
		pf(b, "PDOP", g.PDOP, a["PDOP"], labelW)
		pf(b, "VDOP", g.VDOP, a["VDOP"], labelW)
		pf(b, "Heading", g.Heading, a["Heading"], labelW)
		pf(b, "Velocity (m/s)", g.Velocity, a["Velocity"], labelW)
		pf(b, "GPS Time", g.LocTimestamp, a["GPS Timestamp"], labelW)
		pf(b, "Satellites", gpsSatSummary(&g), a["Sats"], labelW)
		for _, s := range g.SatDetail {
			fmt.Fprintf(b, "  %-8s SV:%-3d  El:%2d  Az:%3d  SNR:%2d\n",
				s.System, s.PRN, s.Elevation, s.Azimuth, s.SNR)
		}
	})

	paths := renderPanel("Device Paths", "", panelW, func(b *strings.Builder) {
		pf(b, "Sysfs", m.dev.SysfsPath, 0, labelW)
		pf(b, "AT Port", m.dev.ATPort, 0, labelW)
		pf(b, "CDC Device", m.dev.CDCDevice, 0, labelW)
		if m.dev.QCQMIDevice != "" {
			pf(b, "QCQMI", m.dev.QCQMIDevice, 0, labelW)
		}
	})

	// Wide sections — always full-width in both layouts.
	var images, ca, bands string

	if len(info.Images.Firmware) > 0 || len(info.Images.PRI) > 0 {
		images = renderPanel("Firmware Images", sectionBadge(checks, "firmware_images"), m.width, func(b *strings.Builder) {
			if len(info.Images.Firmware) > 0 {
				fmt.Fprintf(b, "FW Slots (max %d, active: %d):\n", info.Images.MaxFW, info.Images.ActiveSlot)
				for _, s := range info.Images.Firmware {
					fmt.Fprintf(b, "  Slot %-4s %-6s LRU=%d Fail=%d  %s  %s\n",
						s.Slot, s.Status, s.LRU, s.Failures, s.UniqueID, s.BuildID)
				}
			}
			if len(info.Images.PRI) > 0 {
				fmt.Fprintf(b, "PRI Slots (max %d):\n", info.Images.MaxPRI)
				for _, s := range info.Images.PRI {
					fmt.Fprintf(b, "  Slot %-4s %-6s LRU=%d Fail=%d  %s  %s\n",
						s.Slot, s.Status, s.LRU, s.Failures, s.UniqueID, s.BuildID)
				}
			}
		})
	}

	if len(info.LTECA.Hardware) > 0 || len(info.LTECA.Permitted) > 0 {
		ca = renderPanel("Carrier Aggregation", "", m.width, func(b *strings.Builder) {
			if len(info.LTECA.Hardware) > 0 {
				fmt.Fprintln(b, "Hardware:")
				for _, c := range info.LTECA.Hardware {
					if len(c.Secondary) > 0 {
						fmt.Fprintf(b, "  %-6s + %s\n", c.Primary, joinStrings(c.Secondary, ", "))
					} else {
						fmt.Fprintf(b, "  %-6s (no combos)\n", c.Primary)
					}
				}
			}
			if len(info.LTECA.Permitted) > 0 {
				fmt.Fprintln(b, "Permitted:")
				for _, c := range info.LTECA.Permitted {
					if len(c.Secondary) > 0 {
						fmt.Fprintf(b, "  %-6s + %s\n", c.Primary, joinStrings(c.Secondary, ", "))
					} else {
						fmt.Fprintf(b, "  %-6s (no combos)\n", c.Primary)
					}
				}
			}
			if info.LTECA.Pruned != "" {
				pf(b, "Pruned", info.LTECA.Pruned, 0, labelW)
			}
		})
	}

	if len(info.Network.AvailableBands) > 0 {
		bands = renderPanel("Available Bands", "", m.width, func(b *strings.Builder) {
			for _, band := range info.Network.AvailableBands {
				fmt.Fprintf(b, "%02d  %-24s LTE=%s\n", band.Index, band.Name, band.LTEMask)
			}
		})
	}

	// --- Layout ---
	var result strings.Builder

	// Section-filtered mode: show only the matching panel.
	if m.section != "" {
		sectionPanels := map[string]string{
			"identity": identity,
			"sim":      sim,
			"signal":   signal,
			"firmware": firmware,
			"priid":    priID,
			"network":  network,
			"usb":      usb,
			"power":    power,
			"custom":   custom,
			"gps":      gps,
			"images":   images,
			"ca":       ca,
			"bands":    bands,
		}
		if p := sectionPanels[m.section]; strings.TrimSpace(p) != "" {
			result.WriteString(p)
			result.WriteString("\n")
		}
		return result.String()
	}

	if m.width >= minTwoColWidth {
		colWidth := m.width / 2

		leftParts := []string{identity, sim, signal, firmware, priID}
		rightParts := []string{network, usb, power}
		if custom != "" {
			rightParts = append(rightParts, custom)
		}
		rightParts = append(rightParts, gps, paths)

		left := joinNonEmpty(leftParts, "\n")
		right := joinNonEmpty(rightParts, "\n")

		leftLines := strings.Split(left, "\n")
		rightLines := strings.Split(right, "\n")

		maxLines := max(len(leftLines), len(rightLines))
		for len(leftLines) < maxLines {
			leftLines = append(leftLines, "")
		}
		for len(rightLines) < maxLines {
			rightLines = append(rightLines, "")
		}

		for i := range maxLines {
			l := leftLines[i]
			r := rightLines[i]
			padded := l + strings.Repeat(" ", max(0, colWidth-cellWidth(l)))
			result.WriteString(padded)
			result.WriteString(r)
			result.WriteString("\n")
		}
	} else {
		allParts := []string{identity, sim, signal, firmware, priID, network, usb, power}
		if custom != "" {
			allParts = append(allParts, custom)
		}
		allParts = append(allParts, gps, paths)
		result.WriteString(joinNonEmpty(allParts, "\n"))
		result.WriteString("\n")
	}

	for _, ws := range []string{images, ca, bands} {
		if strings.TrimSpace(ws) != "" {
			result.WriteString(ws)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// ══════════════════════════════════════════════════════════════════════
// Panel builder — bordered box-drawing sections for the TUI
// ══════════════════════════════════════════════════════════════════════

// renderPanel renders a bordered panel with a title embedded in the top border.
// badge is an optional status emoji placed after the title.
func renderPanel(title, badge string, width int, render func(b *strings.Builder)) string {
	if width < 10 {
		width = 10
	}
	var out strings.Builder

	// Top border: ┌─ Title badge ──────────────────┐
	titlePart := fmt.Sprintf("─ %s ", title)
	if badge != "" {
		titlePart += badge + " "
	}
	titleLen := cellWidth(titlePart)
	remaining := max(0, width-2-titleLen) // 2 for ┌ and ┐
	out.WriteString("┌")
	out.WriteString(titlePart)
	out.WriteString(strings.Repeat("─", remaining))
	out.WriteString("┐\n")

	// Body: render fields into a temp buffer, then wrap each line in │...│
	var body strings.Builder
	render(&body)

	innerWidth := width - 4 // "│ " + content + " │"
	for _, line := range strings.Split(strings.TrimRight(body.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		if cellWidth(line) > innerWidth {
			line = truncateToWidth(line, innerWidth-1) + "…"
		}
		pad := max(0, innerWidth-cellWidth(line))
		out.WriteString("│ ")
		out.WriteString(line)
		out.WriteString(strings.Repeat(" ", pad))
		out.WriteString(" │\n")
	}

	// Bottom border: └────────────────────────────────┘
	out.WriteString("└")
	out.WriteString(strings.Repeat("─", width-2))
	out.WriteString("┘")

	return out.String()
}

// pf writes a key-value field into a string builder for use inside renderPanel.
// No leading indent — the panel border handles framing.
func pf(b *strings.Builder, label, value string, age, labelWidth int) {
	if value == "" {
		return
	}
	styledLabel := staleStyle(fmt.Sprintf("%-*s", labelWidth, label+":"), -1)
	styledValue := staleStyle(value, age)
	fmt.Fprintf(b, "%s %s\n", styledLabel, styledValue)
}

// staleStyle applies staleness dimming to a string.
// age < 0 means always faint (used for labels).
func staleStyle(s string, age int) string {
	style := lipgloss.NewStyle()
	switch {
	case age < 0:
		style = style.Faint(true)
	case age >= 10:
		style = style.Foreground(lipgloss.Color("240"))
	case age >= 5:
		style = style.Foreground(lipgloss.Color("245"))
	case age >= 3:
		style = style.Foreground(lipgloss.Color("250"))
	}
	return style.Render(s)
}

// computeChecks derives health check results from the current merged info.
func (m tuiModel) computeChecks() []Check {
	if m.info == nil {
		return nil
	}
	return []Check{
		checkPowerState(m.info, ""),
		checkNetworkRegistration(m.info),
		checkFirmwarePreference(m.info),
		checkFirmwareImages(m.info),
		checkUSBComposition(m.info),
		checkSIMStatus(m.info),
	}
}

// sectionBadge returns the emoji badge for a named check in the list.
func sectionBadge(checks []Check, name string) string {
	for _, c := range checks {
		if c.Name == name {
			return diagBadge(c.Status)
		}
	}
	return ""
}

// joinNonEmpty joins non-empty strings with the separator.
func joinNonEmpty(parts []string, sep string) string {
	var filtered []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, sep)
}

// renderCellTableStyled appends an aligned signal table with TUI staleness
// styling for serving cell, PCC diversity, and neighbor cell rows.
func renderCellTableStyled(b *strings.Builder, info *modem.Info, ages map[string]int) {
	gstatus := info.Diagnostics.GStatus

	hasServing := len(info.LTEDetail.Serving) > 0 || hasGStatusSignal(gstatus)
	hasIntra := len(info.LTEDetail.IntraFreq) > 0
	hasInter := len(info.LTEDetail.InterFreq) > 0
	if !hasServing && !hasIntra && !hasInter {
		return
	}

	// styledRow writes one table row with label faint and values styled by age.
	styledRow := func(label string, earfcn, band, freq, bw, pci, rsrq, rsrp, rssi, snr string, ageKey string) {
		fmt.Fprintf(b, "%s  %s  %s  %s  %s  %s  %s  %s  %s  %s\n",
			staleStyle(fmt.Sprintf("%-11s", label), -1),
			staleStyle(fmt.Sprintf("%6s", earfcn), ages[ageKey]),
			staleStyle(fmt.Sprintf("%4s", band), ages[ageKey]),
			staleStyle(fmt.Sprintf("%7s", freq), ages[ageKey]),
			staleStyle(fmt.Sprintf("%3s", bw), ages[ageKey]),
			staleStyle(fmt.Sprintf("%4s", pci), ages[ageKey]),
			staleStyle(fmt.Sprintf("%6s", rsrq), ages[ageKey]),
			staleStyle(fmt.Sprintf("%6s", rsrp), ages[ageKey]),
			staleStyle(fmt.Sprintf("%6s", rssi), ages[ageKey]),
			staleStyle(fmt.Sprintf("%5s", snr), ages[ageKey]))
	}

	// Resolve serving EARFCN — used for serving row and intra-freq rows.
	servingEARFCN := "--"
	if len(info.LTEDetail.Serving) > 0 {
		servingEARFCN = cellVal(info.LTEDetail.Serving[0], "EARFCN")
	} else if v, ok := gstatus["LTE Rx chan"]; ok && v != "" {
		servingEARFCN = v
	}
	servingBand, servingFreq := earfcnBandFreq(servingEARFCN)

	// Resolve serving bandwidth from GStatus "LTE bw" (e.g. "20 MHz" → "20").
	servingBW := "--"
	if bw, ok := gstatus["LTE bw"]; ok && bw != "" {
		servingBW = strings.TrimSuffix(bw, " MHz")
	}

	// Header + separator (always faint).
	fmt.Fprintln(b, staleStyle(fmt.Sprintf(cellTableFmt, "", "EARFCN", "Band", "Freq", "BW", "PCI", "RSRQ", "RSRP", "RSSI", "SNR"), -1))
	fmt.Fprintln(b, staleStyle(cellTableSep, -1))

	// Serving cell.
	if len(info.LTEDetail.Serving) > 0 {
		s := info.LTEDetail.Serving[0]
		styledRow("Serving",
			servingEARFCN, servingBand, servingFreq, servingBW,
			cellVal(s, "PCI"),
			cellVal(s, "RSRQ"),
			cellVal(s, "RSRP"),
			cellVal(s, "RSSI"),
			cellVal(s, "SNR"),
			"RSRP")
	} else if hasGStatusSignal(gstatus) {
		styledRow("Serving",
			servingEARFCN, servingBand, servingFreq, servingBW,
			"--",
			valOr(gstatus, "RSRQ (dB)", "--"),
			valOr(gstatus, "RSRP (dBm)", "--"),
			"--",
			valOr(gstatus, "SINR (dB)", "--"),
			"RSRP (dBm)")
	}

	// PCC diversity sub-rows.
	rxdRSRP := gstatus["PCC RxD RSRP (dBm)"]
	rxdRSSI := gstatus["PCC RxD RSSI"]
	rxmRSSI := gstatus["PCC RxM RSSI"]
	if rxdRSRP != "" || rxdRSSI != "" {
		styledRow("RxDiversity", "--", "--", "--", "--", "--", "--",
			valOrDefault(rxdRSRP, "--"),
			valOrDefault(rxdRSSI, "--"),
			"--",
			"PCC RxD RSRP (dBm)")
	}
	if rxmRSSI != "" {
		styledRow("RxMIMO", "--", "--", "--", "--", "--", "--", "--", rxmRSSI, "--", "PCC RxM RSSI")
	}

	// IntraFreq neighbors.
	if hasIntra {
		fmt.Fprintln(b, staleStyle(cellTableSep, -1))
		for _, cell := range info.LTEDetail.IntraFreq {
			styledRow("Intra",
				servingEARFCN, servingBand, servingFreq, servingBW,
				cellVal(cell, "PCI"),
				cellVal(cell, "RSRQ"),
				cellVal(cell, "RSRP"),
				cellVal(cell, "RSSI"),
				cellVal(cell, "SNR"),
				"RSRQ")
		}
	}

	// InterFreq neighbors.
	if hasInter {
		fmt.Fprintln(b, staleStyle(cellTableSep, -1))
		for _, cell := range info.LTEDetail.InterFreq {
			earfcn := cellVal(cell, "EARFCN")
			band, freq := earfcnBandFreq(earfcn)
			styledRow("Inter",
				earfcn, band, freq, "--",
				cellVal(cell, "PCI"),
				cellVal(cell, "RSRQ"),
				cellVal(cell, "RSRP"),
				cellVal(cell, "RSSI"),
				cellVal(cell, "SNR"),
				"RSRQ")
		}
	}
}

func renderBars(filled, total int) string {
	var b strings.Builder
	for i := range total {
		if i < filled {
			b.WriteRune('\u2588') // full block
		} else {
			b.WriteRune('\u2591') // light shade
		}
	}
	return b.String()
}

// extractFields returns a flat map of display-label → value for all fields
// shown in the TUI. Used for change detection and age tracking.
func extractFields(info *modem.Info) map[string]string {
	if info == nil {
		return map[string]string{}
	}

	operator := info.Operator
	if operator == "0" {
		operator = ""
	}

	m := map[string]string{
		// Identity
		"Manufacturer": info.Identity.Manufacturer,
		"Model":        info.Identity.Model,
		"Revision":     info.Identity.Revision,
		"HW Rev":       info.Identity.HardwareRev,
		"IMEI":         info.Identity.IMEI,
		"MEID":         info.Identity.MEID,
		"IMEI SV":      info.Identity.IMEISV,
		"FSN":          info.Identity.FSN,
		// Firmware + SIM
		"FW":           info.Firmware.Current.Version,
		"Carrier":      info.Firmware.Current.CarrierName,
		"Config":       info.Firmware.Current.ConfigName,
		"Pref FW":      info.Firmware.Preferred.Version,
		"Pref Carrier": info.Firmware.Preferred.CarrierName,
		"Pref Config":  info.Firmware.Preferred.ConfigName,
		"PRI Part":     info.Firmware.PRIID.PartNumber,
		"PRI Rev":      info.Firmware.PRIID.Revision,
		"PRI Customer": info.Firmware.PRIID.Customer,
		"Carrier PRI":  info.Firmware.PRIID.CarrierPRI,
		"SIM":          info.SIM.Status,
		"IMSI":         info.SIM.IMSI,
		"ICCID":        info.SIM.ICCID,
		// USB
		"VID":         info.USB.VID,
		"PID":         info.USB.PID.App,
		"PID Boot":    info.USB.PID.Boot,
		"Product":     info.USB.Product,
		"Speed":       info.USB.Speed.Current,
		"Speed Max":   info.USB.Speed.Supported,
		"Config Type": info.USB.Composition.ConfigType,
		// Power
		"Power":       info.Power.State,
		"LPM Persist": info.Power.LPMPersistence,
		// Network
		"Operator": operator,
		"EPS":      info.Registration.EPS.Status,
		"CS":       info.Registration.CS.Status,
		"GPRS":     info.Registration.GPRS.Status,
		// GPS
		"GPS Session": info.GPS.SessionStatus,
		"GPS":         info.GPS.FixStatus,
		"GPS TTFF":    info.GPS.TTFF,
		"Latitude":    info.GPS.Latitude,
		"Longitude":   info.GPS.Longitude,
		"Altitude":    info.GPS.Altitude,
		"HDOP":          info.GPS.HDOP,
		"PDOP":          info.GPS.PDOP,
		"VDOP":          info.GPS.VDOP,
		"Heading":       info.GPS.Heading,
		"Velocity":      info.GPS.Velocity,
		"GPS Fix Type":  info.GPS.FixType,
		"HEPE":          info.GPS.HEPE,
		"GPS Timestamp": info.GPS.LocTimestamp,
	}

	// All gstatus keys — dynamic, tracks age for every key the modem reports
	for k, v := range info.Diagnostics.GStatus {
		m[k] = v
	}

	// All custom settings
	for k, v := range info.Custom {
		m["custom:"+k] = v
	}

	// RSSI
	if info.Diagnostics.RSSI != 0 {
		m["RSSI"] = fmt.Sprintf("%d", info.Diagnostics.RSSI)
	}

	// GPS satellites
	if info.GPS.Satellites > 0 {
		m["Sats"] = fmt.Sprintf("%d", info.GPS.Satellites)
	}

	// PCOFFEN
	if info.Power.PCOFFEN != 0 {
		m["PCOFFEN"] = fmt.Sprintf("%d", info.Power.PCOFFEN)
	}

	// RAT
	if info.Network.RATSelection.Name != "" {
		m["RAT"] = fmt.Sprintf("%02d (%s)", info.Network.RATSelection.Index, info.Network.RATSelection.Name)
	}

	// Current band
	if info.Network.CurrentBand.Name != "" {
		m["Current Band"] = fmt.Sprintf("%02d - %s", info.Network.CurrentBand.Index, info.Network.CurrentBand.Name)
	}

	// USB mode
	usbMode := info.USB.Composition.Bitmask
	if len(info.USB.Composition.Interfaces) > 0 {
		usbMode = joinStrings(info.USB.Composition.Interfaces, ", ")
	}
	m["USB Mode"] = usbMode

	// First APN
	if len(info.APNs) > 0 {
		m["APN"] = info.APNs[0]
	}

	return m
}

// mergeInfo produces a merged Info where empty/zero fields in src are
// filled from dst. This makes values "sticky" — once populated, they
// persist until replaced by a new non-empty value.
func mergeInfo(dst, src *modem.Info) *modem.Info {
	if dst == nil {
		return src
	}
	if src == nil {
		return dst
	}

	// Start from src (latest data), fill gaps from dst (old data)
	merged := *src

	// Identity
	mergeStr(&merged.Identity.Manufacturer, dst.Identity.Manufacturer)
	mergeStr(&merged.Identity.Model, dst.Identity.Model)
	mergeStr(&merged.Identity.Revision, dst.Identity.Revision)
	mergeStr(&merged.Identity.HardwareRev, dst.Identity.HardwareRev)
	mergeStr(&merged.Identity.MEID, dst.Identity.MEID)
	mergeStr(&merged.Identity.IMEI, dst.Identity.IMEI)
	mergeStr(&merged.Identity.IMEISV, dst.Identity.IMEISV)
	mergeStr(&merged.Identity.FSN, dst.Identity.FSN)
	mergeStr(&merged.Identity.GCAP, dst.Identity.GCAP)

	// Firmware
	mergeStr(&merged.Firmware.Preferred.Version, dst.Firmware.Preferred.Version)
	mergeStr(&merged.Firmware.Preferred.CarrierName, dst.Firmware.Preferred.CarrierName)
	mergeStr(&merged.Firmware.Preferred.ConfigName, dst.Firmware.Preferred.ConfigName)
	mergeStr(&merged.Firmware.Current.Version, dst.Firmware.Current.Version)
	mergeStr(&merged.Firmware.Current.CarrierName, dst.Firmware.Current.CarrierName)
	mergeStr(&merged.Firmware.Current.ConfigName, dst.Firmware.Current.ConfigName)
	mergeStr(&merged.Firmware.PRIID.PartNumber, dst.Firmware.PRIID.PartNumber)
	mergeStr(&merged.Firmware.PRIID.Revision, dst.Firmware.PRIID.Revision)
	mergeStr(&merged.Firmware.PRIID.Customer, dst.Firmware.PRIID.Customer)
	mergeStr(&merged.Firmware.PRIID.CarrierPRI, dst.Firmware.PRIID.CarrierPRI)

	// USB
	mergeStr(&merged.USB.VID, dst.USB.VID)
	mergeStr(&merged.USB.PID.App, dst.USB.PID.App)
	mergeStr(&merged.USB.PID.Boot, dst.USB.PID.Boot)
	mergeStr(&merged.USB.Product, dst.USB.Product)
	mergeStr(&merged.USB.Speed.Supported, dst.USB.Speed.Supported)
	mergeStr(&merged.USB.Speed.Current, dst.USB.Speed.Current)
	mergeStr(&merged.USB.Composition.Bitmask, dst.USB.Composition.Bitmask)
	mergeStr(&merged.USB.Composition.ConfigType, dst.USB.Composition.ConfigType)
	if len(merged.USB.Composition.Interfaces) == 0 {
		merged.USB.Composition.Interfaces = dst.USB.Composition.Interfaces
	}

	// Network
	mergeStr(&merged.Network.RATSelection.Name, dst.Network.RATSelection.Name)
	if merged.Network.RATSelection.Index == 0 {
		merged.Network.RATSelection.Index = dst.Network.RATSelection.Index
	}
	mergeStr(&merged.Network.CurrentBand.Name, dst.Network.CurrentBand.Name)
	mergeStr(&merged.Network.CurrentBand.GWMask, dst.Network.CurrentBand.GWMask)
	mergeStr(&merged.Network.CurrentBand.LTEMask, dst.Network.CurrentBand.LTEMask)
	mergeStr(&merged.Network.CurrentBand.TDSMask, dst.Network.CurrentBand.TDSMask)
	if merged.Network.CurrentBand.Index == 0 {
		merged.Network.CurrentBand.Index = dst.Network.CurrentBand.Index
	}
	if len(merged.Network.AvailableBands) == 0 {
		merged.Network.AvailableBands = dst.Network.AvailableBands
	}

	// Power
	mergeStr(&merged.Power.State, dst.Power.State)
	mergeStr(&merged.Power.LPMPersistence, dst.Power.LPMPersistence)
	if merged.Power.PCOFFEN == 0 {
		merged.Power.PCOFFEN = dst.Power.PCOFFEN
	}
	if len(merged.Power.LPMVoters) == 0 {
		merged.Power.LPMVoters = dst.Power.LPMVoters
	}

	// Diagnostics — merge GStatus map
	if merged.Diagnostics.GStatus == nil {
		merged.Diagnostics.GStatus = make(map[string]string)
	}
	for k, v := range dst.Diagnostics.GStatus {
		if _, ok := merged.Diagnostics.GStatus[k]; !ok {
			merged.Diagnostics.GStatus[k] = v
		}
	}
	if merged.Diagnostics.RSSI == 0 {
		merged.Diagnostics.RSSI = dst.Diagnostics.RSSI
		merged.Diagnostics.SignalBars = dst.Diagnostics.SignalBars
		merged.Diagnostics.BER = dst.Diagnostics.BER
	}

	// GPS
	mergeStr(&merged.GPS.SessionStatus, dst.GPS.SessionStatus)
	mergeStr(&merged.GPS.FixStatus, dst.GPS.FixStatus)
	mergeStr(&merged.GPS.Latitude, dst.GPS.Latitude)
	mergeStr(&merged.GPS.Longitude, dst.GPS.Longitude)
	mergeStr(&merged.GPS.Altitude, dst.GPS.Altitude)
	mergeStr(&merged.GPS.HDOP, dst.GPS.HDOP)
	mergeStr(&merged.GPS.PDOP, dst.GPS.PDOP)
	mergeStr(&merged.GPS.VDOP, dst.GPS.VDOP)
	mergeStr(&merged.GPS.Heading, dst.GPS.Heading)
	mergeStr(&merged.GPS.Velocity, dst.GPS.Velocity)
	mergeStr(&merged.GPS.TTFF, dst.GPS.TTFF)
	mergeStr(&merged.GPS.FixType, dst.GPS.FixType)
	mergeStr(&merged.GPS.HEPE, dst.GPS.HEPE)
	mergeStr(&merged.GPS.LocTimestamp, dst.GPS.LocTimestamp)
	if merged.GPS.Satellites == 0 {
		merged.GPS.Satellites = dst.GPS.Satellites
	}
	if len(merged.GPS.SatDetail) == 0 {
		merged.GPS.SatDetail = dst.GPS.SatDetail
	}

	// SIM
	mergeStr(&merged.SIM.Status, dst.SIM.Status)
	mergeStr(&merged.SIM.IMSI, dst.SIM.IMSI)
	mergeStr(&merged.SIM.ICCID, dst.SIM.ICCID)

	// Registration
	mergeStr(&merged.Registration.CS.Status, dst.Registration.CS.Status)
	mergeStr(&merged.Registration.CS.Raw, dst.Registration.CS.Raw)
	mergeStr(&merged.Registration.EPS.Status, dst.Registration.EPS.Status)
	mergeStr(&merged.Registration.EPS.Raw, dst.Registration.EPS.Raw)
	mergeStr(&merged.Registration.GPRS.Status, dst.Registration.GPRS.Status)
	mergeStr(&merged.Registration.GPRS.Raw, dst.Registration.GPRS.Raw)

	// Operator
	mergeStr(&merged.Operator, dst.Operator)

	// APNs
	if len(merged.APNs) == 0 {
		merged.APNs = dst.APNs
	}

	// Custom
	if len(merged.Custom) == 0 {
		merged.Custom = dst.Custom
	} else if dst.Custom != nil {
		for k, v := range dst.Custom {
			if _, ok := merged.Custom[k]; !ok {
				merged.Custom[k] = v
			}
		}
	}

	// LTE detail — keep old if new is empty
	if len(merged.LTEDetail.Serving) == 0 {
		merged.LTEDetail.Serving = dst.LTEDetail.Serving
	}
	if len(merged.LTEDetail.IntraFreq) == 0 {
		merged.LTEDetail.IntraFreq = dst.LTEDetail.IntraFreq
	}
	if len(merged.LTEDetail.InterFreq) == 0 {
		merged.LTEDetail.InterFreq = dst.LTEDetail.InterFreq
	}
	if len(merged.LTECA.Hardware) == 0 {
		merged.LTECA = dst.LTECA
	}

	// Images — keep old if new is empty
	if len(merged.Images.Firmware) == 0 {
		merged.Images = dst.Images
	}

	return &merged
}

func mergeStr(dst *string, old string) {
	if *dst == "" {
		*dst = old
	}
}

// startStreamPoll launches a goroutine that opens the AT port and streams
// modem info updates through a channel. Each send from GetInfoStreaming
// produces one message; the channel is closed when the poll completes.
// QMI-sourced telemetry is overlaid when qmicli and a control device are available.
// Signal info is queried every callback; slower queries use the shared cache
// and only refresh on the first callback of each cycle.
//
// When section is non-empty, only that section's AT commands are queried
// (via GetInfoForSection) and a single result is sent.
func startStreamPoll(dev *modem.Device, cache *qmiCache, section string) <-chan pollResultMsg {
	ch := make(chan pollResultMsg, 2)
	go func() {
		defer close(ch)

		qc := createQMIClient(dev)
		if qc != nil {
			defer qc.Close()
		}

		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			ch <- pollResultMsg{err: fmt.Errorf("opening AT port: %w", err)}
			return
		}
		defer port.Close()

		if section != "" {
			info := modem.GetInfoForSection(port, section)
			if qc != nil {
				overlayQMIData(qc, info)
			}
			ch <- pollResultMsg{info: info}
			return
		}

		first := true
		modem.GetInfoStreaming(port, func(info *modem.Info) {
			if qc != nil {
				overlayQMICached(qc, cache, first, info)
				first = false
			}
			ch <- pollResultMsg{info: info}
		})
	}()
	return ch
}

// waitStreamUpdate returns a tea.Cmd that reads the next update from a
// streaming poll channel. Returns pollDoneMsg when the channel closes.
func waitStreamUpdate(ch <-chan pollResultMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return pollDoneMsg{}
		}
		return streamUpdateMsg{result: msg, ch: ch}
	}
}

func tickAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}
