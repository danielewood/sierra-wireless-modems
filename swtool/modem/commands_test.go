package modem

import (
	"testing"
)

func TestParseATI(t *testing.T) {
	t.Parallel()
	resp := "ATI\r\nManufacturer: Sierra Wireless, Incorporated\r\nModel: EM7455B\r\n" +
		"Revision: SWI9X30C_02.24.05.06 r7040 CARMD-EV-FRMWR2 2017/05/19 06:23:09\r\n" +
		"MEID: 35448008243466\r\nIMEI: 354480082434669\r\nIMEI SV: 12\r\n" +
		"FSN: LF103291240310\r\n+GCAP: +CGSM\r\n\r\nOK\r\n"
	id := parseATI(resp)

	checks := map[string]string{
		"Manufacturer": id.Manufacturer,
		"Model":        id.Model,
		"IMEI":         id.IMEI,
		"MEID":         id.MEID,
		"IMEISV":       id.IMEISV,
		"FSN":          id.FSN,
		"GCAP":         id.GCAP,
	}
	expected := map[string]string{
		"Manufacturer": "Sierra Wireless, Incorporated",
		"Model":        "EM7455B",
		"IMEI":         "354480082434669",
		"MEID":         "35448008243466",
		"IMEISV":       "12",
		"FSN":          "LF103291240310",
		"GCAP":         "+CGSM",
	}
	for k, want := range expected {
		if got := checks[k]; got != want {
			t.Errorf("ATI %s: got %q, want %q", k, got, want)
		}
	}
	if id.Revision == "" {
		t.Error("ATI Revision should not be empty")
	}
}

func TestParseIMPREF(t *testing.T) {
	t.Parallel()
	resp := "AT!IMPREF?\r\n!IMPREF:\r\n" +
		" preferred fw version:    02.24.05.06\r\n" +
		" preferred carrier name:  GENERIC\r\n" +
		" preferred config name:   GENERIC_002.026_000\r\n" +
		" current fw version:      02.24.05.06\r\n" +
		" current carrier name:    GENERIC\r\n" +
		" current config name:     GENERIC_002.026_000\r\n\r\nOK\r\n"
	pref, cur := parseIMPREF(resp)

	if pref.Version != "02.24.05.06" {
		t.Errorf("preferred version: got %q", pref.Version)
	}
	if pref.CarrierName != "GENERIC" {
		t.Errorf("preferred carrier: got %q", pref.CarrierName)
	}
	if cur.Version != "02.24.05.06" {
		t.Errorf("current version: got %q", cur.Version)
	}
	if cur.ConfigName != "GENERIC_002.026_000" {
		t.Errorf("current config: got %q", cur.ConfigName)
	}
}

func TestParseUSBCOMP(t *testing.T) {
	t.Parallel()
	resp := "AT!USBCOMP?\r\nConfig Index: 1\r\n" +
		"Config Type:  1 (Generic)\r\n" +
		"Interface bitmask: 0020100D (diag,nmea,modem,mbim,ubist)\r\n\r\nOK\r\n"
	info := parseUSBCOMP(resp)

	if info.ConfigIndex != 1 {
		t.Errorf("ConfigIndex: got %d", info.ConfigIndex)
	}
	if info.Bitmask != "0020100D" {
		t.Errorf("Bitmask: got %q", info.Bitmask)
	}
	if len(info.Interfaces) != 5 {
		t.Errorf("Interfaces: got %d, want 5", len(info.Interfaces))
	}
	if info.Interfaces[0] != "diag" {
		t.Errorf("Interfaces[0]: got %q, want diag", info.Interfaces[0])
	}
}

func TestParseUSBPID(t *testing.T) {
	t.Parallel()
	resp := "AT!USBPID?\r\n!USBPID:\r\nAPP : 81B6\r\nBOOT: 81B5\r\n\r\nOK\r\n"
	info := parseUSBPID(resp)

	if info.App != "81B6" {
		t.Errorf("App: got %q", info.App)
	}
	if info.Boot != "81B5" {
		t.Errorf("Boot: got %q", info.Boot)
	}
}

