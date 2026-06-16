package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// augmentedMockClient wraps MockAgentClient adding error-injection and custom
// event sequences without modifying the production MockAgentClient struct.
type augmentedMockClient struct {
	*MockAgentClient
	availableErr     error
	createSessionErr error
	runEvents        []AgentStreamEvent
	runErr           error
}

func (a *augmentedMockClient) IsAvailable(_ context.Context) error {
	return a.availableErr
}

func (a *augmentedMockClient) CreateSession(_ context.Context, _ *protocol.AgentSessionConfig) (AgentSession, error) {
	if a.createSessionErr != nil {
		return nil, a.createSessionErr
	}
	sess := &configurableSession{
		MockAgentSession: &MockAgentSession{
			events: make(chan AgentStreamEvent, 64),
			mode:   "default",
			model:  "mock-model",
		},
		runEvents: a.runEvents,
		runErr:    a.runErr,
	}
	return sess, nil
}

// configurableSession wraps MockAgentSession with injectable Run behaviour.
type configurableSession struct {
	*MockAgentSession
	runEvents []AgentStreamEvent
	runErr    error
}

func (s *configurableSession) Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment, messageID string) (*AgentRunResult, error) {
	if s.runErr != nil {
		return nil, s.runErr
	}
	if s.runEvents != nil {
		for _, ev := range s.runEvents {
			s.emit(ev)
		}
		return &AgentRunResult{FinalText: "custom"}, nil
	}
	return s.MockAgentSession.Run(ctx, text, images, attachments, messageID)
}

// ---- IsAvailable tests ----

