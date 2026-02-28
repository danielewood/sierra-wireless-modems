// Package qmux implements the QMI multiplexing wire protocol, transports,
// and service message codecs for direct modem communication.
package qmux

import (
	"encoding/binary"
	"fmt"
)

// ServiceID identifies a QMI service.
type ServiceID uint8

const (
	ServiceCTL ServiceID = 0x00
	ServiceDMS ServiceID = 0x02
	ServiceNAS ServiceID = 0x03
)

// qmuxMarker is the first byte of every QMUX frame.
const qmuxMarker = 0x01

// QMUX frame layout (little-endian):
//
//	Byte 0:     marker (0x01)
//	Bytes 1-2:  total length (includes marker)
//	Byte 3:     flags (0x00 for request, 0x80 for response)
//	Byte 4:     service ID
//	Byte 5:     client ID (0x00 for CTL)
//
// SDU (Service Data Unit) immediately follows:
//
//	CTL service (serviceID == 0x00):
//	  Byte 0:     SDU flags
//	  Byte 1:     transaction ID (8-bit!)
//	  Bytes 2-3:  message ID
//	  Bytes 4-5:  TLV total length
//
//	Other services:
//	  Byte 0:     SDU flags
//	  Bytes 1-2:  transaction ID (16-bit)
//	  Bytes 3-4:  message ID
//	  Bytes 5-6:  TLV total length

const (
	qmuxHeaderLen  = 6 // marker + length(2) + flags + serviceID + clientID
	ctlSDUHeaderLen = 6 // flags + txnID(1) + msgID(2) + tlvLen(2)
	svcSDUHeaderLen = 7 // flags + txnID(2) + msgID(2) + tlvLen(2)
	tlvHeaderLen   = 3 // type(1) + length(2)
)

// TLV represents a Type-Length-Value element in a QMI message.
type TLV struct {
	Type  uint8
	Value []byte
}

// Message represents a decoded QMUX message.
type Message struct {
	Service ServiceID
	Client  uint8
	TxnID   uint16
	MsgID   uint16
	TLVs    []TLV
}

// FindTLV returns the first TLV with the given type, or nil if not found.
func (m *Message) FindTLV(typ uint8) *TLV {
	for i := range m.TLVs {
		if m.TLVs[i].Type == typ {
			return &m.TLVs[i]
		}
	}
	return nil
}

// Result checks the mandatory result TLV (type 0x02) present in every
// QMI response. Returns nil if the operation succeeded, or a *QMIError
// describing the failure.
func (m *Message) Result() error {
	tlv := m.FindTLV(0x02)
	if tlv == nil {
		return fmt.Errorf("missing result TLV in response")
	}
	if len(tlv.Value) < 4 {
		return fmt.Errorf("result TLV too short: %d bytes", len(tlv.Value))
	}
	status := binary.LittleEndian.Uint16(tlv.Value[0:2])
	code := binary.LittleEndian.Uint16(tlv.Value[2:4])
	if status != 0 {
		return newQMIError(code)
	}
	return nil
}

// EncodeMessage serializes a Message into a QMUX frame.
func EncodeMessage(msg *Message) ([]byte, error) {
	// Calculate TLV payload size.
	tlvLen := 0
	for _, tlv := range msg.TLVs {
		tlvLen += tlvHeaderLen + len(tlv.Value)
	}

	sduHeaderLen := svcSDUHeaderLen
	if msg.Service == ServiceCTL {
		sduHeaderLen = ctlSDUHeaderLen
	}

	totalLen := qmuxHeaderLen + sduHeaderLen + tlvLen
	buf := make([]byte, totalLen)

	// QMUX header.
	buf[0] = qmuxMarker
	binary.LittleEndian.PutUint16(buf[1:3], uint16(totalLen))
	buf[3] = 0x00 // flags: request
	buf[4] = byte(msg.Service)
	buf[5] = msg.Client

	// SDU header.
	off := qmuxHeaderLen
	buf[off] = 0x00 // SDU flags: request
	off++

	if msg.Service == ServiceCTL {
		buf[off] = byte(msg.TxnID)
		off++
	} else {
		binary.LittleEndian.PutUint16(buf[off:off+2], msg.TxnID)
		off += 2
	}

	binary.LittleEndian.PutUint16(buf[off:off+2], msg.MsgID)
	off += 2
	binary.LittleEndian.PutUint16(buf[off:off+2], uint16(tlvLen))
	off += 2

	// TLV payload.
	for _, tlv := range msg.TLVs {
		buf[off] = tlv.Type
		binary.LittleEndian.PutUint16(buf[off+1:off+3], uint16(len(tlv.Value)))
		copy(buf[off+3:], tlv.Value)
		off += tlvHeaderLen + len(tlv.Value)
	}

	return buf, nil
}

