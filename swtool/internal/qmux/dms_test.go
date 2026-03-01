package qmux

import (
	"errors"
	"testing"
)

// setupDMSConn creates a fake transport and Conn with a pre-allocated DMS client.
func setupDMSConn(t *testing.T) (*fakeTransport, *Conn) {
	t.Helper()
	ft := newFakeTransport()

	// Queue CTL allocate-CID response for DMS → client ID 1.
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 1, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceDMS), 0x01}}},
	})

	return ft, NewConn(ft)
}

func TestGetOperatingMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mode     uint8
		wantStr  string
	}{
		{"online", ModeOnline, "online"},
		{"low-power", ModeLowPower, "low-power"},
		{"offline", ModeOffline, "offline"},
		{"reset", ModeReset, "reset"},
		{"shutdown", ModeShutdown, "shutting-down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ft, conn := setupDMSConn(t)
			defer conn.Close()

			ft.queueResponse(&Message{
				Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgGetOperatingMode,
				TLVs: []TLV{successResult(), EncodeTLVU8(0x01, tt.mode)},
			})

			mode, err := GetOperatingMode(conn)
			if err != nil {
				t.Fatalf("GetOperatingMode: %v", err)
			}
			if mode != tt.mode {
				t.Errorf("mode = %d, want %d", mode, tt.mode)
			}
			if ModeString(mode) != tt.wantStr {
				t.Errorf("ModeString(%d) = %q, want %q", mode, ModeString(mode), tt.wantStr)
			}
		})
	}
}

func TestSetOperatingMode(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgSetOperatingMode,
		TLVs: []TLV{successResult()},
	})

	if err := SetOperatingMode(conn, ModeReset); err != nil {
		t.Fatalf("SetOperatingMode: %v", err)
	}
}

func TestSetOperatingModeError(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgSetOperatingMode,
		TLVs: []TLV{errorResult(0x0003)}, // internal error
	})

	err := SetOperatingMode(conn, ModeReset)
	if err == nil {
		t.Fatal("SetOperatingMode returned nil, want error")
	}
	if !errors.Is(err, ErrInternal) {
		t.Errorf("error = %v, want ErrInternal", err)
	}
}

