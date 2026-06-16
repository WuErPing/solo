// Package relayclient maintains the WebSocket connection to the Solo relay.
package relayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/daemon/internal/wsconn"
	"github.com/WuErPing/solo/protocol"
)

const (
	controlPingInterval = 10 * time.Second
	controlStaleTimeout = 30 * time.Second
	maxReconnectDelay   = 30 * time.Second
)

// dataSocketOpenTimeout is the maximum time allowed from a successful dial to
// AttachExternalConnection completing the session handshake. If the WSServer
// never processes the hello (e.g. stuck hello timeout), this guard terminates
// the data socket so the goroutine does not leak.
// Must be comfortably larger than the hello timeout (15 s) plus the time it
// can take to initialise a session when an agent is actively running (e.g.
// pushing agent state, replaying grace buffer). 60 s matches the relay grace
// period and prevents premature disconnection during long thinking phases.
// Exposed as a var so tests can reduce it.
var dataSocketOpenTimeout = 60 * time.Second

// SessionAttacher is the interface relayclient uses to attach relay data
// sockets into the daemon's WebSocket server.
type SessionAttacher interface {
	AttachExternalConnection(conn wsconn.WSConn)
}

// HelloProcessedNotifier is an optional interface that a SessionAttacher can
// implement to signal that the hello handshake completed successfully. This
// allows the relay client to cancel the openTimer guard, preventing it from
// killing healthy, long-running sessions.
//
// Mirroring Solo: clearTimeout(openTimeout) fires on socket "open".
type HelloProcessedNotifier interface {
	OnHelloProcessed(fn func())
}

// Client connects a Solo daemon to a relay server.
type Client struct {
	serverID string
	endpoint string
	logger   *slog.Logger
	wsServer SessionAttacher

	controlConn   *websocket.Conn
	controlMu     sync.Mutex
	controlCancel context.CancelFunc

	dataConns   map[string]wsconn.WSConn
	dataConnsMu sync.Mutex

	// pendingConns tracks connectionIds for which a data socket open is
	// in flight, preventing duplicate openDataSocket goroutines.
	pendingConns   map[string]struct{}
	pendingConnsMu sync.Mutex

	keyPair *DaemonKeyPair

	reconnectAttempt int
	reconnectTimer   *time.Timer
	reconnectMu      sync.Mutex
	stopped          atomic.Bool

	lastActivityMs atomic.Int64

	disableControlKeepalive bool
}

// NewClient creates a relay client.
func NewClient(serverID, endpoint string, wsServer SessionAttacher, logger *slog.Logger, keyPair *DaemonKeyPair, disableControlKeepalive bool) *Client {
	return &Client{
		serverID:                serverID,
		endpoint:                endpoint,
		logger:                  logger,
		wsServer:                wsServer,
		dataConns:               make(map[string]wsconn.WSConn),
		pendingConns:            make(map[string]struct{}),
		keyPair:                 keyPair,
		disableControlKeepalive: disableControlKeepalive,
	}
}

// Start begins connecting to the relay.
func (c *Client) Start() error {
	if c.stopped.Load() {
		return fmt.Errorf("relay client already stopped")
	}
	c.connectControl()
	return nil
}

// Stop shuts down the relay client and prevents reconnection.
func (c *Client) Stop() {
	c.stopped.Store(true)

	c.reconnectMu.Lock()
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	c.reconnectMu.Unlock()

	c.closeAllDataConns()

	c.controlMu.Lock()
	if c.controlConn != nil {
		_ = c.controlConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "daemon stopping"))
		_ = c.controlConn.Close()
		c.controlConn = nil
	}
	if c.controlCancel != nil {
		c.controlCancel()
		c.controlCancel = nil
	}
	c.controlMu.Unlock()
}

func (c *Client) connectControl() {
	if c.stopped.Load() {
		return
	}

	u := c.buildControlURL()
	c.logger.Info("connecting to relay control socket", "url", u)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.Dial(u, nil)
	if err != nil {
		if resp != nil {
			c.logger.Warn("relay control connect failed", "status", resp.Status, "error", err)
		} else {
			c.logger.Warn("relay control connect failed", "error", err)
		}
		c.scheduleReconnect()
		return
	}

	c.controlMu.Lock()
	c.controlConn = conn
	ctx, cancel := context.WithCancel(context.Background())
	c.controlCancel = cancel
	c.controlMu.Unlock()

	c.reconnectMu.Lock()
	c.reconnectAttempt = 0
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	c.reconnectMu.Unlock()

	c.lastActivityMs.Store(time.Now().UnixMilli())
	c.logger.Info("relay control socket connected")

	go c.controlReadPump(ctx, conn)
	if !c.disableControlKeepalive {
		go c.controlKeepalive(ctx, conn)
	}
}

