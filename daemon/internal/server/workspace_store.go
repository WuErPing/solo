package server

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/WuErPing/solo/protocol"
)

// WorkspaceStore persists workspaces to a JSON file on disk.
type WorkspaceStore struct {
	mu       sync.RWMutex
	byID     map[string]*protocol.WorkspaceDescriptor
	filePath string
	logger   *slog.Logger
}

// NewWorkspaceStore creates a new WorkspaceStore.
func NewWorkspaceStore(soloHome string, logger *slog.Logger) *WorkspaceStore {
	return &WorkspaceStore{
		byID:     make(map[string]*protocol.WorkspaceDescriptor),
		filePath: filepath.Join(soloHome, "workspaces.json"),
		logger:   logger.With("component", "workspace-store"),
	}
}

// Initialize loads persisted workspaces from disk.
func (s *WorkspaceStore) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var workspaces []*protocol.WorkspaceDescriptor
	if err := json.Unmarshal(data, &workspaces); err != nil {
		return err
	}

	for _, ws := range workspaces {
		if ws != nil && ws.ID != "" {
			s.byID[ws.ID] = ws
		}
	}

	s.logger.Info("loaded workspaces", "count", len(s.byID))
	return nil
}

// GetAll returns all persisted workspaces.
func (s *WorkspaceStore) GetAll() []*protocol.WorkspaceDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*protocol.WorkspaceDescriptor, 0, len(s.byID))
	for _, ws := range s.byID {
		result = append(result, ws)
	}
	return result
}

// Get returns a workspace by ID.
func (s *WorkspaceStore) Get(id string) *protocol.WorkspaceDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id]
}

// Upsert saves or updates a workspace and persists to disk.
func (s *WorkspaceStore) Upsert(ws *protocol.WorkspaceDescriptor) error {
	s.mu.Lock()
	s.byID[ws.ID] = ws
	s.mu.Unlock()
	return s.save()
}

// Delete removes a workspace and persists to disk.
func (s *WorkspaceStore) Delete(id string) error {
	s.mu.Lock()
	delete(s.byID, id)
	s.mu.Unlock()
	return s.save()
}

func (s *WorkspaceStore) save() error {
	s.mu.RLock()
	workspaces := make([]*protocol.WorkspaceDescriptor, 0, len(s.byID))
	for _, ws := range s.byID {
		workspaces = append(workspaces, ws)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(workspaces, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}
