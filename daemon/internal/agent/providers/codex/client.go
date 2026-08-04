// Package codex implements the OpenAI Codex CLI agent provider.
package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/daemon/internal/agent/providers/streamevents"
	"github.com/WuErPing/solo/protocol"
)

// processManager abstracts process lifecycle for testability.
type processManager interface {
	Start(ctx context.Context, args []string, cwd string, env []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error)
	Stop(cmd *exec.Cmd, timeout time.Duration) error
	Interrupt(cmd *exec.Cmd) error
	Kill(cmd *exec.Cmd) error
	DrainStderr(stderr io.ReadCloser)
	WaitForExit(cmd *exec.Cmd) (int, error)
}

const codexProviderName = "codex"

// Client implements AgentClient for the OpenAI Codex CLI.
type Client struct {
	binaryPath string
	logger     *slog.Logger
}

// NewClient creates a new Codex agent client.
func NewClient(binaryPath string, logger *slog.Logger) *Client {
	if binaryPath == "" {
		if p, err := base.FindBinary("codex", "CODEX_PATH", []string{
			"$HOME/.npm-global/bin/codex",
			"$HOME/.local/bin/codex",
			"/usr/local/bin/codex",
			"/opt/homebrew/bin/codex",
		}); err == nil {
			binaryPath = p
		}
	}
	return &Client{binaryPath: binaryPath, logger: logger}
}

func (c *Client) Provider() string { return codexProviderName }

func (c *Client) IsAvailable(_ context.Context) error {
	if c.binaryPath == "" {
		return fmt.Errorf("codex binary not found")
	}
	if _, err := os.Stat(c.binaryPath); err != nil {
		return fmt.Errorf("codex binary not accessible: %w", err)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	return newCodexSession(c.binaryPath, config, c.logger), nil
}

func (c *Client) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}

	config := &protocol.AgentSessionConfig{
		Provider: codexProviderName,
	}

	if cwd, ok := handle.Metadata["cwd"].(string); ok {
		config.Cwd = cwd
	}
	if model, ok := handle.Metadata["model"].(string); ok && model != "" {
		config.Model = &model
	}

	session := newCodexSession(c.binaryPath, config, c.logger)
	sessionID := handle.NativeHandle
	if sessionID == "" {
		sessionID = handle.SessionID
	}
	session.base.SetSessionID(sessionID)
	return session, nil
}

func (c *Client) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return codexModels(), nil
}

func (c *Client) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return codexModes(), nil
}

func (c *Client) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

// --- Models & Modes ---

func codexModels() []protocol.AgentModelDefinition {
	return []protocol.AgentModelDefinition{
		{Provider: codexProviderName, ID: "gpt-5.5", Label: "GPT-5.5", Description: "Latest default model", IsDefault: true},
		{Provider: codexProviderName, ID: "gpt-5.5-pro", Label: "GPT-5.5 Pro", Description: "Maximum reasoning quality"},
		{Provider: codexProviderName, ID: "gpt-5.4", Label: "GPT-5.4", Description: "Previous default, most capable"},
		{Provider: codexProviderName, ID: "gpt-5.4-mini", Label: "GPT-5.4 Mini", Description: "Lower-cost testing and lighter workflows"},
		{Provider: codexProviderName, ID: "gpt-5.3-codex", Label: "GPT-5.3 Codex", Description: "Agentic coding and tool-heavy workflows"},
		{Provider: codexProviderName, ID: "gpt-5.2", Label: "GPT-5.2", Description: "Coding-optimized model"},
	}
}

func codexModes() []protocol.AgentMode {
	return []protocol.AgentMode{
		{ID: "auto", Label: "Auto", Description: "Managed sandbox with workspace-write access", Icon: "ShieldAlert", ColorTier: "moderate"},
		{ID: "full-access", Label: "Full Access", Description: "Full system access without sandboxing", Icon: "ShieldOff", ColorTier: "dangerous"},
	}
}

// --- Session ---

type codexSession struct {
	mu         sync.Mutex
	base       *base.BaseSession
	dispatcher *base.ChannelDispatcher
	process    processManager
	binaryPath string
	cmd        *exec.Cmd
	stdoutPipe io.ReadCloser
	stdinPipe  io.WriteCloser
	stderrPipe io.ReadCloser
	turnGuard  *base.TurnGuard
}

func newCodexSession(binaryPath string, config *protocol.AgentSessionConfig, logger *slog.Logger) *codexSession {
	return &codexSession{
		base:       base.NewBaseSession(codexProviderName, config, logger),
		dispatcher: base.NewChannelDispatcher(logger),
		process:    base.NewProcessManager(binaryPath, logger),
		binaryPath: binaryPath,
		turnGuard:  base.NewTurnGuard(),
	}
}

