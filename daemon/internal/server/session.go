package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/metrics"
	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/daemon/internal/schedule"
	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

// Session represents a single client WebSocket connection.
type Session struct {
	clientID        string
	clientType      string
	conn            WSConn
	cfg             *config.Config
	logger          *slog.Logger
	agentMgr        *agent.AgentManager
	timelineStore   *agent.InMemoryTimelineStore
	registry        *agent.ProviderRegistry
	workspaceStore  *WorkspaceStore
	terminalMgr     *terminal.TerminalManager
	projectReg      *workspace.ProjectRegistry
	workspaceReg    *workspace.WorkspaceRegistry
	gitSvc          workspace.WorkspaceGitService
	scriptMgr       *workspace.ScriptManager
	scriptProxy     *workspace.ScriptProxy
	broadcast       func(protocol.WSOutboundMessage)
	coalescer       *agent.StreamCoalescer
	pushTokenStore  push.TokenStore
	activityTracker ActivityTracker
	pusher          push.Pusher
	memoryBridge    MemoryBridge
	scheduleStore    *schedule.Store
	scheduleExecutor *schedule.Executor

	workspaces   map[string]*protocol.WorkspaceDescriptor
	workspacesMu sync.RWMutex

	// Terminal slot mapping for binary frame routing (slot byte <-> terminal ID)
	slotToTerminal map[byte]*terminal.TerminalProcess
	terminalToSlot map[string]byte
	nextSlot       byte
	slotMu         sync.Mutex

	terminalSubscriptions []func() // cleanup funcs for terminal subscriptions

	// Worktree setup progress tracking
	setupProgress   map[string]*workspace.SetupProgressEvent // key: workspaceID
	setupProgressMu sync.RWMutex

	done             chan struct{}
	unsub            func() // unsubscribe from agent manager events
	coalescerFlushID uint64 // registration ID for coalescer flush callback
	doneOnce         sync.Once
	cleanupOnce      sync.Once

	// Multi-socket model: sockets holds all currently attached WebSocket connections.
	// Guarded by socketsMu. Used by AttachSocket / broadcastToSockets.
	sockets   map[string]socketEntry
	socketsMu sync.RWMutex

	// Async send queue (mirrors Node.js ws library behavior: enqueue and return)
	sendQueue *sendQueue
	writeDone chan struct{}

	handlerRegistry *messageHandlerRegistry

	// Inbound message queue decouples ReadMessage from handler execution
	inboundQueue chan inboundQueueItem
	processDone  chan struct{}
	// inboundClosed is set to true immediately after close(inboundQueue) in
	// runReadLoop, before <-processDone completes. AttachSocket checks this flag
	// to detect the window where inboundQueue is closed but processDone is not yet
	// closed (processLoop still draining). Without this check, readLoopFor would
	// send to a closed channel and panic.
	inboundClosed atomic.Bool

	// Grace period: when a client disconnects, the session enters a grace state
	// instead of being immediately destroyed. If the same client reconnects within
	// gracePeriod, the session is resumed via ReplaceConn.
	gracePeriod   time.Duration
	graceTimer    *time.Timer
	graceExpired  bool
	graceMu       sync.Mutex
	connMu        sync.Mutex
	onGraceExpire func() // callback to WSServer to remove session from map

	// graceCriticalBuf buffers critical messages (turn_completed, agent_update, etc.)
	// and agent_stream messages that arrive during grace period so they can be
	// replayed on ReplaceConn.
	graceCriticalBuf []sendQueueItem

	// graceExtensions tracks how many times the grace timer has been extended
	// due to a running agent. Capped to prevent infinite extension.
	graceExtensions int

	// attachingCount tracks how many AttachSocket calls are currently between
	// "grace cancelled" and "socket registered in s.sockets". During this window
	// IsInGrace() returns false but the session is not yet fully connected.
	// handleNewConnection checks IsAttaching() to avoid calling
	// shutdownForReplacement on a session that is in the middle of being resumed.
	attachingCount atomic.Int32

	// attachingConn holds the WebSocket connection that is currently being
	// attached via AttachSocket (set before grace cancellation, cleared after
	// socket registration). shutdownForReplacement closes this conn so the
	// blocking readLoopFor unblocks and AttachSocket can exit cleanly.
	attachingConn   WSConn
	attachingConnMu sync.Mutex

	// isRelay indicates the connection is via a relay (E2EE encrypted).
	// When true, Layer 1 WebSocket ping is skipped to avoid interfering
	// with the relay's perMessageDeflate and E2EE state machine.
	isRelay bool
}

