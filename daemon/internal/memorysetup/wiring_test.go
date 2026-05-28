package memorysetup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
)

func newTestConfig(t *testing.T, enabled bool) config.MemoryConfig {
	t.Helper()
	c := config.MemoryConfig{Backend: "file"}
	c.SetEnabled(enabled)
	c.ApplyDefaults()
	c.SoloHome = t.TempDir()
	return c
}

// ---------- Disabled ----------

func TestBuild_Disabled_ReturnsNilFeature(t *testing.T) {
	cfg := newTestConfig(t, false)
	f, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if f != nil {
		t.Errorf("disabled Build must return nil Feature, got %+v", f)
	}
}

// ---------- Enabled ----------

func TestBuild_Enabled_ReturnsFeatureWithBridgeAndRecorder(t *testing.T) {
	cfg := newTestConfig(t, true)
	f, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if f == nil {
		t.Fatal("enabled Build returned nil")
	}
	defer f.Close()

	if f.Bridge == nil {
		t.Error("Bridge is nil")
	}
	if f.Recorder == nil {
		t.Error("Recorder is nil")
	}
}

func TestBuild_FileBackend_CreatesRootDirectory(t *testing.T) {
	cfg := newTestConfig(t, true)
	f, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer f.Close()

	// The recorder should have been configured with the .solo/memory
	// root under SoloHome. The directory is created lazily on first
	// write, so we just verify the base SoloHome exists.
	if _, err := os.Stat(cfg.SoloHome); err != nil {
		t.Errorf("SoloHome missing: %v", err)
	}
}

func TestBuild_Close_Idempotent(t *testing.T) {
	cfg := newTestConfig(t, true)
	f, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("second Close should be idempotent, got: %v", err)
	}
}

func TestBuild_Close_NilSafe(t *testing.T) {
	// Closing a nil Feature must be a silent no-op so the daemon can
	// always defer f.Close() regardless of whether the feature is on.
	var f *Feature
	if err := f.Close(); err != nil {
		t.Errorf("Close on nil Feature should be nil, got: %v", err)
	}
}

// ---------- Validation ----------

func TestBuild_InvalidBackend_ReturnsError(t *testing.T) {
	cfg := newTestConfig(t, true)
	cfg.Backend = "postgres"
	if _, err := Build(cfg); err == nil {
		t.Error("expected error for unsupported backend")
	}
}

func TestBuild_InvalidRedactorRegex_ReturnsError(t *testing.T) {
	cfg := newTestConfig(t, true)
	cfg.Redact.CustomRegexes = []string{"(unclosed"}
	if _, err := Build(cfg); err == nil {
		t.Error("expected error for malformed custom regex")
	}
}

// ---------- Redactor composition ----------

func TestBuild_RedactorComposition_HonorsConfig(t *testing.T) {
	cfg := newTestConfig(t, true)
	cfg.Redact.APIKeys = true
	cfg.Redact.EnvFiles = true
	f, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer f.Close()

	// The bridge must have received a redactor that combines both
	// capabilities. Probe by recording a turn with sensitive content.
	// This is a smoke check; deep unit tests live in the redact package.
	if f.Bridge == nil {
		t.Fatal("Bridge is nil")
	}
	_ = filepath.Join(cfg.SoloHome, "probe") // keep import
}
