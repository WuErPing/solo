package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// --- RED Phase: Write failing tests for KimiAgentClient ---

func TestKimiAgentClient_Provider_ReturnsKimi(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient("", logger)

	if got := client.Provider(); got != "kimi" {
		t.Errorf("Provider() = %q, want %q", got, "kimi")
	}
}

func TestKimiAgentClient_IsAvailable_BinaryNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient("/nonexistent/kimi", logger)

	if err := client.IsAvailable(context.Background()); err == nil {
		t.Error("IsAvailable() = nil, want error for missing binary")
	}
}

func TestKimiAgentClient_IsAvailable_BinaryFound(t *testing.T) {
	// Create a temporary executable file to simulate kimi binary.
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "kimi-cli")
	if err := os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho ok"), 0755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient(fakeBinary, logger)

	if err := client.IsAvailable(context.Background()); err != nil {
		t.Errorf("IsAvailable() = %v, want nil", err)
	}
}

func TestKimiAgentClient_ListModels_ReturnsStaticModels(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient("", logger)

	models, err := client.ListModels(context.Background(), "")
	if err != nil {
		t.Fatalf("ListModels() = %v", err)
	}

	if len(models) == 0 {
		t.Error("ListModels() returned empty list, want non-empty")
	}

	foundDefault := false
	for _, m := range models {
		if m.IsDefault {
			foundDefault = true
		}
		if m.Provider != "kimi" {
			t.Errorf("model provider = %q, want %q", m.Provider, "kimi")
		}
	}
	if !foundDefault {
		t.Error("ListModels() has no default model")
	}
}

func TestKimiAgentClient_ListClientCommands_ReturnsStaticCommands(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewKimiAgentClient("", logger)
	cmds, err := client.ListClientCommands(context.Background(), "/tmp")
	if err == nil {
		// Binary may not be found in test environment; that's acceptable.
		if len(cmds) == 0 {
			t.Error("expected non-empty command list")
		}
		foundInit := false
		for _, c := range cmds {
			if c.Name == "init" {
				foundInit = true
			}
		}
		if !foundInit {
			t.Error("expected 'init' command in list")
		}
	}
}

func TestKimiAgentClient_ListModes_ReturnsStaticModes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient("", logger)

	modes, err := client.ListModes(context.Background(), "")
	if err != nil {
		t.Fatalf("ListModes() = %v", err)
	}

	if len(modes) == 0 {
		t.Error("ListModes() returned empty list, want non-empty")
	}

	expectedIDs := map[string]bool{"default": false, "bypassPermissions": false, "plan": false}
	for _, m := range modes {
		if _, ok := expectedIDs[m.ID]; ok {
			expectedIDs[m.ID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("ListModes() missing expected mode %q", id)
		}
	}
}

func TestKimiAgentClient_CreateSession_ReturnsSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient("", logger)

	config := &protocol.AgentSessionConfig{
		Provider: "kimi",
		Cwd:      "/tmp/test",
	}

	session, err := client.CreateSession(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateSession() = %v", err)
	}
	if session == nil {
		t.Fatal("CreateSession() returned nil session")
	}
}

func TestKimiAgentClient_ResumeSession_ReturnsSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := NewKimiAgentClient("", logger)

	handle := &protocol.AgentPersistenceHandle{
		Provider:  "kimi",
		SessionID: "test-session-id",
		Metadata: map[string]interface{}{
			"cwd": "/tmp/test",
		},
	}

	session, err := client.ResumeSession(context.Background(), handle)
	if err != nil {
		t.Fatalf("ResumeSession() = %v", err)
	}
	if session == nil {
		t.Fatal("ResumeSession() returned nil session")
	}
}

// --- RED Phase: kimiWireTranslator tests ---

