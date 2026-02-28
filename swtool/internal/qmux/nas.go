package qmux

import (
	"encoding/binary"
	"fmt"
	"math"
)

// NAS message IDs.
const (
	nasMsgGetServingSystem   uint16 = 0x0024
	nasMsgGetCellLocationInfo uint16 = 0x0043
	nasMsgGetSystemInfo      uint16 = 0x004D
	nasMsgGetSignalInfo      uint16 = 0x004F
)

// SignalInfo holds per-technology signal metrics.
type SignalInfo struct {
	LTE   *LTESignal
	WCDMA *WCDMASignal
	GSM   *GSMSignal
}

// LTESignal holds LTE signal metrics.
type LTESignal struct {
	RSSI int16   // dBm
	RSRQ int8    // dB
	RSRP int16   // dBm
	SNR  float32 // dB (10ths on wire, so 78 = 7.8 dB)
}

// WCDMASignal holds WCDMA signal metrics.
type WCDMASignal struct {
	RSSI int8 // dBm
	ECIO int16 // dB (10ths on wire)
}

// GSMSignal holds GSM signal metrics.
type GSMSignal struct {
	RSSI int8 // dBm
}

// GetSignalInfo queries NAS signal info.
//
// Response TLVs:
//
//	0x10: CDMA (skip)
//	0x11: HDR (skip)
//	0x12: GSM — RSSI(1, int8)
//	0x13: WCDMA — RSSI(1, int8) + ECIO(2, int16, 10ths dB)
//	0x14: LTE — RSSI(2, int16) + RSRQ(1, int8) + RSRP(2, int16) + SNR(2, int16, 10ths dB)
func GetSignalInfo(c *Conn) (*SignalInfo, error) {
	resp, err := c.Send(ServiceNAS, nasMsgGetSignalInfo)
	if err != nil {
		return nil, fmt.Errorf("getting signal info: %w", err)
	}
	if err := resp.Result(); err != nil {
		return nil, fmt.Errorf("getting signal info: %w", err)
	}

	info := &SignalInfo{}

	// LTE (TLV 0x14): RSSI(2) + RSRQ(1) + RSRP(2) + SNR(2) = 7 bytes
	if tlv := resp.FindTLV(0x14); tlv != nil && len(tlv.Value) >= 7 {
		info.LTE = &LTESignal{
			RSSI: int16(binary.LittleEndian.Uint16(tlv.Value[0:2])),
			RSRQ: int8(tlv.Value[2]),
			RSRP: int16(binary.LittleEndian.Uint16(tlv.Value[3:5])),
			SNR:  float32(int16(binary.LittleEndian.Uint16(tlv.Value[5:7]))) / 10.0,
		}
	}

	// WCDMA (TLV 0x13): RSSI(1) + ECIO(2) = 3 bytes
	if tlv := resp.FindTLV(0x13); tlv != nil && len(tlv.Value) >= 3 {
		info.WCDMA = &WCDMASignal{
			RSSI: int8(tlv.Value[0]),
			ECIO: int16(binary.LittleEndian.Uint16(tlv.Value[1:3])),
		}
	}

	// GSM (TLV 0x12): RSSI(1) = 1 byte
	if tlv := resp.FindTLV(0x12); tlv != nil && len(tlv.Value) >= 1 {
		info.GSM = &GSMSignal{
			RSSI: int8(tlv.Value[0]),
		}
	}

	return info, nil
}

// ServingSystem holds network registration info.
type ServingSystem struct {
	Registered bool
	CSAttached bool
	PSAttached bool
	RAT        string // "lte", "umts", "gsm", etc.
	Roaming    bool
	MCC        uint16
	MNC        uint16
}

// ratName maps QMI radio interface enum values to names.
func ratName(r uint8) string {
	switch r {
	case 0x00:
		return "none"
	case 0x01:
		return "cdma"
	case 0x02:
		return "umts"
	case 0x03:
		return "gsm"
	case 0x04:
		return "lte"
	case 0x05:
		return "td-scdma"
	default:
		return fmt.Sprintf("unknown-%d", r)
	}
}

