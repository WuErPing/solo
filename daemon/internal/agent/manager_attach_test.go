package agent

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestAttachPersistenceMetadata_NilHandle(t *testing.T) {
	result := attachPersistenceMetadata(nil, "/tmp", nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestAttachPersistenceMetadata_EmptySessionIDFallsBackToNativeHandle(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID:    "",
		NativeHandle: "native-123",
		Provider:     "mock",
	}

	result := attachPersistenceMetadata(handle, "", nil)
	if result.SessionID != "native-123" {
		t.Errorf("SessionID: got %q, want native-123", result.SessionID)
	}
}

func TestAttachPersistenceMetadata_EmptyNativeHandleFallsBackToSessionID(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID:    "session-456",
		NativeHandle: "",
		Provider:     "mock",
	}

	result := attachPersistenceMetadata(handle, "", nil)
	if result.NativeHandle != "session-456" {
		t.Errorf("NativeHandle: got %q, want session-456", result.NativeHandle)
	}
}

func TestAttachPersistenceMetadata_BothEmpty(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID:    "",
		NativeHandle: "",
		Provider:     "mock",
	}

	result := attachPersistenceMetadata(handle, "", nil)
	if result.SessionID != "" || result.NativeHandle != "" {
		t.Error("expected both to remain empty")
	}
}

func TestAttachPersistenceMetadata_CwdFromParameter(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}

	result := attachPersistenceMetadata(handle, "/my/project", nil)
	if cwd, ok := result.Metadata["cwd"].(string); !ok || cwd != "/my/project" {
		t.Errorf("cwd: got %v, want /my/project", result.Metadata["cwd"])
	}
}

func TestAttachPersistenceMetadata_CwdFromConfig(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}
	config := &protocol.AgentSessionConfig{
		Cwd: "/config/path",
	}

	result := attachPersistenceMetadata(handle, "", config)
	if cwd, ok := result.Metadata["cwd"].(string); !ok || cwd != "/config/path" {
		t.Errorf("cwd: got %v, want /config/path", result.Metadata["cwd"])
	}
}

func TestAttachPersistenceMetadata_CwdParamOverridesConfig(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}
	config := &protocol.AgentSessionConfig{
		Cwd: "/config/path",
	}

	result := attachPersistenceMetadata(handle, "/param/path", config)
	if cwd, ok := result.Metadata["cwd"].(string); !ok || cwd != "/param/path" {
		t.Errorf("cwd: got %v, want /param/path (param should override config)", result.Metadata["cwd"])
	}
}

func TestAttachPersistenceMetadata_ModelFromConfig(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}
	model := "claude-3"
	config := &protocol.AgentSessionConfig{
		Model: &model,
	}

	result := attachPersistenceMetadata(handle, "", config)
	if m, ok := result.Metadata["model"].(string); !ok || m != "claude-3" {
		t.Errorf("model: got %v, want claude-3", result.Metadata["model"])
	}
}

func TestAttachPersistenceMetadata_ModeIDFromConfig(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}
	modeID := "fast"
	config := &protocol.AgentSessionConfig{
		ModeID: &modeID,
	}

	result := attachPersistenceMetadata(handle, "", config)
	if m, ok := result.Metadata["modeId"].(string); !ok || m != "fast" {
		t.Errorf("modeId: got %v, want fast", result.Metadata["modeId"])
	}
}

func TestAttachPersistenceMetadata_ThinkingOptionFromConfig(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}
	thinking := "high"
	config := &protocol.AgentSessionConfig{
		ThinkingOptionID: &thinking,
	}

	result := attachPersistenceMetadata(handle, "", config)
	if th, ok := result.Metadata["thinkingOptionId"].(string); !ok || th != "high" {
		t.Errorf("thinkingOptionId: got %v, want high", result.Metadata["thinkingOptionId"])
	}
}

func TestAttachPersistenceMetadata_EmptyStringsNotAdded(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
	}
	empty := ""
	config := &protocol.AgentSessionConfig{
		Model:            &empty,
		ModeID:           &empty,
		ThinkingOptionID: &empty,
	}

	result := attachPersistenceMetadata(handle, "", config)
	if _, ok := result.Metadata["model"]; ok {
		t.Error("empty model should not be added to metadata")
	}
	if _, ok := result.Metadata["modeId"]; ok {
		t.Error("empty modeId should not be added to metadata")
	}
	if _, ok := result.Metadata["thinkingOptionId"]; ok {
		t.Error("empty thinkingOptionId should not be added to metadata")
	}
}

func TestAttachPersistenceMetadata_ExistingMetadataPreserved(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
		Metadata: map[string]interface{}{
			"existing": "value",
		},
	}

	result := attachPersistenceMetadata(handle, "/new/path", nil)
	if v, ok := result.Metadata["existing"].(string); !ok || v != "value" {
		t.Error("existing metadata should be preserved")
	}
	if cwd, ok := result.Metadata["cwd"].(string); !ok || cwd != "/new/path" {
		t.Error("new cwd should be added alongside existing metadata")
	}
}

func TestAttachPersistenceMetadata_DoesNotMutateOriginal(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID: "s1",
		Provider:  "mock",
		Metadata: map[string]interface{}{
			"key": "original",
		},
	}

	result := attachPersistenceMetadata(handle, "/new", nil)
	result.Metadata["key"] = "modified"

	if v := handle.Metadata["key"].(string); v != "original" {
		t.Error("original handle metadata should not be mutated")
	}
}

func TestAttachPersistenceMetadata_AllFieldsPopulated(t *testing.T) {
	handle := &protocol.AgentPersistenceHandle{
		SessionID:    "session-789",
		NativeHandle: "native-789",
		Provider:     "claude",
		Metadata: map[string]interface{}{
			"existing": "value",
		},
	}
	model := "claude-3-opus"
	modeID := "code"
	thinking := "budget-high"
	config := &protocol.AgentSessionConfig{
		Cwd:              "/workspace",
		Model:            &model,
		ModeID:           &modeID,
		ThinkingOptionID: &thinking,
	}

	result := attachPersistenceMetadata(handle, "/override", config)

	if result.SessionID != "session-789" {
		t.Errorf("SessionID: got %q", result.SessionID)
	}
	if result.NativeHandle != "native-789" {
		t.Errorf("NativeHandle: got %q", result.NativeHandle)
	}
	if result.Provider != "claude" {
		t.Errorf("Provider: got %q", result.Provider)
	}
	if v := result.Metadata["existing"].(string); v != "value" {
		t.Error("existing metadata lost")
	}
	if cwd := result.Metadata["cwd"].(string); cwd != "/override" {
		t.Errorf("cwd: got %q, want /override", cwd)
	}
	if m := result.Metadata["model"].(string); m != "claude-3-opus" {
		t.Errorf("model: got %q", m)
	}
	if m := result.Metadata["modeId"].(string); m != "code" {
		t.Errorf("modeId: got %q", m)
	}
	if th := result.Metadata["thinkingOptionId"].(string); th != "budget-high" {
		t.Errorf("thinkingOptionId: got %q", th)
	}
}
