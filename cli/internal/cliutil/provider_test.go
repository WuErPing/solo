package cliutil

import (
	"strings"
	"testing"
)

func TestResolveProviderModel_MissingProvider(t *testing.T) {
	_, err := ResolveProviderModel("", "", nil)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
	pe, ok := err.(*ProviderModelError)
	if !ok || pe.Code != "MISSING_PROVIDER" {
		t.Errorf("expected MISSING_PROVIDER, got %v", err)
	}
}

func TestResolveProviderModel_SlashNotation(t *testing.T) {
	result, err := ResolveProviderModel("openai/gpt-4", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider != "openai" {
		t.Errorf("expected provider openai, got %q", result.Provider)
	}
	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", result.Model)
	}
}

func TestResolveProviderModel_SlashWithWhitespace(t *testing.T) {
	result, err := ResolveProviderModel("  openai / gpt-4  ", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider != "openai" {
		t.Errorf("expected provider openai, got %q", result.Provider)
	}
	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", result.Model)
	}
}

func TestResolveProviderModel_InvalidSlash_EmptyProvider(t *testing.T) {
	_, err := ResolveProviderModel("/gpt-4", "", nil)
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
	pe := err.(*ProviderModelError)
	if pe.Code != "INVALID_PROVIDER" {
		t.Errorf("expected INVALID_PROVIDER, got %s", pe.Code)
	}
}

func TestResolveProviderModel_InvalidSlash_EmptyModel(t *testing.T) {
	_, err := ResolveProviderModel("openai/", "", nil)
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	pe := err.(*ProviderModelError)
	if pe.Code != "INVALID_PROVIDER" {
		t.Errorf("expected INVALID_PROVIDER, got %s", pe.Code)
	}
}

func TestResolveProviderModel_ConflictingModel(t *testing.T) {
	_, err := ResolveProviderModel("openai/gpt-4", "gpt-3.5", nil)
	if err == nil {
		t.Fatal("expected error for conflicting model")
	}
	pe := err.(*ProviderModelError)
	if pe.Code != "CONFLICTING_MODEL" {
		t.Errorf("expected CONFLICTING_MODEL, got %s", pe.Code)
	}
}

func TestResolveProviderModel_AgreeingModel(t *testing.T) {
	result, err := ResolveProviderModel("openai/gpt-4", "gpt-4", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", result.Model)
	}
}

func TestResolveProviderModel_ProviderOnly(t *testing.T) {
	result, err := ResolveProviderModel("openai", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider != "openai" {
		t.Errorf("expected provider openai, got %q", result.Provider)
	}
	if result.Model != "" {
		t.Errorf("expected empty model, got %q", result.Model)
	}
}

func TestResolveProviderModel_ProviderAndModel(t *testing.T) {
	result, err := ResolveProviderModel("openai", "gpt-4", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider != "openai" {
		t.Errorf("expected provider openai, got %q", result.Provider)
	}
	if result.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", result.Model)
	}
}

func TestResolveProviderModel_EmptyModelFlag(t *testing.T) {
	_, err := ResolveProviderModel("openai", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderModelError_Error(t *testing.T) {
	err := &ProviderModelError{Code: "TEST", Message: "msg", Details: "details"}
	if !strings.Contains(err.Error(), "TEST") {
		t.Error("expected Code in error string")
	}
	if !strings.Contains(err.Error(), "msg") {
		t.Error("expected Message in error string")
	}
	if !strings.Contains(err.Error(), "details") {
		t.Error("expected Details in error string")
	}

	errNoDetails := &ProviderModelError{Code: "TEST", Message: "msg"}
	if strings.Contains(errNoDetails.Error(), "details") {
		t.Error("expected no details in short error")
	}
}