// SetIsRelay marks this session as running through a relay.
func (s *Session) SetIsRelay(v bool) { s.isRelay = v }

type sendQueueItem struct {
	msgType int
	data    []byte
}

type inboundQueueItem struct {
	msgType int
	data    []byte
}

// NewSession creates a new session.
// Deprecated: use NewSessionWithConfig
func NewSession(clientID, clientType string, conn WSConn, cfg *config.Config, logger *slog.Logger, agentMgr *agent.AgentManager, timelineStore *agent.InMemoryTimelineStore, registry *agent.ProviderRegistry, workspaceStore *WorkspaceStore, terminalMgr *terminal.TerminalManager, projectReg *workspace.ProjectRegistry, workspaceReg *workspace.WorkspaceRegistry, gitSvc workspace.WorkspaceGitService, scriptMgr *workspace.ScriptManager, scriptProxy *workspace.ScriptProxy, broadcast func(protocol.WSOutboundMessage)) *Session {
	sess := &Session{
		clientID:       clientID,
		clientType:     clientType,
		conn:           conn,
		cfg:            cfg,
		logger:         logger.With("clientId", clientID),
		agentMgr:       agentMgr,
		timelineStore:  timelineStore,
		registry:       registry,
		workspaceStore: workspaceStore,
		terminalMgr:    terminalMgr,
		projectReg:     projectReg,
		workspaceReg:   workspaceReg,
		gitSvc:         gitSvc,
		scriptMgr:      scriptMgr,
		scriptProxy:    scriptProxy,
		broadcast:      broadcast,
		workspaces:     make(map[string]*protocol.WorkspaceDescriptor),
		slotToTerminal: make(map[byte]*terminal.TerminalProcess),
		terminalToSlot: make(map[string]byte),
		setupProgress:  make(map[string]*workspace.SetupProgressEvent),
		scheduleStore:  schedule.NewStore(schedule.WithDataPath(filepath.Join(cfg.SoloHome, "schedules.json"))),
		done:           make(chan struct{}),
		gracePeriod:    time.Duration(protocol.SessionDisconnectGraceMs) * time.Millisecond,
		sendQueue:      newSendQueue(),
		writeDone:      make(chan struct{}),
		inboundQueue:   make(chan inboundQueueItem, 64),
		processDone:    make(chan struct{}),
	}

	// Load persisted workspaces
	for _, ws := range workspaceStore.GetAll() {
		sess.workspaces[ws.ID] = ws
	}

	// Create coalescer for batching timeline events.
	// 500ms window reduces duplicate "Thinking" entries when Claude splits
	// a single thinking block across multiple deltas spaced >200ms apart.
	sess.coalescer = agent.NewStreamCoalescer(500, func(p agent.FlushPayload) {
		sess.handleCoalescedFlush(p)
	})

	// Build handler registry for OCP-compliant message dispatch
	sess.handlerRegistry = newMessageHandlerRegistry()
	sess.registerHandlers()

	return sess
}

// SessionConfig aggregates all dependency parameters needed by NewSession.
// This reduces the parameter count from 16 to 3 (clientID, clientType, conn).
type SessionConfig struct {
	Config         *config.Config
	Logger         *slog.Logger
	AgentMgr       *agent.AgentManager
	TimelineStore  *agent.InMemoryTimelineStore
	Registry       *agent.ProviderRegistry
	WorkspaceStore *WorkspaceStore
	TerminalMgr    *terminal.TerminalManager
	ProjectReg     *workspace.ProjectRegistry
	WorkspaceReg   *workspace.WorkspaceRegistry
	GitSvc         workspace.WorkspaceGitService
	ScriptMgr      *workspace.ScriptManager
	ScriptProxy    *workspace.ScriptProxy
	Broadcast      func(protocol.WSOutboundMessage)
}

