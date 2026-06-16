package server

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// mockConn is a test double that blocks on ReadMessage until signaled.
type mockConn struct {
	readOnce sync.Once
	readErr  chan error
	messages [][]byte
	closed   bool
	mu       sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{
		readErr: make(chan error, 1),
	}
}

func (m *mockConn) ReadMessage() (messageType int, p []byte, err error) {
	// Block until an error is injected (simulating connection loss)
	err = <-m.readErr
	return websocket.TextMessage, nil, err
}

func (m *mockConn) WriteMessage(_ int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, data)
	return nil
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) WriteControl(_ int, _ []byte, _ time.Time) error {
	return nil
}

func (m *mockConn) SetPongHandler(_ func(appData string) error) {}

func (m *mockConn) SetReadDeadline(_ time.Time) error { return nil }

func (m *mockConn) injectReadError(err error) {
	m.readOnce.Do(func() {
		m.readErr <- err
	})
}

// panicError wraps a recovered panic value as an error for test assertions.
type panicError struct {
	value interface{}
}

func (e *panicError) Error() string {
	return "panic occurred"
}

// TestSessionShutdownDoesNotPanicWithPendingCoalescer verifies that when a
// session shuts down while the coalescer still holds pending timeline events,
// the shutdown completes without panicking.
//
// This is a regression test for the following bug:
//  1. Client disconnects
//  2. Session.Run() closes sendQueue
//  3. defer s.coalescer.FlushAll() runs AFTER sendQueue is closed
//  4. FlushAll() callbacks try to write to closed sendQueue -> panic
func TestSessionShutdownDoesNotPanicWithPendingCoalescer(t *testing.T) {
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
	agentMgr.Initialize(context.TODO())
	timelineStore := agent.NewInMemoryTimelineStore()
	workspaceStore := NewWorkspaceStore(cfg.SoloHome, logger)
	terminalMgr := terminal.NewTerminalManager(logger)
	projectReg := workspace.NewProjectRegistry(cfg.SoloHome)
	workspaceReg := workspace.NewWorkspaceRegistry(cfg.SoloHome)
	gitSvc := workspace.NewWorkspaceGitService()
	scriptMgr := workspace.NewScriptManager()
	scriptProxy := workspace.NewScriptProxy(logger, scriptMgr)

	conn := newMockConn()
	sess := NewSessionWithConfig(
		"test-client", string(protocol.ClientCLI), conn,
		SessionConfig{
			Config:         cfg,
			Logger:         logger,
			AgentMgr:       agentMgr,
			TimelineStore:  timelineStore,
			Registry:       registry,
			WorkspaceStore: workspaceStore,
			TerminalMgr:    terminalMgr,
			ProjectReg:     projectReg,
			WorkspaceReg:   workspaceReg,
			GitSvc:         gitSvc,
			ScriptMgr:      scriptMgr,
			ScriptProxy:    scriptProxy,
			Broadcast:      func(_ protocol.WSOutboundMessage) {},
		},
	)

	// Inject pending coalescer data so that FlushAll() has work to do
	// during shutdown.
	sess.coalescer.Handle("agent-1", "timeline", agent.TimelineItem{
		Type: "assistant_message",
		Text: "pending text",
	}, "claude", "turn-1")

	// Start session in a goroutine
	done := make(chan struct{})
	var runErr error
	go func() {
		defer func() {
			if r := recover(); r != nil {
				runErr = &panicError{value: r}
			}
			close(done)
		}()
		sess.Run()
	}()

	// Give Run() time to start its goroutines and complete initial
	// async setup (e.g. sendProviderSnapshot goroutine).
	time.Sleep(200 * time.Millisecond)

	// Simulate connection loss by injecting a read error
	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	// Wait for Run() to complete, with timeout
	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("session.Run() did not complete within timeout")
	}

	if runErr != nil {
		t.Fatalf("session.Run() panicked: %v", runErr)
	}

	// Verify connection was closed cleanly
	if !conn.closed {
		t.Error("expected connection to be closed")
	}
}
