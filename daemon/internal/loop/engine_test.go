package loop

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

type fakeLoopAgent struct {
	id     string
	status protocol.AgentLifecycleStatus
}

type fakeLoopAgentManager struct {
	mu       sync.Mutex
	created  []protocol.AgentSessionConfig
	deleted  []string
	messages []string
	agents   map[string]*fakeLoopAgent
	subs     []agent.AgentEventFunc
}

func newFakeLoopAgentManager() *fakeLoopAgentManager {
	return &fakeLoopAgentManager{
		agents: make(map[string]*fakeLoopAgent),
	}
}

func (m *fakeLoopAgentManager) CreateAgent(_ context.Context, config *protocol.AgentSessionConfig, _ map[string]string) (*agent.ManagedAgent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.created = append(m.created, *config)
	id := "agent-" + config.Provider
	if config.Model != nil && *config.Model != "" {
		id += "-" + *config.Model
	}
	m.agents[id] = &fakeLoopAgent{id: id, status: protocol.AgentIdle}
	return &agent.ManagedAgent{ID: id}, nil
}

func (m *fakeLoopAgentManager) DeleteAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, agentID)
	delete(m.agents, agentID)
	return nil
}

func (m *fakeLoopAgentManager) SendAgentMessage(_ context.Context, _ string, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, text)
	return nil
}

func (m *fakeLoopAgentManager) GetAgent(agentID string) *agent.ManagedAgent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.agents[agentID]; !ok {
		return nil
	}
	return &agent.ManagedAgent{ID: agentID}
}

func (m *fakeLoopAgentManager) Subscribe(handler agent.AgentEventFunc) func() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs = append(m.subs, handler)
	return func() {}
}

func (m *fakeLoopAgentManager) createdConfigs() []protocol.AgentSessionConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]protocol.AgentSessionConfig, len(m.created))
	copy(out, m.created)
	return out
}

func (m *fakeLoopAgentManager) sentMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.messages))
	copy(out, m.messages)
	return out
}

func intPtr(i int) *int { return &i }

func TestLoopEngineUsesAgentTemplate(t *testing.T) {
	mgr := newFakeLoopAgentManager()
	store := NewStore(WithLogger(testLogger()))
	engine := NewEngineWithManager(store, mgr, testLogger())

	model := "claude-3-opus"
	record, err := store.Create(protocol.LoopRunRequest{
		Prompt: "fix tests",
		Cwd:    "/project",
		AgentTemplate: &protocol.AgentTemplate{
			Provider:     "claude",
			Model:        &model,
			SystemPrompt: "base prompt",
			McpServers: map[string]protocol.McpServerConfig{
				"fs": {Type: "stdio", Command: "mcp-fs"},
			},
		},
	}, func() (string, error) { return "mock", nil })
	if err != nil {
		t.Fatalf("create loop: %v", err)
	}

	engine.Start(context.Background(), record.ID)

	// Wait for the worker agent to be created.
	deadline := time.Now().Add(2 * time.Second)
	for len(mgr.createdConfigs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	created := mgr.createdConfigs()
	if len(created) == 0 {
		t.Fatal("no agent created")
	}
	cfg := created[0]
	if cfg.Provider != "claude" {
		t.Errorf("provider: got %q, want claude", cfg.Provider)
	}
	if cfg.Model == nil || *cfg.Model != model {
		t.Errorf("model: got %v, want %v", cfg.Model, &model)
	}
	if cfg.SystemPrompt != "base prompt" {
		t.Errorf("systemPrompt: got %q, want base prompt", cfg.SystemPrompt)
	}
	if len(cfg.McpServers) != 1 || cfg.McpServers["fs"].Type != "stdio" {
		t.Errorf("mcpServers: got %#v, want one stdio server", cfg.McpServers)
	}
	if cfg.Cwd != "/project" {
		t.Errorf("cwd: got %q, want /project", cfg.Cwd)
	}
}

func TestLoopEngineFallsBackToLegacyProviderModel(t *testing.T) {
	mgr := newFakeLoopAgentManager()
	store := NewStore(WithLogger(testLogger()))
	engine := NewEngineWithManager(store, mgr, testLogger())

	model := "claude-3-opus"
	provider := "claude"
	record, err := store.Create(protocol.LoopRunRequest{
		Prompt:   "fix tests",
		Cwd:      "/project",
		Provider: &provider,
		Model:    &model,
	}, func() (string, error) { return "mock", nil })
	if err != nil {
		t.Fatalf("create loop: %v", err)
	}

	engine.Start(context.Background(), record.ID)

	deadline := time.Now().Add(2 * time.Second)
	for len(mgr.createdConfigs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	created := mgr.createdConfigs()
	if len(created) == 0 {
		t.Fatal("no agent created")
	}
	cfg := created[0]
	if cfg.Provider != "claude" {
		t.Errorf("provider: got %q, want claude", cfg.Provider)
	}
	if cfg.Model == nil || *cfg.Model != model {
		t.Errorf("model: got %v, want %v", cfg.Model, &model)
	}
	if cfg.Cwd != "/project" {
		t.Errorf("cwd: got %q, want /project", cfg.Cwd)
	}
}

func TestBuildWorkerPrompt(t *testing.T) {
	base := "Fix all test failures until `make ci` passes"

	tests := []struct {
		name        string
		prev        *protocol.LoopIterationRecord
		wantEqual   string
		wantContain []string
	}{
		{
			name:      "first iteration sends the base prompt unchanged",
			prev:      nil,
			wantEqual: base,
		},
		{
			name: "previous iteration with only passing checks adds no feedback",
			prev: &protocol.LoopIterationRecord{
				VerifyChecks: []protocol.LoopVerifyCheckResult{
					{Command: "make ci", ExitCode: 0, Passed: true, Stdout: "ok"},
				},
			},
			wantEqual: base,
		},
		{
			name: "failed check is fed back with command, exit code, and output",
			prev: &protocol.LoopIterationRecord{
				VerifyChecks: []protocol.LoopVerifyCheckResult{
					{
						Command:  "make ci",
						ExitCode: 2,
						Passed:   false,
						Stdout:   "ScheduleTargetSchema is defined but never used",
						Stderr:   "make[1]: *** [lint] Error 1",
					},
				},
			},
			wantContain: []string{
				base,
				"make ci",
				"2",
				"ScheduleTargetSchema is defined but never used",
				"make[1]: *** [lint] Error 1",
			},
		},
		{
			name: "verify-prompt failure feeds back its reason",
			prev: &protocol.LoopIterationRecord{
				VerifyPrompt: &protocol.LoopVerifyPromptResult{
					Passed: false,
					Reason: "goal not met: tests still red",
				},
			},
			wantContain: []string{base, "goal not met: tests still red"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildWorkerPrompt(base, tc.prev)
			if tc.wantEqual != "" {
				if got != tc.wantEqual {
					t.Fatalf("buildWorkerPrompt = %q, want %q", got, tc.wantEqual)
				}
				return
			}
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("buildWorkerPrompt result missing %q\nfull prompt:\n%s", want, got)
				}
			}
		})
	}
}