func TestParseStoredImagesTLV(t *testing.T) {
	t.Parallel()
	// Build a TLV payload representing 2 list entries (modem + pri),
	// each with 2 image slots.
	//
	// Layout per list: type(1) maxImages(1) runningIndex(1) sublistCount(1)
	// Per entry: storageIndex(1) failureCount(1) uniqueID(16 fixed) buildIDLen(1) buildID(N)

	var data []byte
	data = append(data, 0x02) // listCount = 2

	// Entry 0: modem
	data = append(data, 0x00) // type = modem
	data = append(data, 0x02) // maxImages
	data = append(data, 0x00) // runningIndex
	data = append(data, 0x02) // sublistCount = 2
	// slot 0
	data = append(data, 0x00, 0x00) // storageIndex=0, failureCount=0
	data = append(data, padUID("?_?")...)
	data = append(data, 0x13)                              // buildIDLen = 19
	data = append(data, []byte("02.39.00.00_GENERIC")...) // buildID
	// slot 1
	data = append(data, 0x01, 0x01) // storageIndex=1, failureCount=1
	data = append(data, padUID("?_?")...)
	data = append(data, 0x00) // buildIDLen = 0

	// Entry 1: pri
	data = append(data, 0x01) // type = pri
	data = append(data, 0x02) // maxImages
	data = append(data, 0x00) // runningIndex
	data = append(data, 0x02) // sublistCount = 2
	// slot 0
	data = append(data, 0x00, 0x00) // storageIndex=0, failureCount=0
	data = append(data, padUID("config_ver_0")...)
	data = append(data, 0x13)                              // buildIDLen = 19
	data = append(data, []byte("02.39.00.00_GENERIC")...) // buildID
	// slot 1
	data = append(data, 0x01, 0x00) // storageIndex=1, failureCount=0
	data = append(data, padUID("")...)
	data = append(data, 0x00) // buildIDLen = 0

	images, err := parseStoredImagesTLV(data)
	if err != nil {
		t.Fatalf("parseStoredImagesTLV: %v", err)
	}

	if len(images) != 4 {
		t.Fatalf("got %d images, want 4", len(images))
	}

	// Modem slot 0.
	if images[0].Type != 0 || images[0].Slot != 0 {
		t.Errorf("images[0] type/slot = %d/%d, want 0/0", images[0].Type, images[0].Slot)
	}
	if images[0].UniqueID != "?_?" {
		t.Errorf("images[0].UniqueID = %q, want \"?_?\"", images[0].UniqueID)
	}
	if images[0].BuildID != "02.39.00.00_GENERIC" {
		t.Errorf("images[0].BuildID = %q, want \"02.39.00.00_GENERIC\"", images[0].BuildID)
	}
	if images[0].StorageIndex != 0 || images[0].FailureCount != 0 {
		t.Errorf("images[0] storage/failure = %d/%d, want 0/0", images[0].StorageIndex, images[0].FailureCount)
	}

	// Modem slot 1.
	if images[1].Type != 0 || images[1].Slot != 1 {
		t.Errorf("images[1] type/slot = %d/%d, want 0/1", images[1].Type, images[1].Slot)
	}
	if images[1].BuildID != "" {
		t.Errorf("images[1].BuildID = %q, want empty", images[1].BuildID)
	}
	if images[1].FailureCount != 1 {
		t.Errorf("images[1].FailureCount = %d, want 1", images[1].FailureCount)
	}

	// PRI slot 0.
	if images[2].Type != 1 || images[2].Slot != 0 {
		t.Errorf("images[2] type/slot = %d/%d, want 1/0", images[2].Type, images[2].Slot)
	}
	if images[2].UniqueID != "config_ver_0" {
		t.Errorf("images[2].UniqueID = %q, want \"config_ver_0\"", images[2].UniqueID)
	}

	// PRI slot 1.
	if images[3].Type != 1 || images[3].Slot != 1 {
		t.Errorf("images[3] type/slot = %d/%d, want 1/1", images[3].Type, images[3].Slot)
	}
	if images[3].UniqueID != "" || images[3].BuildID != "" {
		t.Errorf("images[3] IDs = %q/%q, want empty", images[3].UniqueID, images[3].BuildID)
	}
}

func TestParseStoredImagesTLVEmpty(t *testing.T) {
	t.Parallel()
	data := []byte{0x00} // count = 0
	images, err := parseStoredImagesTLV(data)
	if err != nil {
		t.Fatalf("parseStoredImagesTLV: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("got %d images, want 0", len(images))
	}
}

func TestParseStoredImagesTLVTruncated(t *testing.T) {
	t.Parallel()
	// Count says 1 entry but no data follows.
	data := []byte{0x01}
	_, err := parseStoredImagesTLV(data)
	if err == nil {
		t.Error("expected error for truncated data")
	}
}

func TestParseFirmwarePrefTLV(t *testing.T) {
	t.Parallel()
	// 2 entries: modem + pri
	// Layout: count(1) + per entry: type(1) uniqueID(16 fixed) buildIDLen(1) buildID(N)
	var data []byte
	data = append(data, 0x02) // count

	// Modem.
	data = append(data, 0x00)                             // type
	data = append(data, padUID("?_?")...)                  // uniqueID (16 bytes)
	data = append(data, 0x13)                              // buildIDLen = 19
	data = append(data, []byte("02.39.00.00_GENERIC")...) // buildID

	// PRI.
	data = append(data, 0x01)                              // type
	data = append(data, padUID("config_ver_0")...)         // uniqueID (16 bytes)
	data = append(data, 0x13)                              // buildIDLen = 19
	data = append(data, []byte("02.39.00.00_GENERIC")...)  // buildID

	images, err := parseFirmwarePrefTLV(data)
	if err != nil {
		t.Fatalf("parseFirmwarePrefTLV: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2", len(images))
	}
	if images[0].Type != 0 || images[0].UniqueID != "?_?" {
		t.Errorf("images[0] = %+v", images[0])
	}
	if images[1].Type != 1 || images[1].UniqueID != "config_ver_0" {
		t.Errorf("images[1] = %+v", images[1])
	}
}

func TestListStoredImages(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	// Build a simple stored images response with 1 modem slot.
	var storedData []byte
	storedData = append(storedData, 0x01) // listCount = 1
	storedData = append(storedData, 0x00) // type = modem
	storedData = append(storedData, 0x01) // maxImages
	storedData = append(storedData, 0x00) // runningIndex
	storedData = append(storedData, 0x01) // sublistCount = 1
	storedData = append(storedData, 0x00, 0x00) // storageIndex=0, failureCount=0
	storedData = append(storedData, padUID("?_?")...)
	storedData = append(storedData, 0x05) // buildIDLen
	storedData = append(storedData, []byte("1.0_G")...)

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgListStoredImages,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: storedData}},
	})

	images, err := ListStoredImages(conn)
	if err != nil {
		t.Fatalf("ListStoredImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	if images[0].BuildID != "1.0_G" {
		t.Errorf("BuildID = %q, want \"1.0_G\"", images[0].BuildID)
	}
}

