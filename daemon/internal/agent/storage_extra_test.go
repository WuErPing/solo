package agent

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestToStoredAgentRecordWithTitleOption(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	existing := &StoredAgentRecord{
		ID:        "a1",
		Title:     strPtr("Existing Title"),
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	// WithTitle should override existing title
	record := toStoredAgentRecord(agent, existing, &snapshotOptions{
		title: strPtr("New Title"),
	})
	if record.Title == nil || *record.Title != "New Title" {
		t.Errorf("Title: got %v, want 'New Title'", record.Title)
	}
	if record.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt: got %q, want existing", record.CreatedAt)
	}
}

func TestToStoredAgentRecordWithInternalOption(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	agent.Internal = false

	// WithInternal(true) should override agent.Internal
	record := toStoredAgentRecord(agent, nil, &snapshotOptions{
		internal: boolPtr(true),
	})
	if record.Internal != true {
		t.Errorf("Internal: got %v, want true", record.Internal)
	}
}

func TestToStoredAgentRecordWithoutOptions(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	agent.Internal = true
	agent.Config = &protocol.AgentSessionConfig{
		ModeID: strPtr("default"),
	}
	agent.RuntimeInfo = &protocol.AgentRuntimeInfo{
		Provider: "mock",
		Model:    strPtr("m1"),
	}
	agent.Persistence = &protocol.AgentPersistenceHandle{
		Provider:  "mock",
		SessionID: "sid-1",
	}

	record := toStoredAgentRecord(agent, nil, &snapshotOptions{})
	if record.Internal != true {
		t.Errorf("Internal: got %v, want true", record.Internal)
	}
	if record.Config == nil {
		t.Error("expected Config to be set")
	}
	if record.RuntimeInfo == nil {
		t.Error("expected RuntimeInfo to be set")
	}
	if record.Persistence == nil {
		t.Error("expected Persistence to be set")
	}
	if record.LastModeID == nil || *record.LastModeID != "default" {
		t.Error("expected LastModeID to fall back to Config.ModeID")
	}
}

func TestToStoredAgentRecordArchivesFromExisting(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	existing := &StoredAgentRecord{
		ID:         "a1",
		ArchivedAt: strPtr("2024-06-01T00:00:00Z"),
	}

	record := toStoredAgentRecord(agent, existing, &snapshotOptions{})
	if record.ArchivedAt == nil || *record.ArchivedAt != "2024-06-01T00:00:00Z" {
		t.Error("expected ArchivedAt to be inherited from existing")
	}
}

func TestToStoredAgentRecordLastUserMessageAt(t *testing.T) {
	agent := NewManagedAgent("a1", "mock", "/tmp", nil, nil)
	now := time.Now()
	agent.LastUserMessageAt = &now

	record := toStoredAgentRecord(agent, nil, &snapshotOptions{})
	if record.LastUserMessageAt == nil {
		t.Error("expected LastUserMessageAt to be set")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
