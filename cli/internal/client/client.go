package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

// DaemonClient is a WebSocket client that connects to a Solo daemon.
type DaemonClient struct {
	conn     *websocket.Conn
	clientID string
	wsURL    string
	logger   *slog.Logger

	mu        sync.Mutex
	pending   map[string]chan *protocol.WSOutboundMessage // requestId -> response channel
	nextReqID uint64

	subMu       sync.RWMutex
	subscribers map[string][]chan *protocol.WSOutboundMessage // inner msgType -> channels

	providersSnapshot *protocol.ProvidersSnapshotPayload
	serverInfo        *protocol.ServerInfoPayload

	done      chan struct{}
	closeOnce sync.Once
}

// NewDaemonClient connects to the daemon, performs the hello handshake, and starts the read pump.
func NewDaemonClient(ctx context.Context, host, clientID string) (*DaemonClient, error) {
	wsURL, err := ResolveHost(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	c := &DaemonClient{
		wsURL:       wsURL,
		clientID:    clientID,
		pending:     make(map[string]chan *protocol.WSOutboundMessage),
		subscribers: make(map[string][]chan *protocol.WSOutboundMessage),
		logger:      slog.Default().With("component", "client"),
		done:        make(chan struct{}),
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
		conn.Close()
		return nil, fmt.Errorf("send hello: %w", err)
	}

	// Read server_info and providers_snapshot_update with timeout
	conn.SetReadDeadline(time.Now().Add(protocol.HelloTimeoutMs * time.Millisecond))

	// Read server_info
	var serverInfoMsg protocol.WSOutboundMessage
	if err := conn.ReadJSON(&serverInfoMsg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read server_info: %w", err)
	}
	if serverInfoMsg.Type == "session" {
		payload, _ := json.Marshal(serverInfoMsg.Message)
		var status struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		}
		json.Unmarshal(payload, &status)
		if status.Status == "server_info" {
			var si protocol.ServerInfoPayload
			json.Unmarshal(payload, &si)
			c.serverInfo = &si
		}
	}

	// Read providers_snapshot_update
	var providersMsg protocol.WSOutboundMessage
	if err := conn.ReadJSON(&providersMsg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read providers_snapshot: %w", err)
	}
	if providersMsg.Type == "session" {
		payload, _ := json.Marshal(providersMsg.Message)
		var msg struct {
			Type string `json:"type"`
		}
		json.Unmarshal(payload, &msg)
		if msg.Type == "providers_snapshot_update" {
			var update protocol.ProvidersSnapshotUpdate
			json.Unmarshal(payload, &update)
			c.providersSnapshot = &update.Payload
		}
	}

	conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Start read pump
	go c.readPump()

	return c, nil
}

// Close sends a normal WebSocket close and cleans up.
func (c *DaemonClient) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.conn != nil {
			c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			c.conn.Close()
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
func (c *DaemonClient) Request(ctx context.Context, msg protocol.SessionInboundMessage) (*protocol.WSOutboundMessage, error) {
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

// Subscribe registers for a specific outbound message type and returns a channel.
func (c *DaemonClient) Subscribe(msgType string) <-chan *protocol.WSOutboundMessage {
	ch := make(chan *protocol.WSOutboundMessage, 16)
	c.subMu.Lock()
	c.subscribers[msgType] = append(c.subscribers[msgType], ch)
	c.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel.
func (c *DaemonClient) Unsubscribe(msgType string, ch <-chan *protocol.WSOutboundMessage) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	subs := c.subscribers[msgType]
	for i, s := range subs {
		if s == ch {
			c.subscribers[msgType] = append(subs[:i], subs[i+1:]...)
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
		json.Unmarshal(payload, &peek)

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

		// Fan out to subscribers
		c.subMu.RLock()
		subs := c.subscribers[innerType]
		c.subMu.RUnlock()
		for _, ch := range subs {
			select {
			case ch <- &msg:
			default:
			}
		}
	}
}

// setRequestID uses JSON round-trip to set the requestId field on any message struct.
func setRequestID(msg protocol.SessionInboundMessage, reqID string) {
	data, _ := json.Marshal(msg)
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	m["requestId"] = reqID
	data, _ = json.Marshal(m)
	json.Unmarshal(data, msg)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
