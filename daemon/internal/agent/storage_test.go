package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func newTestStorage(t *testing.T) *AgentStorage {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := NewAgentStorage(dir, logger)
	if err := s.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return s
}

func TestAgentStorageInitialize(t *testing.T) {
	s := newTestStorage(t)
	if len(s.List()) != 0 {
		t.Error("expected empty storage")
	}
}

func TestAgentStorageUpsertAndGet(t *testing.T) {
	s := newTestStorage(t)

	record := &StoredAgentRecord{
		ID:         "test-agent-1",
		Provider:   "claude",
		Cwd:        "/tmp/project",
		LastStatus: "idle",
		Labels:     map[string]string{},
	}

	if err := s.Upsert(record); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got := s.Get("test-agent-1")
	if got == nil {
		t.Fatal("expected to find agent")
	}
	if got.Provider != "claude" {
		t.Errorf("Provider: got %q, want %q", got.Provider, "claude")
	}
}

func TestAgentStorageUpsertUpdatesExisting(t *testing.T) {
	s := newTestStorage(t)

	record := &StoredAgentRecord{
		ID:         "test-agent-2",
		Provider:   "claude",
		Cwd:        "/tmp/project",
		LastStatus: "idle",
		Labels:     map[string]string{},
	}
	s.Upsert(record)

	// Update status
	record.LastStatus = "running"
	s.Upsert(record)

	got := s.Get("test-agent-2")
	if got.LastStatus != "running" {
		t.Errorf("LastStatus: got %q, want %q", got.LastStatus, "running")
	}
}

func TestAgentStorageBeginDelete(t *testing.T) {
	s := newTestStorage(t)

	record := &StoredAgentRecord{
		ID:         "test-agent-3",
		Provider:   "claude",
		Cwd:        "/tmp/project",
		LastStatus: "idle",
		Labels:     map[string]string{},
	}
	s.Upsert(record)

	s.BeginDelete("test-agent-3")

	// Upsert should be silently skipped after BeginDelete
	record.LastStatus = "running"
	if err := s.Upsert(record); err != nil {
		t.Fatalf("Upsert after BeginDelete should not error: %v", err)
	}

	// Remove the agent
	s.Remove("test-agent-3")

	if s.Get("test-agent-3") != nil {
		t.Error("expected agent to be removed")
	}
}

func TestAgentStorageList(t *testing.T) {
	s := newTestStorage(t)

	for i := 0; i < 3; i++ {
		s.Upsert(&StoredAgentRecord{
			ID:         fmt.Sprintf("agent-%d", i),
			Provider:   "claude",
			Cwd:        "/tmp/project",
			LastStatus: "idle",
			Labels:     map[string]string{},
		})
	}

	list := s.List()
	if len(list) != 3 {
		t.Errorf("List: got %d, want 3", len(list))
	}
}

func TestAgentStorageDiskRoundTrip(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create and write
	s1 := NewAgentStorage(dir, logger)
	s1.Initialize()
	s1.Upsert(&StoredAgentRecord{
		ID:         "disk-test",
		Provider:   "codex",
		Cwd:        "/home/user/repo",
		LastStatus: "idle",
		Labels:     map[string]string{"env": "test"},
	})

	// Read from fresh instance
	s2 := NewAgentStorage(dir, logger)
	s2.Initialize()

	got := s2.Get("disk-test")
	if got == nil {
		t.Fatal("expected to find agent from disk")
	}
	if got.Provider != "codex" {
		t.Errorf("Provider: got %q, want codex", got.Provider)
	}
	if got.Labels["env"] != "test" {
		t.Errorf("Labels[env]: got %q, want test", got.Labels["env"])
	}
}

func TestProjectDirNameFromCwd(t *testing.T) {
	tests := []struct {
		cwd, want string
	}{
		{"/home/user/project", "home-user-project"},
		{"/tmp", "tmp"},
		{"", "root"},
	}
	for _, tt := range tests {
		got := projectDirNameFromCwd(tt.cwd)
		if got != tt.want {
			t.Errorf("projectDirNameFromCwd(%q) = %q, want %q", tt.cwd, got, tt.want)
		}
	}
}

func TestManagedAgentSnapshotSerializesRequiredCollections(t *testing.T) {
	agent := NewManagedAgent("agent-collections", "mock", "/tmp/project", nil, nil)
	snapshot := agent.ToSnapshot()

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := raw["availableModes"].([]any); !ok {
		t.Fatalf("availableModes: got %#v, want JSON array", raw["availableModes"])
	}
	if _, ok := raw["labels"].(map[string]any); !ok {
		t.Fatalf("labels: got %#v, want JSON object", raw["labels"])
	}
	if _, ok := raw["pendingPermissions"].([]any); !ok {
		t.Fatalf("pendingPermissions: got %#v, want JSON array", raw["pendingPermissions"])
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.json")
	data := []byte(`{"hello":"world"}`)

	if err := writeFileAtomic(target, data); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}

	// Verify no temp files left behind
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "test.json" {
			t.Errorf("unexpected file: %s", e.Name())
		}
	}
}

// Need fmt import
var _ = fmt.Sprintf
