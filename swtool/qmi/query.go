package qmi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/qmux"
	"github.com/danielewood/sierra-wireless-modems/swtool/internal/sysutil"
)

// QueryClient queries modem telemetry via native QMI or qmicli fallback.
// Use NewQueryClient to create one from a CDC-WDM or QCQMI device path.
// The native QMI connection is lazily opened on first query and reused
// for all subsequent queries (connection pooling).
type QueryClient struct {
	device string
	mbim   bool // true for /dev/cdc-wdm* (needs --device-open-mbim)
	log    *log.Logger

	conn      *qmux.Conn // persistent native QMI connection (nil until first use)
	connTried bool        // true after first attempt to open native conn
}

// NewQueryClient creates a QMI query client.
// device is the control device path (e.g. "/dev/cdc-wdm0" or "/dev/qcqmi0").
// mbim should be true when using a CDC-WDM device (MBIM mode).
func NewQueryClient(device string, mbim bool, l *log.Logger) *QueryClient {
	return &QueryClient{device: device, mbim: mbim, log: l}
}

// Close releases the persistent native QMI connection, if any.
func (q *QueryClient) Close() {
	if q.conn != nil {
		q.conn.Close()
		q.conn = nil
	}
}

// nativeConn returns the persistent native QMI connection, opening it
// on first call. Returns nil if native QMI is unavailable.
func (q *QueryClient) nativeConn() *qmux.Conn {
	if q.connTried {
		return q.conn
	}
	q.connTried = true
	conn, err := qmux.Open(q.device)
	if err != nil {
		q.log.Debugf("native QMI connection unavailable, will use qmicli: %v", err)
		return nil
	}
	q.conn = conn
	return q.conn
}

// SignalInfo holds per-technology signal metrics from --nas-get-signal-info.
type SignalInfo struct {
	LTE   *LTESignal   `json:"lte,omitempty"`
	WCDMA *WCDMASignal `json:"wcdma,omitempty"`
	GSM   *GSMSignal   `json:"gsm,omitempty"`
}

// LTESignal holds LTE signal metrics.
type LTESignal struct {
	RSSI string `json:"rssi"` // dBm
	RSRP string `json:"rsrp"` // dBm
	RSRQ string `json:"rsrq"` // dB
	SNR  string `json:"snr"`  // dB
}

// WCDMASignal holds WCDMA signal metrics.
type WCDMASignal struct {
	RSSI string `json:"rssi"` // dBm
	ECIO string `json:"ecio"` // dB
}

// GSMSignal holds GSM signal metrics.
type GSMSignal struct {
	RSSI string `json:"rssi"` // dBm
}

// ServingSystem holds network registration from --nas-get-serving-system.
type ServingSystem struct {
	Registered bool   `json:"registered"`
	MCC        string `json:"mcc,omitempty"`
	MNC        string `json:"mnc,omitempty"`
	RAT        string `json:"rat,omitempty"`  // "lte", "umts", "gsm", etc.
	Roaming    bool   `json:"roaming"`
	Domain     string `json:"domain,omitempty"` // "cs-ps", "ps", "cs"
}

// CellLocationInfo holds serving and neighbor cell data from --nas-get-cell-location-info.
type CellLocationInfo struct {
	LTEIntra []map[string]string `json:"lte_intra,omitempty"`
	LTEInter []map[string]string `json:"lte_inter,omitempty"`
}

// SystemInfo holds network system info from --nas-get-system-info.
type SystemInfo struct {
	ServiceStatus string `json:"service_status,omitempty"`
	Domain        string `json:"domain,omitempty"`
	Roaming       string `json:"roaming,omitempty"`
	MCC           string `json:"mcc,omitempty"`
	MNC           string `json:"mnc,omitempty"`
	TAC           string `json:"tac,omitempty"`
	CellID        string `json:"cell_id,omitempty"`
}

// run executes a qmicli command and returns the output.
// Used as fallback when native QMI is unavailable.
func (q *QueryClient) run(action string) (string, error) {
	args := []string{}
	if q.mbim {
		args = append(args, "--device-open-mbim")
	}
	args = append(args, "-p", "-d", q.device, action)
	return sysutil.RunCommand(q.log, "qmicli", args...)
}

// GetSignalInfo queries NAS signal info (RSSI, RSRP, RSRQ, SNR).
func (q *QueryClient) GetSignalInfo() (*SignalInfo, error) {
	if c := q.nativeConn(); c != nil {
		sig, err := qmux.GetSignalInfo(c)
		if err != nil {
			return nil, fmt.Errorf("nas-get-signal-info: %w", err)
		}
		return convertSignalInfo(sig), nil
	}

	out, err := q.run("--nas-get-signal-info")
	if err != nil {
		return nil, fmt.Errorf("nas-get-signal-info: %w", err)
	}
	return parseSignalInfo(out), nil
}

