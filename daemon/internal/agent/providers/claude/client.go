// Package claude implements the Claude Code CLI agent provider.
package claude

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

const (
	claudeProviderName = "claude"
	claudeModelAuto    = "auto"
)

// Client implements AgentClient for the Claude Code CLI.
type Client struct {
	binaryPath string
	logger     *slog.Logger
}

func NewClient(binaryPath string, logger *slog.Logger) *Client {
	if binaryPath == "" {
		if p, err := findClaudeBinary(); err == nil {
			binaryPath = p
		}
	}
	return &Client{
		binaryPath: binaryPath,
		logger:     logger.With("provider", claudeProviderName),
	}
}

func (c *Client) Provider() string { return claudeProviderName }

func (c *Client) IsAvailable(_ context.Context) error {
	if c.binaryPath == "" {
		return fmt.Errorf("claude binary not found")
	}
	if _, err := os.Stat(c.binaryPath); err != nil {
		return fmt.Errorf("claude binary not accessible: %w", err)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	return newClaudeSession(c.binaryPath, config, c.logger), nil
}

func (c *Client) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	config := &protocol.AgentSessionConfig{
		Provider: claudeProviderName,
	}
	if handle != nil {
		sid := handle.NativeHandle
		if sid == "" {
			sid = handle.SessionID
		}
		if cwd, ok := handle.Metadata["cwd"].(string); ok {
			config.Cwd = cwd
		}
		if model, ok := handle.Metadata["model"].(string); ok && model != "" {
			config.Model = strPtr(model)
		}
		sess := newClaudeSession(c.binaryPath, config, c.logger)
		sess.base.SetSessionID(sid)
		sess.resuming = true
		return sess, nil
	}
	return nil, fmt.Errorf("no persistence handle provided")
}

func (c *Client) ListModels(_ context.Context, _ string) ([]protocol.AgentModelDefinition, error) {
	return claudeModels(), nil
}

func (c *Client) ListModes(_ context.Context, _ string) ([]protocol.AgentMode, error) {
	return claudeModes(), nil
}

func (c *Client) ListClientCommands(_ context.Context, _ string) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

// processManager abstracts process lifecycle for testability.
type processManager interface {
	Start(ctx context.Context, args []string, cwd string, env []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error)
	Stop(cmd *exec.Cmd, timeout time.Duration) error
	Interrupt(cmd *exec.Cmd) error
	Kill(cmd *exec.Cmd) error
	DrainStderr(stderr io.ReadCloser)
	WaitForExit(cmd *exec.Cmd) (int, error)
}

// --- Claude Session ---

type claudeSession struct {
	mu sync.Mutex

	base        *base.BaseSession
	dispatcher  *base.ChannelDispatcher
	permissions *base.PermissionManager
	process     processManager

	binaryPath string
	resuming   bool

	cmd        *exec.Cmd
	stdoutPipe io.ReadCloser
	stderrPipe io.ReadCloser
	stdinPipe  io.WriteCloser

	// Turn tracking
	turnGuard *base.TurnGuard

	// Accumulated usage tracking across turns (matches OpenCode's accumulatedUsage)
	accumulatedUsage *protocol.AgentUsage
}

func newClaudeSession(binaryPath string, config *protocol.AgentSessionConfig, logger *slog.Logger) *claudeSession {
	s := &claudeSession{
		binaryPath:       binaryPath,
		base:             base.NewBaseSession(claudeProviderName, config, logger),
		dispatcher:       base.NewChannelDispatcher(logger),
		permissions:      base.NewPermissionManager(),
		process:          base.NewProcessManager(binaryPath, logger),
		turnGuard:        base.NewTurnGuard(),
		accumulatedUsage: &protocol.AgentUsage{},
	}
	return s
}

