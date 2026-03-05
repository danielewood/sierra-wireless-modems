package modem

import (
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
)

// Info holds fully parsed modem information from AT commands.
type Info struct {
	Identity     Identity          `json:"identity"`
	Firmware     FirmwareInfo      `json:"firmware"`
	USB          USBInfo           `json:"usb"`
	Network      NetworkInfo       `json:"network"`
	Power        PowerInfo         `json:"power"`
	Custom       map[string]string `json:"custom"`
	Images       ImageInfo         `json:"images"`
	Diagnostics  DiagInfo          `json:"diagnostics,omitempty"`
	GPS          GPSInfo           `json:"gps"`
	SIM          SIMInfo           `json:"sim"`
	Registration RegistrationInfo  `json:"registration"`
	Operator     string            `json:"operator,omitempty"`
	APNs         []string          `json:"apns,omitempty"`
	LTEDetail    LTECellInfo       `json:"lte_detail,omitempty"`
	LTECA        LTECAInfo         `json:"lte_ca,omitempty"`
}

// DiagInfo holds live diagnostic data from AT!GSTATUS? and QMI queries.
type DiagInfo struct {
	GStatus    map[string]string `json:"gstatus,omitempty"`
	SignalBars int               `json:"signal_bars,omitempty"`
	RSSI       int               `json:"rssi_dbm,omitempty"`
	BER        int               `json:"ber,omitempty"`
}

// Identity holds modem identification from ATI.
type Identity struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Revision     string `json:"revision"`
	HardwareRev  string `json:"hardware_rev,omitempty"`
	MEID         string `json:"meid"`
	IMEI         string `json:"imei"`
	IMEISV       string `json:"imei_sv"`
	FSN          string `json:"fsn"`
	GCAP         string `json:"gcap"`
}

// FirmwareInfo holds carrier/PRI configuration from AT!IMPREF? and AT!PRIID?.
type FirmwareInfo struct {
	Preferred FirmwarePreference `json:"preferred"`
	Current   FirmwarePreference `json:"current"`
	PRIID     PRIID              `json:"pri_id"`
}

// FirmwarePreference holds a firmware version/carrier/config triple.
type FirmwarePreference struct {
	Version    string `json:"version"`
	CarrierName string `json:"carrier_name"`
	ConfigName  string `json:"config_name"`
}

// PRIID holds PRI identification from AT!PRIID?.
type PRIID struct {
	PartNumber string `json:"part_number"`
	Revision   string `json:"revision"`
	Customer   string `json:"customer"`
	CarrierPRI string `json:"carrier_pri"`
}

// USBInfo holds USB configuration from various AT!USB* commands.
type USBInfo struct {
	Composition USBCompInfo `json:"composition"`
	VID         string      `json:"vid"`
	PID         USBPIDInfo  `json:"pid"`
	Product     string      `json:"product"`
	Speed       USBSpeedInfo `json:"speed"`
}

// USBCompInfo holds parsed USB composition from AT!USBCOMP?.
type USBCompInfo struct {
	ConfigIndex int      `json:"config_index"`
	ConfigType  string   `json:"config_type"`
	Bitmask     string   `json:"bitmask"`
	Interfaces  []string `json:"interfaces"`
}

// USBPIDInfo holds app and bootloader PIDs from AT!USBPID?.
type USBPIDInfo struct {
	App  string `json:"app"`
	Boot string `json:"boot"`
}

// USBSpeedInfo holds USB speed from AT!USBSPEED?.
type USBSpeedInfo struct {
	Supported string `json:"supported"`
	Current   string `json:"current"`
}

// NetworkInfo holds RAT and band settings.
type NetworkInfo struct {
	RATSelection RATSelection `json:"rat_selection"`
	CurrentBand  BandEntry    `json:"current_band"`
	AvailableBands []BandEntry `json:"available_bands"`
}

// RATSelection holds the RAT mode from AT!SELRAT?.
type RATSelection struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// BandEntry holds a single band configuration.
type BandEntry struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	GWMask  string `json:"gw_mask"`
	LTEMask string `json:"lte_mask"`
	TDSMask string `json:"tds_mask"`
}

// PowerInfo holds power control settings from AT!PCINFO? and AT!PCOFFEN?.
type PowerInfo struct {
	State          string         `json:"state"`
	PCOFFEN        int            `json:"pcoffen"`
	LPMVoters      map[string]int `json:"lpm_voters"`
	LPMPersistence string         `json:"lpm_persistence"`
}

// ImageInfo holds firmware/PRI image slots from AT!IMAGE?.
type ImageInfo struct {
	Firmware   []ImageSlot `json:"firmware"`
	MaxFW      int         `json:"max_firmware"`
	ActiveSlot int         `json:"active_slot"`
	PRI        []ImageSlot `json:"pri"`
	MaxPRI     int         `json:"max_pri"`
}

// ImageSlot holds a single firmware or PRI image slot.
type ImageSlot struct {
	Type     string `json:"type"`
	Slot     string `json:"slot"`
	Status   string `json:"status"`
	LRU      int    `json:"lru"`
	Failures int    `json:"failures"`
	UniqueID string `json:"unique_id"`
	BuildID  string `json:"build_id"`
}

// GPSInfo holds GPS status and position from AT!GPSSTATUS?, AT!GPSLOC?,
// and AT!GPSSATINFO?.
type GPSInfo struct {
	SessionStatus string `json:"session_status"`
	FixStatus     string `json:"fix_status"`
	TTFF          string `json:"ttff,omitempty"`
	Latitude      string `json:"latitude,omitempty"`
	Longitude     string `json:"longitude,omitempty"`
	Altitude      string `json:"altitude,omitempty"`
	Satellites    int    `json:"satellites,omitempty"`
	HDOP          string `json:"hdop,omitempty"`
	PDOP          string `json:"pdop,omitempty"`
	VDOP          string `json:"vdop,omitempty"`
	Heading       string `json:"heading,omitempty"`
	Velocity      string `json:"velocity,omitempty"`
	FixType       string `json:"fix_type,omitempty"`       // "2D Fix" or "3D Fix" from GPSLOC
	HEPE          string `json:"hepe,omitempty"`            // Horizontal position error (m)
	LocTimestamp  string `json:"loc_timestamp,omitempty"`   // UTC time from GPSLOC
	// Per-satellite detail from AT!GPSSATINFO?.
	SatDetail []SatelliteInfo `json:"sat_detail,omitempty"`
}

// SatelliteInfo holds per-satellite data from AT!GPSSATINFO?.
type SatelliteInfo struct {
	System    string `json:"system"`              // GPS, GLONASS, Galileo, BeiDou
	PRN       int    `json:"prn"`                 // Satellite PRN/ID
	Elevation int    `json:"elevation,omitempty"`  // Degrees above horizon
	Azimuth   int    `json:"azimuth,omitempty"`    // Degrees from true north
	SNR       int    `json:"snr"`                  // Signal-to-noise ratio (dB-Hz)
	Used      bool   `json:"used,omitempty"`       // Used in fix
}

// SIMInfo holds SIM card information from AT+CPIN?, AT+CIMI, AT+ICCID?.
type SIMInfo struct {
	Status string `json:"status"`
	IMSI   string `json:"imsi,omitempty"`
	ICCID  string `json:"iccid,omitempty"`
}

// RegistrationInfo holds network registration from AT+CREG?, AT+CEREG?, AT+CGREG?.
type RegistrationInfo struct {
	CS   RegStatus `json:"cs"`
	EPS  RegStatus `json:"eps"`
	GPRS RegStatus `json:"gprs"`
}

// RegStatus holds a parsed registration status.
type RegStatus struct {
	Status string `json:"status"`
	Raw    string `json:"raw"`
}

// LTECellInfo holds serving and neighbor cell data from AT!LTEINFO?.
type LTECellInfo struct {
	Serving   []map[string]string `json:"serving,omitempty"`
	IntraFreq []map[string]string `json:"intra_freq,omitempty"`
	InterFreq []map[string]string `json:"inter_freq,omitempty"`
}

// LTECAInfo holds carrier aggregation band combinations from AT!LTECA?.
type LTECAInfo struct {
	Hardware  []BandCombo `json:"hardware,omitempty"`
	Permitted []BandCombo `json:"permitted,omitempty"`
	Pruned    string      `json:"pruned,omitempty"`
}

// BandCombo holds one CA band combination: a primary band and its secondary bands.
type BandCombo struct {
	Primary   string   `json:"primary"`
	Secondary []string `json:"secondary,omitempty"`
}

// GetInfo queries the modem for all configuration settings and returns
// fully parsed structured data. Read-only, safe to run anytime.
func GetInfo(port *Port) (*Info, error) {
	var result *Info
	GetInfoStreaming(port, func(info *Info) {
		result = info
	})
	return result, nil
}

// SIMStatusLPM is placed in Info.SIM.Status when AT+CPIN? is skipped because
// the modem is in low-power mode.  AT+CPIN? blocks for ~10 s in LPM before
// timing out with an error, so we skip it once we know the modem is offline.
const SIMStatusLPM = "low-power-mode"

