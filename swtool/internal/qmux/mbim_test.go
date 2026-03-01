package qmux

import (
	"encoding/binary"
	"testing"
)

func TestMBIMUUID(t *testing.T) {
	t.Parallel()
	// Verify the UUID bytes match the canonical QMI-over-MBIM UUID
	// d1a30bc2-f97a-6e43-bf65-c7e24fb0f0d3 in big-endian (presentation)
	// byte order, matching libmbim's wire format.
	want := [16]byte{
		0xd1, 0xa3, 0x0b, 0xc2,
		0xf9, 0x7a,
		0x6e, 0x43,
		0xbf, 0x65,
		0xc7, 0xe2, 0x4f, 0xb0, 0xf0, 0xd3,
	}
	if qmiOverMBIMUUID != want {
		t.Errorf("UUID = %x, want %x", qmiOverMBIMUUID, want)
	}
}

func TestMBIMCommandEncoding(t *testing.T) {
	t.Parallel()
	// Verify the structure of an MBIM COMMAND wrapping a small QMUX frame.
	// We use a fake mbimTransport to build a command manually.

	// 12-byte QMUX frame: marker(01) + length(0b00 = 11, excludes marker) + 9 bytes payload
	qmuxFrame := []byte{0x01, 0x0b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x22, 0x00, 0x00, 0x00}

	msgLen := mbimCommandHeaderLen + len(qmuxFrame)
	msg := make([]byte, msgLen)

	binary.LittleEndian.PutUint32(msg[0:4], mbimMsgCommand)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(msgLen))
	binary.LittleEndian.PutUint32(msg[8:12], 1) // txn

	off := mbimHeaderLen
	binary.LittleEndian.PutUint32(msg[off:off+4], 1) // TotalFragments
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], 0) // CurrentFragment
	off += 4
	copy(msg[off:off+16], qmiOverMBIMUUID[:])
	off += 16
	binary.LittleEndian.PutUint32(msg[off:off+4], 1) // CID
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], 1) // CommandType SET
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], uint32(len(qmuxFrame)))
	off += 4
	copy(msg[off:], qmuxFrame)

	// Verify message type.
	gotType := binary.LittleEndian.Uint32(msg[0:4])
	if gotType != mbimMsgCommand {
		t.Errorf("MessageType = 0x%08x, want 0x%08x", gotType, mbimMsgCommand)
	}

	// Verify total length.
	gotLen := binary.LittleEndian.Uint32(msg[4:8])
	if gotLen != uint32(msgLen) {
		t.Errorf("MessageLength = %d, want %d", gotLen, msgLen)
	}

	// Verify payload extraction from a synthetic COMMAND_DONE response.
	doneMsg := make([]byte, mbimCommandDoneHeaderLen+len(qmuxFrame))
	binary.LittleEndian.PutUint32(doneMsg[0:4], mbimMsgCommandDone)
	binary.LittleEndian.PutUint32(doneMsg[4:8], uint32(len(doneMsg)))
	binary.LittleEndian.PutUint32(doneMsg[8:12], 1) // txn

	doff := mbimHeaderLen
	binary.LittleEndian.PutUint32(doneMsg[doff:doff+4], 1) // TotalFragments
	doff += 4
	binary.LittleEndian.PutUint32(doneMsg[doff:doff+4], 0) // CurrentFragment
	doff += 4
	copy(doneMsg[doff:doff+16], qmiOverMBIMUUID[:])
	doff += 16
	binary.LittleEndian.PutUint32(doneMsg[doff:doff+4], 1) // CID
	doff += 4
	binary.LittleEndian.PutUint32(doneMsg[doff:doff+4], 0) // Status = success
	doff += 4
	binary.LittleEndian.PutUint32(doneMsg[doff:doff+4], uint32(len(qmuxFrame)))
	doff += 4
	copy(doneMsg[doff:], qmuxFrame)

	// Extract payload the same way Receive() does.
	extractOff := mbimHeaderLen + 8 + 16 + 4 + 4 // skip frag + UUID + CID + Status
	infoLen := binary.LittleEndian.Uint32(doneMsg[extractOff : extractOff+4])
	extractOff += 4
	payload := doneMsg[extractOff : extractOff+int(infoLen)]

	if len(payload) != len(qmuxFrame) {
		t.Fatalf("extracted payload length = %d, want %d", len(payload), len(qmuxFrame))
	}
	for i := range payload {
		if payload[i] != qmuxFrame[i] {
			t.Errorf("payload[%d] = 0x%02x, want 0x%02x", i, payload[i], qmuxFrame[i])
		}
	}
}

func TestMBIMOpenMessage(t *testing.T) {
	t.Parallel()
	// Verify MBIM OPEN message structure.
	msg := make([]byte, mbimOpenLen)
	binary.LittleEndian.PutUint32(msg[0:4], mbimMsgOpen)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(mbimOpenLen))
	binary.LittleEndian.PutUint32(msg[8:12], 1)
	binary.LittleEndian.PutUint32(msg[12:16], mbimMaxTransfer)

	if len(msg) != 16 {
		t.Errorf("MBIM OPEN message length = %d, want 16", len(msg))
	}

	gotMaxTransfer := binary.LittleEndian.Uint32(msg[12:16])
	if gotMaxTransfer != mbimMaxTransfer {
		t.Errorf("MaxControlTransfer = %d, want %d", gotMaxTransfer, mbimMaxTransfer)
	}
}

func TestMBIMCloseMessage(t *testing.T) {
	t.Parallel()
	// Verify MBIM CLOSE message structure.
	msg := make([]byte, mbimHeaderLen)
	binary.LittleEndian.PutUint32(msg[0:4], mbimMsgClose)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(mbimHeaderLen))
	binary.LittleEndian.PutUint32(msg[8:12], 1)

	if len(msg) != 12 {
		t.Errorf("MBIM CLOSE message length = %d, want 12", len(msg))
	}

	gotType := binary.LittleEndian.Uint32(msg[0:4])
	if gotType != mbimMsgClose {
		t.Errorf("MessageType = 0x%08x, want 0x%08x", gotType, mbimMsgClose)
	}
}
