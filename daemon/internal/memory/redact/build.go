package redact

import (
	"fmt"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// RedactorConfig selects which redactors are composed by BuildRedactor.
//
// The same struct is embedded verbatim in config.MemoryConfig so that
// YAML/JSON/TOML decoders populate it directly.
type RedactorConfig struct {
	// EnvFiles enables the EnvFileRedactor with either the default
	// sensitive-key list or SensitiveKeys when non-empty.
	EnvFiles bool `yaml:"env_files" json:"env_files"`
	// APIKeys enables the default regex redactors covering common
	// provider token formats (OpenAI, GitHub, Anthropic, AWS).
	APIKeys bool `yaml:"api_keys" json:"api_keys"`
	// CustomRegexes are user-supplied patterns compiled as individual
	// RegexRedactors named "custom_<i>".
	CustomRegexes []string `yaml:"custom_regexes" json:"custom_regexes"`
	// SensitiveKeys overrides the default EnvFileRedactor sensitive-key
	// list when non-empty.
	SensitiveKeys []string `yaml:"sensitive_keys" json:"sensitive_keys"`
}

// BuildRedactor constructs the composite Redactor described by cfg.
//
//   - Empty config → memory.NoopRedactor (no redaction).
//   - Invalid regex → error (fail-fast so a typo surfaces at startup).
//   - Mixed config → Multi chaining every enabled redactor in order:
//     regex defaults, custom regexes, env-file redactor.
func BuildRedactor(cfg RedactorConfig) (memory.Redactor, error) {
	var chain []memory.Redactor

	if cfg.APIKeys {
		for _, r := range NewDefaultRegexRedactors() {
			chain = append(chain, r)
		}
	}

	for i, pattern := range cfg.CustomRegexes {
		name := fmt.Sprintf("custom_%d", i)
		r, err := NewRegexRedactor(name, pattern)
		if err != nil {
			return nil, fmt.Errorf("redact: custom regex %d: %w", i, err)
		}
		chain = append(chain, r)
	}

	if cfg.EnvFiles {
		keys := cfg.SensitiveKeys
		if len(keys) == 0 {
			keys = DefaultSensitiveKeys()
		}
		envR, err := NewEnvFileRedactor(keys)
		if err != nil {
			return nil, fmt.Errorf("redact: env-file redactor: %w", err)
		}
		chain = append(chain, envR)
	}

	if len(chain) == 0 {
		return memory.NoopRedactor{}, nil
	}
	return NewMulti(chain...), nil
}