// NewSessionWithConfig creates a new session using a SessionConfig.
func NewSessionWithConfig(clientID, clientType string, conn WSConn, cfg SessionConfig) *Session {
	return NewSession(
		clientID,
		clientType,
		conn,
		cfg.Config,
		cfg.Logger,
		cfg.AgentMgr,
		cfg.TimelineStore,
		cfg.Registry,
		cfg.WorkspaceStore,
		cfg.TerminalMgr,
		cfg.ProjectReg,
		cfg.WorkspaceReg,
		cfg.GitSvc,
		cfg.ScriptMgr,
		cfg.ScriptProxy,
		cfg.Broadcast,
	)
}

// Run starts reading messages from the WebSocket connection.
func (s *Session) Run() {
	// Set up server-initiated ping/pong if the connection supports it.
	// This detects half-open connections (e.g. iOS app backgrounded, NAT timeout).
	s.setupPingPong()

	// Start async write pump (mirrors Node.js ws library enqueue behavior)
	go s.writePump()

	// Start message processing loop in a separate goroutine so that slow
	// handlers (e.g. git metadata queries, OpenCode server cold-start) do not
	// block subsequent message reads.
	go s.processLoop()

	// Subscribe to agent manager events
	s.unsub = s.agentMgr.Subscribe(s.handleAgentEvent)
	// Register coalescer flush so buffered timeline entries are flushed
	// even when the agent event pipeline is congested (workCh fallback path).
	s.coalescerFlushID = s.agentMgr.RegisterCoalescerFlusher(func(agentID string) {
		s.coalescer.FlushFor(agentID)
	})

	// Push all currently active agents to the newly connected client so the
	// App store is pre-populated before any agent_stream events can arrive.
	// This prevents agent_stream events from being received for agents that
	// are not yet in the store (which would cause messages to be invisible).
	s.pushActiveAgents()

	// Start the schedule executor
	s.startScheduleExecutor()

	// Send provider snapshot
	s.sendProviderSnapshot()

	s.runReadLoop()
}

// runReadLoop reads messages from the WebSocket until an error occurs.
// On disconnect, the session enters the grace period instead of being
// immediately destroyed. The calling goroutine is freed (Run() returns)
// while the session waits for a potential reconnect via ReplaceConn().
func (s *Session) runReadLoop() {
	defer s.conn.Close()

	for {
		s.connMu.Lock()
		conn := s.conn
		s.connMu.Unlock()

		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Info("session read error", "error", err)
			} else {
				s.logger.Info("session closed", "reason", "normal closure")
			}
			// Stop the read side: signal processLoop to drain and exit.
			// Set inboundClosed before <-processDone so AttachSocket can detect
			// the window where inboundQueue is closed but processDone is not yet
			// closed (processLoop still draining handlers).
			close(s.inboundQueue)
			s.inboundClosed.Store(true)
			<-s.processDone
			s.inboundClosed.Store(false) // processDone is now closed; reset for AttachSocket
			// Flush any pending coalesced timeline events BEFORE closing sendQueue
			s.coalescer.FlushAll()
			// Mark sendQueue as closed BEFORE closing it so concurrent senders
			// (e.g. OutputCoalescer flush -> SendBinaryFrame) see the closed
			// state and return without touching the channel.
			s.closeSendQueue()
			closeWSConn(conn)
			if !waitForWritePump(s.writeDone, writePumpDrainTimeout) {
				s.logger.Warn("write pump did not stop before grace; continuing cleanup")
			}
			// Enter grace period instead of destroying the session
			s.enterGrace()
			return
		}

		select {
		case s.inboundQueue <- inboundQueueItem{msgType: msgType, data: data}:
		default:
			s.logger.Warn("inbound queue full, dropping message", "type", msgType)
		}
	}
}

