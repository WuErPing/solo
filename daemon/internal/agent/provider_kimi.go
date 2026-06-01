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
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

const kimiProviderName = "kimi"

// KimiAgentClient implements AgentClient for the Kimi Code CLI Wire mode.
type KimiAgentClient struct {
	binaryPath string
	logger     *slog.Logger
}

// NewKimiAgentClient creates a new Kimi agent client.
func NewKimiAgentClient(binaryPath string, logger *slog.Logger) *KimiAgentClient {
	if binaryPath == "" {
		if p, err := base.FindBinary("kimi-cli", "KIMI_PATH", []string{
			"$HOME/.local/bin/kimi-cli",
			"/usr/local/bin/kimi-cli",
			"/opt/homebrew/bin/kimi-cli",
		}); err == nil {
			binaryPath = p
		}
	}
	return &KimiAgentClient{
		binaryPath: binaryPath,
		logger:     logger.With("provider", kimiProviderName),
	}
}

func (c *KimiAgentClient) Provider() string { return kimiProviderName }

func (c *KimiAgentClient) IsAvailable(ctx context.Context) error {
	if c.binaryPath == "" {
		return fmt.Errorf("kimi binary not found")
	}
	if _, err := os.Stat(c.binaryPath); err != nil {
		return fmt.Errorf("kimi binary not accessible: %w", err)
	}
	return nil
}

func (c *KimiAgentClient) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	return newKimiSession(c.binaryPath, config, c.logger), nil
}

func (c *KimiAgentClient) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	config := &protocol.AgentSessionConfig{
		Provider: kimiProviderName,
	}
	if handle != nil {
		if cwd, ok := handle.Metadata["cwd"].(string); ok {
			config.Cwd = cwd
		}
		if model, ok := handle.Metadata["model"].(string); ok && model != "" {
			config.Model = &model
		}
	}
	sess := newKimiSession(c.binaryPath, config, c.logger)
	if handle != nil && handle.NativeHandle != "" {
		sess.base.SetSessionID(handle.NativeHandle)
	} else if handle != nil && handle.SessionID != "" {
		sess.base.SetSessionID(handle.SessionID)
	}
	return sess, nil
}

func (c *KimiAgentClient) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	models, err := c.fetchModelsFromConfig()
	if err == nil && len(models) > 0 {
		return models, nil
	}
	if err != nil {
		c.logger.Warn("failed to fetch models from config, falling back to static list", "error", err)
	}
	return kimiModels(), nil
}

func (c *KimiAgentClient) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	return kimiModes(), nil
}

func (c *KimiAgentClient) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}
	// Static list based on Kimi CLI v1.43.0 --wire initialize response.
	// TODO: dynamically fetch from initialize handshake for version accuracy.
	return []protocol.AgentSlashCommand{
		{Name: "init", Description: "Analyze the codebase and generate an AGENTS.md file", ArgumentHint: ""},
		{Name: "compact", Description: "Compact the context (optionally with a custom focus)", ArgumentHint: "[focus]"},
		{Name: "clear", Description: "Clear the context", ArgumentHint: ""},
		{Name: "yolo", Description: "Toggle YOLO mode (auto-approve all actions)", ArgumentHint: ""},
		{Name: "afk", Description: "Toggle afk mode (auto-dismiss AskUserQuestion, auto-approve tool calls)", ArgumentHint: ""},
		{Name: "plan", Description: "Toggle plan mode", ArgumentHint: "[on|off|view|clear]"},
		{Name: "add-dir", Description: "Add a directory to the workspace", ArgumentHint: "[path]"},
		{Name: "export", Description: "Export current session context to a markdown file", ArgumentHint: ""},
		{Name: "import", Description: "Import context from a file or session ID", ArgumentHint: "<source>"},
	}, nil
}

type kimiConfig struct {
	DefaultModel string                    `toml:"default_model"`
	Models       map[string]kimiModelEntry `toml:"models"`
}

type kimiModelEntry struct {
	Model        string   `toml:"model"`
	DisplayName  string   `toml:"display_name"`
	MaxContext   int      `toml:"max_context_size"`
	Capabilities []string `toml:"capabilities"`
}

