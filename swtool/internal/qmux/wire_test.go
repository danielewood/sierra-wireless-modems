package qmux

import (
	"encoding/hex"
	"errors"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  Message
	}{
		{
			name: "CTL allocate client request",
			msg: Message{
				Service: ServiceCTL,
				Client:  0x00,
				TxnID:   1,
				MsgID:   0x0022, // Allocate CID
				TLVs: []TLV{
					EncodeTLVU8(0x01, byte(ServiceDMS)),
				},
			},
		},
		{
			name: "DMS get operating mode request",
			msg: Message{
				Service: ServiceDMS,
				Client:  0x01,
				TxnID:   1,
				MsgID:   0x002D,
			},
		},
		{
			name: "NAS get signal info request",
			msg: Message{
				Service: ServiceNAS,
				Client:  0x02,
				TxnID:   42,
				MsgID:   0x004F,
			},
		},
		{
			name: "message with multiple TLVs",
			msg: Message{
				Service: ServiceDMS,
				Client:  0x01,
				TxnID:   5,
				MsgID:   0x002E, // Set Operating Mode
				TLVs: []TLV{
					EncodeTLVU8(0x01, 0x00),                         // mode = online
					{Type: 0x10, Value: []byte{0xAA, 0xBB, 0xCC}},  // arbitrary
				},
			},
		},
		{
			name: "CTL with zero TLVs",
			msg: Message{
				Service: ServiceCTL,
				Client:  0x00,
				TxnID:   0,
				MsgID:   0x0000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			frame, err := EncodeMessage(&tt.msg)
			if err != nil {
				t.Fatalf("EncodeMessage: %v", err)
			}

			// Verify marker byte.
			if frame[0] != qmuxMarker {
				t.Errorf("marker = 0x%02x, want 0x%02x", frame[0], qmuxMarker)
			}

			got, err := DecodeMessage(frame)
			if err != nil {
				t.Fatalf("DecodeMessage: %v", err)
			}

			if got.Service != tt.msg.Service {
				t.Errorf("Service = %d, want %d", got.Service, tt.msg.Service)
			}
			if got.Client != tt.msg.Client {
				t.Errorf("Client = %d, want %d", got.Client, tt.msg.Client)
			}
			if got.TxnID != tt.msg.TxnID {
				t.Errorf("TxnID = %d, want %d", got.TxnID, tt.msg.TxnID)
			}
			if got.MsgID != tt.msg.MsgID {
				t.Errorf("MsgID = 0x%04x, want 0x%04x", got.MsgID, tt.msg.MsgID)
			}
			if len(got.TLVs) != len(tt.msg.TLVs) {
				t.Fatalf("TLV count = %d, want %d", len(got.TLVs), len(tt.msg.TLVs))
			}
			for i := range got.TLVs {
				if got.TLVs[i].Type != tt.msg.TLVs[i].Type {
					t.Errorf("TLV[%d].Type = 0x%02x, want 0x%02x", i, got.TLVs[i].Type, tt.msg.TLVs[i].Type)
				}
				if string(got.TLVs[i].Value) != string(tt.msg.TLVs[i].Value) {
					t.Errorf("TLV[%d].Value = %x, want %x", i, got.TLVs[i].Value, tt.msg.TLVs[i].Value)
				}
			}
		})
	}
}

func TestDecodeKnownFrame(t *testing.T) {
	t.Parallel()
	// A real CTL allocate-client-ID response (service=DMS, clientID=1).
	// QMUX: marker=01 len=0017(23=24-1) flags=80 svc=00 client=00
	// SDU:  flags=02 txn=01 msg=0022 tlvLen=000C
	// TLV 0x02: result success (00 00 00 00)
	// TLV 0x01: svc=02 client=01
	frame, _ := hex.DecodeString(
		"01" + // marker
			"1700" + // length = 23 (total frame 24, minus 1 for marker)
			"80" + // flags (response)
			"00" + // service CTL
			"00" + // client 0
			"02" + // SDU flags (response)
			"01" + // txn ID (8-bit for CTL)
			"2200" + // msg ID 0x0022
			"0c00" + // TLV length = 12
			"02" + "0400" + "00000000" + // result TLV: success
			"01" + "0200" + "0201", // TLV 0x01: service=DMS(2), clientID=1
	)

	msg, err := DecodeMessage(frame)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}
	if msg.Service != ServiceCTL {
		t.Errorf("Service = %d, want CTL(%d)", msg.Service, ServiceCTL)
	}
	if msg.TxnID != 1 {
		t.Errorf("TxnID = %d, want 1", msg.TxnID)
	}
	if msg.MsgID != 0x0022 {
		t.Errorf("MsgID = 0x%04x, want 0x0022", msg.MsgID)
	}
	if err := msg.Result(); err != nil {
		t.Errorf("Result() = %v, want nil", err)
	}

	// Check the allocation TLV: service=2, clientID=1.
	tlv := msg.FindTLV(0x01)
	if tlv == nil {
		t.Fatal("FindTLV(0x01) = nil")
	}
	if len(tlv.Value) != 2 || tlv.Value[0] != 0x02 || tlv.Value[1] != 0x01 {
		t.Errorf("TLV 0x01 value = %x, want 0201", tlv.Value)
	}
}

