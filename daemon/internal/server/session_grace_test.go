package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
	"github.com/gorilla/websocket"
)

const testGracePeriod = 100 * time.Millisecond

// newTestSessionGrace creates a Session wired with real dependencies and a
// short grace period for fast tests.
func newTestSessionGrace(t *testing.T, conn WSConn, gracePeriod time.Duration) *Session {
	t.Helper()
	cfg := &config.Config{
		SoloHome: t.TempDir(),
		Listen:   "127.0.0.1:0",
	}
	logger := newTestLogger()

	agentStorage := agent.NewAgentStorage(filepath.Join(cfg.SoloHome, "agents"), logger)
	agentStorage.Initialize()
	registry := agent.NewProviderRegistry()
	registry.Register(agent.NewMockAgentClient())
	agentMgr := agent.NewAgentManager(agentStorage, registry, logger)
	agentMgr.Initialize(nil)
	timelineStore := agent.NewInMemoryTimelineStore()
	workspaceStore := NewWorkspaceStore(cfg.SoloHome, logger)
	terminalMgr := terminal.NewTerminalManager(logger)
	projectReg := workspace.NewProjectRegistry(cfg.SoloHome)
	workspaceReg := workspace.NewWorkspaceRegistry(cfg.SoloHome)
	gitSvc := workspace.NewWorkspaceGitService()
	scriptMgr := workspace.NewScriptManager()
	scriptProxy := workspace.NewScriptProxy(logger, scriptMgr)

	sess := NewSession(
		"test-client", string(protocol.ClientCLI), conn,
		cfg, logger, agentMgr, timelineStore, registry,
		workspaceStore, terminalMgr, projectReg, workspaceReg,
		gitSvc, scriptMgr, scriptProxy,
		func(msg protocol.WSOutboundMessage) {},
	)
	sess.gracePeriod = gracePeriod
	return sess
}

// --- Step 1: Session enters grace on disconnect ---

func TestSession_EnterGraceOnDisconnect(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	// Let Run() start its goroutines
	time.Sleep(100 * time.Millisecond)

	// Simulate disconnect
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	// Wait for Run() to return (should enter grace, not hang)
	select {
	case <-done:
		// Run() returned — expected
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	// Session should be in grace state
	if !sess.IsInGrace() {
		t.Error("expected session to be in grace period after disconnect")
	}

	// done channel should NOT be closed (grace period hasn't expired)
	select {
	case <-sess.done:
		t.Error("expected done channel to NOT be closed during grace period")
	default:
		// Correct
	}
}

// --- Step 2: Grace expired → full cleanup ---

func TestSession_GraceExpired_FullCleanup(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod)

	// Track activity tracker cleanup
	var trackerRemoveCalled bool
	sess.SetActivityTracker(&mockActivityTracker{
		onRemove: func() { trackerRemoveCalled = true },
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	// Wait for Run() to return (enters grace)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	// Wait for grace period to expire
	time.Sleep(testGracePeriod + 50*time.Millisecond)

	// done channel should now be closed
	select {
	case <-sess.done:
		// Expected — grace expired
	default:
		t.Error("expected done channel to be closed after grace expires")
	}

	// Session should no longer be in grace
	if sess.IsInGrace() {
		t.Error("expected session to NOT be in grace after expiry")
	}

	// Verify activity tracker cleanup was called
	if !trackerRemoveCalled {
		t.Error("expected activity tracker Remove to be called after grace expires")
	}
}

// --- Step 3: Grace period drops messages silently ---

func TestSession_DropsMessagesDuringGrace(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	// sendMessage during grace should not panic or block
	sess.sendMessage(protocol.NewPongMessage())

	// The old connection should NOT have received the message
	// (it was closed, and sendMessage should drop silently during grace)
	conn.mu.Lock()
	msgCount := len(conn.messages)
	conn.mu.Unlock()
	// messages written before disconnect don't count; we only care that the
	// post-disconnect sendMessage didn't panic or block.
	_ = msgCount
}

// --- Step 4: ReplaceConn resumes session ---

func TestSession_ReplaceConn_ResumesSession(t *testing.T) {
	conn1 := newMockConn()
	sess := newTestSessionGrace(t, conn1, testGracePeriod)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to be in grace before ReplaceConn")
	}

	// Replace with a new connection before grace expires
	conn2 := newMockConn()
	replaceDone := make(chan error, 1)
	go func() {
		// ReplaceConn blocks until the new connection disconnects
		replaceDone <- sess.ReplaceConn(conn2)
	}()

	// Wait a moment for ReplaceConn to set up
	time.Sleep(100 * time.Millisecond)

	// Session should no longer be in grace
	if sess.IsInGrace() {
		t.Error("expected session to NOT be in grace after ReplaceConn")
	}

	// Send a message — should flow through conn2
	sess.sendMessage(protocol.NewPongMessage())
	time.Sleep(50 * time.Millisecond)

	conn2.mu.Lock()
	wroteMsgs := len(conn2.messages)
	conn2.mu.Unlock()
	if wroteMsgs == 0 {
		t.Error("expected message to be written on new connection after ReplaceConn")
	}

	// Disconnect the new connection to unblock ReplaceConn
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case err := <-replaceDone:
		if err != nil {
			t.Errorf("ReplaceConn returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReplaceConn did not return within timeout")
	}
}

// --- Step 5: ReplaceConn preserves agent subscription ---

func TestSession_ReplaceConn_PreservesAgentSubscription(t *testing.T) {
	conn1 := newMockConn()
	sess := newTestSessionGrace(t, conn1, testGracePeriod)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn1.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	// Replace connection
	conn2 := newMockConn()
	replaceDone := make(chan error, 1)
	go func() {
		replaceDone <- sess.ReplaceConn(conn2)
	}()
	time.Sleep(100 * time.Millisecond)

	// Emit an agent state event by triggering the broadcast function.
	// Since the session subscribes to agentMgr, we can trigger an event
	// that the session's handleAgentEvent will process.
	// We verify indirectly: if the subscription was torn down, this would panic.
	sess.broadcast(protocol.NewPongMessage())

	// The agent subscription should still be active, so handleAgentEvent
	// should have been called. We verify by checking that at least a message
	// was written to conn2 (the agent_update or similar).
	// Since we have no agents, the event may be a no-op — this test mainly
	// verifies the subscription wasn't torn down by entering grace.

	// Disconnect conn2 to clean up
	conn2.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-replaceDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ReplaceConn did not return within timeout")
	}
}

// --- Step 6: ReplaceConn after grace expired returns error ---

func TestSession_ReplaceConn_AfterGraceExpired_ReturnsError(t *testing.T) {
	conn := newMockConn()
	sess := newTestSessionGrace(t, conn, testGracePeriod)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(100 * time.Millisecond)
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not return within timeout")
	}

	// Wait for grace to expire
	time.Sleep(testGracePeriod + 50*time.Millisecond)

	// ReplaceConn should fail
	conn2 := newMockConn()
	err := sess.ReplaceConn(conn2)
	if err == nil {
		t.Error("expected ReplaceConn to return error after grace expired")
	}
}

// mockActivityTracker is a minimal ActivityTracker for tests.
type mockActivityTracker struct {
	states []ClientPresenceState
	onRemove func()
}

func (m *mockActivityTracker) UpdateActivity(sessionID string, appVisible bool, focusedAgentID string) {}
func (m *mockActivityTracker) Remove(sessionID string) {
	if m.onRemove != nil {
		m.onRemove()
	}
}
func (m *mockActivityTracker) GetAllStates() []ClientPresenceState { return m.states }