// GetInfoStreaming queries the modem for all configuration settings,
// calling send() with a cumulative snapshot after each individual AT command
// that feeds a diagnostic check.  Commands are ordered so that power state
// (the most critical check) is known first, then fast SIM/firmware/USB checks,
// then slow band and LTE detail queries at the end.
//
// send() is called up to 8 times.  Each numbered send point corresponds to
// a specific diagnostic check becoming resolvable:
//
//	#1  power_state + pcoffen   (after AT!PCINFO? + AT!PCOFFEN?)
//	#2  sim_status              (after AT+CPIN?, or immediately if LPM)
//	#3  network_registration    (after AT+CREG?/CEREG?/CGREG?)
//	#4  firmware_preference     (after AT!IMPREF?)
//	#5  usb_identity            (after AT!USBVID? + AT!USBPID?)
//	#6  usb_composition         (after AT!USBCOMP?)
//	#7  firmware_images         (after AT!IMAGE?)
//	#8  all remaining           (band tables, signal, GPS, LTE detail)
func GetInfoStreaming(port *Port, send func(*Info)) {
	if port.platform == PlatformIntel {
		getInfoStreamingIntel(port, send)
		return
	}

	info := &Info{
		Custom: make(map[string]string),
	}
	info.Diagnostics.GStatus = make(map[string]string)

	port.SendCommand("ATE1")

	if resp, err := port.SendCommand("ATI"); err == nil {
		info.Identity = parseATI(resp)
	}

	// Enter engineering mode — best effort. Many AT! set/query commands
	// need this but standard AT+ queries work without it. Only sent once
	// per port session since the unlock persists until close.
	if !port.engineeringMode {
		if _, err := port.SendCommand(`AT!ENTERCND="A710"`); err != nil {
			port.log.Debugf("engineering mode unlock failed (AT! queries may be limited): %v", err)
		} else {
			port.engineeringMode = true
		}
	}

	// ── Send #1: Power state + PCOFFEN ────────────────────────────
	// Query power state FIRST so we know immediately whether the modem is
	// in low-power mode.  This also lets us skip AT+CPIN? when in LPM —
	// that command blocks for ~10 s before timing out.
	if resp, err := port.SendCommand("AT!PCOFFEN?"); err == nil {
		info.Power.PCOFFEN = parseIntValue(cleanATResponse(resp, "AT!PCOFFEN?"))
	}
	if resp, err := port.SendCommand("AT!PCINFO?"); err == nil {
		info.Power.State, info.Power.LPMVoters, info.Power.LPMPersistence = parsePCINFO(resp)
	}
	send(snapshotInfo(info)) // #1 — power_state + pcoffen resolvable

	// ── Send #2: SIM status ────────────────────────────────────────
	// AT+CPIN? blocks for ~10 s when the modem is in low-power mode.
	// Skip it and mark the SIM as unavailable — the modem must come online
	// before SIM queries work.
	isLPM := strings.Contains(strings.ToLower(info.Power.State), "low") ||
		strings.EqualFold(info.Power.State, "offline")
	if isLPM {
		info.SIM.Status = SIMStatusLPM
	} else {
		if resp, err := port.SendCommand("AT+CPIN?"); err == nil {
			info.SIM.Status = parseCPIN(resp)
		}
	}
	send(snapshotInfo(info)) // #2 — sim_status resolvable

	// ── Send #3: Network registration + SIM detail ─────────────────
	if resp, err := port.SendCommand("AT+CIMI"); err == nil {
		info.SIM.IMSI = parseCIMI(resp)
	}
	if resp, err := port.SendCommand("AT+ICCID?"); err == nil {
		info.SIM.ICCID = parseICCID(resp)
	}
	if resp, err := port.SendCommand("AT+COPS?"); err == nil {
		info.Operator = parseCOPS(resp)
	}
	if resp, err := port.SendCommand("AT+CREG?"); err == nil {
		info.Registration.CS = parseCREG(resp)
	}
	if resp, err := port.SendCommand("AT+CEREG?"); err == nil {
		info.Registration.EPS = parseCEREG(resp)
	}
	if resp, err := port.SendCommand("AT+CGREG?"); err == nil {
		info.Registration.GPRS = parseCGREG(resp)
	}
	send(snapshotInfo(info)) // #3 — network_registration resolvable

	// ── Send #4: Firmware preference ───────────────────────────────
	if resp, err := port.SendCommand("AT!IMPREF?"); err == nil {
		info.Firmware.Preferred, info.Firmware.Current = parseIMPREF(resp)
	}
	send(snapshotInfo(info)) // #4 — firmware_preference resolvable

	// ── Send #5: USB identity ──────────────────────────────────────
	if resp, err := port.SendCommand("AT!PRIID?"); err == nil {
		info.Firmware.PRIID = parsePRIID(resp)
	}
	if resp, err := port.SendCommand("AT!USBVID?"); err == nil {
		info.USB.VID = parseSingleLineValue(resp)
	}
	if resp, err := port.SendCommand("AT!USBPID?"); err == nil {
		info.USB.PID = parseUSBPID(resp)
	}
	send(snapshotInfo(info)) // #5 — usb_identity resolvable

	// ── Send #6: USB composition ───────────────────────────────────
	if resp, err := port.SendCommand("AT!USBCOMP?"); err == nil {
		info.USB.Composition = parseUSBCOMP(resp)
	}
	if resp, err := port.SendCommand("AT!USBPRODUCT?"); err == nil {
		info.USB.Product = cleanATResponse(resp, "AT!USBPRODUCT?")
	}
	if resp, err := port.SendCommand("AT!USBSPEED?"); err == nil {
		info.USB.Speed = parseUSBSpeed(resp)
	}
	send(snapshotInfo(info)) // #6 — usb_composition resolvable

	// ── Send #7: Firmware images ───────────────────────────────────
	if resp, err := port.SendCommand("AT!IMAGE?"); err == nil {
		info.Images = parseIMAGE(resp)
	}
	send(snapshotInfo(info)) // #7 — firmware_images resolvable

	// ── Send #8: Slow / optional queries ──────────────────────────
	// None of these feed diagnostic checks, so they run last.
	if resp, err := port.SendCommand("AT!SELRAT?"); err == nil {
		info.Network.RATSelection = parseSELRAT(resp)
	}
	if resp, err := port.SendCommand("AT!BAND?"); err == nil {
		bands := parseBandTable(resp)
		if len(bands) > 0 {
			info.Network.CurrentBand = bands[0]
		}
	}
	if resp, err := port.SendCommand("AT!BAND=?"); err == nil {
		info.Network.AvailableBands = parseBandTable(resp)
	}
	if resp, err := port.SendCommand("AT!CUSTOM?"); err == nil {
		info.Custom = parseCUSTOM(resp)
	}
	if resp, err := port.SendCommand("AT!GSTATUS?"); err == nil {
		info.Diagnostics.GStatus = parseGSTATUS(resp)
	}
	if resp, err := port.SendCommand("AT+CSQ"); err == nil {
		info.Diagnostics.RSSI, info.Diagnostics.BER = parseCSQ(resp)
		info.Diagnostics.SignalBars = rssiToBars(info.Diagnostics.RSSI)
	}
	if resp, err := port.SendCommand("AT!GPSSTATUS?"); err == nil {
		info.GPS = parseGPSSTATUS(resp)
	}
	if resp, err := port.SendCommand("AT!GPSSATINFO?"); err == nil {
		info.GPS.Satellites, info.GPS.SatDetail = parseGPSSATINFO(resp)
	}
	// AT!GPSLOC? only returns data when a fix is available.
	if info.GPS.FixStatus != "" && !strings.EqualFold(info.GPS.FixStatus, "NO FIX") &&
		!strings.EqualFold(info.GPS.FixStatus, "NONE") {
		if resp, err := port.SendCommand("AT!GPSLOC?"); err == nil {
			parseGPSLOC(resp, &info.GPS)
		}
	}
	if resp, err := port.SendCommand("AT!HWID?"); err == nil {
		info.Identity.HardwareRev = parseHWID(resp)
	}
	if resp, err := port.SendCommand("AT+CGDCONT?"); err == nil {
		info.APNs = parseCGDCONT(resp)
	}
	if resp, err := port.SendCommand("AT!LTEINFO?"); err == nil {
		info.LTEDetail = parseLTEINFO(resp)
	}
	if resp, err := port.SendCommand("AT!LTECA?"); err == nil {
		info.LTECA = parseLTECA(resp)
	}
	send(snapshotInfo(info)) // #8
}

// GetInfoForSection queries only the AT commands needed for a single info
// section. Much faster than GetInfoStreaming for targeted queries like
// "swtool info signal". Returns a partial Info with only the relevant
// fields populated.
func GetInfoForSection(port *Port, section string) *Info {
	if port.platform == PlatformIntel {
		return getInfoForSectionIntel(port, section)
	}

	info := &Info{
		Custom: make(map[string]string),
	}
	info.Diagnostics.GStatus = make(map[string]string)

	port.SendCommand("ATE1")

	// Most AT! commands need engineering mode.
	if !port.engineeringMode {
		if _, err := port.SendCommand(`AT!ENTERCND="A710"`); err == nil {
			port.engineeringMode = true
		}
	}

	switch section {
	case "identity":
		if resp, err := port.SendCommand("ATI"); err == nil {
			info.Identity = parseATI(resp)
		}
		if resp, err := port.SendCommand("AT!HWID?"); err == nil {
			info.Identity.HardwareRev = parseHWID(resp)
		}
	case "power":
		if resp, err := port.SendCommand("AT!PCOFFEN?"); err == nil {
			info.Power.PCOFFEN = parseIntValue(cleanATResponse(resp, "AT!PCOFFEN?"))
		}
		if resp, err := port.SendCommand("AT!PCINFO?"); err == nil {
			info.Power.State, info.Power.LPMVoters, info.Power.LPMPersistence = parsePCINFO(resp)
		}
	case "sim":
		if resp, err := port.SendCommand("AT+CPIN?"); err == nil {
			info.SIM.Status = parseCPIN(resp)
		}
		if resp, err := port.SendCommand("AT+CIMI"); err == nil {
			info.SIM.IMSI = parseCIMI(resp)
		}
		if resp, err := port.SendCommand("AT+ICCID?"); err == nil {
			info.SIM.ICCID = parseICCID(resp)
		}
	case "network":
		// AT+COPS?, AT+CREG?, AT+CEREG?, AT+CGREG? removed — QMI provides
		// operator, registration status, and system mode.
		if resp, err := port.SendCommand("AT!SELRAT?"); err == nil {
			info.Network.RATSelection = parseSELRAT(resp)
		}
		if resp, err := port.SendCommand("AT!BAND?"); err == nil {
			bands := parseBandTable(resp)
			if len(bands) > 0 {
				info.Network.CurrentBand = bands[0]
			}
		}
		if resp, err := port.SendCommand("AT+CGDCONT?"); err == nil {
			info.APNs = parseCGDCONT(resp)
		}
	case "firmware":
		if resp, err := port.SendCommand("AT!IMPREF?"); err == nil {
			info.Firmware.Preferred, info.Firmware.Current = parseIMPREF(resp)
		}
	case "priid":
		if resp, err := port.SendCommand("AT!PRIID?"); err == nil {
			info.Firmware.PRIID = parsePRIID(resp)
		}
	case "usb":
		if resp, err := port.SendCommand("AT!USBVID?"); err == nil {
			info.USB.VID = parseSingleLineValue(resp)
		}
		if resp, err := port.SendCommand("AT!USBPID?"); err == nil {
			info.USB.PID = parseUSBPID(resp)
		}
		if resp, err := port.SendCommand("AT!USBCOMP?"); err == nil {
			info.USB.Composition = parseUSBCOMP(resp)
		}
		if resp, err := port.SendCommand("AT!USBPRODUCT?"); err == nil {
			info.USB.Product = cleanATResponse(resp, "AT!USBPRODUCT?")
		}
		if resp, err := port.SendCommand("AT!USBSPEED?"); err == nil {
			info.USB.Speed = parseUSBSpeed(resp)
		}
	case "images":
		if resp, err := port.SendCommand("AT!IMAGE?"); err == nil {
			info.Images = parseIMAGE(resp)
		}
	case "signal":
		// AT+CSQ and AT!LTEINFO? removed — QMI provides signal and cell data.
		// AT!GSTATUS? stays: Sierra-specific fields (band, EMM/RRC state, Tx Power, etc.).
		if resp, err := port.SendCommand("AT!GSTATUS?"); err == nil {
			info.Diagnostics.GStatus = parseGSTATUS(resp)
		}
	case "gps":
		if resp, err := port.SendCommand("AT!GPSSTATUS?"); err == nil {
			info.GPS = parseGPSSTATUS(resp)
		}
		if resp, err := port.SendCommand("AT!GPSSATINFO?"); err == nil {
			info.GPS.Satellites, info.GPS.SatDetail = parseGPSSATINFO(resp)
		}
		if info.GPS.FixStatus != "" && !strings.EqualFold(info.GPS.FixStatus, "NO FIX") &&
			!strings.EqualFold(info.GPS.FixStatus, "NONE") {
			if resp, err := port.SendCommand("AT!GPSLOC?"); err == nil {
				parseGPSLOC(resp, &info.GPS)
			}
		}
	case "bands":
		if resp, err := port.SendCommand("AT!BAND=?"); err == nil {
			info.Network.AvailableBands = parseBandTable(resp)
		}
	case "custom":
		if resp, err := port.SendCommand("AT!CUSTOM?"); err == nil {
			info.Custom = parseCUSTOM(resp)
		}
	case "ca":
		if resp, err := port.SendCommand("AT!LTECA?"); err == nil {
			info.LTECA = parseLTECA(resp)
		}
	}

	return info
}

