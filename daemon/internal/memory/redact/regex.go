// Package redact provides content redactors that implement memory.Redactor.
package redact

import (
	"fmt"
	"regexp"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// RegexRedactor replaces matches of a compiled regex with a fixed
// placeholder of the form "[redacted:<name>]".
type RegexRedactor struct {
	name    string
	pattern *regexp.Regexp
}

// Compile-time interface check.
var _ memory.Redactor = (*RegexRedactor)(nil)

// NewRegexRedactor builds a RegexRedactor. Returns an error if name is empty,
// pattern is empty, or pattern fails to compile.
func NewRegexRedactor(name, pattern string) (*RegexRedactor, error) {
	if name == "" {
		return nil, fmt.Errorf("redact: RegexRedactor name is required")
	}
	if pattern == "" {
		return nil, fmt.Errorf("redact: RegexRedactor pattern is required")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("redact: invalid regex %q: %w", pattern, err)
	}
	return &RegexRedactor{name: name, pattern: re}, nil
}

// Redact replaces every match of the configured pattern with
// "[redacted:<name>]".
func (r *RegexRedactor) Redact(content string) string {
	return r.pattern.ReplaceAllString(content, "[redacted:"+r.name+"]")
}

// NewDefaultRegexRedactors returns the default set of regex redactors
// covering common provider token formats.
//
// Currently included:
//   - "openai":     sk-<20+ alnum>
//   - "github":     ghp_<36 alnum>
//   - "anthropic":  sk-ant-api03-<20+ alnum/_/->
//   - "aws":        AKIA<16 alnum upper>
func NewDefaultRegexRedactors() []*RegexRedactor {
	// Errors here would indicate a programmer typo in a hard-coded
	// pattern; they must never happen in a tested binary.
	pairs := []struct {
		name, pattern string
	}{
		{"openai", `sk-[A-Za-z0-9]{20,}`},
		{"github", `ghp_[A-Za-z0-9]{36}`},
		{"anthropic", `sk-ant-api03-[A-Za-z0-9_-]{20,}`},
		{"aws", `AKIA[0-9A-Z]{16}`},
	}
	out := make([]*RegexRedactor, 0, len(pairs))
	for _, p := range pairs {
		r, err := NewRegexRedactor(p.name, p.pattern)
		if err != nil {
			panic(err)
		}
		out = append(out, r)
	}
	return out
}