// GetServingSystem queries NAS serving system (operator, registration, RAT).
func (q *QueryClient) GetServingSystem() (*ServingSystem, error) {
	if c := q.nativeConn(); c != nil {
		sys, err := qmux.GetServingSystem(c)
		if err != nil {
			return nil, fmt.Errorf("nas-get-serving-system: %w", err)
		}
		return convertServingSystem(sys), nil
	}

	out, err := q.run("--nas-get-serving-system")
	if err != nil {
		return nil, fmt.Errorf("nas-get-serving-system: %w", err)
	}
	return parseServingSystem(out), nil
}

// GetCellLocationInfo queries NAS cell location (serving + neighbor cells).
func (q *QueryClient) GetCellLocationInfo() (*CellLocationInfo, error) {
	if c := q.nativeConn(); c != nil {
		cells, err := qmux.GetCellLocationInfo(c)
		if err != nil {
			return nil, fmt.Errorf("nas-get-cell-location-info: %w", err)
		}
		return convertCellLocationInfo(cells), nil
	}

	out, err := q.run("--nas-get-cell-location-info")
	if err != nil {
		return nil, fmt.Errorf("nas-get-cell-location-info: %w", err)
	}
	return parseCellLocationInfo(out), nil
}

// GetSystemInfo queries NAS system info (LTE band, TAC, Cell ID, service status).
func (q *QueryClient) GetSystemInfo() (*SystemInfo, error) {
	if c := q.nativeConn(); c != nil {
		sys, err := qmux.GetSystemInfo(c)
		if err != nil {
			return nil, fmt.Errorf("nas-get-system-info: %w", err)
		}
		return convertSystemInfo(sys), nil
	}

	out, err := q.run("--nas-get-system-info")
	if err != nil {
		return nil, fmt.Errorf("nas-get-system-info: %w", err)
	}
	return parseSystemInfo(out), nil
}

// --- Native → Public type converters ---

func convertSignalInfo(n *qmux.SignalInfo) *SignalInfo {
	info := &SignalInfo{}
	if n.LTE != nil {
		info.LTE = &LTESignal{
			RSSI: qmux.FormatSignalDBM(n.LTE.RSSI),
			RSRQ: qmux.FormatSignalDB(n.LTE.RSRQ),
			RSRP: qmux.FormatSignalDBM(n.LTE.RSRP),
			SNR:  qmux.FormatSNR(n.LTE.SNR),
		}
	}
	if n.WCDMA != nil {
		info.WCDMA = &WCDMASignal{
			RSSI: fmt.Sprintf("%d dBm", n.WCDMA.RSSI),
			ECIO: qmux.FormatSignalTenthsDB(n.WCDMA.ECIO),
		}
	}
	if n.GSM != nil {
		info.GSM = &GSMSignal{
			RSSI: fmt.Sprintf("%d dBm", n.GSM.RSSI),
		}
	}
	return info
}

func convertServingSystem(n *qmux.ServingSystem) *ServingSystem {
	sys := &ServingSystem{
		Registered: n.Registered,
		MCC:        strconv.FormatUint(uint64(n.MCC), 10),
		MNC:        strconv.FormatUint(uint64(n.MNC), 10),
		RAT:        n.RAT,
		Roaming:    n.Roaming,
	}
	if !n.Registered {
		sys.MCC = ""
		sys.MNC = ""
	}
	if n.CSAttached && n.PSAttached {
		sys.Domain = "cs-ps"
	} else if n.CSAttached {
		sys.Domain = "cs"
	} else if n.PSAttached {
		sys.Domain = "ps"
	}
	return sys
}

func convertCellLocationInfo(n *qmux.CellLocationInfo) *CellLocationInfo {
	info := &CellLocationInfo{}
	for _, cell := range n.LTEIntra {
		m := map[string]string{
			"PCI":  strconv.FormatUint(uint64(cell.PCI), 10),
			"RSRQ": qmux.FormatSignalTenthsDB(cell.RSRQ),
			"RSRP": qmux.FormatSignalTenthsDBM(cell.RSRP),
			"RSSI": qmux.FormatSignalTenthsDBM(cell.RSSI),
		}
		info.LTEIntra = append(info.LTEIntra, m)
	}
	for _, cell := range n.LTEInter {
		m := map[string]string{
			"PCI":    strconv.FormatUint(uint64(cell.PCI), 10),
			"RSRQ":   qmux.FormatSignalTenthsDB(cell.RSRQ),
			"RSRP":   qmux.FormatSignalTenthsDBM(cell.RSRP),
			"RSSI":   qmux.FormatSignalTenthsDBM(cell.RSSI),
			"EARFCN": strconv.FormatUint(uint64(cell.EARFCN), 10),
		}
		info.LTEInter = append(info.LTEInter, m)
	}
	return info
}

