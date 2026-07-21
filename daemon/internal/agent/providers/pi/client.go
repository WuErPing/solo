// Package pi implements the Pi terminal coding harness agent provider.
package pi

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

const piProviderName = "pi"

// Client implements AgentClient for the Pi CLI.
type Client struct {
	binaryPath string
	logger     *slog.Logger
}

// NewClient creates a new Pi agent client.
func NewClient(binaryPath string, logger *slog.Logger) *Client {
	if binaryPath == "" {
		if p, err := base.FindBinary("pi", "PI_PATH", []string{
			"$HOME/.bun/bin/pi",
			"$HOME/.local/bin/pi",
			"/usr/local/bin/pi",
			"/opt/homebrew/bin/pi",
		}); err == nil {
			binaryPath = p
		}
	}
	return &Client{
		binaryPath: binaryPath,
		logger:     logger.With("provider", piProviderName),
	}
}

func (c *Client) Provider() string { return piProviderName }

func (c *Client) IsAvailable(_ context.Context) error {
	if c.binaryPath == "" {
		return fmt.Errorf("pi binary not found")
	}
	if _, err := os.Stat(c.binaryPath); err != nil {
		return fmt.Errorf("pi binary not accessible: %w", err)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	return newPiSession(c.binaryPath, config, c.logger), nil
}

func (c *Client) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	config := &protocol.AgentSessionConfig{
		Provider: piProviderName,
	}
	if handle != nil {
		if cwd, ok := handle.Metadata["cwd"].(string); ok {
			config.Cwd = cwd
		}
		if model, ok := handle.Metadata["model"].(string); ok && model != "" {
			config.Model = &model
		}
	}
	sess := newPiSession(c.binaryPath, config, c.logger)
	if handle != nil && handle.NativeHandle != "" {
		sess.base.SetSessionID(handle.NativeHandle)
	} else if handle != nil && handle.SessionID != "" {
		sess.base.SetSessionID(handle.SessionID)
	}
	return sess, nil
}

func (c *Client) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return piModels(), nil
}

func (c *Client) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return piModes(), nil
}

func (c *Client) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

func piModels() []protocol.AgentModelDefinition {
	return []protocol.AgentModelDefinition{
		{Provider: piProviderName, ID: "auto", Label: "Auto", Description: "Use Pi's default model configuration", IsDefault: true},
	}
}

func piModes() []protocol.AgentMode {
	return []protocol.AgentMode{
		{ID: "default", Label: "Default", Description: "Standard mode"},
		{ID: "readOnly", Label: "Read Only", Description: "Read-only mode with no file modifications"},
	}
}

// --- Pi Session ---

type piSession struct {
	mu sync.Mutex

	base       *base.BaseSession
	dispatcher *base.ChannelDispatcher
	process    processManager
	binaryPath string

	cmd        *exec.Cmd
	stdoutPipe io.ReadCloser
	stdinPipe  io.WriteCloser
	stderrPipe io.ReadCloser

	turnGuard *base.TurnGuard
}

