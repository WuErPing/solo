package server

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// socketEntry holds per-socket send state for the multi-socket model.
type socketEntry struct {
	id        string
	conn      WSConn
	sendQueue *sendQueue
	writeDone chan struct{}
	done      chan struct{}
}

// socketSeqGlobal generates unique socket IDs across all sessions.
var socketSeqGlobal atomic.Uint64

// startBackgroundLoops starts the session's background goroutines (agent
// subscription, inbound processor) without entering a connection read loop.
// This is a prerequisite for the multi-socket model where the session lifecycle
// is independent of any single WebSocket connection.
//
// It is idempotent: calling it more than once is a no-op.
func (s *Session) startBackgroundLoops() {
	if s.unsub != nil {
		return
	}
	s.unsub = s.agentMgr.Subscribe(s.handleAgentEvent)
	// Register coalescer flush so buffered timeline entries are flushed
	// even when the agent event pipeline is congested (workCh fallback path).
	if s.coalescerFlushID == 0 {
		s.coalescerFlushID = s.agentMgr.RegisterCoalescerFlusher(func(agentID string) {
			s.coalescer.FlushFor(agentID)
		})
	}
	go s.processLoop()
}

// AttachSocket attaches a WebSocket connection to this session as an additional
// I/O channel. Multiple concurrent sockets are supported; all outbound messages
// are broadcast to every attached socket.
//
// It:
//  1. Cancels any in-progress grace timer (reconnect cancels grace).
//  2. Starts background loops if not already running.
//  3. Starts a per-socket write pump and ping loop.
//  4. Pushes current state (active agents + provider snapshot) to this socket.
//  5. Blocks in a per-socket read loop until the socket disconnects.
//  6. On last-socket disconnect: starts grace period.
//
// This is the multi-socket equivalent of Run(). Existing single-socket code
// that calls Run() continues to work unchanged.
func (s *Session) AttachSocket(conn WSConn) {
	id := fmt.Sprintf("sock-%d", socketSeqGlobal.Add(1))
	entry := socketEntry{
		id:        id,
		conn:      conn,
		sendQueue: newSendQueue(),
		writeDone: make(chan struct{}),
		done:      make(chan struct{}),
	}

	// Signal that this AttachSocket call is in progress. This prevents a
	// concurrent handleNewConnection from calling shutdownForReplacement
	// during the window between grace cancellation and socket registration.
	s.attachingCount.Add(1)
	defer s.attachingCount.Add(-1)

	// Store the attaching conn so shutdownForReplacement can close it,
	// forcing the subsequent readLoopFor to unblock and exit cleanly.
	s.attachingConnMu.Lock()
	s.attachingConn = conn
	s.attachingConnMu.Unlock()
	defer func() {
		s.attachingConnMu.Lock()
		s.attachingConn = nil
		s.attachingConnMu.Unlock()
	}()

	// Cancel grace timer if session is in grace (reconnect cancels grace).
	s.graceMu.Lock()
	if s.graceTimer != nil && !s.graceExpired {
		s.graceTimer.Stop()
		s.graceTimer = nil
		s.logger.Info("grace cancelled by new socket attach", "socketId", id)
	}
	s.graceMu.Unlock()

	// If this session went through the legacy Run() path, inboundQueue,
	// processDone and sendQueue were closed when the previous connection
	// disconnected. Recreate them so readLoopFor and SendBinaryFrame can
	// safely operate.
	//
	// There is a narrow race window: runReadLoop closes inboundQueue and sets
	// inboundClosed=true *before* <-processDone completes (processLoop may still
	// be draining in-flight handlers via wg.Wait). If AttachSocket runs during
	// this window, processDone is not yet closed (default branch), but inboundQueue
	// is already closed. readLoopFor sending to a closed channel would panic.
	//
	// Fix: if inboundClosed is set, wait for processDone to ensure the old
	// processLoop has fully exited before rebuilding the channels.
	if s.inboundClosed.Load() {
		// runReadLoop closed inboundQueue but processLoop hasn't exited yet.
		// Wait for it to finish so we can safely rebuild.
		<-s.processDone
	}
	select {
	case <-s.processDone:
		// processDone is closed — previous processLoop has exited; recreate channels.
		s.inboundQueue = make(chan inboundQueueItem, 64)
		s.processDone = make(chan struct{})
		s.inboundClosed.Store(false)
		// Recreate the legacy sendQueue so sendMessage() can enqueue items
		// during the brief window before the socket is registered below.
		// Do NOT restart writePump here: writePump writes to s.conn (the old
		// dead pre-grace connection) and spawns pingLoop on it, which causes
		// "ping failed, closing connection" 5s after every multi-socket attach.
		// In the multi-socket path, writePumpFor/pingLoopFor handle each socket.
		s.sendQueue = newSendQueue()
		s.writeDone = make(chan struct{})
		go s.processLoop()
	default:
		// processLoop is still running (either never used Run(), or startBackgroundLoops
		// started it) — no need to recreate.
	}

	// Register this socket.
	s.socketsMu.Lock()
	if s.sockets == nil {
		s.sockets = make(map[string]socketEntry)
	}
	s.sockets[id] = entry
	s.socketsMu.Unlock()

	// Ensure background loops are running (idempotent).
	s.startBackgroundLoops()

	// Configure pong handler and initial read deadline.
	s.setupPingPongFor(entry)

	// Start per-socket write pump.
	go s.writePumpFor(entry)

	// Push current state to the newly attached socket.
	s.pushActiveAgentsTo(entry)
	// sendProviderSnapshotTo may block while fetching live models (e.g. ListModels
	// for OpenCode makes an HTTP request). Run it in a goroutine so readLoopFor
	// starts immediately and can process inbound messages without delay.
	go s.sendProviderSnapshotTo(entry)

	// Replay any critical messages that were buffered during grace period.
	// This matches ReplaceConn's behavior for the legacy single-socket path.
	// Use non-blocking sends: if the sendQueue is already full (e.g. the writePump
	// hasn't drained yet), drop the overflow item rather than stalling AttachSocket
	// for criticalSendTimeout. The client will receive live state from agent events.
	s.graceMu.Lock()
	replay := s.graceCriticalBuf
	s.graceCriticalBuf = nil
	s.graceMu.Unlock()

	for _, item := range replay {
		entry.sendQueue.Push(item)
	}

	// Block in read loop until this socket disconnects.
	s.readLoopFor(entry)

	// Socket disconnected — remove from map.
	s.socketsMu.Lock()
	delete(s.sockets, id)
	empty := len(s.sockets) == 0
	s.socketsMu.Unlock()

	// Stop the write pump and wait for it to drain.
	entry.sendQueue.Close()
	closeWSConn(entry.conn)
	if !waitForWritePump(entry.writeDone, writePumpDrainTimeout) {
		s.logger.Warn("socket write pump did not stop before grace check", "socketId", id)
	}

	if empty {
		s.enterGrace()
	}
}

