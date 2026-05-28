package memory

import (
	"context"
	"errors"
)

// ErrClosed is returned by TurnRecorder.RecordTurn after Close has been
// called. Implementations must return an error that errors.Is(err, ErrClosed)
// recognizes (wrapping is allowed).
var ErrClosed = errors.New("memory: turn recorder closed")

// TurnRecorder is the stable contract for persisting turns. Implementations
// must be safe for concurrent use. RecordTurn should enqueue asynchronously
// and must not block the caller on I/O.
//
// The current implementation is FileTurnRecorder (phase 1); SQLite and
// memory-middleware implementations share this interface so they can be
// swapped in without touching callers.
type TurnRecorder interface {
	// RecordTurn enqueues a turn for persistence.
	//
	// sessionID is the owning session. Implementations choose the storage
	// layout under their own configured base directory.
	//
	// Returns nil on successful enqueue. Returns ErrClosed if the recorder
	// has been closed. Other errors indicate enqueue failure.
	RecordTurn(ctx context.Context, sessionID string, turn Turn) error

	// Flush blocks until all enqueued turns have been persisted.
	Flush(ctx context.Context) error

	// Close flushes pending turns and releases resources. Must be
	// idempotent. After Close, RecordTurn must return ErrClosed.
	Close() error
}
