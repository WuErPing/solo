// Package filebackend implements memory.TurnRecorder with a local filesystem
// backend. See docs/product/session-memory-spec.md (M2).
package filebackend

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Overflow controls the behavior of RecordTurn when the internal queue
// is full.
type Overflow string

const (
	// OverflowBlock makes RecordTurn block until space is available.
	OverflowBlock Overflow = "block"
	// OverflowError makes RecordTurn return an error immediately.
	OverflowError Overflow = "error"
)

// Default values applied when Config fields are zero.
const (
	DefaultQueueSize     = 1024
	DefaultFlushInterval = 500 * time.Millisecond
	DefaultRoot          = "memory"
	DefaultOverflow      = OverflowBlock
)

// Config parameterizes a FileTurnRecorder.
type Config struct {
	// QueueSize is the capacity of the internal turn channel.
	QueueSize int
	// Overflow selects the behavior when QueueSize is reached.
	Overflow Overflow
	// FlushInterval is a hint for background flush cadence (M5 metrics
	// hook uses it; M2 itself flushes only on demand).
	FlushInterval time.Duration
	// Root is the directory name (relative to BaseDir) under which
	// sessions live. Defaults to "memory".
	Root string
	// BaseDir is the root directory on disk. Required. In production this
	// is SoloHome (~/.solo), so sessions land at ~/.solo/memory/sessions/...
	// For tests, typically a t.TempDir().
	BaseDir string
	// Logger receives error logs from the background writer. If nil,
	// slog.Default() is used.
	Logger *slog.Logger
}

// ApplyDefaults fills zero-valued fields with defaults and validates.
func (c *Config) ApplyDefaults() error {
	if c.BaseDir == "" {
		return errors.New("file: Config.BaseDir is required")
	}
	if c.QueueSize <= 0 {
		c.QueueSize = DefaultQueueSize
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = DefaultFlushInterval
	}
	if c.Root == "" {
		c.Root = DefaultRoot
	}
	if c.Overflow == "" {
		c.Overflow = DefaultOverflow
	}
	switch c.Overflow {
	case OverflowBlock, OverflowError:
		// ok
	default:
		return fmt.Errorf("file: unknown overflow policy %q", c.Overflow)
	}
	return nil
}