// readLoopFor is the per-socket read loop. Returns when the socket errors or closes.
func (s *Session) readLoopFor(entry socketEntry) {
	defer entry.conn.Close()

	for {
		msgType, data, err := entry.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Info("socket read error", "socketId", entry.id, "error", err)
			} else {
				s.logger.Info("socket closed", "socketId", entry.id)
			}
			return
		}

		select {
		case s.inboundQueue <- inboundQueueItem{msgType: msgType, data: data}:
		default:
			s.logger.Warn("inbound queue full, dropping message", "socketId", entry.id, "type", msgType)
		}
	}
}

// writePumpFor drains a socket entry's send queue and writes to the WebSocket.
// Runs as a goroutine; closes entry.writeDone when it exits.
func (s *Session) writePumpFor(entry socketEntry) {
	defer close(entry.writeDone)

	// Start a dedicated ping loop for this socket.
	if pc, ok := entry.conn.(PingableConn); ok {
		go s.pingLoopFor(pc, entry)
	}

	for {
		item, ok := entry.sendQueue.Pop()
		if !ok {
			return
		}
		if err := writeMessageWithDeadline(entry.conn, item.msgType, item.data); err != nil {
			s.logger.Warn("socket write error, closing", "socketId", entry.id, "error", err)
			go entry.conn.Close()
			entry.sendQueue.Drain()
			return
		}
	}
}

// pingLoopFor sends periodic WebSocket ping frames for a specific socket.
// Exits when the socket's done channel is closed (sendQueue closed by AttachSocket).
func (s *Session) pingLoopFor(pc PingableConn, entry socketEntry) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Add(pingInterval)
			if err := pc.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				s.logger.Warn("socket ping failed", "socketId", entry.id, "error", err)
				go entry.conn.Close()
				return
			}
		case <-entry.writeDone:
			return
		case <-s.done:
			return
		}
	}
}

// setupPingPongFor configures the pong handler and initial read deadline for a socket.
func (s *Session) setupPingPongFor(entry socketEntry) {
	pc, ok := entry.conn.(PingableConn)
	if !ok {
		return
	}
	timeout := s.effectivePingTimeout()
	// On each pong, extend deadline to allow the next full ping-pong cycle.
	pc.SetPongHandler(func(appData string) error {
		return pc.SetReadDeadline(time.Now().Add(pingInterval + timeout))
	})
	// Initial deadline covers the first full ping-pong cycle.
	pc.SetReadDeadline(time.Now().Add(pingInterval + timeout))
}

// broadcastToSockets enqueues item to every currently attached socket.
func (s *Session) broadcastToSockets(item sendQueueItem) {
	s.socketsMu.RLock()
	defer s.socketsMu.RUnlock()

	for _, e := range s.sockets {
		e.sendQueue.Push(item)
	}
}

// pushActiveAgentsTo sends agent_update upserts for all active agents directly
// to a specific socket entry (called on attach, before the socket is publicly
// registered for broadcast).
func (s *Session) pushActiveAgentsTo(entry socketEntry) {
	for _, ag := range s.agentMgr.ListAgentsWithPersisted() {
		snapshot := ag.ToSnapshot()
		msg := protocol.NewSessionMessage(&protocol.AgentUpdateMessage{
			Type: "agent_update",
			Payload: protocol.AgentUpdatePayload{
				Kind:    "upsert",
				Agent:   &snapshot,
				Project: s.projectPlacementForAgent(ag),
			},
		})
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		entry.sendQueue.Push(sendQueueItem{msgType: websocket.TextMessage, data: data})
	}
}

// sendProviderSnapshotTo sends the provider snapshot directly to a specific socket entry.
// Safe to call from a goroutine: uses recover to handle the case where the socket
// disconnects (and entry.sendQueue is closed) while ListModels is still running.
func (s *Session) sendProviderSnapshotTo(entry socketEntry) {
	defer func() { recover() }() //nolint:errcheck // intentional: guard against send-on-closed-channel
	entries := s.registry.ToProviderSnapshotEntries()
	msg := protocol.NewSessionMessage(&protocol.ProvidersSnapshotUpdate{
		Type: "providers_snapshot_update",
		Payload: protocol.ProvidersSnapshotPayload{
			Entries:     entries,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
	})
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	entry.sendQueue.Push(sendQueueItem{msgType: websocket.TextMessage, data: data})
}
