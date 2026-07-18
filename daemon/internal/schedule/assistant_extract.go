package schedule

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/WuErPing/solo/protocol"
)

// assistIntent is the typed decode target for the LLM's JSON output
// (see docs/product/chat-schedule-assistant-design.md Appendix A).
type assistIntent struct {
	Kind       string                    `json:"kind"` // "proposal" | "clarify" | "answer"
	Op         string                    `json:"op"`   // "create" | "update" | "pause" | "resume" | "delete"
	ScheduleID string                    `json:"scheduleId"`
	Name       string                    `json:"name"`
	Prompt     string                    `json:"prompt"`
	Cadence    *protocol.ScheduleCadence `json:"cadence"`
	Target     *protocol.ScheduleTarget  `json:"target"`
	Cwd        string                    `json:"cwd"`
	MaxRuns    *int                      `json:"maxRuns"`
	ExpiresAt  string                    `json:"expiresAt"`
	Summary    string                    `json:"summary"`
	Warnings   []string                  `json:"warnings"`
	Message    string                    `json:"message"`
}

// assistRefError reports a schedule/agent reference that does not resolve to
// exactly one known entry. The Assistant turns it into a clarify payload
// (no retry) listing up to 5 candidates.
type assistRefError struct {
	what       string // "schedule" | "agent"
	ref        string
	ambiguous  bool
	candidates []string
}

func (e *assistRefError) Error() string {
	quoted := make([]string, 0, len(e.candidates))
	for _, c := range e.candidates {
		quoted = append(quoted, strconv.Quote(c))
	}
	switch {
	case e.ambiguous:
		return fmt.Sprintf("%s %q is ambiguous — did you mean: %s?", e.what, e.ref, strings.Join(quoted, ", "))
	case len(e.candidates) > 0:
		return fmt.Sprintf("%s %q not found — did you mean: %s?", e.what, e.ref, strings.Join(quoted, ", "))
	default:
		return fmt.Sprintf("%s %q not found", e.what, e.ref)
	}
}

// parseAssistIntent locates the JSON in raw LLM output, decodes it, and runs
// schema + semantic validation. Semantic validation may rewrite references
// (schedule/agent names) to ids. Returned errors are either *assistRefError
// (clarify, no retry) or plain validation errors (retryable).
func parseAssistIntent(raw string, store *Store, agents []AgentInfo, now time.Time) (*assistIntent, error) {
	span, err := extractAssistJSONSpan(raw)
	if err != nil {
		return nil, err
	}
	var intent assistIntent
	if err := json.Unmarshal([]byte(span), &intent); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := validateAssistSchema(&intent); err != nil {
		return nil, err
	}
	if err := validateAssistSemantics(&intent, store, agents, now); err != nil {
		return nil, err
	}
	return &intent, nil
}

// extractAssistJSONSpan prefers a ```json fenced block; otherwise it returns
// the outermost balanced {…} span (tolerating prose around it).
func extractAssistJSONSpan(raw string) (string, error) {
	const fence = "```json"
	if i := strings.Index(raw, fence); i >= 0 {
		rest := raw[i+len(fence):]
		if j := strings.Index(rest, "```"); j >= 0 {
			return strings.TrimSpace(rest[:j]), nil
		}
		return "", fmt.Errorf("truncated JSON: unterminated json fence")
	}

	start := strings.Index(raw, "{")
	if start < 0 {
		return "", fmt.Errorf("no JSON object found in LLM output")
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("truncated JSON object in LLM output")
}

// validateAssistSchema checks required fields per kind/op.
func validateAssistSchema(intent *assistIntent) error {
	switch intent.Kind {
	case "clarify", "answer":
		if strings.TrimSpace(intent.Message) == "" {
			return fmt.Errorf("kind %q requires a message", intent.Kind)
		}
		return nil
	case "proposal":
	default:
		return fmt.Errorf("unknown kind %q", intent.Kind)
	}

	if intent.Summary == "" {
		return fmt.Errorf("proposal requires summary")
	}
	switch intent.Op {
	case "create":
		if intent.Prompt == "" {
			return fmt.Errorf("create requires prompt")
		}
		if intent.Cadence == nil {
			return fmt.Errorf("create requires cadence")
		}
		if intent.Target == nil {
			return fmt.Errorf("create requires target")
		}
	case "update":
		if intent.ScheduleID == "" {
			return fmt.Errorf("update requires scheduleId")
		}
		if intent.Name == "" && intent.Prompt == "" && intent.Cadence == nil &&
			intent.Target == nil && intent.Cwd == "" && intent.MaxRuns == nil && intent.ExpiresAt == "" {
			return fmt.Errorf("update requires at least one changed field")
		}
	case "pause", "resume", "delete":
		if intent.ScheduleID == "" {
			return fmt.Errorf("%s requires scheduleId", intent.Op)
		}
	default:
		return fmt.Errorf("unknown op %q", intent.Op)
	}
	return nil
}

// validateAssistSemantics checks the intent against live store/agent state.
func validateAssistSemantics(intent *assistIntent, store *Store, agents []AgentInfo, now time.Time) error {
	if intent.Kind != "proposal" {
		return nil
	}

	if intent.Prompt != "" && utf8.RuneCountInString(intent.Prompt) > 4000 {
		return fmt.Errorf("prompt exceeds 4000 characters")
	}
	if intent.Cadence != nil {
		if err := validateAssistCadence(*intent.Cadence, now); err != nil {
			return err
		}
	}
	if intent.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, intent.ExpiresAt)
		if err != nil {
			return fmt.Errorf("expiresAt must be RFC3339: %v", err)
		}
		if !t.After(now) {
			return fmt.Errorf("expiresAt must be in the future")
		}
	}
	if intent.MaxRuns != nil && *intent.MaxRuns <= 0 {
		return fmt.Errorf("maxRuns must be > 0")
	}
	if intent.Target != nil {
		if err := validateScheduleTarget(*intent.Target); err != nil {
			return fmt.Errorf("invalid target: %w", err)
		}
		if intent.Target.Type == "agent" {
			id, err := resolveAssistAgentRef(intent.Target.AgentID, agents)
			if err != nil {
				return err
			}
			intent.Target.AgentID = id
		}
	}
	switch intent.Op {
	case "update", "pause", "resume", "delete":
		id, err := resolveAssistScheduleRef(intent.ScheduleID, store)
		if err != nil {
			return err
		}
		intent.ScheduleID = id
	}
	return nil
}

