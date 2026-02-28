package qmux

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// MBIM message types.
const (
	mbimMsgOpen        uint32 = 0x00000001
	mbimMsgClose       uint32 = 0x00000002
	mbimMsgCommand     uint32 = 0x00000003
	mbimMsgOpenDone    uint32 = 0x80000001
	mbimMsgCloseDone   uint32 = 0x80000002
	mbimMsgCommandDone uint32 = 0x80000003
)

// MBIM constants.
const (
	mbimMaxTransfer = 4096

	// MBIM header: MessageType(4) + MessageLength(4) + TransactionId(4)
	mbimHeaderLen = 12

	// MBIM OPEN body: MaxControlTransfer(4)
	mbimOpenLen = mbimHeaderLen + 4

	// MBIM COMMAND fragment header after the 12-byte common header:
	//   TotalFragments(4) + CurrentFragment(4) +
	//   DeviceServiceId(16) + CID(4) + CommandType(4) +
	//   InformationBufferLength(4)
	mbimCommandHeaderLen = mbimHeaderLen + 36

	// MBIM COMMAND_DONE body has same layout as COMMAND but with Status(4) instead of CommandType(4)
	mbimCommandDoneHeaderLen = mbimHeaderLen + 36
)

// qmiOverMBIMUUID is the QMI-over-MBIM device service UUID in MBIM
// mixed-endian wire format:
//
//	UUID: d1a30bc2-f97a-6e43-bf65-c7e24fb0f0d3
//	Wire: c2 0b a3 d1 7a f9 43 6e bf 65 c7 e2 4f b0 f0 d3
//
// MBIM UUIDs use mixed-endian: first 3 groups are little-endian,
// last 2 groups are big-endian (inherited from Microsoft COM).
var qmiOverMBIMUUID = [16]byte{
	0xc2, 0x0b, 0xa3, 0xd1, // d1a30bc2 reversed
	0x7a, 0xf9,             // f97a reversed
	0x43, 0x6e,             // 6e43 reversed
	0xbf, 0x65,             // bf65 (big-endian)
	0xc7, 0xe2, 0x4f, 0xb0, 0xf0, 0xd3, // c7e24fb0f0d3 (big-endian)
}

// mbimTransport wraps QMUX frames in MBIM COMMAND messages for
// /dev/cdc-wdm* devices.
type mbimTransport struct {
	f     *os.File
	txnID uint32 // MBIM transaction counter
}

func openMBIMTransport(devicePath string) (*mbimTransport, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening MBIM device %s: %w", devicePath, err)
	}

	t := &mbimTransport{f: f, txnID: 0}

	if err := t.open(); err != nil {
		f.Close()
		return nil, err
	}

	return t, nil
}

// open sends MBIM_OPEN and waits for MBIM_OPEN_DONE.
func (t *mbimTransport) open() error {
	msg := make([]byte, mbimOpenLen)
	binary.LittleEndian.PutUint32(msg[0:4], mbimMsgOpen)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(mbimOpenLen))
	binary.LittleEndian.PutUint32(msg[8:12], t.nextTxn())
	binary.LittleEndian.PutUint32(msg[12:16], mbimMaxTransfer)

	if _, err := t.f.Write(msg); err != nil {
		return fmt.Errorf("sending MBIM OPEN: %w", err)
	}

	resp, err := t.readMessage()
	if err != nil {
		return fmt.Errorf("reading MBIM OPEN response: %w", err)
	}

	if len(resp) < mbimHeaderLen {
		return fmt.Errorf("MBIM OPEN response too short: %d bytes", len(resp))
	}
	msgType := binary.LittleEndian.Uint32(resp[0:4])
	if msgType != mbimMsgOpenDone {
		return fmt.Errorf("expected MBIM OPEN_DONE (0x%08x), got 0x%08x", mbimMsgOpenDone, msgType)
	}

	// OPEN_DONE has a status field at offset 12.
	if len(resp) >= 16 {
		status := binary.LittleEndian.Uint32(resp[12:16])
		if status != 0 {
			return fmt.Errorf("MBIM OPEN failed with status %d", status)
		}
	}

	return nil
}