// GetConfigSummary queries only the settings that flash/configure would change.
// Much faster than GetInfo (~8 AT commands vs 30+). Returns partial Info with
// only Firmware, USB, and Network fields populated.
func GetConfigSummary(port *Port) *Info {
	info := &Info{
		Custom: make(map[string]string),
	}

	port.SendCommand("ATE1")

	if !port.engineeringMode {
		if _, err := port.SendCommand(`AT!ENTERCND="A710"`); err == nil {
			port.engineeringMode = true
		}
	}

	if resp, err := port.SendCommand("AT!IMPREF?"); err == nil {
		info.Firmware.Preferred, info.Firmware.Current = parseIMPREF(resp)
	}
	if resp, err := port.SendCommand("AT!USBCOMP?"); err == nil {
		info.USB.Composition = parseUSBCOMP(resp)
	}
	if resp, err := port.SendCommand("AT!USBVID?"); err == nil {
		info.USB.VID = parseSingleLineValue(resp)
	}
	if resp, err := port.SendCommand("AT!USBPID?"); err == nil {
		info.USB.PID = parseUSBPID(resp)
	}
	if resp, err := port.SendCommand("AT!USBPRODUCT?"); err == nil {
		info.USB.Product = cleanATResponse(resp, "AT!USBPRODUCT?")
	}
	if resp, err := port.SendCommand("AT!SELRAT?"); err == nil {
		info.Network.RATSelection = parseSELRAT(resp)
	}
	if resp, err := port.SendCommand("AT!BAND?"); err == nil {
		bands := parseBandTable(resp)
		if len(bands) > 0 {
			info.Network.CurrentBand = bands[0]
		}
	}
	if resp, err := port.SendCommand("AT!USBSPEED?"); err == nil {
		info.USB.Speed = parseUSBSpeed(resp)
	}
	if resp, err := port.SendCommand("AT!PCOFFEN?"); err == nil {
		info.Power.PCOFFEN = parseIntValue(cleanATResponse(resp, "AT!PCOFFEN?"))
	}
	if resp, err := port.SendCommand("AT!CUSTOM?"); err == nil {
		info.Custom = parseCUSTOM(resp)
	}

	return info
}

// EnterCommandMode sends AT!ENTERCND="A710" to unlock engineering commands.
func EnterCommandMode(port *Port) error {
	_, err := port.SendCommand(`AT!ENTERCND="A710"`)
	if err != nil {
		return fmt.Errorf("entering command mode: %w", err)
	}
	return nil
}

// ClearFirmwareImages sends AT!IMAGE=0 to clear stored firmware images.
func ClearFirmwareImages(port *Port) error {
	if err := EnterCommandMode(port); err != nil {
		return err
	}
	_, err := port.SendCommand("AT!IMAGE=0")
	if err != nil {
		return fmt.Errorf("clearing firmware images: %w", err)
	}
	return nil
}

// ConfigureSettings holds all parameters for modem configuration.
// Only non-empty/flagged fields are applied.
type ConfigureSettings struct {
	USBComp       string // "1,1,0000100D"
	USBVID        string // "1199"
	USBPID        string // "9071,9070"
	USBProduct    string // "EM7455"
	PRIIDPartNum  string
	PRIIDRev      string
	PRIIDCustomer string // "Generic-Laptop"
	SelRat        string // "06" or "00"
	Band          string // "09" or "00"
	FastEnumEN    int    // 0-3
	SetFastEnum   bool   // true if FastEnumEN was explicitly set
	USBSpeed      int    // 0 or 1
	SetUSBSpeed   bool   // true if USBSpeed was explicitly set
}

// ATCommands returns the AT command strings for all populated settings.
// Does not include AT!ENTERCND, carrier preference, or AT!RESET.
func (c ConfigureSettings) ATCommands() []string {
	var cmds []string
	if c.USBComp != "" {
		cmds = append(cmds, fmt.Sprintf("AT!USBCOMP=%s", c.USBComp))
	}
	if c.USBVID != "" {
		cmds = append(cmds, fmt.Sprintf("AT!USBVID=%s", c.USBVID))
	}
	if c.USBPID != "" {
		cmds = append(cmds, fmt.Sprintf("AT!USBPID=%s", c.USBPID))
	}
	if c.USBProduct != "" {
		cmds = append(cmds, fmt.Sprintf(`AT!USBPRODUCT="%s"`, c.USBProduct))
	}
	if c.PRIIDPartNum != "" && c.PRIIDRev != "" {
		cmds = append(cmds, fmt.Sprintf(`AT!PRIID="%s","%s","%s"`, c.PRIIDPartNum, c.PRIIDRev, c.PRIIDCustomer))
	}
	if c.SelRat != "" {
		cmds = append(cmds, fmt.Sprintf("AT!SELRAT=%s", c.SelRat))
	}
	if c.Band != "" {
		cmds = append(cmds, fmt.Sprintf("AT!BAND=%s", c.Band))
	}
	if c.SetFastEnum {
		cmds = append(cmds, fmt.Sprintf(`AT!CUSTOM="FASTENUMEN",%d`, c.FastEnumEN))
	}
	if c.SetUSBSpeed {
		cmds = append(cmds, fmt.Sprintf("AT!USBSPEED=%d", c.USBSpeed))
	}
	return cmds
}

// ApplySettings sends AT commands for the populated fields in cfg,
// then resets the modem. Only non-empty/flagged fields are applied.
func ApplySettings(port *Port, cfg ConfigureSettings) error {
	if err := EnterCommandMode(port); err != nil {
		return err
	}

	for _, cmd := range cfg.ATCommands() {
		if _, err := port.SendCommand(cmd); err != nil {
			return fmt.Errorf("setting %q: %w", cmd, err)
		}
	}

	return nil
}

// ResetModem sends AT!RESET to reboot the modem.
func ResetModem(port *Port) error {
	_, _ = port.SendCommand("AT!RESET")
	return nil
}

// ---------------------------------------------------------------------------
// Parsers
// ---------------------------------------------------------------------------

// parseATI parses the ATI response into Identity fields.
//
// Input:
//
//	Manufacturer: Sierra Wireless, Incorporated
//	Model: EM7455B
//	Revision: SWI9X30C_02.24.05.06 r7040 ...
//	MEID: 35448008243466
//	IMEI: 354480082434669
//	IMEI SV: 12
//	FSN: LF103291240310
//	+GCAP: +CGSM
func parseATI(resp string) Identity {
	id := Identity{}
	for _, line := range splitLines(resp) {
		k, v := splitKV(line, ":")
		switch k {
		case "Manufacturer":
			id.Manufacturer = v
		case "Model":
			id.Model = v
		case "Revision":
			id.Revision = v
		case "MEID":
			id.MEID = v
		case "IMEI":
			id.IMEI = v
		case "IMEI SV":
			id.IMEISV = v
		case "FSN":
			id.FSN = v
		case "+GCAP":
			id.GCAP = v
		}
	}
	return id
}

// parseIMPREF parses AT!IMPREF? response into preferred/current firmware info.
//
// Input:
//
//	!IMPREF:
//	 preferred fw version:    02.24.05.06
//	 preferred carrier name:  GENERIC
//	 preferred config name:   GENERIC_002.026_000
//	 current fw version:      02.24.05.06
//	 current carrier name:    GENERIC
//	 current config name:     GENERIC_002.026_000
func parseIMPREF(resp string) (pref, cur FirmwarePreference) {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		k, v := splitKV(line, ":")
		k = strings.TrimSpace(k)
		switch k {
		case "preferred fw version":
			pref.Version = v
		case "preferred carrier name":
			pref.CarrierName = v
		case "preferred config name":
			pref.ConfigName = v
		case "current fw version":
			cur.Version = v
		case "current carrier name":
			cur.CarrierName = v
		case "current config name":
			cur.ConfigName = v
		}
	}
	return
}

