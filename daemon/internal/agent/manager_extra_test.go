package agent

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func createTestManagerWithMock(t *testing.T) *AgentManager {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(NewMockAgentClient())
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}
	return manager
}

func TestAgentManagerListAgents(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// ListAgents should include active agents
	list := m.ListAgents()
	if len(list) != 1 || list[0].ID != ag.ID {
		t.Errorf("ListAgents: got %d agents, want 1", len(list))
	}

	// ListAllAgents should include from storage too
	all := m.ListAllAgents()
	if len(all) != 1 {
		t.Errorf("ListAllAgents: got %d agents, want 1", len(all))
	}
}

func TestAgentManagerListAgentsWithPersisted(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Remove from memory only, keeping storage intact
	m.mu.Lock()
	delete(m.agents, ag.ID)
	m.mu.Unlock()

	// ListAgents should be empty
	if len(m.ListAgents()) != 0 {
		t.Error("expected ListAgents to be empty")
	}

	// ListAgentsWithPersisted should include the persisted agent
	list := m.ListAgentsWithPersisted()
	if len(list) != 1 {
		t.Errorf("expected 1 agent in ListAgentsWithPersisted, got %d", len(list))
	}
}

func TestAgentManagerFindAgentByPersistence(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Agent should have persistence after creation
	if ag.Persistence == nil {
		t.Fatal("expected agent to have persistence")
	}

	found := m.findAgentByPersistence(ag.Persistence)
	if found == nil || found.ID != ag.ID {
		t.Error("expected to find agent by persistence")
	}

	// Non-matching handle
	notFound := m.findAgentByPersistence(&protocol.AgentPersistenceHandle{Provider: "other", SessionID: "other"})
	if notFound != nil {
		t.Error("expected nil for non-matching handle")
	}
}

func TestAgentManagerEnsureAgentSession(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// ensureAgentSession on agent with existing session should return same agent
	same, err := m.ensureAgentSession(context.Background(), ag)
	if err != nil {
		t.Fatalf("ensureAgentSession: %v", err)
	}
	if same.ID != ag.ID {
		t.Error("expected same agent")
	}

	// Remove session and ensure it is recreated
	ag.SetSession(nil)
	recreated, err := m.ensureAgentSession(context.Background(), ag)
	if err != nil {
		t.Fatalf("ensureAgentSession after clearing: %v", err)
	}
	if recreated.GetSession() == nil {
		t.Error("expected session to be recreated")
	}
}

func TestAgentManagerResumeAgentFromPersistenceValidation(t *testing.T) {
	m := createTestManagerWithMock(t)

	// Nil handle
	if _, err := m.ResumeAgentFromPersistence(context.Background(), nil, nil); err == nil {
		t.Error("expected error for nil handle")
	}

	// Empty provider
	if _, err := m.ResumeAgentFromPersistence(context.Background(), &protocol.AgentPersistenceHandle{Provider: ""}, nil); err == nil {
		t.Error("expected error for empty provider")
	}
}

func TestAgentManagerResumeAgentFromPersistenceExisting(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Resume with matching persistence handle should find existing agent
	if ag.Persistence == nil {
		t.Fatal("expected persistence")
	}
	resumed, err := m.ResumeAgentFromPersistence(context.Background(), ag.Persistence, nil)
	if err != nil {
		t.Fatalf("ResumeAgentFromPersistence: %v", err)
	}
	if resumed.ID != ag.ID {
		t.Errorf("expected same agent ID, got %s", resumed.ID)
	}
}

