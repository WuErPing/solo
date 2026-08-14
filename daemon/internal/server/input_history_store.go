package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// maxInputHistoryEntries bounds the persisted input history; when exceeded, the
// least-used entries (ties broken by oldest lastUsed) are evicted.
const maxInputHistoryEntries = 200

// InputHistoryEntry represents a deduplicated user text input with a usage counter.
type InputHistoryEntry struct {
	Text     string `json:"text"`
	Count    int    `json:"count"`
	LastUsed string `json:"lastUsed"`
}

// InputHistoryStore persists a deduplicated, frequency-counted list of user text
// inputs submitted to tmux panes.
type InputHistoryStore struct {
	mu       sync.RWMutex
	entries  []InputHistoryEntry
	dataPath string
}

func NewInputHistoryStore(dataDir string) *InputHistoryStore {
	s := &InputHistoryStore{
		dataPath: filepath.Join(dataDir, "tmux-input-history.json"),
	}
	s.load()
	return s
}

func (s *InputHistoryStore) load() {
	b, err := os.ReadFile(s.dataPath)
	if err != nil {
		return // file doesn't exist or unreadable — start empty
	}
	var entries []InputHistoryEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return
	}
	s.entries = entries
}

func (s *InputHistoryStore) save() error {
	dir := filepath.Dir(s.dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	b, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal input history: %w", err)
	}
	tmp := s.dataPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write input history file: %w", err)
	}
	if err := os.Rename(tmp, s.dataPath); err != nil {
		return fmt.Errorf("rename input history file: %w", err)
	}
	return nil
}

// Record increments the usage counter for the given text, adding a new entry if
// unseen. Text is stored verbatim; callers should trim before recording.
func (s *InputHistoryStore) Record(text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	for i, e := range s.entries {
		if e.Text == text {
			s.entries[i].Count++
			s.entries[i].LastUsed = now
			_ = s.save()
			return
		}
	}
	s.entries = append(s.entries, InputHistoryEntry{Text: text, Count: 1, LastUsed: now})
	if len(s.entries) > maxInputHistoryEntries {
		sort.SliceStable(s.entries, func(i, j int) bool {
			a, b := s.entries[i], s.entries[j]
			if a.Count != b.Count {
				return a.Count > b.Count
			}
			return a.LastUsed > b.LastUsed
		})
		s.entries = s.entries[:maxInputHistoryEntries]
	}
	_ = s.save()
}

// Entries returns a copy of the current entries sorted by count descending,
// ties broken by most recent lastUsed.
func (s *InputHistoryStore) Entries() []InputHistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InputHistoryEntry, len(s.entries))
	copy(out, s.entries)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].LastUsed > out[j].LastUsed
	})
	return out
}
