package memory

import "testing"

func TestNoopRedactor_ReturnsUnchanged(t *testing.T) {
	r := NoopRedactor{}
	in := "hello world\nline 2"
	if got := r.Redact(in); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestNoopRedactor_ImplementsRedactor(t *testing.T) {
	t.Helper()
	var _ Redactor = NoopRedactor{}
}