// Send wraps a QMUX frame in an MBIM COMMAND message and writes it.
func (t *mbimTransport) Send(frame []byte) error {
	msgLen := mbimCommandHeaderLen + len(frame)
	msg := make([]byte, msgLen)

	// Common header.
	binary.LittleEndian.PutUint32(msg[0:4], mbimMsgCommand)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(msgLen))
	binary.LittleEndian.PutUint32(msg[8:12], t.nextTxn())

	off := mbimHeaderLen

	// Fragment header: single fragment.
	binary.LittleEndian.PutUint32(msg[off:off+4], 1) // TotalFragments
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], 0) // CurrentFragment
	off += 4

	// Device service UUID.
	copy(msg[off:off+16], qmiOverMBIMUUID[:])
	off += 16

	// CID = 1 (QMI-over-MBIM uses CID 1).
	binary.LittleEndian.PutUint32(msg[off:off+4], 1)
	off += 4

	// CommandType = 1 (SET).
	binary.LittleEndian.PutUint32(msg[off:off+4], 1)
	off += 4

	// InformationBufferLength.
	binary.LittleEndian.PutUint32(msg[off:off+4], uint32(len(frame)))
	off += 4

	// Payload = QMUX frame.
	copy(msg[off:], frame)

	if _, err := t.f.Write(msg); err != nil {
		return fmt.Errorf("sending MBIM COMMAND: %w", err)
	}
	return nil
}

// Receive reads an MBIM COMMAND_DONE and extracts the QMUX frame payload.
func (t *mbimTransport) Receive() ([]byte, error) {
	for {
		resp, err := t.readMessage()
		if err != nil {
			return nil, fmt.Errorf("reading MBIM response: %w", err)
		}
		if len(resp) < mbimHeaderLen {
			return nil, fmt.Errorf("MBIM response too short: %d bytes", len(resp))
		}

		msgType := binary.LittleEndian.Uint32(resp[0:4])

		// Skip unsolicited indications — we only want COMMAND_DONE.
		if msgType != mbimMsgCommandDone {
			continue
		}

		if len(resp) < mbimCommandDoneHeaderLen {
			return nil, fmt.Errorf("MBIM COMMAND_DONE too short: %d bytes", len(resp))
		}

		// Extract InformationBufferLength at the same offset as COMMAND.
		off := mbimHeaderLen + 8 + 16 + 4 + 4 // skip frag + UUID + CID + Status
		infoLen := binary.LittleEndian.Uint32(resp[off : off+4])
		off += 4

		if uint32(len(resp)-off) < infoLen {
			return nil, fmt.Errorf("MBIM COMMAND_DONE payload truncated: need %d, have %d", infoLen, len(resp)-off)
		}

		return resp[off : off+int(infoLen)], nil
	}
}

// Close sends MBIM_CLOSE and closes the file.
func (t *mbimTransport) Close() error {
	msg := make([]byte, mbimHeaderLen)
	binary.LittleEndian.PutUint32(msg[0:4], mbimMsgClose)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(mbimHeaderLen))
	binary.LittleEndian.PutUint32(msg[8:12], t.nextTxn())

	// Best-effort: send close, ignore errors.
	t.f.Write(msg)

	return t.f.Close()
}

func (t *mbimTransport) nextTxn() uint32 {
	t.txnID++
	return t.txnID
}

func (t *mbimTransport) readMessage() ([]byte, error) {
	buf := make([]byte, mbimMaxTransfer)

	if err := t.f.SetReadDeadline(time.Now().Add(DefaultReadTimeout)); err != nil {
		// Not all file types support deadlines; fall through.
	}

	n, err := t.f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}