func (s *codexSession) Run(ctx context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, messageID string) (*agent.AgentRunResult, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if _, err := s.turnGuard.Acquire(); err != nil {
		return nil, err
	}
	s.base.SetCancelFn(cancel)
	defer func() {
		s.turnGuard.Release()
		cancel()
	}()

	if err := s.startProcessLocked(runCtx, text); err != nil {
		return nil, err
	}

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(codexProviderName)
	translator := newCodexTranslator(s.base.Logger(), messageID, text)
	translator.session = s
	detector := streamevents.TerminalDetector{}

	result, err := pump.RunBlocking(runCtx, s.stdoutPipe, translator, detector)
	if err != nil {
		return nil, err
	}

	return &agent.AgentRunResult{
		SessionID: s.base.SessionID(),
		FinalText: translator.finalText(),
		Usage:     translator.lastUsage(),
		Canceled:  result.Canceled,
	}, nil
}

func (s *codexSession) StartTurn(ctx context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment) (<-chan agent.AgentStreamEvent, error) {
	runCtx, cancel := context.WithCancel(ctx)

	if _, err := s.turnGuard.Acquire(); err != nil {
		cancel()
		return nil, err
	}
	s.base.SetCancelFn(cancel)

	if err := s.startProcessLocked(runCtx, text); err != nil {
		s.turnGuard.Release()
		cancel()
		return nil, err
	}

	baseCh := s.dispatcher.Subscribe()
	ch := make(chan agent.AgentStreamEvent, 256)

	go func() {
		defer func() {
			s.turnGuard.Release()
			cancel()
			s.dispatcher.Unsubscribe(baseCh)
		}()
		for evt := range baseCh {
			if se, ok := evt.(agent.AgentStreamEvent); ok {
				ch <- se
			}
		}
		close(ch)
	}()

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(codexProviderName)
	translator := newCodexTranslator(s.base.Logger(), "", text)
	translator.session = s
	detector := streamevents.TerminalDetector{}

	go func() {
		_, _ = pump.RunBlocking(runCtx, s.stdoutPipe, translator, detector)
	}()

	return ch, nil
}

func (s *codexSession) startProcessLocked(ctx context.Context, prompt string) error {
	args := s.buildArgs(prompt, s.base.SessionID())
	cwd := ""
	if cfg := s.base.Config(); cfg != nil {
		cwd = cfg.Cwd
	}

	stdout, stderr, stdin, cmd, err := s.process.Start(ctx, args, cwd, s.buildEnv())
	if err != nil {
		return fmt.Errorf("start codex process: %w", err)
	}

	s.cmd = cmd
	s.stdoutPipe = stdout
	s.stdinPipe = stdin
	s.stderrPipe = stderr

	// Close stdin — codex exec reads prompt from args, not stdin.
	_ = stdin.Close()

	go s.process.DrainStderr(stderr)

	// Health check: if process exits immediately, surface the error.
	time.Sleep(100 * time.Millisecond)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		exitCode, waitErr := s.process.WaitForExit(cmd)
		if exitCode != 0 {
			s.process.DrainStderr(stderr)
			return fmt.Errorf("codex exited immediately with code %d: %v", exitCode, waitErr)
		}
	}

	return nil
}

func (s *codexSession) buildArgs(prompt string, sessionID string) []string {
	var args []string

	// codex-cli 0.146: JSON output is the documented `--json` flag
	// (`--experimental-json` still works as a hidden legacy alias but is not
	// accepted by the top-level `resume` command). Resume is a subcommand of
	// `exec` and takes the prompt positionally after the session id —
	// omitting it silently drops the turn's text.
	if sessionID != "" {
		args = []string{"exec", "resume", sessionID, prompt, "--json", "--ephemeral", "--skip-git-repo-check"}
	} else {
		args = []string{"exec", "--json", "--ephemeral", "--skip-git-repo-check"}
	}

	// Sandbox mode based on current mode
	mode := s.base.CurrentMode()
	if mode != "full-access" {
		args = append(args, "--sandbox", "workspace-write")
	}

	// Model
	model := s.base.CurrentModel()
	if model != "" {
		args = append(args, "--model", model)
	}

	// Prompt (only for exec, not resume — resume got it above)
	if sessionID == "" {
		args = append(args, prompt)
	}

	return args
}

func (s *codexSession) buildEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	blocked := map[string]bool{
		"CLAUDECODE":                                true,
		"CLAUDE_CODE_ENTRYPOINT":                    true,
		"CLAUDE_CODE_SSE_PORT":                      true,
		"CLAUDE_AGENT_SDK_VERSION":                  true,
		"CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING": true,
	}
	for _, e := range env {
		key := e
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			key = e[:idx]
		}
		if !blocked[key] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (s *codexSession) Subscribe() <-chan agent.AgentStreamEvent {
	baseCh := s.dispatcher.Events()
	ch := make(chan agent.AgentStreamEvent, 256)
	go func() {
		for evt := range baseCh {
			if se, ok := evt.(agent.AgentStreamEvent); ok {
				ch <- se
			}
		}
		close(ch)
	}()
	return ch
}