func convertSystemInfo(n *qmux.SystemInfo) *SystemInfo {
	info := &SystemInfo{
		ServiceStatus: qmux.ServiceStatusString(n.ServiceStatus),
		Domain:        qmux.DomainString(n.Domain),
	}
	if n.Roaming {
		info.Roaming = "on"
	} else {
		info.Roaming = "off"
	}
	if n.MCC > 0 {
		info.MCC = strconv.FormatUint(uint64(n.MCC), 10)
	}
	if n.MNC > 0 {
		info.MNC = strconv.FormatUint(uint64(n.MNC), 10)
	}
	if n.TAC > 0 {
		info.TAC = strconv.FormatUint(uint64(n.TAC), 10)
	}
	if n.CellID > 0 {
		info.CellID = strconv.FormatUint(uint64(n.CellID), 10)
	}
	return info
}

// --- Parsers ---

// parseSignalInfo parses --nas-get-signal-info output.
//
// Example:
//
//	[/dev/cdc-wdm0] Successfully got signal info
//	LTE:
//		RSSI: '-55 dBm'
//		RSRQ: '-11 dB'
//		RSRP: '-89 dBm'
//		SNR: '7.8 dB'
//	WCDMA:
//		RSSI: '-60 dBm'
//		ECIO: '-4 dB'
func parseSignalInfo(output string) *SignalInfo {
	info := &SignalInfo{}
	section := ""

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Section headers: "LTE:", "WCDMA:", "GSM:", etc.
		if !strings.Contains(trimmed, "'") && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		key, val := parseQMIKV(trimmed)
		if key == "" {
			continue
		}

		switch section {
		case "LTE":
			if info.LTE == nil {
				info.LTE = &LTESignal{}
			}
			switch key {
			case "RSSI":
				info.LTE.RSSI = val
			case "RSRP":
				info.LTE.RSRP = val
			case "RSRQ":
				info.LTE.RSRQ = val
			case "SNR":
				info.LTE.SNR = val
			}
		case "WCDMA":
			if info.WCDMA == nil {
				info.WCDMA = &WCDMASignal{}
			}
			switch key {
			case "RSSI":
				info.WCDMA.RSSI = val
			case "ECIO":
				info.WCDMA.ECIO = val
			}
		case "GSM":
			if info.GSM == nil {
				info.GSM = &GSMSignal{}
			}
			if key == "RSSI" {
				info.GSM.RSSI = val
			}
		}
	}
	return info
}

// parseServingSystem parses --nas-get-serving-system output.
//
// Example:
//
//	[/dev/cdc-wdm0] Successfully got serving system:
//		Registration state: 'registered'
//		CS: 'attached'
//		PS: 'attached'
//		Selected network: '3gpp'
//		Radio interfaces: '1'
//			[0]: 'lte'
//		Roaming status: 'off'
//		Data service capability: 'lte'
//		Current PLMN:
//			MCC: '310'
//			MNC: '260'
//			Description: 'T-Mobile'
func parseServingSystem(output string) *ServingSystem {
	sys := &ServingSystem{}
	inPLMN := false

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Detect PLMN subsection
		if trimmed == "Current PLMN:" {
			inPLMN = true
			continue
		}

		key, val := parseQMIKV(trimmed)
		if key == "" {
			continue
		}

		// Radio interface lines like "[0]: 'lte'"
		if strings.HasPrefix(key, "[") && sys.RAT == "" {
			sys.RAT = val
			continue
		}

		if inPLMN {
			switch key {
			case "MCC":
				sys.MCC = val
			case "MNC":
				sys.MNC = val
			case "Description":
				// End of PLMN subsection after description
				inPLMN = false
			}
			continue
		}

		switch key {
		case "Registration state":
			sys.Registered = val == "registered"
		case "Roaming status":
			sys.Roaming = val == "on"
		case "CS":
			if val == "attached" {
				sys.Domain = appendDomain(sys.Domain, "cs")
			}
		case "PS":
			if val == "attached" {
				sys.Domain = appendDomain(sys.Domain, "ps")
			}
		}
	}
	return sys
}

