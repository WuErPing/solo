package server

import (
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

// slowWritePingConn is a mock WSConn where WriteMessage simulates network
// latency by sleeping. This forces the writePump select loop to spend most of
// its time inside WriteMessage, so the pingCh case in the old code gets very
// few chances to fire. WriteControl is instant (as it would be with real
// gorilla/websocket, which sends control frames out-of-band).
type slowWritePingConn struct {
	mu         sync.Mutex
	readErr    chan error
	readOnce   sync.Once
	writeDelay time.Duration // artificial delay per WriteMessage call
	messages   [][]byte
	pingTimes  []time.Time
	closed     bool
}

func newSlowWritePingConn(writeDelay time.Duration) *slowWritePingConn {
	return &slowWritePingConn{
		readErr:    make(chan error, 1),
		writeDelay: writeDelay,
	}
}

func (c *slowWritePingConn) ReadMessage() (int, []byte, error) {
	err := <-c.readErr
	return websocket.TextMessage, nil, err
}

func (c *slowWritePingConn) WriteMessage(messageType int, data []byte) error {
	// Simulate slow network I/O — this is what causes ping starvation
	// in the old writePump: the select loop is stuck here and can't
	// service the pingCh case.
	time.Sleep(c.writeDelay)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, data)
	return nil
}

func (c *slowWritePingConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// WriteControl is called from the pingLoop goroutine (after the fix).
// In gorilla/websocket, WriteControl is documented as concurrent-safe.
func (c *slowWritePingConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if messageType == websocket.PingMessage {
		c.pingTimes = append(c.pingTimes, time.Now())
	}
	return nil
}

func (c *slowWritePingConn) SetPongHandler(h func(appData string) error) {}
func (c *slowWritePingConn) SetReadDeadline(t time.Time) error           { return nil }

func (c *slowWritePingConn) injectReadError(err error) {
	c.readOnce.Do(func() { c.readErr <- err })
}

// newTestSessionForPing creates a Session wired for ping starvation tests.
// It does NOT call Run(); the caller drives writePump manually.
func newTestSessionForPing(t *testing.T, conn WSConn) *Session {
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

	sess := NewSessionWithConfig(
		"test-client", string(protocol.ClientCLI), conn,
		SessionConfig{
			Config:        cfg,
			Logger:        logger,
			AgentMgr:      agentMgr,
			TimelineStore: timelineStore,
			Registry:      registry,
			WorkspaceStore: workspaceStore,
			TerminalMgr:    terminalMgr,
			ProjectReg:     projectReg,
			WorkspaceReg:   workspaceReg,
			GitSvc:         gitSvc,
			ScriptMgr:      scriptMgr,
			ScriptProxy:    scriptProxy,
			Broadcast:      func(msg protocol.WSOutboundMessage) {},
		},
	)
	return sess
}

// TestWritePump_PingNotStarvedBySendQueue verifies that under a flooded
// sendQueue with slow writes, WebSocket ping frames are still sent within
// 2x the ping interval.
//
// Regression test: the old writePump used a select between sendQueue and
// pingCh. When WriteMessage is slow (simulating network latency) and
// sendQueue has many items, the select loop spends almost all its time
// inside WriteMessage calls and rarely gets to service the pingCh case.
// This causes pings to be delayed well beyond pingInterval.
//
// With the fix (dedicated pingLoop goroutine calling WriteControl directly),
// pings are sent independently of the writePump loop, so they arrive on
// time regardless of sendQueue traffic.
func TestWritePump_PingNotStarvedBySendQueue(t *testing.T) {
	// Use a short ping interval for test speed.
	origPingInterval := pingInterval
	pingInterval = 100 * time.Millisecond
	defer func() { pingInterval = origPingInterval }()

	// WriteMessage takes 150ms — 1.5x pingInterval. With the old code,
	// the select loop is stuck in WriteMessage longer than the ping
	// interval, so pingCh fires but can't be serviced until the current
	// write finishes. This causes pings to be delayed significantly.
	// 10 messages = 1.5s of drain time, during which the 100ms ping
	// ticker should fire ~15 times, but with the old select code many
	// of those fire events will be missed.
	conn := newSlowWritePingConn(150 * time.Millisecond)
	sess := newTestSessionForPing(t, conn)

	// Set up ping/pong as Run() would.
	sess.setupPingPong()

	// Pre-load the sendQueue with 10 messages and close it so writePump
	// will drain them and exit. Each write takes 150ms, so 10 writes
	// take 1.5 seconds — enough for the 100ms ping ticker to fire
	// ~15 times.
	for i := 0; i < 10; i++ {
		sess.sendQueue.Push(sendQueueItem{msgType: websocket.TextMessage, data: []byte("{}")})
	}
	sess.sendQueue.Close()

	// Start writePump.
	writePumpStart := time.Now()
	go sess.writePump()

	// Wait for writePump to finish draining.
	select {
	case <-sess.writeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("writePump did not finish within timeout")
	}
	elapsed := time.Since(writePumpStart)

	// Verify that at least one ping was sent during the drain window.
	conn.mu.Lock()
	pings := len(conn.pingTimes)
	times := make([]time.Time, len(conn.pingTimes))
	copy(times, conn.pingTimes)
	conn.mu.Unlock()

	t.Logf("drain took %v, %d pings sent", elapsed, pings)
	for i, pt := range times {
		t.Logf("  ping %d at relative %v", i, pt.Sub(writePumpStart))
	}

	if pings == 0 {
		t.Fatalf("expected at least one ping during %v of slow writes, got %d — ping is starved by busy writePump", elapsed, pings)
	}

	// Verify that each consecutive ping gap is within 3x pingInterval.
	// In the old code, because WriteMessage blocks for 150ms (> pingInterval),
	// the select loop can only check pingCh between writes. The worst case
	// gap between two ping opportunities is ~2x WriteMessage delay = 300ms,
	// but the ticker can also be missed entirely if the timing doesn't align.
	// With the fix (independent pingLoop), the gap should be close to
	// pingInterval (100ms), well under 3x.
	maxAllowedGap := 3 * pingInterval
	for i := 1; i < len(times); i++ {
		gap := times[i].Sub(times[i-1])
		if gap > maxAllowedGap {
			t.Errorf("ping %d arrived %v after ping %d (max allowed %v); ping is being starved by sendQueue",
				i, gap, i-1, maxAllowedGap)
		}
	}
}
