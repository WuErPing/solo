package codex

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/contracttest"
	"github.com/WuErPing/solo/protocol"
)

// -----------------------------------------------------------------------
// Models & Modes
// -----------------------------------------------------------------------

func TestCodexModels(t *testing.T) {
	models := codexModels()
	if len(models) == 0 {
		t.Fatal("codexModels() returned empty")
	}

	ids := make(map[string]bool)
	for _, m := range models {
		ids[m.ID] = true
	}

	expectedIDs := []string{"gpt-5.5", "gpt-5.5-pro", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"}
	for _, id := range expectedIDs {
		if !ids[id] {
			t.Errorf("missing model %q", id)
		}
	}

	// Verify exactly one default
	defaults := 0
	for _, m := range models {
		if m.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("expected 1 default model, got %d", defaults)
	}
}

func TestCodexModes(t *testing.T) {
	modes := codexModes()
	if len(modes) != 2 {
		t.Fatalf("expected 2 modes, got %d", len(modes))
	}

	ids := make(map[string]bool)
	for _, m := range modes {
		ids[m.ID] = true
	}
	if !ids["auto"] {
		t.Error("missing mode 'auto'")
	}
	if !ids["full-access"] {
		t.Error("missing mode 'full-access'")
	}
}

// -----------------------------------------------------------------------
// buildArgs
// -----------------------------------------------------------------------

func TestCodexBuildArgs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	t.Run("basic_prompt", func(t *testing.T) {
		session := &codexSession{
			base:       base.NewBaseSession("codex", &protocol.AgentSessionConfig{Cwd: "/tmp"}, logger),
			binaryPath: "/usr/bin/codex",
		}
		args := session.buildArgs("hello world", "")
		// Should contain: exec, --experimental-json, --ephemeral, --skip-git-repo-check, --sandbox workspace-write, prompt
		assertContains(t, args, "exec")
		assertContains(t, args, "--experimental-json")
		assertContains(t, args, "--ephemeral")
		assertContains(t, args, "--skip-git-repo-check")
		assertContains(t, args, "--sandbox")
		assertContains(t, args, "workspace-write")
		assertContains(t, args, "hello world")
	})

	t.Run("full_access_mode", func(t *testing.T) {
		session := &codexSession{
			base:       base.NewBaseSession("codex", &protocol.AgentSessionConfig{Cwd: "/tmp"}, logger),
			binaryPath: "/usr/bin/codex",
		}
		_ = session.base.SetMode("full-access")
		args := session.buildArgs("test", "")
		// full-access should NOT have --sandbox flag
		assertNotContains(t, args, "--sandbox")
	})

	t.Run("with_model", func(t *testing.T) {
		session := &codexSession{
			base:       base.NewBaseSession("codex", &protocol.AgentSessionConfig{Cwd: "/tmp"}, logger),
			binaryPath: "/usr/bin/codex",
		}
		_ = session.base.SetModel("gpt-5.4")
		args := session.buildArgs("test", "")
		assertContains(t, args, "--model")
		assertContains(t, args, "gpt-5.4")
	})

	t.Run("with_resume", func(t *testing.T) {
		session := &codexSession{
			base:       base.NewBaseSession("codex", &protocol.AgentSessionConfig{Cwd: "/tmp"}, logger),
			binaryPath: "/usr/bin/codex",
		}
		args := session.buildArgs("test", "abc-123")
		// Should use resume subcommand instead of exec
		assertContains(t, args, "resume")
		assertContains(t, args, "abc-123")
	})
}

// -----------------------------------------------------------------------
// buildEnv
// -----------------------------------------------------------------------

func TestCodexBuildEnv(t *testing.T) {
	session := &codexSession{}
	env := session.buildEnv()

	envMap := make(map[string]bool)
	for _, e := range env {
		envMap[e] = true
	}

	// Should NOT contain Solo-specific vars
	for _, blocked := range []string{
		"CLAUDECODE=1",
		"CLAUDE_CODE_ENTRYPOINT=test",
	} {
		if envMap[blocked] {
			t.Errorf("env should not contain %q", blocked)
		}
	}

	// Should contain PATH
	foundPath := false
	for _, e := range env {
		if len(e) >= 5 && e[:5] == "PATH=" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Error("env should contain PATH")
	}
}