func TestParseUSBSpeed(t *testing.T) {
	t.Parallel()
	resp := "AT!USBSPEED?\r\nSUPPORTED:Super-Speed\r\nCURRENT  :High-Speed\r\n\r\nOK\r\n"
	info := parseUSBSpeed(resp)

	if info.Supported != "Super-Speed" {
		t.Errorf("Supported: got %q", info.Supported)
	}
	if info.Current != "High-Speed" {
		t.Errorf("Current: got %q", info.Current)
	}
}

func TestParsePRIID(t *testing.T) {
	t.Parallel()
	resp := "AT!PRIID?\r\nPRI Part Number: 9907375\r\nRevision: 001.001\r\n" +
		"Customer: PebbleCreekMLK\r\n\r\n" +
		"Carrier PRI: 9999999_9904609_SWI9X30C_02.24.05.06_00_GENERIC_002.026_000\r\n\r\nOK\r\n"
	p := parsePRIID(resp)

	if p.PartNumber != "9907375" {
		t.Errorf("PartNumber: got %q", p.PartNumber)
	}
	if p.Revision != "001.001" {
		t.Errorf("Revision: got %q", p.Revision)
	}
	if p.Customer != "PebbleCreekMLK" {
		t.Errorf("Customer: got %q", p.Customer)
	}
	if p.CarrierPRI == "" {
		t.Error("CarrierPRI should not be empty")
	}
}

func TestParseSELRAT(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		resp      string
		wantIndex int
		wantName  string
	}{
		{
			name:      "with prefix",
			resp:      "AT!SELRAT?\r\n!SELRAT: 06, LTE Only\r\n\r\nOK\r\n",
			wantIndex: 6,
			wantName:  "LTE Only",
		},
		{
			name:      "without prefix",
			resp:      "AT!SELRAT?\r\n06, LTE Only\r\n\r\nOK\r\n",
			wantIndex: 6,
			wantName:  "LTE Only",
		},
		{
			name:      "automatic",
			resp:      "AT!SELRAT?\r\n!SELRAT: 00, Automatic\r\n\r\nOK\r\n",
			wantIndex: 0,
			wantName:  "Automatic",
		},
		{
			name:      "empty response",
			resp:      "\r\nOK\r\n",
			wantIndex: 0,
			wantName:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sel := parseSELRAT(tt.resp)
			if sel.Index != tt.wantIndex {
				t.Errorf("Index: got %d, want %d", sel.Index, tt.wantIndex)
			}
			if sel.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", sel.Name, tt.wantName)
			}
		})
	}
}

func TestParseBandTable(t *testing.T) {
	t.Parallel()
	resp := "AT!BAND=?\r\n" +
		"Index, Name,                        GW Band Mask     L Band Mask      TDS Band Mask\r\n" +
		"00, All bands,                      0002000007C00000 00000100130818DF 0000000000000000\r\n" +
		"09, LTE ALL,                        0000000000000000 00000100130818DF 0000000000000000\r\n\r\nOK\r\n"
	bands := parseBandTable(resp)

	if len(bands) != 2 {
		t.Fatalf("bands: got %d, want 2", len(bands))
	}
	if bands[0].Name != "All bands" {
		t.Errorf("bands[0].Name: got %q", bands[0].Name)
	}
	if bands[0].Index != 0 {
		t.Errorf("bands[0].Index: got %d", bands[0].Index)
	}
	if bands[1].Name != "LTE ALL" {
		t.Errorf("bands[1].Name: got %q", bands[1].Name)
	}
	if bands[1].GWMask != "0000000000000000" {
		t.Errorf("bands[1].GWMask: got %q", bands[1].GWMask)
	}
}

