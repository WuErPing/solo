// Package client provides a WebSocket client for connecting to the Solo daemon.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

const (
	// DefaultRequestTimeout bounds Request calls whose context carries no deadline.
	DefaultRequestTimeout = 30 * time.Second

	// Keepalive: ping every 25s; if no pong (or any traffic) arrives within
	// 60s the connection is considered dead and the read pump exits.
	wsPingInterval = 25 * time.Second
	wsPongWait     = 60 * time.Second
)

// Subscription is a handle to a fan-out channel for one outbound message type.
type Subscription struct {
	msgType string
	ch      chan *protocol.WSOutboundMessage
	dropped atomic.Int64
}

// Messages returns the receive channel. It is closed when the connection ends.
func (s *Subscription) Messages() <-chan *protocol.WSOutboundMessage { return s.ch }

// DroppedCount reports how many messages were dropped for this subscription
// because the consumer could not keep up.
func (s *Subscription) DroppedCount() int64 { return s.dropped.Load() }

// Option configures a DaemonClient.
type Option func(*DaemonClient)

// WithRequestTimeout overrides the default RPC timeout applied to Request
// calls whose context carries no deadline.
func WithRequestTimeout(d time.Duration) Option {
	return func(c *DaemonClient) {
		if d > 0 {
			c.requestTimeout = d
		}
	}
}

// DaemonClient is a WebSocket client that connects to a Solo daemon.
type DaemonClient struct {
	conn     *websocket.Conn
	clientID string
	wsURL    string
	logger   *slog.Logger

	requestTimeout time.Duration

	mu        sync.Mutex
	pending   map[string]chan *protocol.WSOutboundMessage // requestId -> response channel
	nextReqID uint64

	subMu       sync.RWMutex
	subscribers map[string][]*Subscription // inner msgType -> subscriptions

	providersSnapshot *protocol.ProvidersSnapshotPayload
	serverInfo        *protocol.ServerInfoPayload

	done      chan struct{}
	closeOnce sync.Once
}

// NewDaemonClient connects to the daemon, performs the hello handshake, and starts the read pump.
func NewDaemonClient(ctx context.Context, host, clientID string, opts ...Option) (*DaemonClient, error) {
	wsURL, err := ResolveHost(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	c := &DaemonClient{
		wsURL:          wsURL,
		clientID:       clientID,
		requestTimeout: DefaultRequestTimeout,
		pending:        make(map[string]chan *protocol.WSOutboundMessage),
		subscribers:    make(map[string][]*Subscription),
		logger:         slog.Default().With("component", "client"),
		done:           make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}

	// Dial WebSocket
	dialer := DialerForHost(host)
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", wsURL, err)
	}
	c.conn = conn

	// Send hello
	hello := protocol.WSInboundMessage{
		Type:            "hello",
		ClientID:        clientID,
		ClientType:      protocol.ClientCLI,
		ProtocolVersion: protocol.WSProtocolVersion,
	}
	if err := conn.WriteJSON(hello); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send hello: %w", err)
	}

	// Read server_info and providers_snapshot_update with timeout
	_ = conn.SetReadDeadline(time.Now().Add(protocol.HelloTimeoutMs * time.Millisecond))

	// Read server_info
	var serverInfoMsg protocol.WSOutboundMessage
	if err := conn.ReadJSON(&serverInfoMsg); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read server_info: %w", err)
	}
	if serverInfoMsg.Type == "session" {
		payload, _ := json.Marshal(serverInfoMsg.Message)
		var status struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(payload, &status)
		if status.Status == "server_info" {
			var si protocol.ServerInfoPayload
			_ = json.Unmarshal(payload, &si)
			c.serverInfo = &si
		}
	}

	// Read providers_snapshot_update
	var providersMsg protocol.WSOutboundMessage
	if err := conn.ReadJSON(&providersMsg); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read providers_snapshot: %w", err)
	}
	if providersMsg.Type == "session" {
		payload, _ := json.Marshal(providersMsg.Message)
		var msg struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(payload, &msg)
		if msg.Type == "providers_snapshot_update" {
			var update protocol.ProvidersSnapshotUpdate
			_ = json.Unmarshal(payload, &update)
			c.providersSnapshot = &update.Payload
		}
	}

	_ = conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Start read pump
	go c.readPump()

	return c, nil
}

// Close sends a normal WebSocket close and cleans up.
func (c *DaemonClient) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.conn != nil {
			_ = c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = c.conn.Close()
		}
	})
	return nil
}

// GenerateRequestID returns a unique request ID.
func (c *DaemonClient) GenerateRequestID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextReqID++
	return fmt.Sprintf("cli-%d", c.nextReqID)
}

