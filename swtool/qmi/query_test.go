package qmi

import (
	"testing"
)

func TestParseSignalInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		lte    *LTESignal
		wcdma  *WCDMASignal
		gsm    *GSMSignal
	}{
		{
			name: "lte only",
			input: `[/dev/cdc-wdm0] Successfully got signal info
LTE:
	RSSI: '-55 dBm'
	RSRQ: '-11 dB'
	RSRP: '-89 dBm'
	SNR: '7.8 dB'
`,
			lte: &LTESignal{RSSI: "-55 dBm", RSRQ: "-11 dB", RSRP: "-89 dBm", SNR: "7.8 dB"},
		},
		{
			name: "lte and wcdma",
			input: `[/dev/cdc-wdm0] Successfully got signal info
LTE:
	RSSI: '-62 dBm'
	RSRQ: '-9 dB'
	RSRP: '-95 dBm'
	SNR: '12.4 dB'
WCDMA:
	RSSI: '-70 dBm'
	ECIO: '-4 dB'
`,
			lte:   &LTESignal{RSSI: "-62 dBm", RSRQ: "-9 dB", RSRP: "-95 dBm", SNR: "12.4 dB"},
			wcdma: &WCDMASignal{RSSI: "-70 dBm", ECIO: "-4 dB"},
		},
		{
			name: "gsm only",
			input: `[/dev/cdc-wdm0] Successfully got signal info
GSM:
	RSSI: '-80 dBm'
`,
			gsm: &GSMSignal{RSSI: "-80 dBm"},
		},
		{
			name:  "empty output",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseSignalInfo(tt.input)
			checkLTESignal(t, got.LTE, tt.lte)
			checkWCDMASignal(t, got.WCDMA, tt.wcdma)
			checkGSMSignal(t, got.GSM, tt.gsm)
		})
	}
}

func TestParseServingSystem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  ServingSystem
	}{
		{
			name: "registered lte",
			input: `[/dev/cdc-wdm0] Successfully got serving system:
	Registration state: 'registered'
	CS: 'attached'
	PS: 'attached'
	Selected network: '3gpp'
	Radio interfaces: '1'
		[0]: 'lte'
	Roaming status: 'off'
	Data service capability: 'lte'
	Current PLMN:
		MCC: '310'
		MNC: '260'
		Description: 'T-Mobile'
`,
			want: ServingSystem{
				Registered: true,
				MCC:        "310",
				MNC:        "260",
				RAT:        "lte",
				Roaming:    false,
				Domain:     "cs-ps",
			},
		},
		{
			name: "not registered",
			input: `[/dev/cdc-wdm0] Successfully got serving system:
	Registration state: 'not-registered'
	CS: 'detached'
	PS: 'detached'
	Selected network: '3gpp'
	Radio interfaces: '0'
	Roaming status: 'off'
`,
			want: ServingSystem{
				Registered: false,
				Roaming:    false,
			},
		},
		{
			name: "roaming umts",
			input: `[/dev/cdc-wdm0] Successfully got serving system:
	Registration state: 'registered'
	CS: 'attached'
	PS: 'attached'
	Selected network: '3gpp'
	Radio interfaces: '1'
		[0]: 'umts'
	Roaming status: 'on'
	Current PLMN:
		MCC: '234'
		MNC: '15'
		Description: 'Vodafone UK'
`,
			want: ServingSystem{
				Registered: true,
				MCC:        "234",
				MNC:        "15",
				RAT:        "umts",
				Roaming:    true,
				Domain:     "cs-ps",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseServingSystem(tt.input)
			if got.Registered != tt.want.Registered {
				t.Errorf("Registered = %v, want %v", got.Registered, tt.want.Registered)
			}
			if got.MCC != tt.want.MCC {
				t.Errorf("MCC = %q, want %q", got.MCC, tt.want.MCC)
			}
			if got.MNC != tt.want.MNC {
				t.Errorf("MNC = %q, want %q", got.MNC, tt.want.MNC)
			}
			if got.RAT != tt.want.RAT {
				t.Errorf("RAT = %q, want %q", got.RAT, tt.want.RAT)
			}
			if got.Roaming != tt.want.Roaming {
				t.Errorf("Roaming = %v, want %v", got.Roaming, tt.want.Roaming)
			}
			if got.Domain != tt.want.Domain {
				t.Errorf("Domain = %q, want %q", got.Domain, tt.want.Domain)
			}
		})
	}
}

func TestParseCellLocationInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantIntra int
		wantInter int
		checkCell func(t *testing.T, info *CellLocationInfo)
	}{
		{
			name: "intra and inter freq cells",
			input: `[/dev/cdc-wdm0] Successfully got cell location info
LTE Info - Intra-Frequency:
	Cell [0]:
		Physical Cell ID: '123'
		RSRQ: '-11.0 dB'
		RSRP: '-89.0 dBm'
		RSSI: '-55.0 dBm'
	Cell [1]:
		Physical Cell ID: '456'
		RSRQ: '-15.0 dB'
		RSRP: '-102.0 dBm'
		RSSI: '-70.0 dBm'
LTE Info - Inter-Frequency:
	Cell [0]:
		Physical Cell ID: '789'
		EARFCN: '2100'
		RSRQ: '-13.0 dB'
		RSRP: '-95.0 dBm'
		RSSI: '-62.0 dBm'
`,
			wantIntra: 2,
			wantInter: 1,
			checkCell: func(t *testing.T, info *CellLocationInfo) {
				t.Helper()
				if info.LTEIntra[0]["PCI"] != "123" {
					t.Errorf("intra[0] PCI = %q, want %q", info.LTEIntra[0]["PCI"], "123")
				}
				if info.LTEIntra[1]["RSRP"] != "-102.0 dBm" {
					t.Errorf("intra[1] RSRP = %q, want %q", info.LTEIntra[1]["RSRP"], "-102.0 dBm")
				}
				if info.LTEInter[0]["PCI"] != "789" {
					t.Errorf("inter[0] PCI = %q, want %q", info.LTEInter[0]["PCI"], "789")
				}
				if info.LTEInter[0]["EARFCN"] != "2100" {
					t.Errorf("inter[0] EARFCN = %q, want %q", info.LTEInter[0]["EARFCN"], "2100")
				}
			},
		},
		{
			name:      "empty output",
			input:     "",
			wantIntra: 0,
			wantInter: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseCellLocationInfo(tt.input)
			if len(got.LTEIntra) != tt.wantIntra {
				t.Errorf("LTEIntra count = %d, want %d", len(got.LTEIntra), tt.wantIntra)
			}
			if len(got.LTEInter) != tt.wantInter {
				t.Errorf("LTEInter count = %d, want %d", len(got.LTEInter), tt.wantInter)
			}
			if tt.checkCell != nil {
				tt.checkCell(t, got)
			}
		})
	}
}

func TestParseSystemInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  SystemInfo
	}{
		{
			name: "lte available",
			input: `[/dev/cdc-wdm0] Successfully got system info:
CDMA 1x:
	Service status: 'no-service'
CDMA 1xEV-DO:
	Service status: 'no-service'
GSM:
	Service status: 'no-service'
WCDMA:
	Service status: 'no-service'
LTE:
	Service status: 'available'
	True service status: 'available'
	Preferred data path: 'yes'
	Domain: 'cs-ps'
	Roaming status: 'off'
	MCC: '310'
	MNC: '260'
	Tracking area code: '12345'
	Cell ID: '67890'
`,
			want: SystemInfo{
				ServiceStatus: "available",
				Domain:        "cs-ps",
				Roaming:       "off",
				MCC:           "310",
				MNC:           "260",
				TAC:           "12345",
				CellID:        "67890",
			},
		},
		{
			name: "no lte section",
			input: `[/dev/cdc-wdm0] Successfully got system info:
GSM:
	Service status: 'available'
	Domain: 'cs'
`,
			want: SystemInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseSystemInfo(tt.input)
			if got.ServiceStatus != tt.want.ServiceStatus {
				t.Errorf("ServiceStatus = %q, want %q", got.ServiceStatus, tt.want.ServiceStatus)
			}
			if got.Domain != tt.want.Domain {
				t.Errorf("Domain = %q, want %q", got.Domain, tt.want.Domain)
			}
			if got.Roaming != tt.want.Roaming {
				t.Errorf("Roaming = %q, want %q", got.Roaming, tt.want.Roaming)
			}
			if got.MCC != tt.want.MCC {
				t.Errorf("MCC = %q, want %q", got.MCC, tt.want.MCC)
			}
			if got.MNC != tt.want.MNC {
				t.Errorf("MNC = %q, want %q", got.MNC, tt.want.MNC)
			}
			if got.TAC != tt.want.TAC {
				t.Errorf("TAC = %q, want %q", got.TAC, tt.want.TAC)
			}
			if got.CellID != tt.want.CellID {
				t.Errorf("CellID = %q, want %q", got.CellID, tt.want.CellID)
			}
		})
	}
}

func TestParseQMIKV(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		wantKey string
		wantVal string
	}{
		{"RSSI: '-55 dBm'", "RSSI", "-55 dBm"},
		{"Registration state: 'registered'", "Registration state", "registered"},
		{"MCC: '310'", "MCC", "310"},
		{"no colon here", "", ""},
		{"", "", ""},
		{"Key: value without quotes", "Key", "value without quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			key, val := parseQMIKV(tt.input)
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
			if val != tt.wantVal {
				t.Errorf("val = %q, want %q", val, tt.wantVal)
			}
		})
	}
}

