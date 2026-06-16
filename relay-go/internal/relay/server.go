package relay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/WuErPing/solo/protocol"
	relaymetrics "github.com/WuErPing/solo/relay/internal/metrics"
)

const version = "relay-go-v1"

type Server struct {
	Store           *SessionStore
	MaxBuffer       int
	Logger          *slog.Logger
	connCount       atomic.Int64
	NudgeSyncDelay  time.Duration
	NudgeResetDelay time.Duration
	AllowedOrigins  []string
}

func NewServer(store *SessionStore, maxBuffer int, logger *slog.Logger, allowedOrigins []string) *Server {
	return &Server{
		Store:           store,
		MaxBuffer:       maxBuffer,
		Logger:          logger,
		NudgeSyncDelay:  10 * time.Second,
		NudgeResetDelay: 5 * time.Second,
		AllowedOrigins:  allowedOrigins,
	}
}

func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if len(s.AllowedOrigins) == 0 {
		return false
	}
	for _, allowed := range s.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	s.Logger.Warn("rejected WebSocket connection: origin not allowed", "origin", origin)
	return false
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc(protocol.WSEndpoint, s.handleWebSocket)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"sessions":    s.Store.SessionCount(),
		"connections": s.connCount.Load(),
		"version":     version,
	}); err != nil {
		s.Logger.Warn("health response encode failed", "error", err)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	serverID := q.Get("serverId")
	role := ConnectionRole(q.Get("role"))
	connectionID := q.Get("connectionId")
	version := q.Get("v")

	if serverID == "" || (role != RoleServer && role != RoleClient) {
		http.Error(w, "invalid parameters: serverId and role are required", http.StatusBadRequest)
		return
	}

	if version != "" && version != protocol.RelayProtocolVersion {
		http.Error(w, "unsupported protocol version", http.StatusBadRequest)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: s.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Logger.Error("websocket upgrade failed", "error", err)
		return
	}

	s.connCount.Add(1)
	relaymetrics.Connections.Inc()

	sess := s.Store.GetOrCreate(serverID)
	sess.mu.Lock()
	resolvedConnectionID := s.handleV2Connect(sess, role, connectionID, conn)
	sess.mu.Unlock()

	s.readPump(sess, serverID, role, resolvedConnectionID, conn)
}

func (s *Server) handleV2Connect(sess *Session, role ConnectionRole, connectionID string, conn *websocket.Conn) string {
	resolvedID := connectionID
	if role == RoleClient && resolvedID == "" {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			s.Logger.Error("failed to read random bytes", "error", err)
			resolvedID = fmt.Sprintf("conn_%d", time.Now().UnixNano())
		} else {
			resolvedID = "conn_" + hex.EncodeToString(b)
		}
	}

	isControl := role == RoleServer && resolvedID == ""
	isData := role == RoleServer && resolvedID != ""

	switch {
	case isControl:
		if sess.ControlSocket != nil {
			_ = sess.ControlSocket.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Replaced by new connection"))
			_ = sess.ControlSocket.Conn.Close()
		}
		sess.ControlSocket = &ControlSocket{Conn: conn, CreatedAt: time.Now()}
		s.Logger.Info("socket connected", "event", "control_connected", "serverId", sess.ServerID, "role", "server")
		sendSync(conn, sess.ListConnectedConnectionIDs())

	case isData:
		cd := sess.GetOrCreateConnection(resolvedID, s.MaxBuffer)
		if cd.ServerDataSocket != nil {
			_ = cd.ServerDataSocket.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Replaced by new connection"))
			_ = cd.ServerDataSocket.Conn.Close()
		}
		cd.ServerDataSocket = &DataSocket{Conn: conn, ConnectionID: resolvedID, CreatedAt: time.Now()}
		s.Logger.Info("socket connected", "event", "data_connected", "serverId", sess.ServerID, "role", "server", "connectionId", resolvedID)
		cancelControlNudge(sess, resolvedID)

		if cd.Buffer.Len() > 0 {
			cd.Buffer.Flush(conn, s.Logger)
		}
		if len(cd.ClientSockets) > 0 {
			notifyControl(sess, ConnectedMessage{Type: "connected", ConnectionID: resolvedID})
		} else {
			// No client yet — start a nudge to remind the daemon to connect a client.
			startServerDataNudge(sess, resolvedID, s.NudgeSyncDelay, s.NudgeResetDelay, s.Logger)
		}

	case role == RoleClient:
		cd := sess.GetOrCreateConnection(resolvedID, s.MaxBuffer)
		cd.ClientSockets = append(cd.ClientSockets, &ClientSocket{
			Conn: conn, ConnectionID: resolvedID, CreatedAt: time.Now(),
		})
		s.Logger.Info("socket connected", "event", "client_connected", "serverId", sess.ServerID, "role", "client", "connectionId", resolvedID)
		notifyControl(sess, ConnectedMessage{Type: "connected", ConnectionID: resolvedID})
		cancelServerDataNudge(sess, resolvedID)

		if cd.ServerDataSocket != nil {
			if cd.Buffer.Len() > 0 {
				cd.Buffer.Flush(cd.ServerDataSocket.Conn, s.Logger)
			}
		} else {
			startControlNudge(s.Store, sess, resolvedID, s.NudgeSyncDelay, s.NudgeResetDelay, s.Logger)
		}
	}
	return resolvedID
}

