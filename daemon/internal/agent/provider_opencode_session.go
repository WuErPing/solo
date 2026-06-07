package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

// --- OpenCode Session ---

type openCodeSession struct {
	baseURL       string
	releaseServer func()

	base *base.BaseSession

	mu sync.Mutex

	// Event dispatcher (shared with Claude provider pattern)
	dispatcher *base.ChannelDispatcher

	// Turn ID tracking (matches Solo)
	nextTurnID             int
	activeForegroundTurnID string

	// Permission tracking
	pendingPerms map[string]pendingPermission

	// Event translation state
	messageRoles            map[string]string
	streamedPartKeys        map[string]bool
	emittedStructuredMsgIDs map[string]bool
	partTypes               map[string]string
	runningToolCalls        map[string]timelineItem

	// Usage tracking (matches Solo's accumulatedUsage)
	accumulatedUsage *opencodeUsage

	// MCP state
	mcpConfigured   bool
	mcpSetupPromise chan struct{} // closed when MCP setup completes
	mcpSetupErr     error

	// Modes cache
	availableModesCache []protocol.AgentMode

	// Context window tracking
	modelContextWindows                 map[string]int // "providerID/modelID" -> maxTokens
	selectedModelContextWindowMaxTokens int            // resolved from config model or assistant message

	// Commands cache (preloaded in background for fast ListCommands)
	cachedCommands  []protocol.AgentSlashCommand
	commandsReadyCh chan struct{} // closed when cachedCommands is populated
	commandsMu      sync.RWMutex  // protects cachedCommands

	// SSE idle timeout: if no event received within this window, the connection
	// is considered dead. Defaults to opencodeSSEReadIdleTimeout; overridable for tests.
	sseReadIdleTimeout time.Duration
}

type pendingPermission struct {
	kind  string // "tool" or "question"
	input map[string]interface{}
}

type timelineItem struct {
	CallID string
	Name   string
	Status string
	Input  interface{}
	Output interface{}
	Error  interface{}
}

// opencodeUsage tracks cumulative token usage (matches Solo's AgentUsage).
type opencodeUsage struct {
	InputTokens             *float64
	OutputTokens            *float64
	CachedInputTokens       *float64
	ContextWindowMaxTokens  *int
	ContextWindowUsedTokens *int
	TotalCostUSD            *float64
}

func newOpenCodeSession(baseURL, sessionID string, config *protocol.AgentSessionConfig, logger *slog.Logger, releaseServer func(), modelContextWindows map[string]int) *openCodeSession {
	ctxWindows := modelContextWindows
	if ctxWindows == nil {
		ctxWindows = make(map[string]int)
	}

	model := ""
	if config.Model != nil {
		model = *config.Model
	}

	// Resolve context window max tokens from configured model
	selectedContextWindow := 0
	if model != "" {
		if cw, ok := ctxWindows[model]; ok && cw > 0 {
			selectedContextWindow = cw
		}
	}

	s := &openCodeSession{
		baseURL:                             baseURL,
		releaseServer:                       releaseServer,
		base:                                base.NewBaseSession(opencodeProviderName, config, logger),
		dispatcher:                          base.NewChannelDispatcher(logger),
		pendingPerms:                        make(map[string]pendingPermission),
		messageRoles:                        make(map[string]string),
		streamedPartKeys:                    make(map[string]bool),
		emittedStructuredMsgIDs:             make(map[string]bool),
		partTypes:                           make(map[string]string),
		runningToolCalls:                    make(map[string]timelineItem),
		accumulatedUsage:                    &opencodeUsage{},
		modelContextWindows:                 ctxWindows,
		selectedModelContextWindowMaxTokens: selectedContextWindow,
		commandsReadyCh:                     make(chan struct{}),
		sseReadIdleTimeout:                  opencodeSSEReadIdleTimeout,
	}
	s.base.SetSessionID(sessionID)

	// Preload commands in background so ListCommands is fast
	go s.warmupCommands()

	// Apply default mode for OpenCode
	if config.ModeID == nil || *config.ModeID == "" || *config.ModeID == "default" {
		s.base.SetMode("build")
	}

	return s
}

