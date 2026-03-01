package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/spf13/cobra"
)

var (
	flagInfoJSON     bool
	flagInfoWatch    bool
	flagInfoInterval time.Duration
)

// validSections lists the allowed section names for `swtool info [section]`.
// "ltedetail" is an alias for "signal" (LTE detail is shown inside Signal).
var validSections = []string{
	"identity", "power", "sim", "network", "firmware", "priid",
	"usb", "images", "signal", "gps", "bands", "custom", "ca",
}

var infoCmd = &cobra.Command{
	Use:       "info [section]",
	Short:     "Display current modem settings (read-only, safe)",
	Long:      "Queries the modem via AT commands and displays all configuration settings.\n\nOptionally pass a section name to show only that section in plain format.\nValid sections: " + strings.Join(validSections, ", "),
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: validSections,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := requireModem()

		if dev.ATPort == "" {
			return fmt.Errorf("no AT port found for modem %s", dev.Name)
		}

		// Resolve optional section filter.
		var section string
		if len(args) > 0 {
			section = strings.ToLower(args[0])
			// "ltedetail" is an alias for "signal"
			if section == "ltedetail" {
				section = "signal"
			}
			if !slices.Contains(validSections, section) {
				return fmt.Errorf("unknown section %q (valid: %s)", section, strings.Join(validSections, ", "))
			}
		}

		if flagInfoWatch {
			return runInfoTUI(dev, flagInfoInterval, flagInfoJSON, section)
		}

		port, err := modem.OpenPort(dev.ATPort, logger)
		if err != nil {
			return err
		}
		defer port.Close()

		// JSON path: collect all data first (non-streaming).
		if flagInfoJSON {
			info, err := modem.GetInfo(port)
			if err != nil {
				return fmt.Errorf("reading modem info: %w", err)
			}
			if qc := createQMIClient(dev); qc != nil {
				overlayQMIData(qc, info)
				qc.Close()
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}

		// Filtered path: query only the AT commands needed for this section.
		if section != "" {
			info := modem.GetInfoForSection(port, section)
			printInfoSection(dev, info, section)
			return nil
		}

		printInfoStreaming(dev, port)
		return nil
	},
}

func init() {
	infoCmd.Flags().BoolVar(&flagInfoJSON, "json", false, "output as JSON")
	infoCmd.Flags().BoolVarP(&flagInfoWatch, "watch", "w", false, "live-updating TUI dashboard")
	infoCmd.Flags().DurationVar(&flagInfoInterval, "interval", 3*time.Second, "polling interval for --watch mode")
	rootCmd.AddCommand(infoCmd)
}