func TestGetFirmwarePreference(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	var prefData []byte
	prefData = append(prefData, 0x01)             // count
	prefData = append(prefData, 0x00)             // type = modem
	prefData = append(prefData, padUID("?_?")...) // uniqueID (16 bytes)
	prefData = append(prefData, 0x05)             // buildIDLen
	prefData = append(prefData, []byte("1.0_G")...)

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgGetFirmwarePref,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: prefData}},
	})

	prefs, err := GetFirmwarePreference(conn)
	if err != nil {
		t.Fatalf("GetFirmwarePreference: %v", err)
	}
	if len(prefs) != 1 {
		t.Fatalf("got %d prefs, want 1", len(prefs))
	}
	if prefs[0].BuildID != "1.0_G" {
		t.Errorf("BuildID = %q, want \"1.0_G\"", prefs[0].BuildID)
	}
}

func TestSetFirmwarePreference(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgSetFirmwarePref,
		TLVs: []TLV{successResult()},
	})

	err := SetFirmwarePreference(conn, "02.39.00.00", "config_ver", "GENERIC")
	if err != nil {
		t.Fatalf("SetFirmwarePreference: %v", err)
	}

	// Verify the sent frame contains the right TLV payload.
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if len(ft.sent) < 2 {
		t.Fatal("expected at least 2 sent frames (CTL alloc + DMS set)")
	}
	setFrame := ft.sent[1]
	msg, err := DecodeMessage(setFrame)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}
	tlv := msg.FindTLV(0x01)
	if tlv == nil {
		t.Fatal("missing TLV 0x01 in set firmware preference request")
	}
	// First byte is count = 2.
	if tlv.Value[0] != 0x02 {
		t.Errorf("preference count = %d, want 2", tlv.Value[0])
	}
}

func TestSetUSBComposition(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsSWISetUSBComposition,
		TLVs: []TLV{successResult()},
	})

	if err := SetUSBComposition(conn, 8); err != nil {
		t.Fatalf("SetUSBComposition: %v", err)
	}
}

func TestDeleteStoredImage(t *testing.T) {
	t.Parallel()
	ft, conn := setupDMSConn(t)
	defer conn.Close()

	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: dmsMsgDeleteStoredImage,
		TLVs: []TLV{successResult()},
	})

	err := DeleteStoredImage(conn, StoredImage{
		Type:     0,
		UniqueID: "?_?",
		BuildID:  "1.0_G",
	})
	if err != nil {
		t.Fatalf("DeleteStoredImage: %v", err)
	}
}

func TestImageTypeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		typ  uint8
		want string
	}{
		{0, "modem"},
		{1, "pri"},
		{2, "unknown-2"},
	}
	for _, tt := range tests {
		if got := ImageTypeString(tt.typ); got != tt.want {
			t.Errorf("ImageTypeString(%d) = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestBuildIDParts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input       string
		wantFW      string
		wantCarrier string
	}{
		{"02.39.00.00_GENERIC", "02.39.00.00", "GENERIC"},
		{"SWI9X50C_01.14.02.00_ATT", "SWI9X50C_01.14.02.00", "ATT"},
		{"nounderscores", "nounderscores", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			fw, carrier := buildIDParts(tt.input)
			if fw != tt.wantFW {
				t.Errorf("fw = %q, want %q", fw, tt.wantFW)
			}
			if carrier != tt.wantCarrier {
				t.Errorf("carrier = %q, want %q", carrier, tt.wantCarrier)
			}
		})
	}
}