// accumulateUsage merges per-turn usage into the accumulated total (matches OpenCode behavior).
func (s *claudeSession) accumulateUsage(turn *protocol.AgentUsage) {
	if turn == nil {
		return
	}
	a := s.accumulatedUsage
	if turn.InputTokens != nil {
		if a.InputTokens == nil {
			a.InputTokens = new(float64)
		}
		*a.InputTokens += *turn.InputTokens
	}
	if turn.OutputTokens != nil {
		if a.OutputTokens == nil {
			a.OutputTokens = new(float64)
		}
		*a.OutputTokens += *turn.OutputTokens
	}
	if turn.CachedInputTokens != nil {
		if a.CachedInputTokens == nil {
			a.CachedInputTokens = new(float64)
		}
		*a.CachedInputTokens += *turn.CachedInputTokens
	}
	if turn.TotalCostUSD != nil {
		if a.TotalCostUSD == nil {
			a.TotalCostUSD = new(float64)
		}
		*a.TotalCostUSD += *turn.TotalCostUSD
	}
}

func (s *claudeSession) Run(ctx context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment, messageID string) (*agent.AgentRunResult, error) {
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

	// Clear turn when done.
	defer func() {
		s.turnGuard.Release()
		cancel()
	}()

	// Proactively emit thread_started and user_message so all connected
	// clients see the turn start and prompt immediately, even if the CLI
	// doesn't echo them back. The translator skips its own emission via flags.
	now := time.Now()
	s.dispatcher.Emit(agent.AgentStreamEvent{
		Event: protocol.ThreadStartedStreamEvent{
			Provider:  claudeProviderName,
			SessionID: s.base.SessionID(),
		},
		Timestamp: now,
	})
	if text != "" {
		s.dispatcher.Emit(agent.AgentStreamEvent{
			Event: protocol.TimelineStreamEvent{
				Item:     protocol.TimelineItem{Type: "user_message", Text: text, MessageID: messageID},
				Provider: claudeProviderName,
			},
			Timestamp: now,
		})
	}

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(claudeProviderName)
	translator := &claudeTranslator{
		session:               s,
		streamedContentBlocks: make(map[int]int),
		messageID:             messageID,
		threadStartedEmitted:  true,
		userMessageEmitted:    text != "",
	}
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

func (s *claudeSession) StartTurn(ctx context.Context, text string, _ []protocol.ImageAttachment, _ []protocol.AgentAttachment) (<-chan agent.AgentStreamEvent, error) {
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
	pump.SetProvider(claudeProviderName)
	translator := &claudeTranslator{session: s, streamedContentBlocks: make(map[int]int), messageID: ""}
	detector := streamevents.TerminalDetector{}
	pump.RunBackground(runCtx, stdoutPipe, translator, detector)

	return ch, nil
}

func (s *claudeSession) Subscribe() <-chan agent.AgentStreamEvent {
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

// startProcessLocked starts a new Claude CLI process.
// Must be called with s.mu held.
func (s *claudeSession) startProcessLocked(ctx context.Context, prompt string) error {
	args := s.buildArgs(prompt)
	stdout, stderr, stdin, cmd, err := s.process.Start(ctx, args, s.base.Config().Cwd, s.buildEnv())
	if err != nil {
		return err
	}

	s.stdinPipe = stdin
	s.stdoutPipe = stdout
	s.stderrPipe = stderr
	s.cmd = cmd

	go s.process.DrainStderr(stderr)

	return nil
}

func (s *claudeSession) buildArgs(prompt string) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
	}

	model := s.base.CurrentModel()
	if model != "" && model != claudeModelAuto {
		args = append(args, "--model", model)
	}

	mode := s.base.CurrentMode()
	if mode != "" {
		args = append(args, "--permission-mode", mode)
	}

	thinking := s.base.CurrentThinking()
	if thinking != "" {
		args = append(args, "--effort", thinking)
	}

	if s.base.SessionID() != "" {
		args = append(args, "--resume", s.base.SessionID())
	}

	config := s.base.Config()
	if config != nil && config.SystemPrompt != "" {
		args = append(args, "--system-prompt", config.SystemPrompt)
	}
	if config != nil && len(config.OutputSchema) > 0 {
		schemaJSON, _ := json.Marshal(config.OutputSchema)
		args = append(args, "--json-schema", string(schemaJSON))
	}
	if len(config.McpServers) > 0 {
		mcpJSON, _ := json.Marshal(config.McpServers)
		args = append(args, "--mcp-config", string(mcpJSON))
	}

	if prompt != "" {
		args = append(args, prompt)
	}

	return args
}

