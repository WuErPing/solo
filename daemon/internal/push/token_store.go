package push

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TokenStore manages Expo push tokens for iOS devices.
type TokenStore interface {
	// Register adds a token to the store. Duplicate registrations are ignored.
	Register(token string)
	// Remove deletes a token from the store. No-op if token doesn't exist.
	Remove(token string)
	// GetAll returns all registered tokens.
	GetAll() []string
}

// InMemoryTokenStore is a thread-safe in-memory implementation of TokenStore.
type InMemoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]struct{}
}

// NewInMemoryTokenStore creates a new in-memory token store.
func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{
		tokens: make(map[string]struct{}),
	}
}

// Register adds a token to the store.
func (s *InMemoryTokenStore) Register(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = struct{}{}
}

// Remove deletes a token from the store.
func (s *InMemoryTokenStore) Remove(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

// GetAll returns all registered tokens.
func (s *InMemoryTokenStore) GetAll() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.tokens))
	for token := range s.tokens {
		result = append(result, token)
	}
	return result
}

// PersistedTokenStore is a thread-safe file-persisted implementation of TokenStore.
type PersistedTokenStore struct {
	mu       sync.RWMutex
	tokens   map[string]struct{}
	filePath string
	logger   *slog.Logger
}

// NewPersistedTokenStore creates a new persisted token store that reads from and writes to filePath.
func NewPersistedTokenStore(filePath string, logger *slog.Logger) *PersistedTokenStore {
	s := &PersistedTokenStore{
		tokens:   make(map[string]struct{}),
		filePath: filePath,
		logger:   logger,
	}
	s.loadFromDisk()
	return s
}

// Register adds a token to the store and persists to disk.
func (s *PersistedTokenStore) Register(token string) {
	normalized := strings.TrimSpace(token)
	if normalized == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tokens[normalized]; exists {
		return
	}
	s.tokens[normalized] = struct{}{}
	s.persist()
}

// Remove deletes a token from the store and persists to disk.
func (s *PersistedTokenStore) Remove(token string) {
	normalized := strings.TrimSpace(token)
	if normalized == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tokens[normalized]; !exists {
		return
	}
	delete(s.tokens, normalized)
	s.persist()
}

// GetAll returns all registered tokens.
func (s *PersistedTokenStore) GetAll() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.tokens))
	for token := range s.tokens {
		result = append(result, token)
	}
	return result
}

func (s *PersistedTokenStore) loadFromDisk() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.logger.Warn("failed to load push tokens", "error", err)
		}
		return
	}
	var raw struct {
		Tokens []string `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		s.logger.Warn("failed to parse push tokens", "error", err)
		return
	}
	for _, t := range raw.Tokens {
		normalized := strings.TrimSpace(t)
		if normalized != "" {
			s.tokens[normalized] = struct{}{}
		}
	}
	s.logger.Info("loaded push tokens", "total", len(s.tokens))
}

// persist writes the current tokens to disk atomically.
// Must be called with s.mu held for writing.
func (s *PersistedTokenStore) persist() {
	tokens := make([]string, 0, len(s.tokens))
	for t := range s.tokens {
		tokens = append(tokens, t)
	}
	payload, err := json.Marshal(struct {
		Tokens []string `json:"tokens"`
	}{Tokens: tokens})
	if err != nil {
		s.logger.Warn("failed to marshal push tokens", "error", err)
		return
	}
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.logger.Warn("failed to create push token directory", "error", err)
		return
	}
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0644); err != nil {
		s.logger.Warn("failed to write push tokens temp file", "error", err)
		return
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		s.logger.Warn("failed to rename push tokens file", "error", err)
	}
}