// processLoop drains the inbound queue and dispatches each message to its own
// goroutine, mirroring the non-blocking behaviour of Solo's async event loop.
// A WaitGroup ensures that all in-flight handlers finish before processDone is
// closed, so the shutdown sequence in Run() can safely drain the write pump.
func (s *Session) processLoop() {
	defer close(s.processDone)
	var wg sync.WaitGroup
	for item := range s.inboundQueue {
		wg.Add(1)
		item := item
		metrics.MessagesReceivedTotal.Inc()
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("handler panic recovered", "panic", r)
				}
			}()
			switch item.msgType {
			case websocket.TextMessage:
				s.handleTextMessage(item.data)
			case websocket.BinaryMessage:
				s.handleBinaryMessage(item.data)
			}
		}()
	}
	wg.Wait()
}

// pingInterval is how often the server sends WebSocket ping frames.
// Atomic so tests can override it for fast execution without races.
var pingInterval atomic.Int64

func init() { pingInterval.Store(int64(5 * time.Second)) }

// pingTimeout is the maximum time to wait for a pong response.
// If no pong is received within this time after a ping, the read deadline
// expires and ReadMessage() returns an error, triggering grace period.
const pingTimeout = 5 * time.Second

// mobilePingTimeout is the extended read deadline for mobile clients
// which may have their network suspended when backgrounded by the OS.
// iOS and Android can suspend network activity for up to ~30s; 60s gives
// ample headroom to survive a background/foreground cycle.
const mobilePingTimeout = 60 * time.Second

// effectivePingTimeout returns the appropriate ping timeout for this session's
// client type. Mobile clients get a longer timeout to survive OS-level network
// suspension when the app is backgrounded.
func (s *Session) effectivePingTimeout() time.Duration {
	if s.clientType == string(protocol.ClientMobile) {
		return mobilePingTimeout
	}
	return pingTimeout
}

// setupPingPong configures server-initiated ping/pong keepalive if the
// connection supports it. This detects half-open connections where the
// client (e.g. iOS app backgrounded) has silently disconnected.
func (s *Session) setupPingPong() {
	pc, ok := s.conn.(PingableConn)
	if !ok {
		return
	}

	timeout := s.effectivePingTimeout()

	// When a pong is received, extend deadline to allow the next full ping-pong cycle.
	pc.SetPongHandler(func(appData string) error {
		return pc.SetReadDeadline(time.Now().Add(time.Duration(pingInterval.Load()) + timeout))
	})

	// Set initial read deadline to allow the first full ping-pong cycle:
	// pingInterval (time until first ping is sent) + timeout (time for pong to arrive).
	pc.SetReadDeadline(time.Now().Add(time.Duration(pingInterval.Load()) + timeout))
}

// writePump drains the send queue and writes to the WebSocket.
// If a write fails, it closes the connection so the client can reconnect.
// Ping/pong keepalive runs in a separate goroutine (pingLoop) so that pings
// are never starved by a busy sendQueue.
func (s *Session) writePump() {
	defer close(s.writeDone)

	// Layer 1 WebSocket ping is skipped for relay (E2EE) connections.
	// The relay already has its own keepalive mechanism (Layer 2 JSON ping/pong
	// on the control socket every 10s). Sending protocol-level Ping frames
	// through the encrypted channel can interfere with the relay's
	// perMessageDeflate or E2EE state machine, causing premature data socket
	// closure and cascading client disconnections.
	if !s.isRelay {
		if pc, ok := s.conn.(PingableConn); ok {
			go s.pingLoop(pc)
		}
	}

	for {
		item, ok := s.sendQueue.Pop()
		if !ok {
			return // queue closed and drained
		}
		if err := writeMessageWithDeadline(s.conn, item.msgType, item.data); err != nil {
			s.logger.Warn("write error, closing connection to trigger reconnect", "error", err)
			go s.conn.Close()
			s.sendQueue.Drain()
			return
		}
	}
}