// sendToChannel sends an event to a channel, blocking for critical events and
// non-blocking for all others. This is the canonical pattern used by both
// Subscribe() and StartTurn to ensure critical lifecycle events are never dropped.
func sendToChannel(ch chan<- AgentStreamEvent, evt AgentStreamEvent) {
	if evt.IsCriticalEvent() {
		ch <- evt // blocking, never drop
	} else {
		select {
		case ch <- evt:
		default:
		}
	}
}

// subscribeEvents registers a persistent callback for all events via the dispatcher.
func (s *openCodeSession) subscribeEvents(cb func(AgentStreamEvent)) func() {
	ch := s.dispatcher.Subscribe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for evt := range ch {
			cb(evt.(AgentStreamEvent))
		}
	}()
	return func() {
		s.dispatcher.Unsubscribe(ch)
		<-done
	}
}

// notifySubscribers sends an event to all registered subscribers.
func (s *openCodeSession) notifySubscribers(evt AgentStreamEvent) {
	s.dispatcher.Emit(evt)
}

// Subscribe returns a channel that receives all events (implements AgentSession).
func (s *openCodeSession) Subscribe() <-chan AgentStreamEvent {
	src := s.dispatcher.Subscribe()
	ch := make(chan AgentStreamEvent, 256)
	go func() {
		defer close(ch)
		for evt := range src {
			if aevt, ok := evt.(AgentStreamEvent); ok {
				sendToChannel(ch, aevt)
			}
		}
	}()
	return ch
}

func (s *openCodeSession) createTurnID() string {
	s.nextTurnID++
	return fmt.Sprintf("opencode-turn-%d", s.nextTurnID)
}

func (s *openCodeSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) (*AgentRunResult, error) {
	runCtx, cancel := context.WithCancel(ctx)
	turnID := s.createTurnID()
	s.mu.Lock()
	s.base.SetCancelFn(cancel)
	s.activeForegroundTurnID = turnID
	// Initialize usage with context window max tokens (matches Solo)
	s.accumulatedUsage = &opencodeUsage{}
	if s.selectedModelContextWindowMaxTokens > 0 {
		cw := s.selectedModelContextWindowMaxTokens
		s.accumulatedUsage.ContextWindowMaxTokens = &cw
	}
	s.mu.Unlock()
	defer cancel()

	// Start consuming SSE BEFORE sending the prompt.
	// OpenCode's SSE endpoint does not replay past events. If we connect
	// after sending the prompt we miss events and the turn hangs.
	type consumeResult struct {
		result *AgentRunResult
		err    error
	}
	resultCh := make(chan consumeResult, 1)
	go func() {
		result, err := s.consumeSSE(runCtx, turnID)
		resultCh <- consumeResult{result, err}
	}()

	// Give the SSE connection time to establish before sending the prompt.
	// Without this delay the opencode server can truncate the event stream.
	time.Sleep(200 * time.Millisecond)

	if err := s.sendPrompt(runCtx, text, images, attachments); err != nil {
		s.mu.Lock()
		s.activeForegroundTurnID = ""
		s.mu.Unlock()
		return nil, err
	}

	r := <-resultCh
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

func (s *openCodeSession) StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error) {
	runCtx, cancel := context.WithCancel(ctx)
	turnID := s.createTurnID()
	s.mu.Lock()
	s.base.SetCancelFn(cancel)
	s.activeForegroundTurnID = turnID
	// Initialize usage with context window max tokens (matches Solo)
	s.accumulatedUsage = &opencodeUsage{}
	if s.selectedModelContextWindowMaxTokens > 0 {
		cw := s.selectedModelContextWindowMaxTokens
		s.accumulatedUsage.ContextWindowMaxTokens = &cw
	}
	s.mu.Unlock()

	ch := make(chan AgentStreamEvent, 256)
	unsub := s.subscribeEvents(func(evt AgentStreamEvent) {
		sendToChannel(ch, evt)
	})

	// Start consuming SSE BEFORE sending the prompt.
	go func() {
		defer unsub()
		defer close(ch)
		s.consumeSSE(runCtx, turnID)
	}()

	// Give the SSE connection time to establish before sending the prompt.
	// Without this delay the opencode server can truncate the event stream.
	time.Sleep(200 * time.Millisecond)

	if err := s.sendPrompt(runCtx, text, images, attachments); err != nil {
		cancel()
		s.mu.Lock()
		s.activeForegroundTurnID = ""
		s.mu.Unlock()
		return nil, err
	}

	return ch, nil
}

