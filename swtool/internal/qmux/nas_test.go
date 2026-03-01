package qmux

import (
	"encoding/binary"
	"testing"
)

// setupNASConn creates a fake transport and Conn with a pre-allocated NAS client.
func setupNASConn(t *testing.T) (*fakeTransport, *Conn) {
	t.Helper()
	ft := newFakeTransport()

	// Queue CTL allocate-CID response for NAS → client ID 2.
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 1, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceNAS), 0x02}}},
	})

	return ft, NewConn(ft)
}

// putInt16 writes a signed int16 as little-endian uint16 into buf.
func putInt16(buf []byte, v int16) {
	binary.LittleEndian.PutUint16(buf, uint16(v))
}

// int8Byte returns the byte representation of a signed int8.
func int8Byte(v int8) byte {
	return byte(v)
}

func TestGetSignalInfoLTE(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	// LTE signal TLV 0x14: RSSI(2) + RSRQ(1) + RSRP(2) + SNR(2) = 7 bytes
	// RSSI=-55, RSRQ=-11, RSRP=-89, SNR=78 (7.8 dB)
	lteData := make([]byte, 7)
	putInt16(lteData[0:2], -55)
	lteData[2] = int8Byte(-11)
	putInt16(lteData[3:5], -89)
	putInt16(lteData[5:7], 78) // 78 = 7.8 dB

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetSignalInfo,
		TLVs: []TLV{
			successResult(),
			{Type: 0x14, Value: lteData},
		},
	})

	info, err := GetSignalInfo(conn)
	if err != nil {
		t.Fatalf("GetSignalInfo: %v", err)
	}
	if info.LTE == nil {
		t.Fatal("LTE signal is nil")
	}
	if info.LTE.RSSI != -55 {
		t.Errorf("RSSI = %d, want -55", info.LTE.RSSI)
	}
	if info.LTE.RSRQ != -11 {
		t.Errorf("RSRQ = %d, want -11", info.LTE.RSRQ)
	}
	if info.LTE.RSRP != -89 {
		t.Errorf("RSRP = %d, want -89", info.LTE.RSRP)
	}
	if info.LTE.SNR < 7.7 || info.LTE.SNR > 7.9 {
		t.Errorf("SNR = %f, want ~7.8", info.LTE.SNR)
	}
	if info.WCDMA != nil {
		t.Errorf("WCDMA should be nil, got %+v", info.WCDMA)
	}
}

func TestGetSignalInfoWCDMA(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	// WCDMA signal TLV 0x13: RSSI(1) + ECIO(2) = 3 bytes
	wcdmaData := make([]byte, 3)
	wcdmaData[0] = int8Byte(-70)
	putInt16(wcdmaData[1:3], -40) // -4.0 dB

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetSignalInfo,
		TLVs: []TLV{
			successResult(),
			{Type: 0x13, Value: wcdmaData},
		},
	})

	info, err := GetSignalInfo(conn)
	if err != nil {
		t.Fatalf("GetSignalInfo: %v", err)
	}
	if info.WCDMA == nil {
		t.Fatal("WCDMA signal is nil")
	}
	if info.WCDMA.RSSI != -70 {
		t.Errorf("WCDMA RSSI = %d, want -70", info.WCDMA.RSSI)
	}
	if info.WCDMA.ECIO != -40 {
		t.Errorf("WCDMA ECIO = %d, want -40", info.WCDMA.ECIO)
	}
}

func TestGetSignalInfoGSM(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetSignalInfo,
		TLVs: []TLV{
			successResult(),
			{Type: 0x12, Value: []byte{int8Byte(-80)}},
		},
	})

	info, err := GetSignalInfo(conn)
	if err != nil {
		t.Fatalf("GetSignalInfo: %v", err)
	}
	if info.GSM == nil {
		t.Fatal("GSM signal is nil")
	}
	if info.GSM.RSSI != -80 {
		t.Errorf("GSM RSSI = %d, want -80", info.GSM.RSSI)
	}
}