func (s *Server) readPump(sess *Session, serverID string, role ConnectionRole, connectionID string, conn *websocket.Conn) {
	defer func() {
		s.handleClose(sess, serverID, role, connectionID, conn)
		_ = conn.Close()
		s.connCount.Add(-1)
		relaymetrics.Connections.Dec()
		s.Store.CleanupIfEmpty(serverID)
	}()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.Logger.Debug("read error", "error", err, "serverId", serverID, "role", role)
			}
			return
		}

		sess.mu.Lock()
		s.handleMessage(sess, role, connectionID, conn, msgType, msg)
		sess.mu.Unlock()
	}
}

func (s *Server) handleMessage(sess *Session, role ConnectionRole, connectionID string, conn *websocket.Conn, msgType int, msg []byte) {
	// Control channel keepalive: respond to JSON ping with pong.
	// Only text messages are parsed, matching Solo's typeof data === "string" check.
	if role == RoleServer && connectionID == "" && msgType == websocket.TextMessage {
		var parsed struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(msg, &parsed) == nil && parsed.Type == "ping" {
			handleControlPing(conn)
		}
		return
	}

	if connectionID == "" {
		return
	}

	cd := sess.Connections[connectionID]
	if cd == nil {
		return
	}

	switch role {
	case RoleClient:
		if cd.ServerDataSocket != nil {
			if err := cd.ServerDataSocket.Conn.WriteMessage(msgType, msg); err != nil {
				s.Logger.Warn("write to server failed", "connectionId", connectionID, "error", err)
			}
			relaymetrics.FramesForwarded.Inc()
		} else {
			cd.Buffer.Push(msgType, msg)
		}

	case RoleServer:
		remaining := cd.ClientSockets[:0]
		for _, client := range cd.ClientSockets {
			if err := client.Conn.WriteMessage(msgType, msg); err != nil {
				s.Logger.Warn("write to client failed, removing dead socket", "error", err)
				_ = client.Conn.Close()
			} else {
				relaymetrics.FramesForwarded.Inc()
				remaining = append(remaining, client)
			}
		}
		// Clear trailing pointers to avoid memory leak.
		for i := len(remaining); i < len(cd.ClientSockets); i++ {
			cd.ClientSockets[i] = nil
		}
		cd.ClientSockets = remaining
	}
}

func (s *Server) handleClose(sess *Session, serverID string, role ConnectionRole, connectionID string, conn *websocket.Conn) {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	s.Logger.Info("socket closed", "event", "socket_closed", "serverId", serverID, "role", string(role), "connectionId", connectionID)

	switch {
	case role == RoleServer && connectionID == "":
		sess.ControlSocket = nil
		for _, cd := range sess.Connections {
			if cd.ServerDataSocket != nil {
				_ = cd.ServerDataSocket.Conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "Control disconnected"))
				_ = cd.ServerDataSocket.Conn.Close()
				cd.ServerDataSocket = nil
			}
		}

	case role == RoleServer && connectionID != "":
		cd := sess.Connections[connectionID]
		if cd == nil {
			return
		}
		cd.ServerDataSocket = nil
		for _, client := range cd.ClientSockets {
			_ = client.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseServiceRestart, "Server disconnected"))
			_ = client.Conn.Close()
		}
		cd.ClientSockets = cd.ClientSockets[:0]
		cd.Buffer.Clear()
		notifyControl(sess, DisconnectedMessage{Type: "disconnected", ConnectionID: connectionID})

	case role == RoleClient:
		if connectionID == "" {
			return
		}
		cd := sess.Connections[connectionID]
		if cd == nil {
			return
		}
		remaining := make([]*ClientSocket, 0, len(cd.ClientSockets))
		for _, cs := range cd.ClientSockets {
			if cs.Conn != conn {
				remaining = append(remaining, cs)
			}
		}
		cd.ClientSockets = remaining

		if len(cd.ClientSockets) == 0 {
			cd.Buffer.Clear()
			cancelControlNudge(sess, connectionID)
			if cd.ServerDataSocket != nil {
				_ = cd.ServerDataSocket.Conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "Client disconnected"))
				_ = cd.ServerDataSocket.Conn.Close()
				cd.ServerDataSocket = nil
			}
			notifyControl(sess, DisconnectedMessage{Type: "disconnected", ConnectionID: connectionID})
		}
	}
}

func (s *Server) Addr(host, port string) string {
	return fmt.Sprintf("%s:%s", host, port)
}