// GetServingSystem queries NAS serving system.
//
// Response TLV 0x01 (mandatory):
//
//	registrationState(1) + csAttach(1) + psAttach(1) +
//	selectedNetwork(1) + radioInterfaceCount(1) + radioInterfaces(N*1)
//
// TLV 0x10: Roaming indicator (1 byte: 0=off, 1=on)
// TLV 0x12: Current PLMN — MCC(2) + MNC(2) + descLen(1) + desc(N)
func GetServingSystem(c *Conn) (*ServingSystem, error) {
	resp, err := c.Send(ServiceNAS, nasMsgGetServingSystem)
	if err != nil {
		return nil, fmt.Errorf("getting serving system: %w", err)
	}
	if err := resp.Result(); err != nil {
		return nil, fmt.Errorf("getting serving system: %w", err)
	}

	sys := &ServingSystem{}

	// Mandatory TLV 0x01.
	tlv := resp.FindTLV(0x01)
	if tlv == nil || len(tlv.Value) < 5 {
		return nil, fmt.Errorf("getting serving system: missing or short serving system TLV")
	}
	sys.Registered = tlv.Value[0] == 0x01 // 1 = registered
	sys.CSAttached = tlv.Value[1] == 0x01
	sys.PSAttached = tlv.Value[2] == 0x01
	// tlv.Value[3] = selectedNetwork (skip)
	riCount := int(tlv.Value[4])
	if riCount > 0 && len(tlv.Value) >= 6 {
		sys.RAT = ratName(tlv.Value[5])
	}

	// Roaming indicator (TLV 0x10).
	if tlv := resp.FindTLV(0x10); tlv != nil && len(tlv.Value) >= 1 {
		sys.Roaming = tlv.Value[0] != 0x00
	}

	// Current PLMN (TLV 0x12): MCC(2) + MNC(2) + descLen(1) + desc(N)
	if tlv := resp.FindTLV(0x12); tlv != nil && len(tlv.Value) >= 4 {
		sys.MCC = binary.LittleEndian.Uint16(tlv.Value[0:2])
		sys.MNC = binary.LittleEndian.Uint16(tlv.Value[2:4])
	}

	return sys, nil
}

// SystemInfo holds LTE system info (TAC, Cell ID, service status).
type SystemInfo struct {
	ServiceStatus uint8  // 0=no-service, 1=limited, 2=available
	Domain        uint8  // 0=none, 1=cs, 2=ps, 3=cs-ps
	Roaming       bool
	MCC           uint16
	MNC           uint16
	TAC           uint16
	CellID        uint32
}

// ServiceStatusString returns a human-readable service status name.
func ServiceStatusString(s uint8) string {
	switch s {
	case 0:
		return "no-service"
	case 1:
		return "limited"
	case 2:
		return "available"
	default:
		return fmt.Sprintf("unknown-%d", s)
	}
}

// DomainString returns a human-readable domain name.
func DomainString(d uint8) string {
	switch d {
	case 0:
		return "none"
	case 1:
		return "cs"
	case 2:
		return "ps"
	case 3:
		return "cs-ps"
	default:
		return fmt.Sprintf("unknown-%d", d)
	}
}

// GetSystemInfo queries NAS system info.
//
// This response has many technology-specific TLVs. We parse:
//
//	TLV 0x14: LTE Service Status — svcStatus(1) + trueStatus(1) + prefData(1)
//	TLV 0x1D: LTE System Info — domain(1) + svcCapability(1) + roaming(1) +
//	           forbidden(1) + lacValid(1) + lac(2) + cidValid(1) + cid(4) +
//	           regReject(1) + plmnValid(1) + mcc(2) + mnc(2) +
//	           tacValid(1) + tac(2)
func GetSystemInfo(c *Conn) (*SystemInfo, error) {
	resp, err := c.Send(ServiceNAS, nasMsgGetSystemInfo)
	if err != nil {
		return nil, fmt.Errorf("getting system info: %w", err)
	}
	if err := resp.Result(); err != nil {
		return nil, fmt.Errorf("getting system info: %w", err)
	}

	info := &SystemInfo{}

	// LTE Service Status (TLV 0x14): svcStatus(1) + trueStatus(1) + prefData(1)
	if tlv := resp.FindTLV(0x14); tlv != nil && len(tlv.Value) >= 1 {
		info.ServiceStatus = tlv.Value[0]
	}

	// LTE System Info (TLV 0x1D): complex nested structure.
	if tlv := resp.FindTLV(0x1D); tlv != nil {
		parseLTESystemInfoTLV(tlv.Value, info)
	}

	return info, nil
}

