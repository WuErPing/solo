package server

import (
	"time"

	"github.com/WuErPing/solo/protocol"
)

// serverControlFlushDelay gives the async send queue a moment to flush the
// status reply onto the wire (including through the relay data socket) before
// the process exit is requested.
const serverControlFlushDelay = 200 * time.Millisecond

// handleRestartServer accepts a restart only when the daemon runs under the
// supervisor (SOLO_SUPERVISED=1): the daemon replies, then exits with
// protocol.ExitCodeRestartRequested so the supervisor respawns it. When
// unsupervised the request is refused — nothing would bring the daemon back.
func (s *Session) handleRestartServer(m *protocol.RestartServerRequest) {
	if !s.cfg.Supervised {
		s.logger.Info("restart refused: daemon is not supervised", "requestId", m.RequestID)
		s.sendRPCError(m.RequestID, m.MsgType(),
			"daemon is not supervised; restart requires solo-supervisor",
			strPtr("NOT_SUPERVISED"))
		return
	}
	reason := ""
	if m.Reason != nil {
		reason = *m.Reason
	}
	s.logger.Info("restart requested", "requestId", m.RequestID, "reason", reason)
	s.sendMessage(protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.RestartRequestedStatusPayload{
			Status:    "restart_requested",
			ClientID:  s.clientID,
			Reason:    m.Reason,
			RequestID: m.RequestID,
		},
	}))
	s.requestExitAfter(protocol.ExitCodeRestartRequested, reason, serverControlFlushDelay)
}

// handleShutdownServer replies and then exits cleanly (exit code 0). Under the
// supervisor a clean exit is not respawned, so this stops the daemon for good.
func (s *Session) handleShutdownServer(m *protocol.ShutdownServerRequest) {
	s.logger.Info("shutdown requested", "requestId", m.RequestID)
	s.sendMessage(protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.ShutdownRequestedStatusPayload{
			Status:    "shutdown_requested",
			ClientID:  s.clientID,
			RequestID: m.RequestID,
		},
	}))
	s.requestExitAfter(protocol.ExitCodeClean, "shutdown requested", serverControlFlushDelay)
}

// requestExitAfter asks the daemon process to exit after a short delay so the
// reply above can be flushed first. Sessions without an exit requester
// (tests, minimal constructions) log and skip.
func (s *Session) requestExitAfter(code int, reason string, delay time.Duration) {
	if s.requestExit == nil {
		s.logger.Warn("exit request ignored: no exit requester wired", "code", code)
		return
	}
	go func() {
		time.Sleep(delay)
		s.requestExit(code, reason)
	}()
}