// parseCellLocationInfo parses --nas-get-cell-location-info output.
//
// The output has sections for different technologies. We focus on LTE:
//
//	LTE Info - Intra-Frequency:
//		Cell [0]:
//			Physical Cell ID: '123'
//			RSRQ: '-11.0 dB'
//			RSRP: '-89.0 dBm'
//			RSSI: '-55.0 dBm'
//			Cell Selection RX Level: '0'
//	LTE Info - Inter-Frequency:
//		...
func parseCellLocationInfo(output string) *CellLocationInfo {
	info := &CellLocationInfo{}
	section := ""
	var currentCell map[string]string

	flushCell := func() {
		if currentCell == nil {
			return
		}
		switch {
		case strings.Contains(section, "Intra"):
			info.LTEIntra = append(info.LTEIntra, currentCell)
		case strings.Contains(section, "Inter"):
			info.LTEInter = append(info.LTEInter, currentCell)
		}
		currentCell = nil
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Section headers
		if strings.HasPrefix(trimmed, "LTE Info") {
			flushCell()
			section = trimmed
			continue
		}

		// Cell marker: "Cell [0]:", "Cell [1]:", etc.
		if strings.HasPrefix(trimmed, "Cell [") && strings.HasSuffix(trimmed, ":") {
			flushCell()
			currentCell = make(map[string]string)
			continue
		}

		if currentCell != nil {
			key, val := parseQMIKV(trimmed)
			if key != "" {
				// Use short keys matching the AT command style
				shortKey := cellLocationKey(key)
				currentCell[shortKey] = val
			}
		}
	}
	flushCell()
	return info
}

// parseSystemInfo parses --nas-get-system-info output.
//
// Example:
//
//	[/dev/cdc-wdm0] Successfully got system info:
//	LTE:
//		Service status: 'available'
//		True service status: 'available'
//		Preferred data path: 'yes'
//		Domain: 'cs-ps'
//		Roaming status: 'off'
//		MCC: '310'
//		MNC: '260'
//		Tracking area code: '12345'
//		Cell ID: '67890'
//		...
func parseSystemInfo(output string) *SystemInfo {
	info := &SystemInfo{}
	inLTE := false

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Look for LTE section (most relevant for our modems)
		if !strings.Contains(trimmed, "'") && strings.HasSuffix(trimmed, ":") {
			section := strings.TrimSuffix(trimmed, ":")
			inLTE = section == "LTE"
			continue
		}

		if !inLTE {
			continue
		}

		key, val := parseQMIKV(trimmed)
		if key == "" {
			continue
		}

		switch key {
		case "Service status":
			info.ServiceStatus = val
		case "Domain":
			info.Domain = val
		case "Roaming status":
			info.Roaming = val
		case "MCC":
			info.MCC = val
		case "MNC":
			info.MNC = val
		case "Tracking area code":
			info.TAC = val
		case "Cell ID":
			info.CellID = val
		}
	}
	return info
}

// --- DMS Stored Images / Firmware Preference ---

// StoredImage represents a single firmware image slot from --dms-list-stored-images.
type StoredImage struct {
	Type         string // "modem" or "pri"
	Slot         int    // 0-based, from [MODEM0], [PRI1], etc.
	UniqueID     string // PRI: config version; MODEM: "?_?"
	BuildID      string // "{fw_version}_{carrier}" e.g. "02.39.00.00_GENERIC"
	StorageIndex int
	FailureCount int
	Current      bool // true if >>>>>>>>>> [CURRENT] <<<<<<<<<<
}

// FirmwarePreferenceImage represents a firmware preference entry from --dms-get-firmware-preference.
type FirmwarePreferenceImage struct {
	Type     string // "modem" or "pri"
	UniqueID string
	BuildID  string
}

// slotHeaderRe matches section headers like [MODEM0], [PRI1], etc.
var slotHeaderRe = regexp.MustCompile(`^\[([A-Z]+)(\d+)\]$`)

