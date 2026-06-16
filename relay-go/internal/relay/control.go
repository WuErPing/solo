// Package relay implements the Solo WebSocket relay server.
package relay

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type SyncMessage struct {
	Type          string   `json:"type"`
	ConnectionIDs []string `json:"connectionIds"`
}

type ConnectedMessage struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId"`
}

type DisconnectedMessage struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId"`
}

type PongMessage struct {
	Type string `json:"type"`
	Ts   int64  `json:"ts"`
}

func sendControlJSON(conn *websocket.Conn, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	// Best-effort control message; the peer will time out if it never arrives.
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func sendSync(conn *websocket.Conn, connectionIDs []string) {
	sendControlJSON(conn, SyncMessage{Type: "sync", ConnectionIDs: connectionIDs})
}

func notifyControl(sess *Session, v any) {
	if sess.ControlSocket == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	if err := sess.ControlSocket.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		// Connection is failing; close it best-effort and drop the reference.
		_ = sess.ControlSocket.Conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Control send failed"))
		_ = sess.ControlSocket.Conn.Close()
		sess.ControlSocket = nil
	}
}

func handleControlPing(conn *websocket.Conn) {
	sendControlJSON(conn, PongMessage{Type: "pong", Ts: time.Now().UnixMilli()})
}

var nudgeMu sync.Mutex

func startControlNudge(_ *SessionStore, sess *Session, connectionID string, syncDelay, resetDelay time.Duration, logger *slog.Logger) {
	nudgeMu.Lock()
	defer nudgeMu.Unlock()

	if existing, ok := sess.controlNudgeTimers[connectionID]; ok {
		existing.timer.Stop()
	}

	t := time.AfterFunc(syncDelay, func() {
		sess.mu.Lock()
		defer sess.mu.Unlock()

		cd := sess.Connections[connectionID]
		if cd == nil || len(cd.ClientSockets) == 0 {
			return
		}
		if cd.ServerDataSocket != nil {
			return
		}

		if sess.ControlSocket != nil {
			sendSync(sess.ControlSocket.Conn, sess.ListConnectedConnectionIDs())
		}

		t2 := time.AfterFunc(resetDelay, func() {
			sess.mu.Lock()
			defer sess.mu.Unlock()

			cd := sess.Connections[connectionID]
			if cd == nil || len(cd.ClientSockets) == 0 {
				return
			}
			if cd.ServerDataSocket != nil {
				return
			}

			if sess.ControlSocket != nil {
				logger.Warn("force-closing unresponsive control socket", "serverId", sess.ServerID, "connectionId", connectionID)
				_ = sess.ControlSocket.Conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Control unresponsive"))
				_ = sess.ControlSocket.Conn.Close()
				sess.ControlSocket = nil
			}
		})

		nudgeMu.Lock()
		sess.controlNudgeTimers[connectionID] = &nudgeTimer{timer: t2}
		nudgeMu.Unlock()
	})

	sess.controlNudgeTimers[connectionID] = &nudgeTimer{timer: t}
}

func cancelControlNudge(sess *Session, connectionID string) {
	nudgeMu.Lock()
	defer nudgeMu.Unlock()

	if t, ok := sess.controlNudgeTimers[connectionID]; ok {
		t.timer.Stop()
		delete(sess.controlNudgeTimers, connectionID)
	}
}

func startServerDataNudge(sess *Session, connectionID string, syncDelay, resetDelay time.Duration, logger *slog.Logger) {
	nudgeMu.Lock()
	defer nudgeMu.Unlock()

	if existing, ok := sess.serverDataNudgeTimers[connectionID]; ok {
		existing.timer.Stop()
	}

	t := time.AfterFunc(syncDelay, func() {
		sess.mu.Lock()
		defer sess.mu.Unlock()

		cd := sess.Connections[connectionID]
		if cd == nil || cd.ServerDataSocket == nil {
			return
		}
		if len(cd.ClientSockets) > 0 {
			return
		}

		if sess.ControlSocket != nil {
			sendSync(sess.ControlSocket.Conn, sess.ListConnectedConnectionIDs())
		}

		t2 := time.AfterFunc(resetDelay, func() {
			sess.mu.Lock()
			defer sess.mu.Unlock()

			cd := sess.Connections[connectionID]
			if cd == nil || cd.ServerDataSocket == nil {
				return
			}
			if len(cd.ClientSockets) > 0 {
				return
			}

			if sess.ControlSocket != nil {
				logger.Warn("force-closing unresponsive control socket (server-data waiting for client)", "serverId", sess.ServerID, "connectionId", connectionID)
				_ = sess.ControlSocket.Conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Control unresponsive"))
				_ = sess.ControlSocket.Conn.Close()
				sess.ControlSocket = nil
			}
		})

		nudgeMu.Lock()
		sess.serverDataNudgeTimers[connectionID] = &nudgeTimer{timer: t2}
		nudgeMu.Unlock()
	})

	sess.serverDataNudgeTimers[connectionID] = &nudgeTimer{timer: t}
}

func cancelServerDataNudge(sess *Session, connectionID string) {
	nudgeMu.Lock()
	defer nudgeMu.Unlock()

	if t, ok := sess.serverDataNudgeTimers[connectionID]; ok {
		t.timer.Stop()
		delete(sess.serverDataNudgeTimers, connectionID)
	}
}