func TestParsePCINFO(t *testing.T) {
	t.Parallel()
	resp := "AT!PCINFO?\r\nState: Online\r\n" +
		"LPM voters - Temp:0, Volt:0, User:0, W_DISABLE:0, IMSWITCH:1, BIOS:0, LWM2M:0, OMADM:0, FOTA:0\r\n" +
		"LPM persistence - None\r\n\r\nOK\r\n"
	state, voters, persistence := parsePCINFO(resp)

	if state != "Online" {
		t.Errorf("State: got %q", state)
	}
	if persistence != "None" {
		t.Errorf("Persistence: got %q", persistence)
	}
	if voters["IMSWITCH"] != 1 {
		t.Errorf("IMSWITCH voter: got %d, want 1", voters["IMSWITCH"])
	}
	if voters["Temp"] != 0 {
		t.Errorf("Temp voter: got %d, want 0", voters["Temp"])
	}
}

func TestParseCUSTOM(t *testing.T) {
	t.Parallel()
	resp := "AT!CUSTOM?\r\n!CUSTOM:\r\n" +
		"             GPSENABLE          0x01\r\n" +
		"             FASTENUMEN         0x02\r\n" +
		"             IPV6ENABLE         0x01\r\n\r\nOK\r\n"
	m := parseCUSTOM(resp)

	if m["GPSENABLE"] != "0x01" {
		t.Errorf("GPSENABLE: got %q", m["GPSENABLE"])
	}
	if m["FASTENUMEN"] != "0x02" {
		t.Errorf("FASTENUMEN: got %q", m["FASTENUMEN"])
	}
}

func TestParseIMAGE(t *testing.T) {
	t.Parallel()
	resp := "AT!IMAGE?\r\n" +
		"TYPE SLOT STATUS LRU FAILURES UNIQUE_ID   BUILD_ID\r\n" +
		"FW   1    GOOD   1   0 0      ?_?         02.24.05.06_?\r\n" +
		"FW   2    EMPTY  0   0 0\r\n" +
		"Max FW images: 4\r\n" +
		"Active FW image is at slot 1\r\n\r\n" +
		"TYPE SLOT STATUS LRU FAILURES UNIQUE_ID   BUILD_ID\r\n" +
		"PRI  FF   GOOD   0   0 0      002.026_000 02.24.05.06_GENERIC\r\n" +
		"Max PRI images: 50\r\n\r\nOK\r\n"
	info := parseIMAGE(resp)

	if len(info.Firmware) != 2 {
		t.Fatalf("Firmware slots: got %d, want 2", len(info.Firmware))
	}
	if info.Firmware[0].Status != "GOOD" {
		t.Errorf("FW slot 1 status: got %q", info.Firmware[0].Status)
	}
	if info.Firmware[0].BuildID != "02.24.05.06_?" {
		t.Errorf("FW slot 1 build: got %q", info.Firmware[0].BuildID)
	}
	if info.Firmware[1].Status != "EMPTY" {
		t.Errorf("FW slot 2 status: got %q", info.Firmware[1].Status)
	}
	if info.MaxFW != 4 {
		t.Errorf("MaxFW: got %d", info.MaxFW)
	}
	if info.ActiveSlot != 1 {
		t.Errorf("ActiveSlot: got %d", info.ActiveSlot)
	}
	if len(info.PRI) != 1 {
		t.Fatalf("PRI slots: got %d, want 1", len(info.PRI))
	}
	if info.PRI[0].UniqueID != "002.026_000" {
		t.Errorf("PRI UniqueID: got %q", info.PRI[0].UniqueID)
	}
}