func validateAssistCadence(c protocol.ScheduleCadence, now time.Time) error {
	switch c.Type {
	case "cron":
		if c.Expression == "" {
			return fmt.Errorf("cron cadence requires expression")
		}
		if NextRunAt(c, now) == nil {
			return fmt.Errorf("invalid cron expression %q", c.Expression)
		}
	case "every":
		if c.EveryMs < 60000 {
			return fmt.Errorf("every cadence must be at least 60000ms")
		}
	default:
		return fmt.Errorf("unknown cadence type %q", c.Type)
	}
	return nil
}

// resolveAssistScheduleRef resolves a schedule id, or a unique
// case-insensitive name match, to the schedule id.
func resolveAssistScheduleRef(ref string, store *Store) (string, error) {
	if _, ok := store.Get(ref); ok {
		return ref, nil
	}
	summaries := store.List()
	var exact []protocol.ScheduleSummary
	for _, s := range summaries {
		if s.Name != nil && strings.EqualFold(*s.Name, ref) {
			exact = append(exact, s)
		}
	}
	sort.Slice(exact, func(i, j int) bool { return exact[i].ID < exact[j].ID })
	switch {
	case len(exact) == 1:
		return exact[0].ID, nil
	case len(exact) > 1:
		return "", &assistRefError{what: "schedule", ref: ref, ambiguous: true, candidates: assistScheduleNames(exact, 5)}
	}
	return "", &assistRefError{what: "schedule", ref: ref, candidates: fuzzyAssistScheduleCandidates(summaries, ref, 5)}
}

func fuzzyAssistScheduleCandidates(summaries []protocol.ScheduleSummary, ref string, limit int) []string {
	foldRef := strings.ToLower(ref)
	var out []string
	for _, s := range summaries {
		if s.Name == nil {
			continue
		}
		foldName := strings.ToLower(*s.Name)
		if strings.Contains(foldName, foldRef) || strings.Contains(foldRef, foldName) {
			out = append(out, *s.Name)
		}
	}
	sort.Strings(out)
	out = dedupeAssistStrings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func assistScheduleNames(summaries []protocol.ScheduleSummary, limit int) []string {
	out := make([]string, 0, len(summaries))
	for _, s := range summaries {
		if s.Name != nil {
			out = append(out, *s.Name)
		}
	}
	sort.Strings(out)
	out = dedupeAssistStrings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// resolveAssistAgentRef resolves an agent id, or a unique case-insensitive
// title match, to the agent id.
func resolveAssistAgentRef(ref string, agents []AgentInfo) (string, error) {
	for _, a := range agents {
		if a.ID == ref {
			return ref, nil
		}
	}
	var exact []AgentInfo
	for _, a := range agents {
		if a.Title != "" && strings.EqualFold(a.Title, ref) {
			exact = append(exact, a)
		}
	}
	sort.Slice(exact, func(i, j int) bool { return exact[i].ID < exact[j].ID })
	switch {
	case len(exact) == 1:
		return exact[0].ID, nil
	case len(exact) > 1:
		return "", &assistRefError{what: "agent", ref: ref, ambiguous: true, candidates: assistAgentTitles(exact, 5)}
	}

	foldRef := strings.ToLower(ref)
	var fuzzy []AgentInfo
	for _, a := range agents {
		foldTitle := strings.ToLower(a.Title)
		if a.Title != "" && (strings.Contains(foldTitle, foldRef) || strings.Contains(foldRef, foldTitle)) {
			fuzzy = append(fuzzy, a)
		}
	}
	return "", &assistRefError{what: "agent", ref: ref, candidates: assistAgentTitles(fuzzy, 5)}
}

func assistAgentTitles(agents []AgentInfo, limit int) []string {
	out := make([]string, 0, len(agents))
	for _, a := range agents {
		title := a.Title
		if title == "" {
			title = a.ID
		}
		out = append(out, title)
	}
	sort.Strings(out)
	out = dedupeAssistStrings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func dedupeAssistStrings(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
