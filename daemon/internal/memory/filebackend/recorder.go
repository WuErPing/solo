package filebackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// FileTurnRecorder persists turns as markdown files under
// <BaseDir>/<root>/sessions/{YYYY-MM}/{sessionID}/turns/{seq:04d}-{role}.md
// and maintains a sessions.jsonl index at <BaseDir>/<root>/sessions.jsonl.
//
// In production, BaseDir is SoloHome (~/.solo) and root defaults to "memory",
// so the full path is ~/.solo/memory/sessions/...
//
// Writes are performed by a single background goroutine to keep the hot
// path (RecordTurn) non-blocking.
type FileTurnRecorder struct {
	cfg Config

	turnCh     chan memory.Turn
	flushReqCh chan flushReq
	doneCh     chan struct{}

	closeOnce sync.Once
	closed    bool
	closeMu   sync.Mutex
	flushMu   sync.Mutex

	// sessions tracks session IDs that have already been written to the
	// index, so each session produces exactly one JSONL entry.
	sessions   map[string]bool
	sessionsMu sync.Mutex
}

type flushReq struct {
	ctx  context.Context
	done chan error
}

// New constructs a FileTurnRecorder and starts its background writer.
func New(cfg Config) (*FileTurnRecorder, error) {
	if err := cfg.ApplyDefaults(); err != nil {
		return nil, err
	}
	r := &FileTurnRecorder{
		cfg:        cfg,
		turnCh:     make(chan memory.Turn, cfg.QueueSize),
		flushReqCh: make(chan flushReq),
		doneCh:     make(chan struct{}),
		sessions:   make(map[string]bool),
	}
	go r.writeLoop()
	return r, nil
}

// RecordTurn enqueues a turn. Returns memory.ErrClosed after Close, or a
// validation error if the turn is malformed. Overflow behavior is governed
// by cfg.Overflow.
func (r *FileTurnRecorder) RecordTurn(ctx context.Context, _ string, turn memory.Turn) error {
	r.closeMu.Lock()
	if r.closed {
		r.closeMu.Unlock()
		return memory.ErrClosed
	}
	r.closeMu.Unlock()

	if err := turn.Validate(); err != nil {
		return err
	}

	switch r.cfg.Overflow {
	case OverflowError:
		select {
		case r.turnCh <- turn:
			return nil
		default:
			return errors.New("file: turn queue full")
		}
	case OverflowBlock:
		fallthrough
	default:
		select {
		case r.turnCh <- turn:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Flush blocks until every turn enqueued before this call has been
// persisted. It is safe to call repeatedly.
func (r *FileTurnRecorder) Flush(ctx context.Context) error {
	r.flushMu.Lock()
	defer r.flushMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	done := make(chan error, 1)
	select {
	case r.flushReqCh <- flushReq{ctx: ctx, done: done}:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close flushes pending turns and stops the writer. Idempotent.
func (r *FileTurnRecorder) Close() error {
	var err error
	r.closeOnce.Do(func() {
		r.closeMu.Lock()
		r.closed = true
		r.closeMu.Unlock()
		close(r.turnCh)
		<-r.doneCh
	})
	return err
}

// writeLoop is the single background goroutine that performs all I/O.
func (r *FileTurnRecorder) writeLoop() {
	defer close(r.doneCh)

	for {
		select {
		case turn, ok := <-r.turnCh:
			if !ok {
				// Drain any last-moment flush requests that raced with Close.
				r.drainFlushReqs()
				return
			}
			r.persistTurn(turn)

		case req := <-r.flushReqCh:
			// Drain all pending turns before acknowledging the flush.
			r.drainTurns()
			select {
			case req.done <- nil:
			default:
			}
		}
	}
}

// drainTurns blocks until turnCh is empty.
func (r *FileTurnRecorder) drainTurns() {
	for {
		select {
		case turn, ok := <-r.turnCh:
			if !ok {
				return
			}
			r.persistTurn(turn)
		default:
			return
		}
	}
}

// drainFlushReqs acknowledges any buffered flush requests during shutdown.
func (r *FileTurnRecorder) drainFlushReqs() {
	for {
		select {
		case req := <-r.flushReqCh:
			select {
			case req.done <- nil:
			default:
			}
		default:
			return
		}
	}
}

// persistTurn writes one turn file and (on first sight) its session index
// entry. Errors are swallowed here; M5 wires them into metrics.
func (r *FileTurnRecorder) persistTurn(turn memory.Turn) {
	ym := turn.Ts.Format("2006-01")
	turnsDir := filepath.Join(r.cfg.BaseDir, r.cfg.Root, "sessions", ym, turn.SessionID, "turns")
	if err := os.MkdirAll(turnsDir, dirPerm); err != nil {
		return
	}

	name := fmt.Sprintf("%04d-%s.md", turn.Seq, turn.Role)
	path := filepath.Join(turnsDir, name)

	// Write-once: never clobber an existing turn file.
	if _, err := os.Stat(path); err == nil {
		return
	}

	fm, err := turn.FrontmatterYAML()
	if err != nil {
		return
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fm)
	sb.WriteString("\n---\n")
	sb.WriteString(turn.Content)

	if err := os.WriteFile(path, []byte(sb.String()), filePerm); err != nil {
		return
	}

	r.maybeWriteSessionIndex(turn)
}

// sessionIndexEntry is the JSONL schema for sessions.jsonl.
type sessionIndexEntry struct {
	ID         string    `json:"id"`
	StartedAt  time.Time `json:"startedAt"`
	TurnsCount int       `json:"turnsCount"`
	Source     string    `json:"source,omitempty"`
}

func (r *FileTurnRecorder) maybeWriteSessionIndex(turn memory.Turn) {
	r.sessionsMu.Lock()
	if r.sessions[turn.SessionID] {
		r.sessionsMu.Unlock()
		return
	}
	r.sessions[turn.SessionID] = true
	r.sessionsMu.Unlock()

	indexPath := filepath.Join(r.cfg.BaseDir, r.cfg.Root, "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), dirPerm); err != nil {
		return
	}

	entry := sessionIndexEntry{
		ID:         turn.SessionID,
		StartedAt:  turn.Ts,
		TurnsCount: 1,
		Source:     string(turn.Source),
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line = append(line, '\n')

	if err := appendToFile(indexPath, line); err != nil {
		return
	}
}

// appendToFile atomically appends data to a file, ensuring Close is
// observed. Errors are returned to the caller.
func appendToFile(path string, data []byte) (err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = f.Write(data)
	return err
}
