package gateway

import (
	"encoding/json"
	"sync"
	"testing"
)

func mockClient(connID string, role string, scopes []string) (*WsClient, *[][]byte) {
	var sent [][]byte
	var mu sync.Mutex
	return &WsClient{
		ConnID:  connID,
		Connect: ConnectParams{Role: role, Scopes: scopes},
		Send: func(data []byte) error {
			mu.Lock()
			sent = append(sent, append([]byte{}, data...))
			mu.Unlock()
			return nil
		},
		Close:          func(code int, reason string) error { return nil },
		BufferedAmount: func() int64 { return 0 },
	}, &sent
}

func TestBroadcaster_BroadcastAll(t *testing.T) {
	b := NewBroadcaster()
	c1, sent1 := mockClient("c1", "operator", nil)
	c2, sent2 := mockClient("c2", "operator", nil)
	b.AddClient(c1)
	b.AddClient(c2)

	b.Broadcast("test.event", map[string]string{"msg": "hello"}, nil)

	if len(*sent1) != 1 {
		t.Errorf("c1 received %d messages, want 1", len(*sent1))
	}
	if len(*sent2) != 1 {
		t.Errorf("c2 received %d messages, want 1", len(*sent2))
	}

	var frame eventFrame
	json.Unmarshal((*sent1)[0], &frame)
	if frame.Event != "test.event" {
		t.Errorf("event = %q, want test.event", frame.Event)
	}
	if frame.Seq == nil || *frame.Seq != 1 {
		t.Errorf("seq should be 1")
	}
}

func TestBroadcaster_BroadcastToConnIDs(t *testing.T) {
	b := NewBroadcaster()
	c1, sent1 := mockClient("c1", "operator", nil)
	c2, sent2 := mockClient("c2", "operator", nil)
	b.AddClient(c1)
	b.AddClient(c2)

	targets := map[string]struct{}{"c1": {}}
	b.BroadcastToConnIDs("targeted", "data", targets, nil)

	if len(*sent1) != 1 {
		t.Errorf("c1 should receive targeted message")
	}
	if len(*sent2) != 0 {
		t.Errorf("c2 should NOT receive targeted message")
	}
}

func TestBroadcaster_ScopeGuard(t *testing.T) {
	b := NewBroadcaster()
	cAdmin, sentAdmin := mockClient("admin", "operator", []string{scopeAdmin})
	cBasic, sentBasic := mockClient("basic", "operator", nil)
	b.AddClient(cAdmin)
	b.AddClient(cBasic)

	b.Broadcast("exec.approval.requested", nil, nil)

	if len(*sentAdmin) != 1 {
		t.Error("admin should receive approval events")
	}
	if len(*sentBasic) != 0 {
		t.Error("basic operator should NOT receive approval events")
	}
}

func TestBroadcaster_SlowConsumer_Drop(t *testing.T) {
	b := NewBroadcaster()
	var closed bool
	c := &WsClient{
		ConnID:         "slow",
		Connect:        ConnectParams{Role: "operator"},
		Send:           func(data []byte) error { return nil },
		Close:          func(code int, reason string) error { closed = true; return nil },
		BufferedAmount: func() int64 { return MaxBufferedBytes + 1 },
	}
	b.AddClient(c)

	b.Broadcast("test", nil, &BroadcastOptions{DropIfSlow: true})
	if closed {
		t.Error("dropIfSlow should NOT close, just skip")
	}
}

func TestBroadcaster_SlowConsumer_Close(t *testing.T) {
	b := NewBroadcaster()
	var closedCode int
	c := &WsClient{
		ConnID:         "slow",
		Connect:        ConnectParams{Role: "operator"},
		Send:           func(data []byte) error { return nil },
		Close:          func(code int, reason string) error { closedCode = code; return nil },
		BufferedAmount: func() int64 { return MaxBufferedBytes + 1 },
	}
	b.AddClient(c)

	b.Broadcast("test", nil, nil) // not dropIfSlow, so should close
	if closedCode != 1008 {
		t.Errorf("slow consumer should be closed with 1008, got %d", closedCode)
	}
}

func TestBroadcaster_RemoveClient(t *testing.T) {
	b := NewBroadcaster()
	c, _ := mockClient("c1", "operator", nil)
	b.AddClient(c)
	if b.ClientCount() != 1 {
		t.Fatalf("count = %d, want 1", b.ClientCount())
	}
	b.RemoveClient("c1")
	if b.ClientCount() != 0 {
		t.Errorf("count = %d after removal, want 0", b.ClientCount())
	}
}

func TestBroadcaster_SeqIncrement(t *testing.T) {
	b := NewBroadcaster()
	c, sent := mockClient("c1", "operator", nil)
	b.AddClient(c)

	b.Broadcast("e1", nil, nil)
	b.Broadcast("e2", nil, nil)

	if len(*sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(*sent))
	}
	var f1, f2 eventFrame
	json.Unmarshal((*sent)[0], &f1)
	json.Unmarshal((*sent)[1], &f2)
	if *f1.Seq != 1 || *f2.Seq != 2 {
		t.Errorf("seq should increment: got %d, %d", *f1.Seq, *f2.Seq)
	}
}
