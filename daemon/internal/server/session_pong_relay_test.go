package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// readPong pops the next message from the session send queue and decodes it
// as a pong. handlePing pushes synchronously, so Pop returns immediately.
func readPong(t *testing.T, sess *Session) protocol.PongPayload {
	t.Helper()
	item, ok := sess.sendQueue.Pop()
	if !ok {
		t.Fatal("send queue closed before pong was pushed")
	}
	var envelope struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(item.data, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var pong protocol.PongMessage
	if err := json.Unmarshal(envelope.Message, &pong); err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}
	return pong.Payload
}

// TestHandlePing_RelayRttMs verifies that the pong carries relayRttMs only
// for relay sessions with a fresh relay-leg RTT measurement.
func TestHandlePing_RelayRttMs(t *testing.T) {
	fresh := func() (int64, time.Time, bool) { return 42, time.Now(), true }
	stale := func() (int64, time.Time, bool) { return 42, time.Now().Add(-time.Minute), true }
	unavailable := func() (int64, time.Time, bool) { return 0, time.Time{}, false }

	tests := []struct {
		name     string
		isRelay  bool
		provider func() (int64, time.Time, bool)
		wantRtt  *int64
	}{
		{name: "relay session with fresh measurement", isRelay: true, provider: fresh, wantRtt: ptrInt64(42)},
		{name: "direct session leaves it absent", isRelay: false, provider: fresh, wantRtt: nil},
		{name: "stale measurement is dropped", isRelay: true, provider: stale, wantRtt: nil},
		{name: "no measurement available", isRelay: true, provider: unavailable, wantRtt: nil},
		{name: "relay session without provider", isRelay: true, provider: nil, wantRtt: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newSlowWritePingConn(0)
			sess := newTestSessionForPing(t, conn)
			sess.SetIsRelay(tt.isRelay)
			if tt.provider != nil {
				sess.SetRelayRTTProvider(tt.provider)
			}

			sess.handlePing(&protocol.PingMessage{Type: "ping", RequestID: "req-1"})
			payload := readPong(t, sess)

			if payload.RequestID != "req-1" {
				t.Errorf("RequestID = %q, want %q", payload.RequestID, "req-1")
			}
			if tt.wantRtt == nil {
				if payload.RelayRttMs != nil {
					t.Errorf("RelayRttMs = %d, want absent", *payload.RelayRttMs)
				}
			} else {
				if payload.RelayRttMs == nil {
					t.Fatalf("RelayRttMs absent, want %d", *tt.wantRtt)
				}
				if *payload.RelayRttMs != *tt.wantRtt {
					t.Errorf("RelayRttMs = %d, want %d", *payload.RelayRttMs, *tt.wantRtt)
				}
			}
		})
	}
}

func ptrInt64(v int64) *int64 { return &v }