func TestKimiWireTranslator_EmitsStreamEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	tests := []struct {
		name         string
		wire         string
		wantTerminal bool
		wantLen      int
		check        func(t *testing.T, evts []interface{})
	}{
		{
			name:         "TurnBegin",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"TurnBegin","payload":{"user_input":"test prompt"}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				se, ok := evt.Event.(protocol.StreamEvent)
				if !ok {
					t.Fatalf("expected protocol.StreamEvent, got %T", evt.Event)
				}
				if se.StreamEventType() != "thread_started" {
					t.Errorf("type = %q, want thread_started", se.StreamEventType())
				}
				ts, ok := se.(protocol.ThreadStartedStreamEvent)
				if !ok {
					t.Fatalf("expected ThreadStartedStreamEvent, got %T", se)
				}
				if ts.Provider != "kimi" {
					t.Errorf("provider = %q, want kimi", ts.Provider)
				}
			},
		},
		{
			name:         "ContentPart text",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":"Hello world"}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl, ok := evt.Event.(protocol.TimelineStreamEvent)
				if !ok {
					t.Fatalf("expected TimelineStreamEvent, got %T", evt.Event)
				}
				if tl.Provider != "kimi" {
					t.Errorf("provider = %q, want kimi", tl.Provider)
				}
				if tl.Item.Type != "assistant_message" || tl.Item.Text != "Hello world" {
					t.Errorf("item = %+v, want assistant_message/Hello world", tl.Item)
				}
				data, err := json.Marshal(tl)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				if !strings.Contains(string(data), `"type":"timeline"`) {
					t.Errorf("JSON missing timeline type: %s", data)
				}
			},
		},
		{
			name:         "ContentPart think",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"think","think":"Let me think..."}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Type != "reasoning" || tl.Item.Text != "Let me think..." {
					t.Errorf("item = %+v", tl.Item)
				}
			},
		},
		{
			name:         "ToolCall",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"ToolCall","payload":{"type":"function","id":"tc-1","function":{"name":"read_file","arguments":"{\"path\":\"main.py\"}"}}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Type != "tool_call" {
					t.Errorf("item.Type = %q, want tool_call", tl.Item.Type)
				}
				if tl.Item.CallID != "tc-1" {
					t.Errorf("item.CallID = %q, want tc-1", tl.Item.CallID)
				}
				if tl.Item.Name != "read_file" {
					t.Errorf("item.Name = %q, want read_file", tl.Item.Name)
				}
				if tl.Item.Status != "running" {
					t.Errorf("item.Status = %q, want running", tl.Item.Status)
				}
			},
		},
		{
			name:         "ToolResult completed",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"ToolResult","payload":{"tool_call_id":"tc-1","return_value":{"is_error":false}}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Type != "tool_call" || tl.Item.CallID != "tc-1" || tl.Item.Status != "completed" {
					t.Errorf("item = %+v", tl.Item)
				}
			},
		},
		{
			name:         "ToolResult failed",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"ToolResult","payload":{"tool_call_id":"tc-1","return_value":{"is_error":true}}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Status != "failed" {
					t.Errorf("status = %q, want failed", tl.Item.Status)
				}
			},
		},
		{
			name:         "TurnEnd",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"TurnEnd","payload":{}}}`,
			wantTerminal: true,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tc, ok := evt.Event.(protocol.TurnCompletedStreamEvent)
				if !ok {
					t.Fatalf("expected TurnCompletedStreamEvent, got %T", evt.Event)
				}
				if tc.Provider != "kimi" {
					t.Errorf("provider = %q, want kimi", tc.Provider)
				}
				data, err := json.Marshal(tc)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				if !strings.Contains(string(data), `"type":"turn_completed"`) {
					t.Errorf("JSON missing turn_completed type: %s", data)
				}
			},
		},
		{
			name:         "ApprovalRequest",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"ApprovalRequest","payload":{"id":"appr-1","tool_call_id":"tc-1","sender":"file_editor","action":"write","description":"write main.py"}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				pr, ok := evt.Event.(protocol.PermissionRequestedStreamEvent)
				if !ok {
					t.Fatalf("expected PermissionRequestedStreamEvent, got %T", evt.Event)
				}
				if pr.Provider != "kimi" {
					t.Errorf("provider = %q, want kimi", pr.Provider)
				}
				if pr.Request.ID != "appr-1" || pr.Request.Kind != "tool" || pr.Request.Title != "write" {
					t.Errorf("request = %+v", pr.Request)
				}
			},
		},
		{
			name:         "CompactionBegin",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"CompactionBegin","payload":{}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Type != "compaction" || tl.Item.CompactionStatus != "loading" {
					t.Errorf("item = %+v", tl.Item)
				}
			},
		},
		{
			name:         "CompactionEnd",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"CompactionEnd","payload":{}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Type != "compaction" || tl.Item.CompactionStatus != "completed" {
					t.Errorf("item = %+v", tl.Item)
				}
			},
		},
		{
			name:         "StepRetry",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"StepRetry","payload":{"n":1,"next_attempt":2,"max_attempts":3,"wait_s":5,"error_type":"rate_limit"}}}`,
			wantTerminal: false,
			wantLen:      1,
			check: func(t *testing.T, evts []interface{}) {
				evt := evts[0].(AgentStreamEvent)
				tl := evt.Event.(protocol.TimelineStreamEvent)
				if tl.Item.Type != "error" {
					t.Errorf("item.Type = %q, want error", tl.Item.Type)
				}
				want := "Step 1 retry 2/3 after 5s: rate_limit"
				if tl.Item.Text != want {
					t.Errorf("item.Text = %q, want %q", tl.Item.Text, want)
				}
			},
		},
		{
			name:         "UnknownEvent ignored",
			wire:         `{"jsonrpc":"2.0","method":"event","params":{"type":"UnknownEvent","payload":{}}}`,
			wantTerminal: false,
			wantLen:      0,
			check:        func(t *testing.T, evts []interface{}) {},
		},
		{
			name:         "NonEvent method ignored",
			wire:         `{"jsonrpc":"2.0","id":"2","result":{"status":"finished"}}`,
			wantTerminal: false,
			wantLen:      0,
			check:        func(t *testing.T, evts []interface{}) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, isTerminal, err := tr.Translate([]byte(tt.wire), time.Now())
			if err != nil {
				t.Fatalf("Translate error: %v", err)
			}
			if isTerminal != tt.wantTerminal {
				t.Errorf("isTerminal = %v, want %v", isTerminal, tt.wantTerminal)
			}
			if len(events) != tt.wantLen {
				t.Fatalf("expected %d events, got %d", tt.wantLen, len(events))
			}
			if tt.wantLen > 0 {
				tt.check(t, events)
			}
		})
	}
}