func TestParseGSTATUS_Basic(t *testing.T) {
	t.Parallel()
	resp := "AT!GSTATUS?\r\n!GSTATUS:\r\n" +
		"Current Time:  22016\t\tTemperature: 40\r\n" +
		"Reset Counter: 1\t\tMode:        ONLINE\r\n" +
		"System mode:   LTE\t\tPS state:    Attached\r\n" +
		"LTE band:      B2\t\tLTE bw:      20 MHz\r\n" +
		"LTE Rx chan:   800\t\tLTE Tx chan: 18800\r\n" +
		"EMM state:     Registered\tNormal Service\r\n" +
		"RRC state:     RRC Connected\r\n" +
		"IMS reg state: No Srv\r\n" +
		"RSSI (dBm):    -60\t\tTx Power:    --\r\n" +
		"RSRP (dBm):    -90\t\tTAC:         1234 (0x04D2)\r\n" +
		"RSRQ (dB):     -10\t\tCell ID:     01234567 (012345)\r\n" +
		"SINR (dB):      5.2\r\n\r\nOK\r\n"

	m := parseGSTATUS(resp)

	checks := map[string]string{
		"Temperature":  "40",
		"System mode":  "LTE",
		"PS state":     "Attached",
		"LTE band":     "B2",
		"LTE bw":       "20 MHz",
		"Mode":         "ONLINE",
		"RSSI (dBm)":   "-60",
		"RSRP (dBm)":   "-90",
		"RSRQ (dB)":    "-10",
		"SINR (dB)":    "5.2",
		"TAC":          "1234 (0x04D2)",
		"Cell ID":      "01234567 (012345)",
		"RRC state":    "RRC Connected",
		"IMS reg state": "No Srv",
	}

	for k, want := range checks {
		if got, ok := m[k]; !ok {
			t.Errorf("GSTATUS missing key %q", k)
		} else if got != want {
			t.Errorf("GSTATUS %q: got %q, want %q", k, got, want)
		}
	}

	// EMM state should include continuation text
	if emm := m["EMM state"]; emm != "Registered Normal Service" {
		t.Errorf("EMM state: got %q, want %q", emm, "Registered Normal Service")
	}
}

func TestParseGSTATUS_CA(t *testing.T) {
	t.Parallel()
	// When CA is active, per-antenna-path lines appear with duplicate keys
	resp := "AT!GSTATUS?\r\n!GSTATUS:\r\n" +
		"Current Time:  22016\t\tTemperature: 40\r\n" +
		"System mode:   LTE\t\tPS state:    Attached\r\n" +
		"LTE band:      B2\t\tLTE bw:      20 MHz\r\n" +
		"LTE CA state:  ACTIVE\t\tLTE Scell band:B12\r\n" +
		"PCC RxM RSSI:  -45\t\tRSRP (dBm):  -81\r\n" +
		"PCC RxD RSSI:  -44\t\tRSRP (dBm):  -79\r\n" +
		"SCC RxM RSSI:  -52\t\tRSRP (dBm):  -83\r\n" +
		"SCC RxD RSSI:  -51\t\tRSRP (dBm):  -81\r\n" +
		"Tx Power:      15\t\tTAC:         8C65 (35666)\r\n" +
		"RSRQ (dB):     -14.0\t\tCell ID:     0A14666A (169106666)\r\n" +
		"SINR (dB):     23.8\r\n\r\nOK\r\n"

	m := parseGSTATUS(resp)

	// Per-path RSSI should be stored under their unique keys
	if v := m["PCC RxM RSSI"]; v != "-45" {
		t.Errorf("PCC RxM RSSI: got %q, want -45", v)
	}
	if v := m["SCC RxD RSSI"]; v != "-51" {
		t.Errorf("SCC RxD RSSI: got %q, want -51", v)
	}

	// The first RSRP should be stored as "RSRP (dBm)", subsequent as prefixed
	if _, ok := m["RSRP (dBm)"]; !ok {
		t.Error("GSTATUS CA: missing RSRP (dBm)")
	}

	// CA state should be captured
	if v := m["LTE CA state"]; v != "ACTIVE" {
		t.Errorf("LTE CA state: got %q", v)
	}

	// Temperature should still work
	if v := m["Temperature"]; v != "40" {
		t.Errorf("Temperature: got %q", v)
	}
}

func TestParseCSQ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		resp     string
		wantRSSI int
		wantBER  int
	}{
		{
			name:     "normal",
			resp:     "AT+CSQ\r\n+CSQ: 25,99\r\n\r\nOK\r\n",
			wantRSSI: -113 + 25*2, // -63
			wantBER:  99,
		},
		{
			name:     "unknown",
			resp:     "AT+CSQ\r\n+CSQ: 99,99\r\n\r\nOK\r\n",
			wantRSSI: 0,
			wantBER:  99,
		},
		{
			name:     "zero rssi",
			resp:     "AT+CSQ\r\n+CSQ: 0,0\r\n\r\nOK\r\n",
			wantRSSI: -113,
			wantBER:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rssi, ber := parseCSQ(tt.resp)
			if rssi != tt.wantRSSI {
				t.Errorf("RSSI: got %d, want %d", rssi, tt.wantRSSI)
			}
			if ber != tt.wantBER {
				t.Errorf("BER: got %d, want %d", ber, tt.wantBER)
			}
		})
	}
}

