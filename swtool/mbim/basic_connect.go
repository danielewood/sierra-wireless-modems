package mbim

import "fmt"

// DeviceCaps holds the response from MBIM_CID_DEVICE_CAPS.
type DeviceCaps struct {
	DeviceType    uint32
	CellularClass uint32
	DataClass     uint32
	DeviceID      string // IMEI
	FirmwareInfo  string
	HardwareInfo  string
}

// SubscriberReadyStatus holds the response from MBIM_CID_SUBSCRIBER_READY_STATUS.
type SubscriberReadyStatus struct {
	ReadyState   uint32
	SubscriberID string // IMSI
	SimICCID     string
}

// ReadyState enum values.
const (
	ReadyStateNotInitialized uint32 = 0
	ReadyStateInitialized    uint32 = 1
	ReadyStateSimNotInserted uint32 = 2
	ReadyStateBadSim         uint32 = 3
	ReadyStateFailure        uint32 = 4
	ReadyStateNotActivated   uint32 = 5
	ReadyStateDeviceLocked   uint32 = 6
)

// ReadyStateName returns a human-readable name for a ReadyState value.
func ReadyStateName(s uint32) string {
	switch s {
	case ReadyStateNotInitialized:
		return "not initialized"
	case ReadyStateInitialized:
		return "ready"
	case ReadyStateSimNotInserted:
		return "SIM not inserted"
	case ReadyStateBadSim:
		return "bad SIM"
	case ReadyStateFailure:
		return "failure"
	case ReadyStateNotActivated:
		return "not activated"
	case ReadyStateDeviceLocked:
		return "device locked"
	default:
		return fmt.Sprintf("unknown (%d)", s)
	}
}

// RegisterState holds the response from MBIM_CID_REGISTER_STATE.
type RegisterState struct {
	NwError            uint32
	State              uint32
	Mode               uint32
	AvailableDataClass uint32
	CurrentCellClass   uint32
	ProviderID         string
	ProviderName       string
	RoamingText        string
}

// Register state enum values.
const (
	RegStateUnknown      uint32 = 0
	RegStateDeregistered uint32 = 1
	RegStateSearching    uint32 = 2
	RegStateHome         uint32 = 3
	RegStateRoaming      uint32 = 4
	RegStatePartner      uint32 = 5
	RegStateDenied       uint32 = 6
)

// RegStateName returns a human-readable name for a RegisterState value.
func RegStateName(s uint32) string {
	switch s {
	case RegStateUnknown:
		return "unknown"
	case RegStateDeregistered:
		return "deregistered"
	case RegStateSearching:
		return "searching"
	case RegStateHome:
		return "registered (home)"
	case RegStateRoaming:
		return "registered (roaming)"
	case RegStatePartner:
		return "registered (partner)"
	case RegStateDenied:
		return "denied"
	default:
		return fmt.Sprintf("unknown (%d)", s)
	}
}

// Data class flags.
const (
	DataClassGPRS  uint32 = 1 << 0
	DataClassEDGE  uint32 = 1 << 1
	DataClassUMTS  uint32 = 1 << 2
	DataClassHSDPA uint32 = 1 << 3
	DataClassHSUPA uint32 = 1 << 4
	DataClassLTE   uint32 = 1 << 5
)

// DataClassName returns the highest RAT from data class flags.
func DataClassName(dc uint32) string {
	if dc&DataClassLTE != 0 {
		return "LTE"
	}
	if dc&(DataClassHSDPA|DataClassHSUPA) != 0 {
		return "HSPA"
	}
	if dc&DataClassUMTS != 0 {
		return "UMTS"
	}
	if dc&DataClassEDGE != 0 {
		return "EDGE"
	}
	if dc&DataClassGPRS != 0 {
		return "GPRS"
	}
	return ""
}

// DataClassDescription returns a comma-separated list of all supported data classes.
func DataClassDescription(dc uint32) string {
	var classes []string
	if dc&DataClassGPRS != 0 {
		classes = append(classes, "GPRS")
	}
	if dc&DataClassEDGE != 0 {
		classes = append(classes, "EDGE")
	}
	if dc&DataClassUMTS != 0 {
		classes = append(classes, "UMTS")
	}
	if dc&DataClassHSDPA != 0 {
		classes = append(classes, "HSDPA")
	}
	if dc&DataClassHSUPA != 0 {
		classes = append(classes, "HSUPA")
	}
	if dc&DataClassLTE != 0 {
		classes = append(classes, "LTE")
	}
	if dc&(1<<6) != 0 { // custom
		classes = append(classes, "HSPA+")
	}
	if len(classes) == 0 {
		return "none"
	}
	return fmt.Sprintf("%s", joinSlice(classes, ", "))
}