// --- RED Phase: kimiWireTerminalDetector tests ---

func TestKimiWireTerminalDetector_IsTerminal_TurnCompleted(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	det := &kimiWireTerminalDetector{session: sess}

	evt := AgentStreamEvent{
		Event: protocol.TurnCompletedStreamEvent{
			Provider: kimiProviderName,
		},
	}

	result, err, isTerm := det.IsTerminal(evt)
	if !isTerm {
		t.Fatal("expected terminal")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Canceled {
		t.Error("expected Canceled=false")
	}
}

func TestKimiWireTerminalDetector_IsTerminal_NonTerminal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	det := &kimiWireTerminalDetector{session: sess}

	evt := AgentStreamEvent{
		Event: protocol.TimelineStreamEvent{
			Provider: kimiProviderName,
			Item:     protocol.TimelineItem{Type: "assistant_message", Text: "hi"},
		},
	}

	_, _, isTerm := det.IsTerminal(evt)
	if isTerm {
		t.Error("expected non-terminal")
	}
}

// --- RED Phase: kimiSession JSON-RPC communication tests ---

// fakeWriteCloser is a test double that captures writes without blocking.
type fakeWriteCloser struct {
	mu     sync.Mutex
	data   []byte
	closed bool
}

func (f *fakeWriteCloser) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fmt.Errorf("write to closed fakeWriteCloser")
	}
	f.data = append(f.data, p...)
	return len(p), nil
}

func (f *fakeWriteCloser) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeWriteCloser) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return string(f.data)
}

// fakeKimiProcessManager is a test double for Wire mode.
type fakeKimiProcessManager struct {
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	stdin      io.WriteCloser
	cmd        *exec.Cmd
	started    bool
	startArgs  []string
}

func newFakeKimiProcessManager(stdout io.ReadCloser, stderr io.ReadCloser, stdin io.WriteCloser, cmd *exec.Cmd) *fakeKimiProcessManager {
	return &fakeKimiProcessManager{stdout: stdout, stderr: stderr, stdin: stdin, cmd: cmd}
}

func (f *fakeKimiProcessManager) Start(ctx context.Context, args []string, cwd string, env []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error) {
	f.started = true
	f.startArgs = args
	return f.stdout, f.stderr, f.stdin, f.cmd, nil
}

func (f *fakeKimiProcessManager) Stop(cmd *exec.Cmd, timeout time.Duration) error  { return nil }
func (f *fakeKimiProcessManager) Interrupt(cmd *exec.Cmd) error                    { return nil }
func (f *fakeKimiProcessManager) Kill(cmd *exec.Cmd) error                         { return nil }
func (f *fakeKimiProcessManager) DrainStderr(stderr io.ReadCloser)                 {}
func (f *fakeKimiProcessManager) WaitForExit(cmd *exec.Cmd) (int, error)           { return 0, nil }

// newTestKimiSessionWithPipes creates a kimiSession wired to fake pipes for testing.
func newTestKimiSessionWithPipes(logger *slog.Logger) (*kimiSession, *io.PipeWriter, *fakeWriteCloser) {
	stdoutR, stdoutW := io.Pipe()
	fakeStdin := &fakeWriteCloser{}
	fakeCmd := exec.Command("sleep", "3600")

	s := &kimiSession{
		base:             base.NewBaseSession(kimiProviderName, &protocol.AgentSessionConfig{Cwd: "/tmp/test"}, logger),
		dispatcher:       base.NewChannelDispatcher(logger),
		binaryPath:       "fake-kimi",
		stdinPipe:        fakeStdin,
		process:          newFakeKimiProcessManager(stdoutR, io.NopCloser(nil), fakeStdin, fakeCmd),
		turnGuard:        base.NewTurnGuard(),
	}
	return s, stdoutW, fakeStdin
}