func TestParseGPSSTATUS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp string
		want GPSInfo
	}{
		{
			name: "active fix",
			resp: "AT!GPSSTATUS?\r\n" +
				"Fix Session Status = ACTIVE\r\n" +
				"TTFF (sec) = 26\r\n" +
				"Fix Status  = 3D\r\n" +
				"PDOP = 1.5  HDOP = 0.8  VDOP = 1.2\r\n" +
				"Latitude:  N  38 53 42.123\r\n" +
				"Longitude: W 077 02 11.456\r\n" +
				"Altitude (m) = 72.3\r\n" +
				"Heading (deg) =   0.0  Velocity (m/s) =   0.0\r\n" +
				"Satellite count = 12\r\n\r\nOK\r\n",
			want: GPSInfo{
				SessionStatus: "ACTIVE",
				FixStatus:     "3D",
				TTFF:          "26",
				Latitude:      "N  38 53 42.123",
				Longitude:     "W 077 02 11.456",
				Altitude:      "72.3",
				Satellites:    12,
				HDOP:          "0.8",
				PDOP:          "1.5",
				VDOP:          "1.2",
				Heading:       "0.0",
				Velocity:      "0.0",
			},
		},
		{
			name: "no fix",
			resp: "AT!GPSSTATUS?\r\n" +
				"Fix Session Status = NONE\r\n" +
				"TTFF (sec) = 0\r\n" +
				"Fix Status  = NO FIX\r\n\r\nOK\r\n",
			want: GPSInfo{
				SessionStatus: "NONE",
				FixStatus:     "NO FIX",
				TTFF:          "0",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseGPSSTATUS(tt.resp)
			if got.SessionStatus != tt.want.SessionStatus {
				t.Errorf("SessionStatus: got %q, want %q", got.SessionStatus, tt.want.SessionStatus)
			}
			if got.FixStatus != tt.want.FixStatus {
				t.Errorf("FixStatus: got %q, want %q", got.FixStatus, tt.want.FixStatus)
			}
			if got.Latitude != tt.want.Latitude {
				t.Errorf("Latitude: got %q, want %q", got.Latitude, tt.want.Latitude)
			}
			if got.Longitude != tt.want.Longitude {
				t.Errorf("Longitude: got %q, want %q", got.Longitude, tt.want.Longitude)
			}
			if got.Satellites != tt.want.Satellites {
				t.Errorf("Satellites: got %d, want %d", got.Satellites, tt.want.Satellites)
			}
			if got.HDOP != tt.want.HDOP {
				t.Errorf("HDOP: got %q, want %q", got.HDOP, tt.want.HDOP)
			}
			if got.PDOP != tt.want.PDOP {
				t.Errorf("PDOP: got %q, want %q", got.PDOP, tt.want.PDOP)
			}
			if got.VDOP != tt.want.VDOP {
				t.Errorf("VDOP: got %q, want %q", got.VDOP, tt.want.VDOP)
			}
			if got.Heading != tt.want.Heading {
				t.Errorf("Heading: got %q, want %q", got.Heading, tt.want.Heading)
			}
			if got.Velocity != tt.want.Velocity {
				t.Errorf("Velocity: got %q, want %q", got.Velocity, tt.want.Velocity)
			}
		})
	}
}