// pingLoop sends periodic WebSocket ping frames to detect half-open connections.
// It runs in its own goroutine so it's never starved by a busy sendQueue.
// WriteControl is explicitly documented as concurrent-safe by gorilla/websocket,
// so it can be called from this goroutine while writePump calls WriteMessage.
func (s *Session) pingLoop(pc PingableConn) {
	ticker := time.NewTicker(time.Duration(pingInterval.Load()))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Add(time.Duration(pingInterval.Load()))
			if err := pc.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				s.logger.Warn("ping failed, closing connection", "error", err)
				go s.conn.Close()
				return
			}
		case <-s.done:
			return
		}
	}
}

func (s *Session) handleTextMessage(data []byte) {
	var wsMsg protocol.WSInboundMessage
	if err := json.Unmarshal(data, &wsMsg); err != nil {
		s.logger.Warn("invalid JSON message", "error", err)
		return
	}

	switch wsMsg.Type {
	case "ping":
		s.sendPong()
	case "session":
		s.handleSessionMessage(wsMsg.Message)
	case "recording_state":
	default:
		s.logger.Warn("unknown WS message type", "type", wsMsg.Type)
	}
}

func (s *Session) handleBinaryMessage(data []byte) {
	frame := protocol.DecodeTerminalFrame(data)
	if frame == nil {
		s.logger.Warn("invalid binary frame")
		return
	}
	switch frame.Opcode {
	case protocol.TerminalInput:
		s.handleTerminalInputBinary(frame.Slot, frame.Payload)
	case protocol.TerminalResize:
		s.handleTerminalResizeBinary(frame.Slot, frame.Payload)
	default:
		s.logger.Debug("unhandled binary opcode", "opcode", frame.Opcode)
	}
}

func (s *Session) handleSessionMessage(raw json.RawMessage) {
	msg, err := protocol.DecodeSessionInboundMessage(raw)
	if err != nil {
		s.logger.Warn("invalid session message", "error", err)
		s.sendRPCError("", "", err.Error(), nil)
		return
	}

	s.handlerRegistry.Handle(s, msg)
}

// --- Message Handlers ---

func (s *Session) handlePing(m *protocol.PingMessage) {
	now := time.Now().UnixMilli()
	pong := protocol.NewSessionMessage(&protocol.PongMessage{
		Type: "pong",
		Payload: protocol.PongPayload{
			RequestID:        m.RequestID,
			ServerReceivedAt: now,
			ServerSentAt:     now,
		},
	})
	s.sendMessage(pong)
}

// relPath returns the path of target relative to base, using forward slashes.
// If target equals base, it returns ".". On error, it falls back to target.

// isTextContent checks if data is likely text by looking for null bytes.

// --- Agent Event Handling ---

// handleAgentEvent processes events from the AgentManager.

// handleStreamEvent processes a stream event from an agent session.

// handleCoalescedFlush processes a coalesced timeline item.

// pushActiveAgents sends agent_update upserts for all currently active agents
// to this session. Called once on connection so the client store is populated
// before any agent_stream events arrive.

// sendAgentStream sends an agent_stream message to the client.

// sendProviderSnapshot sends the current provider snapshot to the client.

// handleGetProvidersSnapshot handles get_providers_snapshot_request.

// handleOpenProject handles open_project_request: registers a workspace and returns its descriptor.

// handleFetchAgentHistory handles fetch_agent_history_request.

// handleSetAgentThinking handles set_agent_thinking_request.

// handleSetAgentFeature handles set_agent_feature_request.

// handleUpdateAgent handles update_agent_request.

// handleRefreshAgent handles refresh_agent_request.

// handleCloseItems handles close_items_request.

// --- Terminal Handlers ---

// subscribeTerminalOutput subscribes to a terminal's output and sends it as binary frames.

// extractTimelineItem converts a timeline item from either agent.TimelineItem
// or map[string]interface{} (fallback) to agent.TimelineItem.

// --- Grace Period ---