func TestGetSignalInfoEmpty(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetSignalInfo,
		TLVs: []TLV{successResult()},
	})

	info, err := GetSignalInfo(conn)
	if err != nil {
		t.Fatalf("GetSignalInfo: %v", err)
	}
	if info.LTE != nil || info.WCDMA != nil || info.GSM != nil {
		t.Error("expected all nil signal structs for empty response")
	}
}

func TestGetServingSystem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mandTLV  []byte // TLV 0x01 value
		roamTLV  []byte // TLV 0x10 value (nil = absent)
		plmnTLV  []byte // TLV 0x12 value (nil = absent)
		wantReg  bool
		wantRAT  string
		wantRoam bool
		wantMCC  uint16
		wantMNC  uint16
	}{
		{
			name: "registered LTE",
			// regState=1(registered) csAttach=1 psAttach=1 selNet=1 riCount=1 ri[0]=8(lte)
			mandTLV: []byte{0x01, 0x01, 0x01, 0x01, 0x01, 0x08},
			roamTLV: []byte{0x00}, // not roaming
			plmnTLV: func() []byte {
				b := make([]byte, 7)
				binary.LittleEndian.PutUint16(b[0:2], 310) // MCC
				binary.LittleEndian.PutUint16(b[2:4], 260) // MNC
				b[4] = 0x08                                // descLen
				copy(b[5:], []byte("T-M"))                 // desc (truncated)
				return b
			}(),
			wantReg:  true,
			wantRAT:  "lte",
			wantRoam: false,
			wantMCC:  310,
			wantMNC:  260,
		},
		{
			name:    "not registered",
			mandTLV: []byte{0x00, 0x00, 0x00, 0x01, 0x00}, // regState=0, no RIs
			wantReg: false,
			wantRAT: "",
		},
		{
			name:    "registered roaming UMTS",
			mandTLV: []byte{0x01, 0x01, 0x01, 0x01, 0x01, 0x05},
			roamTLV: []byte{0x01}, // roaming
			plmnTLV: func() []byte {
				b := make([]byte, 6)
				binary.LittleEndian.PutUint16(b[0:2], 234)
				binary.LittleEndian.PutUint16(b[2:4], 15)
				b[4] = 0x02
				b[5] = 'V'
				return b
			}(),
			wantReg:  true,
			wantRAT:  "umts",
			wantRoam: true,
			wantMCC:  234,
			wantMNC:  15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ft, conn := setupNASConn(t)
			defer conn.Close()

			tlvs := []TLV{
				successResult(),
				{Type: 0x01, Value: tt.mandTLV},
			}
			if tt.roamTLV != nil {
				tlvs = append(tlvs, TLV{Type: 0x10, Value: tt.roamTLV})
			}
			if tt.plmnTLV != nil {
				tlvs = append(tlvs, TLV{Type: 0x12, Value: tt.plmnTLV})
			}

			ft.queueResponse(&Message{
				Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetServingSystem,
				TLVs: tlvs,
			})

			sys, err := GetServingSystem(conn)
			if err != nil {
				t.Fatalf("GetServingSystem: %v", err)
			}
			if sys.Registered != tt.wantReg {
				t.Errorf("Registered = %v, want %v", sys.Registered, tt.wantReg)
			}
			if sys.RAT != tt.wantRAT {
				t.Errorf("RAT = %q, want %q", sys.RAT, tt.wantRAT)
			}
			if sys.Roaming != tt.wantRoam {
				t.Errorf("Roaming = %v, want %v", sys.Roaming, tt.wantRoam)
			}
			if sys.MCC != tt.wantMCC {
				t.Errorf("MCC = %d, want %d", sys.MCC, tt.wantMCC)
			}
			if sys.MNC != tt.wantMNC {
				t.Errorf("MNC = %d, want %d", sys.MNC, tt.wantMNC)
			}
		})
	}
}