func (s *codexSession) Interrupt(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.base.Cancel()
	if s.cmd != nil {
		_ = s.process.Interrupt(s.cmd)
	}
	s.turnGuard.Release()
	s.dispatcher.Emit(agent.AgentStreamEvent{
		AgentID:   s.base.SessionID(),
		Event:     protocol.TurnCanceledStreamEvent{Reason: "interrupted"},
		Timestamp: time.Now(),
	})
	return nil
}

func (s *codexSession) Close() error {
	if s.base.IsClosed() {
		return nil
	}
	s.turnGuard.Release()
	_ = s.base.Close()

	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()

	if cmd != nil {
		_ = s.process.Kill(cmd)
		_, _ = s.process.WaitForExit(cmd)
	}
	s.dispatcher.Close()
	return nil
}

func (s *codexSession) RespondPermission(_ string, _ protocol.AgentPermissionResponse) error {
	return nil // Codex exec does not support interactive permissions
}

func (s *codexSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	return s.base.GetRuntimeInfo(), nil
}

func (s *codexSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return codexModes(), nil
}

func (s *codexSession) GetCurrentMode(_ context.Context) (*string, error) {
	return s.base.GetCurrentModePtr(), nil
}

func (s *codexSession) SetMode(modeID string) error {
	return s.base.SetMode(modeID)
}

func (s *codexSession) SetModel(modelID string) error {
	return s.base.SetModel(modelID)
}

func (s *codexSession) SetThinkingOption(optionID string) error {
	return s.base.SetThinkingOption(optionID)
}

func (s *codexSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return s.base.DescribePersistence()
}

func (s *codexSession) GetPendingPermissions() []interface{} {
	return nil
}

func (s *codexSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

func (s *codexSession) StreamHistory(_ context.Context) ([]agent.AgentStreamEvent, error) {
	return nil, nil
}

// --- Translator ---

type codexTranslator struct {
	logger    *slog.Logger
	session   *codexSession // set by Run/StartTurn; nil in translator-only tests
	messageID string
	prompt    string
	threadStarted  bool
	userMsgEmitted bool
	textBuf        string
	usage          *protocol.AgentUsage
}

func newCodexTranslator(logger *slog.Logger, messageID, prompt string) *codexTranslator {
	return &codexTranslator{
		logger:    logger,
		messageID: messageID,
		prompt:    prompt,
	}
}

func (t *codexTranslator) finalText() string {
	return t.textBuf
}

func (t *codexTranslator) lastUsage() *protocol.AgentUsage {
	return t.usage
}

func (t *codexTranslator) Translate(raw []byte, now time.Time) ([]interface{}, bool, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, false, fmt.Errorf("parse codex JSON: %w", err)
	}

	typ, _ := event["type"].(string)
	if typ == "" {
		return nil, false, nil
	}

	b := streamevents.New(codexProviderName, now)

	switch typ {
	// --- Current schema (codex-cli >= 0.146; verified against
	// testdata/real_stream_v0.146.ndjson, a real stdout capture) ---

	case "thread.started":
		threadID, _ := event["thread_id"].(string)
		// Write the real thread id back to the session so subsequent turns
		// resume natively (`codex exec resume <id>`) — same pattern as the
		// claude init message and pi session event writebacks.
		if threadID != "" && t.session != nil {
			t.session.base.SetSessionID(threadID)
		}
		b.ThreadStarted(threadID)
		t.threadStarted = true

	case "turn.started":
		// Synthesize user_message event (codex exec does not echo the prompt)
		if !t.userMsgEmitted && t.messageID != "" {
			b.UserMessage(t.prompt, t.messageID)
			t.userMsgEmitted = true
		}

	case "item.started":
		t.translateItem(b, event, false)

	case "item.completed":
		t.translateItem(b, event, true)

	case "turn.completed":
		if usage := parseCodexUsage(event["usage"]); usage != nil {
			t.usage = usage
			b.Usage(usage)
		}
		b.TurnCompleted(t.usage)

	case "turn.failed":
		msg := "turn failed"
		if e, ok := event["error"].(map[string]interface{}); ok {
			if m, ok := e["message"].(string); ok && m != "" {
				msg = m
			}
		}
		b.TurnFailed(msg)

	case "error":
		// Non-fatal diagnostic line; the terminal failure follows as
		// turn.failed. Log only.
		if t.logger != nil {
			t.logger.Warn("codex error event", "event", event)
		}

	// --- Legacy schema (codex-cli < 0.146 experimental JSON). Kept for
	// backward compatibility with older installed binaries. ---

	case "TurnStartedNotification":
		b.ThreadStarted("")
		t.threadStarted = true

		// Synthesize user_message event (codex exec does not echo the prompt)
		if !t.userMsgEmitted && t.messageID != "" {
			b.UserMessage(t.prompt, t.messageID)
			t.userMsgEmitted = true
		}

	case "AgentMessageDeltaNotification":
		delta, _ := event["delta"].(string)
		if delta != "" {
			t.textBuf += delta
			b.AssistantMessage(delta)
		}

	case "ReasoningTextDeltaNotification":
		delta, _ := event["delta"].(string)
		b.Reasoning(delta)

	case "LocalShellCall", "FunctionCall":
		callID, _ := event["call_id"].(string)
		name, _ := event["name"].(string)
		var args interface{}
		if a, ok := event["arguments"]; ok {
			args = a
		}
		b.ToolCall(callID, name, buildCodexToolCallDetail(args), "running")

	case "FunctionCallOutput", "CustomToolCallOutput":
		callID, _ := event["call_id"].(string)
		b.ToolCall(callID, "", nil, "completed")

	case "ThreadTokenUsageUpdatedNotification":
		usage := &protocol.AgentUsage{}
		if v, ok := event["input_tokens"].(float64); ok {
			usage.InputTokens = &v
		}
		if v, ok := event["output_tokens"].(float64); ok {
			usage.OutputTokens = &v
		}
		if v, ok := event["cached_input_tokens"].(float64); ok {
			usage.CachedInputTokens = &v
		}
		t.usage = usage
		b.Usage(usage)

	case "TurnCompletedNotification":
		b.TurnCompleted(t.usage)

	case "TurnAbortedNotification":
		reason, _ := event["reason"].(string)
		if reason == "" {
			reason = "turn aborted"
		}
		b.TurnFailed(reason)

	default:
		// Unknown event type — ignore gracefully
	}

	return b.Events(), b.Terminal(), nil
}