// extractAssistantText extracts text content from a Pi assistant message.
func extractAssistantText(msg *piMessage) string {
	if msg == nil || msg.Role != "assistant" {
		return ""
	}
	var parts []string
	for _, c := range msg.Content {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func newPiSession(binaryPath string, config *protocol.AgentSessionConfig, logger *slog.Logger) *piSession {
	return &piSession{
		base:       base.NewBaseSession(piProviderName, config, logger),
		dispatcher: base.NewChannelDispatcher(logger),
		process:    base.NewProcessManager(binaryPath, logger),
		binaryPath: binaryPath,
		turnGuard:  base.NewTurnGuard(),
	}
}

func (s *piSession) Run(ctx context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, messageID string) (*agent.AgentRunResult, error) {
	runCtx, cancel := context.WithCancel(ctx)

	if _, err := s.turnGuard.Acquire(); err != nil {
		cancel()
		return nil, fmt.Errorf("a turn is already active")
	}
	s.base.SetCancelFn(cancel)

	s.mu.Lock()
	if err := s.startProcessLocked(runCtx, text); err != nil {
		s.turnGuard.Release()
		s.mu.Unlock()
		cancel()
		return nil, err
	}
	stdoutPipe := s.stdoutPipe
	s.mu.Unlock()

	defer func() {
		s.turnGuard.Release()
		cancel()
	}()

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(piProviderName)
	translator := &piTranslator{session: s, messageID: messageID}
	detector := streamevents.TerminalDetector{}

	result, err := pump.RunBlocking(runCtx, stdoutPipe, translator, detector)
	if err != nil {
		return nil, err
	}

	var usage *protocol.AgentUsage
	if result.Usage != nil {
		if u, ok := result.Usage.(*protocol.AgentUsage); ok {
			usage = u
		}
	}

	return &agent.AgentRunResult{
		SessionID: s.base.SessionID(),
		FinalText: result.FinalText,
		Usage:     usage,
		Canceled:  result.Canceled,
	}, nil
}

func (s *piSession) StartTurn(ctx context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment) (<-chan agent.AgentStreamEvent, error) {
	runCtx, cancel := context.WithCancel(ctx)

	if _, err := s.turnGuard.Acquire(); err != nil {
		cancel()
		return nil, fmt.Errorf("a turn is already active")
	}
	s.base.SetCancelFn(cancel)

	s.mu.Lock()
	if err := s.startProcessLocked(runCtx, text); err != nil {
		s.turnGuard.Release()
		s.mu.Unlock()
		cancel()
		return nil, err
	}
	stdoutPipe := s.stdoutPipe
	s.mu.Unlock()

	baseCh := s.dispatcher.Subscribe()
	ch := make(chan agent.AgentStreamEvent, 256)
	go func() {
		for evt := range baseCh {
			if e, ok := evt.(agent.AgentStreamEvent); ok {
				ch <- e
			}
		}
		close(ch)
	}()

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(piProviderName)
	translator := &piTranslator{session: s, messageID: ""}
	detector := streamevents.TerminalDetector{}
	go func() {
		_, _ = pump.RunBlocking(runCtx, stdoutPipe, translator, detector)
		s.dispatcher.Unsubscribe(baseCh)
	}()

	return ch, nil
}

func (s *piSession) Subscribe() <-chan agent.AgentStreamEvent {
	baseCh := s.dispatcher.Events()
	ch := make(chan agent.AgentStreamEvent, 256)
	go func() {
		for evt := range baseCh {
			if e, ok := evt.(agent.AgentStreamEvent); ok {
				ch <- e
			}
		}
		close(ch)
	}()
	return ch
}

// startProcessLocked starts a new pi --mode json process.
// Must be called with s.mu held.
func (s *piSession) startProcessLocked(ctx context.Context, prompt string) error {
	args := s.buildArgs(prompt)
	stdout, stderr, stdin, cmd, err := s.process.Start(ctx, args, s.base.Config().Cwd, s.buildEnv())
	if err != nil {
		return err
	}

	s.stdinPipe = stdin
	s.stdoutPipe = stdout
	s.stderrPipe = stderr
	s.cmd = cmd

	// Close stdin immediately — pi -p mode does not read from stdin,
	// and keeping it open causes the process to hang indefinitely.
	if stdin != nil {
		_ = stdin.Close()
	}

	go s.process.DrainStderr(stderr)

	// Health check: wait briefly to detect immediate process crash.
	time.Sleep(100 * time.Millisecond)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		exitCode := cmd.ProcessState.ExitCode()
		if exitCode != 0 {
			stderrBytes, _ := io.ReadAll(stderr)
			s.base.Logger().Error("pi CLI exited immediately",
				"exitCode", exitCode,
				"args", args,
				"stderr", string(stderrBytes))
			return fmt.Errorf("pi CLI exited immediately with code %d: %s", exitCode, string(stderrBytes))
		}
	}

	return nil
}

func (s *piSession) buildArgs(prompt string) []string {
	args := []string{
		"-p", prompt,
		"--mode", "json",
	}

	config := s.base.Config()

	model := s.base.CurrentModel()
	if model != "" && model != "auto" {
		args = append(args, "--model", model)
	}

	mode := s.base.CurrentMode()
	if mode == "readOnly" {
		args = append(args, "--tools", "read,grep,find,ls")
	}

	thinking := s.base.CurrentThinking()
	if thinking != "" {
		args = append(args, "--thinking", thinking)
	}

	if s.base.SessionID() != "" {
		args = append(args, "--session", s.base.SessionID())
	}

	if config != nil && config.SystemPrompt != "" {
		args = append(args, "--system-prompt", config.SystemPrompt)
	}

	return args
}

func (s *piSession) buildEnv() []string {
	env := os.Environ()

	// Strip Solo-specific env vars that might leak into Pi.
	stripped := map[string]bool{
		"CLAUDECODE":                                true,
		"CLAUDE_CODE_ENTRYPOINT":                    true,
		"CLAUDE_CODE_SSE_PORT":                      true,
		"CLAUDE_AGENT_SDK_VERSION":                  true,
		"CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING": true,
	}

	filtered := make([]string, 0, len(env))
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if !stripped[parts[0]] {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

// --- Session Interface Methods ---

func (s *piSession) Interrupt(_ context.Context) error {
	s.mu.Lock()
	s.base.Cancel()
	if err := s.process.Interrupt(s.cmd); err != nil {
		s.base.Logger().Warn("failed to interrupt pi process", "error", err)
	}
	s.mu.Unlock()
	s.turnGuard.Release()

	s.dispatcher.Emit(agent.AgentStreamEvent{
		Event: protocol.TurnCanceledStreamEvent{
			Provider: piProviderName,
			Reason:   "user_cancel",
		},
		Timestamp: time.Now(),
	})
	return nil
}

func (s *piSession) Close() error {
	if s.base.IsClosed() {
		return nil
	}

	s.turnGuard.Release()

	closeErr := s.base.Close()

	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()

	if s.process != nil && cmd != nil {
		_ = s.process.Kill(cmd)
		_, _ = s.process.WaitForExit(cmd)
	}
	s.dispatcher.Close()
	return closeErr
}

func (s *piSession) RespondPermission(_ string, _ protocol.AgentPermissionResponse) error {
	// Pi does not support interactive permission requests via the JSON stream.
	return nil
}

func (s *piSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	return s.base.GetRuntimeInfo(), nil
}

func (s *piSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return piModes(), nil
}

func (s *piSession) GetCurrentMode(_ context.Context) (*string, error) {
	return s.base.GetCurrentModePtr(), nil
}

func (s *piSession) SetMode(modeID string) error {
	return s.base.SetMode(modeID)
}

func (s *piSession) SetModel(modelID string) error {
	return s.base.SetModel(modelID)
}

func (s *piSession) SetThinkingOption(optionID string) error {
	return s.base.SetThinkingOption(optionID)
}

func (s *piSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return s.base.DescribePersistence()
}

func (s *piSession) GetPendingPermissions() []interface{} {
	return nil
}

func (s *piSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

func (s *piSession) StreamHistory(_ context.Context) ([]agent.AgentStreamEvent, error) {
	return nil, nil
}

// --- Pi Translator ---

type piTranslator struct {
	session     *piSession
	textEmitted bool // true if text_delta was emitted for current assistant message
	messageID   string
}

func (t *piTranslator) Translate(raw []byte, timestamp time.Time) ([]interface{}, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}

	var msg piEvent
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, false, fmt.Errorf("parse pi event: %w", err)
	}

	b := streamevents.New(piProviderName, timestamp)
	t.translateEvent(b, msg)
	return b.Events(), b.Terminal(), nil
}

// piEventDispatch maps a Pi event type to its translator handler. Supporting a
// new event type is a one-line table entry rather than a control-flow edit.
// Unlisted types (e.g. "agent_end", a no-op) are ignored.
var piEventDispatch = map[string]func(t *piTranslator, b *streamevents.Builder, msg piEvent){
	"session":        (*piTranslator).onSession,
	"agent_start":    (*piTranslator).onTurnLifecycle,
	"turn_start":     (*piTranslator).onTurnLifecycle,
	"message_start":  (*piTranslator).onMessageStart,
	"message_end":    (*piTranslator).onMessageEnd,
	"message_update": (*piTranslator).onMessageUpdate,
	"turn_end":       (*piTranslator).onTurnEnd,
}

func (t *piTranslator) translateEvent(b *streamevents.Builder, msg piEvent) {
	if handle, ok := piEventDispatch[msg.Type]; ok {
		handle(t, b, msg)
	}
}

func (t *piTranslator) onSession(b *streamevents.Builder, msg piEvent) {
	if msg.ID != "" {
		t.session.base.SetSessionID(msg.ID)
	}
	b.ThreadStarted(msg.ID)
}

// onTurnLifecycle resets per-turn text tracking so the second turn (after a tool
// call) correctly detects whether text was emitted. Handles agent_start/turn_start.
func (t *piTranslator) onTurnLifecycle(_ *streamevents.Builder, _ piEvent) {
	t.textEmitted = false
}

func (t *piTranslator) onMessageStart(b *streamevents.Builder, msg piEvent) {
	if msg.Message == nil {
		return
	}
	if msg.Message.Role == "assistant" {
		t.textEmitted = false
	}
	if msg.Message.Role == "user" {
		if text := userMessageText(msg.Message); text != "" {
			b.UserMessage(text, t.messageID)
		}
	}
}

func (t *piTranslator) onMessageEnd(b *streamevents.Builder, msg piEvent) {
	if msg.Message == nil || msg.Message.Role != "assistant" {
		return
	}
	// Emit the full text if no text_delta was seen for this message.
	if !t.textEmitted {
		b.AssistantMessage(extractAssistantText(msg.Message))
	}
	t.textEmitted = false

	// Accumulate usage from the assistant message_end event.
	if msg.Message.Usage != nil {
		b.Usage(t.buildUsage(msg.Message.Usage))
	}
}

func (t *piTranslator) onMessageUpdate(b *streamevents.Builder, msg piEvent) {
	if msg.AssistantMessageEvent != nil {
		t.translateAssistantMessageEvent(b, msg.AssistantMessageEvent)
	}
}

func (t *piTranslator) onTurnEnd(b *streamevents.Builder, msg piEvent) {
	// A turn_end with stopReason="toolUse" is an intermediate turn — Pi will start
	// another turn with the actual assistant response after the tool runs. Do not
	// emit turn_completed or text for intermediate turns.
	if msg.Message != nil && msg.Message.StopReason == "toolUse" {
		t.textEmitted = false
		return
	}

	// Emit the full text if no text_delta was seen for the final assistant message.
	if !t.textEmitted && msg.Message != nil {
		b.AssistantMessage(extractAssistantText(msg.Message))
	}
	t.textEmitted = false

	b.TurnCompleted(t.turnEndUsage(msg))
}

// turnEndUsage prefers turn-level usage, falling back to the assistant message's.
func (t *piTranslator) turnEndUsage(msg piEvent) *protocol.AgentUsage {
	if usage := t.buildUsage(msg.Usage); usage != nil {
		return usage
	}
	if msg.Message != nil {
		return t.buildUsage(msg.Message.Usage)
	}
	return nil
}

// userMessageText joins the non-empty text parts of a user message.
func userMessageText(m *piMessage) string {
	var parts []string
	for _, c := range m.Content {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (t *piTranslator) translateAssistantMessageEvent(b *streamevents.Builder, evt *piAssistantMessageEvent) {
	switch evt.Type {
	case "thinking_start":
		// No-op; wait for delta.

	case "thinking_delta":
		b.Reasoning(evt.Delta)

	case "text_start":
		// No-op; wait for delta.

	case "text_delta":
		if evt.Delta != "" {
			t.textEmitted = true
			b.AssistantMessage(evt.Delta)
		}

	case "toolcall_start":
		// Pi uses 'toolcall_start' (no underscore) with 'partial' field.
		tc := evt.ToolCall
		if tc == nil && evt.Partial != nil && evt.Partial.ID != "" {
			tc = evt.Partial
		}
		if tc != nil {
			b.ToolCall(tc.ID, tc.Name, t.buildToolCallDetail(tc), "running")
		}

	case "toolcall_delta":
		// Pi toolcall_delta carries incremental arguments in 'delta'.
		// We accumulate them on the session so toolcall_end has the full args.
		// For now, we don't emit intermediate tool_call events.
		// The frontend will see running → completed when toolcall_end arrives.

	case "toolcall_end":
		// Pi uses 'toolcall_end' (no underscore) with 'toolCall' field.
		if evt.ToolCall != nil {
			b.ToolCall(evt.ToolCall.ID, evt.ToolCall.Name, t.buildToolCallDetail(evt.ToolCall), "completed")
		}
	}
}

func (t *piTranslator) buildUsage(u *piUsage) *protocol.AgentUsage {
	if u == nil {
		return nil
	}
	usage := &protocol.AgentUsage{}
	if u.InputTokens > 0 {
		v := float64(u.InputTokens)
		usage.InputTokens = &v
	}
	if u.OutputTokens > 0 {
		v := float64(u.OutputTokens)
		usage.OutputTokens = &v
	}
	if u.CacheRead > 0 {
		v := float64(u.CacheRead)
		usage.CachedInputTokens = &v
	}
	if u.Cost != nil && u.Cost.Total > 0 {
		usage.TotalCostUSD = &u.Cost.Total
	}
	return usage
}

func (t *piTranslator) buildToolCallDetail(tc *piToolCall) protocol.ToolCallDetail {
	var input interface{} = tc.Arguments
	if tc.Arguments != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Arguments), &parsed); err == nil {
			input = parsed
		}
	}
	return protocol.UnknownDetail{
		Type:   "unknown",
		Input:  input,
		Output: nil,
	}
}

// --- Terminal Detector ---
//
// Terminal detection is handled by the shared streamevents.TerminalDetector.

// --- Pi JSON Event Types ---

type piEvent struct {
	Type                  string                   `json:"type"`
	ID                    string                   `json:"id"`
	Cwd                   string                   `json:"cwd"`
	Message               *piMessage               `json:"message"`
	AssistantMessageEvent *piAssistantMessageEvent `json:"assistantMessageEvent"`
	Usage                 *piUsage                 `json:"usage"`
}

type piMessage struct {
	Role       string      `json:"role"`
	Content    []piContent `json:"content"`
	Usage      *piUsage    `json:"usage"`
	StopReason string      `json:"stopReason"`
}

type piContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type piAssistantMessageEvent struct {
	Type     string      `json:"type"`
	Delta    string      `json:"delta"`
	ToolCall *piToolCall `json:"toolCall"`
	Partial  *piToolCall `json:"partial"`
}

type piToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type piUsage struct {
	InputTokens  int     `json:"input"`
	OutputTokens int     `json:"output"`
	CacheRead    int     `json:"cacheRead"`
	CacheWrite   int     `json:"cacheWrite"`
	TotalTokens  int     `json:"totalTokens"`
	Cost         *piCost `json:"cost"`
}

type piCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}