func TestGetSystemInfo(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	// TLV 0x14: LTE Service Status = available(2)
	svcTLV := []byte{0x02, 0x02, 0x01} // svcStatus=2, trueStatus=2, prefData=1

	// TLV 0x19: LTE System Info (21 bytes)
	// domain=3(cs-ps) svcCap=3 roaming=0 forbidden=0
	// lacValid=0 lac=0000 cidValid=1 cid=67890 regReject=0
	// plmnValid=1 mcc=310 mnc=260 tacValid=1 tac=12345
	sysData := make([]byte, 21)
	sysData[0] = 0x03 // domain = cs-ps
	sysData[1] = 0x03 // svcCapability
	sysData[2] = 0x00 // roaming = off
	sysData[3] = 0x00 // forbidden
	sysData[4] = 0x00 // lacValid
	// lac bytes 5-6 = 0
	sysData[7] = 0x01 // cidValid
	binary.LittleEndian.PutUint32(sysData[8:12], 67890) // cellID
	sysData[12] = 0x00 // regReject
	sysData[13] = 0x01 // plmnValid
	binary.LittleEndian.PutUint16(sysData[14:16], 310) // MCC
	binary.LittleEndian.PutUint16(sysData[16:18], 260) // MNC
	sysData[18] = 0x01                                  // tacValid
	binary.LittleEndian.PutUint16(sysData[19:21], 12345) // TAC

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetSystemInfo,
		TLVs: []TLV{
			successResult(),
			{Type: 0x14, Value: svcTLV},
			{Type: 0x19, Value: sysData},
		},
	})

	info, err := GetSystemInfo(conn)
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if info.ServiceStatus != 2 {
		t.Errorf("ServiceStatus = %d, want 2", info.ServiceStatus)
	}
	if ServiceStatusString(info.ServiceStatus) != "available" {
		t.Errorf("ServiceStatusString = %q, want \"available\"", ServiceStatusString(info.ServiceStatus))
	}
	if info.Domain != 3 {
		t.Errorf("Domain = %d, want 3", info.Domain)
	}
	if DomainString(info.Domain) != "cs-ps" {
		t.Errorf("DomainString = %q, want \"cs-ps\"", DomainString(info.Domain))
	}
	if info.Roaming {
		t.Error("Roaming = true, want false")
	}
	if info.MCC != 310 {
		t.Errorf("MCC = %d, want 310", info.MCC)
	}
	if info.MNC != 260 {
		t.Errorf("MNC = %d, want 260", info.MNC)
	}
	if info.TAC != 12345 {
		t.Errorf("TAC = %d, want 12345", info.TAC)
	}
	if info.CellID != 67890 {
		t.Errorf("CellID = %d, want 67890", info.CellID)
	}
}

func TestGetCellLocationInfoIntra(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	// Build intra-freq TLV 0x13.
	// Header: 18 bytes + cellCount(1) + cells
	intraData := make([]byte, 19+2*10) // header + 2 cells × 10 bytes
	intraData[18] = 0x02               // cellCount = 2

	off := 19
	// Cell 0: PCI=123, RSRQ=-110, RSRP=-890, RSSI=-550
	binary.LittleEndian.PutUint16(intraData[off:off+2], 123)
	putInt16(intraData[off+2:off+4], -110)
	putInt16(intraData[off+4:off+6], -890)
	putInt16(intraData[off+6:off+8], -550)
	binary.LittleEndian.PutUint16(intraData[off+8:off+10], 0) // srxlev
	off += 10

	// Cell 1: PCI=456, RSRQ=-150, RSRP=-1020, RSSI=-700
	binary.LittleEndian.PutUint16(intraData[off:off+2], 456)
	putInt16(intraData[off+2:off+4], -150)
	putInt16(intraData[off+4:off+6], -1020)
	putInt16(intraData[off+6:off+8], -700)
	binary.LittleEndian.PutUint16(intraData[off+8:off+10], 0)

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetCellLocationInfo,
		TLVs: []TLV{
			successResult(),
			{Type: 0x13, Value: intraData},
		},
	})

	info, err := GetCellLocationInfo(conn)
	if err != nil {
		t.Fatalf("GetCellLocationInfo: %v", err)
	}
	if len(info.LTEIntra) != 2 {
		t.Fatalf("LTEIntra count = %d, want 2", len(info.LTEIntra))
	}
	if info.LTEIntra[0].PCI != 123 {
		t.Errorf("cell[0].PCI = %d, want 123", info.LTEIntra[0].PCI)
	}
	if info.LTEIntra[0].RSRP != -890 {
		t.Errorf("cell[0].RSRP = %d, want -890", info.LTEIntra[0].RSRP)
	}
	if info.LTEIntra[1].PCI != 456 {
		t.Errorf("cell[1].PCI = %d, want 456", info.LTEIntra[1].PCI)
	}
}