// sendPrompt sends a prompt to OpenCode, supporting attachments, system prompt, and slash commands.
func (s *openCodeSession) sendPrompt(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) error {
	// Ensure MCP servers are configured before first turn
	if err := s.ensureMcpConfigured(ctx); err != nil {
		s.base.Logger().Warn("MCP configuration failed", "error", err)
	}

	// Check for slash command
	if cmdName, cmdArgs := parseSlashCommandInput(text); cmdName != "" {
		if s.isSlashCommand(ctx, cmdName) {
			return s.sendSlashCommand(ctx, cmdName, cmdArgs)
		}
	}

	parts := buildOpenCodePromptParts(text, images, attachments)

	body := map[string]interface{}{
		"parts": parts,
	}
	if s.base.CurrentModel() != "" {
		if p, m := parseOpenCodeModel(s.base.CurrentModel()); p != "" {
			body["model"] = map[string]string{"providerID": p, "modelID": m}
		} else {
			body["model"] = s.base.CurrentModel()
		}
	}
	if s.base.CurrentMode() != "" {
		body["agent"] = s.base.CurrentMode()
	}
	if s.base.CurrentThinking() != "" {
		body["variant"] = s.base.CurrentThinking()
	}
	// System prompt support (gap #8)
	if s.base.Config().SystemPrompt != "" {
		body["system"] = s.base.Config().SystemPrompt
	}
	// outputSchema support (Gap #6)
	if len(s.base.Config().OutputSchema) > 0 {
		body["format"] = map[string]interface{}{
			"type":   "json_schema",
			"schema": s.base.Config().OutputSchema,
		}
	}

	path := "/session/" + s.base.SessionID() + "/prompt_async"
	s.base.Logger().Info("sending opencode prompt", "sessionID", s.base.SessionID(), "path", path, "model", s.base.CurrentModel(), "mode", s.base.CurrentMode())
	if err := opencodePostJSON(ctx, s.baseURL, path, s.base.Config().Cwd, body, nil); err != nil {
		return err
	}
	s.base.Logger().Info("opencode prompt sent successfully")
	return nil
}

// sendSlashCommand routes through /session.command for slash commands.
// Fire-and-forget: the SSE stream delivers terminal events. If the API call
// hits a headers timeout, we log and wait for SSE (matches Solo behavior).
func (s *openCodeSession) sendSlashCommand(ctx context.Context, name, args string) error {
	body := map[string]interface{}{
		"command":   name,
		"arguments": args,
	}
	if s.base.CurrentModel() != "" {
		body["model"] = s.base.CurrentModel()
	}
	if s.base.CurrentMode() != "" {
		body["agent"] = s.base.CurrentMode()
	}
	if s.base.CurrentThinking() != "" {
		body["variant"] = s.base.CurrentThinking()
	}
	go func() {
		path := "/session/" + s.base.SessionID() + "/command"
		err := opencodePostJSON(context.Background(), s.baseURL, path, s.base.Config().Cwd, body, nil)
		if err != nil && isHeadersTimeoutError(err) {
			s.base.Logger().Warn("OpenCode slash command hit a header timeout; waiting for SSE terminal event", "command", name, "error", err)
		}
	}()
	return nil
}

// isSlashCommand checks if the command name is a known slash command.
func (s *openCodeSession) isSlashCommand(ctx context.Context, name string) bool {
	httpCtx, cancel := context.WithTimeout(ctx, opencodeCommandListTimeout)
	defer cancel()

	var commands []struct {
		Name string `json:"name"`
	}
	if err := opencodeGetContext(httpCtx, s.baseURL, "/command", s.base.Config().Cwd, &commands); err != nil {
		return false
	}
	for _, cmd := range commands {
		if cmd.Name == name {
			return true
		}
	}
	return false
}

