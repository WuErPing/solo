package memorysetup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

	// Record a turn carrying an API-key-shaped secret through the wired
	// Bridge, then verify the content persisted on disk was redacted.
	// Deep redactor unit tests live in the redact package; this proves
	// Build actually composes the configured redactors into the Bridge.
	if f.Bridge == nil {
		t.Fatal("Bridge is nil")
	}
	const secret = "sk-abcdefghij1234567890ABCD" // matches the default openai pattern
	f.Bridge.OnUserTurn("sess-redact", "agent-1", "my token is "+secret)
	if err := f.Recorder.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var files []string
	root := filepath.Join(cfg.SoloHome, "memory")
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".md" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) == 0 {
		t.Fatalf("no turn files persisted under %s", root)
	}
	redacted := false
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(data), secret) {
			t.Errorf("turn file %s contains unredacted secret", path)
		}
		if strings.Contains(string(data), "[redacted:openai]") {
			redacted = true
		}
	}
	if !redacted {
		t.Error("no persisted turn shows the redaction placeholder")
	}
}
