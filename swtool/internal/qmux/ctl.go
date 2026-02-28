package qmux

import "fmt"

// CTL service message IDs.
const (
	ctlMsgAllocateCID uint16 = 0x0022
	ctlMsgReleaseCID  uint16 = 0x0023
)

// allocateClientID requests a new client ID for the given service
// via CTL message 0x0022.
func allocateClientID(c *Conn, svc ServiceID) (uint8, error) {
	resp, err := c.sendCTL(ctlMsgAllocateCID, EncodeTLVU8(0x01, byte(svc)))
	if err != nil {
		return 0, fmt.Errorf("allocating client ID for service 0x%02x: %w", svc, err)
	}
	if err := resp.Result(); err != nil {
		return 0, fmt.Errorf("allocating client ID for service 0x%02x: %w", svc, err)
	}

	// TLV 0x01: service(1) + clientID(1)
	tlv := resp.FindTLV(0x01)
	if tlv == nil || len(tlv.Value) < 2 {
		return 0, fmt.Errorf("allocate CID response missing allocation TLV")
	}
	return tlv.Value[1], nil
}

// releaseClientID releases a previously allocated client ID via
// CTL message 0x0023.
func releaseClientID(c *Conn, svc ServiceID, clientID uint8) error {
	tlv := TLV{Type: 0x01, Value: []byte{byte(svc), clientID}}
	resp, err := c.sendCTL(ctlMsgReleaseCID, tlv)
	if err != nil {
		return fmt.Errorf("releasing client ID %d for service 0x%02x: %w", clientID, svc, err)
	}
	if err := resp.Result(); err != nil {
		return fmt.Errorf("releasing client ID %d for service 0x%02x: %w", clientID, svc, err)
	}
	return nil
}