func TestKimiSession_Run_SendsInitializeAndPrompt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, stdoutW, stdinR := newTestKimiSessionWithPipes(logger)

	// Write Wire events to stdout so the pump can read them.
	go func() {
		stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"event","params":{"type":"TurnBegin","payload":{"user_input":"hello"}}}` + "\n"))
		stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":"Hi!"}}}` + "\n"))
		stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"event","params":{"type":"TurnEnd","payload":{}}}` + "\n"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := sess.Run(ctx, "hello", nil, nil, "")
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}

	// Verify initialize and prompt were sent via stdin.
	stdinText := stdinR.String()

	if !strings.Contains(stdinText, `"method":"initialize"`) {
		t.Error("expected initialize request in stdin")
	}
	if !strings.Contains(stdinText, `"method":"prompt"`) {
		t.Error("expected prompt request in stdin")
	}
}

func TestKimiSession_Interrupt_SendsCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, stdoutW, stdinR := newTestKimiSessionWithPipes(logger)

	go func() {
		// Block briefly then let pump exit.
		time.Sleep(100 * time.Millisecond)
		stdoutW.Close()
	}()

	go func() {
		sess.Run(context.Background(), "hello", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)
	sess.Interrupt(context.Background())

	// Verify cancel was sent via stdin.
	stdinText := stdinR.String()
	if !strings.Contains(stdinText, `"method":"cancel"`) {
		t.Errorf("expected cancel request in stdin, got: %s", stdinText)
	}
}

func TestKimiSession_RespondPermission_SendsApprovalResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, _, stdinR := newTestKimiSessionWithPipes(logger)

	// Simulate an incoming ApprovalRequest to set up pending state.
	sess.RespondPermission("approval-1", protocol.AgentPermissionResponse{Behavior: "allow"})

	// Verify approval response was sent via stdin.
	stdinText := stdinR.String()
	if !strings.Contains(stdinText, `"result":{"request_id":"approval-1","response":"approve"}`) {
		t.Errorf("expected approval response in stdin, got: %s", stdinText)
	}
}

func TestKimiSession_StartTurn_DeliversEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, stdoutW, stdinR := newTestKimiSessionWithPipes(logger)

	go func() {
		stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"event","params":{"type":"TurnBegin","payload":{"user_input":"hello"}}}` + "\n"))
		stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":"Hi!"}}}` + "\n"))
		stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"event","params":{"type":"TurnEnd","payload":{}}}` + "\n"))
		stdoutW.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := sess.StartTurn(ctx, "hello", nil, nil)
	if err != nil {
		t.Fatalf("StartTurn() = %v", err)
	}

	var gotThreadStarted, gotText, gotTurnEnd bool
	done := false
	for !done {
		select {
		case evt, ok := <-ch:
			if !ok {
				done = true
				break
			}
			switch se := evt.Event.(type) {
			case protocol.ThreadStartedStreamEvent:
				gotThreadStarted = true
			case protocol.TimelineStreamEvent:
				if se.Item.Type == "assistant_message" {
					gotText = true
				}
			case protocol.TurnCompletedStreamEvent:
				gotTurnEnd = true
				done = true
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for events")
		}
	}

	if !gotThreadStarted {
		t.Error("expected thread_started event")
	}
	if !gotText {
		t.Error("expected text timeline event")
	}
	if !gotTurnEnd {
		t.Error("expected turn_completed event")
	}

	// Verify initialize and prompt were sent via stdin.
	stdinText := stdinR.String()
	if !strings.Contains(stdinText, `"method":"initialize"`) {
		t.Error("expected initialize request in stdin")
	}
	if !strings.Contains(stdinText, `"method":"prompt"`) {
		t.Error("expected prompt request in stdin")
	}
}

func TestKimiSession_Run_RejectsConcurrentRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, _, _ := newTestKimiSessionWithPipes(logger)

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	go func() {
		sess.Run(ctx1, "first", nil, nil, "")
	}()

	time.Sleep(50 * time.Millisecond)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	_, err := sess.Run(ctx2, "second", nil, nil, "")
	if err == nil {
		t.Fatal("expected concurrent Run to fail, got nil")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected 'already active' error, got: %v", err)
	}
}

func TestKimiSession_StartTurn_RejectsConcurrentStartTurn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, _, _ := newTestKimiSessionWithPipes(logger)

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	go func() {
		sess.StartTurn(ctx1, "first", nil, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	_, err := sess.StartTurn(ctx2, "second", nil, nil)
	if err == nil {
		t.Fatal("expected concurrent StartTurn to fail, got nil")
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Fatalf("expected 'already active' error, got: %v", err)
	}
}
