// Package bridge adapts session-level user/assistant turn events into the
// memory.TurnRecorder contract. It builds Turn records, applies redaction,
// maintains per-session seq/parent chains, and forwards to the recorder.
//
// The Bridge is designed to be called synchronously from session hooks;
// it never blocks on I/O because TurnRecorder.RecordTurn only enqueues.
package bridge

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// Bridge observes session turn events and persists them through a
// memory.TurnRecorder. Safe for concurrent use.
type Bridge struct {
	recorder memory.TurnRecorder
	redactor memory.Redactor
	clock    memory.Clock
	logger   *slog.Logger

	mu       sync.Mutex
	sessions map[string]*sessionState

	// chunkMu guards chunks, which accumulates streaming assistant text
	// per agentID until the enclosing turn ends (OnAssistantTurnEnd) or
	// the bridge shuts down (Close).
	chunkMu sync.Mutex
	chunks  map[string]*chunkBuffer
}

// chunkBuffer is the in-flight assistant text for a single agent, plus
// the sessionID captured from the first chunk so the eventual turn can
// be recorded without the server re-sending context.
type chunkBuffer struct {
	sessionID string
	parts     []string
}

// Close flushes every pending chunk buffer to the recorder (one turn
// per agent with buffered content) and releases resources. Safe to
// call multiple times.
func (b *Bridge) Close() error {
	b.chunkMu.Lock()
	pending := b.chunks
	b.chunks = make(map[string]*chunkBuffer)
	b.chunkMu.Unlock()

	for agentID, buf := range pending {
		b.flushChunk(agentID, buf)
	}
	return nil
}

// sessionState tracks the seq counter and last turn ID for one session,
// enabling the parent chain across turns. mu guards the full assign →
// record → update sequence so concurrent turns on the same session
// produce a consistent chain.
type sessionState struct {
	mu       sync.Mutex
	nextSeq  uint64
	lastTurn string
}

// Option configures a Bridge.
type Option func(*Bridge)

// WithRedactor installs a Redactor; defaults to memory.NoopRedactor.
func WithRedactor(r memory.Redactor) Option {
	return func(b *Bridge) { b.redactor = r }
}

// WithClock installs a Clock; defaults to memory.SystemClock.
func WithClock(c memory.Clock) Option {
	return func(b *Bridge) { b.clock = c }
}

// WithLogger installs a logger; defaults to slog.Default.
func WithLogger(l *slog.Logger) Option {
	return func(b *Bridge) { b.logger = l }
}

// New constructs a Bridge. recorder must be non-nil.
func New(recorder memory.TurnRecorder, opts ...Option) (*Bridge, error) {
	if recorder == nil {
		return nil, errNilRecorder
	}
	b := &Bridge{
		recorder: recorder,
		redactor: memory.NoopRedactor{},
		clock:    memory.SystemClock{},
		logger:   slog.Default(),
		sessions: make(map[string]*sessionState),
		chunks:   make(map[string]*chunkBuffer),
	}
	for _, opt := range opts {
		opt(b)
	}
	if b.redactor == nil {
		b.redactor = memory.NoopRedactor{}
	}
	if b.clock == nil {
		b.clock = memory.SystemClock{}
	}
	if b.logger == nil {
		b.logger = slog.Default()
	}
	return b, nil
}

// OnUserTurn records a user turn.
func (b *Bridge) OnUserTurn(sessionID, agentID, content string) {
	b.record(sessionID, agentID, memory.RoleUser, content)
}

// OnAssistantTurn records an assistant turn immediately. Use this for
// one-shot assistant messages (e.g. attention_required broadcasts). For
// streaming assistant output, prefer OnAssistantChunk + OnAssistantTurnEnd
// so the chunks coalesce into a single persisted turn.
func (b *Bridge) OnAssistantTurn(sessionID, agentID, content string) {
	b.record(sessionID, agentID, memory.RoleAssistant, content)
}

// OnAssistantChunk appends a streaming-assistant text fragment to the
// per-agent buffer. The fragment is NOT persisted yet; call
// OnAssistantTurnEnd (or Close) to flush the accumulated content as a
// single assistant turn. Empty fragments are ignored.
func (b *Bridge) OnAssistantChunk(agentID, sessionID, fragment string) {
	if agentID == "" || fragment == "" {
		return
	}

	b.chunkMu.Lock()
	defer b.chunkMu.Unlock()

	buf, ok := b.chunks[agentID]
	if !ok {
		buf = &chunkBuffer{sessionID: sessionID}
		b.chunks[agentID] = buf
	}
	buf.parts = append(buf.parts, fragment)
}

// OnAssistantTurnEnd closes the current streaming assistant turn for the
// named agent: accumulated chunks are joined, redacted, and persisted
// as one assistant turn. No-op if there are no buffered chunks for the
// agent (so a stray turn_completed without prior chunks is harmless).
//
// The sessionID parameter is part of the public contract for symmetry
// with OnAssistantChunk; the buffered sessionID captured from the first
// chunk is what gets recorded.
//
//nolint:revive // sessionID kept for API symmetry; buffer carries the captured one.
func (b *Bridge) OnAssistantTurnEnd(agentID, sessionID string) {
	b.chunkMu.Lock()
	buf, ok := b.chunks[agentID]
	if ok {
		delete(b.chunks, agentID)
	}
	b.chunkMu.Unlock()

	if !ok || len(buf.parts) == 0 {
		return
	}
	b.flushChunk(agentID, buf)
}

// flushChunk joins the buffered parts, redacts the combined text, and
// records the resulting assistant turn. sessionID comes from the buffer
// (captured on first chunk) so a Close-time flush still knows where the
// turn belongs.
func (b *Bridge) flushChunk(agentID string, buf *chunkBuffer) {
	combined := strings.Join(buf.parts, "")
	b.record(buf.sessionID, agentID, memory.RoleAssistant, combined)
}

// OnSystemTurn records a system turn (e.g., system prompt).
func (b *Bridge) OnSystemTurn(sessionID, agentID, content string) {
	b.record(sessionID, agentID, memory.RoleSystem, content)
}

func (b *Bridge) record(sessionID, agentID string, role memory.TurnRole, content string) {
	if sessionID == "" || agentID == "" {
		return
	}

	redacted := b.redactor.Redact(content)

	b.mu.Lock()
	state, ok := b.sessions[sessionID]
	if !ok {
		state = &sessionState{nextSeq: 1}
		b.sessions[sessionID] = state
	}
	b.mu.Unlock()

	// Serialize all turns on the same session so that seq assignment,
	// recording, and parent-chain update happen atomically.
	state.mu.Lock()
	defer state.mu.Unlock()

	seq := state.nextSeq
	parent := state.lastTurn

	turn := memory.Turn{
		ID:        memory.NewTurnID(),
		SessionID: sessionID,
		Seq:       seq,
		Role:      role,
		Ts:        b.clock.Now(),
		Source:    memory.SourceCLI,
		Content:   redacted,
		ParentID:  parent,
	}

	if err := b.recorder.RecordTurn(context.Background(), sessionID, turn); err != nil {
		b.logger.Warn("memory: record failed",
			"sessionID", sessionID,
			"seq", seq,
			"err", err,
		)
		return
	}

	state.nextSeq = seq + 1
	state.lastTurn = turn.ID
}