func (s *claudeSession) buildEnv() []string {
	env := os.Environ()

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

	filtered = append(filtered,
		"MCP_TIMEOUT=600000",
		"MCP_TOOL_TIMEOUT=600000",
	)

	return filtered
}

// --- Session Interface Methods ---

func (s *claudeSession) Interrupt(_ context.Context) error {
	s.mu.Lock()
	s.base.Cancel()
	if err := s.process.Interrupt(s.cmd); err != nil {
		s.base.Logger().Warn("failed to interrupt claude process", "error", err)
	}
	s.mu.Unlock()
	s.turnGuard.Release()

	s.dispatcher.Emit(agent.AgentStreamEvent{
		Event: protocol.TurnCanceledStreamEvent{
			Provider: claudeProviderName,
			Reason:   "user_cancel",
		},
		Timestamp: time.Now(),
	})

	return nil
}

func (s *claudeSession) Close() error {
	if s.base.IsClosed() {
		return nil
	}

	s.turnGuard.Release()

	closeErr := s.base.Close()

	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()

	if cmd != nil {
		if err := s.process.Kill(cmd); err != nil {
			s.base.Logger().Warn("failed to kill claude process", "error", err)
		}
		// Ensure process is reaped to prevent zombies
		if _, err := s.process.WaitForExit(cmd); err != nil {
			s.base.Logger().Debug("claude process wait result", "error", err)
		}
	}
	s.permissions.RejectAll()
	s.dispatcher.Close()

	return closeErr
}

func (s *claudeSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	// Write permission response to Claude CLI stdin
	if s.stdinPipe != nil {
		behavior := "deny"
		if response.Behavior == "allow" {
			behavior = "allow"
		}
		permResp := map[string]interface{}{
			"type":      "permission_response",
			"requestId": requestID,
			"behavior":  behavior,
		}
		if response.Message != "" {
			permResp["message"] = response.Message
		}
		data, _ := json.Marshal(permResp)
		data = append(data, '\n')
		if _, err := s.stdinPipe.Write(data); err != nil {
			s.base.Logger().Warn("failed to write permission response to stdin", "error", err)
		}
	}
	return s.permissions.Respond(requestID, response)
}

func (s *claudeSession) GetRuntimeInfo(_ context.Context) (*protocol.AgentRuntimeInfo, error) {
	return s.base.GetRuntimeInfo(), nil
}

func (s *claudeSession) GetAvailableModes(_ context.Context) ([]protocol.AgentMode, error) {
	return claudeModes(), nil
}

func (s *claudeSession) GetCurrentMode(_ context.Context) (*string, error) {
	return s.base.GetCurrentModePtr(), nil
}

func (s *claudeSession) SetMode(modeID string) error {
	return s.base.SetMode(modeID)
}

func (s *claudeSession) SetModel(modelID string) error {
	return s.base.SetModel(modelID)
}

func (s *claudeSession) SetThinkingOption(optionID string) error {
	return s.base.SetThinkingOption(optionID)
}

func (s *claudeSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return s.base.DescribePersistence()
}

func (s *claudeSession) GetPendingPermissions() []interface{} {
	return s.permissions.GetPending()
}

func (s *claudeSession) ListCommands(_ context.Context) ([]protocol.AgentSlashCommand, error) {
	return nil, nil
}

func (s *claudeSession) StreamHistory(_ context.Context) ([]agent.AgentStreamEvent, error) {
	return nil, nil
}

