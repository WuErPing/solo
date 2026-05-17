package relay

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ConnectionRole string

const (
	RoleServer ConnectionRole = "server"
	RoleClient ConnectionRole = "client"
)

type ControlSocket struct {
	Conn      *websocket.Conn
	CreatedAt time.Time
}

type DataSocket struct {
	Conn         *websocket.Conn
	ConnectionID string
	CreatedAt    time.Time
}

type ClientSocket struct {
	Conn         *websocket.Conn
	ConnectionID string
	CreatedAt    time.Time
}

type ConnectionData struct {
	ServerDataSocket *DataSocket
	Buffer           *FrameBuffer
	ClientSockets    []*ClientSocket
}

type nudgeTimer struct {
	timer *time.Timer
}

type Session struct {
	mu                  sync.Mutex
	ServerID            string
	ControlSocket       *ControlSocket
	Connections         map[string]*ConnectionData
	controlNudgeTimers  map[string]*nudgeTimer
	serverDataNudgeTimers map[string]*nudgeTimer
}

func NewSession(serverID string, maxBuffer int) *Session {
	return &Session{
		ServerID:              serverID,
		Connections:           make(map[string]*ConnectionData),
		controlNudgeTimers:    make(map[string]*nudgeTimer),
		serverDataNudgeTimers: make(map[string]*nudgeTimer),
	}
}

func (s *Session) GetOrCreateConnection(connectionID string, maxBuffer int) *ConnectionData {
	cd, ok := s.Connections[connectionID]
	if !ok {
		cd = &ConnectionData{
			Buffer:        NewFrameBuffer(maxBuffer),
			ClientSockets: make([]*ClientSocket, 0),
		}
		s.Connections[connectionID] = cd
	}
	return cd
}

func (s *Session) ListConnectedConnectionIDs() []string {
	ids := make([]string, 0)
	for id, cd := range s.Connections {
		if len(cd.ClientSockets) > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *Session) IsEmpty() bool {
	if s.ControlSocket != nil {
		return false
	}
	for _, cd := range s.Connections {
		if cd.ServerDataSocket != nil || len(cd.ClientSockets) > 0 {
			return false
		}
	}
	return true
}
