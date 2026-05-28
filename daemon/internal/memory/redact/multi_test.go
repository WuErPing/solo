package redact

import (
	"strings"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// ---------- Interface compliance ----------

func TestMultiRedactor_ImplementsRedactor(t *testing.T) {
	t.Helper()
	var _ memory.Redactor = NewMulti()
}

// ---------- Empty / single ----------

func TestMultiRedactor_Empty_ReturnsInputUnchanged(t *testing.T) {
	m := NewMulti()
	in := "hello world\nPASSWORD=secret"
	if got := m.Redact(in); got != in {
		t.Errorf("empty Multi modified input:\ngot:  %q\nwant: %q", got, in)
	}
}

func TestMultiRedactor_Nil_ReturnsInputUnchanged(t *testing.T) {
	m := NewMulti(nil, nil)
	in := "any text"
	if got := m.Redact(in); got != in {
		t.Errorf("nil-only Multi modified input: %q", got)
	}
}

// ---------- Composition ----------

func TestMultiRedactor_AppliesInOrder(t *testing.T) {
	regex, _ := NewRegexRedactor("openai", `sk-[A-Za-z0-9]{20,}`)
	env, _ := NewEnvFileRedactor([]string{"PASSWORD"})

	m := NewMulti(regex, env)

	in := "key=sk-abcdefghijklmnopqrstuvwxyz\nPASSWORD=hunter2"
	got := m.Redact(in)

	if strings.Contains(got, "sk-abc") {
		t.Errorf("regex redactor did not run: %q", got)
	}
	if strings.Contains(got, "hunter2") {
		t.Errorf("env redactor did not run: %q", got)
	}
}

func TestMultiRedactor_LaterSeesEarlierOutput(t *testing.T) {
	// First redactor tags "X" as "[A]"; second must see "[A]" (not "X").
	first := &literalReplacer{from: "X", to: "[A]"}
	second := &literalReplacer{from: "[A]", to: "[B]"}

	m := NewMulti(first, second)
	got := m.Redact("payload X end")
	if !strings.Contains(got, "[B]") {
		t.Errorf("second redactor did not see first's output: %q", got)
	}
	if strings.Contains(got, "[A]") {
		t.Errorf("first's output was not further transformed: %q", got)
	}
	if !strings.Contains(got, "payload") || !strings.Contains(got, "end") {
		t.Errorf("adjacent text damaged: %q", got)
	}
}

func TestMultiRedactor_SkipsNilEntries(t *testing.T) {
	regex, _ := NewRegexRedactor("tok", `sk-[A-Za-z0-9]{20,}`)
	m := NewMulti(nil, regex, nil)
	in := "key=sk-abcdefghijklmnopqrst"
	got := m.Redact(in)
	if strings.Contains(got, "sk-abcde") {
		t.Errorf("nil entries broke the chain: %q", got)
	}
}

// literalReplacer is a tiny memory.Redactor used to pin down Multi semantics.
type literalReplacer struct {
	from, to string
}

func (l *literalReplacer) Redact(s string) string {
	return strings.ReplaceAll(s, l.from, l.to)
}