func TestMockAgentClientIsAvailableSuccess(t *testing.T) {
	m := NewMockAgentClient()
	if err := m.IsAvailable(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAugmentedMockIsAvailableError(t *testing.T) {
	m := &augmentedMockClient{
		MockAgentClient: NewMockAgentClient(),
		availableErr:    errors.New("provider offline"),
	}
	err := m.IsAvailable(context.Background())
	if err == nil || err.Error() != "provider offline" {
		t.Errorf("expected 'provider offline', got %v", err)
	}
}

// ---- CreateSession / ResumeSession tests ----

func TestMockAgentClientCreateSessionReturnsMockSession(t *testing.T) {
	m := NewMockAgentClient()
	sess, err := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{Provider: "mock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestAugmentedMockCreateSessionError(t *testing.T) {
	m := &augmentedMockClient{
		MockAgentClient:  NewMockAgentClient(),
		createSessionErr: errors.New("quota exceeded"),
	}
	_, err := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	if err == nil || err.Error() != "quota exceeded" {
		t.Errorf("expected 'quota exceeded', got %v", err)
	}
}

func TestMockAgentClientResumeSessionReturnsMockSession(t *testing.T) {
	m := NewMockAgentClient()
	handle := &protocol.AgentPersistenceHandle{Provider: "mock", SessionID: "sid"}
	sess, err := m.ResumeSession(context.Background(), handle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
}

// ---- Run / events tests ----

func TestMockSessionRunEmitsStreamEventTypes(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{Provider: "mock"})
	mockSess := sess.(*MockAgentSession)

	done := make(chan struct{})
	var events []AgentStreamEvent
	go func() {
		defer close(done)
		for ev := range mockSess.events {
			events = append(events, ev)
		}
	}()

	result, err := mockSess.Run(context.Background(), "hi", nil, nil, "msg-1")
	mockSess.Close()
	<-done

	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.FinalText == "" {
		t.Error("expected non-empty FinalText")
	}

	var types []string
	for _, ev := range events {
		se, ok := ev.Event.(protocol.StreamEvent)
		if !ok {
			t.Errorf("event[%d] is not a protocol.StreamEvent, got %T", len(types), ev.Event)
			continue
		}
		types = append(types, se.StreamEventType())
	}
	want := []string{"thread_started", "timeline", "timeline", "turn_completed"}
	if len(types) != len(want) {
		t.Fatalf("event types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("event[%d] type = %q, want %q", i, types[i], want[i])
		}
	}
}

func TestMockSessionRunEmitsExpectedTimelineContent(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{Provider: "mock"})
	mockSess := sess.(*MockAgentSession)

	done := make(chan struct{})
	var events []AgentStreamEvent
	go func() {
		defer close(done)
		for ev := range mockSess.events {
			events = append(events, ev)
		}
	}()

	result, err := mockSess.Run(context.Background(), "hi", nil, nil, "msg-1")
	mockSess.Close()
	<-done

	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.FinalText != "Mock response to: hi" {
		t.Errorf("FinalText = %q, want %q", result.FinalText, "Mock response to: hi")
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// First timeline event should be user_message.
	timeline1, ok := events[1].Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("event[1] is not TimelineStreamEvent, got %T", events[1].Event)
	}
	if timeline1.Provider != "mock" {
		t.Errorf("timeline1.Provider = %q, want mock", timeline1.Provider)
	}
	if timeline1.Item.Type != "user_message" {
		t.Errorf("timeline1.Item.Type = %q, want user_message", timeline1.Item.Type)
	}
	if timeline1.Item.Text != "hi" {
		t.Errorf("timeline1.Item.Text = %q, want hi", timeline1.Item.Text)
	}
	if timeline1.Item.MessageID != "msg-1" {
		t.Errorf("timeline1.Item.MessageID = %q, want msg-1", timeline1.Item.MessageID)
	}

	// Second timeline event should be assistant_message.
	timeline2, ok := events[2].Event.(protocol.TimelineStreamEvent)
	if !ok {
		t.Fatalf("event[2] is not TimelineStreamEvent, got %T", events[2].Event)
	}
	if timeline2.Item.Type != "assistant_message" {
		t.Errorf("timeline2.Item.Type = %q, want assistant_message", timeline2.Item.Type)
	}
	wantText := "Mock response to: hi"
	if timeline2.Item.Text != wantText {
		t.Errorf("timeline2.Item.Text = %q, want %q", timeline2.Item.Text, wantText)
	}

	// Thread started event should carry session ID and provider.
	started, ok := events[0].Event.(protocol.ThreadStartedStreamEvent)
	if !ok {
		t.Fatalf("event[0] is not ThreadStartedStreamEvent, got %T", events[0].Event)
	}
	if started.Provider != "mock" {
		t.Errorf("started.Provider = %q, want mock", started.Provider)
	}
	if started.SessionID == "" {
		t.Error("expected non-empty SessionID")
	}

	// Turn completed event.
	completed, ok := events[3].Event.(protocol.TurnCompletedStreamEvent)
	if !ok {
		t.Fatalf("event[3] is not TurnCompletedStreamEvent, got %T", events[3].Event)
	}
	if completed.Provider != "mock" {
		t.Errorf("completed.Provider = %q, want mock", completed.Provider)
	}
}

func TestConfigurableSessionCustomEvents(t *testing.T) {
	customEvents := []AgentStreamEvent{
		{Event: map[string]interface{}{"type": "thread_started", "provider": "mock"}, Timestamp: time.Now()},
		{Event: map[string]interface{}{"type": "turn_completed", "provider": "mock"}, Timestamp: time.Now()},
	}
	sess := &configurableSession{
		MockAgentSession: &MockAgentSession{
			events: make(chan AgentStreamEvent, 64),
			mode:   "default",
			model:  "mock-model",
		},
		runEvents: customEvents,
	}

	result, err := sess.Run(context.Background(), "test", nil, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalText != "custom" {
		t.Errorf("FinalText = %q, want custom", result.FinalText)
	}
}

func TestConfigurableSessionRunError(t *testing.T) {
	sess := &configurableSession{
		MockAgentSession: &MockAgentSession{
			events: make(chan AgentStreamEvent, 64),
			mode:   "default",
			model:  "mock-model",
		},
		runErr: errors.New("context deadline exceeded"),
	}
	_, err := sess.Run(context.Background(), "test", nil, nil, "")
	if err == nil || err.Error() != "context deadline exceeded" {
		t.Errorf("expected deadline error, got %v", err)
	}
}

// ---- ListModels / ListModes / ListClientCommands ----

func TestMockAgentClientListModels(t *testing.T) {
	m := NewMockAgentClient()
	models, err := m.ListModels(context.Background(), "/cwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) == 0 {
		t.Error("expected at least one model")
	}
	if !models[0].IsDefault {
		t.Error("expected first model to be default")
	}
}

func TestMockAgentClientListModes(t *testing.T) {
	m := NewMockAgentClient()
	modes, err := m.ListModes(context.Background(), "/cwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(modes) == 0 {
		t.Error("expected at least one mode")
	}
}

func TestMockAgentClientListClientCommandsReturnsNil(t *testing.T) {
	m := NewMockAgentClient()
	cmds, err := m.ListClientCommands(context.Background(), "/cwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmds != nil {
		t.Errorf("expected nil commands, got %v", cmds)
	}
}

// ---- MockAgentSession accessors ----

func TestMockSessionSetAndGetMode(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	if err := mockSess.SetMode("custom"); err != nil {
		t.Fatalf("SetMode error: %v", err)
	}
	got, err := mockSess.GetCurrentMode(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentMode error: %v", err)
	}
	if *got != "custom" {
		t.Errorf("mode = %q, want custom", *got)
	}
}

func TestMockSessionSetModel(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	if err := mockSess.SetModel("gpt-4"); err != nil {
		t.Fatalf("SetModel error: %v", err)
	}
	info, err := mockSess.GetRuntimeInfo(context.Background())
	if err != nil {
		t.Fatalf("GetRuntimeInfo error: %v", err)
	}
	if info.Model == nil || *info.Model != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", info.Model)
	}
}

func TestMockSessionDescribePersistence(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	handle := mockSess.DescribePersistence()
	if handle == nil {
		t.Fatal("expected non-nil persistence handle")
	}
	if handle.Provider != "mock" {
		t.Errorf("Provider = %q, want mock", handle.Provider)
	}
	if handle.SessionID == "" {
		t.Error("expected non-empty SessionID")
	}
}

func TestMockSessionGetPendingPermissionsEmpty(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	perms := mockSess.GetPendingPermissions()
	if perms != nil {
		t.Errorf("expected nil permissions, got %v", perms)
	}
}

func TestMockSessionCloseIdempotent(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	if err := mockSess.Close(); err != nil {
		t.Fatalf("first Close error: %v", err)
	}
	if err := mockSess.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
}

func TestMockSessionRespondPermission(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	err := mockSess.RespondPermission("req-1", protocol.AgentPermissionResponse{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMockSessionInterrupt(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	if err := mockSess.Interrupt(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMockSessionStreamHistoryEmpty(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	history, err := mockSess.StreamHistory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d events", len(history))
	}
}

func TestMockSessionListCommands(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	cmds, err := mockSess.ListCommands(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmds != nil {
		t.Errorf("expected nil, got %v", cmds)
	}
}

func TestMockSessionGetAvailableModes(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	modes, err := mockSess.GetAvailableModes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(modes) == 0 {
		t.Error("expected at least one mode")
	}
}

func TestMockSessionSetThinkingOption(t *testing.T) {
	m := NewMockAgentClient()
	sess, _ := m.CreateSession(context.Background(), &protocol.AgentSessionConfig{})
	mockSess := sess.(*MockAgentSession)

	if err := mockSess.SetThinkingOption("budget-high"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