func (c *KimiAgentClient) fetchModelsFromConfig() ([]protocol.AgentModelDefinition, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(home, ".kimi", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg kimiConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Models) == 0 {
		return nil, fmt.Errorf("no models found in config")
	}

	var models []protocol.AgentModelDefinition
	// Always include an Auto option.
	models = append(models, protocol.AgentModelDefinition{
		Provider:    kimiProviderName,
		ID:          "auto",
		Label:       "Auto",
		Description: "Use Kimi's default model",
		IsDefault:   true,
	})
	for key, entry := range cfg.Models {
		label := entry.DisplayName
		if label == "" {
			label = entry.Model
		}
		isDefault := key == cfg.DefaultModel
		models = append(models, protocol.AgentModelDefinition{
			Provider:    kimiProviderName,
			ID:          key,
			Label:       label,
			Description: fmt.Sprintf("%s (context: %d)", entry.Model, entry.MaxContext),
			IsDefault:   isDefault,
		})
	}
	return models, nil
}

func kimiModels() []protocol.AgentModelDefinition {
	return []protocol.AgentModelDefinition{
		{Provider: kimiProviderName, ID: "auto", Label: "Auto", Description: "Use Kimi's default model", IsDefault: true},
	}
}

func kimiModes() []protocol.AgentMode {
	return []protocol.AgentMode{
		{ID: "default", Label: "Default", Description: "Standard mode"},
		{ID: "bypassPermissions", Label: "Bypass Permissions", Description: "Skip all permission prompts"},
		{ID: "plan", Label: "Plan", Description: "Planning mode"},
	}
}

// --- Kimi Session ---

type kimiSession struct {
	mu sync.Mutex

	base        *base.BaseSession
	dispatcher  *base.ChannelDispatcher
	permissions *base.PermissionManager
	process     processManager
	binaryPath  string

	cmd        *exec.Cmd
	stdoutPipe io.ReadCloser
	stdinPipe  io.WriteCloser
	stderrPipe io.ReadCloser

	activeTurnID    string
	nextTurnOrdinal int
}

func newKimiSession(binaryPath string, config *protocol.AgentSessionConfig, logger *slog.Logger) *kimiSession {
	return &kimiSession{
		base:        base.NewBaseSession(kimiProviderName, config, logger),
		dispatcher:  base.NewChannelDispatcher(logger),
		permissions: base.NewPermissionManager(),
		process:     base.NewProcessManager(binaryPath, logger),
		binaryPath:  binaryPath,
	}
}

func (s *kimiSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) (*AgentRunResult, error) {
	runCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	if s.activeTurnID != "" {
		s.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("a turn is already active (turnID: %s)", s.activeTurnID)
	}
	s.nextTurnOrdinal++
	turnID := fmt.Sprintf("kimi-turn-%d", s.nextTurnOrdinal)
	s.activeTurnID = turnID
	s.base.SetCancelFn(cancel)

	if err := s.startProcessLocked(runCtx, text); err != nil {
		s.activeTurnID = ""
		s.mu.Unlock()
		cancel()
		return nil, err
	}
	stdoutPipe := s.stdoutPipe
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.activeTurnID = ""
		s.mu.Unlock()
		cancel()
	}()

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(kimiProviderName)
	translator := &kimiWireTranslator{session: s}
	detector := &kimiWireTerminalDetector{session: s}

	result, err := pump.RunBlocking(runCtx, stdoutPipe, translator, detector)
	if err != nil {
		return nil, err
	}

	return &AgentRunResult{
		SessionID: s.base.SessionID(),
		Canceled:  result.Canceled,
	}, nil
}