// translateItem handles item.started / item.completed events (current
// schema). Assistant text arrives whole on item.completed — there are no
// delta events in the current schema.
func (t *codexTranslator) translateItem(b *streamevents.Builder, event map[string]interface{}, completed bool) {
	item, _ := event["item"].(map[string]interface{})
	if item == nil {
		return
	}
	itemType, _ := item["type"].(string)
	id, _ := item["id"].(string)

	switch itemType {
	case "agent_message":
		if !completed {
			return
		}
		text := codexItemText(item["text"])
		if text != "" {
			t.textBuf += text
			b.AssistantMessage(text)
		}

	case "reasoning":
		if !completed {
			return
		}
		b.Reasoning(codexItemText(item["text"]))

	case "error":
		// Diagnostic item (e.g. model metadata fallback); the terminal
		// failure arrives as turn.failed. Log only.
		if t.logger != nil {
			t.logger.Warn("codex error item", "item", item)
		}

	default:
		// Tool-call-like items: command_execution, file_change,
		// mcp_tool_call, web_search, etc.
		status := "running"
		if completed {
			status = "completed"
		}
		b.ToolCall(id, itemType, protocol.UnknownDetail{Type: "codex_tool_call", Input: item}, status)
	}
}

// codexItemText normalizes an item text field: a plain string, or a list of
// strings (some codex versions emit reasoning text as an array).
func codexItemText(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, p := range t {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

// parseCodexUsage converts the usage object attached to turn.completed.
func parseCodexUsage(v interface{}) *protocol.AgentUsage {
	m, _ := v.(map[string]interface{})
	if m == nil {
		return nil
	}
	usage := &protocol.AgentUsage{}
	if n, ok := m["input_tokens"].(float64); ok {
		usage.InputTokens = &n
	}
	if n, ok := m["cached_input_tokens"].(float64); ok {
		usage.CachedInputTokens = &n
	}
	if n, ok := m["output_tokens"].(float64); ok {
		usage.OutputTokens = &n
	}
	return usage
}

func buildCodexToolCallDetail(args interface{}) protocol.ToolCallDetail {
	if args == nil {
		return protocol.UnknownDetail{Type: "codex_tool_call", Input: "null"}
	}
	switch v := args.(type) {
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(v), &m); err == nil {
			return protocol.UnknownDetail{Type: "codex_tool_call", Input: m}
		}
		return protocol.UnknownDetail{Type: "codex_tool_call", Input: v}
	default:
		return protocol.UnknownDetail{Type: "codex_tool_call", Input: v}
	}
}

// --- Terminal Detector ---
//
// Terminal detection is handled by the shared streamevents.TerminalDetector.
