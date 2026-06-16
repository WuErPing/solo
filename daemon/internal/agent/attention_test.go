package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func createTestManager(t *testing.T) *AgentManager {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	storage := NewAgentStorage(t.TempDir(), logger)
	if err := storage.Initialize(); err != nil {
		t.Fatalf("Initialize storage: %v", err)
	}
	registry := NewProviderRegistry()
	registry.Register(&hangingAfterTerminalClient{})
	manager := NewAgentManager(storage, registry, logger)
	if err := manager.Initialize(context.TODO()); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}
	return manager
}

func TestAgentManager_EmitAttentionRequired_OnTurnCompleted(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(context.TODO(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if _, ok := event.Stream.Event.(protocol.AttentionRequiredStreamEvent); ok {
				attentionEvents = append(attentionEvents, event)
			}
		}
	})

	// Simulate turn_completed event
	event := AgentStreamEvent{
		AgentID:   ag.ID,
		Event:     protocol.TurnCompletedStreamEvent{},
		Timestamp: time.Now(),
	}
	manager.handleStreamEvent(ag, event)

	if len(attentionEvents) != 1 {
		t.Fatalf("expected 1 attention event, got %d", len(attentionEvents))
	}

	payload := attentionEvents[0].Stream.Event.(protocol.AttentionRequiredStreamEvent)
	if payload.Reason != "finished" {
		t.Errorf("expected reason 'finished', got %v", payload.Reason)
	}
	if payload.Provider != ag.Provider {
		t.Errorf("expected provider %s, got %v", ag.Provider, payload.Provider)
	}

	// Verify attention state
	if !ag.Attention.Requires {
		t.Error("expected attention to be required")
	}
	if ag.Attention.Reason != "finished" {
		t.Errorf("expected attention reason 'finished', got %s", ag.Attention.Reason)
	}
}

func TestAgentManager_EmitAttentionRequired_OnTurnFailed(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(context.TODO(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if _, ok := event.Stream.Event.(protocol.AttentionRequiredStreamEvent); ok {
				attentionEvents = append(attentionEvents, event)
			}
		}
	})

	// Simulate turn_failed event
	event := AgentStreamEvent{
		AgentID:   ag.ID,
		Event:     protocol.TurnFailedStreamEvent{Error: "something went wrong"},
		Timestamp: time.Now(),
	}
	manager.handleStreamEvent(ag, event)

	if len(attentionEvents) != 1 {
		t.Fatalf("expected 1 attention event, got %d", len(attentionEvents))
	}

	payload := attentionEvents[0].Stream.Event.(protocol.AttentionRequiredStreamEvent)
	if payload.Reason != "error" {
		t.Errorf("expected reason 'error', got %v", payload.Reason)
	}

	// Verify error state
	if ag.Lifecycle != protocol.AgentError {
		t.Errorf("expected lifecycle error, got %s", ag.Lifecycle)
	}
}

func TestAgentManager_EmitAttentionRequired_OnPermissionRequested(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(context.TODO(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if _, ok := event.Stream.Event.(protocol.AttentionRequiredStreamEvent); ok {
				attentionEvents = append(attentionEvents, event)
			}
		}
	})

	// Simulate permission_requested event
	event := AgentStreamEvent{
		AgentID: ag.ID,
		Event: protocol.PermissionRequestedStreamEvent{
			Request: protocol.PermissionRequest{ID: "perm-1", Name: "shell"},
		},
		Timestamp: time.Now(),
	}
	manager.handleStreamEvent(ag, event)

	if len(attentionEvents) != 1 {
		t.Fatalf("expected 1 attention event, got %d", len(attentionEvents))
	}

	payload := attentionEvents[0].Stream.Event.(protocol.AttentionRequiredStreamEvent)
	if payload.Reason != "permission" {
		t.Errorf("expected reason 'permission', got %v", payload.Reason)
	}

	// Verify attention state
	if !ag.Attention.Requires {
		t.Error("expected attention to be required")
	}
	if ag.Attention.Reason != "permission" {
		t.Errorf("expected attention reason 'permission', got %s", ag.Attention.Reason)
	}
}

func TestAgentManager_EmitAttentionRequired_MultipleEvents(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(context.TODO(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if _, ok := event.Stream.Event.(protocol.AttentionRequiredStreamEvent); ok {
				attentionEvents = append(attentionEvents, event)
			}
		}
	})

	// First: permission requested
	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.PermissionRequestedStreamEvent{},
	})

	// Then: turn completed
	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TurnCompletedStreamEvent{},
	})

	if len(attentionEvents) != 2 {
		t.Fatalf("expected 2 attention events, got %d", len(attentionEvents))
	}
}

func TestAgentManager_EmitAttentionRequired_IncludesNotificationPayload(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(context.TODO(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvent AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if _, ok := event.Stream.Event.(protocol.AttentionRequiredStreamEvent); ok {
				attentionEvent = event
			}
		}
	})

	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TurnCompletedStreamEvent{},
	})

	payload := attentionEvent.Stream.Event.(protocol.AttentionRequiredStreamEvent)

	if payload.Notification["title"] != "Agent finished" {
		t.Errorf("expected notification title 'Agent finished', got %v", payload.Notification["title"])
	}
	body, _ := payload.Notification["body"].(string)
	if body == "" {
		t.Error("expected notification body to be non-empty")
	}
	data, ok := payload.Notification["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected notification data to be map[string]interface{}, got %T", payload.Notification["data"])
	}
	if data["agentId"] != ag.ID {
		t.Errorf("expected notification data agentId %s, got %v", ag.ID, data["agentId"])
	}
	if data["reason"] != "finished" {
		t.Errorf("expected notification data reason 'finished', got %v", data["reason"])
	}
}

func TestAgentManager_EmitAttentionRequired_IncludesShouldNotifyAndTimestamp(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(context.TODO(), &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvent AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if _, ok := event.Stream.Event.(protocol.AttentionRequiredStreamEvent); ok {
				attentionEvent = event
			}
		}
	})

	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   protocol.TurnCompletedStreamEvent{},
	})

	payload := attentionEvent.Stream.Event.(protocol.AttentionRequiredStreamEvent)

	if payload.ShouldNotify != false {
		t.Errorf("expected shouldNotify false from manager (Session overrides), got %v", payload.ShouldNotify)
	}
}