func TestGetCellLocationInfoInter(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	// Build inter-freq TLV 0x14.
	// Header: ueInIdle(1) + freqCount(1)
	// Freq 0: earfcn=2100, threshXLow, threshXHigh, cellReselPriority, cellCount=1
	//   Cell: pci=789, rsrq=-130, rsrp=-950, rssi=-620, srxlev=0
	interData := make([]byte, 0, 20)
	interData = append(interData, 0x00) // ueInIdle
	interData = append(interData, 0x01) // freqCount = 1

	// Frequency 0
	earfcn := make([]byte, 2)
	binary.LittleEndian.PutUint16(earfcn, 2100)
	interData = append(interData, earfcn...)
	interData = append(interData, 0x00, 0x00, 0x00) // threshXLow, threshXHigh, cellReselPriority
	interData = append(interData, 0x01)              // cellCount = 1

	cellBuf := make([]byte, 10)
	binary.LittleEndian.PutUint16(cellBuf[0:2], 789)
	putInt16(cellBuf[2:4], -130)
	putInt16(cellBuf[4:6], -950)
	putInt16(cellBuf[6:8], -620)
	binary.LittleEndian.PutUint16(cellBuf[8:10], 0)
	interData = append(interData, cellBuf...)

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetCellLocationInfo,
		TLVs: []TLV{
			successResult(),
			{Type: 0x14, Value: interData},
		},
	})

	info, err := GetCellLocationInfo(conn)
	if err != nil {
		t.Fatalf("GetCellLocationInfo: %v", err)
	}
	if len(info.LTEInter) != 1 {
		t.Fatalf("LTEInter count = %d, want 1", len(info.LTEInter))
	}
	if info.LTEInter[0].PCI != 789 {
		t.Errorf("cell.PCI = %d, want 789", info.LTEInter[0].PCI)
	}
	if info.LTEInter[0].EARFCN != 2100 {
		t.Errorf("cell.EARFCN = %d, want 2100", info.LTEInter[0].EARFCN)
	}
	if info.LTEInter[0].RSRP != -950 {
		t.Errorf("cell.RSRP = %d, want -950", info.LTEInter[0].RSRP)
	}
}

func TestGetCellLocationInfoEmpty(t *testing.T) {
	t.Parallel()
	ft, conn := setupNASConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 1, MsgID: nasMsgGetCellLocationInfo,
		TLVs: []TLV{successResult()},
	})

	info, err := GetCellLocationInfo(conn)
	if err != nil {
		t.Fatalf("GetCellLocationInfo: %v", err)
	}
	if len(info.LTEIntra) != 0 || len(info.LTEInter) != 0 {
		t.Error("expected empty cell lists")
	}
}

func TestFormatSignalValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{"dBm", func() string { return FormatSignalDBM(-55) }, "-55 dBm"},
		{"dB", func() string { return FormatSignalDB(-11) }, "-11 dB"},
		{"SNR", func() string { return FormatSNR(7.8) }, "7.8 dB"},
		{"tenths dBm", func() string { return FormatSignalTenthsDBM(-890) }, "-89.0 dBm"},
		{"tenths dB", func() string { return FormatSignalTenthsDB(-110) }, "-11.0 dB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.fn(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