// warmupCommands preloads the command list in the background.
func (s *openCodeSession) warmupCommands() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	commands, err := s.fetchCommands(ctx)
	if err == nil {
		s.commandsMu.Lock()
		s.cachedCommands = commands
		s.commandsMu.Unlock()
	}
	close(s.commandsReadyCh)
}

// fetchCommands performs the actual HTTP request to load commands.
func (s *openCodeSession) fetchCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	var commands []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Hints       []string `json:"hints"`
	}
	if err := opencodeGet(ctx, s.baseURL, "/command", s.base.Config().Cwd, &commands); err != nil {
		return nil, err
	}
	result := make([]protocol.AgentSlashCommand, 0, len(commands))
	for _, cmd := range commands {
		entry := protocol.AgentSlashCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
		}
		if len(cmd.Hints) > 0 {
			entry.ArgumentHint = strings.Join(cmd.Hints, " ")
		}
		result = append(result, entry)
	}
	return result, nil
}

// ListCommands returns available slash commands (matches Solo's listCommands).
// Uses cached commands if available; otherwise waits for background warmup
// with a 2-second timeout and falls back to an empty list.
func (s *openCodeSession) ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error) {
	// Fast path: cached commands available
	s.commandsMu.RLock()
	if len(s.cachedCommands) > 0 {
		defer s.commandsMu.RUnlock()
		return s.cachedCommands, nil
	}
	s.commandsMu.RUnlock()

	// Wait for background warmup (with timeout)
	select {
	case <-s.commandsReadyCh:
		s.commandsMu.RLock()
		defer s.commandsMu.RUnlock()
		return s.cachedCommands, nil
	case <-time.After(2 * time.Second):
		return []protocol.AgentSlashCommand{}, nil
	}
}

// --- Session Interface Methods ---

func (s *openCodeSession) Interrupt(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.base.Cancel()

	go opencodePostJSON(context.Background(), s.baseURL, "/session/"+s.base.SessionID()+"/abort", s.base.Config().Cwd, nil, nil)

	s.notifySubscribers(AgentStreamEvent{
		Event: protocol.TurnCanceledStreamEvent{
			Provider: opencodeProviderName,
		},
		Timestamp: time.Now(),
	})

	return nil
}

func (s *openCodeSession) Close() error {
	s.mu.Lock()

	if s.base.IsClosed() {
		s.mu.Unlock()
		return nil
	}
	// closed handled by base.Close()

	s.base.Cancel()

	// Synthesize failed tool calls for any still-running tool calls
	var notifyEvents []AgentStreamEvent
	for callID, item := range s.runningToolCalls {
		notifyEvents = append(notifyEvents, AgentStreamEvent{
			Event: protocol.TimelineStreamEvent{
				Item:     TimelineItem{Type: "tool_call", CallID: callID, Name: item.Name, Status: "failed", Error: &protocol.ToolError{Message: "Session closed"}},
				Provider: opencodeProviderName,
			},
			Timestamp: time.Now(),
		})
	}
	s.runningToolCalls = make(map[string]timelineItem)

	// Reject all pending permissions via HTTP
	for reqID, perm := range s.pendingPerms {
		if perm.kind == "question" {
			go opencodePostJSON(context.Background(), s.baseURL, "/question/"+reqID+"/reject", s.base.Config().Cwd, nil, nil)
		} else {
			go opencodePostJSON(context.Background(), s.baseURL, "/permission/"+reqID+"/reply", s.base.Config().Cwd,
				map[string]interface{}{"requestID": reqID, "reply": "reject", "message": "Session closed"}, nil)
		}
	}
	s.pendingPerms = make(map[string]pendingPermission)

	s.mu.Unlock()

	// Notify subscribers of failed tool calls (outside lock)
	for _, ne := range notifyEvents {
		s.dispatcher.Emit(ne)
	}

	// Emit session_closed event
	s.dispatcher.Emit(AgentStreamEvent{
		Event: protocol.TimelineStreamEvent{
			Item:     TimelineItem{Type: "session_closed"},
			Provider: opencodeProviderName,
		},
		Timestamp: time.Now(),
	})

	// Close dispatcher (closes all subscriber channels)
	s.dispatcher.Close()

	// Abort + archive (fire-and-forget)
	go func() {
		opencodePostJSON(context.Background(), s.baseURL, "/session/"+s.base.SessionID()+"/abort", s.base.Config().Cwd, nil, nil)
		opencodePostJSON(context.Background(), s.baseURL, "/session/"+s.base.SessionID(), s.base.Config().Cwd,
			map[string]interface{}{
				"time": map[string]interface{}{"archived": time.Now().UnixMilli()},
			}, nil)
		if s.releaseServer != nil {
			s.releaseServer()
		}
	}()

	return nil
}

