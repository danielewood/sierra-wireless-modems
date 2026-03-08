// Package mbim provides a native MBIM client for querying modem state
// via the MBIM protocol over /dev/cdc-wdm* devices.
package mbim

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
)

// MBIM message types.
const (
	msgOpen        uint32 = 0x00000001
	msgClose       uint32 = 0x00000002
	msgCommand     uint32 = 0x00000003
	msgOpenDone    uint32 = 0x80000001
	msgCommandDone uint32 = 0x80000003
)

// MBIM command types.
const (
	cmdQuery uint32 = 0
	cmdSet   uint32 = 1
)

// MBIM sizes.
const (
	maxTransfer      = 512
	headerLen        = 12 // MessageType(4) + MessageLength(4) + TransactionId(4)
	openLen          = headerLen + 4
	commandHeaderLen = headerLen + 36
	readTimeout      = 5 * time.Second
)

// Basic Connect service UUID (big-endian wire order).
//
//	a289cc33-bcbb-8b4f-b6b0-133ec2aae6df
var basicConnectUUID = [16]byte{
	0xa2, 0x89, 0xcc, 0x33,
	0xbc, 0xbb,
	0x8b, 0x4f,
	0xb6, 0xb0,
	0x13, 0x3e, 0xc2, 0xaa, 0xe6, 0xdf,
}

// Basic Connect CIDs.
const (
	cidDeviceCaps            uint32 = 1
	cidSubscriberReadyStatus uint32 = 2
	cidRegisterState         uint32 = 9
	cidSignalState           uint32 = 11
)

// Client is a native MBIM client that queries modem state.
type Client struct {
	f     *os.File
	txnID uint32
	log   *log.Logger
}

// NewClient opens an MBIM session on the given cdc-wdm device.
func NewClient(devicePath string, l *log.Logger) (*Client, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR|syscall.O_EXCL|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("opening MBIM device %s: %w", devicePath, err)
	}
	// Clear O_NONBLOCK after open — we want blocking reads with deadline.
	if err := syscall.SetNonblock(int(f.Fd()), false); err != nil {
		f.Close()
		return nil, fmt.Errorf("clearing O_NONBLOCK: %w", err)
	}

	c := &Client{f: f, log: l}

	if err := c.open(); err != nil {
		f.Close()
		return nil, err
	}

	return c, nil
}

// Close sends MBIM_CLOSE and closes the device.
func (c *Client) Close() error {
	msg := make([]byte, headerLen)
	binary.LittleEndian.PutUint32(msg[0:4], msgClose)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(headerLen))
	binary.LittleEndian.PutUint32(msg[8:12], c.nextTxn())
	c.f.Write(msg) // best-effort
	return c.f.Close()
}

func (c *Client) open() error {
	msg := make([]byte, openLen)
	binary.LittleEndian.PutUint32(msg[0:4], msgOpen)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(openLen))
	binary.LittleEndian.PutUint32(msg[8:12], c.nextTxn())
	binary.LittleEndian.PutUint32(msg[12:16], maxTransfer)

	if _, err := c.f.Write(msg); err != nil {
		return fmt.Errorf("sending MBIM OPEN: %w", err)
	}

	resp, err := c.readMessage()
	if err != nil {
		return fmt.Errorf("reading MBIM OPEN response: %w", err)
	}

	if len(resp) < headerLen {
		return fmt.Errorf("MBIM OPEN response too short: %d bytes", len(resp))
	}

	msgType := binary.LittleEndian.Uint32(resp[0:4])

	// If we get CLOSE_DONE, a stale session was cleaned up. Retry OPEN.
	if msgType == msgCloseDone {
		c.log.Debugf("MBIM got CLOSE_DONE (stale session), retrying OPEN")
		binary.LittleEndian.PutUint32(msg[8:12], c.nextTxn())
		if _, err := c.f.Write(msg); err != nil {
			return fmt.Errorf("sending MBIM OPEN (retry): %w", err)
		}
		resp, err = c.readMessage()
		if err != nil {
			return fmt.Errorf("reading MBIM OPEN response (retry): %w", err)
		}
		if len(resp) < headerLen {
			return fmt.Errorf("MBIM OPEN response too short: %d bytes", len(resp))
		}
		msgType = binary.LittleEndian.Uint32(resp[0:4])
	}

	if msgType != msgOpenDone {
		return fmt.Errorf("expected MBIM OPEN_DONE, got 0x%08x", msgType)
	}
	if len(resp) >= 16 {
		if status := binary.LittleEndian.Uint32(resp[12:16]); status != 0 {
			return fmt.Errorf("MBIM OPEN failed with status %d", status)
		}
	}

	return nil
}