// printInfoStreaming queries the modem with GetInfoStreaming and prints
// bordered panels progressively as each AT command group completes.
// QMI data is queried once upfront and overlaid into each streaming snapshot.
func printInfoStreaming(dev *modem.Device, port *modem.Port) {
	const panelW = 80
	const labelW = 20
	w := os.Stdout

	// Pre-query QMI data once — overlaid into every streaming callback.
	qc := createQMIClient(dev)
	if qc != nil {
		defer qc.Close()
	}

	// Print device paths as a header — these are known before any AT commands.
	fmt.Fprintf(w, " %s  AT: %s", dev.SysfsPath, dev.ATPort)
	if dev.CDCDevice != "" {
		fmt.Fprintf(w, "  CDC: %s", dev.CDCDevice)
	}
	if dev.QCQMIDevice != "" {
		fmt.Fprintf(w, "  QCQMI: %s", dev.QCQMIDevice)
	}
	fmt.Fprintln(w)

	printed := make(map[string]bool)

	modem.GetInfoStreaming(port, func(info *modem.Info) {
		if qc != nil {
			overlayQMIData(qc, info)
		}

		// Identity — available after ATI (before first send point).
		if !printed["identity"] && info.Identity.Manufacturer != "" {
			printed["identity"] = true
			printPanel(w, "Identity", "", panelW, labelW, func(b *strings.Builder) {
				sf(b, "Manufacturer", info.Identity.Manufacturer, labelW)
				sf(b, "Model", info.Identity.Model, labelW)
				sf(b, "Revision", info.Identity.Revision, labelW)
				sf(b, "HW Rev", info.Identity.HardwareRev, labelW)
				sf(b, "IMEI", info.Identity.IMEI, labelW)
				sf(b, "MEID", info.Identity.MEID, labelW)
				sf(b, "IMEI SV", info.Identity.IMEISV, labelW)
				sf(b, "FSN", info.Identity.FSN, labelW)
			})
		}

		// Power — available once AT!PCINFO? has been queried.
		if !printed["power"] && info.Power.State != "" {
			printed["power"] = true
			printPanel(w, "Power", diagBadge(checkPowerState(info, "").Status), panelW, labelW, func(b *strings.Builder) {
				sf(b, "State", info.Power.State, labelW)
				if info.Power.PCOFFEN != 0 {
					sf(b, "PCOFFEN", fmt.Sprintf("%d", info.Power.PCOFFEN), labelW)
				}
				sf(b, "LPM Persist", info.Power.LPMPersistence, labelW)
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
		}

		// SIM — available once AT+CPIN? has been queried.
		if !printed["sim"] && info.SIM.Status != "" {
			printed["sim"] = true
			printPanel(w, "SIM", diagBadge(checkSIMStatus(info).Status), panelW, labelW, func(b *strings.Builder) {
				sf(b, "Status", info.SIM.Status, labelW)
				sf(b, "IMSI", info.SIM.IMSI, labelW)
				sf(b, "ICCID", info.SIM.ICCID, labelW)
			})
		}

		// Network — available once registration queries complete.
		if !printed["network"] && info.Registration.EPS.Status != "" {
			printed["network"] = true
			printPanel(w, "Network", diagBadge(checkNetworkRegistration(info).Status), panelW, labelW, func(b *strings.Builder) {
				if op := info.Operator; op != "" && op != "0" {
					sf(b, "Operator", op, labelW)
				}
				sf(b, "EPS", info.Registration.EPS.Status, labelW)
				sf(b, "CS", info.Registration.CS.Status, labelW)
				sf(b, "GPRS", info.Registration.GPRS.Status, labelW)
			})
		}

		// Firmware preference — available once AT!IMPREF? returns.
		if !printed["firmware"] && info.Firmware.Preferred.Version != "" {
			printed["firmware"] = true
			printPanel(w, "Firmware", diagBadge(checkFirmwarePreference(info).Status), panelW, labelW, func(b *strings.Builder) {
				f := info.Firmware
				sf(b, "Cur FW", f.Current.Version, labelW)
				sf(b, "Cur Carrier", f.Current.CarrierName, labelW)
				sf(b, "Cur Config", f.Current.ConfigName, labelW)
				sf(b, "Pref FW", f.Preferred.Version, labelW)
				sf(b, "Pref Carrier", f.Preferred.CarrierName, labelW)
				sf(b, "Pref Config", f.Preferred.ConfigName, labelW)
			})
		}

		// PRI ID — available once AT!PRIID? returns.
		if !printed["priid"] && info.Firmware.PRIID.PartNumber != "" {
			printed["priid"] = true
			printPanel(w, "PRI ID", "", panelW, labelW, func(b *strings.Builder) {
				p := info.Firmware.PRIID
				sf(b, "Part Number", p.PartNumber, labelW)
				sf(b, "Revision", p.Revision, labelW)
				sf(b, "Customer", p.Customer, labelW)
				sf(b, "Carrier PRI", p.CarrierPRI, labelW)
			})
		}

		// USB — wait for composition data so the panel is complete.
		if !printed["usb"] && info.USB.Composition.Bitmask != "" {
			printed["usb"] = true
			printPanel(w, "USB", diagBadge(checkUSBIdentity(info).Status), panelW, labelW, func(b *strings.Builder) {
				u := info.USB
				sf(b, "VID", u.VID, labelW)
				sf(b, "PID (App)", u.PID.App, labelW)
				sf(b, "PID (Boot)", u.PID.Boot, labelW)
				sf(b, "Product", u.Product, labelW)
				sf(b, "Speed Supported", u.Speed.Supported, labelW)
				sf(b, "Speed Current", u.Speed.Current, labelW)
				comp := u.Composition.Bitmask
				if len(u.Composition.Interfaces) > 0 {
					comp = fmt.Sprintf("%s (%s)", u.Composition.Bitmask, joinStrings(u.Composition.Interfaces, ", "))
				}
				sf(b, "Composition", comp, labelW)
			})
		}

		// Firmware images — available once AT!IMAGE? returns.
		if !printed["images"] && (len(info.Images.Firmware) > 0 || len(info.Images.PRI) > 0) {
			printed["images"] = true
			printPanel(w, "Firmware Images", diagBadge(checkFirmwareImages(info).Status), panelW, labelW, func(b *strings.Builder) {
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

		// Network detail — available once AT!SELRAT?/AT!BAND? return.
		if !printed["netdetail"] && (info.Network.RATSelection.Name != "" || info.Network.CurrentBand.Name != "") {
			printed["netdetail"] = true
			printPanel(w, "Network Detail", "", panelW, labelW, func(b *strings.Builder) {
				if info.Network.RATSelection.Name != "" {
					sf(b, "RAT", fmt.Sprintf("%02d (%s)", info.Network.RATSelection.Index, info.Network.RATSelection.Name), labelW)
				}
				if info.Network.CurrentBand.Name != "" {
					sf(b, "Current Band", fmt.Sprintf("%02d - %s", info.Network.CurrentBand.Index, info.Network.CurrentBand.Name), labelW)
				}
				for i, apn := range info.APNs {
					sf(b, fmt.Sprintf("APN %d", i+1), apn, labelW)
				}
			})
		}

		// Available bands — available once AT!BAND=? returns.
		if !printed["bands"] && len(info.Network.AvailableBands) > 0 {
			printed["bands"] = true
			printPanel(w, "Available Bands", "", panelW, labelW, func(b *strings.Builder) {
				for _, band := range info.Network.AvailableBands {
					fmt.Fprintf(b, "%02d  %-24s LTE=%s\n", band.Index, band.Name, band.LTEMask)
				}
			})
		}

		// Custom settings — available once AT!CUSTOM? returns.
		if !printed["custom"] && len(info.Custom) > 0 {
			printed["custom"] = true
			printPanel(w, "Custom Settings", "", panelW, labelW, func(b *strings.Builder) {
				for k, v := range info.Custom {
					sf(b, k, v, labelW)
				}
			})
		}

		// Signal / GStatus — available once AT!GSTATUS? or QMI data is present.
		if !printed["signal"] && len(info.Diagnostics.GStatus) > 0 {
			printed["signal"] = true
			printPanel(w, "Signal", "", panelW, labelW, func(b *strings.Builder) {
				d := info.Diagnostics
				skip := make(map[string]bool)
				if d.RSSI != 0 {
					bars := renderBars(d.SignalBars, 5)
					sf(b, "RSSI", fmt.Sprintf("%s %d dBm", bars, d.RSSI), labelW)
					skip["RSSI (dBm)"] = true // bars already show RSSI
				}
				for _, key := range []string{
					"System mode", "PS state", "LTE band", "LTE bw",
					"LTE Rx chan", "LTE Tx chan", "EMM state", "RRC state",
					"RSSI (dBm)", "RSRP (dBm)", "RSRQ (dB)", "SINR (dB)",
					"Tx Power", "TAC", "Cell ID", "Current Time",
				} {
					if skip[key] {
						continue
					}
					if v, ok := d.GStatus[key]; ok {
						sf(b, key, formatGStatusValue(key, v), labelW)
					}
				}
				if url := cellLookupURL(d.GStatus); url != "" {
					sf(b, "Cell Lookup", url, labelW)
				}
				renderLTEDetail(b, info, labelW)
			})
		}

		// GPS — available once AT!GPSSTATUS? returns.
		if !printed["gps"] && (info.GPS.FixStatus != "" || info.GPS.SessionStatus != "") {
			printed["gps"] = true
			printPanel(w, "GPS", "", panelW, labelW, func(b *strings.Builder) {
				sf(b, "Session", info.GPS.SessionStatus, labelW)
				sf(b, "Fix Status", info.GPS.FixStatus, labelW)
				sf(b, "Latitude", info.GPS.Latitude, labelW)
				sf(b, "Longitude", info.GPS.Longitude, labelW)
				sf(b, "Altitude (m)", info.GPS.Altitude, labelW)
				if info.GPS.Satellites > 0 {
					sf(b, "Satellites", fmt.Sprintf("%d", info.GPS.Satellites), labelW)
				}
			})
		}

		// Carrier Aggregation — available once AT!LTECA? returns.
		if !printed["ca"] && (len(info.LTECA.Hardware) > 0 || len(info.LTECA.Permitted) > 0) {
			printed["ca"] = true
			printPanel(w, "Carrier Aggregation", "", panelW, labelW, func(b *strings.Builder) {
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
					sf(b, "Pruned", info.LTECA.Pruned, labelW)
				}
			})
		}
	})
}

// printPanel renders a bordered panel to w and flushes immediately.
// Skips output if the body callback produces no visible content.
func printPanel(w *os.File, title, badge string, panelW, labelW int, render func(b *strings.Builder)) {
	var body strings.Builder
	render(&body)
	if strings.TrimSpace(body.String()) == "" {
		return
	}
	panel := renderPanel(title, badge, panelW, func(b *strings.Builder) {
		b.WriteString(body.String())
	})
	fmt.Fprintln(w, panel)
}

// printInfoSection renders a single section in plain format from pre-queried data.
// Called when the user specifies a section filter (e.g. "swtool info signal").
func printInfoSection(dev *modem.Device, info *modem.Info, section string) {
	const labelW = 20
	w := os.Stdout

	// QMI-first: overlay QMI data for sections where it replaces AT commands.
	if section == "signal" || section == "network" {
		if qc := createQMIClient(dev); qc != nil {
			overlayQMIData(qc, info)
			qc.Close()
		}
	}

	switch section {
	case "identity":
		printSectionFields(w, "Identity", labelW, func(b *strings.Builder) {
			sf(b, "Manufacturer", info.Identity.Manufacturer, labelW)
			sf(b, "Model", info.Identity.Model, labelW)
			sf(b, "Revision", info.Identity.Revision, labelW)
			sf(b, "HW Rev", info.Identity.HardwareRev, labelW)
			sf(b, "IMEI", info.Identity.IMEI, labelW)
			sf(b, "MEID", info.Identity.MEID, labelW)
			sf(b, "IMEI SV", info.Identity.IMEISV, labelW)
			sf(b, "FSN", info.Identity.FSN, labelW)
		})
	case "power":
		printSectionFields(w, "Power", labelW, func(b *strings.Builder) {
			sf(b, "State", info.Power.State, labelW)
			if info.Power.PCOFFEN != 0 {
				sf(b, "PCOFFEN", fmt.Sprintf("%d", info.Power.PCOFFEN), labelW)
			}
			sf(b, "LPM Persist", info.Power.LPMPersistence, labelW)
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
	case "sim":
		printSectionFields(w, "SIM", labelW, func(b *strings.Builder) {
			sf(b, "Status", info.SIM.Status, labelW)
			sf(b, "IMSI", info.SIM.IMSI, labelW)
			sf(b, "ICCID", info.SIM.ICCID, labelW)
		})
	case "network":
		printSectionFields(w, "Network", labelW, func(b *strings.Builder) {
			if op := info.Operator; op != "" && op != "0" {
				sf(b, "Operator", op, labelW)
			}
			sf(b, "EPS", info.Registration.EPS.Status, labelW)
			sf(b, "CS", info.Registration.CS.Status, labelW)
			sf(b, "GPRS", info.Registration.GPRS.Status, labelW)
			if info.Network.RATSelection.Name != "" {
				sf(b, "RAT", fmt.Sprintf("%02d (%s)", info.Network.RATSelection.Index, info.Network.RATSelection.Name), labelW)
			}
			if info.Network.CurrentBand.Name != "" {
				sf(b, "Current Band", fmt.Sprintf("%02d - %s", info.Network.CurrentBand.Index, info.Network.CurrentBand.Name), labelW)
			}
			for i, apn := range info.APNs {
				sf(b, fmt.Sprintf("APN %d", i+1), apn, labelW)
			}
		})
	case "firmware":
		printSectionFields(w, "Firmware", labelW, func(b *strings.Builder) {
			f := info.Firmware
			sf(b, "Cur FW", f.Current.Version, labelW)
			sf(b, "Cur Carrier", f.Current.CarrierName, labelW)
			sf(b, "Cur Config", f.Current.ConfigName, labelW)
			sf(b, "Pref FW", f.Preferred.Version, labelW)
			sf(b, "Pref Carrier", f.Preferred.CarrierName, labelW)
			sf(b, "Pref Config", f.Preferred.ConfigName, labelW)
		})
	case "priid":
		printSectionFields(w, "PRI ID", labelW, func(b *strings.Builder) {
			p := info.Firmware.PRIID
			sf(b, "Part Number", p.PartNumber, labelW)
			sf(b, "Revision", p.Revision, labelW)
			sf(b, "Customer", p.Customer, labelW)
			sf(b, "Carrier PRI", p.CarrierPRI, labelW)
		})
	case "usb":
		printSectionFields(w, "USB", labelW, func(b *strings.Builder) {
			u := info.USB
			sf(b, "VID", u.VID, labelW)
			sf(b, "PID (App)", u.PID.App, labelW)
			sf(b, "PID (Boot)", u.PID.Boot, labelW)
			sf(b, "Product", u.Product, labelW)
			sf(b, "Speed Supported", u.Speed.Supported, labelW)
			sf(b, "Speed Current", u.Speed.Current, labelW)
			comp := u.Composition.Bitmask
			if len(u.Composition.Interfaces) > 0 {
				comp = fmt.Sprintf("%s (%s)", u.Composition.Bitmask, joinStrings(u.Composition.Interfaces, ", "))
			}
			sf(b, "Composition", comp, labelW)
		})
	case "images":
		printSectionFields(w, "Firmware Images", labelW, func(b *strings.Builder) {
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
	case "signal":
		printSectionFields(w, "Signal", labelW, func(b *strings.Builder) {
			d := info.Diagnostics
			shown := make(map[string]bool)
			if d.RSSI != 0 {
				bars := renderBars(d.SignalBars, 5)
				sf(b, "RSSI", fmt.Sprintf("%s %d dBm", bars, d.RSSI), labelW)
				shown["RSSI (dBm)"] = true // bars already show RSSI
			}
			importantKeys := []string{
				"System mode", "PS state", "LTE band", "LTE bw",
				"LTE Rx chan", "LTE Tx chan", "EMM state", "RRC state",
				"RSSI (dBm)", "RSRP (dBm)", "RSRQ (dB)", "SINR (dB)",
				"Tx Power", "TAC", "Cell ID", "Current Time",
			}
			for _, key := range importantKeys {
				if shown[key] {
					continue
				}
				if v, ok := d.GStatus[key]; ok {
					sf(b, key, formatGStatusValue(key, v), labelW)
					shown[key] = true
				}
			}
			if url := cellLookupURL(d.GStatus); url != "" {
				sf(b, "Cell Lookup", url, labelW)
			}
			// Show ALL remaining GStatus keys (the point of filtering).
			var remaining []string
			for k := range d.GStatus {
				if !shown[k] {
					remaining = append(remaining, k)
				}
			}
			slices.Sort(remaining)
			for _, key := range remaining {
				sf(b, key, formatGStatusValue(key, d.GStatus[key]), labelW)
			}
			renderLTEDetail(b, info, labelW)
		})
	case "gps":
		printSectionFields(w, "GPS", labelW, func(b *strings.Builder) {
			sf(b, "Session", info.GPS.SessionStatus, labelW)
			sf(b, "Fix Status", info.GPS.FixStatus, labelW)
			sf(b, "Latitude", info.GPS.Latitude, labelW)
			sf(b, "Longitude", info.GPS.Longitude, labelW)
			sf(b, "Altitude (m)", info.GPS.Altitude, labelW)
			if info.GPS.Satellites > 0 {
				sf(b, "Satellites", fmt.Sprintf("%d", info.GPS.Satellites), labelW)
			}
		})
	case "bands":
		printSectionFields(w, "Available Bands", labelW, func(b *strings.Builder) {
			for _, band := range info.Network.AvailableBands {
				fmt.Fprintf(b, "%02d  %-24s LTE=%s\n", band.Index, band.Name, band.LTEMask)
			}
		})
	case "custom":
		printSectionFields(w, "Custom Settings", labelW, func(b *strings.Builder) {
			for k, v := range info.Custom {
				sf(b, k, v, labelW)
			}
		})
	case "ca":
		printSectionFields(w, "Carrier Aggregation", labelW, func(b *strings.Builder) {
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
				sf(b, "Pruned", info.LTECA.Pruned, labelW)
			}
		})
	}
}

// printSectionFields renders a section header and plain label-value lines
// without box-drawing borders. Used for the filtered "swtool info <section>" path.
func printSectionFields(w *os.File, title string, labelW int, render func(b *strings.Builder)) {
	var body strings.Builder
	render(&body)
	if strings.TrimSpace(body.String()) == "" {
		return
	}
	fmt.Fprintf(w, "== %s ==\n", title)
	fmt.Fprint(w, body.String())
}

// sf writes a simple field (no staleness styling) for the non-watch path.
func sf(b *strings.Builder, label, value string, labelW int) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%-*s %s\n", labelW, label+":", value)
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for _, v := range s[1:] {
		result += sep + v
	}
	return result
}

// formatGStatusValue applies display formatting for specific GStatus keys.
func formatGStatusValue(key, value string) string {
	switch key {
	case "Current Time":
		return formatUptime(value)
	case "TAC":
		return formatHexDec(value)
	case "Cell ID":
		return formatHexDec(value)
	case "LTE Rx chan":
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return modem.FormatEARFCN(n)
		}
	case "LTE Tx chan":
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return modem.FormatULEARFCN(n)
		}
	}
	return value
}

// formatUptime converts a seconds string like "42951" to "11h 55m 51s".
func formatUptime(s string) string {
	// GSTATUS "Current Time" may have trailing whitespace or extra text
	s = strings.TrimSpace(s)
	sec, err := strconv.Atoi(s)
	if err != nil || sec < 0 {
		return s
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	rem := sec % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds (%s)", h, m, rem, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds (%s)", m, rem, s)
	}
	return fmt.Sprintf("%ds", rem)
}

// formatHexDec ensures hex values show both hex and decimal.
// If already formatted like "4E00 (19968)" returns as-is.
// If just a hex string like "4E00", adds the decimal.
func formatHexDec(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "(") {
		return s // already has decimal
	}
	if n, err := strconv.ParseUint(s, 16, 64); err == nil {
		return fmt.Sprintf("%s (%d)", s, n)
	}
	return s
}

// renderLTEDetail appends neighbor cell rows to the signal panel body.
// Serving cell data is omitted — it duplicates the signal fields above
// (RSRP, RSRQ, SINR, TAC, Cell ID from QMI/GStatus).
func renderLTEDetail(b *strings.Builder, info *modem.Info, labelW int) {
	if len(info.LTEDetail.IntraFreq) > 0 {
		fmt.Fprintln(b, "Neighbor Cells (IntraFreq):")
		for _, cell := range info.LTEDetail.IntraFreq {
			renderCellRow(b, cell)
		}
	}
	if len(info.LTEDetail.InterFreq) > 0 {
		fmt.Fprintln(b, "Neighbor Cells (InterFreq):")
		for _, cell := range info.LTEDetail.InterFreq {
			renderCellRow(b, cell)
		}
	}
}

// cellLookupURL constructs a CellMapper URL from GStatus fields.
// Requires TAC and a non-zero Cell ID. Uses MCC/MNC if available in GStatus.
func cellLookupURL(gstatus map[string]string) string {
	tac := extractHexValue(gstatus["TAC"])
	cid := extractHexValue(gstatus["Cell ID"])
	if tac == "" || cid == "" || cid == "0" || cid == "00000000" {
		return ""
	}

	// Convert hex to decimal for the URL
	tacDec, err := strconv.ParseUint(tac, 16, 64)
	if err != nil {
		return ""
	}
	cidDec, err := strconv.ParseUint(cid, 16, 64)
	if err != nil || cidDec == 0 {
		return ""
	}

	// eNB ID is Cell ID >> 8 (for LTE, the top 20 bits of 28-bit CID)
	enbID := cidDec >> 8

	mcc := strings.TrimSpace(gstatus["MCC"])
	mnc := strings.TrimSpace(gstatus["MNC"])
	if mcc == "" || mnc == "" {
		return fmt.Sprintf("https://www.cellmapper.net/map?type=LTE&TAC=%d&eNB_ID=%d", tacDec, enbID)
	}

	return fmt.Sprintf("https://www.cellmapper.net/map?MCC=%s&MNC=%s&type=LTE&latitude=0&longitude=0&zoom=14&tower_id=%d",
		mcc, mnc, enbID)
}

// extractHexValue pulls the hex portion from a GStatus value.
// Handles both "4E00" and "4E00 (19968)" formats.
func extractHexValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, " "); idx > 0 {
		return s[:idx]
	}
	return s
}
