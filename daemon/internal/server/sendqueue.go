package server

import "sync"

// sendQueue is an unbounded, non-blocking FIFO queue for WebSocket send items.
// It replaces a fixed-capacity buffered channel (previously 256) to eliminate
// message drops when the writePump is slow (e.g. relay consumption lag).
//
// Producers call Push (never blocks beyond mutex contention).
// The consumer calls Pop (blocks until items are available or Close is called).
// After Close, Pop drains remaining items before returning (sendQueueItem{}, false).
type sendQueue struct {
	mu     sync.Mutex
	buf    []sendQueueItem
	head   int
	closed bool
	ready  chan struct{} // cap 1; non-blocking signal to wake Pop
	done   chan struct{} // closed by Close, wakes Pop for drain-or-exit
}

// newSendQueue creates an unbounded send queue.
func newSendQueue() *sendQueue {
	return &sendQueue{
		ready: make(chan struct{}, 1),
		done:  make(chan struct{}),
	}
}

// Push appends an item to the queue. Returns false if the queue is closed.
// Never blocks (only brief mutex contention).
func (q *sendQueue) Push(item sendQueueItem) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.buf = append(q.buf, item)
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
	return true
}

// Pop removes and returns the next item. Blocks until an item is available
// or the queue is closed. After Close, drains remaining items; returns
// (sendQueueItem{}, false) when the queue is empty and closed.
func (q *sendQueue) Pop() (sendQueueItem, bool) {
	select {
	case <-q.ready:
	case <-q.done:
		return q.popOne()
	}
	return q.popOne()
}

func (q *sendQueue) popOne() (sendQueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head >= len(q.buf) {
		return sendQueueItem{}, false
	}
	item := q.buf[q.head]
	q.buf[q.head] = sendQueueItem{} // help GC
	q.head++

	// Compact when more than half the buffer is consumed to prevent
	// unbounded backing-array growth in long-lived sessions.
	if q.head > 0 && q.head >= cap(q.buf)/2 {
		n := copy(q.buf, q.buf[q.head:])
		q.buf = q.buf[:n]
		q.head = 0
	}

	if q.head < len(q.buf) {
		select {
		case q.ready <- struct{}{}:
		default:
		}
	}
	return item, true
}

// Close marks the queue as closed and wakes blocked Pop callers.
// After Close, Push returns false. Pop drains remaining items, then returns false.
func (q *sendQueue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.mu.Unlock()
	close(q.done)
}

// Drain discards all queued items without sending them.
func (q *sendQueue) Drain() {
	q.mu.Lock()
	q.buf = nil
	q.head = 0
	q.mu.Unlock()
	// Wake any blocked Pop so it can discover the queue is empty.
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// IsClosed reports whether Close has been called.
func (q *sendQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// Len returns the number of items currently in the queue.
func (q *sendQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buf) - q.head
}
