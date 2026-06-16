package base

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// CriticalEvent is implemented by events that must never be dropped.
// Implementations should return true for terminal/lifecycle events.
type CriticalEvent interface {
	IsCriticalEvent() bool
}

// SemiCriticalEvent is implemented by events that should survive transient
// backpressure (e.g., reasoning/thinking timeline events). Unlike critical
// events which block for up to 5s, semi-critical events use a short 100ms
// timeout — long enough to survive momentary channel fullness, short enough
// to not block the dispatcher under sustained load.
type SemiCriticalEvent interface {
	IsSemiCriticalEvent() bool
}

const criticalSendTimeout = 5 * time.Second
const semiCriticalSendTimeout = 100 * time.Millisecond

// EventDispatcher abstracts event distribution mechanisms.
type EventDispatcher interface {
	// Emit sends an event to all subscribers.
	Emit(evt interface{})
	// Close shuts down the dispatcher.
	Close()
}

// ChannelDispatcher uses a shared channel + subscriber map (Claude style).
type ChannelDispatcher struct {
	events      chan interface{}
	subscribers map[uint64]chan interface{}
	nextSubID   uint64
	mu          sync.RWMutex
	logger      *slog.Logger
	closed      bool

	// hasMainConsumer tracks whether Events() has been called.
	// If nobody reads from the main events channel, Emit skips it
	// to avoid blocking for criticalSendTimeout (5s) on every critical event.
	hasMainConsumer atomic.Bool
}

// NewChannelDispatcher creates a new channel-based dispatcher.
func NewChannelDispatcher(logger *slog.Logger) *ChannelDispatcher {
	return &ChannelDispatcher{
		events:      make(chan interface{}, 256),
		subscribers: make(map[uint64]chan interface{}),
		logger:      logger,
	}
}

// Subscribe returns a new channel that receives all events.
func (d *ChannelDispatcher) Subscribe() <-chan interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		ch := make(chan interface{})
		close(ch)
		return ch
	}

	id := d.nextSubID
	d.nextSubID++
	ch := make(chan interface{}, 2560)
	d.subscribers[id] = ch
	return ch
}

// Unsubscribe removes a subscriber.
func (d *ChannelDispatcher) Unsubscribe(ch <-chan interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for id, sub := range d.subscribers {
		if sub == ch {
			close(sub)
			delete(d.subscribers, id)
			return
		}
	}
}

// safeSendCh sends evt to ch with a timeout, recovering from sends to closed channels.
// Returns true if the event was sent, false if it timed out or the channel was closed.
func safeSendCh(ch chan interface{}, evt interface{}, timeout time.Duration) bool {
	defer func() { _ = recover() }() // handle send on closed channel
	select {
	case ch <- evt:
		return true
	case <-time.After(timeout):
		return false
	}
}

// safeTrySendCh attempts a non-blocking send, recovering from sends to closed channels.
// Returns true if the event was sent, false if the channel was full or closed.
func safeTrySendCh(ch chan interface{}, evt interface{}) bool {
	defer func() { _ = recover() }() // handle send on closed channel
	select {
	case ch <- evt:
		return true
	default:
		return false
	}
}

// Emit sends an event to all subscribers and the main channel.
func (d *ChannelDispatcher) Emit(evt interface{}) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return
	}

	// Check event priority level.
	_, isCritical := evt.(CriticalEvent)
	_, isSemiCritical := evt.(SemiCriticalEvent)

	// Copy subscriber list to release lock before potentially blocking sends.
	subs := make([]chan interface{}, 0, len(d.subscribers))
	for _, ch := range d.subscribers {
		subs = append(subs, ch)
	}
	d.mu.RUnlock()

	// Send to main channel only if someone is consuming it (outside lock).
	// OpenCode uses Subscribe() only; Claude uses Events(). Skipping the main
	// channel when unused avoids a 5-second criticalSendTimeout block on every
	// critical event (the 256-slot buffer fills up and nobody drains it).
	if d.hasMainConsumer.Load() {
		if isCritical {
			if !safeSendCh(d.events, evt, criticalSendTimeout) {
				d.logger.Error("event channel full, CRITICAL event timed out")
			}
		} else if isSemiCritical {
			if !safeSendCh(d.events, evt, semiCriticalSendTimeout) {
				d.logger.Warn("event channel full, semi-critical event timed out")
			}
		} else {
			if !safeTrySendCh(d.events, evt) {
				d.logger.Warn("event channel full, dropping event")
			}
		}
	}

	// Send to all subscribers (outside lock)
	// Use a shorter timeout for subscriber sends: if a subscriber can't
	// consume within 500ms, it's already too slow and blocking the
	// dispatcher is worse than dropping the event.
	const subscriberCriticalTimeout = 500 * time.Millisecond
	for _, ch := range subs {
		if isCritical {
			if !safeSendCh(ch, evt, subscriberCriticalTimeout) {
				d.logger.Error("subscriber channel full, CRITICAL event timed out")
			}
		} else if isSemiCritical {
			if !safeSendCh(ch, evt, semiCriticalSendTimeout) {
				d.logger.Warn("subscriber channel full, semi-critical event timed out")
			}
		} else {
			if !safeTrySendCh(ch, evt) {
				d.logger.Warn("subscriber channel full, dropping event")
			}
		}
	}
}

