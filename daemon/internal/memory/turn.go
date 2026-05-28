// Package memory records agent/user turns for later replay, retrieval, and
// migration to DB or memory middleware. See docs/product/session-memory-spec.md.
package memory

import (
	"errors"
	"fmt"
	"regexp"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// TurnRole identifies who produced a turn.
type TurnRole string

const (
	RoleUser      TurnRole = "user"
	RoleAssistant TurnRole = "assistant"
	RoleSystem    TurnRole = "system"
)

// IsUser reports whether r is the user role.
func (r TurnRole) IsUser() bool { return r == RoleUser }

// IsAssistant reports whether r is the assistant role.
func (r TurnRole) IsAssistant() bool { return r == RoleAssistant }

// IsSystem reports whether r is the system role.
func (r TurnRole) IsSystem() bool { return r == RoleSystem }

// TurnSource identifies the ingress channel that produced a turn.
type TurnSource string

const (
	SourceCLI   TurnSource = "cli"
	SourceApp   TurnSource = "app"
	SourceRelay TurnSource = "relay"
)

// TokenUsage captures prompt/completion token counts.
type TokenUsage struct {
	Prompt     int `yaml:"prompt"`
	Completion int `yaml:"completion"`
}

// AttachmentRef identifies a non-text attachment on a turn.
type AttachmentRef struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"` // "image" | "file"
	Size int    `yaml:"size"`
}

// TurnMetadata carries optional per-turn sideband data.
type TurnMetadata struct {
	Model        string          `yaml:"model,omitempty"`
	Tokens       *TokenUsage     `yaml:"tokens,omitempty"`
	ToolCalls    []string        `yaml:"toolCalls,omitempty"`
	FinishReason string          `yaml:"finishReason,omitempty"`
	Attachments  []AttachmentRef `yaml:"attachments,omitempty"`
}

// Turn is a single user/assistant/system message in a session.
type Turn struct {
	ID        string        `yaml:"id"`
	SessionID string        `yaml:"sessionId"`
	Seq       uint64        `yaml:"seq"`
	Role      TurnRole      `yaml:"role"`
	Ts        time.Time     `yaml:"ts"`
	Source    TurnSource    `yaml:"source,omitempty"`
	Content   string        `yaml:"-"` // body appended after frontmatter, never in it
	Metadata  *TurnMetadata `yaml:"metadata,omitempty"`
	ParentID  string        `yaml:"parent,omitempty"`
}

// NewTurn builds a Turn with a freshly generated ID. Seq, Metadata, and
// ParentID remain zero-valued and are filled in by the caller.
func NewTurn(sessionID string, role TurnRole, source TurnSource, ts time.Time, content string) Turn {
	return Turn{
		ID:        NewTurnID(),
		SessionID: sessionID,
		Role:      role,
		Ts:        ts,
		Source:    source,
		Content:   content,
	}
}

// Validate checks required fields and enum membership.
func (t Turn) Validate() error {
	if t.ID == "" {
		return errors.New("memory: turn ID is required")
	}
	if t.SessionID == "" {
		return errors.New("memory: turn SessionID is required")
	}
	switch t.Role {
	case RoleUser, RoleAssistant, RoleSystem:
		// ok
	case "":
		return errors.New("memory: turn Role is required")
	default:
		return fmt.Errorf("memory: invalid turn role %q", t.Role)
	}
	if t.Source != "" {
		switch t.Source {
		case SourceCLI, SourceApp, SourceRelay:
			// ok
		default:
			return fmt.Errorf("memory: invalid turn source %q", t.Source)
		}
	}
	return nil
}

// turnIDPattern matches IDs produced by NewTurnID:
//
//	<hex16 microseconds since epoch>-<hex8 monotonic counter>
var turnIDPattern = regexp.MustCompile(
	`^[0-9a-f]{16}-[0-9a-f]{8}$`,
)

// turnIDCounter is an atomic monotonic counter used as the low-entropy
// suffix when multiple IDs are minted within the same microsecond.
// Wraps at 32 bits; a single microsecond never legitimately holds 2^32 IDs.
var turnIDCounter atomic.Uint64

// NewTurnID generates a globally unique, lexicographically monotonic turn ID.
//
// Layout (25 chars, fixed width):
//
//	xxxxxxxxxxxxxxxx-xxxxxxxx
//	│                └─ 8 hex digits: monotonic counter (atomic)
//	└─ 16 hex digits: microseconds since Unix epoch
//
// Lexicographic order == chronological order for IDs produced on a single
// host; cross-host ties are broken by the counter, which is serialized by
// the atomic increment.
func NewTurnID() string {
	ts := uint64(time.Now().UnixMicro())
	seq := turnIDCounter.Add(1) - 1
	// Cap to 8 hex digits; overflow would break fixed-width layout.
	if seq > 0xffffffff {
		panic("memory: turn ID counter overflow")
	}
	return fmt.Sprintf("%016x-%08x", ts, seq)
}

// IsTurnID reports whether s has the format produced by NewTurnID.
func IsTurnID(s string) bool {
	return turnIDPattern.MatchString(s)
}

// FrontmatterYAML serializes the turn's metadata envelope (without Content)
// as YAML bytes, suitable for insertion between `---` fences.
func (t Turn) FrontmatterYAML() ([]byte, error) {
	return yaml.Marshal(t)
}
