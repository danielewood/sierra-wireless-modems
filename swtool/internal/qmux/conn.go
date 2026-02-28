package qmux

import (
	"fmt"
	"sync"
)

// Conn manages a QMI connection over a transport. It handles client ID
// allocation, transaction ID sequencing, and message send/receive.
//
// Conn is safe for sequential use. The modem processes one request per
// service at a time, so callers should not send concurrent requests.
type Conn struct {
	transport Transport

	mu       sync.Mutex
	clients  map[ServiceID]uint8 // service → allocated client ID
	ctlTxn   uint8               // 8-bit CTL transaction counter
	svcTxn   uint16              // 16-bit service transaction counter
	closed   bool
}

// Open creates a new QMI connection to the given device path.
// It auto-detects the transport type (MBIM vs raw QMI).
func Open(devicePath string) (*Conn, error) {
	t, err := OpenTransport(devicePath)
	if err != nil {
		return nil, err
	}
	return NewConn(t), nil
}

// NewConn creates a Conn from an existing transport. Useful for testing
// with fake transports.
func NewConn(t Transport) *Conn {
	return &Conn{
		transport: t,
		clients:   make(map[ServiceID]uint8),
	}
}

// Send sends a QMI request to the given service and returns the response.
// Client IDs are automatically allocated on first use per service.
func (c *Conn) Send(svc ServiceID, msgID uint16, tlvs ...TLV) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("connection closed")
	}

	// Get or allocate a client ID for this service.
	clientID, err := c.getClientID(svc)
	if err != nil {
		return nil, err
	}

	// Build and send the request.
	c.svcTxn++
	msg := &Message{
		Service: svc,
		Client:  clientID,
		TxnID:   c.svcTxn,
		MsgID:   msgID,
		TLVs:    tlvs,
	}

	frame, err := EncodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("encoding message: %w", err)
	}

	if err := c.transport.Send(frame); err != nil {
		return nil, fmt.Errorf("sending message: %w", err)
	}

	// Read response. Loop until we get a matching response (skip indications).
	for {
		respFrame, err := c.transport.Receive()
		if err != nil {
			return nil, fmt.Errorf("receiving response: %w", err)
		}

		resp, err := DecodeMessage(respFrame)
		if err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		// Match by service and transaction ID.
		if resp.Service == svc && resp.TxnID == c.svcTxn {
			return resp, nil
		}
		// Otherwise it's an indication or stale response — skip it.
	}
}

// sendCTL sends a CTL service message (8-bit txn ID, client 0).
// Must be called with c.mu held.
func (c *Conn) sendCTL(msgID uint16, tlvs ...TLV) (*Message, error) {
	c.ctlTxn++
	msg := &Message{
		Service: ServiceCTL,
		Client:  0x00,
		TxnID:   uint16(c.ctlTxn),
		MsgID:   msgID,
		TLVs:    tlvs,
	}

	frame, err := EncodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("encoding CTL message: %w", err)
	}

	if err := c.transport.Send(frame); err != nil {
		return nil, fmt.Errorf("sending CTL message: %w", err)
	}

	for {
		respFrame, err := c.transport.Receive()
		if err != nil {
			return nil, fmt.Errorf("receiving CTL response: %w", err)
		}

		resp, err := DecodeMessage(respFrame)
		if err != nil {
			return nil, fmt.Errorf("decoding CTL response: %w", err)
		}

		if resp.Service == ServiceCTL && resp.TxnID == uint16(c.ctlTxn) {
			return resp, nil
		}
	}
}

// getClientID returns the allocated client ID for svc, allocating one
// if needed. Must be called with c.mu held.
func (c *Conn) getClientID(svc ServiceID) (uint8, error) {
	if id, ok := c.clients[svc]; ok {
		return id, nil
	}

	id, err := allocateClientID(c, svc)
	if err != nil {
		return 0, err
	}
	c.clients[svc] = id
	return id, nil
}

// Close releases all allocated client IDs and closes the transport.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Best-effort release of all allocated client IDs.
	for svc, id := range c.clients {
		_ = releaseClientID(c, svc, id)
	}

	return c.transport.Close()
}
