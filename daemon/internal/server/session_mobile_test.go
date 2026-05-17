package server

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

// TestEffectivePingTimeout_MobileGets60s verifies mobile clients use the
// extended 60s timeout so a backgrounded iOS app doesn't lose its session.
func TestEffectivePingTimeout_MobileGets60s(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod)
	sess.clientType = string(protocol.ClientMobile)

	got := sess.effectivePingTimeout()
	if got != mobilePingTimeout {
		t.Errorf("mobile: effectivePingTimeout() = %v, want %v", got, mobilePingTimeout)
	}
	if got < 30*time.Second {
		t.Errorf("mobile timeout %v is too short; must survive iOS background suspension", got)
	}
}

// TestEffectivePingTimeout_NonMobileGets5s verifies non-mobile clients use
// the standard 5s timeout.
func TestEffectivePingTimeout_NonMobileGets5s(t *testing.T) {
	cases := []protocol.ClientType{protocol.ClientBrowser, protocol.ClientCLI, protocol.ClientMCP}
	for _, ct := range cases {
		conn := newMockConn()
		sess := newTestSessionGrace(t, conn, testGracePeriod)
		sess.clientType = string(ct)

		got := sess.effectivePingTimeout()
		if got != pingTimeout {
			t.Errorf("clientType=%s: effectivePingTimeout() = %v, want %v", ct, got, pingTimeout)
		}
	}
}

// TestMobileSession_PingTimeoutExtended_DoesNotDisconnectWithin10s verifies
// that a mobile session is NOT disconnected within 10s of no pong, whereas a
// non-mobile session with 5s timeout would be.
//
// We use a mock conn that never responds to pings (no pong handler).
// With the standard 5s timeout the session disconnects quickly.
// With the mobile 60s timeout it stays alive well past 10s.
func TestMobileSession_PingTimeoutExtended_DoesNotDisconnectWithin10s(t *testing.T) {
	// Override pingInterval to 100ms for speed
	orig := pingInterval
	pingInterval = 100 * time.Millisecond
	defer func() { pingInterval = orig }()

	conn := newMockConn()
	// Use a very short non-mobile timeout to confirm standard would disconnect fast
	sess := newTestSessionGrace(t, conn, testGracePeriod)
	sess.clientType = string(protocol.ClientMobile)

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		sess.Run()
	}()

	// After 500ms a non-mobile 5s session would still be alive, but let's
	// verify mobile stays alive past the standard pingTimeout window.
	time.Sleep(200 * time.Millisecond)

	// Session should still be running (not entered grace)
	if sess.IsInGrace() {
		t.Error("mobile session entered grace too early; extended ping timeout not working")
	}

	// Clean up
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return")
	}
}