type claudeTranslator struct {
	session   *claudeSession
	messageID string
	// streamedContentBlocks tracks the total character length of content that
	// has been emitted via stream_event messages for each block index.
	// When the subsequent assistant message arrives, blocks at these indices
	// are compared: if the assistant has more content than what was streamed,
	// the remaining difference is emitted as a recovery. If the content is
	// the same (or shorter), the block is skipped to avoid duplication.
	streamedContentBlocks map[int]int
	// threadStartedEmitted is set when Run() proactively emits
	// ThreadStartedStreamEvent so the translator skips its own emission
	// on the "init" system message.
	threadStartedEmitted bool
	// userMessageEmitted is set when Run() proactively emits user_message
	// so the translator skips its own emission if the CLI echoes it back.
	userMessageEmitted bool
}

func (t *claudeTranslator) Translate(raw []byte, timestamp time.Time) ([]interface{}, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}

	var msg sdkMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, false, fmt.Errorf("parse SDK message: %w", err)
	}

	if msg.Type == "keep_alive" {
		return nil, false, nil
	}

	events := t.translateMessage(msg, timestamp)
	isTerminal := msg.Type == "result"
	return events, isTerminal, nil
}

func (t *claudeTranslator) translateMessage(msg sdkMessage, now time.Time) []interface{} {
	switch msg.Type {
	case "system":
		return t.translateSystemMessage(msg, now)
	case "user":
		return t.translateUserMessage(msg, now, t.messageID)
	case "assistant":
		return t.translateAssistantMessage(msg, now)
	case "stream_event":
		return t.translateStreamEvent(msg, now)
	case "result":
		return t.translateResultMessage(msg, now)
	case "tool_progress":
		return streamevents.New(claudeProviderName, now).
			ToolCall(msg.ToolUseID, msg.ToolName, nil, "running").Events()
	case "permission_request":
		return t.translatePermissionRequest(msg, now)
	}
	return nil
}

func (t *claudeTranslator) translateSystemMessage(msg sdkMessage, now time.Time) []interface{} {
	b := streamevents.New(claudeProviderName, now)

	switch msg.Subtype {
	case "init":
		t.session.base.SetSessionID(msg.SessionID)
		t.session.base.SetCurrentModel(msg.Model)
		t.session.base.SetCurrentMode(msg.PermissionMode)

		if !t.threadStartedEmitted {
			b.ThreadStarted(msg.SessionID)
		}

	case "status":
		if msg.Status == "compacting" {
			b.Timeline(protocol.TimelineItem{Type: "compaction", CompactionStatus: "loading"})
		}

	case "compact_boundary":
		trigger := ""
		preTokens := 0
		if msg.CompactMetadata != nil {
			trigger = msg.CompactMetadata.Trigger
			preTokens = msg.CompactMetadata.PreTokens
		}
		b.Timeline(protocol.TimelineItem{Type: "compaction", CompactionStatus: "completed", Trigger: trigger, PreTokens: preTokens})

	case "task_notification":
		status := "completed"
		if msg.TaskStatus == "failed" {
			status = "failed"
		}
		b.Timeline(protocol.TimelineItem{Type: "tool_call", CallID: msg.TaskID, Name: "task", Detail: protocol.PlainTextDetail{Type: "plain_text", Text: msg.Summary}, Status: status})
		// Also emit as a todo item (matches OpenCode's todo.updated behavior)
		if msg.Summary != "" {
			b.Timeline(protocol.TimelineItem{Type: "todo", TodoItems: []protocol.TodoItem{{Text: msg.Summary, Completed: status == "completed"}}})
		}
	}

	return b.Events()
}

func (t *claudeTranslator) translateUserMessage(msg sdkMessage, now time.Time, messageID string) []interface{} {
	if t.userMessageEmitted {
		return nil
	}
	if msg.Message == nil {
		return nil
	}

	var userMsg sdkUserMessage
	if err := json.Unmarshal(msg.Message, &userMsg); err != nil {
		return nil
	}

	var textParts []string
	for _, c := range userMsg.Content {
		if c.Type == "text" && c.Text != "" {
			textParts = append(textParts, c.Text)
		}
	}
	if len(textParts) == 0 {
		return nil
	}

	return streamevents.New(claudeProviderName, now).
		UserMessage(strings.Join(textParts, "\n"), messageID).Events()
}

