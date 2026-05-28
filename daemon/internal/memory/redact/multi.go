package redact

import "github.com/WuErPing/solo/daemon/internal/memory"

// Multi chains several Redactors and applies them in order. Nil entries are
// ignored. Safe for concurrent use if each underlying Redactor is.
type Multi struct {
	chain []memory.Redactor
}

// Compile-time interface check.
var _ memory.Redactor = (*Multi)(nil)

// NewMulti builds a Multi from the given redactors, dropping nil entries.
func NewMulti(rs ...memory.Redactor) *Multi {
	chain := make([]memory.Redactor, 0, len(rs))
	for _, r := range rs {
		if r != nil {
			chain = append(chain, r)
		}
	}
	return &Multi{chain: chain}
}

// Redact feeds content through each chained Redactor in order.
func (m *Multi) Redact(content string) string {
	for _, r := range m.chain {
		content = r.Redact(content)
	}
	return content
}