func joinSlice(s []string, sep string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}

// SignalState holds the response from MBIM_CID_SIGNAL_STATE.
type SignalState struct {
	RSSI      uint32 // 0-31 (coded), 99 = unknown
	ErrorRate uint32 // 0-7 (coded), 99 = unknown
}

// RSSIdBm converts the coded RSSI value (0-31) to dBm.
// Returns 0 if the value is unknown (99).
func (s SignalState) RSSIdBm() int {
	if s.RSSI >= 99 {
		return 0
	}
	// 3GPP TS 27.007: 0 = -113 dBm, 1 = -111, ..., 31 = -51 dBm
	return int(s.RSSI)*2 - 113
}

// GetDeviceCaps queries MBIM_CID_DEVICE_CAPS.
func (c *Client) GetDeviceCaps() (*DeviceCaps, error) {
	buf, err := c.query(basicConnectUUID, cidDeviceCaps)
	if err != nil {
		return nil, fmt.Errorf("querying device caps: %w", err)
	}
	if len(buf) < 0x40 {
		return nil, fmt.Errorf("device caps response too short: %d bytes", len(buf))
	}
	return &DeviceCaps{
		DeviceType:    u32(buf, 0x00),
		CellularClass: u32(buf, 0x04),
		DataClass:     u32(buf, 0x10),
		DeviceID:      readUTF16String(buf, u32(buf, 0x28), u32(buf, 0x2C)),
		FirmwareInfo:  readUTF16String(buf, u32(buf, 0x30), u32(buf, 0x34)),
		HardwareInfo:  readUTF16String(buf, u32(buf, 0x38), u32(buf, 0x3C)),
	}, nil
}

// GetSubscriberReadyStatus queries MBIM_CID_SUBSCRIBER_READY_STATUS.
func (c *Client) GetSubscriberReadyStatus() (*SubscriberReadyStatus, error) {
	buf, err := c.query(basicConnectUUID, cidSubscriberReadyStatus)
	if err != nil {
		return nil, fmt.Errorf("querying subscriber status: %w", err)
	}
	if len(buf) < 0x1C {
		return nil, fmt.Errorf("subscriber status response too short: %d bytes", len(buf))
	}
	return &SubscriberReadyStatus{
		ReadyState:   u32(buf, 0x00),
		SubscriberID: readUTF16String(buf, u32(buf, 0x04), u32(buf, 0x08)),
		SimICCID:     readUTF16String(buf, u32(buf, 0x0C), u32(buf, 0x10)),
	}, nil
}

// GetRegisterState queries MBIM_CID_REGISTER_STATE.
func (c *Client) GetRegisterState() (*RegisterState, error) {
	buf, err := c.query(basicConnectUUID, cidRegisterState)
	if err != nil {
		return nil, fmt.Errorf("querying register state: %w", err)
	}
	if len(buf) < 0x30 {
		return nil, fmt.Errorf("register state response too short: %d bytes", len(buf))
	}
	return &RegisterState{
		NwError:            u32(buf, 0x00),
		State:              u32(buf, 0x04),
		Mode:               u32(buf, 0x08),
		AvailableDataClass: u32(buf, 0x0C),
		CurrentCellClass:   u32(buf, 0x10),
		ProviderID:         readUTF16String(buf, u32(buf, 0x14), u32(buf, 0x18)),
		ProviderName:       readUTF16String(buf, u32(buf, 0x1C), u32(buf, 0x20)),
		RoamingText:        readUTF16String(buf, u32(buf, 0x24), u32(buf, 0x28)),
	}, nil
}

// GetSignalState queries MBIM_CID_SIGNAL_STATE.
func (c *Client) GetSignalState() (*SignalState, error) {
	buf, err := c.query(basicConnectUUID, cidSignalState)
	if err != nil {
		return nil, fmt.Errorf("querying signal state: %w", err)
	}
	if len(buf) < 20 {
		return nil, fmt.Errorf("signal state response too short: %d bytes", len(buf))
	}
	return &SignalState{
		RSSI:      u32(buf, 0x00),
		ErrorRate: u32(buf, 0x04),
	}, nil
}