func TestParseCPIN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp string
		want string
	}{
		{"ready", "AT+CPIN?\r\n+CPIN: READY\r\n\r\nOK\r\n", "READY"},
		{"sim pin", "AT+CPIN?\r\n+CPIN: SIM PIN\r\n\r\nOK\r\n", "SIM PIN"},
		{"empty", "\r\nOK\r\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseCPIN(tt.resp); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseCIMI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp string
		want string
	}{
		{"normal", "AT+CIMI\r\n310410123456789\r\n\r\nOK\r\n", "310410123456789"},
		{"empty", "AT+CIMI\r\n\r\nOK\r\n", ""},
		{"short number", "AT+CIMI\r\n12345\r\n\r\nOK\r\n", ""}, // < 10 digits
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseCIMI(tt.resp); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseICCID(t *testing.T) {
	t.Parallel()
	resp := "AT+ICCID?\r\n+ICCID: 8901260882318054816\r\n\r\nOK\r\n"
	if got := parseICCID(resp); got != "8901260882318054816" {
		t.Errorf("got %q", got)
	}
}

func TestParseCOPS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp string
		want string
	}{
		{
			"normal",
			"AT+COPS?\r\n+COPS: 0,0,\"T-Mobile\",7\r\n\r\nOK\r\n",
			"T-Mobile",
		},
		{
			"not registered",
			"AT+COPS?\r\n+COPS: 0\r\n\r\nOK\r\n",
			"0",
		},
		{
			"att",
			"AT+COPS?\r\n+COPS: 0,0,\"AT&T\",7\r\n\r\nOK\r\n",
			"AT&T",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseCOPS(tt.resp); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRegStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		resp       string
		prefix     string
		wantStatus string
	}{
		{
			"creg registered",
			"+CREG: 0,1\r\n",
			"+CREG:",
			"Registered, home",
		},
		{
			"cereg roaming",
			"+CEREG: 0,5,\"1234\",\"01234567\",7\r\n",
			"+CEREG:",
			"Registered, roaming",
		},
		{
			"cgreg searching",
			"+CGREG: 0,2\r\n",
			"+CGREG:",
			"Searching",
		},
		{
			"not registered",
			"+CREG: 0,0\r\n",
			"+CREG:",
			"Not registered",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reg := parseRegStatus(tt.resp, tt.prefix)
			if reg.Status != tt.wantStatus {
				t.Errorf("Status: got %q, want %q", reg.Status, tt.wantStatus)
			}
		})
	}
}

func TestParseCGDCONT(t *testing.T) {
	t.Parallel()
	resp := "AT+CGDCONT?\r\n" +
		"+CGDCONT: 1,\"IP\",\"internet\",,0,0,0\r\n" +
		"+CGDCONT: 2,\"IPV4V6\",\"broadband\",,0,0,0\r\n\r\nOK\r\n"
	apns := parseCGDCONT(resp)

	if len(apns) != 2 {
		t.Fatalf("APNs: got %d, want 2", len(apns))
	}
	if apns[0] != "1,\"IP\",\"internet\",,0,0,0" {
		t.Errorf("APN[0]: got %q", apns[0])
	}
}

func TestParseLTEINFO(t *testing.T) {
	t.Parallel()
	resp := "AT!LTEINFO?\r\n!LTEINFO:\r\n" +
		"Serving:   EARFCN MCC MNC   TAC      CID Bd D U SNR PCI  RSRQ   RSRP   RSSI RXLV\r\n" +
		"              800 310 410 35666 0A14666A  2 5 5  22 404 -14.1  -79.6  -45.5 --\r\n\r\n" +
		"IntraFreq:                                          PCI  RSRQ   RSRP   RSSI RXLV\r\n" +
		"                                                    404 -14.1  -79.6  -45.5 --\r\n" +
		"                                                    402 -20.0  -89.3  -57.6 --\r\n\r\n" +
		"InterFreq: EARFCN ThresholdLow ThresholdHi Priority PCI  RSRQ   RSRP   RSSI RXLV\r\n" +
		"             5110            0           0        0 272 -12.1  -80.9  -50.6   0\r\n\r\nOK\r\n"

	info := parseLTEINFO(resp)

	if len(info.Serving) != 1 {
		t.Fatalf("Serving: got %d, want 1", len(info.Serving))
	}
	if info.Serving[0]["EARFCN"] != "800" {
		t.Errorf("Serving EARFCN: got %q", info.Serving[0]["EARFCN"])
	}
	if info.Serving[0]["PCI"] != "404" {
		t.Errorf("Serving PCI: got %q", info.Serving[0]["PCI"])
	}
	if info.Serving[0]["Bd"] != "2" {
		t.Errorf("Serving Bd: got %q", info.Serving[0]["Bd"])
	}

	if len(info.IntraFreq) != 2 {
		t.Fatalf("IntraFreq: got %d, want 2", len(info.IntraFreq))
	}
	if info.IntraFreq[0]["PCI"] != "404" {
		t.Errorf("IntraFreq[0] PCI: got %q", info.IntraFreq[0]["PCI"])
	}
	if info.IntraFreq[1]["PCI"] != "402" {
		t.Errorf("IntraFreq[1] PCI: got %q", info.IntraFreq[1]["PCI"])
	}

	if len(info.InterFreq) != 1 {
		t.Fatalf("InterFreq: got %d, want 1", len(info.InterFreq))
	}
	if info.InterFreq[0]["EARFCN"] != "5110" {
		t.Errorf("InterFreq EARFCN: got %q", info.InterFreq[0]["EARFCN"])
	}
}