// DecodeMessage deserializes a QMUX frame into a Message.
func DecodeMessage(frame []byte) (*Message, error) {
	if len(frame) < qmuxHeaderLen {
		return nil, fmt.Errorf("frame too short: %d bytes (need at least %d)", len(frame), qmuxHeaderLen)
	}
	if frame[0] != qmuxMarker {
		return nil, fmt.Errorf("invalid QMUX marker: 0x%02x", frame[0])
	}

	frameLen := int(binary.LittleEndian.Uint16(frame[1:3]))
	if frameLen > len(frame) {
		return nil, fmt.Errorf("frame length %d exceeds buffer %d", frameLen, len(frame))
	}
	// Use only the bytes the frame declares.
	frame = frame[:frameLen]

	msg := &Message{
		Service: ServiceID(frame[4]),
		Client:  frame[5],
	}

	off := qmuxHeaderLen

	// Determine SDU header size based on service.
	sduHeaderLen := svcSDUHeaderLen
	if msg.Service == ServiceCTL {
		sduHeaderLen = ctlSDUHeaderLen
	}

	if len(frame) < off+sduHeaderLen {
		return nil, fmt.Errorf("frame too short for SDU header: %d bytes", len(frame))
	}

	// Skip SDU flags byte.
	off++

	// Transaction ID.
	if msg.Service == ServiceCTL {
		msg.TxnID = uint16(frame[off])
		off++
	} else {
		msg.TxnID = binary.LittleEndian.Uint16(frame[off : off+2])
		off += 2
	}

	// Message ID.
	msg.MsgID = binary.LittleEndian.Uint16(frame[off : off+2])
	off += 2

	// TLV total length.
	tlvLen := int(binary.LittleEndian.Uint16(frame[off : off+2]))
	off += 2

	// Sanity check.
	if off+tlvLen > len(frame) {
		return nil, fmt.Errorf("TLV length %d exceeds remaining frame (%d bytes)", tlvLen, len(frame)-off)
	}

	// Decode TLVs.
	end := off + tlvLen
	for off < end {
		if off+tlvHeaderLen > end {
			return nil, fmt.Errorf("truncated TLV header at offset %d", off)
		}
		typ := frame[off]
		vlen := int(binary.LittleEndian.Uint16(frame[off+1 : off+3]))
		off += tlvHeaderLen

		if off+vlen > end {
			return nil, fmt.Errorf("TLV type 0x%02x length %d exceeds remaining data", typ, vlen)
		}

		value := make([]byte, vlen)
		copy(value, frame[off:off+vlen])
		msg.TLVs = append(msg.TLVs, TLV{Type: typ, Value: value})
		off += vlen
	}

	return msg, nil
}

// EncodeTLVU8 creates a TLV with a single uint8 value.
func EncodeTLVU8(typ uint8, val uint8) TLV {
	return TLV{Type: typ, Value: []byte{val}}
}

// EncodeTLVU16 creates a TLV with a uint16 value (little-endian).
func EncodeTLVU16(typ uint8, val uint16) TLV {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, val)
	return TLV{Type: typ, Value: b}
}

// EncodeTLVU32 creates a TLV with a uint32 value (little-endian).
func EncodeTLVU32(typ uint8, val uint32) TLV {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, val)
	return TLV{Type: typ, Value: b}
}

// DecodeTLVU8 reads a single uint8 from a TLV value.
func DecodeTLVU8(tlv *TLV) (uint8, error) {
	if len(tlv.Value) < 1 {
		return 0, fmt.Errorf("TLV 0x%02x too short for uint8: %d bytes", tlv.Type, len(tlv.Value))
	}
	return tlv.Value[0], nil
}

// DecodeTLVU16 reads a uint16 from a TLV value (little-endian).
func DecodeTLVU16(tlv *TLV) (uint16, error) {
	if len(tlv.Value) < 2 {
		return 0, fmt.Errorf("TLV 0x%02x too short for uint16: %d bytes", tlv.Type, len(tlv.Value))
	}
	return binary.LittleEndian.Uint16(tlv.Value[0:2]), nil
}

// DecodeTLVU32 reads a uint32 from a TLV value (little-endian).
func DecodeTLVU32(tlv *TLV) (uint32, error) {
	if len(tlv.Value) < 4 {
		return 0, fmt.Errorf("TLV 0x%02x too short for uint32: %d bytes", tlv.Type, len(tlv.Value))
	}
	return binary.LittleEndian.Uint32(tlv.Value[0:4]), nil
}
