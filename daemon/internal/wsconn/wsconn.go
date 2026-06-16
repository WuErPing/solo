// Package wsconn defines the WebSocket connection interface used by the server.
package wsconn

import "time"

// WSConn is the minimal WebSocket connection interface used by Session
// and relayclient. Both *websocket.Conn and E2EE wrappers implement
// this interface.
type WSConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
}

// PingableConn extends WSConn with methods needed for server-initiated
// ping/pong keepalive. Both *websocket.Conn and E2EE wrappers implement this.
type PingableConn interface {
	WSConn
	WriteControl(messageType int, data []byte, deadline time.Time) error
	SetPongHandler(h func(appData string) error)
	SetReadDeadline(t time.Time) error
}

// WriteDeadlineConn is implemented by WebSocket connections that can bound
// blocking writes. *websocket.Conn implements this, as do relay wrappers that
// delegate to a gorilla WebSocket.
type WriteDeadlineConn interface {
	SetWriteDeadline(t time.Time) error
}