// RespondPermission responds to a pending permission request (gap #11, #12).
func (s *openCodeSession) RespondPermission(requestID string, response protocol.AgentPermissionResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pending, ok := s.pendingPerms[requestID]
	if !ok {
		return fmt.Errorf("no pending permission for request %s", requestID)
	}
	delete(s.pendingPerms, requestID)

	if pending.kind == "question" {
		if response.Behavior == "deny" {
			go opencodePostJSON(context.Background(), s.baseURL, "/question/"+requestID+"/reject", s.base.Config().Cwd, nil, nil)
		} else {
			// Extract answers from response (gap #11)
			answers := extractQuestionAnswers(pending.input, response)
			go opencodePostJSON(context.Background(), s.baseURL, "/question/"+requestID+"/reply", s.base.Config().Cwd,
				map[string]interface{}{"answers": answers}, nil)
		}
	} else {
		reply := "reject"
		if response.Behavior == "allow" {
			reply = "once"
		}
		body := map[string]interface{}{
			"requestID": requestID,
			"reply":     reply,
		}
		// Include message on deny (gap #12)
		if response.Behavior == "deny" && response.Message != "" {
			body["message"] = response.Message
		}
		go opencodePostJSON(context.Background(), s.baseURL, "/permission/"+requestID+"/reply", s.base.Config().Cwd, body, nil)
	}

	return nil
}

func (s *openCodeSession) GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error) {
	return s.base.GetRuntimeInfo(), nil
}

func (s *openCodeSession) GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error) {
	// Return cached modes if available (gap #17)
	s.mu.Lock()
	if s.availableModesCache != nil {
		modes := s.availableModesCache
		s.mu.Unlock()
		return modes, nil
	}
	s.mu.Unlock()

	var agents []struct {
		Name        string `json:"name"`
		Mode        string `json:"mode"`
		Hidden      bool   `json:"hidden"`
		Description string `json:"description"`
	}

	if err := opencodeGet(ctx, s.baseURL, "/agent", s.base.Config().Cwd, &agents); err != nil {
		return opencodeDefaultModes(), nil
	}

	var modes []protocol.AgentMode
	for _, a := range agents {
		if a.Mode != "primary" || a.Hidden {
			continue
		}
		label := capitalizeFirst(a.Name)
		desc := a.Description
		if desc == "" {
			desc = opencodeDefaultModeDescriptions[a.Name]
		}
		modes = append(modes, protocol.AgentMode{ID: a.Name, Label: label, Description: desc})
	}
	if len(modes) == 0 {
		return opencodeDefaultModes(), nil
	}
	sorted := sortOpenCodeModes(modes)

	s.mu.Lock()
	s.availableModesCache = sorted
	s.mu.Unlock()

	return sorted, nil
}

func (s *openCodeSession) GetCurrentMode(ctx context.Context) (*string, error) {
	return s.base.GetCurrentModePtr(), nil
}

func (s *openCodeSession) SetMode(modeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if modeID == "" || modeID == "default" {
		modeID = "build"
	}
	s.base.SetMode(modeID)
	return nil
}

func (s *openCodeSession) SetModel(modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.base.SetCurrentModel(modelID)
	// Update context window tracking (matches Solo)
	if modelID != "" {
		if cw, ok := s.modelContextWindows[modelID]; ok && cw > 0 {
			s.selectedModelContextWindowMaxTokens = cw
		} else {
			s.selectedModelContextWindowMaxTokens = 0
		}
	} else {
		s.selectedModelContextWindowMaxTokens = 0
	}
	return nil
}