func (s *kimiSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	runCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	if s.activeTurnID != "" {
		s.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("a turn is already active (turnID: %s)", s.activeTurnID)
	}
	s.nextTurnOrdinal++
	turnID := fmt.Sprintf("kimi-turn-%d", s.nextTurnOrdinal)
	s.activeTurnID = turnID
	s.base.SetCancelFn(cancel)

	if err := s.startProcessLocked(runCtx, text); err != nil {
		s.activeTurnID = ""
		s.mu.Unlock()
		cancel()
		return nil, err
	}
	stdoutPipe := s.stdoutPipe
	s.mu.Unlock()

	baseCh := s.dispatcher.Subscribe()
	ch := make(chan AgentStreamEvent, 256)
	go func() {
		for evt := range baseCh {
			if e, ok := evt.(AgentStreamEvent); ok {
				ch <- e
			}
		}
		close(ch)
	}()

	pump := base.NewEventPump(s.base.Logger(), s.dispatcher)
	pump.SetProvider(kimiProviderName)
	translator := &kimiWireTranslator{session: s}
	detector := &kimiWireTerminalDetector{session: s}
	go func() {
		_, _ = pump.RunBlocking(runCtx, stdoutPipe, translator, detector)
		s.dispatcher.Unsubscribe(baseCh)
	}()

	return ch, nil
}

func (s *kimiSession) Subscribe() <-chan AgentStreamEvent {
	baseCh := s.dispatcher.Events()
	ch := make(chan AgentStreamEvent, 256)
	go func() {
		for evt := range baseCh {
			if e, ok := evt.(AgentStreamEvent); ok {
				ch <- e
			}
		}
		close(ch)
	}()
	return ch
}

func (s *kimiSession) Interrupt(ctx context.Context) error {
	s.mu.Lock()
	s.base.Cancel()
	if s.stdinPipe != nil {
		s.writeJSONRPCRequest("cancel", nil)
	}
	s.activeTurnID = ""
	s.mu.Unlock()

	s.dispatcher.Emit(AgentStreamEvent{
		Event: map[string]interface{}{
			"type":     "turn_canceled",
			"provider": kimiProviderName,
			"reason":   "user_cancel",
		},
		Timestamp: time.Now(),
	})
	return nil
}

func (s *kimiSession) Close() error {
	if s.base.IsClosed() {
		return nil
	}

	s.mu.Lock()
	s.activeTurnID = ""
	s.mu.Unlock()

	s.base.Close()
	if s.process != nil {
		_ = s.process.Kill(s.cmd)
		if s.cmd != nil {
			_, _ = s.process.WaitForExit(s.cmd)
		}
	}
	s.dispatcher.Close()
	return nil
}

func (s *kimiSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	behavior := "reject"
	if response.Behavior == "allow" {
		behavior = "approve"
	}
	resp := map[string]interface{}{
		"request_id": requestID,
		"response":   behavior,
	}
	if response.Message != "" {
		resp["feedback"] = response.Message
	}
	return s.writeJSONRPCResponse(requestID, resp)
}

// startProcessLocked starts a new kimi --wire process and sends initialize + prompt.
// Must be called with s.mu held.
func (s *kimiSession) startProcessLocked(ctx context.Context, prompt string) error {
	args := s.buildArgs()
	stdout, stderr, stdin, cmd, err := s.process.Start(ctx, args, s.base.Config().Cwd, os.Environ())
	if err != nil {
		return err
	}

	s.stdoutPipe = stdout
	s.stderrPipe = stderr
	s.stdinPipe = stdin
	s.cmd = cmd

	go s.process.DrainStderr(stderr)

	// Send initialize handshake.
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      "init-1",
		"params": map[string]interface{}{
			"protocol_version": "1.10",
			"client": map[string]interface{}{
				"name":    "solo",
				"version": "0.1.0",
			},
			"capabilities": map[string]interface{}{
				"supports_question":  true,
				"supports_plan_mode": true,
			},
		},
	}
	if err := s.writeJSONRPC(initReq); err != nil {
		return fmt.Errorf("send initialize: %w", err)
	}

	// Send prompt request.
	promptReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "prompt",
		"id":      "prompt-1",
		"params": map[string]interface{}{
			"user_input": prompt,
		},
	}
	if err := s.writeJSONRPC(promptReq); err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}

	return nil
}