func (c *Client) controlReadPump(ctx context.Context, conn *websocket.Conn) {
	defer func() {
		c.logger.Info("relay control socket closed")
		c.controlMu.Lock()
		if c.controlConn == conn {
			c.controlConn = nil
		}
		c.controlMu.Unlock()
		_ = conn.Close()
		c.scheduleReconnect()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Debug("relay control read error", "error", err)
			}
			return
		}

		c.lastActivityMs.Store(time.Now().UnixMilli())

		if msgType != websocket.TextMessage {
			continue
		}

		c.handleControlMessage(data)
	}
}

func (c *Client) handleControlMessage(data []byte) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		c.logger.Debug("invalid control message", "error", err)
		return
	}

	switch base.Type {
	case "sync":
		var msg struct {
			ConnectionIDs []string `json:"connectionIds"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}
		c.logger.Info("relay sync", "connectionIds", msg.ConnectionIDs)
	case "connected":
		var msg struct {
			ConnectionID string `json:"connectionId"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}
		c.logger.Info("relay client connected", "connectionId", msg.ConnectionID)

		// Avoid opening duplicate data sockets: check both existing
		// and pending (in-flight) data socket opens.
		c.pendingConnsMu.Lock()
		_, pending := c.pendingConns[msg.ConnectionID]
		if pending {
			c.pendingConnsMu.Unlock()
			c.logger.Debug("data socket open already pending, skipping", "connectionId", msg.ConnectionID)
			return
		}
		c.pendingConns[msg.ConnectionID] = struct{}{}
		c.pendingConnsMu.Unlock()

		go c.openDataSocket(msg.ConnectionID)
	case "disconnected":
		var msg struct {
			ConnectionID string `json:"connectionId"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}
		c.logger.Info("relay client disconnected", "connectionId", msg.ConnectionID)
		c.closeDataConn(msg.ConnectionID)
	case "ping":
		c.sendPong()
	case "pong":
		// activity already recorded by read pump
	default:
		c.logger.Debug("unknown control message", "type", base.Type)
	}
}

func (c *Client) sendPong() {
	c.controlMu.Lock()
	conn := c.controlConn
	c.controlMu.Unlock()
	if conn == nil {
		return
	}
	pong := struct {
		Type string `json:"type"`
		Ts   int64  `json:"ts"`
	}{
		Type: "pong",
		Ts:   time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(pong)
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) controlKeepalive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(controlPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastMs := c.lastActivityMs.Load()
			if time.Now().UnixMilli()-lastMs > int64(controlStaleTimeout.Milliseconds()) {
				c.logger.Warn("relay control socket stale, forcing reconnect")
				_ = conn.Close()
				return
			}

			c.controlMu.Lock()
			if c.controlConn == conn {
				ping := struct {
					Type string `json:"type"`
					Ts   int64  `json:"ts"`
				}{
					Type: "ping",
					Ts:   time.Now().UnixMilli(),
				}
				data, _ := json.Marshal(ping)
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					c.logger.Debug("relay ping write error", "error", err)
				}
			}
			c.controlMu.Unlock()
		}
	}
}

func (c *Client) openDataSocket(connectionID string) {
	// Always clear the pending marker when done.
	defer func() {
		c.pendingConnsMu.Lock()
		delete(c.pendingConns, connectionID)
		c.pendingConnsMu.Unlock()
	}()

	c.openDataSocketURL(connectionID, c.buildDataURL(connectionID))
}

// openDataSocketURL dials connectionID at the given WebSocket URL, performs
// the optional E2EE handshake, and feeds the connection into the WSServer.
// A dataSocketOpenTimeout guard terminates the connection if
// AttachExternalConnection does not return within the deadline — protecting
// against hung hello handshakes that would otherwise leak the goroutine.
func (c *Client) openDataSocketURL(connectionID, u string) {
	logger := c.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("opening relay data socket", "connectionId", connectionID, "url", u)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	rawConn, resp, err := dialer.Dial(u, nil)
	if err != nil {
		if resp != nil {
			logger.Warn("relay data connect failed", "connectionId", connectionID, "status", resp.Status, "error", err)
		} else {
			logger.Warn("relay data connect failed", "connectionId", connectionID, "error", err)
		}
		return
	}

	// Register the raw conn in dataConns BEFORE the E2EE handshake so that
	// closeDataConn (called when the relay sends a "disconnected" control
	// message) can find and close it even during the handshake window.
	// Without this, a "disconnected" arriving before line 374 would leave
	// the socket unclosable, causing AttachSocket's readLoopFor to block
	// forever and the session to be permanently stuck in "attaching".
	c.dataConnsMu.Lock()
	c.dataConns[connectionID] = rawConn
	c.dataConnsMu.Unlock()

	// Perform E2EE handshake if daemon keypair is available.
	var conn wsconn.WSConn = rawConn
	if c.keyPair != nil {
		e2eeConn, err := PerformE2EEHandshake(rawConn, c.keyPair.SecretKeyB64, logger)
		if err != nil {
			logger.Warn("e2ee handshake failed, closing data socket", "connectionId", connectionID, "error", err)
			c.dataConnsMu.Lock()
			delete(c.dataConns, connectionID)
			c.dataConnsMu.Unlock()
			_ = rawConn.Close()
			return
		}
		conn = e2eeConn
		logger.Info("e2ee handshake succeeded on data socket", "connectionId", connectionID)
		// Update to e2ee-wrapped conn so closeDataConn closes the right object.
		c.dataConnsMu.Lock()
		c.dataConns[connectionID] = conn
		c.dataConnsMu.Unlock()
	}

	logger.Info("relay data socket connected", "connectionId", connectionID, "e2ee", c.keyPair != nil)

	// Mirror Solo's approach: the openTimeout is cleared as soon as the hello
	// handshake completes (socket "open"), NOT when the session ends. This
	// prevents killing healthy, long-running relay sessions.
	//
	// The default dataSocketOpenTimeout (60s) now only applies when the hello
	// handshake itself hangs. Once the hello is processed, the session manages
	// its own lifecycle via the read-loop.
	//
	// If the SessionAttacher implements HelloProcessedNotifier, the callback
	// cancels the timer as soon as the hello handshake completes (inside
	// handleNewConnection, right after server_info is sent).
	openTimer := time.AfterFunc(dataSocketOpenTimeout, func() {
		logger.Warn("relay data socket open timeout, terminating", "connectionId", connectionID)
		_ = conn.Close()
	})

	if notifier, ok := c.wsServer.(HelloProcessedNotifier); ok {
		notifier.OnHelloProcessed(func() {
			if !openTimer.Stop() {
				// Timer already fired — connection is being closed.
				return
			}
			logger.Info("relay data socket open timer cancelled after hello", "connectionId", connectionID)
		})
	}

	// Feed into WSServer — this blocks until the session ends.
	c.wsServer.AttachExternalConnection(conn)

	openTimer.Stop()

	// Session ended, clean up.
	c.dataConnsMu.Lock()
	delete(c.dataConns, connectionID)
	c.dataConnsMu.Unlock()
	_ = conn.Close()
	logger.Info("relay data socket closed", "connectionId", connectionID)
}

func (c *Client) closeDataConn(connectionID string) {
	c.dataConnsMu.Lock()
	conn, ok := c.dataConns[connectionID]
	if ok {
		delete(c.dataConns, connectionID)
	}
	c.dataConnsMu.Unlock()
	if conn != nil {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client disconnected"))
		_ = conn.Close()
	}
}

func (c *Client) closeAllDataConns() {
	c.dataConnsMu.Lock()
	conns := make([]wsconn.WSConn, 0, len(c.dataConns))
	for _, conn := range c.dataConns {
		conns = append(conns, conn)
	}
	c.dataConns = make(map[string]wsconn.WSConn)
	c.dataConnsMu.Unlock()

	for _, conn := range conns {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "daemon stopping"))
		_ = conn.Close()
	}
}

func (c *Client) scheduleReconnect() {
	if c.stopped.Load() {
		return
	}

	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	if c.reconnectTimer != nil {
		return // already scheduled
	}

	c.reconnectAttempt++
	delay := time.Duration(c.reconnectAttempt) * time.Second
	if delay > maxReconnectDelay {
		delay = maxReconnectDelay
	}

	c.logger.Info("scheduling relay reconnect", "attempt", c.reconnectAttempt, "delay", delay)
	c.reconnectTimer = time.AfterFunc(delay, func() {
		c.reconnectMu.Lock()
		c.reconnectTimer = nil
		c.reconnectMu.Unlock()
		c.connectControl()
	})
}

func (c *Client) buildControlURL() string {
	scheme := c.wsScheme()
	return fmt.Sprintf("%s://%s%s?serverId=%s&role=server&v=%s",
		scheme, c.endpoint, protocol.WSEndpoint, url.QueryEscape(c.serverID), protocol.RelayProtocolVersion)
}

func (c *Client) buildDataURL(connectionID string) string {
	scheme := c.wsScheme()
	return fmt.Sprintf("%s://%s%s?serverId=%s&role=server&v=%s&connectionId=%s",
		scheme, c.endpoint, protocol.WSEndpoint, url.QueryEscape(c.serverID),
		protocol.RelayProtocolVersion, url.QueryEscape(connectionID))
}

func (c *Client) wsScheme() string {
	host, port, err := net.SplitHostPort(c.endpoint)
	if err != nil {
		return "ws"
	}
	_ = host
	if port == "443" || strings.HasSuffix(c.endpoint, ":443") {
		return "wss"
	}
	return "ws"
}