func (s *openCodeSession) SetThinkingOption(optionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.base.SetThinkingOption(optionID)
	return nil
}

func (s *openCodeSession) DescribePersistence() *protocol.AgentPersistenceHandle {
	return s.base.DescribePersistence()
}

func (s *openCodeSession) GetPendingPermissions() []interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]interface{}, 0, len(s.pendingPerms))
	for reqID := range s.pendingPerms {
		result = append(result, map[string]interface{}{"requestId": reqID})
	}
	return result
}

// StreamHistory fetches all session messages and yields timeline events (matches Solo's streamHistory).
func (s *openCodeSession) StreamHistory(ctx context.Context) ([]AgentStreamEvent, error) {
	var messages []struct {
		Info struct {
			ID         string      `json:"id"`
			Role       string      `json:"role"`
			Structured interface{} `json:"structured"`
		} `json:"info"`
		Parts []struct {
			Type   string          `json:"type"`
			Text   string          `json:"text"`
			Tool   string          `json:"tool"`
			CallID string          `json:"callID"`
			ID     string          `json:"id"`
			State  json.RawMessage `json:"state"`
		} `json:"parts"`
	}

	msgPath := "/session/" + s.base.SessionID() + "/message"
	if err := opencodeGet(ctx, s.baseURL, msgPath, s.base.Config().Cwd, &messages); err != nil {
		return nil, fmt.Errorf("fetch session messages: %w", err)
	}

	var events []AgentStreamEvent
	for _, msg := range messages {
		if msg.Info.Role == "user" {
			var text string
			for _, p := range msg.Parts {
				if p.Type == "text" {
					text += p.Text
				}
			}
			if text != "" {
				events = append(events, AgentStreamEvent{
					Event: protocol.TimelineStreamEvent{
						Item:     TimelineItem{Type: "user_message", Text: text},
						Provider: opencodeProviderName,
					},
					Timestamp: time.Now(),
				})
			}
		} else {
			emittedAssistantText := false
			for _, part := range msg.Parts {
				switch part.Type {
				case "text":
					if part.Text != "" {
						emittedAssistantText = true
						events = append(events, AgentStreamEvent{
							Event: protocol.TimelineStreamEvent{
								Item:     TimelineItem{Type: "assistant_message", Text: part.Text},
								Provider: opencodeProviderName,
							},
							Timestamp: time.Now(),
						})
					}
				case "reasoning":
					if part.Text != "" {
						events = append(events, AgentStreamEvent{
							Event: protocol.TimelineStreamEvent{
								Item:     TimelineItem{Type: "reasoning", Text: part.Text},
								Provider: opencodeProviderName,
							},
							Timestamp: time.Now(),
						})
					}
				case "tool":
					callID := part.CallID
					if callID == "" {
						callID = part.ID
					}
					var toolStatus, toolInput, toolOutput, toolError interface{}
					if part.State != nil {
						var state struct {
							Status string      `json:"status"`
							Input  interface{} `json:"input"`
							Output interface{} `json:"output"`
							Error  interface{} `json:"error"`
						}
						json.Unmarshal(part.State, &state)
						toolStatus = state.Status
						toolInput = state.Input
						toolOutput = state.Output
						toolError = state.Error
					}
					status := normalizeToolStatus(toolStatus.(string))
					events = append(events, AgentStreamEvent{
						Event: protocol.TimelineStreamEvent{
							Item:     buildToolCallTimelineItem(callID, part.Tool, status, toolInput, toolOutput, toolError),
							Provider: opencodeProviderName,
						},
						Timestamp: time.Now(),
					})
				}
			}
			if !emittedAssistantText {
				if text := stringifyStructuredMessage(msg.Info.Structured); text != "" {
					events = append(events, AgentStreamEvent{
						Event: protocol.TimelineStreamEvent{
							Item:     TimelineItem{Type: "assistant_message", Text: text},
							Provider: opencodeProviderName,
						},
						Timestamp: time.Now(),
					})
				}
			}
		}
	}
	return events, nil
}