// -----------------------------------------------------------------------
// Translator
// -----------------------------------------------------------------------

func TestCodexTranslator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	newTranslator := func() *codexTranslator {
		return &codexTranslator{
			logger:    logger,
			messageID: "msg-test-001",
		}
	}

	t.Run("turn_started_emits_thread_started", func(t *testing.T) {
		tr := newTranslator()
		raw := `{"type":"TurnStartedNotification","thread_id":"t-123"}`
		events, isTerminal, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isTerminal {
			t.Error("turn_started should not be terminal")
		}
		if len(events) == 0 {
			t.Fatal("expected at least 1 event")
		}
		if _, ok := events[0].(protocol.ThreadStartedStreamEvent); !ok {
			t.Errorf("expected ThreadStartedStreamEvent, got %T", events[0])
		}
	})

	t.Run("agent_message_delta", func(t *testing.T) {
		tr := newTranslator()
		// First emit thread started to set up state
		raw1 := `{"type":"TurnStartedNotification","thread_id":"t-123"}`
		_, _, _ = tr.Translate([]byte(raw1), time.Now())

		raw := `{"type":"AgentMessageDeltaNotification","delta":"Hello"}`
		events, isTerminal, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isTerminal {
			t.Error("agent_message_delta should not be terminal")
		}
		found := false
		for _, evt := range events {
			if te, ok := evt.(protocol.TimelineStreamEvent); ok {
				if te.Item.Type == "assistant_message" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("expected assistant_message TimelineStreamEvent, got %v", eventTypesListFromInterface(events))
		}
	})

	t.Run("reasoning_delta", func(t *testing.T) {
		tr := newTranslator()
		raw1 := `{"type":"TurnStartedNotification","thread_id":"t-123"}`
		_, _, _ = tr.Translate([]byte(raw1), time.Now())

		raw := `{"type":"ReasoningTextDeltaNotification","delta":"thinking..."}`
		events, _, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		found := false
		for _, evt := range events {
			if te, ok := evt.(protocol.TimelineStreamEvent); ok {
				if te.Item.Type == "reasoning" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("expected reasoning TimelineStreamEvent, got %v", eventTypesListFromInterface(events))
		}
	})

	t.Run("turn_completed_is_terminal", func(t *testing.T) {
		tr := newTranslator()
		raw := `{"type":"TurnCompletedNotification","thread_id":"t-123"}`
		events, isTerminal, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isTerminal {
			t.Error("turn_completed should be terminal")
		}
		if len(events) == 0 {
			t.Fatal("expected at least 1 event")
		}
		// Last event should be TurnCompletedStreamEvent
		last := events[len(events)-1]
		if _, ok := last.(protocol.TurnCompletedStreamEvent); !ok {
			t.Errorf("expected TurnCompletedStreamEvent, got %T", last)
		}
	})

	t.Run("turn_aborted_is_terminal", func(t *testing.T) {
		tr := newTranslator()
		raw := `{"type":"TurnAbortedNotification","reason":"StreamContextWindowExceeded"}`
		events, isTerminal, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isTerminal {
			t.Error("turn_aborted should be terminal")
		}
		if len(events) == 0 {
			t.Fatal("expected at least 1 event")
		}
		last := events[len(events)-1]
		if _, ok := last.(protocol.TurnFailedStreamEvent); !ok {
			t.Errorf("expected TurnFailedStreamEvent, got %T", last)
		}
	})

	t.Run("usage_updated", func(t *testing.T) {
		tr := newTranslator()
		raw := `{"type":"ThreadTokenUsageUpdatedNotification","input_tokens":100,"output_tokens":50}`
		events, _, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		found := false
		for _, evt := range events {
			if _, ok := evt.(protocol.UsageUpdatedStreamEvent); ok {
				found = true
			}
		}
		if !found {
			t.Errorf("expected UsageUpdatedStreamEvent, got %v", eventTypesListFromInterface(events))
		}
	})

	t.Run("unknown_type_no_error", func(t *testing.T) {
		tr := newTranslator()
		raw := `{"type":"SomeUnknownNotification","data":"value"}`
		events, isTerminal, err := tr.Translate([]byte(raw), time.Now())
		if err != nil {
			t.Fatalf("unexpected error for unknown type: %v", err)
		}
		if isTerminal {
			t.Error("unknown type should not be terminal")
		}
		_ = events // may be empty, that's fine
	})

	t.Run("invalid_json_returns_error", func(t *testing.T) {
		tr := newTranslator()
		_, _, err := tr.Translate([]byte("not json"), time.Now())
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// -----------------------------------------------------------------------
// Terminal Detector
// -----------------------------------------------------------------------

func TestCodexTerminalDetector(t *testing.T) {
	detector := &codexTerminalDetector{}

	t.Run("turn_completed_is_terminal", func(t *testing.T) {
		evt := agent.AgentStreamEvent{Event: protocol.TurnCompletedStreamEvent{}}
		result, isTerm, err := detector.IsTerminal(evt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isTerm {
			t.Error("expected terminal")
		}
		if result == nil {
			t.Error("expected non-nil result")
		}
	})

	t.Run("turn_failed_is_terminal", func(t *testing.T) {
		evt := agent.AgentStreamEvent{Event: protocol.TurnFailedStreamEvent{Error: "test"}}
		result, isTerm, err := detector.IsTerminal(evt)
		// TurnFailedStreamEvent is terminal and returns an error
		if !isTerm {
			t.Error("expected terminal")
		}
		if result == nil {
			t.Error("expected non-nil result")
		}
		if err == nil {
			t.Error("expected error for turn failed")
		}
	})

	t.Run("timeline_is_not_terminal", func(t *testing.T) {
		evt := agent.AgentStreamEvent{Event: protocol.TimelineStreamEvent{}}
		_, isTerm, _ := detector.IsTerminal(evt)
		if isTerm {
			t.Error("timeline should not be terminal")
		}
	})

	t.Run("non_agent_stream_event_not_terminal", func(t *testing.T) {
		_, isTerm, _ := detector.IsTerminal("some string")
		if isTerm {
			t.Error("string should not be terminal")
		}
	})
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func assertContains(t *testing.T, slice []string, target string) {
	t.Helper()
	for _, s := range slice {
		if s == target {
			return
		}
	}
	t.Errorf("expected slice to contain %q, got %v", target, slice)
}

func assertNotContains(t *testing.T, slice []string, target string) {
	t.Helper()
	for _, s := range slice {
		if s == target {
			t.Errorf("expected slice to NOT contain %q", target)
			return
		}
	}
}

func eventTypesListFromInterface(events []interface{}) []string {
	types := make([]string, len(events))
	for i, evt := range events {
		switch e := evt.(type) {
		case protocol.ThreadStartedStreamEvent:
			types[i] = "thread_started"
		case protocol.TimelineStreamEvent:
			types[i] = "timeline:" + e.Item.Type
		case protocol.TurnCompletedStreamEvent:
			types[i] = "turn_completed"
		case protocol.TurnFailedStreamEvent:
			types[i] = "turn_failed"
		case protocol.TurnCanceledStreamEvent:
			types[i] = "turn_canceled"
		case protocol.UsageUpdatedStreamEvent:
			types[i] = "usage_updated"
		default:
			types[i] = jsonType(evt)
		}
	}
	return types
}

func jsonType(v interface{}) string {
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if t, ok := m["type"].(string); ok {
		return t
	}
	return "unknown"
}

func TestProviderContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Codex contract test in short mode")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient("", logger)
	contracttest.RunProviderContractSuite(t, "codex", client)
}
