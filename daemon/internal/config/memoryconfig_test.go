package config

import (
	"strings"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/memory/redact"
)

func TestMemoryConfig_ApplyDefaults_FillsZeroFields(t *testing.T) {
	var c MemoryConfig
	c.ApplyDefaults()

	if c.Backend != "file" {
		t.Errorf("Backend = %q, want 'file'", c.Backend)
	}
	if c.QueueSize != 1024 {
		t.Errorf("QueueSize = %d, want 1024", c.QueueSize)
	}
	if c.RetentionDays != 90 {
		t.Errorf("RetentionDays = %d, want 90", c.RetentionDays)
	}
	if c.Overflow != "block" {
		t.Errorf("Overflow = %q, want 'block'", c.Overflow)
	}
	if c.Root != "memory" {
		t.Errorf("Root = %q, want 'memory'", c.Root)
	}
}

func TestMemoryConfig_ApplyDefaults_PreservesExplicitValues(t *testing.T) {
	c := MemoryConfig{
		Backend:       "sqlite",
		QueueSize:     512,
		RetentionDays: 30,
		Overflow:      "error",
		Root:          "custom/memory",
	}
	c.ApplyDefaults()

	if c.Backend != "sqlite" {
		t.Errorf("Backend = %q, want 'sqlite'", c.Backend)
	}
	if c.QueueSize != 512 {
		t.Errorf("QueueSize = %d, want 512", c.QueueSize)
	}
	if c.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", c.RetentionDays)
	}
	if c.Overflow != "error" {
		t.Errorf("Overflow = %q, want 'error'", c.Overflow)
	}
	if c.Root != "custom/memory" {
		t.Errorf("Root = %q, want 'custom/memory'", c.Root)
	}
}

func TestMemoryConfig_Validate_AcceptsKnownBackends(t *testing.T) {
	for _, b := range []string{"file", "sqlite", "middleware"} {
		c := MemoryConfig{Backend: b}
		c.ApplyDefaults()
		if err := c.Validate(); err != nil {
			t.Errorf("backend %q should validate: %v", b, err)
		}
	}
}

func TestMemoryConfig_Validate_RejectsUnknownBackend(t *testing.T) {
	c := MemoryConfig{Backend: "postgres"}
	if err := c.Validate(); err == nil {
		t.Error("expected validation error for unknown backend")
	}
}

func TestMemoryConfig_Validate_RejectsBadOverflow(t *testing.T) {
	c := MemoryConfig{Backend: "file", Overflow: "drop"}
	if err := c.Validate(); err == nil {
		t.Error("expected validation error for unknown overflow policy")
	}
}

func TestMemoryConfig_Validate_RejectsNonPositiveQueueSize(t *testing.T) {
	c := MemoryConfig{Backend: "file", Overflow: "block", QueueSize: 0}
	if err := c.Validate(); err == nil {
		t.Error("expected validation error for QueueSize=0")
	}
}

// Enabled is a *bool: nil means "auto-enable" (the new default behavior).
func TestMemoryConfig_NilEnabledMeansAutoEnabled(t *testing.T) {
	var c MemoryConfig
	if !c.IsEnabled() {
		t.Error("nil Enabled should auto-enable (opt-out model)")
	}
}

func TestMemoryConfig_ExplicitFalse_Disables(t *testing.T) {
	var c MemoryConfig
	c.SetEnabled(false)
	if c.IsEnabled() {
		t.Error("explicit Enabled=false should disable")
	}
}

func TestMemoryConfig_ExplicitTrue_Enables(t *testing.T) {
	var c MemoryConfig
	c.SetEnabled(true)
	if !c.IsEnabled() {
		t.Error("explicit Enabled=true should enable")
	}
}

// Round-trip: the embedded RedactorConfig must flow through to BuildRedactor
// and actually redact matching content.
func TestMemoryConfig_Redact_RoundTrip(t *testing.T) {
	var c MemoryConfig
	c.SetEnabled(true)
	c.Redact.APIKeys = true
	c.Redact.EnvFiles = true
	c.ApplyDefaults()

	r, err := redact.BuildRedactor(c.Redact)
	if err != nil {
		t.Fatalf("BuildRedactor: %v", err)
	}
	in := "PASSWORD=hunter2\nOPENAI=sk-abcdefghijklmnopqrstuvwxyz"
	got := r.Redact(in)
	if strings.Contains(got, "hunter2") {
		t.Errorf("PASSWORD leaked: %q", got)
	}
	if strings.Contains(got, "sk-abc") {
		t.Errorf("OpenAI token leaked: %q", got)
	}
}
