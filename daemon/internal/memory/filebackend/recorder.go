package filebackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
// <BaseDir>/<root>/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md
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

	// logger receives error logs from the background writer.
	logger *slog.Logger

	// lastErr captures the first persistence error encountered during the
	// final drain in Close. It is set by writeLoop before closing doneCh.
	lastErr   error
	lastErrMu sync.Mutex

	// persistErr is the first persistence error encountered since the last
	// Flush or Close. It is set by writeLoop/drainTurns and read/reset by
	// Flush so that callers observe background failures.
	persistErr   error
	persistErrMu sync.Mutex

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
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	r := &FileTurnRecorder{
		cfg:        cfg,
		turnCh:     make(chan memory.Turn, cfg.QueueSize),
		flushReqCh: make(chan flushReq),
		doneCh:     make(chan struct{}),
		logger:     logger,
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
// Returns the first persistence error encountered while draining the
// remaining queue; nil if all turns were persisted successfully.
func (r *FileTurnRecorder) Close() error {
	r.closeOnce.Do(func() {
		r.closeMu.Lock()
		r.closed = true
		r.closeMu.Unlock()
		close(r.turnCh)
		<-r.doneCh
	})
	r.lastErrMu.Lock()
	defer r.lastErrMu.Unlock()
	return r.lastErr
}

// writeLoop is the single background goroutine that performs all I/O.
func (r *FileTurnRecorder) writeLoop() {
	defer close(r.doneCh)

	var closeErr error
	for {
		select {
		case turn, ok := <-r.turnCh:
			if !ok {
				// Persist the final drain error so Close can return it.
				r.setLastErr(closeErr)
				// Drain any last-moment flush requests that raced with Close.
				r.drainFlushReqs()
				return
			}
			if err := r.persistTurn(turn); err != nil {
				r.logPersistError(err, turn)
				r.setPersistErr(err)
				if closeErr == nil {
					closeErr = err
				}
			}

		case req := <-r.flushReqCh:
			// Drain all pending turns before acknowledging the flush, then
			// surface any background persistence error that occurred since
			// the last flush.
			err := r.drainTurns()
			if err == nil {
				err = r.takePersistErr()
			}
			select {
			case req.done <- err:
			default:
			}
		}
	}
}

// drainTurns blocks until turnCh is empty and returns the first persistence
// error encountered during the drain.
func (r *FileTurnRecorder) drainTurns() error {
	var err error
	for {
		select {
		case turn, ok := <-r.turnCh:
			if !ok {
				return err
			}
			if perr := r.persistTurn(turn); perr != nil {
				r.logPersistError(perr, turn)
				r.setPersistErr(perr)
				if err == nil {
					err = perr
				}
			}
		default:
			return err
		}
	}
}

// drainFlushReqs acknowledges any buffered flush requests during shutdown.
// Close-time flush requests are answered with the final drain error via
// setLastErr/Close, so we simply unblock them here.
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

// setLastErr stores the first error encountered during the final drain.
func (r *FileTurnRecorder) setLastErr(err error) {
	r.lastErrMu.Lock()
	defer r.lastErrMu.Unlock()
	r.lastErr = err
}

// setPersistErr stores the first persistence error since the last Flush.
func (r *FileTurnRecorder) setPersistErr(err error) {
	r.persistErrMu.Lock()
	defer r.persistErrMu.Unlock()
	if r.persistErr == nil {
		r.persistErr = err
	}
}

// takePersistErr returns and resets the first persistence error since the
// last Flush. Callers should combine it with any drain-time error.
func (r *FileTurnRecorder) takePersistErr() error {
	r.persistErrMu.Lock()
	defer r.persistErrMu.Unlock()
	err := r.persistErr
	r.persistErr = nil
	return err
}

// logPersistError logs a background persistence failure with context.
func (r *FileTurnRecorder) logPersistError(err error, turn memory.Turn) {
	r.logger.Error("failed to persist turn",
		"error", err,
		"sessionID", turn.SessionID,
		"seq", turn.Seq,
		"role", turn.Role,
	)
}

// persistTurn writes one turn file and (on first sight) its session index
// entry. It returns an error if any I/O or serialization step fails.
func (r *FileTurnRecorder) persistTurn(turn memory.Turn) error {
	ymd := turn.Ts.Format("2006-01-02")
	turnsDir := filepath.Join(r.cfg.BaseDir, r.cfg.Root, "sessions", ymd, turn.SessionID, "turns")
	if err := os.MkdirAll(turnsDir, dirPerm); err != nil {
		return fmt.Errorf("mkdir turns dir: %w", err)
	}

	name := fmt.Sprintf("%04d-%s.md", turn.Seq, turn.Role)
	path := filepath.Join(turnsDir, name)

	// Write-once: never clobber an existing turn file.
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat turn file: %w", err)
	}

	fm, err := turn.FrontmatterYAML()
	if err != nil {
		return fmt.Errorf("frontmatter yaml: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fm)
	sb.WriteString("\n---\n")
	sb.WriteString(turn.Content)

	if err := os.WriteFile(path, []byte(sb.String()), filePerm); err != nil {
		return fmt.Errorf("write turn file: %w", err)
	}

	if err := r.maybeWriteSessionIndex(turn); err != nil {
		return fmt.Errorf("session index: %w", err)
	}
	return nil
}

// sessionIndexEntry is the JSONL schema for sessions.jsonl.
type sessionIndexEntry struct {
	ID         string    `json:"id"`
	StartedAt  time.Time `json:"startedAt"`
	TurnsCount int       `json:"turnsCount"`
	Source     string    `json:"source,omitempty"`
}

func (r *FileTurnRecorder) maybeWriteSessionIndex(turn memory.Turn) error {
	r.sessionsMu.Lock()
	if r.sessions[turn.SessionID] {
		r.sessionsMu.Unlock()
		return nil
	}
	r.sessions[turn.SessionID] = true
	r.sessionsMu.Unlock()

	indexPath := filepath.Join(r.cfg.BaseDir, r.cfg.Root, "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(indexPath), dirPerm); err != nil {
		return fmt.Errorf("mkdir index dir: %w", err)
	}

	entry := sessionIndexEntry{
		ID:         turn.SessionID,
		StartedAt:  turn.Ts,
		TurnsCount: 1,
		Source:     string(turn.Source),
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal index entry: %w", err)
	}
	line = append(line, '\n')

	if err := appendToFile(indexPath, line); err != nil {
		return fmt.Errorf("append index entry: %w", err)
	}
	return nil
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