// parseUSBCOMP parses AT!USBCOMP? response.
//
// Input:
//
//	Config Index: 1
//	Config Type:  1 (Generic)
//	Interface bitmask: 0020100D (diag,nmea,modem,mbim,ubist)
func parseUSBCOMP(resp string) USBCompInfo {
	info := USBCompInfo{}
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if k, v := splitKV(line, ":"); k != "" {
			switch k {
			case "Config Index":
				info.ConfigIndex = parseIntValue(v)
			case "Config Type":
				info.ConfigType = v
			case "Interface bitmask":
				// "0020100D (diag,nmea,modem,mbim,ubist)"
				parts := strings.SplitN(v, "(", 2)
				info.Bitmask = strings.TrimSpace(parts[0])
				if len(parts) == 2 {
					ifList := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")
					for _, iface := range strings.Split(ifList, ",") {
						iface = strings.TrimSpace(iface)
						if iface != "" {
							info.Interfaces = append(info.Interfaces, iface)
						}
					}
				}
			}
		}
	}
	return info
}

// parseUSBPID parses AT!USBPID? response.
//
// Input:
//
//	!USBPID:
//	APP : 81B6
//	BOOT: 81B5
func parseUSBPID(resp string) USBPIDInfo {
	info := USBPIDInfo{}
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if k, v := splitKV(line, ":"); k != "" {
			switch strings.TrimSpace(k) {
			case "APP":
				info.App = v
			case "BOOT":
				info.Boot = v
			}
		}
	}
	return info
}

// parseUSBSpeed parses AT!USBSPEED? response.
//
// Input:
//
//	SUPPORTED:Super-Speed
//	CURRENT  :High-Speed
func parseUSBSpeed(resp string) USBSpeedInfo {
	info := USBSpeedInfo{}
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if k, v := splitKV(line, ":"); k != "" {
			switch strings.TrimSpace(k) {
			case "SUPPORTED":
				info.Supported = v
			case "CURRENT":
				info.Current = v
			}
		}
	}
	return info
}

// parsePRIID parses AT!PRIID? response.
//
// Input:
//
//	PRI Part Number: 9907375
//	Revision: 001.001
//	Customer: PebbleCreekMLK
//
//	Carrier PRI: 9999999_9904609_SWI9X30C_02.24.05.06_00_GENERIC_002.026_000
func parsePRIID(resp string) PRIID {
	p := PRIID{}
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if k, v := splitKV(line, ":"); k != "" {
			switch strings.TrimSpace(k) {
			case "PRI Part Number":
				p.PartNumber = v
			case "Revision":
				p.Revision = v
			case "Customer":
				p.Customer = v
			case "Carrier PRI":
				p.CarrierPRI = v
			}
		}
	}
	return p
}

// parseSELRAT parses AT!SELRAT? response.
//
// Input: "!SELRAT: 06, LTE Only"
func parseSELRAT(resp string) RATSelection {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") {
			continue
		}
		line = strings.TrimPrefix(line, "!SELRAT:")
		line = strings.TrimSpace(line)
		// "06, LTE Only"
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			return RATSelection{
				Index: parseIntValue(strings.TrimSpace(parts[0])),
				Name:  strings.TrimSpace(parts[1]),
			}
		}
	}
	return RATSelection{}
}

// parseBandTable parses the tabular output from AT!BAND? or AT!BAND=?.
//
// Input:
//
//	Index, Name,                        GW Band Mask     L Band Mask      TDS Band Mask
//	00, All bands,                      0002000007C00000 00000100130818DF 0000000000000000
//	09, LTE ALL                         0000000000000000 00000100130818DF 0000000000000000
func parseBandTable(resp string) []BandEntry {
	var entries []BandEntry
	// Match lines like: "00, All bands,     <hex> <hex> <hex>"
	// or:               "09, LTE ALL        <hex> <hex> <hex>"
	re := regexp.MustCompile(`^\s*(\d{2}),\s+(.+?)\s{2,}([0-9A-Fa-f]{16})\s+([0-9A-Fa-f]{16})\s+([0-9A-Fa-f]{16})`)

	for _, line := range splitLines(resp) {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		idx := parseIntValue(m[1])
		name := strings.TrimRight(strings.TrimSpace(m[2]), ",")
		entries = append(entries, BandEntry{
			Index:   idx,
			Name:    name,
			GWMask:  m[3],
			LTEMask: m[4],
			TDSMask: m[5],
		})
	}
	return entries
}

// parsePCINFO parses AT!PCINFO? response.
//
// Input:
//
//	State: Online
//	LPM voters - Temp:0, Volt:0, User:0, W_DISABLE:0, IMSWITCH:0, BIOS:0, LWM2M:0, OMADM:0, FOTA:0
//	LPM persistence - None
func parsePCINFO(resp string) (state string, voters map[string]int, persistence string) {
	voters = make(map[string]int)
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if k, v := splitKV(line, ":"); strings.TrimSpace(k) == "State" {
			state = v
			continue
		}
		if strings.HasPrefix(line, "LPM voters") {
			// "LPM voters - Temp:0, Volt:0, User:0, ..."
			after := strings.SplitN(line, "-", 2)
			if len(after) == 2 {
				pairs := strings.Split(strings.TrimSpace(after[1]), ",")
				for _, pair := range pairs {
					k, v := splitKV(strings.TrimSpace(pair), ":")
					if k != "" {
						voters[k] = parseIntValue(v)
					}
				}
			}
			continue
		}
		if strings.HasPrefix(line, "LPM persistence") {
			after := strings.SplitN(line, "-", 2)
			if len(after) == 2 {
				persistence = strings.TrimSpace(after[1])
			}
		}
	}
	return
}

// parseCUSTOM parses AT!CUSTOM? response into key-value pairs.
//
// Input:
//
//	!CUSTOM:
//	             GPSENABLE		0x04
//	             GPIOSARENABLE	0x01
//	             IPV6ENABLE		0x01
func parseCUSTOM(resp string) map[string]string {
	m := make(map[string]string)
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "!CUSTOM") || strings.HasPrefix(line, "AT!") {
			continue
		}
		// Split on whitespace: "GPSENABLE  0x04"
		fields := strings.Fields(line)
		if len(fields) == 2 {
			m[fields[0]] = fields[1]
		}
	}
	return m
}

// parseIMAGE parses AT!IMAGE? response.
//
// Input:
//
//	TYPE SLOT STATUS LRU FAILURES UNIQUE_ID   BUILD_ID
//	FW   1    GOOD   1   0 0      ?_?         02.24.05.06_?
//	FW   2    EMPTY  0   0 0
//	Max FW images: 4
//	Active FW image is at slot 1
//
//	TYPE SLOT STATUS LRU FAILURES UNIQUE_ID   BUILD_ID
//	PRI  FF   GOOD   0   0 0      002.026_000 02.24.05.06_GENERIC
//	Max PRI images: 50
func parseIMAGE(resp string) ImageInfo {
	info := ImageInfo{}
	// Match image slot lines: "FW   1    GOOD   1   0 0      ?_?         02.24.05.06_?"
	// or:                     "PRI  FF   GOOD   0   0 0      002.026_000 02.24.05.06_GENERIC"
	re := regexp.MustCompile(`^\s*(FW|PRI)\s+(\S+)\s+(GOOD|EMPTY|BAD)\s+(\d+)\s+(\d+)\s+(\d+)\s*(.*)$`)

	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)

		if m := re.FindStringSubmatch(line); m != nil {
			slot := ImageSlot{
				Type:     m[1],
				Slot:     m[2],
				Status:   m[3],
				LRU:      parseIntValue(m[4]),
				Failures: parseIntValue(m[5]),
				// m[6] is the second failure field (seems to always match m[5])
			}
			// Remaining fields: unique_id and build_id
			rest := strings.TrimSpace(m[7])
			if rest != "" {
				fields := strings.Fields(rest)
				if len(fields) >= 1 {
					slot.UniqueID = fields[0]
				}
				if len(fields) >= 2 {
					slot.BuildID = fields[1]
				}
			}

			if m[1] == "FW" {
				info.Firmware = append(info.Firmware, slot)
			} else {
				info.PRI = append(info.PRI, slot)
			}
			continue
		}

		if strings.HasPrefix(line, "Max FW images:") {
			info.MaxFW = parseIntValue(strings.TrimPrefix(line, "Max FW images:"))
		}
		if strings.HasPrefix(line, "Max PRI images:") {
			info.MaxPRI = parseIntValue(strings.TrimPrefix(line, "Max PRI images:"))
		}
		if strings.HasPrefix(line, "Active FW image is at slot") {
			info.ActiveSlot = parseIntValue(strings.TrimPrefix(line, "Active FW image is at slot"))
		}
	}
	return info
}

// parseGSTATUS parses AT!GSTATUS? response into key-value pairs.
//
// Handles two formats:
//   - Non-CA: single RSSI/RSRP/RSRQ/SINR lines
//   - CA active: per-antenna-path lines like "PCC RxM RSSI: -45  RSRP (dBm): -81"
//     where duplicate keys (RSRP, RSSI) are prefixed with their path (e.g. "PCC RxM RSRP (dBm)")
//
// Also handles continuation text without colons (e.g. "EMM state: Registered\tNormal Service")
// by appending to the previous key's value.
func parseGSTATUS(resp string) map[string]string {
	m := make(map[string]string)
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "!GSTATUS") || strings.HasPrefix(line, "AT!") {
			continue
		}
		if line == "OK" || line == "ERROR" {
			continue
		}

		pairs := splitGStatusPairs(line)
		for _, pair := range pairs {
			k, v := splitKV(pair.text, ":")
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k == "" {
				continue
			}
			// Append continuation text from non-KV segments
			if pair.continuation != "" {
				v = v + " " + pair.continuation
			}
			// Handle per-antenna-path lines by prefixing duplicate keys.
			// "PCC RxM RSSI" stays as-is (already unique), but the second
			// KV pair on the same line like "RSRP (dBm)" needs the path prefix.
			if pair.pathPrefix != "" {
				if _, exists := m[k]; exists {
					k = pair.pathPrefix + " " + k
				}
			}
			m[k] = v
		}
	}
	return m
}