const msgCloseDone uint32 = 0x80000002

// query sends an MBIM COMMAND (QUERY) and returns the InformationBuffer.
func (c *Client) query(uuid [16]byte, cid uint32) ([]byte, error) {
	msgLen := commandHeaderLen
	msg := make([]byte, msgLen)

	binary.LittleEndian.PutUint32(msg[0:4], msgCommand)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(msgLen))
	binary.LittleEndian.PutUint32(msg[8:12], c.nextTxn())

	off := headerLen
	binary.LittleEndian.PutUint32(msg[off:off+4], 1) // TotalFragments
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], 0) // CurrentFragment
	off += 4
	copy(msg[off:off+16], uuid[:])
	off += 16
	binary.LittleEndian.PutUint32(msg[off:off+4], cid)
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], cmdQuery)
	off += 4
	binary.LittleEndian.PutUint32(msg[off:off+4], 0) // InformationBufferLength = 0

	c.log.Debugf("MBIM>>> CID=%d", cid)

	if _, err := c.f.Write(msg); err != nil {
		return nil, fmt.Errorf("sending MBIM COMMAND: %w", err)
	}

	// Read responses until we get COMMAND_DONE.
	for {
		resp, err := c.readMessage()
		if err != nil {
			return nil, fmt.Errorf("reading MBIM response: %w", err)
		}
		if len(resp) < headerLen {
			return nil, fmt.Errorf("MBIM response too short: %d bytes", len(resp))
		}

		msgType := binary.LittleEndian.Uint32(resp[0:4])
		if msgType != msgCommandDone {
			continue // skip indications
		}

		if len(resp) < commandHeaderLen {
			return nil, fmt.Errorf("MBIM COMMAND_DONE too short: %d bytes", len(resp))
		}

		// Check status (at offset headerLen + 8 + 16 + 4 = headerLen + 28)
		statusOff := headerLen + 8 + 16 + 4
		status := binary.LittleEndian.Uint32(resp[statusOff : statusOff+4])
		if status != 0 {
			return nil, fmt.Errorf("MBIM CID %d returned status %d", cid, status)
		}

		// Extract InformationBuffer.
		infoLenOff := statusOff + 4
		infoLen := binary.LittleEndian.Uint32(resp[infoLenOff : infoLenOff+4])
		dataOff := infoLenOff + 4

		if uint32(len(resp)-dataOff) < infoLen {
			return nil, fmt.Errorf("MBIM COMMAND_DONE payload truncated")
		}

		c.log.Debugf("MBIM<<< CID=%d status=0 len=%d", cid, infoLen)
		return resp[dataOff : dataOff+int(infoLen)], nil
	}
}

func (c *Client) nextTxn() uint32 {
	c.txnID++
	return c.txnID
}

func (c *Client) readMessage() ([]byte, error) {
	buf := make([]byte, maxTransfer)
	if err := c.f.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		// Not all file types support deadlines; fall through.
	}
	n, err := c.f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// readUTF16String extracts a UTF-16LE string from buf at the given offset and size.
func readUTF16String(buf []byte, offset, size uint32) string {
	if size == 0 || int(offset+size) > len(buf) {
		return ""
	}
	data := buf[offset : offset+size]
	// Decode UTF-16LE pairs.
	u16 := make([]uint16, len(data)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	// Trim null terminators.
	for len(u16) > 0 && u16[len(u16)-1] == 0 {
		u16 = u16[:len(u16)-1]
	}
	return string(utf16.Decode(u16))
}

// u32 reads a little-endian uint32 at offset, with bounds check.
func u32(buf []byte, off int) uint32 {
	if off+4 > len(buf) {
		return 0
	}
	return binary.LittleEndian.Uint32(buf[off : off+4])
}
