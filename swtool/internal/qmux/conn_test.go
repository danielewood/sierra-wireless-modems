package qmux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// fakeTransport implements Transport for testing. It records sent frames
// and returns pre-configured response frames.
type fakeTransport struct {
	mu        sync.Mutex
	sent      [][]byte         // frames sent to device
	responses [][]byte         // queued responses to return
	closed    bool
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{}
}

func (f *fakeTransport) Send(frame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return fmt.Errorf("transport closed")
	}
	cp := make([]byte, len(frame))
	copy(cp, frame)
	f.sent = append(f.sent, cp)
	return nil
}

func (f *fakeTransport) Receive() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, fmt.Errorf("transport closed")
	}
	if len(f.responses) == 0 {
		return nil, fmt.Errorf("no responses queued")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// queueResponse encodes a response Message and adds it to the queue.
func (f *fakeTransport) queueResponse(msg *Message) {
	frame, _ := EncodeMessage(msg)
	// Set response flag in QMUX header.
	frame[3] = 0x80
	// Set response flag in SDU.
	frame[qmuxHeaderLen] = 0x02
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses = append(f.responses, frame)
}

// successResult returns a result TLV indicating success.
func successResult() TLV {
	return TLV{Type: 0x02, Value: []byte{0x00, 0x00, 0x00, 0x00}}
}

// errorResult returns a result TLV with the given QMI error code.
func errorResult(code uint16) TLV {
	v := make([]byte, 4)
	binary.LittleEndian.PutUint16(v[0:2], 1)    // status = failure
	binary.LittleEndian.PutUint16(v[2:4], code)  // error code
	return TLV{Type: 0x02, Value: v}
}

func TestConnSendReceive(t *testing.T) {
	t.Parallel()
	ft := newFakeTransport()

	// Queue CTL allocate-CID response for DMS (service=2, clientID=1).
	ft.queueResponse(&Message{
		Service: ServiceCTL,
		Client:  0,
		TxnID:   1,
		MsgID:   ctlMsgAllocateCID,
		TLVs: []TLV{
			successResult(),
			{Type: 0x01, Value: []byte{byte(ServiceDMS), 0x01}},
		},
	})

	// Queue DMS GetOperatingMode response.
	ft.queueResponse(&Message{
		Service: ServiceDMS,
		Client:  1,
		TxnID:   1,
		MsgID:   0x002D,
		TLVs: []TLV{
			successResult(),
			EncodeTLVU8(0x01, 0x00), // mode = online
		},
	})

	conn := NewConn(ft)
	defer conn.Close()

	resp, err := conn.Send(ServiceDMS, 0x002D)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := resp.Result(); err != nil {
		t.Fatalf("Result: %v", err)
	}

	tlv := resp.FindTLV(0x01)
	if tlv == nil {
		t.Fatal("missing TLV 0x01")
	}
	mode, err := DecodeTLVU8(tlv)
	if err != nil {
		t.Fatalf("DecodeTLVU8: %v", err)
	}
	if mode != 0x00 {
		t.Errorf("mode = 0x%02x, want 0x00 (online)", mode)
	}
}

func TestConnClientIDReuse(t *testing.T) {
	t.Parallel()
	ft := newFakeTransport()

	// First call: allocate CID.
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 1, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceDMS), 0x05}}},
	})
	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 5, TxnID: 1, MsgID: 0x002D,
		TLVs: []TLV{successResult(), EncodeTLVU8(0x01, 0x00)},
	})

	// Second call: no allocation needed (client 5 reused).
	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 5, TxnID: 2, MsgID: 0x002D,
		TLVs: []TLV{successResult(), EncodeTLVU8(0x01, 0x01)},
	})

	conn := NewConn(ft)
	defer conn.Close()

	// First send triggers allocation.
	_, err := conn.Send(ServiceDMS, 0x002D)
	if err != nil {
		t.Fatalf("first Send: %v", err)
	}

	// Second send reuses the client ID.
	resp, err := conn.Send(ServiceDMS, 0x002D)
	if err != nil {
		t.Fatalf("second Send: %v", err)
	}

	tlv := resp.FindTLV(0x01)
	mode, _ := DecodeTLVU8(tlv)
	if mode != 0x01 {
		t.Errorf("second response mode = 0x%02x, want 0x01", mode)
	}

	// Verify only one CTL allocate was sent (first 2 frames: allocate + DMS;
	// third frame: DMS only — no second allocate).
	ft.mu.Lock()
	sentCount := len(ft.sent)
	ft.mu.Unlock()
	if sentCount != 3 {
		t.Errorf("sent %d frames, want 3 (1 CTL alloc + 2 DMS)", sentCount)
	}
}