// Request sends a session inbound message and waits for the matching response.
// If ctx carries no deadline, DefaultRequestTimeout (or WithRequestTimeout)
// bounds the wait so an unresponsive daemon cannot hang the caller forever.
func (c *DaemonClient) Request(ctx context.Context, msg protocol.SessionInboundMessage) (*protocol.WSOutboundMessage, error) {
	if _, ok := ctx.Deadline(); !ok {
		timeout := c.requestTimeout
		if timeout <= 0 {
			timeout = DefaultRequestTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	reqID := c.GenerateRequestID()

	// Set the request ID on the message via reflection on the struct
	setRequestID(msg, reqID)

	ch := make(chan *protocol.WSOutboundMessage, 1)

	c.mu.Lock()
	c.pending[reqID] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, reqID)
		c.mu.Unlock()
	}()

	// Send the message
	wsMsg := protocol.WSInboundMessage{
		Type:    "session",
		Message: mustMarshal(msg),
	}
	if err := c.conn.WriteJSON(wsMsg); err != nil {
		return nil, fmt.Errorf("write message: %w", err)
	}

	// Wait for response or context cancellation
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("connection closed")
	}
}

// Subscribe registers for a specific outbound message type and returns a
// Subscription handle. The handle's Messages channel is closed when the
// connection ends, so consumers can observe disconnects.
func (c *DaemonClient) Subscribe(msgType string) *Subscription {
	sub := &Subscription{msgType: msgType, ch: make(chan *protocol.WSOutboundMessage, 16)}
	c.subMu.Lock()
	select {
	case <-c.done:
		// Connection already ended; hand back a closed channel.
		close(sub.ch)
	default:
		c.subscribers[msgType] = append(c.subscribers[msgType], sub)
	}
	c.subMu.Unlock()
	return sub
}

// Unsubscribe removes a subscription. It is safe to call after the
// subscription channel has been closed by a disconnect.
func (c *DaemonClient) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}
	c.subMu.Lock()
	defer c.subMu.Unlock()
	subs := c.subscribers[sub.msgType]
	for i, s := range subs {
		if s == sub {
			c.subscribers[sub.msgType] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// ProvidersSnapshot returns the cached providers snapshot received after hello.
func (c *DaemonClient) ProvidersSnapshot() *protocol.ProvidersSnapshotPayload {
	return c.providersSnapshot
}

// ServerInfo returns the cached server info received after hello.
func (c *DaemonClient) ServerInfo() *protocol.ServerInfoPayload {
	return c.serverInfo
}

// readPump reads messages from the WebSocket and routes them.
func (c *DaemonClient) readPump() {
	defer func() {
		// Signal that the connection is done
		c.closeOnce.Do(func() {
			close(c.done)
		})
		// Drain all pending requests
		c.mu.Lock()
		for id, ch := range c.pending {
			select {
			case ch <- nil:
			default:
			}
			delete(c.pending, id)
		}
		c.mu.Unlock()
		// Close all subscription channels so consumers observe the disconnect.
		c.subMu.Lock()
		for msgType, subs := range c.subscribers {
			for _, sub := range subs {
				close(sub.ch)
			}
			delete(c.subscribers, msgType)
		}
		c.subMu.Unlock()
	}()

	// Keepalive: bound the read so a silently dead daemon is detected, and
	// ping periodically so the daemon's own idle detection sees traffic.
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	pingTicker := time.NewTicker(wsPingInterval)
	defer pingTicker.Stop()
	go func() {
		for {
			select {
			case <-pingTicker.C:
				// WriteControl is safe to call concurrently with WriteJSON.
				if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			case <-c.done:
				return
			}
		}
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Debug("read error", "error", err)
			}
			return
		}

		var msg protocol.WSOutboundMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Debug("invalid JSON", "error", err)
			continue
		}

		if msg.Type == "pong" {
			continue
		}

		if msg.Type != "session" {
			continue
		}

		// Peek into the inner message to extract type and requestId
		payload, _ := json.Marshal(msg.Message)
		var peek struct {
			Type    string `json:"type"`
			Payload struct {
				RequestID string `json:"requestId"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(payload, &peek)

		innerType := peek.Type
		reqID := peek.Payload.RequestID

		// Try to route to pending request by requestId
		if reqID != "" {
			c.mu.Lock()
			if ch, ok := c.pending[reqID]; ok {
				select {
				case ch <- &msg:
				default:
				}
				c.mu.Unlock()
				continue
			}
			c.mu.Unlock()
		}

		// Fan out to subscribers, counting drops per subscription instead of
		// silently losing messages.
		c.subMu.RLock()
		subs := c.subscribers[innerType]
		c.subMu.RUnlock()
		for _, sub := range subs {
			select {
			case sub.ch <- &msg:
			default:
				sub.dropped.Add(1)
			}
		}
	}
}

// setRequestID uses JSON round-trip to set the requestId field on any message struct.
func setRequestID(msg protocol.SessionInboundMessage, reqID string) {
	data, _ := json.Marshal(msg)
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	m["requestId"] = reqID
	data, _ = json.Marshal(m)
	_ = json.Unmarshal(data, msg)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