// gstatusPair holds a parsed KV segment from a GSTATUS line.
type gstatusPair struct {
	text         string // the "key: value" text
	continuation string // any non-KV text that follows (e.g. "Normal Service")
	pathPrefix   string // antenna path prefix for deduplication (e.g. "PCC RxM")
}

// splitGStatusPairs splits a GSTATUS line into individual key:value segments.
// Handles tab-separated pairs, continuation text without colons, and
// per-antenna-path prefixes for CA lines.
func splitGStatusPairs(line string) []gstatusPair {
	parts := strings.Split(line, "\t")

	// Detect antenna path prefix (e.g. "PCC RxM", "SCC RxD") for CA lines
	var pathPrefix string
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"PCC RxM", "PCC RxD", "SCC RxM", "SCC RxD"} {
		if strings.HasPrefix(trimmed, prefix) {
			pathPrefix = prefix
			break
		}
	}

	var result []gstatusPair
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, ":") {
			result = append(result, gstatusPair{
				text:       p,
				pathPrefix: pathPrefix,
			})
		} else if len(result) > 0 {
			// Continuation text (no colon) — append to previous pair
			result[len(result)-1].continuation = p
		}
	}

	if len(result) > 0 {
		return result
	}
	// No tab-delimited pairs found; treat entire line as one pair
	return []gstatusPair{{text: line}}
}

// parseCSQ parses AT+CSQ response.
//
// Input: "+CSQ: 20,99"
// RSSI: 0-31 (mapped to dBm), 99=unknown
// BER: 0-7, 99=unknown
func parseCSQ(resp string) (rssiDBM, ber int) {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+CSQ:") {
			continue
		}
		val := strings.TrimPrefix(line, "+CSQ:")
		val = strings.TrimSpace(val)
		parts := strings.SplitN(val, ",", 2)
		if len(parts) >= 1 {
			rssi := parseIntValue(strings.TrimSpace(parts[0]))
			if rssi >= 0 && rssi <= 31 {
				rssiDBM = -113 + (rssi * 2) // 3GPP mapping
			}
		}
		if len(parts) >= 2 {
			ber = parseIntValue(strings.TrimSpace(parts[1]))
		}
		return
	}
	return
}

// rssiToBars converts RSSI dBm to a 0-5 signal bar count.
func rssiToBars(rssiDBM int) int {
	if rssiDBM == 0 {
		return 0
	}
	switch {
	case rssiDBM >= -70:
		return 5
	case rssiDBM >= -85:
		return 4
	case rssiDBM >= -100:
		return 3
	case rssiDBM >= -110:
		return 2
	case rssiDBM >= -120:
		return 1
	default:
		return 0
	}
}

// ---------------------------------------------------------------------------
// GPS, SIM, Registration, Operator, APN, LTE detail parsers
// ---------------------------------------------------------------------------

// parseGPSSTATUS parses AT!GPSSTATUS? response.
//
// Input:
//
//	Fix Session Status = ACTIVE
//	TTFF (sec) = 26
//	Fix Status  = 3D
//	PDOP = 1.5  HDOP = 0.8  VDOP = 1.2
//	Latitude:  N  38 53 42.123
//	Longitude: W 077 02 11.456
//	Altitude (m) = 72.3
//	Heading (deg) =   0.0  Velocity (m/s) =   0.0
//	Satellite count = 12
func parseGPSSTATUS(resp string) GPSInfo {
	g := GPSInfo{}
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") {
			continue
		}

		// Lines may be prefixed with a timestamp, e.g.:
		//   1980 01 06 6 00:14:05 Fix Session Status = NONE
		// Use Contains instead of HasPrefix to match regardless.
		// Check "Fix Session Status" before "Fix Status" (substring).
		switch {
		case strings.Contains(line, "Fix Session Status"):
			_, g.SessionStatus = splitKV(line, "=")
		case strings.Contains(line, "Fix Status"):
			_, g.FixStatus = splitKV(line, "=")
		case strings.Contains(line, "TTFF"):
			_, g.TTFF = splitKV(line, "=")
		case strings.Contains(line, "Latitude"):
			_, g.Latitude = splitKV(line, ":")
		case strings.Contains(line, "Longitude"):
			_, g.Longitude = splitKV(line, ":")
		case strings.Contains(line, "Altitude"):
			_, g.Altitude = splitKV(line, "=")
		case strings.Contains(line, "Satellite count"):
			_, v := splitKV(line, "=")
			g.Satellites = parseIntValue(v)
		case strings.Contains(line, "Heading"):
			// "Heading (deg) =   0.0  Velocity (m/s) =   0.0"
			parts := strings.Split(line, "Velocity")
			if len(parts) >= 1 {
				_, g.Heading = splitKV(parts[0], "=")
			}
			if len(parts) >= 2 {
				_, g.Velocity = splitKV(parts[1], "=")
			}
		case strings.Contains(line, "PDOP"):
			// "PDOP = 1.5  HDOP = 0.8  VDOP = 1.2"
			re := regexp.MustCompile(`(\w+)\s*=\s*([0-9.]+)`)
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				switch m[1] {
				case "PDOP":
					g.PDOP = m[2]
				case "HDOP":
					g.HDOP = m[2]
				case "VDOP":
					g.VDOP = m[2]
				}
			}
		}
	}
	return g
}

// parseGPSSATINFO parses AT!GPSSATINFO? response.
// Returns satellite count and per-satellite details.
//
// Input:
//
//	Satellites in view:  18 (2026 03 01 6 18:54:38)
//	* SV:  3  ELEV: 18  AZI:   63  SNR: 25
//	* SV:  6  ELEV: 70  AZI:  234  SNR: 27
//	* SV:313  ELEV: 53  AZI:  209  SNR: 33
func parseGPSSATINFO(resp string) (int, []SatelliteInfo) {
	var sats []SatelliteInfo
	count := 0
	re := regexp.MustCompile(`SV:\s*(\d+)\s+ELEV:\s*(\d+)\s+AZI:\s*(\d+)\s+SNR:\s*(\d+)`)
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") {
			continue
		}
		if strings.Contains(line, "Satellites in view") {
			reCount := regexp.MustCompile(`Satellites in view:\s*(\d+)`)
			if m := reCount.FindStringSubmatch(line); len(m) > 1 {
				count = parseIntValue(m[1])
			}
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) > 4 {
			prn := parseIntValue(m[1])
			sats = append(sats, SatelliteInfo{
				System:    gnssSystem(prn),
				PRN:       prn,
				Elevation: parseIntValue(m[2]),
				Azimuth:   parseIntValue(m[3]),
				SNR:       parseIntValue(m[4]),
			})
		}
	}
	return count, sats
}

// gnssSystem returns the GNSS constellation name from a satellite PRN number.
// Sierra Wireless modems use standard NMEA-style PRN ranges.
func gnssSystem(prn int) string {
	switch {
	case prn >= 1 && prn <= 32:
		return "GPS"
	case prn >= 33 && prn <= 64:
		return "SBAS"
	case prn >= 65 && prn <= 96:
		return "GLONASS"
	case prn >= 120 && prn <= 158:
		return "SBAS"
	case prn >= 193 && prn <= 200:
		return "QZSS"
	case prn >= 201 && prn <= 263:
		return "BeiDou"
	case prn >= 301 && prn <= 336:
		return "Galileo"
	case prn >= 401 && prn <= 414:
		return "NavIC"
	default:
		return fmt.Sprintf("SV%d", prn)
	}
}

// parseGPSLOC parses AT!GPSLOC? response and fills position fields into a GPSInfo.
// This supplements AT!GPSSTATUS? with richer position data when a fix is available.
//
// Input:
//
//	Lat: 21 Deg 8 Min 55.04 Sec N  (0x003C27F5)
//	Lon: 86 Deg 49 Min 46.79 Sec W  (0xFF090491)
//	Time: 2026 03 01 6 18:56:23 (GPS)
//	LocUncAngle: 0.0 deg  LocUncA: 2 m  LocUncP: 2 m  HEPE: 2.828 m
//	3D Fix
//	Altitude: 6 m  LocUncVe: 3.0 m
//	Heading: 0.0 deg  VelHoriz: 0.0 m/s  VelVert: 0.0 m/s
func parseGPSLOC(resp string, gps *GPSInfo) {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Lat:"):
			v := strings.TrimPrefix(line, "Lat: ")
			if idx := strings.Index(v, "(0x"); idx > 0 {
				v = strings.TrimSpace(v[:idx])
			}
			gps.Latitude = v
		case strings.HasPrefix(line, "Lon:"):
			v := strings.TrimPrefix(line, "Lon: ")
			if idx := strings.Index(v, "(0x"); idx > 0 {
				v = strings.TrimSpace(v[:idx])
			}
			gps.Longitude = v
		case strings.HasPrefix(line, "Time:"):
			gps.LocTimestamp = strings.TrimSpace(strings.TrimPrefix(line, "Time:"))
		case strings.HasPrefix(line, "Altitude:"):
			re := regexp.MustCompile(`Altitude:\s*([0-9.-]+)\s*m`)
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				gps.Altitude = m[1]
			}
		case strings.Contains(line, "HEPE"):
			re := regexp.MustCompile(`HEPE:\s*([0-9.]+)\s*m`)
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				gps.HEPE = m[1]
			}
		case strings.Contains(line, "Heading"):
			re := regexp.MustCompile(`Heading:\s*([0-9.]+)`)
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				gps.Heading = m[1]
			}
			re2 := regexp.MustCompile(`VelHoriz:\s*([0-9.]+)`)
			if m := re2.FindStringSubmatch(line); len(m) > 1 {
				gps.Velocity = m[1]
			}
		case strings.HasSuffix(line, "Fix"):
			// "3D Fix" or "2D Fix"
			gps.FixType = line
		}
	}
}

