package schedule

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func strPtr(s string) *string { return &s }

func baseAssistPromptContext() assistPromptContext {
	return assistPromptContext{
		now:      time.Date(2026, 7, 17, 22, 50, 0, 0, time.FixedZone("CST", 8*3600)),
		timezone: "Asia/Shanghai",
		agents: []AgentInfo{
			{ID: "a1", Title: "backend-worker", Provider: "claude", Cwd: "~/work/backend", Status: "running"},
		},
		schedules: []protocol.ScheduleSummary{
			{
				ID:        "s1",
				Name:      strPtr("Nightly test summary"),
				Cadence:   protocol.ScheduleCadence{Type: "cron", Expression: "0 9 * * 1-5"},
				Status:    "active",
				NextRunAt: strPtr("2026-07-18T01:00:00Z"),
			},
		},
		transcript: []protocol.ScheduleAssistTurn{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
		message: "every weekday at 9am, summarize the nightly test runs",
	}
}

func TestBuildAssistUserPrompt_ContainsContext(t *testing.T) {
	prompt := buildAssistUserPrompt(baseAssistPromptContext())

	wantParts := []string{
		"Current time (client): 2026-07-17T22:50:00+08:00",
		"Client timezone: Asia/Shanghai",
		`- id=a1 name="backend-worker" provider=claude cwd=~/work/backend status=running`,
		`- id=s1 name="Nightly test summary" cadence=cron "0 9 * * 1-5" status=active nextRunAt=2026-07-18T01:00:00Z`,
		"User: hi",
		"Assistant: hello",
		"User: every weekday at 9am, summarize the nightly test runs",
	}
	for _, part := range wantParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", part, prompt)
		}
	}
}

func TestBuildAssistUserPrompt_EveryCadenceAndNilNextRun(t *testing.T) {
	ctx := baseAssistPromptContext()
	ctx.schedules = []protocol.ScheduleSummary{
		{
			ID:      "s2",
			Name:    strPtr("Hourly cleanup"),
			Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
			Status:  "paused",
		},
	}

	prompt := buildAssistUserPrompt(ctx)

	if !strings.Contains(prompt, `- id=s2 name="Hourly cleanup" cadence=every 3600000ms status=paused nextRunAt=-`) {
		t.Errorf("prompt missing every-cadence schedule line\n--- prompt ---\n%s", prompt)
	}
}

func TestBuildAssistUserPrompt_TranscriptTrimmedToLast10(t *testing.T) {
	ctx := baseAssistPromptContext()
	ctx.transcript = nil
	for i := 0; i < 12; i++ {
		ctx.transcript = append(ctx.transcript, protocol.ScheduleAssistTurn{
			Role:    "user",
			Content: fmt.Sprintf("turn-%02d", i),
		})
	}

	prompt := buildAssistUserPrompt(ctx)

	if strings.Contains(prompt, "turn-00") || strings.Contains(prompt, "turn-01") {
		t.Error("expected oldest transcript turns to be dropped")
	}
	if !strings.Contains(prompt, "turn-11") {
		t.Error("expected newest transcript turn to be kept")
	}
}

func TestBuildAssistUserPrompt_AgentsTruncatedWithNote(t *testing.T) {
	ctx := baseAssistPromptContext()
	ctx.agents = nil
	for i := 0; i < 55; i++ {
		ctx.agents = append(ctx.agents, AgentInfo{
			ID:       fmt.Sprintf("agent-%03d", i),
			Title:    fmt.Sprintf("worker-%03d", i),
			Provider: "claude",
			Cwd:      "/w",
			Status:   "idle",
		})
	}

	prompt := buildAssistUserPrompt(ctx)

	if !strings.Contains(prompt, "and 5 more") {
		t.Error("expected truncation note for agents beyond 50")
	}
	if strings.Contains(prompt, "id=agent-050") {
		t.Error("expected agents beyond 50 to be omitted")
	}
	if !strings.Contains(prompt, "id=agent-049") {
		t.Error("expected first 50 agents to be kept")
	}
}

func TestBuildAssistUserPrompt_SchedulesTruncatedWithNote(t *testing.T) {
	ctx := baseAssistPromptContext()
	ctx.schedules = nil
	for i := 0; i < 55; i++ {
		ctx.schedules = append(ctx.schedules, protocol.ScheduleSummary{
			ID:      fmt.Sprintf("sched-%03d", i),
			Name:    strPtr(fmt.Sprintf("job-%03d", i)),
			Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
			Status:  "active",
		})
	}

	prompt := buildAssistUserPrompt(ctx)

	if !strings.Contains(prompt, "and 5 more") {
		t.Error("expected truncation note for schedules beyond 50")
	}
	if strings.Contains(prompt, "id=sched-050") {
		t.Error("expected schedules beyond 50 to be omitted")
	}
}

func TestBuildAssistUserPrompt_TotalCapRespected(t *testing.T) {
	ctx := baseAssistPromptContext()
	// ~1500 chars of schedule context.
	ctx.schedules = nil
	for i := 0; i < 10; i++ {
		ctx.schedules = append(ctx.schedules, protocol.ScheduleSummary{
			ID:      fmt.Sprintf("sched-%03d", i),
			Name:    strPtr(fmt.Sprintf("job-%03d-%s", i, strings.Repeat("n", 100))),
			Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
			Status:  "active",
		})
	}
	// 10 transcript turns of 1000 chars each = 10000 chars — must be trimmed.
	ctx.transcript = nil
	for i := 0; i < 10; i++ {
		ctx.transcript = append(ctx.transcript, protocol.ScheduleAssistTurn{
			Role:    "user",
			Content: fmt.Sprintf("turn-%02d-%s", i, strings.Repeat("x", 1000)),
		})
	}

	prompt := buildAssistUserPrompt(ctx)

	idx := strings.LastIndex(prompt, "\n\nUser: ")
	if idx < 0 {
		t.Fatal("prompt missing final user message marker")
	}
	if got := len(prompt[:idx]); got > maxAssistPromptContextChars {
		t.Errorf("context+transcript length = %d, exceeds cap %d", got, maxAssistPromptContextChars)
	}
	// Oldest transcript turns dropped first; context block stays intact.
	if strings.Contains(prompt, "turn-00-") {
		t.Error("expected oldest transcript turn to be dropped before shrinking context")
	}
	if !strings.Contains(prompt, "turn-09-") {
		t.Error("expected newest transcript turn to be kept")
	}
	if !strings.Contains(prompt, "id=sched-009") {
		t.Error("expected schedule context to survive transcript trimming")
	}
	if !strings.Contains(prompt, "omitted") {
		t.Error("expected a note that transcript turns were omitted")
	}
}

func TestAssistSystemPrompt_Contract(t *testing.T) {
	sys := assistSystemPrompt()

	for _, part := range []string{`"kind":"proposal"`, "60000", "clarify", "answer", "cron"} {
		if !strings.Contains(sys, part) {
			t.Errorf("system prompt missing %q", part)
		}
	}
}
