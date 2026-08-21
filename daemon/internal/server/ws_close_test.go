package server

import (
	"testing"
	"time"
)

// Regression test: WSServer.Close must not hold s.mu while expiring grace
// sessions — expireGrace runs the onGraceExpire callback, which re-acquires
// s.mu and would self-deadlock (RWMutex is not reentrant). This deadlock
// blocked every graceful shutdown that had a session in grace.
func TestWSServer_Close_ExpiresGraceSessionsWithoutDeadlock(t *testing.T) {
	conn := newMockConn()
	// Long grace period so the timer does not fire during the test.
	sess := newTestSessionGrace(t, conn, time.Hour)
	logger := newTestLogger()

	ws := &WSServer{
		logger:   logger,
		sessions: make(map[string]*Session),
		done:     make(chan struct{}),
	}
	ws.tmuxWatcher = NewTmuxPaneWatcher(logger, ws.broadcast)
	ws.sessions["test-client"] = sess
	sess.onGraceExpire = func() {
		ws.mu.Lock()
		delete(ws.sessions, "test-client")
		ws.mu.Unlock()
	}
	sess.enterGrace()
	if !sess.IsInGrace() {
		t.Fatal("session should be in grace")
	}

	done := make(chan struct{})
	go func() {
		ws.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WSServer.Close deadlocked on a grace session")
	}

	ws.mu.RLock()
	remaining := len(ws.sessions)
	ws.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("grace session not removed, %d sessions remain", remaining)
	}
}