// parseCPIN parses AT+CPIN? response.
//
// Input: "+CPIN: READY" or "+CPIN: SIM PIN"
func parseCPIN(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+CPIN:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "+CPIN:"))
		}
	}
	return ""
}

// parseCIMI parses AT+CIMI response (plain IMSI number).
//
// Input:
//
//	AT+CIMI
//	310410123456789
//	OK
func parseCIMI(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT") {
			continue
		}
		// IMSI is a plain numeric string (15 digits)
		if len(line) >= 10 && isDigits(line) {
			return line
		}
	}
	return ""
}

// parseICCID parses AT+ICCID? response.
//
// Input: "+ICCID: 8901260882318054816"
func parseICCID(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+ICCID:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "+ICCID:"))
		}
	}
	return ""
}

// parseCOPS parses AT+COPS? response.
//
// Input: "+COPS: 0,0,\"T-Mobile\",7"
func parseCOPS(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+COPS:") {
			continue
		}
		val := strings.TrimPrefix(line, "+COPS:")
		val = strings.TrimSpace(val)
		// Format: mode,format,"operator",act
		// Extract the quoted operator name
		start := strings.Index(val, "\"")
		if start < 0 {
			return val // Return raw if no quotes
		}
		end := strings.Index(val[start+1:], "\"")
		if end < 0 {
			return val[start+1:]
		}
		return val[start+1 : start+1+end]
	}
	return ""
}

// regStatusName maps 3GPP registration stat values to names.
var regStatusNames = map[int]string{
	0: "Not registered",
	1: "Registered, home",
	2: "Searching",
	3: "Registration denied",
	4: "Unknown",
	5: "Registered, roaming",
}

// parseRegStatus parses a +CREG/+CEREG/+CGREG response line.
//
// Input: "+CREG: 0,1" or "+CEREG: 0,1,\"1234\",\"01234567\",7"
func parseRegStatus(resp, prefix string) RegStatus {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		parts := strings.SplitN(val, ",", 3)
		if len(parts) < 2 {
			return RegStatus{Raw: val}
		}
		stat := parseIntValue(strings.TrimSpace(parts[1]))
		name, ok := regStatusNames[stat]
		if !ok {
			name = fmt.Sprintf("Unknown (%d)", stat)
		}
		return RegStatus{Status: name, Raw: val}
	}
	return RegStatus{}
}

func parseCREG(resp string) RegStatus  { return parseRegStatus(resp, "+CREG:") }
func parseCEREG(resp string) RegStatus { return parseRegStatus(resp, "+CEREG:") }
func parseCGREG(resp string) RegStatus { return parseRegStatus(resp, "+CGREG:") }

// parseCGDCONT parses AT+CGDCONT? response into APN description strings.
//
// Input: "+CGDCONT: 1,\"IP\",\"internet\",,0,0,0"
func parseCGDCONT(resp string) []string {
	var apns []string
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+CGDCONT:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "+CGDCONT:"))
		apns = append(apns, val)
	}
	return apns
}

// parseLTEINFO parses AT!LTEINFO? response into serving, intra, and inter sections.
//
// Input:
//
//	!LTEINFO:
//	Serving:   EARFCN MCC MNC   TAC      CID Bd D U SNR PCI  RSRQ   RSRP   RSSI RXLV
//	              800 310 410 35666 0A14666A  2 5 5  22 404 -14.1  -79.6  -45.5 --
//	IntraFreq:                                          PCI  RSRQ   RSRP   RSSI RXLV
//	                                                    404 -14.1  -79.6  -45.5 --
//	InterFreq: EARFCN ThresholdLow ThresholdHi Priority PCI  RSRQ   RSRP   RSSI RXLV
//	             5110            0           0        0 272 -12.1  -80.9  -50.6   0
func parseLTEINFO(resp string) LTECellInfo {
	info := LTECellInfo{}
	var currentSection string
	var headers []string

	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") || line == "!LTEINFO:" {
			continue
		}

		// Detect section headers
		switch {
		case strings.HasPrefix(line, "Serving:"):
			currentSection = "serving"
			headerLine := strings.TrimPrefix(line, "Serving:")
			headers = strings.Fields(strings.TrimSpace(headerLine))
			continue
		case strings.HasPrefix(line, "IntraFreq:"):
			currentSection = "intra"
			headerLine := strings.TrimPrefix(line, "IntraFreq:")
			headers = strings.Fields(strings.TrimSpace(headerLine))
			continue
		case strings.HasPrefix(line, "InterFreq:"):
			currentSection = "inter"
			headerLine := strings.TrimPrefix(line, "InterFreq:")
			headers = strings.Fields(strings.TrimSpace(headerLine))
			continue
		}

		if currentSection == "" || len(headers) == 0 {
			continue
		}

		// Non-LTE section header (e.g. "WCDMA:", "UMTS:") — stop parsing.
		// Data rows start with numbers, not "WORD:".
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasSuffix(fields[0], ":") {
			currentSection = ""
			continue
		}

		row := make(map[string]string)
		for i, h := range headers {
			if i < len(fields) {
				row[h] = fields[i]
			}
		}

		switch currentSection {
		case "serving":
			info.Serving = append(info.Serving, row)
		case "intra":
			info.IntraFreq = append(info.IntraFreq, row)
		case "inter":
			info.InterFreq = append(info.InterFreq, row)
		}
	}
	return info
}

// parseLTECA parses AT!LTECA? response into structured CA band combinations.
//
// Input:
//
//	Hardware:
//	LTEB1: B8,
//	LTEB2: B2, B5, B12, B13, B29,
//	LTEB4: B4, B5, B12, B13, B29,
//	Permitted Bands:
//	LTEB1: B8,
//	LTEB2: B2, B5, B12, B13, B29,
//	Prune_ca_combos:
//	Empty
func parseLTECA(resp string) LTECAInfo {
	info := LTECAInfo{}
	section := "" // "hardware", "permitted", "pruned"

	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") {
			continue
		}
		if strings.HasPrefix(line, "!LTECA") {
			continue
		}

		// Detect section headers
		lower := strings.ToLower(line)
		switch {
		case lower == "hardware:" || strings.HasPrefix(lower, "hardware"):
			section = "hardware"
			continue
		case strings.HasPrefix(lower, "permitted"):
			section = "permitted"
			continue
		case strings.HasPrefix(lower, "prune"):
			section = "pruned"
			continue
		}

		// Parse band combo lines: "LTEB2: B2, B5, B12, B13, B29,"
		if strings.HasPrefix(line, "LTEB") {
			combo := parseBandComboLine(line)
			switch section {
			case "hardware":
				info.Hardware = append(info.Hardware, combo)
			case "permitted":
				info.Permitted = append(info.Permitted, combo)
			}
			continue
		}

		// Pruned section content (usually "Empty")
		if section == "pruned" && info.Pruned == "" {
			info.Pruned = line
		}
	}
	return info
}

// parseBandComboLine parses "LTEB2: B2, B5, B12, B13, B29," into a BandCombo.
func parseBandComboLine(line string) BandCombo {
	combo := BandCombo{}

	// Split on ":"  →  "LTEB2" and "B2, B5, B12, B13, B29,"
	k, v := splitKV(line, ":")
	// Extract primary band: "LTEB2" → "B2"
	combo.Primary = strings.TrimPrefix(k, "LTE")

	// Parse secondary bands from comma-separated list
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" && strings.HasPrefix(part, "B") {
			combo.Secondary = append(combo.Secondary, part)
		}
	}
	return combo
}

// parseHWID parses AT!HWID? response.
//
// Input: "Revision: 0.5"
func parseHWID(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" || strings.HasPrefix(line, "AT!") {
			continue
		}
		if strings.HasPrefix(line, "!HWID") {
			continue
		}
		// Response is "Revision: 0.5" — extract the value after ":"
		if k, v := splitKV(line, ":"); k != "" {
			return v
		}
		// Fallback: return the raw line (some firmware may just return a value)
		return line
	}
	return ""
}

// isDigits returns true if s contains only ASCII digits.
func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitLines splits response text into lines, handling \r\n.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// splitKV splits "key: value" or "key:value" into key and value.
// Returns empty key if no colon found.
func splitKV(line, sep string) (key, value string) {
	idx := strings.Index(line, sep)
	if idx < 0 {
		return "", ""
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+len(sep):])
}

// parseIntValue parses a trimmed string to int, returning 0 on failure.
func parseIntValue(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// parseSingleLineValue extracts a plain value from responses like:
//
//	AT!USBVID?
//	!USBVID:
//	413C
//
//	OK
//
// Returns the first non-empty line that isn't a command echo, prefix, or OK/ERROR.
func parseSingleLineValue(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" {
			continue
		}
		// Skip any AT command echo (own or foreign)
		if strings.HasPrefix(line, "AT") {
			continue
		}
		// Skip header/prefix lines like "!USBVID:"
		if strings.HasPrefix(line, "!") && strings.HasSuffix(line, ":") {
			continue
		}
		// Skip lines that look like response headers from other commands
		if strings.HasPrefix(line, "!") {
			continue
		}
		return line
	}
	return ""
}

// snapshotInfo creates a deep copy of info, cloning all map fields to prevent
// aliasing when the original continues to be modified after the snapshot.
func snapshotInfo(info *Info) *Info {
	cp := *info
	cp.Custom = maps.Clone(info.Custom)
	cp.Diagnostics.GStatus = maps.Clone(info.Diagnostics.GStatus)
	cp.Power.LPMVoters = maps.Clone(info.Power.LPMVoters)
	cp.LTEDetail.Serving = cloneSliceMap(info.LTEDetail.Serving)
	cp.LTEDetail.IntraFreq = cloneSliceMap(info.LTEDetail.IntraFreq)
	cp.LTEDetail.InterFreq = cloneSliceMap(info.LTEDetail.InterFreq)
	return &cp
}

// cloneSliceMap deep-copies a slice of string maps.
func cloneSliceMap(s []map[string]string) []map[string]string {
	if s == nil {
		return nil
	}
	cp := make([]map[string]string, len(s))
	for i, m := range s {
		cp[i] = maps.Clone(m)
	}
	return cp
}