func TestDecodeErrorResponse(t *testing.T) {
	t.Parallel()
	// DMS response with QMI error 26 (0x001A = no-effect).
	// QMUX: marker=01 len=0013(19=20-1) flags=80 svc=02 client=01
	// SDU:  flags=02 txn=0100 msg=002D tlvLen=0004
	// TLV 0x02: result error (01 00 1A 00)
	frame, _ := hex.DecodeString(
		"01" +
			"1300" + // length = 19 (total frame 20, minus 1 for marker)
			"80" +
			"02" + // service DMS
			"01" + // client 1
			"02" + // SDU flags
			"0100" + // txn ID 1 (16-bit for DMS)
			"2d00" + // msg ID 0x002D
			"0700" + // TLV length = 7
			"02" + "0400" + "01001a00", // result: error, code=0x001A (no-effect)
	)

	msg, err := DecodeMessage(frame)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	err = msg.Result()
	if err == nil {
		t.Fatal("Result() = nil, want error")
	}
	if !errors.Is(err, ErrNoEffect) {
		t.Errorf("Result() = %v, want ErrNoEffect", err)
	}
}

func TestDecodeInvalidFrames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		frame []byte
	}{
		{"empty", nil},
		{"too short", []byte{0x01, 0x05, 0x00}},
		{"wrong marker", []byte{0x02, 0x06, 0x00, 0x00, 0x00, 0x00}},
		{"length exceeds buffer", []byte{0x01, 0xFF, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeMessage(tt.frame)
			if err == nil {
				t.Error("DecodeMessage() = nil error, want error")
			}
		})
	}
}

func TestFindTLV(t *testing.T) {
	t.Parallel()
	msg := &Message{
		TLVs: []TLV{
			{Type: 0x01, Value: []byte{0xAA}},
			{Type: 0x02, Value: []byte{0xBB, 0xCC}},
			{Type: 0x10, Value: []byte{0xDD}},
		},
	}

	if tlv := msg.FindTLV(0x01); tlv == nil || tlv.Value[0] != 0xAA {
		t.Errorf("FindTLV(0x01) = %v, want {0xAA}", tlv)
	}
	if tlv := msg.FindTLV(0x02); tlv == nil || len(tlv.Value) != 2 {
		t.Errorf("FindTLV(0x02) = %v, want 2-byte value", tlv)
	}
	if tlv := msg.FindTLV(0xFF); tlv != nil {
		t.Errorf("FindTLV(0xFF) = %v, want nil", tlv)
	}
}

func TestResultMissingTLV(t *testing.T) {
	t.Parallel()
	msg := &Message{}
	if err := msg.Result(); err == nil {
		t.Error("Result() = nil, want error for missing TLV")
	}
}

func TestTLVHelpers(t *testing.T) {
	t.Parallel()

	t.Run("U8", func(t *testing.T) {
		t.Parallel()
		tlv := EncodeTLVU8(0x01, 42)
		val, err := DecodeTLVU8(&tlv)
		if err != nil {
			t.Fatalf("DecodeTLVU8: %v", err)
		}
		if val != 42 {
			t.Errorf("val = %d, want 42", val)
		}
	})

	t.Run("U16", func(t *testing.T) {
		t.Parallel()
		tlv := EncodeTLVU16(0x01, 0xBEEF)
		val, err := DecodeTLVU16(&tlv)
		if err != nil {
			t.Fatalf("DecodeTLVU16: %v", err)
		}
		if val != 0xBEEF {
			t.Errorf("val = 0x%04x, want 0xBEEF", val)
		}
	})

	t.Run("U32", func(t *testing.T) {
		t.Parallel()
		tlv := EncodeTLVU32(0x01, 0xDEADBEEF)
		val, err := DecodeTLVU32(&tlv)
		if err != nil {
			t.Fatalf("DecodeTLVU32: %v", err)
		}
		if val != 0xDEADBEEF {
			t.Errorf("val = 0x%08x, want 0xDEADBEEF", val)
		}
	})

	t.Run("U8 too short", func(t *testing.T) {
		t.Parallel()
		tlv := TLV{Type: 0x01, Value: nil}
		_, err := DecodeTLVU8(&tlv)
		if err == nil {
			t.Error("DecodeTLVU8(nil) = nil, want error")
		}
	})

	t.Run("U16 too short", func(t *testing.T) {
		t.Parallel()
		tlv := TLV{Type: 0x01, Value: []byte{0x01}}
		_, err := DecodeTLVU16(&tlv)
		if err == nil {
			t.Error("DecodeTLVU16(1 byte) = nil, want error")
		}
	})

	t.Run("U32 too short", func(t *testing.T) {
		t.Parallel()
		tlv := TLV{Type: 0x01, Value: []byte{0x01, 0x02}}
		_, err := DecodeTLVU32(&tlv)
		if err == nil {
			t.Error("DecodeTLVU32(2 bytes) = nil, want error")
		}
	})
}

func TestQMIErrorSentinels(t *testing.T) {
	t.Parallel()
	err := newQMIError(5)
	if !errors.Is(err, ErrClientIdsExhausted) {
		t.Errorf("newQMIError(5) should match ErrClientIdsExhausted")
	}

	err = newQMIError(0xFFFF)
	if errors.Is(err, ErrClientIdsExhausted) {
		t.Errorf("newQMIError(0xFFFF) should not match ErrClientIdsExhausted")
	}
	if err.Error() != "QMI error 65535" {
		t.Errorf("Error() = %q, want \"QMI error 65535\"", err.Error())
	}
}

func TestIsQMIError(t *testing.T) {
	t.Parallel()
	if !IsQMIError(ErrClientIdsExhausted, 5) {
		t.Error("IsQMIError(ErrClientIdsExhausted, 5) = false, want true")
	}
	if IsQMIError(ErrClientIdsExhausted, 1) {
		t.Error("IsQMIError(ErrClientIdsExhausted, 1) = true, want false")
	}
	if IsQMIError(errors.New("other"), 5) {
		t.Error("IsQMIError(non-QMI error) = true, want false")
	}
}
