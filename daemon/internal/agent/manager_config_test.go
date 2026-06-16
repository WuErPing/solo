package agent

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// --- cloneAgentConfig ---

func TestCloneAgentConfig_Nil(t *testing.T) {
	got := cloneAgentConfig(nil, "opencode", "/tmp")
	if got.Provider != "opencode" {
		t.Errorf("expected provider opencode, got %q", got.Provider)
	}
	if got.Cwd != "/tmp" {
		t.Errorf("expected cwd /tmp, got %q", got.Cwd)
	}
}

func TestCloneAgentConfig_FillsDefaults(t *testing.T) {
	cfg := &protocol.AgentSessionConfig{Cwd: "/home"}
	got := cloneAgentConfig(cfg, "claude", "/tmp")
	if got.Provider != "claude" {
		t.Errorf("expected provider claude, got %q", got.Provider)
	}
	if got.Cwd != "/home" {
		t.Errorf("expected cwd /home (from config), got %q", got.Cwd)
	}
}

func TestCloneAgentConfig_DoesNotOverwrite(t *testing.T) {
	cfg := &protocol.AgentSessionConfig{Provider: "opencode", Cwd: "/home"}
	got := cloneAgentConfig(cfg, "claude", "/tmp")
	if got.Provider != "opencode" {
		t.Errorf("expected provider opencode (from config), got %q", got.Provider)
	}
}

func TestCloneAgentConfig_ShallowCopy(t *testing.T) {
	model := "sonnet"
	cfg := &protocol.AgentSessionConfig{Provider: "opencode", Model: &model}
	got := cloneAgentConfig(cfg, "opencode", "/tmp")
	// Shallow copy: pointer fields share the same backing storage
	if *got.Model != "sonnet" {
		t.Errorf("expected model sonnet, got %q", *got.Model)
	}
	// But the top-level struct is a copy
	got.Provider = "claude"
	if cfg.Provider != "opencode" {
		t.Error("modifying the clone should not affect the original")
	}
}

// --- mergeAgentConfig ---

func TestMergeAgentConfig_NilOverrides(t *testing.T) {
	base := &protocol.AgentSessionConfig{Provider: "opencode", Cwd: "/home"}
	got := mergeAgentConfig(base, nil, "claude", "/tmp")
	if got.Provider != "opencode" {
		t.Errorf("expected base provider, got %q", got.Provider)
	}
}

func TestMergeAgentConfig_OverridesApplied(t *testing.T) {
	base := &protocol.AgentSessionConfig{Provider: "opencode", Cwd: "/home"}
	model := "sonnet"
	mode := "default"
	overrides := &protocol.AgentSessionConfig{
		Provider: "claude",
		Model:    &model,
		ModeID:   &mode,
		Cwd:      "/other",
	}
	got := mergeAgentConfig(base, overrides, "opencode", "/home")
	if got.Provider != "claude" {
		t.Errorf("expected provider claude, got %q", got.Provider)
	}
	if got.Cwd != "/other" {
		t.Errorf("expected cwd /other, got %q", got.Cwd)
	}
	if *got.Model != "sonnet" {
		t.Errorf("expected model sonnet, got %q", *got.Model)
	}
	if *got.ModeID != "default" {
		t.Errorf("expected mode default, got %q", *got.ModeID)
	}
}

func TestMergeAgentConfig_PartialOverrides(t *testing.T) {
	model := "sonnet"
	base := &protocol.AgentSessionConfig{Provider: "opencode", Cwd: "/home"}
	overrides := &protocol.AgentSessionConfig{Model: &model}
	got := mergeAgentConfig(base, overrides, "opencode", "/home")
	if got.Provider != "opencode" {
		t.Errorf("expected provider from base, got %q", got.Provider)
	}
	if *got.Model != "sonnet" {
		t.Errorf("expected model from overrides, got %q", *got.Model)
	}
}

// --- configFromPersistenceHandle ---

func TestConfigFromPersistenceHandle_Basic(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		Provider:     "claude",
		SessionID:    "sess-1",
		NativeHandle: "native-1",
		Metadata: map[string]interface{}{
			"cwd":    "/project",
			"model":  "sonnet",
			"modeId": "default",
		},
	}
	got := configFromPersistenceHandle(handle, nil)
	if got.Provider != "claude" {
		t.Errorf("expected provider claude, got %q", got.Provider)
	}
	if got.Cwd != "/project" {
		t.Errorf("expected cwd /project, got %q", got.Cwd)
	}
	if got.Model == nil || *got.Model != "sonnet" {
		t.Errorf("expected model sonnet, got %v", got.Model)
	}
	if got.ModeID == nil || *got.ModeID != "default" {
		t.Errorf("expected modeId default, got %v", got.ModeID)
	}
}

func TestConfigFromPersistenceHandle_WithOverrides(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		Provider: "claude",
		Metadata: map[string]interface{}{"cwd": "/old"},
	}
	newModel := "opus"
	overrides := &protocol.AgentSessionConfig{Model: &newModel}
	got := configFromPersistenceHandle(handle, overrides)
	if *got.Model != "opus" {
		t.Errorf("expected model opus from overrides, got %v", got.Model)
	}
}

func TestConfigFromPersistenceHandle_NilMetadata(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{Provider: "opencode"}
	got := configFromPersistenceHandle(handle, nil)
	if got.Provider != "opencode" {
		t.Errorf("expected provider opencode, got %q", got.Provider)
	}
}