func TestConnQMIError(t *testing.T) {
	t.Parallel()
	ft := newFakeTransport()

	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 1, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceDMS), 0x01}}},
	})
	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: 0x002D,
		TLVs: []TLV{errorResult(26)}, // no-effect
	})

	conn := NewConn(ft)
	defer conn.Close()

	resp, err := conn.Send(ServiceDMS, 0x002D)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := resp.Result(); !errors.Is(err, ErrNoEffect) {
		t.Errorf("Result() = %v, want ErrNoEffect", err)
	}
}

func TestConnClose(t *testing.T) {
	t.Parallel()
	ft := newFakeTransport()

	// Queue CTL alloc response.
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 1, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceDMS), 0x01}}},
	})
	// Queue DMS response.
	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: 0x002D,
		TLVs: []TLV{successResult(), EncodeTLVU8(0x01, 0x00)},
	})
	// Queue CTL release response (for Close).
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 2, MsgID: ctlMsgReleaseCID,
		TLVs: []TLV{successResult()},
	})

	conn := NewConn(ft)
	_, err := conn.Send(ServiceDMS, 0x002D)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Transport should be closed.
	ft.mu.Lock()
	closed := ft.closed
	ft.mu.Unlock()
	if !closed {
		t.Error("transport not closed after conn.Close()")
	}

	// Verify a release CID message was sent.
	ft.mu.Lock()
	sentCount := len(ft.sent)
	ft.mu.Unlock()
	// Expected: 1 allocate + 1 DMS + 1 release = 3
	if sentCount != 3 {
		t.Errorf("sent %d frames, want 3 (alloc + DMS + release)", sentCount)
	}
}

func TestConnSendAfterClose(t *testing.T) {
	t.Parallel()
	ft := newFakeTransport()
	conn := NewConn(ft)
	conn.Close()

	_, err := conn.Send(ServiceDMS, 0x002D)
	if err == nil {
		t.Error("Send after Close returned nil error")
	}
}

func TestConnMultipleServices(t *testing.T) {
	t.Parallel()
	ft := newFakeTransport()

	// Allocate DMS client.
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 1, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceDMS), 0x01}}},
	})
	ft.queueResponse(&Message{
		Service: ServiceDMS, Client: 1, TxnID: 1, MsgID: 0x002D,
		TLVs: []TLV{successResult(), EncodeTLVU8(0x01, 0x00)},
	})

	// Allocate NAS client.
	ft.queueResponse(&Message{
		Service: ServiceCTL, TxnID: 2, MsgID: ctlMsgAllocateCID,
		TLVs: []TLV{successResult(), {Type: 0x01, Value: []byte{byte(ServiceNAS), 0x02}}},
	})
	ft.queueResponse(&Message{
		Service: ServiceNAS, Client: 2, TxnID: 2, MsgID: 0x004F,
		TLVs: []TLV{successResult()},
	})

	conn := NewConn(ft)
	defer conn.Close()

	// Send DMS request.
	_, err := conn.Send(ServiceDMS, 0x002D)
	if err != nil {
		t.Fatalf("DMS Send: %v", err)
	}

	// Send NAS request — should allocate a new client ID.
	_, err = conn.Send(ServiceNAS, 0x004F)
	if err != nil {
		t.Fatalf("NAS Send: %v", err)
	}

	// Verify we have two allocated clients.
	conn.mu.Lock()
	clientCount := len(conn.clients)
	conn.mu.Unlock()
	if clientCount != 2 {
		t.Errorf("allocated %d clients, want 2", clientCount)
	}
}
