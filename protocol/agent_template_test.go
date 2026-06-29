package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func ptr(s string) *string { return &s }

func TestAgentTemplateAlias(t *testing.T) {
	// AgentTemplate must be identical to AgentSessionConfig.
	var _ = AgentSessionConfig(AgentTemplate{})
	var _ = AgentTemplate(AgentSessionConfig{})

	tmpl := AgentTemplate{
		Provider:     "claude",
		Cwd:          "/tmp",
		SystemPrompt: "be helpful",
		McpServers: map[string]McpServerConfig{
			"fs": {Type: "stdio", Command: "mcp-server-filesystem"},
		},
	}

	b, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got AgentTemplate
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(got, tmpl) {
		t.Errorf("round-trip mismatch:\n got %#v\nwant %#v", got, tmpl)
	}
}

func TestScheduleTargetWithAgentTemplate(t *testing.T) {
	tmpl := &AgentTemplate{
		Provider:       "kimi",
		Cwd:            "/workspace",
		Model:          ptr("kimi-k2"),
		ModeID:         ptr("code"),
		ApprovalPolicy: "dangerous-only",
		SandboxMode:    "none",
		NetworkAccess:  true,
		WebSearch:      true,
		SystemPrompt:   "write clean code",
		McpServers: map[string]McpServerConfig{
			"git": {Type: "stdio", Command: "mcp-server-git"},
		},
		Extra: map[string]interface{}{
			"temperature": 0.2,
		},
	}

	target := ScheduleTarget{
		Type:   "new-agent",
		Config: tmpl,
	}

	b, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ScheduleTarget
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != target.Type {
		t.Errorf("type: got %q, want %q", got.Type, target.Type)
	}
	if got.Config == nil {
		t.Fatal("config is nil after round-trip")
	}
	if !reflect.DeepEqual(got.Config, tmpl) {
		t.Errorf("config mismatch:\n got %#v\nwant %#v", got.Config, tmpl)
	}
}

func TestScheduleAgentConfigDeprecatedAlias(t *testing.T) {
	// JSON produced by the old ScheduleAgentConfig struct must decode into
	// the new AgentTemplate field of ScheduleTarget.
	oldJSON := `{
		"type": "new-agent",
		"config": {
			"provider": "claude",
			"cwd": "/legacy",
			"model": "claude-3-5-sonnet",
			"systemPrompt": "legacy prompt",
			"mcpServers": {
				"fs": {"type": "stdio", "command": "mcp-fs"}
			}
		}
	}`

	var got ScheduleTarget
	if err := json.Unmarshal([]byte(oldJSON), &got); err != nil {
		t.Fatalf("unmarshal old JSON: %v", err)
	}

	if got.Type != "new-agent" {
		t.Errorf("type: got %q, want new-agent", got.Type)
	}
	if got.Config == nil {
		t.Fatal("config is nil")
	}
	if got.Config.Provider != "claude" {
		t.Errorf("provider: got %q, want claude", got.Config.Provider)
	}
	if got.Config.Cwd != "/legacy" {
		t.Errorf("cwd: got %q, want /legacy", got.Config.Cwd)
	}
	if got.Config.SystemPrompt != "legacy prompt" {
		t.Errorf("systemPrompt: got %q, want legacy prompt", got.Config.SystemPrompt)
	}
	if len(got.Config.McpServers) != 1 {
		t.Fatalf("mcpServers length: got %d, want 1", len(got.Config.McpServers))
	}
	if got.Config.McpServers["fs"].Type != "stdio" {
		t.Errorf("mcp server type: got %q, want stdio", got.Config.McpServers["fs"].Type)
	}
}

func TestLoopRecordLegacyCompatibility(t *testing.T) {
	legacyJSON := `{
		"id": "loop-legacy",
		"prompt": "fix tests",
		"cwd": "/project",
		"provider": "claude",
		"model": "claude-3-opus",
		"workerProvider": "kimi",
		"workerModel": "kimi-k2",
		"verifierProvider": "opencode",
		"verifierModel": "deepseek",
		"status": "running",
		"createdAt": "2026-06-29T00:00:00Z",
		"updatedAt": "2026-06-29T00:00:00Z",
		"startedAt": "2026-06-29T00:00:00Z",
		"nextLogSeq": 1,
		"iterations": [],
		"logs": []
	}`

	var got LoopRecord
	if err := json.Unmarshal([]byte(legacyJSON), &got); err != nil {
		t.Fatalf("unmarshal legacy JSON: %v", err)
	}

	if got.Provider != "claude" {
		t.Errorf("provider: got %q, want claude", got.Provider)
	}
	if got.Model == nil || *got.Model != "claude-3-opus" {
		t.Errorf("model: got %v, want claude-3-opus", got.Model)
	}
	if got.WorkerProvider == nil || *got.WorkerProvider != "kimi" {
		t.Errorf("workerProvider: got %v, want kimi", got.WorkerProvider)
	}
	if got.WorkerModel == nil || *got.WorkerModel != "kimi-k2" {
		t.Errorf("workerModel: got %v, want kimi-k2", got.WorkerModel)
	}
	if got.VerifierProvider == nil || *got.VerifierProvider != "opencode" {
		t.Errorf("verifierProvider: got %v, want opencode", got.VerifierProvider)
	}
	if got.VerifierModel == nil || *got.VerifierModel != "deepseek" {
		t.Errorf("verifierModel: got %v, want deepseek", got.VerifierModel)
	}
}

func TestLoopRecordTemplateRoundTrip(t *testing.T) {
	base := &AgentTemplate{
		Provider:     "claude",
		Cwd:          "/project",
		Model:        ptr("claude-3-opus"),
		SystemPrompt: "base prompt",
		McpServers: map[string]McpServerConfig{
			"base": {Type: "stdio", Command: "base-cmd"},
		},
	}
	worker := &AgentTemplate{
		Provider:     "kimi",
		Cwd:          "/project",
		Model:        ptr("kimi-k2"),
		SystemPrompt: "worker prompt",
	}
	verifier := &AgentTemplate{
		Provider:     "opencode",
		Cwd:          "/project",
		Model:        ptr("deepseek-chat"),
		SystemPrompt: "verifier prompt",
	}

	record := LoopRecord{
		ID:                    "loop-template",
		Prompt:                "fix tests",
		Cwd:                   "/project",
		Status:                "running",
		CreatedAt:             "2026-06-29T00:00:00Z",
		UpdatedAt:             "2026-06-29T00:00:00Z",
		StartedAt:             "2026-06-29T00:00:00Z",
		NextLogSeq:            1,
		Iterations:            []LoopIterationRecord{},
		Logs:                  []LoopLogEntry{},
		AgentTemplate:         base,
		WorkerAgentTemplate:   worker,
		VerifierAgentTemplate: verifier,
	}

	b, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got LoopRecord
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(got.AgentTemplate, base) {
		t.Errorf("agentTemplate mismatch:\n got %#v\nwant %#v", got.AgentTemplate, base)
	}
	if !reflect.DeepEqual(got.WorkerAgentTemplate, worker) {
		t.Errorf("workerAgentTemplate mismatch:\n got %#v\nwant %#v", got.WorkerAgentTemplate, worker)
	}
	if !reflect.DeepEqual(got.VerifierAgentTemplate, verifier) {
		t.Errorf("verifierAgentTemplate mismatch:\n got %#v\nwant %#v", got.VerifierAgentTemplate, verifier)
	}
}