func TestParseStoredImages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []StoredImage
	}{
		{
			name: "normal two slots with current",
			input: `[/dev/cdc-wdm0] Device has 4 stored images:
	[MODEM0]:
		Unique ID: '?_?'
		Build ID: '02.39.00.00_GENERIC'
		Storage index: '0'
		Failure count: '0'
	>>>>>>>>>> [CURRENT] <<<<<<<<<<
	[PRI0]:
		Unique ID: '9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.072_000'
		Build ID: '02.39.00.00_GENERIC'
		Storage index: '0'
		Failure count: '0'
	>>>>>>>>>> [CURRENT] <<<<<<<<<<
	[MODEM1]:
		Unique ID: '?_?'
		Build ID: ''
		Storage index: '1'
		Failure count: '0'
	[PRI1]:
		Unique ID: ''
		Build ID: ''
		Storage index: '1'
		Failure count: '0'
`,
			want: []StoredImage{
				{Type: "modem", Slot: 0, UniqueID: "?_?", BuildID: "02.39.00.00_GENERIC", StorageIndex: 0, FailureCount: 0, Current: true},
				{Type: "pri", Slot: 0, UniqueID: "9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.072_000", BuildID: "02.39.00.00_GENERIC", StorageIndex: 0, FailureCount: 0, Current: true},
				{Type: "modem", Slot: 1, UniqueID: "?_?", BuildID: "", StorageIndex: 1, FailureCount: 0, Current: false},
				{Type: "pri", Slot: 1, UniqueID: "", BuildID: "", StorageIndex: 1, FailureCount: 0, Current: false},
			},
		},
		{
			name: "current marker on pri only",
			input: `[/dev/cdc-wdm0] Device has 2 stored images:
	[MODEM0]:
		Unique ID: '?_?'
		Build ID: '02.39.00.00_GENERIC'
		Storage index: '0'
		Failure count: '1'
	[PRI0]:
		Unique ID: 'some_config'
		Build ID: '02.39.00.00_GENERIC'
		Storage index: '0'
		Failure count: '0'
	>>>>>>>>>> [CURRENT] <<<<<<<<<<
`,
			want: []StoredImage{
				{Type: "modem", Slot: 0, UniqueID: "?_?", BuildID: "02.39.00.00_GENERIC", StorageIndex: 0, FailureCount: 1, Current: false},
				{Type: "pri", Slot: 0, UniqueID: "some_config", BuildID: "02.39.00.00_GENERIC", StorageIndex: 0, FailureCount: 0, Current: true},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseStoredImages(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d images, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("image[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseFirmwarePreference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []FirmwarePreferenceImage
	}{
		{
			name: "normal modem and pri",
			input: `[/dev/cdc-wdm0] Firmware preference successfully retrieved:
	[0]:
		Image type: 'modem'
		Unique ID: '?_?'
		Build ID: '02.39.00.00_GENERIC'
	[1]:
		Image type: 'pri'
		Unique ID: '9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.072_000'
		Build ID: '02.39.00.00_GENERIC'
`,
			want: []FirmwarePreferenceImage{
				{Type: "modem", UniqueID: "?_?", BuildID: "02.39.00.00_GENERIC"},
				{Type: "pri", UniqueID: "9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.072_000", BuildID: "02.39.00.00_GENERIC"},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseFirmwarePreference(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d images, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("image[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParsePRIBuildID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input       string
		wantVersion string
		wantCarrier string
	}{
		{"02.39.00.00_GENERIC", "02.39.00.00", "GENERIC"},
		{"SWI9X50C_01.14.02.00_ATT", "SWI9X50C_01.14.02.00", "ATT"},
		{"?", "?", ""},
		{"", "", ""},
		{"nounderscores", "nounderscores", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			gotVer, gotCarrier := ParsePRIBuildID(tt.input)
			if gotVer != tt.wantVersion {
				t.Errorf("version = %q, want %q", gotVer, tt.wantVersion)
			}
			if gotCarrier != tt.wantCarrier {
				t.Errorf("carrier = %q, want %q", gotCarrier, tt.wantCarrier)
			}
		})
	}
}

// --- test helpers ---

func checkLTESignal(t *testing.T, got, want *LTESignal) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("LTE = %+v, want nil", got)
		}
		return
	}
	if got == nil {
		t.Fatal("LTE = nil, want non-nil")
	}
	if got.RSSI != want.RSSI {
		t.Errorf("LTE.RSSI = %q, want %q", got.RSSI, want.RSSI)
	}
	if got.RSRP != want.RSRP {
		t.Errorf("LTE.RSRP = %q, want %q", got.RSRP, want.RSRP)
	}
	if got.RSRQ != want.RSRQ {
		t.Errorf("LTE.RSRQ = %q, want %q", got.RSRQ, want.RSRQ)
	}
	if got.SNR != want.SNR {
		t.Errorf("LTE.SNR = %q, want %q", got.SNR, want.SNR)
	}
}

func checkWCDMASignal(t *testing.T, got, want *WCDMASignal) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("WCDMA = %+v, want nil", got)
		}
		return
	}
	if got == nil {
		t.Fatal("WCDMA = nil, want non-nil")
	}
	if got.RSSI != want.RSSI {
		t.Errorf("WCDMA.RSSI = %q, want %q", got.RSSI, want.RSSI)
	}
	if got.ECIO != want.ECIO {
		t.Errorf("WCDMA.ECIO = %q, want %q", got.ECIO, want.ECIO)
	}
}

func checkGSMSignal(t *testing.T, got, want *GSMSignal) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("GSM = %+v, want nil", got)
		}
		return
	}
	if got == nil {
		t.Fatal("GSM = nil, want non-nil")
	}
	if got.RSSI != want.RSSI {
		t.Errorf("GSM.RSSI = %q, want %q", got.RSSI, want.RSSI)
	}
}
