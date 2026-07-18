package schedule

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WuErPing/solo/protocol"
)

const (
	// maxAssistContextAgents / maxAssistContextSchedules cap how many entries
	// the context block lists; overflow is noted as "…and N more".
	maxAssistContextAgents    = 50
	maxAssistContextSchedules = 50
	// maxAssistTranscriptTurns caps the client-supplied transcript (oldest dropped).
	maxAssistTranscriptTurns = 10
	// maxAssistPromptContextChars caps the context block + transcript section.
	// Oldest transcript turns are dropped first; the context block is hard
	// truncated only as a last resort.
	maxAssistPromptContextChars = 8000
)

// assistPromptContext carries everything the user prompt is rendered from.
type assistPromptContext struct {
	now        time.Time // client wall clock (from req.ClientNow, fallback time.Now())
	timezone   string
	agents     []AgentInfo
	schedules  []protocol.ScheduleSummary
	transcript []protocol.ScheduleAssistTurn // oldest first, ≤ 10 after trim
	message    string
}

// assistSystemPrompt is the fixed parse contract sent as the system message.
// See docs/product/chat-schedule-assistant-design.md Appendix A.
func assistSystemPrompt() string {
	return `You are the Solo Schedule Assistant. Convert the user's request into ONE JSON
object and output ONLY that JSON (optionally in a ` + "```json" + ` fence). No prose.

Output shapes:
{"kind":"proposal","op":"create","name":string,"prompt":string,
 "cadence":{"type":"cron","expression":string} | {"type":"every","everyMs":number},
 "target":{"type":"agent","agentId":string} | {"type":"new-agent","config":{"provider":string,"cwd":string}},
 "maxRuns":number?,"expiresAt":string?,"summary":string,"warnings":string[]?}
{"kind":"proposal","op":"update","scheduleId":string, ...changed fields..., "summary":string}
{"kind":"proposal","op":"pause"|"resume"|"delete","scheduleId":string,"name":string,"summary":string}
{"kind":"clarify","message":string}
{"kind":"answer","message":string}

Rules:
- Cron expressions are in the client timezone given below. Minimum interval: 60000ms.
- Prefer "cron" for clock times ("at 9", "weekday mornings"); "every" for pure intervals.
- Reference agents/schedules ONLY from the context block; copy ids exactly.
- Target type "agent" only when a matching existing agent is listed below;
  otherwise "new-agent" with provider+cwd when inferable, else kind="clarify".
- Ambiguous reference or missing required info (target agent, time) → kind="clarify".
- Pure questions about schedules → kind="answer".
- "expiresAt" must be RFC3339 in the future; "maxRuns" must be > 0;
  "prompt" must be ≤ 4000 characters.
- Reply in the user's language.`
}

// buildAssistUserPrompt renders the context block + transcript + user message.
func buildAssistUserPrompt(ctx assistPromptContext) string {
	contextBlock := renderAssistContextBlock(ctx)

	turns := ctx.transcript
	if len(turns) > maxAssistTranscriptTurns {
		turns = turns[len(turns)-maxAssistTranscriptTurns:]
	}
	lines := make([]string, 0, len(turns))
	for _, turn := range turns {
		role := "Assistant"
		if turn.Role == "user" {
			role = "User"
		}
		lines = append(lines, role+": "+sanitizeAssistLine(turn.Content))
	}

	// Enforce the total cap: drop oldest transcript turns first.
	dropped := 0
	section := renderAssistTranscriptSection(lines, dropped)
	for len(lines) > 0 && len(contextBlock)+len(section) > maxAssistPromptContextChars {
		lines = lines[1:]
		dropped++
		section = renderAssistTranscriptSection(lines, dropped)
	}

	out := contextBlock + section
	if len(out) > maxAssistPromptContextChars {
		const suffix = "\n…(context truncated)"
		out = out[:maxAssistPromptContextChars-len(suffix)] + suffix
	}
	return out + "\n\nUser: " + ctx.message
}

func renderAssistContextBlock(ctx assistPromptContext) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Current time (client): %s\n", ctx.now.Format(time.RFC3339))
	fmt.Fprintf(&sb, "Client timezone: %s\n", sanitizeAssistLine(ctx.timezone))

	sb.WriteString("\nExisting agents:\n")
	agents := append([]AgentInfo(nil), ctx.agents...)
	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
	if len(agents) == 0 {
		sb.WriteString("- (none)\n")
	}
	for i, a := range agents {
		if i >= maxAssistContextAgents {
			fmt.Fprintf(&sb, "- …and %d more\n", len(agents)-maxAssistContextAgents)
			break
		}
		fmt.Fprintf(&sb, "- id=%s name=%s provider=%s cwd=%s status=%s\n",
			sanitizeAssistLine(a.ID), strconv.Quote(a.Title),
			sanitizeAssistLine(a.Provider), sanitizeAssistLine(a.Cwd), sanitizeAssistLine(a.Status))
	}

	sb.WriteString("\nExisting schedules:\n")
	schedules := append([]protocol.ScheduleSummary(nil), ctx.schedules...)
	sort.Slice(schedules, func(i, j int) bool { return schedules[i].ID < schedules[j].ID })
	if len(schedules) == 0 {
		sb.WriteString("- (none)\n")
	}
	for i, s := range schedules {
		if i >= maxAssistContextSchedules {
			fmt.Fprintf(&sb, "- …and %d more\n", len(schedules)-maxAssistContextSchedules)
			break
		}
		name := ""
		if s.Name != nil {
			name = *s.Name
		}
		nextRunAt := "-"
		if s.NextRunAt != nil && *s.NextRunAt != "" {
			nextRunAt = sanitizeAssistLine(*s.NextRunAt)
		}
		fmt.Fprintf(&sb, "- id=%s name=%s cadence=%s status=%s nextRunAt=%s\n",
			sanitizeAssistLine(s.ID), strconv.Quote(name),
			renderAssistCadence(s.Cadence), sanitizeAssistLine(s.Status), nextRunAt)
	}

	return strings.TrimRight(sb.String(), "\n")
}

func renderAssistCadence(c protocol.ScheduleCadence) string {
	switch c.Type {
	case "cron":
		return fmt.Sprintf("cron %q", c.Expression)
	case "every":
		return fmt.Sprintf("every %dms", c.EveryMs)
	default:
		return sanitizeAssistLine(c.Type)
	}
}

func renderAssistTranscriptSection(lines []string, dropped int) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\nRecent conversation:\n")
	if dropped > 0 {
		fmt.Fprintf(&sb, "(…%d earlier turn(s) omitted)\n", dropped)
	}
	sb.WriteString(strings.Join(lines, "\n"))
	return sb.String()
}

// sanitizeAssistLine strips line breaks from untrusted single-line fields
// (agent/schedule context data) so they cannot break the prompt structure.
func sanitizeAssistLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}
