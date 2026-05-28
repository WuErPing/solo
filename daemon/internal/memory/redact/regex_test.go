package redact

import (
	"strings"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// ---------- Interface compliance ----------

func TestRegexRedactor_ImplementsRedactor(t *testing.T) {
	r, err := NewRegexRedactor("api_key", `sk-[A-Za-z0-9]{20,}`)
	if err != nil {
		t.Fatalf("NewRegexRedactor: %v", err)
	}
	var _ memory.Redactor = r
}

// ---------- Constructor ----------

func TestNewRegexRedactor_RejectsEmptyName(t *testing.T) {
	if _, err := NewRegexRedactor("", "x"); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestNewRegexRedactor_RejectsEmptyPattern(t *testing.T) {
	if _, err := NewRegexRedactor("x", ""); err == nil {
		t.Error("expected error for empty pattern")
	}
}

func TestNewRegexRedactor_RejectsInvalidRegex(t *testing.T) {
	if _, err := NewRegexRedactor("x", "(unclosed"); err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestNewRegexRedactor_CaseInsensitiveFlag(t *testing.T) {
	r, err := NewRegexRedactor("tok", `(?i)secret-[a-z]+`)
	if err != nil {
		t.Fatalf("NewRegexRedactor: %v", err)
	}
	got := r.Redact("found SECRET-abc in logs")
	if strings.Contains(got, "SECRET-abc") {
		t.Errorf("case-insensitive flag not honored: %q", got)
	}
	if !strings.Contains(got, "[redacted:tok]") {
		t.Errorf("expected [redacted:tok], got %q", got)
	}
}

// ---------- Behavior ----------

func TestRegexRedactor_ReplacesMatches(t *testing.T) {
	r, _ := NewRegexRedactor("openai", `sk-[A-Za-z0-9]{20,}`)
	got := r.Redact("key=sk-abc123def456ghi789jkl012 rest")
	if strings.Contains(got, "sk-abc123") {
		t.Errorf("match not redacted: %q", got)
	}
	if !strings.Contains(got, "[redacted:openai]") {
		t.Errorf("placeholder missing: %q", got)
	}
	if !strings.Contains(got, "rest") {
		t.Errorf("surrounding text damaged: %q", got)
	}
}

func TestRegexRedactor_NoMatch_NoChange(t *testing.T) {
	r, _ := NewRegexRedactor("tok", `sk-[A-Za-z0-9]{20,}`)
	in := "no tokens here\njust normal text"
	if got := r.Redact(in); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestRegexRedactor_MultipleMatches_AllReplaced(t *testing.T) {
	r, _ := NewRegexRedactor("openai", `sk-[A-Za-z0-9]{20,}`)
	in := "a=sk-abcdefghijklmnopqrstuvwxyz b=sk-zyxwvutsrqponmlkjihgfedcba"
	got := r.Redact(in)
	if strings.Contains(got, "sk-") {
		t.Errorf("some tokens survived: %q", got)
	}
	if strings.Count(got, "[redacted:openai]") != 2 {
		t.Errorf("expected 2 placeholders, got %q", got)
	}
}

func TestRegexRedactor_MultilineContent(t *testing.T) {
	r, _ := NewRegexRedactor("gh", `ghp_[A-Za-z0-9]{36}`)
	in := "line1\npat=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij\nline3"
	got := r.Redact(in)
	if strings.Contains(got, "ghp_ABCDEFG") {
		t.Errorf("token not redacted across newlines: %q", got)
	}
	if !strings.Contains(got, "line1") || !strings.Contains(got, "line3") {
		t.Errorf("adjacent lines damaged: %q", got)
	}
}

// ---------- Default set ----------

func TestNewDefaultRegexRedactors_CoversCommonTokens(t *testing.T) {
	redactors := NewDefaultRegexRedactors()
	if len(redactors) == 0 {
		t.Fatal("default set empty")
	}

	chain := make([]memory.Redactor, 0, len(redactors))
	for _, r := range redactors {
		chain = append(chain, r)
	}
	multi := NewMulti(chain...)

	cases := []struct {
		label   string
		input   string
		notWant string
	}{
		{"OpenAI", "key=sk-abcdefghijklmnopqrstuvwxyz", "sk-abcde"},
		{"GitHub PAT", "tok=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij", "ghp_ABCDE"},
		{"Anthropic", "key=sk-ant-api03-abcdefghijklmnopqrstuvwxyz", "sk-ant-"},
		{"AWS access key", "AWS_ACCESS=AKIAIOSFODNN7EXAMPLE rest", "AKIAIOSFODNN7EXAMPLE"},
	}
	for _, c := range cases {
		got := multi.Redact(c.input)
		if strings.Contains(got, c.notWant) {
			t.Errorf("%s: %q survived in %q", c.label, c.notWant, got)
		}
	}
}