func (t *claudeTranslator) translateAssistantMessage(msg sdkMessage, now time.Time) []interface{} {
	if msg.Message == nil {
		return nil
	}

	var assistantMsg sdkAssistantMessage
	if err := json.Unmarshal(msg.Message, &assistantMsg); err != nil {
		return nil
	}

	b := streamevents.New(claudeProviderName, now)
	for i, block := range assistantMsg.Content {
		streamedLen := t.streamedContentBlocks[i]
		delete(t.streamedContentBlocks, i)

		switch block.Type {
		case "text":
			if block.Text != "" {
				// If the assistant has more content than what was streamed
				// via deltas, emit the remaining difference as a recovery.
				// If content matches (or is shorter), skip to avoid duplication.
				if streamedLen > 0 && streamedLen < len(block.Text) {
					b.AssistantMessage(block.Text[streamedLen:])
				} else if streamedLen == 0 {
					b.AssistantMessage(block.Text)
				}
			}
		case "thinking":
			if block.Thinking != "" {
				// Same recovery logic for thinking blocks.
				if streamedLen > 0 && streamedLen < len(block.Thinking) {
					b.Reasoning(block.Thinking[streamedLen:])
				} else if streamedLen == 0 {
					b.Reasoning(block.Thinking)
				}
			}
		case "tool_use":
			// Parse tool input into structured detail (matches OpenCode's deriveToolCallDetail)
			var toolInput interface{}
			if block.Input != nil {
				_ = json.Unmarshal(block.Input, &toolInput)
			}
			detail := base.DeriveToolCallDetail(block.Name, toolInput, nil)
			b.ToolCall(block.ID, block.Name, detail, "completed")
		}
	}

	return b.Events()
}

func (t *claudeTranslator) translateStreamEvent(msg sdkMessage, now time.Time) []interface{} {
	if msg.Event == nil {
		return nil
	}

	var streamEvt sdkStreamEvent
	if err := json.Unmarshal(msg.Event, &streamEvt); err != nil {
		return nil
	}

	b := streamevents.New(claudeProviderName, now)

	switch streamEvt.Type {
	case "content_block_start":
		if streamEvt.ContentBlock != nil {
			switch streamEvt.ContentBlock.Type {
			case "text":
				if streamEvt.ContentBlock.Text != "" {
					t.streamedContentBlocks[streamEvt.Index] += len(streamEvt.ContentBlock.Text)
					b.AssistantMessage(streamEvt.ContentBlock.Text)
				}
			case "thinking":
				if streamEvt.ContentBlock.Thinking != "" {
					t.streamedContentBlocks[streamEvt.Index] += len(streamEvt.ContentBlock.Thinking)
					b.Reasoning(streamEvt.ContentBlock.Thinking)
				}
			case "tool_use":
				b.ToolCall(streamEvt.ContentBlock.ID, streamEvt.ContentBlock.Name, nil, "running")
			}
		}

	case "content_block_delta":
		if streamEvt.Delta != nil {
			switch streamEvt.Delta.Type {
			case "text_delta":
				if streamEvt.Delta.Text != "" {
					t.streamedContentBlocks[streamEvt.Index] += len(streamEvt.Delta.Text)
					b.AssistantMessage(streamEvt.Delta.Text)
				}
			case "thinking_delta":
				if streamEvt.Delta.Thinking != "" {
					t.streamedContentBlocks[streamEvt.Index] += len(streamEvt.Delta.Thinking)
					b.Reasoning(streamEvt.Delta.Thinking)
				}
			case "input_json_delta":
				// Partial tool input - wait for full block
			}
		}

	case "message_delta":
		// Turn completed naturally when Delta.StopReason == "end_turn"; nothing to emit.

	case "content_block_stop":
		b.Raw(protocol.FlushSignalStreamEvent{Type: "flush_signal"})

	case "message_start", "message_stop":
		// No action needed
	}

	return b.Events()
}