func TestParseLTECA(t *testing.T) {
	t.Parallel()
	resp := "AT!LTECA?\r\n!LTECA:\r\n" +
		"Hardware:\r\n" +
		"LTEB2: B2, B5, B12, B13, B29,\r\n" +
		"LTEB4: B4, B5, B12, B13, B29,\r\n" +
		"Permitted Bands:\r\n" +
		"LTEB2: B5, B12,\r\n" +
		"LTEB25:\r\n" +
		"Prune_ca_combos:\r\n" +
		"Empty\r\n\r\nOK\r\n"
	got := parseLTECA(resp)

	if len(got.Hardware) != 2 {
		t.Fatalf("Hardware: got %d combos, want 2", len(got.Hardware))
	}
	if got.Hardware[0].Primary != "B2" {
		t.Errorf("Hardware[0].Primary: got %q, want B2", got.Hardware[0].Primary)
	}
	if len(got.Hardware[0].Secondary) != 5 {
		t.Errorf("Hardware[0].Secondary: got %v, want [B2 B5 B12 B13 B29]", got.Hardware[0].Secondary)
	}
	if got.Hardware[1].Primary != "B4" {
		t.Errorf("Hardware[1].Primary: got %q, want B4", got.Hardware[1].Primary)
	}

	if len(got.Permitted) != 2 {
		t.Fatalf("Permitted: got %d combos, want 2", len(got.Permitted))
	}
	if got.Permitted[0].Primary != "B2" {
		t.Errorf("Permitted[0].Primary: got %q", got.Permitted[0].Primary)
	}
	if len(got.Permitted[0].Secondary) != 2 {
		t.Errorf("Permitted[0].Secondary: got %v, want [B5 B12]", got.Permitted[0].Secondary)
	}
	// LTEB25: with no secondary bands
	if got.Permitted[1].Primary != "B25" {
		t.Errorf("Permitted[1].Primary: got %q, want B25", got.Permitted[1].Primary)
	}
	if len(got.Permitted[1].Secondary) != 0 {
		t.Errorf("Permitted[1].Secondary: got %v, want empty", got.Permitted[1].Secondary)
	}

	if got.Pruned != "Empty" {
		t.Errorf("Pruned: got %q, want Empty", got.Pruned)
	}
}

func TestParseHWID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp string
		want string
	}{
		{
			"with revision label",
			"AT!HWID?\r\nRevision: 0.5\r\n\r\nOK\r\n",
			"0.5",
		},
		{
			"with prefix line",
			"AT!HWID?\r\n!HWID:\r\nRevision: 0.5\r\n\r\nOK\r\n",
			"0.5",
		},
		{
			"raw value",
			"AT!HWID?\r\n1102803\r\n\r\nOK\r\n",
			"1102803",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseHWID(tt.resp); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsDigits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    string
		want bool
	}{
		{"123", true},
		{"0", true},
		{"", false},
		{"12a3", false},
		{"310410123456789", true},
	}
	for _, tt := range tests {
		if got := isDigits(tt.s); got != tt.want {
			t.Errorf("isDigits(%q): got %v, want %v", tt.s, got, tt.want)
		}
	}
}
