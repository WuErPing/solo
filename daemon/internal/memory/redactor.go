// Package memory defines the Redactor contract used by session hooks to
// strip sensitive data from turn content before persistence.
package memory

// Redactor rewrites content so that sensitive tokens, env-file lines, and
// other configured patterns are replaced with audit-friendly placeholders.
//
// Implementations must be safe for concurrent use.
type Redactor interface {
	Redact(content string) string
}

// NoopRedactor returns content unchanged. Useful as a default and in tests.
type NoopRedactor struct{}

// Redact returns content unchanged.
func (NoopRedactor) Redact(content string) string { return content }
