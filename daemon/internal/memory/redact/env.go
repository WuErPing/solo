package redact

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// defaultSensitiveKeys is the default set of key substrings (case-insensitive)
// that EnvFileRedactor treats as sensitive. Keys are stored in upper-case.
var defaultSensitiveKeys = []string{
	"PASSWORD",
	"SECRET",
	"TOKEN",
	"API_KEY",
	"APIKEY",
	"PRIVATE_KEY",
	"DATABASE_URL",
	"DSN",
	"AUTH",
	"CREDENTIAL",
	"ACCESS_KEY",
	"ENCRYPTION_KEY",
}

// DefaultSensitiveKeys returns a copy of the built-in sensitive key list.
func DefaultSensitiveKeys() []string {
	out := make([]string, len(defaultSensitiveKeys))
	copy(out, defaultSensitiveKeys)
	return out
}

// envLinePattern matches a KEY = VALUE line with optional surrounding whitespace
// and an optional quoted value. The KEY must start with a letter or underscore
// and consist of alphanumerics / underscores.
var envLinePattern = regexp.MustCompile(`(?m)^([ \t]*)([A-Za-z_][A-Za-z0-9_]*)([ \t]*=[ \t]*)(.*?)([ \t]*)$`)

// EnvFileRedactor redacts "KEY=value" lines whose KEY (case-insensitive)
// contains any of the configured sensitive substrings. The entire value is
// replaced with "[redacted:<KEY>]".
type EnvFileRedactor struct {
	keys map[string]bool // upper-cased substrings
}

// Compile-time interface check.
var _ memory.Redactor = (*EnvFileRedactor)(nil)

// NewEnvFileRedactor constructs an EnvFileRedactor from a list of sensitive
// key substrings. Returns an error if the list is empty or contains empty
// entries.
func NewEnvFileRedactor(sensitiveKeys []string) (*EnvFileRedactor, error) {
	if len(sensitiveKeys) == 0 {
		return nil, fmt.Errorf("redact: EnvFileRedactor requires non-empty sensitive keys")
	}
	keys := make(map[string]bool, len(sensitiveKeys))
	for _, k := range sensitiveKeys {
		if k == "" {
			return nil, fmt.Errorf("redact: EnvFileRedactor sensitive key entry is empty")
		}
		keys[strings.ToUpper(k)] = true
	}
	return &EnvFileRedactor{keys: keys}, nil
}

// Redact rewrites KEY=value lines whose KEY matches any sensitive substring.
// Non-env lines are returned unchanged.
func (e *EnvFileRedactor) Redact(content string) string {
	return envLinePattern.ReplaceAllStringFunc(content, func(line string) string {
		m := envLinePattern.FindStringSubmatch(line)
		if m == nil {
			return line
		}
		leading, key, sep, value := m[1], m[2], m[3], m[4]
		upper := strings.ToUpper(key)
		if !e.isSensitive(upper) {
			return line
		}
		// Preserve leading whitespace and the "KEY = " separator; replace
		// only the value with a placeholder that names the key.
		_ = value
		return leading + key + sep + "[redacted:" + key + "]"
	})
}

// isSensitive reports whether upperKey contains any sensitive substring.
func (e *EnvFileRedactor) isSensitive(upperKey string) bool {
	for k := range e.keys {
		if strings.Contains(upperKey, k) {
			return true
		}
	}
	return false
}