func parseLTESystemInfoTLV(data []byte, info *SystemInfo) {
	// Minimum bytes: domain(1) + svcCap(1) + roaming(1) + forbidden(1) +
	//   lacValid(1) + lac(2) + cidValid(1) + cid(4) + regReject(1) +
	//   plmnValid(1) + mcc(2) + mnc(2) + tacValid(1) + tac(2)
	// = 20 bytes minimum
	if len(data) < 20 {
		return
	}

	off := 0
	info.Domain = data[off]
	off++ // domain
	off++ // svcCapability (skip)
	info.Roaming = data[off] != 0x00
	off++ // roaming
	off++ // forbidden (skip)

	// LAC
	lacValid := data[off]
	off++
	off += 2 // lac (skip regardless)
	_ = lacValid

	// Cell ID
	cidValid := data[off]
	off++
	if cidValid != 0 {
		info.CellID = binary.LittleEndian.Uint32(data[off : off+4])
	}
	off += 4

	off++ // regReject (skip)

	// PLMN
	plmnValid := data[off]
	off++
	if plmnValid != 0 && off+4 <= len(data) {
		info.MCC = binary.LittleEndian.Uint16(data[off : off+2])
		info.MNC = binary.LittleEndian.Uint16(data[off+2 : off+4])
	}
	off += 4

	// TAC
	if off+3 <= len(data) {
		tacValid := data[off]
		off++
		if tacValid != 0 {
			info.TAC = binary.LittleEndian.Uint16(data[off : off+2])
		}
	}
}

// CellLocationInfo holds LTE serving and neighbor cell info.
type CellLocationInfo struct {
	LTEIntra []LTECell // Intra-frequency cells
	LTEInter []LTECell // Inter-frequency cells
}

// LTECell holds info about a single LTE cell.
type LTECell struct {
	PCI    uint16 // Physical Cell ID
	RSRQ   int16  // dB (10ths)
	RSRP   int16  // dBm (10ths)
	RSSI   int16  // dBm (10ths)
	EARFCN uint16 // only for inter-frequency
}

// GetCellLocationInfo queries NAS cell location info.
//
// Response TLVs:
//
//	TLV 0x13: LTE Intra-Frequency —
//	  ueInIdle(1) + plmn(3) + tac(2) + globalCellID(4) + earfcn(2) +
//	  servingCellID(2) + cellReselPriority(1) + sNonIntraSearch(1) +
//	  threshServing(1) + sIntraSearch(1) + cellCount(1) +
//	  per cell: pci(2) + rsrq(2) + rsrp(2) + rssi(2) + srxlev(2)
//
//	TLV 0x14: LTE Inter-Frequency —
//	  ueInIdle(1) + freqCount(1) +
//	  per frequency: earfcn(2) + threshXLow(1) + threshXHigh(1) +
//	    cellReselPriority(1) + cellCount(1) +
//	    per cell: pci(2) + rsrq(2) + rsrp(2) + rssi(2) + srxlev(2)
func GetCellLocationInfo(c *Conn) (*CellLocationInfo, error) {
	resp, err := c.Send(ServiceNAS, nasMsgGetCellLocationInfo)
	if err != nil {
		return nil, fmt.Errorf("getting cell location info: %w", err)
	}
	if err := resp.Result(); err != nil {
		return nil, fmt.Errorf("getting cell location info: %w", err)
	}

	info := &CellLocationInfo{}

	// LTE Intra-Frequency (TLV 0x13).
	if tlv := resp.FindTLV(0x13); tlv != nil {
		info.LTEIntra = parseLTEIntraFreqTLV(tlv.Value)
	}

	// LTE Inter-Frequency (TLV 0x14).
	if tlv := resp.FindTLV(0x14); tlv != nil {
		info.LTEInter = parseLTEInterFreqTLV(tlv.Value)
	}

	return info, nil
}