// enterGrace transitions the session into the grace state. The session's
// agent subscriptions and terminal subscriptions are preserved. A timer is
// started; if it fires before ReplaceConn is called, the session is fully
// cleaned up.
func (s *Session) enterGrace() {
	s.graceMu.Lock()
	defer s.graceMu.Unlock()
	if s.graceExpired {
		return
	}
	if s.gracePeriod <= 0 {
		// Grace period disabled — destroy immediately
		s.graceExpired = true
		s.closeDone()
		s.fullCleanup()
		return
	}
	s.graceTimer = time.AfterFunc(s.gracePeriod, s.expireGrace)
	s.logger.Info("session entered grace period", "gracePeriod", s.gracePeriod)
}

// maxGraceExtensions caps the number of grace timer extensions to prevent a
// stuck agent from keeping the session alive indefinitely. Each extension is
// one gracePeriod; 10 extensions at 90s = 15 minutes max extra time.
const maxGraceExtensions = 10

// expireGrace is called by the grace timer when the reconnect window expires.
// If any agent is still processing a turn, the grace period is extended rather
// than expiring. This prevents session cleanup during long-running turns (e.g.
// git commit helper with multiple tool calls taking >90s).
func (s *Session) expireGrace() {
	s.graceMu.Lock()

	// If any agent is still running AND making progress, extend grace instead
	// of expiring. This prevents session cleanup during long-running turns,
	// but also prevents a stuck agent (repetitive output, no events) from
	// keeping the session alive indefinitely.
	if s.hasRunningAgentsWithProgress() && s.graceExtensions < maxGraceExtensions {
		s.graceExtensions++
		s.graceTimer = time.AfterFunc(s.gracePeriod, s.expireGrace)
		s.graceMu.Unlock()
		s.logger.Info("grace extended — agent still running",
			"extensions", s.graceExtensions, "maxExtensions", maxGraceExtensions)
		return
	}

	s.graceExpired = true
	s.graceTimer = nil
	s.graceCriticalBuf = nil
	s.graceMu.Unlock()

	s.closeDone()
	s.fullCleanup()

	if s.onGraceExpire != nil {
		s.onGraceExpire()
	}
	s.logger.Info("session grace expired, fully cleaned up")
}

// hasRunningAgentsWithProgress returns true if any agent is
// LifecycleRunning AND has produced stream events recently (i.e. is not
// stalled). This prevents stuck agents from keeping the session alive.
func (s *Session) hasRunningAgentsWithProgress() bool {
	if s.agentMgr == nil {
		return false
	}
	return s.agentMgr.HasRunningAgentsWithRecentProgress()
}

// fullCleanup tears down all subscriptions and state. Called either on grace
// expiry or on server shutdown.
func (s *Session) fullCleanup() {
	s.cleanupOnce.Do(func() {
		for _, unsub := range s.terminalSubscriptions {
			unsub()
		}
		s.terminalSubscriptions = nil
		if s.unsub != nil {
			s.unsub()
			s.unsub = nil
		}
		if s.coalescerFlushID != 0 {
			s.agentMgr.UnregisterCoalescerFlusher(s.coalescerFlushID)
			s.coalescerFlushID = 0
		}
		s.cleanupActivityTracker()
	})
}

// IsInGrace reports whether the session is in the grace period — i.e. the
// client has disconnected but the session is waiting for a potential reconnect.
func (s *Session) IsInGrace() bool {
	s.graceMu.Lock()
	defer s.graceMu.Unlock()
	return s.graceTimer != nil && !s.graceExpired
}

// IsAttaching reports whether an AttachSocket call is currently in progress
// between cancelling the grace timer and registering the socket in s.sockets.
// During this window IsInGrace() returns false, but the session is being
// actively resumed and must not be replaced by a concurrent connection.
func (s *Session) IsAttaching() bool {
	return s.attachingCount.Load() > 0
}