func (s *kimiSession) buildArgs() []string {
	args := []string{"--wire"}

	config := s.base.Config()
	if config != nil && config.Cwd != "" {
		args = append(args, "--work-dir", config.Cwd)
	}

	model := s.base.CurrentModel()
	if model != "" && model != "auto" {
		args = append(args, "--model", model)
	}

	mode := s.base.CurrentMode()
	if mode == "plan" {
		args = append(args, "--plan")
	}
	if mode == "bypassPermissions" {
		args = append(args, "--yolo")
	}

	if s.base.SessionID() != "" {
		args = append(args, "--session", s.base.SessionID())
	}

	return args
}

func (s *kimiSession) writeJSONRPC(req map[string]interface{}) error {
	if s.stdinPipe == nil {
		return fmt.Errorf("stdin pipe not available")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.stdinPipe.Write(data)
	return err
}

func (s *kimiSession) writeJSONRPCRequest(method string, params interface{}) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	return s.writeJSONRPC(req)
}

func (s *kimiSession) writeJSONRPCResponse(id string, result interface{}) error {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	return s.writeJSONRPC(resp)
}

func (s *kimiSession) GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error) {
	return s.base.GetRuntimeInfo(), nil
}

func (s *kimiSession) GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error) {
	return kimiModes(), nil
}

func (s *kimiSession) GetCurrentMode(ctx context.Context) (*string, error) {
	return s.base.GetCurrentModePtr(), nil
}

func (s *kimiSession) SetMode(modeID string) error {
	return s.base.SetMode(modeID)
}

func (s *kimiSession) SetModel(modelID string) error {
	return s.base.SetModel(modelID)
}

func (s *kimiSession) SetThinkingOption(optionID string) error {
	return s.base.SetThinkingOption(optionID)
}

func (s *kimiSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return s.base.DescribePersistence()
}

func (s *kimiSession) GetPendingPermissions() []interface{} {
	return nil
}

func (s *kimiSession) ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	// Return the same static list as the client-level command list.
	// TODO: cache commands parsed from the initialize response for accuracy.
	return []protocol.AgentSlashCommand{
		{Name: "init", Description: "Analyze the codebase and generate an AGENTS.md file", ArgumentHint: ""},
		{Name: "compact", Description: "Compact the context (optionally with a custom focus)", ArgumentHint: "[focus]"},
		{Name: "clear", Description: "Clear the context", ArgumentHint: ""},
		{Name: "yolo", Description: "Toggle YOLO mode (auto-approve all actions)", ArgumentHint: ""},
		{Name: "afk", Description: "Toggle afk mode (auto-dismiss AskUserQuestion, auto-approve tool calls)", ArgumentHint: ""},
		{Name: "plan", Description: "Toggle plan mode", ArgumentHint: "[on|off|view|clear]"},
		{Name: "add-dir", Description: "Add a directory to the workspace", ArgumentHint: "[path]"},
		{Name: "export", Description: "Export current session context to a markdown file", ArgumentHint: ""},
		{Name: "import", Description: "Import context from a file or session ID", ArgumentHint: "<source>"},
	}, nil
}

func (s *kimiSession) StreamHistory(ctx context.Context) ([]AgentStreamEvent, error) {
	return nil, nil
}

// --- Wire Translator ---

type kimiWireTranslator struct {
	session *kimiSession
}