func parseLTEIntraFreqTLV(data []byte) []LTECell {
	// Header: ueInIdle(1) + plmn(3) + tac(2) + globalCellID(4) + earfcn(2) +
	//         servingCellID(2) + cellReselPriority(1) + sNonIntraSearch(1) +
	//         threshServing(1) + sIntraSearch(1) + cellCount(1)
	// = 19 bytes header
	if len(data) < 19 {
		return nil
	}

	off := 18 // skip to cellCount
	cellCount := int(data[off])
	off++

	var cells []LTECell
	// Each cell: pci(2) + rsrq(2) + rsrp(2) + rssi(2) + srxlev(2) = 10 bytes
	for range cellCount {
		if off+10 > len(data) {
			break
		}
		cell := LTECell{
			PCI:  binary.LittleEndian.Uint16(data[off : off+2]),
			RSRQ: int16(binary.LittleEndian.Uint16(data[off+2 : off+4])),
			RSRP: int16(binary.LittleEndian.Uint16(data[off+4 : off+6])),
			RSSI: int16(binary.LittleEndian.Uint16(data[off+6 : off+8])),
		}
		cells = append(cells, cell)
		off += 10
	}

	return cells
}

func parseLTEInterFreqTLV(data []byte) []LTECell {
	// Header: ueInIdle(1) + freqCount(1)
	if len(data) < 2 {
		return nil
	}

	off := 1 // skip ueInIdle
	freqCount := int(data[off])
	off++

	var cells []LTECell
	for range freqCount {
		// Per-frequency header: earfcn(2) + threshXLow(1) + threshXHigh(1) +
		//                       cellReselPriority(1) + cellCount(1)
		if off+6 > len(data) {
			break
		}
		earfcn := binary.LittleEndian.Uint16(data[off : off+2])
		off += 2
		off += 3 // skip threshXLow, threshXHigh, cellReselPriority
		cellCount := int(data[off])
		off++

		for range cellCount {
			if off+10 > len(data) {
				break
			}
			cell := LTECell{
				PCI:    binary.LittleEndian.Uint16(data[off : off+2]),
				RSRQ:   int16(binary.LittleEndian.Uint16(data[off+2 : off+4])),
				RSRP:   int16(binary.LittleEndian.Uint16(data[off+4 : off+6])),
				RSSI:   int16(binary.LittleEndian.Uint16(data[off+6 : off+8])),
				EARFCN: earfcn,
			}
			cells = append(cells, cell)
			off += 10
		}
	}

	return cells
}

// FormatSignalDBM formats a signal value as a dBm string.
func FormatSignalDBM(val int16) string {
	return fmt.Sprintf("%d dBm", val)
}

// FormatSignalDB formats a signal value as a dB string.
func FormatSignalDB(val int8) string {
	return fmt.Sprintf("%d dB", val)
}

// FormatSNR formats a SNR value as a dB string with one decimal.
func FormatSNR(val float32) string {
	// Round to 1 decimal place.
	rounded := math.Round(float64(val)*10) / 10
	return fmt.Sprintf("%.1f dB", rounded)
}

// FormatSignalTenthsDBM formats a value stored in 10ths of dBm.
func FormatSignalTenthsDBM(val int16) string {
	whole := val / 10
	frac := val % 10
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%d dBm", whole, frac)
}

// FormatSignalTenthsDB formats a value stored in 10ths of dB.
func FormatSignalTenthsDB(val int16) string {
	whole := val / 10
	frac := val % 10
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%d dB", whole, frac)
}
