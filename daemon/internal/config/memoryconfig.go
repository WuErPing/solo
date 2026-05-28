package config

import (
	"errors"

	"github.com/WuErPing/solo/daemon/internal/memory/redact"
)

// MemoryConfig controls the session-memory persistence feature.
//
// Enabled by default: a zero-value MemoryConfig runs the feature. Set
// Enabled=false explicitly (in config.json: `"memory": {"enabled": false}`)
// to opt out. Pointer-typed so the loader can distinguish "not configured"
// from "explicitly off".
type MemoryConfig struct {
	// Enabled turns the feature on. nil == enabled (default);
	// explicit false == disabled; explicit true == enabled.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Backend selects the recorder implementation. Supported: "file"
	// (phase 1), "sqlite", "middleware". Default: "file".
	Backend string `yaml:"backend" json:"backend"`
	// RetentionDays is a hint to the backend for pruning older turns.
	// Default: 90.
	RetentionDays int `yaml:"retention_days" json:"retention_days"`
	// QueueSize is the capacity of the recorder's internal turn channel.
	// Default: 1024.
	QueueSize int `yaml:"queue_size" json:"queue_size"`
	// Overflow governs behavior when the queue is full: "block" (default)
	// or "error".
	Overflow string `yaml:"overflow" json:"overflow"`
	// Root is the directory name (relative to SoloHome) under which
	// sessions are written. Default: "memory" — i.e. ~/.solo/memory.
	Root string `yaml:"root" json:"root"`
	// Redact configures the content redactor applied before a turn is
	// persisted.
	Redact redact.RedactorConfig `yaml:"redact" json:"redact"`
	// SoloHome is the daemon's solo home directory. Populated at runtime
	// from the top-level config.SoloHome (not exposed in YAML/JSON).
	SoloHome string `yaml:"-" json:"-"`
}

// IsEnabled reports whether the memory feature should run. Nil or true →
// enabled; explicit false → disabled.
func (c MemoryConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// SetEnabled is a helper that stores v in a fresh bool and points
// Enabled at it. Convenient in tests and one-off construction.
func (c *MemoryConfig) SetEnabled(v bool) {
	c.Enabled = &v
}

// ApplyDefaults fills zero-valued fields with sensible defaults. It is
// idempotent and safe to call multiple times.
func (c *MemoryConfig) ApplyDefaults() {
	if c.Backend == "" {
		c.Backend = "file"
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = 90
	}
	if c.QueueSize <= 0 {
		c.QueueSize = 1024
	}
	if c.Overflow == "" {
		c.Overflow = "block"
	}
	if c.Root == "" {
		c.Root = "memory"
	}
	// AutoGitignore has no sentinel zero value; leave the bool as-is.
	// Users wanting it on must set it explicitly.
}

// Validate reports misconfigurations. ApplyDefaults should be called first.
func (c MemoryConfig) Validate() error {
	switch c.Backend {
	case "file", "sqlite", "middleware":
		// ok
	default:
		return errors.New("memory: unknown backend " + c.Backend)
	}
	switch c.Overflow {
	case "block", "error":
		// ok
	default:
		return errors.New("memory: unknown overflow policy " + c.Overflow)
	}
	if c.QueueSize <= 0 {
		return errors.New("memory: queue_size must be > 0")
	}
	if c.RetentionDays <= 0 {
		return errors.New("memory: retention_days must be > 0")
	}
	return nil
}
