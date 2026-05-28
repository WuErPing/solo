package redact

import (
	"strings"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// ---------- Interface compliance ----------

func TestEnvFileRedactor_ImplementsRedactor(t *testing.T) {
	r, err := NewEnvFileRedactor([]string{"PASSWORD"})
	if err != nil {
		t.Fatalf("NewEnvFileRedactor: %v", err)
	}
	var _ memory.Redactor = r
}

// ---------- Constructor ----------

func TestNewEnvFileRedactor_RejectsEmptySensitiveList(t *testing.T) {
	if _, err := NewEnvFileRedactor(nil); err == nil {
		t.Error("expected error for nil list")
	}
	if _, err := NewEnvFileRedactor([]string{}); err == nil {
		t.Error("expected error for empty list")
	}
}

func TestNewEnvFileRedactor_RejectsEmptyKey(t *testing.T) {
	if _, err := NewEnvFileRedactor([]string{"PASSWORD", ""}); err == nil {
		t.Error("expected error for empty entry in sensitive list")
	}
}

func TestDefaultSensitiveKeys_NonEmpty(t *testing.T) {
	if len(DefaultSensitiveKeys()) == 0 {
		t.Error("DefaultSensitiveKeys() must not be empty")
	}
}

// ---------- Behavior ----------

func TestEnvFileRedactor_RedactsMatchingKey(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"PASSWORD", "SECRET"})
	in := "host=localhost\nPASSWORD=hunter2\nuser=alice"
	got := r.Redact(in)

	if strings.Contains(got, "hunter2") {
		t.Errorf("PASSWORD value leaked: %q", got)
	}
	if !strings.Contains(got, "[redacted:PASSWORD]") {
		t.Errorf("placeholder missing: %q", got)
	}
	if !strings.Contains(got, "host=localhost") {
		t.Errorf("non-sensitive line damaged: %q", got)
	}
	if !strings.Contains(got, "user=alice") {
		t.Errorf("non-sensitive line damaged: %q", got)
	}
}

func TestEnvFileRedactor_LeavesNonEnvLinesAlone(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"SECRET"})
	in := "regular log line\nno equals here\nanother line"
	if got := r.Redact(in); got != in {
		t.Errorf("non-env content modified:\ngot:  %q\nwant: %q", got, in)
	}
}

func TestEnvFileRedactor_CaseInsensitive(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"password"})
	in := "PASSWORD=secret123"
	got := r.Redact(in)
	if strings.Contains(got, "secret123") {
		t.Errorf("case-insensitive match failed: %q", got)
	}
}

func TestEnvFileRedactor_PreservesKeyNameInPlaceholder(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"DATABASE_URL"})
	in := "DATABASE_URL=postgres://user:pass@host/db"
	got := r.Redact(in)
	if !strings.Contains(got, "[redacted:DATABASE_URL]") {
		t.Errorf("expected key name in placeholder, got %q", got)
	}
}

func TestEnvFileRedactor_HandlesQuotedValues(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"PASSWORD"})
	in := `PASSWORD="my secret"`
	got := r.Redact(in)
	if strings.Contains(got, "my secret") {
		t.Errorf("quoted value leaked: %q", got)
	}
}

func TestEnvFileRedactor_HandlesWhitespaceAroundEquals(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"PASSWORD"})
	in := "PASSWORD = hunter2"
	got := r.Redact(in)
	if strings.Contains(got, "hunter2") {
		t.Errorf("value with whitespace not redacted: %q", got)
	}
}

func TestEnvFileRedactor_HandlesEmptyValue(t *testing.T) {
	r, _ := NewEnvFileRedactor([]string{"PASSWORD"})
	in := "PASSWORD=\nOTHER=value"
	got := r.Redact(in)
	if !strings.Contains(got, "[redacted:PASSWORD]") {
		t.Errorf("empty-value line not redacted: %q", got)
	}
	if !strings.Contains(got, "OTHER=value") {
		t.Errorf("unrelated line damaged: %q", got)
	}
}