// cleanATResponse strips the command echo, OK/ERROR terminators, and any
// foreign data that doesn't belong to this command's response.
func cleanATResponse(resp, cmd string) string {
	// Strip everything before the command echo (stale data from prior commands)
	if cmd != "" {
		if idx := strings.Index(resp, cmd); idx >= 0 {
			resp = resp[idx+len(cmd):]
		}
	}
	resp = strings.Replace(resp, "\r\nOK\r\n", "", 1)
	resp = strings.TrimSuffix(strings.TrimSpace(resp), "OK")
	cleaned := strings.TrimSpace(resp)

	// If the cleaned result still contains foreign AT command echoes,
	// extract just the first non-echo, non-prefix line.
	var lines []string
	for _, line := range splitLines(cleaned) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" {
			continue
		}
		if strings.HasPrefix(line, "AT") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}
	return cleaned
}

// ---------------------------------------------------------------------------
// Intel XMM info gathering
// ---------------------------------------------------------------------------

// getInfoStreamingIntel queries an Intel XMM modem (EM7345) using standard
// 3GPP and Intel AT+X commands. Maps results into the same Info struct so
// the display code works unchanged.
func getInfoStreamingIntel(port *Port, send func(*Info)) {
	info := &Info{
		Custom: make(map[string]string),
	}
	info.Diagnostics.GStatus = make(map[string]string)

	port.SendCommand("ATE1")

	// ── Identity (3GPP standard commands) ─────────────────────────
	if resp, err := port.SendCommand("AT+CGMI"); err == nil {
		info.Identity.Manufacturer = parsePlainValue(resp, "+CGMI")
	}
	if resp, err := port.SendCommand("AT+CGMM"); err == nil {
		info.Identity.Model = parsePlainValue(resp, "+CGMM")
	}
	if resp, err := port.SendCommand("AT+CGMR"); err == nil {
		info.Identity.Revision = parsePlainValue(resp, "+CGMR")
	}
	if resp, err := port.SendCommand("AT+CGSN"); err == nil {
		info.Identity.IMEI = parsePlainValue(resp, "+CGSN")
	}
	send(snapshotInfo(info)) // #1 — identity

	// ── Power state ───────────────────────────────────────────────
	if resp, err := port.SendCommand("AT+CFUN?"); err == nil {
		info.Power.State = parseCFUN(resp)
	}
	send(snapshotInfo(info)) // #2 — power

	// ── SIM ───────────────────────────────────────────────────────
	isLPM := info.Power.State == "minimum" || info.Power.State == "offline"
	if isLPM {
		info.SIM.Status = SIMStatusLPM
	} else {
		if resp, err := port.SendCommand("AT+CPIN?"); err == nil {
			info.SIM.Status = parseCPIN(resp)
		}
	}
	if resp, err := port.SendCommand("AT+CIMI"); err == nil {
		info.SIM.IMSI = parseCIMI(resp)
	}
	if resp, err := port.SendCommand("AT+CCID"); err == nil {
		info.SIM.ICCID = parseICCID(resp)
	}
	send(snapshotInfo(info)) // #3 — SIM

	// ── Network registration ──────────────────────────────────────
	if resp, err := port.SendCommand("AT+COPS?"); err == nil {
		info.Operator = parseCOPS(resp)
	}
	if resp, err := port.SendCommand("AT+CREG?"); err == nil {
		info.Registration.CS = parseCREG(resp)
	}
	if resp, err := port.SendCommand("AT+CEREG?"); err == nil {
		info.Registration.EPS = parseCEREG(resp)
	}
	if resp, err := port.SendCommand("AT+CGREG?"); err == nil {
		info.Registration.GPRS = parseCGREG(resp)
	}
	send(snapshotInfo(info)) // #4 — registration

	// ── Firmware (Intel-specific) ─────────────────────────────────
	if resp, err := port.SendCommand("AT+XGENDATA"); err == nil {
		info.Firmware.Current.Version, info.Firmware.Current.CarrierName = parseXGENDATA(resp)
		info.Firmware.Preferred = info.Firmware.Current // Intel has no separate preferred
	}
	send(snapshotInfo(info)) // #5 — firmware

	// ── Band / RAT (Intel AT+XACT) ───────────────────────────────
	if resp, err := port.SendCommand("AT+XACT?"); err == nil {
		info.Network.RATSelection, info.Network.CurrentBand = parseXACTQuery(resp)
	}
	if resp, err := port.SendCommand("AT+XACT=?"); err == nil {
		info.Network.AvailableBands = parseXACTTest(resp)
	}
	if resp, err := port.SendCommand("AT+CGDCONT?"); err == nil {
		info.APNs = parseCGDCONT(resp)
	}

	// ── Signal (Intel AT+XCESQ) ──────────────────────────────────
	if resp, err := port.SendCommand("AT+XCESQ?"); err == nil {
		parseXCESQ(resp, info)
	}
	if resp, err := port.SendCommand("AT+CSQ"); err == nil {
		rssi, ber := parseCSQ(resp)
		if info.Diagnostics.RSSI == 0 {
			info.Diagnostics.RSSI = rssi
			info.Diagnostics.BER = ber
			info.Diagnostics.SignalBars = rssiToBars(rssi)
		}
	}

	// ── Extended registration (shows active band) ─────────────────
	if resp, err := port.SendCommand("AT+XREG?"); err == nil {
		parseXREG(resp, info)
	}

	// ── GPS (Intel GNSS) ──────────────────────────────────────────
	if resp, err := port.SendCommand("AT%GPS?"); err == nil {
		info.GPS = parsePercentGPS(resp)
	}

	send(snapshotInfo(info)) // #6 — signal + bands + final
}

// getInfoForSectionIntel queries only the AT commands needed for a single
// info section on Intel XMM modems.
func getInfoForSectionIntel(port *Port, section string) *Info {
	info := &Info{
		Custom: make(map[string]string),
	}
	info.Diagnostics.GStatus = make(map[string]string)

	port.SendCommand("ATE1")

	switch section {
	case "identity":
		if resp, err := port.SendCommand("AT+CGMI"); err == nil {
			info.Identity.Manufacturer = parsePlainValue(resp, "+CGMI")
		}
		if resp, err := port.SendCommand("AT+CGMM"); err == nil {
			info.Identity.Model = parsePlainValue(resp, "+CGMM")
		}
		if resp, err := port.SendCommand("AT+CGMR"); err == nil {
			info.Identity.Revision = parsePlainValue(resp, "+CGMR")
		}
		if resp, err := port.SendCommand("AT+CGSN"); err == nil {
			info.Identity.IMEI = parsePlainValue(resp, "+CGSN")
		}
	case "power":
		if resp, err := port.SendCommand("AT+CFUN?"); err == nil {
			info.Power.State = parseCFUN(resp)
		}
	case "sim":
		if resp, err := port.SendCommand("AT+CPIN?"); err == nil {
			info.SIM.Status = parseCPIN(resp)
		}
		if resp, err := port.SendCommand("AT+CIMI"); err == nil {
			info.SIM.IMSI = parseCIMI(resp)
		}
		if resp, err := port.SendCommand("AT+CCID"); err == nil {
			info.SIM.ICCID = parseICCID(resp)
		}
	case "network":
		if resp, err := port.SendCommand("AT+COPS?"); err == nil {
			info.Operator = parseCOPS(resp)
		}
		if resp, err := port.SendCommand("AT+CREG?"); err == nil {
			info.Registration.CS = parseCREG(resp)
		}
		if resp, err := port.SendCommand("AT+CEREG?"); err == nil {
			info.Registration.EPS = parseCEREG(resp)
		}
		if resp, err := port.SendCommand("AT+CGREG?"); err == nil {
			info.Registration.GPRS = parseCGREG(resp)
		}
		if resp, err := port.SendCommand("AT+XACT?"); err == nil {
			info.Network.RATSelection, info.Network.CurrentBand = parseXACTQuery(resp)
		}
		if resp, err := port.SendCommand("AT+CGDCONT?"); err == nil {
			info.APNs = parseCGDCONT(resp)
		}
	case "firmware":
		if resp, err := port.SendCommand("AT+XGENDATA"); err == nil {
			info.Firmware.Current.Version, info.Firmware.Current.CarrierName = parseXGENDATA(resp)
			info.Firmware.Preferred = info.Firmware.Current
		}
	case "signal":
		if resp, err := port.SendCommand("AT+XCESQ?"); err == nil {
			parseXCESQ(resp, info)
		}
		if resp, err := port.SendCommand("AT+XREG?"); err == nil {
			parseXREG(resp, info)
		}
	case "bands":
		if resp, err := port.SendCommand("AT+XACT=?"); err == nil {
			info.Network.AvailableBands = parseXACTTest(resp)
		}
	case "gps":
		if resp, err := port.SendCommand("AT%GPS?"); err == nil {
			info.GPS = parsePercentGPS(resp)
		}
	}

	return info
}

// ---------------------------------------------------------------------------
// Intel XMM AT command parsers
// ---------------------------------------------------------------------------

// parsePlainValue extracts a value from a response where the modem echoes
// the command then returns a plain text value on the next line.
//
// Example:
//
//	AT+CGMI
//	Sierra Wireless Inc.
//	OK
func parsePlainValue(resp, prefix string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" || line == "ERROR" {
			continue
		}
		if strings.HasPrefix(line, "AT") {
			continue
		}
		// Skip +PREFIX: header lines (shouldn't appear for these commands, but be safe)
		if strings.HasPrefix(line, prefix+":") {
			return strings.TrimSpace(line[len(prefix)+1:])
		}
		return line
	}
	return ""
}

// parseCFUN parses AT+CFUN? response into a human-readable power state.
//
// Input: +CFUN: 1,0
func parseCFUN(resp string) string {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+CFUN:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "+CFUN:"))
		parts := strings.SplitN(val, ",", 2)
		if len(parts) == 0 {
			return val
		}
		switch strings.TrimSpace(parts[0]) {
		case "0":
			return "minimum"
		case "1":
			return "online"
		case "4":
			return "offline"
		default:
			return val
		}
	}
	return ""
}