func (s *Session) closeDone() {
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

func (s *Session) closeSendQueue() {
	s.sendQueue.Close()
}

func (s *Session) shutdownForReplacement() {
	s.graceMu.Lock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}
	s.graceExpired = true
	s.graceCriticalBuf = nil
	s.graceMu.Unlock()

	s.closeSendQueue()
	s.connMu.Lock()
	conn := s.conn
	s.connMu.Unlock()
	closeWSConn(conn)
	s.closeAttachedSocketsForReplacement()

	// Close the in-flight attaching conn (if any) so the blocking
	// readLoopFor inside AttachSocket unblocks and the goroutine exits.
	s.attachingConnMu.Lock()
	if s.attachingConn != nil {
		closeWSConn(s.attachingConn)
		s.attachingConn = nil
	}
	s.attachingConnMu.Unlock()

	s.closeDone()
	s.fullCleanup()
}

func (s *Session) closeAttachedSocketsForReplacement() {
	s.socketsMu.Lock()
	entries := make([]socketEntry, 0, len(s.sockets))
	for _, entry := range s.sockets {
		entries = append(entries, entry)
	}
	s.sockets = make(map[string]socketEntry)
	s.socketsMu.Unlock()

	for _, entry := range entries {
		closeWSConn(entry.conn)
	}
}

// ReplaceConn attempts to resume a grace-period session with a new WebSocket
// connection. It stops the grace timer, rebuilds internal channels, restarts
// the write/process pumps, replays buffered critical messages, and enters the
// read loop with the new connection.
// Returns an error if the session is not in grace or grace has already expired.
func (s *Session) ReplaceConn(newConn WSConn) error {
	s.graceMu.Lock()
	if s.graceTimer == nil || s.graceExpired {
		s.graceMu.Unlock()
		return fmt.Errorf("session not in grace period or grace expired")
	}
	s.graceTimer.Stop()
	s.graceTimer = nil
	// Capture and clear the buffered critical messages from grace period
	buf := s.graceCriticalBuf
	s.graceCriticalBuf = nil
	s.graceMu.Unlock()

	s.connMu.Lock()
	s.conn = newConn
	s.connMu.Unlock()

	// Recreate channels for the new run cycle
	s.sendQueue = newSendQueue()
	s.writeDone = make(chan struct{})
	s.inboundQueue = make(chan inboundQueueItem, 64)
	s.processDone = make(chan struct{})

	// Start the pumps again
	go s.writePump()
	go s.processLoop()

	// Push current state to the reconnected client
	s.pushActiveAgents()
	s.sendProviderSnapshot()

	// Replay buffered critical messages (turn_completed, agent_update, etc.)
	// that arrived during grace period. These are enqueued directly into the
	// sendQueue to avoid going through sendMessage again.
	for _, item := range buf {
		s.sendQueue.Push(item)
	}

	s.logger.Info("session resumed via ReplaceConn", "replayedMessages", len(buf))
	// Enter read loop with the new connection (blocking)
	s.runReadLoop()
	return nil
}

// --- Push Notification ---

// SetPushTokenStore sets the push token store for this session.
func (s *Session) SetPushTokenStore(store push.TokenStore) {
	s.pushTokenStore = store
}

// SetActivityTracker sets the activity tracker for this session.
func (s *Session) SetActivityTracker(tracker ActivityTracker) {
	s.activityTracker = tracker
}

// cleanupActivityTracker removes this session's client from the activity tracker.
func (s *Session) cleanupActivityTracker() {
	if s.activityTracker != nil && s.clientID != "" {
		s.activityTracker.Remove(s.clientID)
	}
}

// SetPusher sets the push service for this session.
func (s *Session) SetPusher(p push.Pusher) {
	s.pusher = p
}

// handleRegisterPushToken handles register_push_token messages.
func (s *Session) handleRegisterPushToken(msg *protocol.RegisterPushTokenMessage) {
	if s.pushTokenStore == nil {
		s.logger.Warn("push token store not configured, ignoring registration")
		return
	}
	if msg.Token == "" {
		s.logger.Warn("empty push token received, ignoring")
		return
	}

	s.pushTokenStore.Register(msg.Token)
	s.logger.Info("push token registered", "tokenPrefix", msg.Token[:min(len(msg.Token), 8)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