// parseStoredImages parses --dms-list-stored-images output.
//
// Example:
//
//	[/dev/cdc-wdm0] Device has 4 stored images:
//		[MODEM0]:
//			Unique ID: '?_?'
//			Build ID: '02.39.00.00_GENERIC'
//			Storage index: '0'
//			Failure count: '0'
//		>>>>>>>>>> [CURRENT] <<<<<<<<<<
//		[PRI0]:
//			Unique ID: '9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.072_000'
//			Build ID: '02.39.00.00_GENERIC'
//			Storage index: '0'
//			Failure count: '0'
//		>>>>>>>>>> [CURRENT] <<<<<<<<<<
//		[MODEM1]:
//			Unique ID: '?_?'
//			Build ID: ''
//			Storage index: '1'
//			Failure count: '0'
//		[PRI1]:
//			Unique ID: ''
//			Build ID: ''
//			Storage index: '1'
//			Failure count: '0'
func parseStoredImages(output string) []StoredImage {
	var images []StoredImage
	var cur *StoredImage

	flush := func() {
		if cur != nil {
			images = append(images, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check for slot header like [MODEM0], [PRI1].
		// The trimmed line may have a trailing colon, e.g. "[MODEM0]:" — strip it.
		header := strings.TrimSuffix(trimmed, ":")
		if m := slotHeaderRe.FindStringSubmatch(header); m != nil {
			flush()
			slot, _ := strconv.Atoi(m[2])
			cur = &StoredImage{
				Type: strings.ToLower(m[1]),
				Slot: slot,
			}
			continue
		}

		// Check for current marker.
		if strings.Contains(trimmed, "[CURRENT]") {
			if cur != nil {
				cur.Current = true
			} else if len(images) > 0 {
				// Marker on a line after the slot's KV pairs but before next header.
				images[len(images)-1].Current = true
			}
			continue
		}

		// Parse key-value pairs within a slot.
		if cur == nil {
			continue
		}
		key, val := parseQMIKV(trimmed)
		if key == "" {
			continue
		}
		switch key {
		case "Unique ID":
			cur.UniqueID = val
		case "Build ID":
			cur.BuildID = val
		case "Storage index":
			cur.StorageIndex, _ = strconv.Atoi(val)
		case "Failure count":
			cur.FailureCount, _ = strconv.Atoi(val)
		}
	}
	flush()
	return images
}

// parseFirmwarePreference parses --dms-get-firmware-preference output.
//
// Example:
//
//	[/dev/cdc-wdm0] Firmware preference successfully retrieved:
//		[0]:
//			Image type: 'modem'
//			Unique ID: '?_?'
//			Build ID: '02.39.00.00_GENERIC'
//		[1]:
//			Image type: 'pri'
//			Unique ID: '9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.072_000'
//			Build ID: '02.39.00.00_GENERIC'
func parseFirmwarePreference(output string) []FirmwarePreferenceImage {
	var images []FirmwarePreferenceImage
	var cur *FirmwarePreferenceImage

	flush := func() {
		if cur != nil {
			images = append(images, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Array-style index headers: [0]:, [1]:, etc.
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]:") {
			flush()
			cur = &FirmwarePreferenceImage{}
			continue
		}

		if cur == nil {
			continue
		}
		key, val := parseQMIKV(trimmed)
		if key == "" {
			continue
		}
		switch key {
		case "Image type":
			cur.Type = val
		case "Unique ID":
			cur.UniqueID = val
		case "Build ID":
			cur.BuildID = val
		}
	}
	flush()
	return images
}

// ParsePRIBuildID splits a stored-image Build ID like "02.39.00.00_GENERIC"
// into firmware version and carrier components. It uses the last underscore
// as the delimiter, since firmware versions contain dots but not underscores.
// Returns ("", "") if the input is empty or has no underscore.
func ParsePRIBuildID(buildID string) (fwVersion, carrier string) {
	if buildID == "" {
		return "", ""
	}
	idx := strings.LastIndex(buildID, "_")
	if idx < 0 {
		return buildID, ""
	}
	return buildID[:idx], buildID[idx+1:]
}

// --- Helpers ---

// parseQMIKV parses a qmicli key-value line like "RSSI: '-55 dBm'" into ("RSSI", "-55 dBm").
// Returns ("", "") if the line doesn't match the pattern.
func parseQMIKV(line string) (string, string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", ""
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])

	// Strip surrounding single quotes
	val = strings.Trim(val, "'")
	return key, val
}

// cellLocationKey maps qmicli verbose key names to short keys
// that match our existing AT command cell info format.
func cellLocationKey(key string) string {
	switch key {
	case "Physical Cell ID":
		return "PCI"
	case "RSRQ":
		return "RSRQ"
	case "RSRP":
		return "RSRP"
	case "RSSI":
		return "RSSI"
	case "Cell Selection RX Level":
		return "RxLev"
	case "EARFCN":
		return "EARFCN"
	case "Cell ID":
		return "CID"
	case "Tracking Area Code":
		return "TAC"
	default:
		return key
	}
}

// appendDomain builds a domain string like "cs-ps" from individual components.
func appendDomain(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "-" + add
}