func (t *claudeTranslator) translateResultMessage(msg sdkMessage, now time.Time) []interface{} {
	b := streamevents.New(claudeProviderName, now)

	if msg.Subtype == "success" {
		var usage *protocol.AgentUsage
		if msg.Usage != nil {
			inputT := msg.Usage.InputTokens
			outputT := msg.Usage.OutputTokens
			cachedT := msg.Usage.CacheReadInputTokens
			cost := msg.TotalCostUSD
			usage = &protocol.AgentUsage{
				InputTokens:       &inputT,
				OutputTokens:      &outputT,
				CachedInputTokens: &cachedT,
				TotalCostUSD:      &cost,
			}

			// Accumulate usage across turns (matches OpenCode behavior)
			t.session.accumulateUsage(usage)

			// Emit usage_updated event (matches OpenCode behavior)
			b.Usage(t.session.accumulatedUsage)
		}
		b.TurnCompleted(usage)
	} else {
		errMsg := "unknown error"
		if len(msg.Errors) > 0 {
			errMsg = strings.Join(msg.Errors, "; ")
		}
		b.TurnFailed(errMsg)
	}

	return b.Events()
}

// translatePermissionRequest handles permission_request events from the Claude SDK.
// Claude CLI in --print mode emits these when a tool requires user approval.
func (t *claudeTranslator) translatePermissionRequest(msg sdkMessage, now time.Time) []interface{} {
	requestID := msg.UUID
	if requestID == "" {
		requestID = msg.ToolUseID
	}
	if requestID == "" {
		return nil
	}

	// Register pending permission with the session's permission manager
	t.session.permissions.Register(requestID)

	// Build structured detail (matches OpenCode's permission detail format)
	detail := map[string]interface{}{"type": "unknown"}
	toolName := msg.ToolName
	input := map[string]interface{}{}

	if toolName != "" {
		detail = map[string]interface{}{"type": toolName}
		input["tool"] = toolName
	}
	if msg.Description != "" {
		input["description"] = msg.Description
		detail["description"] = msg.Description
	}
	if msg.Summary != "" {
		input["summary"] = msg.Summary
	}
	if msg.Message != nil {
		var msgData interface{}
		_ = json.Unmarshal(msg.Message, &msgData)
		if msgData != nil {
			input["message"] = msgData
		}
	}

	request := protocol.PermissionRequest{
		ID:       requestID,
		Provider: claudeProviderName,
		Name:     toolName,
		Kind:     "tool",
		Title:    base.HumanReadablePermission(toolName),
		Input:    input,
		Detail:   detail,
	}

	return streamevents.New(claudeProviderName, now).PermissionRequested(request).Events()
}

// --- Terminal Detector ---
//
// Terminal detection is handled by the shared streamevents.TerminalDetector.

// --- SDK Types ---

type sdkMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`

	SessionID      string `json:"session_id,omitempty"`
	PermissionMode string `json:"permissionMode,omitempty"`
	Model          string `json:"model,omitempty"`
	ClaudeVersion  string `json:"claude_code_version,omitempty"`

	Message         json.RawMessage `json:"message,omitempty"`
	ParentToolUseID *string         `json:"parent_tool_use_id,omitempty"`

	Result       string    `json:"result,omitempty"`
	IsError      bool      `json:"is_error,omitempty"`
	DurationMs   float64   `json:"duration_ms,omitempty"`
	NumTurns     int       `json:"num_turns,omitempty"`
	TotalCostUSD float64   `json:"total_cost_usd,omitempty"`
	Usage        *sdkUsage `json:"usage,omitempty"`
	Errors       []string  `json:"errors,omitempty"`

	Event json.RawMessage `json:"event,omitempty"`

	Status string `json:"status,omitempty"`

	TaskID      string `json:"task_id,omitempty"`
	TaskStatus  string `json:"task_status,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`

	CompactMetadata *sdkCompactMetadata `json:"compact_metadata,omitempty"`

	ToolUseID string `json:"tool_use_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`

	UUID string `json:"uuid,omitempty"`
}