func (t *kimiWireTranslator) Translate(raw []byte, timestamp time.Time) ([]interface{}, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}

	var msg struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, false, fmt.Errorf("parse JSON-RPC: %w", err)
	}

	if msg.Method != "event" {
		return nil, false, nil
	}

	var events []interface{}
	isTerminal := false

	switch msg.Params.Type {
	case "TurnBegin":
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":      "thread_started",
				"sessionId": t.session.base.SessionID(),
				"provider":  kimiProviderName,
			},
			Timestamp: timestamp,
		})

	case "ContentPart":
		var payload struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Think string `json:"think"`
		}
		json.Unmarshal(msg.Params.Payload, &payload)

		itemType := "assistant_message"
		text := payload.Text
		if payload.Type == "think" {
			itemType = "reasoning"
			text = payload.Think
		}
		if text != "" {
			events = append(events, AgentStreamEvent{
				Event: map[string]interface{}{
					"type":     "timeline",
					"item":     TimelineItem{Type: itemType, Text: text},
					"provider": kimiProviderName,
				},
				Timestamp: timestamp,
			})
		}

	case "ToolCall":
		var payload struct {
			Type     string `json:"type"`
			ID       string `json:"id"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		json.Unmarshal(msg.Params.Payload, &payload)
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "timeline",
				"item":     TimelineItem{Type: "tool_call", CallID: payload.ID, Name: payload.Function.Name, Status: "running"},
				"provider": kimiProviderName,
			},
			Timestamp: timestamp,
		})

	case "ToolResult":
		var payload struct {
			ToolCallID  string `json:"tool_call_id"`
			ReturnValue struct {
				IsError bool `json:"is_error"`
			} `json:"return_value"`
		}
		json.Unmarshal(msg.Params.Payload, &payload)
		status := "completed"
		if payload.ReturnValue.IsError {
			status = "failed"
		}
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "timeline",
				"item":     TimelineItem{Type: "tool_call", CallID: payload.ToolCallID, Status: status},
				"provider": kimiProviderName,
			},
			Timestamp: timestamp,
		})

	case "TurnEnd":
		isTerminal = true
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "turn_completed",
				"provider": kimiProviderName,
			},
			Timestamp: timestamp,
		})

	case "ApprovalRequest":
		var payload struct {
			ID          string `json:"id"`
			ToolCallID  string `json:"tool_call_id"`
			Sender      string `json:"sender"`
			Action      string `json:"action"`
			Description string `json:"description"`
		}
		json.Unmarshal(msg.Params.Payload, &payload)
		request := map[string]interface{}{
			"id":       payload.ID,
			"provider": kimiProviderName,
			"name":     payload.Sender,
			"kind":     "tool",
			"title":    payload.Action,
			"input": map[string]interface{}{
				"tool":        payload.Sender,
				"description": payload.Description,
			},
			"detail": map[string]interface{}{
				"type":        payload.Sender,
				"description": payload.Description,
			},
		}
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "permission_requested",
				"provider": kimiProviderName,
				"request":  request,
			},
			Timestamp: timestamp,
		})

	case "CompactionBegin":
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "timeline",
				"item":     TimelineItem{Type: "compaction", CompactionStatus: "loading"},
				"provider": kimiProviderName,
			},
			Timestamp: timestamp,
		})

	case "CompactionEnd":
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "timeline",
				"item":     TimelineItem{Type: "compaction", CompactionStatus: "completed"},
				"provider": kimiProviderName,
			},
			Timestamp: timestamp,
		})

	case "StepRetry":
		var payload struct {
			N           int    `json:"n"`
			NextAttempt int    `json:"next_attempt"`
			MaxAttempts int    `json:"max_attempts"`
			WaitS       int    `json:"wait_s"`
			ErrorType   string `json:"error_type"`
		}
		json.Unmarshal(msg.Params.Payload, &payload)
		events = append(events, AgentStreamEvent{
			Event: map[string]interface{}{
				"type":     "timeline",
				"item":     TimelineItem{Type: "error", Text: fmt.Sprintf("Step %d retry %d/%d after %ds: %s", payload.N, payload.NextAttempt, payload.MaxAttempts, payload.WaitS, payload.ErrorType)},
				"provider": kimiProviderName,
			},
			Timestamp: timestamp,
		})
	}

	return events, isTerminal, nil
}

// --- Terminal Detector ---

type kimiWireTerminalDetector struct {
	session *kimiSession
}

func (d *kimiWireTerminalDetector) IsTerminal(evt interface{}) (*base.AgentRunResult, error, bool) {
	streamEvt, ok := evt.(AgentStreamEvent)
	if !ok {
		return nil, nil, false
	}
	payload, ok := streamEvt.Event.(map[string]interface{})
	if !ok {
		return nil, nil, false
	}
	eventType, _ := payload["type"].(string)
	if eventType == "turn_completed" {
		return &base.AgentRunResult{}, nil, true
	}
	return nil, nil, false
}
