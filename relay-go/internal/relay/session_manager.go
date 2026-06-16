package relay

import (
	"log/slog"
	"sync"

	relaymetrics "github.com/WuErPing/solo/relay/internal/metrics"
)

type SessionStore struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	maxBuffer int
	logger    *slog.Logger
}

func NewSessionStore(maxBuffer int, logger *slog.Logger) *SessionStore {
	return &SessionStore{
		sessions:  make(map[string]*Session),
		maxBuffer: maxBuffer,
		logger:    logger,
	}
}

func (s *SessionStore) Get(serverID string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[serverID]
}

func (s *SessionStore) GetOrCreate(serverID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[serverID]
	if !ok {
		sess = NewSession(serverID, s.maxBuffer)
		s.sessions[serverID] = sess
		relaymetrics.Sessions.Inc()
		s.logger.Info("session created", "event", "session_created", "serverId", serverID)
	}
	return sess
}

func (s *SessionStore) SessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *SessionStore) CleanupIfEmpty(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[serverID]
	if !ok {
		return
	}
	sess.mu.Lock()
	empty := sess.IsEmpty()
	sess.mu.Unlock()

	if empty {
		delete(s.sessions, serverID)
		relaymetrics.Sessions.Dec()
		s.logger.Info("session cleaned up", "event", "session_cleaned_up", "serverId", serverID)
	}
}