type sdkUsage struct {
	InputTokens              float64 `json:"input_tokens"`
	OutputTokens             float64 `json:"output_tokens"`
	CacheReadInputTokens     float64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens float64 `json:"cache_creation_input_tokens"`
}

type sdkCompactMetadata struct {
	Trigger   string `json:"trigger"`
	PreTokens int    `json:"pre_tokens"`
}

type sdkContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
}

type sdkAssistantMessage struct {
	Role    string            `json:"role"`
	Content []sdkContentBlock `json:"content"`
	Model   string            `json:"model,omitempty"`
}

type sdkStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`

	ContentBlock *sdkContentBlock `json:"content_block,omitempty"`
	Delta        *sdkStreamDelta  `json:"delta,omitempty"`
}

type sdkStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type sdkUserMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type sdkUserMessage struct {
	Role    string                  `json:"role"`
	Content []sdkUserMessageContent `json:"content"`
}

// --- Binary Finder ---

func strPtr(s string) *string { return &s }

func findClaudeBinary() (string, error) {
	return base.FindBinary("claude", "CLAUDE_CODE_PATH", []string{
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
		"$HOME/.local/bin/claude",
		"$HOME/.npm-global/bin/claude",
	})
}

// --- Model and Mode Definitions ---

func claudeModels() []protocol.AgentModelDefinition {
	return []protocol.AgentModelDefinition{
		{
			Provider: claudeProviderName, ID: claudeModelAuto, Label: "Auto", Description: "Use Claude's default model", IsDefault: true,
			ThinkingOptions: []protocol.AgentSelectOption{
				{ID: "low", Label: "Low"},
				{ID: "medium", Label: "Medium", IsDefault: true},
				{ID: "high", Label: "High"},
				{ID: "max", Label: "Max"},
			},
			DefaultThinkingOptionID: "medium",
		},
		{
			Provider: claudeProviderName, ID: "claude-sonnet-4-6", Label: "Sonnet 4.6",
			ThinkingOptions: []protocol.AgentSelectOption{
				{ID: "low", Label: "Low"},
				{ID: "medium", Label: "Medium", IsDefault: true},
				{ID: "high", Label: "High"},
				{ID: "max", Label: "Max"},
			},
			DefaultThinkingOptionID: "medium",
		},
		{
			Provider: claudeProviderName, ID: "claude-opus-4-7", Label: "Opus 4.7",
			ThinkingOptions: []protocol.AgentSelectOption{
				{ID: "low", Label: "Low"},
				{ID: "medium", Label: "Medium", IsDefault: true},
				{ID: "high", Label: "High"},
				{ID: "max", Label: "Max"},
			},
			DefaultThinkingOptionID: "medium",
		},
		{
			Provider: claudeProviderName, ID: "claude-haiku-4-5", Label: "Haiku 4.5",
			ThinkingOptions: []protocol.AgentSelectOption{
				{ID: "low", Label: "Low"},
				{ID: "medium", Label: "Medium", IsDefault: true},
				{ID: "high", Label: "High"},
				{ID: "max", Label: "Max"},
			},
			DefaultThinkingOptionID: "medium",
		},
	}
}

func claudeModes() []protocol.AgentMode {
	return []protocol.AgentMode{
		{ID: "default", Label: "Default", Description: "Always ask for permission", Icon: "ShieldCheck", ColorTier: "safe"},
		{ID: "acceptEdits", Label: "Accept Edits", Description: "Auto-accept file edits", Icon: "ShieldAlert", ColorTier: "moderate"},
		{ID: "plan", Label: "Plan", Description: "Planning mode", Icon: "ShieldCheck", ColorTier: "planning"},
		{ID: "bypassPermissions", Label: "Bypass Permissions", Description: "Skip all permission prompts", Icon: "ShieldOff", ColorTier: "dangerous"},
	}
}
