package agent

import (
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
	if err := manager.Initialize(nil); err != nil {
		t.Fatalf("Initialize manager: %v", err)
	}
	return manager
}

func TestAgentManager_EmitAttentionRequired_OnTurnCompleted(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(nil, &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if payload, ok := event.Stream.Event.(map[string]interface{}); ok {
				if payload["type"] == "attention_required" {
					attentionEvents = append(attentionEvents, event)
				}
			}
		}
	})

	// Simulate turn_completed event
	event := AgentStreamEvent{
		AgentID: ag.ID,
		Event: map[string]interface{}{
			"type": "turn_completed",
		},
		Timestamp: time.Now(),
	}
	manager.handleStreamEvent(ag, event)

	if len(attentionEvents) != 1 {
		t.Fatalf("expected 1 attention event, got %d", len(attentionEvents))
	}

	payload := attentionEvents[0].Stream.Event.(map[string]interface{})
	if payload["reason"] != "finished" {
		t.Errorf("expected reason 'finished', got %v", payload["reason"])
	}
	if payload["provider"] != ag.Provider {
		t.Errorf("expected provider %s, got %v", ag.Provider, payload["provider"])
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

	ag, err := manager.CreateAgent(nil, &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if payload, ok := event.Stream.Event.(map[string]interface{}); ok {
				if payload["type"] == "attention_required" {
					attentionEvents = append(attentionEvents, event)
				}
			}
		}
	})

	// Simulate turn_failed event
	event := AgentStreamEvent{
		AgentID: ag.ID,
		Event: map[string]interface{}{
			"type":  "turn_failed",
			"error": "something went wrong",
		},
		Timestamp: time.Now(),
	}
	manager.handleStreamEvent(ag, event)

	if len(attentionEvents) != 1 {
		t.Fatalf("expected 1 attention event, got %d", len(attentionEvents))
	}

	payload := attentionEvents[0].Stream.Event.(map[string]interface{})
	if payload["reason"] != "error" {
		t.Errorf("expected reason 'error', got %v", payload["reason"])
	}

	// Verify error state
	if ag.Lifecycle != LifecycleError {
		t.Errorf("expected lifecycle error, got %s", ag.Lifecycle)
	}
}

func TestAgentManager_EmitAttentionRequired_OnPermissionRequested(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(nil, &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if payload, ok := event.Stream.Event.(map[string]interface{}); ok {
				if payload["type"] == "attention_required" {
					attentionEvents = append(attentionEvents, event)
				}
			}
		}
	})

	// Simulate permission_requested event
	event := AgentStreamEvent{
		AgentID: ag.ID,
		Event: map[string]interface{}{
			"type": "permission_requested",
			"request": map[string]interface{}{
				"id":   "perm-1",
				"name": "shell",
			},
		},
		Timestamp: time.Now(),
	}
	manager.handleStreamEvent(ag, event)

	if len(attentionEvents) != 1 {
		t.Fatalf("expected 1 attention event, got %d", len(attentionEvents))
	}

	payload := attentionEvents[0].Stream.Event.(map[string]interface{})
	if payload["reason"] != "permission" {
		t.Errorf("expected reason 'permission', got %v", payload["reason"])
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

	ag, err := manager.CreateAgent(nil, &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvents []AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if payload, ok := event.Stream.Event.(map[string]interface{}); ok {
				if payload["type"] == "attention_required" {
					attentionEvents = append(attentionEvents, event)
				}
			}
		}
	})

	// First: permission requested
	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   map[string]interface{}{"type": "permission_requested"},
	})

	// Then: turn completed
	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   map[string]interface{}{"type": "turn_completed"},
	})

	if len(attentionEvents) != 2 {
		t.Fatalf("expected 2 attention events, got %d", len(attentionEvents))
	}
}

func TestAgentManager_EmitAttentionRequired_IncludesNotificationPayload(t *testing.T) {
	manager := createTestManager(t)

	ag, err := manager.CreateAgent(nil, &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvent AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if payload, ok := event.Stream.Event.(map[string]interface{}); ok {
				if payload["type"] == "attention_required" {
					attentionEvent = event
				}
			}
		}
	})

	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   map[string]interface{}{"type": "turn_completed"},
	})

	payload := attentionEvent.Stream.Event.(map[string]interface{})

	notification, ok := payload["notification"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected notification to be map[string]interface{}, got %T", payload["notification"])
	}
	if notification["title"] != "Agent finished" {
		t.Errorf("expected notification title 'Agent finished', got %v", notification["title"])
	}
	body, _ := notification["body"].(string)
	if body == "" {
		t.Error("expected notification body to be non-empty")
	}
	data, ok := notification["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected notification data to be map[string]interface{}, got %T", notification["data"])
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

	ag, err := manager.CreateAgent(nil, &protocol.AgentSessionConfig{
		Provider: "hanging-terminal",
		Cwd:      t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	var attentionEvent AgentEvent
	manager.Subscribe(func(event AgentEvent) {
		if event.Type == EventAgentStream && event.Stream != nil {
			if payload, ok := event.Stream.Event.(map[string]interface{}); ok {
				if payload["type"] == "attention_required" {
					attentionEvent = event
				}
			}
		}
	})

	manager.handleStreamEvent(ag, AgentStreamEvent{
		AgentID: ag.ID,
		Event:   map[string]interface{}{"type": "turn_completed"},
	})

	payload := attentionEvent.Stream.Event.(map[string]interface{})

	shouldNotify, ok := payload["shouldNotify"].(bool)
	if !ok {
		t.Fatalf("expected shouldNotify to be bool, got %T", payload["shouldNotify"])
	}
	if shouldNotify != false {
		t.Errorf("expected shouldNotify false from manager (Session overrides), got %v", shouldNotify)
	}

	timestamp, _ := payload["timestamp"].(string)
	if timestamp == "" {
		t.Error("expected timestamp to be non-empty string")
	}
}