// --- attachPersistenceMetadata ---

func TestAttachPersistenceMetadata_NilInput(t *testing.T) {
	got := attachPersistenceMetadata(nil, "/tmp", nil)
	if got != nil {
		t.Error("expected nil for nil handle")
	}
}

func TestAttachPersistenceMetadata_SyncsSessionIDs(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{SessionID: "sess-1"}
	got := attachPersistenceMetadata(handle, "/tmp", nil)
	if got.NativeHandle != "sess-1" {
		t.Errorf("expected NativeHandle=sess-1, got %q", got.NativeHandle)
	}

	handle2 := &protocol.AgentPersistenceHandle{NativeHandle: "native-1"}
	got2 := attachPersistenceMetadata(handle2, "/tmp", nil)
	if got2.SessionID != "native-1" {
		t.Errorf("expected SessionID=native-1, got %q", got2.SessionID)
	}
}

func TestAttachPersistenceMetadata_AttachesCwdAndModel(t *testing.T) {
	model := "sonnet"
	mode := "default"
	thinking := "low"
	config := &protocol.AgentSessionConfig{
		Cwd:              "/project",
		Model:            &model,
		ModeID:           &mode,
		ThinkingOptionID: &thinking,
	}
	handle := &protocol.AgentPersistenceHandle{SessionID: "s1"}
	got := attachPersistenceMetadata(handle, "", config)
	if got.Metadata["cwd"] != "/project" {
		t.Errorf("expected cwd in metadata, got %v", got.Metadata["cwd"])
	}
	if got.Metadata["model"] != "sonnet" {
		t.Errorf("expected model in metadata, got %v", got.Metadata["model"])
	}
	if got.Metadata["modeId"] != "default" {
		t.Errorf("expected modeId in metadata, got %v", got.Metadata["modeId"])
	}
	if got.Metadata["thinkingOptionId"] != "low" {
		t.Errorf("expected thinkingOptionId in metadata, got %v", got.Metadata["thinkingOptionId"])
	}
}

func TestAttachPersistenceMetadata_PreservesExistingMetadata(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Metadata:  map[string]interface{}{"custom": "value"},
	}
	got := attachPersistenceMetadata(handle, "/tmp", nil)
	if got.Metadata["custom"] != "value" {
		t.Error("expected existing metadata to be preserved")
	}
	if got.Metadata["cwd"] != "/tmp" {
		t.Error("expected cwd to be added")
	}
}

// --- streamEventTypeString ---

func TestStreamEventTypeString(t *testing.T) {
	tests := []struct {
		name  string
		event interface{}
		want  string
	}{
		{"TurnCompleted", protocol.TurnCompletedStreamEvent{Provider: "test"}, "turn_completed"},
		{"TurnFailed", protocol.TurnFailedStreamEvent{Provider: "test"}, "turn_failed"},
		{"MapWith Type", map[string]interface{}{"type": "timeline"}, "timeline"},
		{"MapNoType", map[string]interface{}{}, ""},
		{"Nil", nil, ""},
		{"OtherType", 42, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamEventTypeString(tt.event)
			if got != tt.want {
				t.Errorf("streamEventTypeString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- recordToManagedAgent ---

func TestRecordToManagedAgent_Basic(t *testing.T) {
	r := &StoredAgentRecord{
		ID:         "agent-1",
		Provider:   "opencode",
		Cwd:        "/project",
		CreatedAt:  "2025-01-01T00:00:00Z",
		UpdatedAt:  "2025-01-01T00:00:00Z",
		LastStatus: "idle",
	}
	got := recordToManagedAgent(r)
	if got.ID != "agent-1" {
		t.Errorf("expected ID agent-1, got %q", got.ID)
	}
	if got.Provider != "opencode" {
		t.Errorf("expected provider opencode, got %q", got.Provider)
	}
	if got.Lifecycle != protocol.AgentIdle {
		t.Errorf("expected lifecycle idle, got %q", got.Lifecycle)
	}
}

func TestRecordToManagedAgent_WithAttention(t *testing.T) {
	reason := "needs input"
	r := &StoredAgentRecord{
		ID:                "agent-1",
		Provider:          "opencode",
		Cwd:               "/project",
		CreatedAt:         "2025-01-01T00:00:00Z",
		UpdatedAt:         "2025-01-01T00:00:00Z",
		RequiresAttention: true,
		AttentionReason:   &reason,
	}
	got := recordToManagedAgent(r)
	if !got.Attention.Requires {
		t.Error("expected attention to be required")
	}
	if got.Attention.Reason != "needs input" {
		t.Errorf("expected attention reason 'needs input', got %q", got.Attention.Reason)
	}
}

func TestRecordToManagedAgent_WithConfig(t *testing.T) {
	model := "sonnet"
	r := &StoredAgentRecord{
		ID:        "agent-1",
		Provider:  "claude",
		Cwd:       "/project",
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-01T00:00:00Z",
		Config: &SerializableConfig{
			Model: &model,
		},
	}
	got := recordToManagedAgent(r)
	if got.Config == nil {
		t.Fatal("expected non-nil config")
	}
	if got.Config.Model == nil || *got.Config.Model != "sonnet" {
		t.Errorf("expected model sonnet, got %v", got.Config.Model)
	}
}
