package agent

import (
	"context"
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

func TestKimiWireTranslator_ContentPartText(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	wireEvent := `{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":"Hello world"}}}`
	events, isTerminal, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if isTerminal {
		t.Error("expected non-terminal")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt, ok := events[0].(AgentStreamEvent)
	if !ok {
		t.Fatalf("expected AgentStreamEvent, got %T", events[0])
	}
	payload, ok := evt.Event.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map event, got %T", evt.Event)
	}
	if payload["type"] != "timeline" {
		t.Errorf("type = %q, want timeline", payload["type"])
	}
	item, ok := payload["item"].(TimelineItem)
	if !ok {
		t.Fatalf("expected TimelineItem, got %T", payload["item"])
	}
	if item.Type != "assistant_message" {
		t.Errorf("item.Type = %q, want assistant_message", item.Type)
	}
	if item.Text != "Hello world" {
		t.Errorf("item.Text = %q, want Hello world", item.Text)
	}
}

func TestKimiWireTranslator_ContentPartThink(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	wireEvent := `{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"think","think":"Let me think..."}}}`
	events, _, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0].(AgentStreamEvent)
	payload := evt.Event.(map[string]interface{})
	item := payload["item"].(TimelineItem)
	if item.Type != "reasoning" {
		t.Errorf("item.Type = %q, want reasoning", item.Type)
	}
	if item.Text != "Let me think..." {
		t.Errorf("item.Text = %q, want Let me think...", item.Text)
	}
}

func TestKimiWireTranslator_ToolCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	wireEvent := `{"jsonrpc":"2.0","method":"event","params":{"type":"ToolCall","payload":{"type":"function","id":"tc-1","function":{"name":"read_file","arguments":"{\"path\":\"main.py\"}"}}}}`
	events, _, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0].(AgentStreamEvent)
	payload := evt.Event.(map[string]interface{})
	item := payload["item"].(TimelineItem)
	if item.Type != "tool_call" {
		t.Errorf("item.Type = %q, want tool_call", item.Type)
	}
	if item.CallID != "tc-1" {
		t.Errorf("item.CallID = %q, want tc-1", item.CallID)
	}
	if item.Name != "read_file" {
		t.Errorf("item.Name = %q, want read_file", item.Name)
	}
	if item.Status != "running" {
		t.Errorf("item.Status = %q, want running", item.Status)
	}
}

func TestKimiWireTranslator_TurnBegin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	wireEvent := `{"jsonrpc":"2.0","method":"event","params":{"type":"TurnBegin","payload":{"user_input":"test prompt"}}}`
	events, isTerminal, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if isTerminal {
		t.Error("expected non-terminal")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0].(AgentStreamEvent)
	payload := evt.Event.(map[string]interface{})
	if payload["type"] != "thread_started" {
		t.Errorf("type = %q, want thread_started", payload["type"])
	}
	if payload["provider"] != "kimi" {
		t.Errorf("provider = %q, want kimi", payload["provider"])
	}
}

func TestKimiWireTranslator_TurnEnd(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	wireEvent := `{"jsonrpc":"2.0","method":"event","params":{"type":"TurnEnd","payload":{}}}`
	events, isTerminal, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if !isTerminal {
		t.Error("expected terminal")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0].(AgentStreamEvent)
	payload := evt.Event.(map[string]interface{})
	if payload["type"] != "turn_completed" {
		t.Errorf("type = %q, want turn_completed", payload["type"])
	}
}

func TestKimiWireTranslator_IgnoresNonEventMethods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	// prompt response — not an event notification
	wireEvent := `{"jsonrpc":"2.0","id":"2","result":{"status":"finished"}}`
	events, isTerminal, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if isTerminal {
		t.Error("expected non-terminal")
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestKimiWireTranslator_UnknownEventType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	tr := &kimiWireTranslator{session: sess}

	wireEvent := `{"jsonrpc":"2.0","method":"event","params":{"type":"UnknownEvent","payload":{}}}`
	events, _, err := tr.Translate([]byte(wireEvent), time.Now())
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for unknown type, got %d", len(events))
	}
}

// --- RED Phase: kimiWireTerminalDetector tests ---

func TestKimiWireTerminalDetector_IsTerminal_TurnCompleted(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := newKimiSession("fake-kimi", &protocol.AgentSessionConfig{}, logger)
	det := &kimiWireTerminalDetector{session: sess}

	evt := AgentStreamEvent{
		Event: map[string]interface{}{
			"type":     "turn_completed",
			"provider": "kimi",
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
		Event: map[string]interface{}{
			"type":     "timeline",
			"provider": "kimi",
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
			payload, ok := evt.Event.(map[string]interface{})
			if !ok {
				continue
			}
			switch payload["type"] {
			case "thread_started":
				gotThreadStarted = true
			case "timeline":
				if item, ok := payload["item"].(TimelineItem); ok {
					if item.Type == "assistant_message" {
						gotText = true
					}
				}
			case "turn_completed":
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
