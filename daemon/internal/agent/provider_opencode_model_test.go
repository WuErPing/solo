package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestBuildOpenCodeModelDefinition(t *testing.T) {
	model := opencodeModelInfo{
		Name:   "Test Model",
		Family: json.RawMessage(`"test-family"`),
		Variants: map[string]interface{}{
			"fast":     "v1",
			"thorough": "v2",
		},
		Capabilities: &opencodeCapabilities{
			Temperature: true,
			Reasoning:   true,
			Attachment:  false,
			ToolCall:    true,
		},
		Cost: map[string]interface{}{"input": 0.001},
		Limit: &opencodeModelLimit{
			Context: ptr(float64(128000)),
			Input:   ptr(float64(32000)),
			Output:  ptr(float64(16000)),
		},
	}

	def := buildOpenCodeModelDefinition("provider1", "Provider One", "model1", model)

	if def.Provider != opencodeProviderName {
		t.Errorf("Provider: got %q, want %q", def.Provider, opencodeProviderName)
	}
	if def.ID != "provider1/model1" {
		t.Errorf("ID: got %q, want %q", def.ID, "provider1/model1")
	}
	if def.Label != "Test Model" {
		t.Errorf("Label: got %q, want %q", def.Label, "Test Model")
	}
	if len(def.ThinkingOptions) != 2 {
		t.Errorf("expected 2 thinking options, got %d", len(def.ThinkingOptions))
	}
	if def.DefaultThinkingOptionID != "fast" {
		t.Error("expected default thinking option 'fast'")
	}

	// Metadata checks
	meta := def.Metadata
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta["providerId"] != "provider1" {
		t.Errorf("metadata providerId: got %v", meta["providerId"])
	}
	if meta["supportsReasoning"] != true {
		t.Error("expected supportsReasoning true")
	}
	if meta["supportsAttachments"] != false {
		t.Error("expected supportsAttachments false")
	}
	if meta["contextWindowMaxTokens"] != 128000 {
		t.Errorf("contextWindow: got %v", meta["contextWindowMaxTokens"])
	}
}

func TestBuildOpenCodeModelDefinition_NoVariants(t *testing.T) {
	model := opencodeModelInfo{
		Name: "Simple Model",
	}

	def := buildOpenCodeModelDefinition("p", "P", "m", model)
	if def.DefaultThinkingOptionID != "" {
		t.Error("expected empty default thinking when no variants")
	}
	if len(def.ThinkingOptions) != 0 {
		t.Errorf("expected 0 thinking options, got %d", len(def.ThinkingOptions))
	}
}

func TestExtractModelContextWindow(t *testing.T) {
	model := opencodeModelInfo{
		Limit: &opencodeModelLimit{Context: ptr(float64(64000))},
	}
	got := extractModelContextWindow(model)
	if got == nil || *got != 64000 {
		t.Errorf("expected 64000, got %v", got)
	}

	modelNoLimit := opencodeModelInfo{}
	got = extractModelContextWindow(modelNoLimit)
	if got != nil {
		t.Error("expected nil for no limit")
	}
}

func TestBuildOpenCodeModelDefinition_FamilyAsString(t *testing.T) {
	model := opencodeModelInfo{
		Name:   "M",
		Family: json.RawMessage(`"gpt"`),
	}
	def := buildOpenCodeModelDefinition("p", "P", "m", model)
	if def.Description != "P - gpt" {
		t.Errorf("Description: got %q", def.Description)
	}
}

func TestBuildOpenCodeModelDefinition_FamilyAsObject(t *testing.T) {
	model := opencodeModelInfo{
		Name:   "M",
		Family: json.RawMessage(`{"name":"gpt-4"}`),
	}
	def := buildOpenCodeModelDefinition("p", "P", "m", model)
	// Object family is marshaled to string representation
	if !strings.HasPrefix(def.Description, "P - ") {
		t.Errorf("Description: got %q", def.Description)
	}
}
