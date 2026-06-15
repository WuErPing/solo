package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentCommandEntry represents a deduplicated coding agent launch command.
type AgentCommandEntry struct {
	AgentName string `json:"agentName"`
	LaunchCmd string `json:"launchCmd"`
	LastSeen  string `json:"lastSeen"`
}

// AgentCommandStore persists a deduplicated list of agent launch commands to disk.
type AgentCommandStore struct {
	mu       sync.RWMutex
	entries  []AgentCommandEntry
	dataPath string
}

func NewAgentCommandStore(dataDir string) *AgentCommandStore {
	s := &AgentCommandStore{
		dataPath: filepath.Join(dataDir, "agent-commands.json"),
	}
	s.load()
	return s
}

func (s *AgentCommandStore) load() {
	b, err := os.ReadFile(s.dataPath)
	if err != nil {
		return // file doesn't exist or unreadable — start empty
	}
	var entries []AgentCommandEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return
	}
	// Drop stale entries whose LaunchCmd was captured before the setproctitle-
	// aware fallback was added (e.g. agentName="kimi" with launchCmd=
	// "kimi-code", which isn't a real binary on PATH). To preserve legitimate
	// wrappers (e.g. "node kimi"), only drop when the binary doesn't exist.
	clean := entries[:0]
	for _, e := range entries {
		if isStaleLaunchCmd(e.LaunchCmd, e.AgentName) {
			continue
		}
		clean = append(clean, e)
	}
	s.entries = clean
}

func (s *AgentCommandStore) save() error {
	dir := filepath.Dir(s.dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	b, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent commands: %w", err)
	}
	tmp := s.dataPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write agent commands file: %w", err)
	}
	if err := os.Rename(tmp, s.dataPath); err != nil {
		return fmt.Errorf("rename agent commands file: %w", err)
	}
	return nil
}

// Merge adds new entries, deduplicating by LaunchCmd. Updates LastSeen for existing entries.
func (s *AgentCommandStore) Merge(newEntries []AgentCommandEntry) {
	if len(newEntries) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build index of existing entries by LaunchCmd.
	index := make(map[string]int, len(s.entries))
	for i, e := range s.entries {
		index[e.LaunchCmd] = i
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, ne := range newEntries {
		if ne.LaunchCmd == "" {
			continue
		}
		if idx, ok := index[ne.LaunchCmd]; ok {
			s.entries[idx].LastSeen = now
		} else {
			ne.LastSeen = now
			s.entries = append(s.entries, ne)
			index[ne.LaunchCmd] = len(s.entries) - 1
		}
	}
	_ = s.save()
}

// Entries returns a copy of the current entries.
func (s *AgentCommandStore) Entries() []AgentCommandEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AgentCommandEntry, len(s.entries))
	copy(out, s.entries)
	return out
}