// parseXGENDATA parses AT+XGENDATA response into version and carrier strings.
//
// Input:
//
//	+XGENDATA: "    FIH7160_XMM7160_V1.2_MBIM_GNSS_NAND_REV_4.5 2016-Oct-20 09:18:18
//	*FIH7160_V1.2_WW_01.1644.00_TS*"
func parseXGENDATA(resp string) (version, carrier string) {
	// Extract content between quotes
	full := ""
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+XGENDATA:") {
			full = strings.TrimPrefix(line, "+XGENDATA:")
		} else if full != "" && line != "OK" && line != "" && !strings.HasPrefix(line, "AT") {
			full += " " + line
		}
	}
	full = strings.TrimSpace(full)

	// Extract the carrier variant between * markers
	re := regexp.MustCompile(`\*([^*]+)\*`)
	if m := re.FindStringSubmatch(full); len(m) > 1 {
		version = strings.TrimSpace(m[1])
	}

	// Extract carrier from the version string (e.g., "FIH7160_V1.2_WW_01.1644.00_TS")
	// The "WW" part is the carrier variant
	parts := strings.Split(version, "_")
	for i, p := range parts {
		if p == "WW" || p == "ATT" || p == "VZW" || p == "SPR" || p == "TMO" {
			carrier = p
			break
		}
		// Last resort: second segment after platform+version
		if i >= 2 && len(p) <= 4 && carrier == "" {
			carrier = p
		}
	}
	if carrier == "" {
		carrier = "GENERIC"
	}

	return version, carrier
}

// xactModeNames maps AT+XACT mode indices to descriptions.
var xactModeNames = [...]string{
	0: "2G only",
	1: "3G only",
	2: "4G only",
	3: "2G+3G",
	4: "3G+4G",
	5: "2G+4G",
	6: "2G+3G+4G",
}

// parseXACTQuery parses AT+XACT? response into RAT selection and a summary band entry.
//
// Input: +XACT: 6,2,1,900,1800,1900,850,1,2,4,5,8,101,102,...,120
//
// Format: <mode>,<pref1>,<pref2>,<band1>,<band2>,...
func parseXACTQuery(resp string) (rat RATSelection, band BandEntry) {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+XACT:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "+XACT:"))
		parts := strings.Split(val, ",")
		if len(parts) < 1 {
			return
		}

		mode, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		rat.Index = mode
		if mode >= 0 && mode < len(xactModeNames) {
			rat.Name = xactModeNames[mode]
		}

		// Bands start at index 3 (after mode, pref1, pref2)
		if len(parts) > 3 {
			band.Name = formatXACTBands(parts[3:])
		}
		return
	}
	return
}

// formatXACTBands formats a list of XACT band number strings into a
// human-readable summary like "2G:900/1800 3G:B1/B2 4G:B1/B2/B3".
func formatXACTBands(raw []string) string {
	var bands2G, bands3G, bands4G []string
	for _, b := range raw {
		n, err := strconv.Atoi(strings.TrimSpace(b))
		if err != nil || n == 0 {
			continue
		}
		switch {
		case n > 300:
			bands2G = append(bands2G, fmt.Sprintf("%d", n))
		case n >= 100:
			bands4G = append(bands4G, fmt.Sprintf("B%d", n-100))
		default:
			bands3G = append(bands3G, fmt.Sprintf("B%d", n))
		}
	}
	var summary []string
	if len(bands2G) > 0 {
		summary = append(summary, "2G:"+strings.Join(bands2G, "/"))
	}
	if len(bands3G) > 0 {
		summary = append(summary, "3G:"+strings.Join(bands3G, "/"))
	}
	if len(bands4G) > 0 {
		summary = append(summary, "4G:"+strings.Join(bands4G, "/"))
	}
	return strings.Join(summary, " ")
}

// parseXACTTest parses AT+XACT=? response into available band entries.
// Returns one BandEntry per band group (2G, 3G, 4G) for display.
//
// Input: +XACT: (0-6),(0-2),0,900,1800,1900,850,1,2,4,5,8,101,...,120
func parseXACTTest(resp string) []BandEntry {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+XACT:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "+XACT:"))

		// Strip parenthesized ranges like "(0-6),(0-2)" to get the band list.
		// Find the last ')' and take everything after it.
		lastParen := strings.LastIndex(val, ")")
		if lastParen >= 0 {
			val = val[lastParen+1:]
		}
		// Remove leading comma
		val = strings.TrimLeft(val, ",")

		parts := strings.Split(val, ",")
		var bands2G, bands3G, bands4G []string
		for _, p := range parts {
			n, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil || n == 0 {
				continue
			}
			switch {
			case n > 300:
				bands2G = append(bands2G, fmt.Sprintf("%d MHz", n))
			case n >= 100:
				bands4G = append(bands4G, fmt.Sprintf("Band %d", n-100))
			default:
				bands3G = append(bands3G, fmt.Sprintf("Band %d", n))
			}
		}

		var entries []BandEntry
		if len(bands2G) > 0 {
			entries = append(entries, BandEntry{
				Index: 0,
				Name:  "GSM: " + strings.Join(bands2G, ", "),
			})
		}
		if len(bands3G) > 0 {
			entries = append(entries, BandEntry{
				Index: 1,
				Name:  "UMTS: " + strings.Join(bands3G, ", "),
			})
		}
		if len(bands4G) > 0 {
			entries = append(entries, BandEntry{
				Index: 2,
				Name:  "LTE: " + strings.Join(bands4G, ", "),
			})
		}
		return entries
	}
	return nil
}

// parseXCESQ parses AT+XCESQ? response and populates info.Diagnostics.
//
// Input: +XCESQ: 0,99,99,255,255,24,51,18
//
// Fields: n, rxlev, ber, rscp, ecn0, rsrq, rsrp, rssnr
func parseXCESQ(resp string, info *Info) {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+XCESQ:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "+XCESQ:"))
		parts := strings.Split(val, ",")
		if len(parts) < 8 {
			return
		}

		// Parse each field
		rxlev, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		rscp, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
		ecn0, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
		rsrq, _ := strconv.Atoi(strings.TrimSpace(parts[5]))
		rsrp, _ := strconv.Atoi(strings.TrimSpace(parts[6]))
		rssnr, _ := strconv.Atoi(strings.TrimSpace(parts[7]))

		// Populate GStatus map with signal info in the same format
		// the display code expects (matching AT!GSTATUS? keys).
		gs := info.Diagnostics.GStatus

		// LTE signal (most common for EM7345)
		if rsrp != 255 {
			rsrpDBM := rsrp - 141
			gs["RSRP (dBm)"] = fmt.Sprintf("%d", rsrpDBM)
			info.Diagnostics.RSSI = rsrpDBM
			info.Diagnostics.SignalBars = rssiToBars(rsrpDBM)
		}
		if rsrq != 255 {
			rsrqDB := float64(rsrq)/2.0 - 19.5
			gs["RSRQ (dB)"] = fmt.Sprintf("%.1f", rsrqDB)
		}
		if rssnr != 255 {
			snrDB := float64(rssnr) / 2.0
			gs["SINR (dB)"] = fmt.Sprintf("%.1f", snrDB)
		}

		// UMTS signal
		if rscp != 255 {
			rscpDBM := rscp - 121
			gs["RSCP (dBm)"] = fmt.Sprintf("%d", rscpDBM)
			if info.Diagnostics.RSSI == 0 {
				info.Diagnostics.RSSI = rscpDBM
				info.Diagnostics.SignalBars = rssiToBars(rscpDBM)
			}
		}
		if ecn0 != 255 {
			ecioDB := float64(ecn0)/2.0 - 24.5
			gs["Ec/Io (dB)"] = fmt.Sprintf("%.1f", ecioDB)
		}

		// GSM signal
		if rxlev != 99 {
			rssiDBM := rxlev - 110
			gs["RSSI (dBm)"] = fmt.Sprintf("%d", rssiDBM)
			if info.Diagnostics.RSSI == 0 {
				info.Diagnostics.RSSI = rssiDBM
				info.Diagnostics.SignalBars = rssiToBars(rssiDBM)
			}
		}

		return
	}
}

// parsePercentGPS parses AT%GPS? response into a GPSInfo struct.
//
// Input: %GPS: <enabled> <fix> <sats> <hdop> <pdop> <lat> <lon> <alt>
// Example: %GPS: 1 0 0 0.00000 0.00000 0.00000 0.00000 0
func parsePercentGPS(resp string) GPSInfo {
	var gps GPSInfo
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "%GPS:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "%GPS:"))
		fields := strings.Fields(val)
		if len(fields) < 8 {
			return gps
		}

		// Field 0: enabled (0/1)
		if fields[0] == "1" {
			gps.SessionStatus = "active"
		} else {
			gps.SessionStatus = "inactive"
		}

		// Field 1: fix (0/1)
		if fields[1] == "1" {
			gps.FixStatus = "fix acquired"
		} else {
			gps.FixStatus = "no fix"
			return gps
		}

		// Fields 2-7: sats, hdop, pdop, lat, lon, alt
		if n, err := strconv.Atoi(fields[2]); err == nil && n > 0 {
			gps.Satellites = n
		}
		if fields[3] != "0.00000" && fields[3] != "0" {
			gps.HDOP = fields[3]
		}
		if fields[4] != "0.00000" && fields[4] != "0" {
			gps.PDOP = fields[4]
		}
		if fields[5] != "0.00000" && fields[5] != "0" {
			gps.Latitude = fields[5]
		}
		if fields[6] != "0.00000" && fields[6] != "0" {
			gps.Longitude = fields[6]
		}
		if fields[7] != "0" {
			gps.Altitude = fields[7]
		}

		return gps
	}
	return gps
}

// parseXREG parses AT+XREG? response and adds band info to diagnostics.
//
// Input: +XREG: 0,8,BAND_LTE_20,0
func parseXREG(resp string, info *Info) {
	for _, line := range splitLines(resp) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+XREG:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "+XREG:"))
		parts := strings.Split(val, ",")
		if len(parts) >= 3 {
			bandStr := strings.TrimSpace(parts[2])
			if strings.HasPrefix(bandStr, "BAND_") {
				info.Diagnostics.GStatus["LTE band"] = bandStr
			}
		}
		return
	}
}