func TestBuildWorkerPromptTruncatesHugeOutput(t *testing.T) {
	huge := strings.Repeat("x", maxFeedbackOutputRunes*4)
	prev := &protocol.LoopIterationRecord{
		VerifyChecks: []protocol.LoopVerifyCheckResult{
			{Command: "make ci", ExitCode: 1, Passed: false, Stdout: huge},
		},
	}
	got := buildWorkerPrompt("base", prev)
	if len(got) > len("base")+maxFeedbackOutputRunes+512 {
		t.Errorf("expected output to be truncated, got length %d", len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected a truncation marker in the fed-back output")
	}
}

func TestLoopEngineFeedsVerificationFailureToNextWorker(t *testing.T) {
	mgr := newFakeLoopAgentManager()
	store := NewStore(WithLogger(testLogger()))
	engine := NewEngineWithManager(store, mgr, testLogger())

	provider := "mock"
	record, err := store.Create(protocol.LoopRunRequest{
		Prompt:        "fix it",
		Cwd:           "",
		Provider:      &provider,
		VerifyChecks:  []string{"echo OUT_TOKEN; echo ERR_TOKEN 1>&2; exit 3"},
		MaxIterations: intPtr(2),
		SleepMs:       intPtr(0),
	}, func() (string, error) { return "mock", nil })
	if err != nil {
		t.Fatalf("create loop: %v", err)
	}

	engine.Start(context.Background(), record.ID)

	// Wait for the loop to reach a terminal state (it must fail after 2 tries).
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec, ok := store.Get(record.ID)
		if ok && rec.Status != string(StatusRunning) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("loop did not finish in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	messages := mgr.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 worker messages, got %d: %#v", len(messages), messages)
	}
	if messages[0] != "fix it" {
		t.Errorf("first message should be the bare prompt, got %q", messages[0])
	}
	for _, want := range []string{"fix it", "OUT_TOKEN", "ERR_TOKEN", "3", "echo OUT_TOKEN"} {
		if !strings.Contains(messages[1], want) {
			t.Errorf("second message missing %q\nfull message:\n%s", want, messages[1])
		}
	}
}

func TestRunShellCheckRespectsTimeout(t *testing.T) {
	ctx := context.Background()

	exitCode, _, _, err := runShellCheck(ctx, "", "sleep 2", 50*time.Millisecond)
	if err == nil {
		t.Error("expected a timeout error for a command exceeding the timeout")
	}
	if exitCode == 0 {
		t.Error("expected a non-zero exit code when the command is killed by the timeout")
	}

	exitCode, _, _, err = runShellCheck(ctx, "", "exit 0", time.Second)
	if err != nil {
		t.Errorf("fast command should not error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("fast command exit code = %d, want 0", exitCode)
	}
}

func TestNewEngineUsesGenerousVerifyTimeout(t *testing.T) {
	mgr := newFakeLoopAgentManager()
	store := NewStore(WithLogger(testLogger()))
	engine := NewEngineWithManager(store, mgr, testLogger())

	if engine.verifyTimeout < 10*time.Minute {
		t.Errorf("verify timeout = %s, want at least 10m so a full CI run is not killed", engine.verifyTimeout)
	}
}
