// Package terminal runs local pseudo-terminal processes.
package terminal

import (
	"sync"
	"time"
)

const defaultFlushDelay = 5 * time.Millisecond

type OutputCoalescer struct {
	mu      sync.Mutex
	pending []byte
	timer   *time.Timer
	onFlush func(payload []byte)
	stopped bool
}

func NewOutputCoalescer(onFlush func(payload []byte)) *OutputCoalescer {
	return &OutputCoalescer{
		onFlush: onFlush,
	}
}

func (c *OutputCoalescer) Add(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.pending = append(c.pending, data...)
	if c.timer == nil {
		c.timer = time.AfterFunc(defaultFlushDelay, c.flush)
	}
}

func (c *OutputCoalescer) Flush() {
	c.flush()
}

func (c *OutputCoalescer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	if c.timer != nil {
		c.timer.Stop()
	}
	if len(c.pending) > 0 {
		data := c.pending
		c.pending = nil
		c.onFlush(data)
	}
}

func (c *OutputCoalescer) flush() {
	c.mu.Lock()
	data := c.pending
	c.pending = nil
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()

	if len(data) > 0 {
		c.onFlush(data)
	}
}