// Close shuts down the dispatcher.
func (d *ChannelDispatcher) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true

	for _, ch := range d.subscribers {
		close(ch)
	}
	d.subscribers = make(map[uint64]chan interface{})
	close(d.events)
}

// Events returns the main event channel.
func (d *ChannelDispatcher) Events() <-chan interface{} {
	d.hasMainConsumer.Store(true)
	return d.events
}

// CallbackDispatcher uses callback functions (OpenCode style).
type CallbackDispatcher struct {
	subscribers map[uint64]func(interface{})
	nextSubID   uint64
	mu          sync.RWMutex
	logger      *slog.Logger
	closed      bool
}

// NewCallbackDispatcher creates a new callback-based dispatcher.
func NewCallbackDispatcher(logger *slog.Logger) *CallbackDispatcher {
	return &CallbackDispatcher{
		subscribers: make(map[uint64]func(interface{})),
		logger:      logger,
	}
}

// Subscribe registers a callback and returns an unsubscribe function.
func (d *CallbackDispatcher) Subscribe(cb func(interface{})) func() {
	d.mu.Lock()
	id := d.nextSubID
	d.nextSubID++
	d.subscribers[id] = cb
	d.mu.Unlock()

	return func() {
		d.mu.Lock()
		delete(d.subscribers, id)
		d.mu.Unlock()
	}
}

// Emit sends an event to all registered callbacks.
func (d *CallbackDispatcher) Emit(evt interface{}) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return
	}

	// Copy subscribers to avoid holding lock during callbacks
	subs := make([]func(interface{}), 0, len(d.subscribers))
	for _, cb := range d.subscribers {
		subs = append(subs, cb)
	}
	d.mu.RUnlock()

	for _, cb := range subs {
		cb(evt)
	}
}

// Close shuts down the dispatcher.
func (d *CallbackDispatcher) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.subscribers = make(map[uint64]func(interface{}))
}

// --- Permission Manager ---

// PermissionManager tracks pending permission requests.
type PermissionManager struct {
	pending map[string]chan protocol.AgentPermissionResponse
	timers  map[string]*time.Timer
	mu      sync.Mutex
}

// NewPermissionManager creates a new permission manager.
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{
		pending: make(map[string]chan protocol.AgentPermissionResponse),
		timers:  make(map[string]*time.Timer),
	}
}

// Register registers a new pending permission request.
func (pm *PermissionManager) Register(requestID string) <-chan protocol.AgentPermissionResponse {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	ch := make(chan protocol.AgentPermissionResponse, 1)
	pm.pending[requestID] = ch
	return ch
}

// RegisterWithTimeout registers a pending permission request with an automatic
// timeout. If no response is received within the timeout, the request is
// auto-rejected with "deny" and the optional onTimeout callback is invoked.
func (pm *PermissionManager) RegisterWithTimeout(requestID string, timeout time.Duration, onTimeout func()) <-chan protocol.AgentPermissionResponse {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	ch := make(chan protocol.AgentPermissionResponse, 1)
	pm.pending[requestID] = ch

	pm.timers[requestID] = time.AfterFunc(timeout, func() {
		pm.mu.Lock()
		defer pm.mu.Unlock()

		// Only auto-reject if the request is still pending.
		if _, ok := pm.pending[requestID]; !ok {
			return
		}

		ch <- protocol.AgentPermissionResponse{Behavior: "deny"}
		delete(pm.pending, requestID)
		delete(pm.timers, requestID)

		if onTimeout != nil {
			onTimeout()
		}
	})

	return ch
}

// Respond responds to a pending permission request.
func (pm *PermissionManager) Respond(requestID string, response protocol.AgentPermissionResponse) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	ch, ok := pm.pending[requestID]
	if !ok {
		return fmt.Errorf("no pending permission for request %s", requestID)
	}

	// Stop the timeout timer if one exists.
	if t, ok := pm.timers[requestID]; ok {
		t.Stop()
		delete(pm.timers, requestID)
	}

	ch <- response
	delete(pm.pending, requestID)
	return nil
}

// GetPending returns all pending permission request IDs.
func (pm *PermissionManager) GetPending() []interface{} {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	result := make([]interface{}, 0, len(pm.pending))
	for reqID := range pm.pending {
		result = append(result, map[string]interface{}{
			"requestId": reqID,
		})
	}
	return result
}

// RejectAll rejects all pending permissions with "deny".
func (pm *PermissionManager) RejectAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for reqID, ch := range pm.pending {
		ch <- protocol.AgentPermissionResponse{Behavior: "deny"}
		delete(pm.pending, reqID)
	}
	for _, t := range pm.timers {
		t.Stop()
	}
	pm.timers = make(map[string]*time.Timer)
}

// Close closes all pending permission channels.
func (pm *PermissionManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, ch := range pm.pending {
		close(ch)
	}
	for _, t := range pm.timers {
		t.Stop()
	}
	pm.pending = make(map[string]chan protocol.AgentPermissionResponse)
	pm.timers = make(map[string]*time.Timer)
}