func TestAgentManagerHydrateTimeline(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	store := NewInMemoryTimelineStore()
	store.Initialize(ag.ID)

	// First hydrate should succeed
	if err := m.HydrateTimeline(context.Background(), ag.ID, store); err != nil {
		t.Fatalf("HydrateTimeline: %v", err)
	}

	// Second hydrate should be a no-op (idempotent)
	if err := m.HydrateTimeline(context.Background(), ag.ID, store); err != nil {
		t.Fatalf("HydrateTimeline second call: %v", err)
	}

	// Non-existent agent should error
	if err := m.HydrateTimeline(context.Background(), "nonexistent", store); err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestAgentManagerDeleteAgent(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	if err := m.DeleteAgent(ag.ID); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}

	if m.GetAgent(ag.ID) != nil {
		t.Error("expected agent to be deleted")
	}

	// Deleting non-existent agent should error
	if err := m.DeleteAgent("nonexistent"); err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestAgentManagerArchiveAgent(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	if err := m.ArchiveAgent(ag.ID); err != nil {
		t.Fatalf("ArchiveAgent: %v", err)
	}

	if m.GetAgent(ag.ID) != nil {
		t.Error("expected agent to be removed from memory")
	}

	// Storage should still have it with archivedAt
	record := m.storage.Get(ag.ID)
	if record == nil || record.ArchivedAt == nil {
		t.Error("expected archived record in storage")
	}

	// Archiving non-existent agent should error
	if err := m.ArchiveAgent("nonexistent"); err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestAgentManagerCancelAgentRun(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Cancel should succeed when agent has session
	if err := m.CancelAgentRun(context.Background(), ag.ID); err != nil {
		t.Fatalf("CancelAgentRun: %v", err)
	}

	// Cancel non-existent agent should error
	if err := m.CancelAgentRun(context.Background(), "nonexistent"); err == nil {
		t.Error("expected error for non-existent agent")
	}

	// Cancel agent without session should error
	ag2 := NewManagedAgent("no-session", "mock", "/tmp", nil, nil)
	m.mu.Lock()
	m.agents["no-session"] = ag2
	m.mu.Unlock()
	if err := m.CancelAgentRun(context.Background(), "no-session"); err == nil {
		t.Error("expected error for agent without session")
	}
}

func TestAgentManagerClearAgentAttention(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	ag.SetAttention(true, "test")

	snapshot, err := m.ClearAgentAttention(ag.ID)
	if err != nil {
		t.Fatalf("ClearAgentAttention: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.RequiresAttention {
		t.Error("expected attention to be cleared")
	}

	// Clear non-existent agent should error
	if _, err := m.ClearAgentAttention("nonexistent"); err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestAgentManagerCoalescerFlusher(t *testing.T) {
	m := createTestManagerWithMock(t)

	var flushedAgentID string
	id := m.RegisterCoalescerFlusher(func(agentID string) {
		flushedAgentID = agentID
	})

	m.fireCoalescerFlush("agent-1")
	if flushedAgentID != "agent-1" {
		t.Errorf("flushedAgentID: got %q, want agent-1", flushedAgentID)
	}

	m.UnregisterCoalescerFlusher(id)
	flushedAgentID = ""
	m.fireCoalescerFlush("agent-2")
	if flushedAgentID != "" {
		t.Error("expected no flush after unregister")
	}
}

func TestRecordToManagedAgent(t *testing.T) {
	record := &StoredAgentRecord{
		ID:                 "a1",
		Provider:           "mock",
		Cwd:                "/tmp",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:          time.Now().UTC().Format(time.RFC3339),
		LastStatus:         "idle",
		RequiresAttention:  true,
		AttentionReason:    strPtr("msg"),
		AttentionTimestamp: strPtr(time.Now().UTC().Format(time.RFC3339)),
		Title:              strPtr("Test Agent"),
		Internal:           true,
		Labels:             map[string]string{"env": "test"},
		Config: &SerializableConfig{
			Title: strPtr("Config Title"),
		},
		RuntimeInfo: &StoredRuntimeInfo{
			Provider: "mock",
			Model:    strPtr("m1"),
		},
		Persistence: &protocol.AgentPersistenceHandle{
			Provider:  "mock",
			SessionID: "sid-1",
		},
		LastError: strPtr("oops"),
		ArchivedAt: strPtr("2024-01-01T00:00:00Z"),
	}

	agent := recordToManagedAgent(record)
	if agent.ID != "a1" {
		t.Errorf("ID: got %q", agent.ID)
	}
	if !agent.Attention.Requires {
		t.Error("expected attention required")
	}
	if !agent.Internal {
		t.Error("expected internal to be true")
	}
	if agent.Config == nil || agent.Config.Title == nil || *agent.Config.Title != "Config Title" {
		t.Error("expected config title")
	}
	if agent.RuntimeInfo == nil || agent.RuntimeInfo.Model == nil || *agent.RuntimeInfo.Model != "m1" {
		t.Error("expected runtime info model")
	}
	if agent.Persistence == nil || agent.Persistence.SessionID != "sid-1" {
		t.Error("expected persistence")
	}
	if agent.LastError == nil || *agent.LastError != "oops" {
		t.Error("expected last error")
	}
	if agent.ArchivedAt == nil {
		t.Error("expected archived at")
	}
}

func TestStoredToRuntimeInfoNil(t *testing.T) {
	if storedToRuntimeInfo(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestCloneAgentConfig(t *testing.T) {
	// nil config
	cfg := cloneAgentConfig(nil, "mock", "/tmp")
	if cfg.Provider != "mock" || cfg.Cwd != "/tmp" {
		t.Errorf("got %+v", cfg)
	}

	// existing config with empty provider/cwd
	base := &protocol.AgentSessionConfig{Provider: "", Cwd: ""}
	cfg = cloneAgentConfig(base, "mock", "/tmp")
	if cfg.Provider != "mock" || cfg.Cwd != "/tmp" {
		t.Errorf("got %+v", cfg)
	}

	// existing config with values preserved
	base = &protocol.AgentSessionConfig{Provider: "pi", Cwd: "/home", ModeID: strPtr("default")}
	cfg = cloneAgentConfig(base, "mock", "/tmp")
	if cfg.Provider != "pi" || cfg.Cwd != "/home" {
		t.Errorf("got %+v", cfg)
	}
}

func TestMergeAgentConfig(t *testing.T) {
	base := &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp", ModeID: strPtr("default")}
	overrides := &protocol.AgentSessionConfig{
		Provider:         "pi",
		Cwd:              "/home",
		ModeID:           strPtr("advanced"),
		Model:            strPtr("gpt-4"),
		ThinkingOptionID: strPtr("full"),
		FeatureValues:    map[string]interface{}{"voice": true},
		Title:            strPtr("New Title"),
		Extra:            map[string]interface{}{"key": "val"},
		SystemPrompt:     "be helpful",
		McpServers:       map[string]protocol.McpServerConfig{"test": {Type: "stdio"}},
		OutputSchema:     map[string]interface{}{"type": "object"},
	}

	merged := mergeAgentConfig(base, overrides, "mock", "/tmp")
	if merged.Provider != "pi" {
		t.Errorf("Provider: got %q", merged.Provider)
	}
	if merged.ModeID == nil || *merged.ModeID != "advanced" {
		t.Error("expected ModeID overridden")
	}
	if merged.Model == nil || *merged.Model != "gpt-4" {
		t.Error("expected Model overridden")
	}
	if merged.ThinkingOptionID == nil || *merged.ThinkingOptionID != "full" {
		t.Error("expected ThinkingOptionID overridden")
	}
	if merged.FeatureValues == nil {
		t.Error("expected FeatureValues overridden")
	}
	if v, ok := merged.FeatureValues["voice"].(bool); !ok || !v {
		t.Error("expected FeatureValues voice to be true")
	}
	if merged.Title == nil || *merged.Title != "New Title" {
		t.Error("expected Title overridden")
	}
	if merged.Extra == nil {
		t.Error("expected Extra overridden")
	}
	if merged.SystemPrompt != "be helpful" {
		t.Error("expected SystemPrompt overridden")
	}
	if merged.McpServers == nil {
		t.Error("expected McpServers overridden")
	}
	if merged.OutputSchema == nil {
		t.Error("expected OutputSchema overridden")
	}
}

func TestConfigFromPersistenceHandle(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		Provider:  "mock",
		SessionID: "sid-1",
		Metadata: map[string]interface{}{
			"cwd":                "/tmp",
			"model":              "gpt-4",
			"modeId":             "default",
			"thinkingOptionId":   "full",
		},
	}
	overrides := &protocol.AgentSessionConfig{Title: strPtr("Override")}

	cfg := configFromPersistenceHandle(handle, overrides)
	if cfg.Provider != "mock" {
		t.Errorf("Provider: got %q", cfg.Provider)
	}
	if cfg.Cwd != "/tmp" {
		t.Errorf("Cwd: got %q", cfg.Cwd)
	}
	if cfg.Model == nil || *cfg.Model != "gpt-4" {
		t.Error("expected model from metadata")
	}
	if cfg.ModeID == nil || *cfg.ModeID != "default" {
		t.Error("expected modeId from metadata")
	}
	if cfg.ThinkingOptionID == nil || *cfg.ThinkingOptionID != "full" {
		t.Error("expected thinkingOptionId from metadata")
	}
	if cfg.Title == nil || *cfg.Title != "Override" {
		t.Error("expected title from overrides")
	}
}

// TestAgentManagerConcurrentCreateDelete stresses CreateAgent and DeleteAgent
// running concurrently. Run with -race to verify data-race freedom.
func TestAgentManagerConcurrentCreateDelete(t *testing.T) {
	m := createTestManagerWithMock(t)

	var wg sync.WaitGroup
	var idsMu sync.Mutex
	var ids []string

	// Concurrent creators
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
				if err != nil {
					t.Errorf("create agent: %v", err)
					return
				}
				idsMu.Lock()
				ids = append(ids, ag.ID)
				idsMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	list := m.ListAgents()
	if len(list) != 50 {
		t.Errorf("expected 50 agents, got %d", len(list))
	}

	// Concurrent deleters
	idsMu.Lock()
	toDelete := make([]string, len(ids))
	copy(toDelete, ids)
	idsMu.Unlock()

	var delWg sync.WaitGroup
	for _, id := range toDelete {
		delWg.Add(1)
		go func(agentID string) {
			defer delWg.Done()
			if err := m.DeleteAgent(agentID); err != nil {
				t.Errorf("delete agent: %v", err)
			}
		}(id)
	}
	delWg.Wait()

	if len(m.ListAgents()) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(m.ListAgents()))
	}
}

// TestAgentManagerConcurrentCreateArchive stresses CreateAgent and ArchiveAgent
// running concurrently with reads. Run with -race.
func TestAgentManagerConcurrentCreateArchive(t *testing.T) {
	m := createTestManagerWithMock(t)

	var wg sync.WaitGroup
	created := make(chan string, 30)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
				if err != nil {
					t.Errorf("create agent: %v", err)
					return
				}
				created <- ag.ID
			}
		}(i)
	}

	// Concurrent readers + archivers
	var readWg sync.WaitGroup
	readWg.Add(1)
	go func() {
		defer readWg.Done()
		for id := range created {
			_ = m.GetAgent(id)
			_ = m.ListAgents()
			_ = m.ArchiveAgent(id)
		}
	}()

	wg.Wait()
	close(created)
	readWg.Wait()

	if len(m.ListAgents()) != 0 {
		t.Errorf("expected 0 agents after archive, got %d", len(m.ListAgents()))
	}
}

// TestAgentManagerConcurrentReadWrite stresses read-only operations against
// lifecycle mutations. Run with -race.
func TestAgentManagerConcurrentReadWrite(t *testing.T) {
	m := createTestManagerWithMock(t)

	ag, err := m.CreateAgent(context.Background(), &protocol.AgentSessionConfig{Provider: "mock", Cwd: "/tmp"}, nil)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = m.GetAgent(ag.ID)
				_ = m.ListAgents()
				_ = m.ListAllAgents()
			}
		}(i)
	}

	// Mutators
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				ag.SetAttention(true, "test")
				ag.ClearAttention()
				ag.SetError("err")
				ag.ClearError()
			}
		}(i)
	}

	wg.Wait()
}
