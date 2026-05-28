package agent

import (
	"context"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestMockAgentClientListModelsModesCommands(t *testing.T) {
	client := NewMockAgentClient()

	models, err := client.ListModels(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("ListModels: got %d models", len(models))
	}

	modes, err := client.ListModes(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("ListModes: %v", err)
	}
	if len(modes) != 1 {
		t.Errorf("ListModes: got %d modes", len(modes))
	}

	cmds, err := client.ListClientCommands(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("ListClientCommands: %v", err)
	}
	if cmds != nil {
		t.Error("expected nil commands")
	}
}

func TestMockAgentSessionMethods(t *testing.T) {
	client := NewMockAgentClient()
	sess, err := client.CreateSession(context.Background(), &protocol.AgentSessionConfig{Provider: "mock"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ms := sess.(*MockAgentSession)

	// StartTurn
	ch, err := ms.StartTurn(context.Background(), "hello", nil, nil)
	if err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// RespondPermission
	if err := ms.RespondPermission("req-1", protocol.AgentPermissionResponse{}); err != nil {
		t.Fatalf("RespondPermission: %v", err)
	}

	// GetPendingPermissions
	if perms := ms.GetPendingPermissions(); perms != nil {
		t.Error("expected nil pending permissions")
	}

	// ListCommands
	if cmds, err := ms.ListCommands(context.Background()); err != nil || cmds != nil {
		t.Error("expected nil commands")
	}

	// SetMode / SetModel / SetThinkingOption
	if err := ms.SetMode("test"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if err := ms.SetModel("test-model"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	if err := ms.SetThinkingOption("full"); err != nil {
		t.Fatalf("SetThinkingOption: %v", err)
	}
}

func TestMockAgentClientResumeSession(t *testing.T) {
	client := NewMockAgentClient()
	sess, err := client.ResumeSession(context.Background(), &protocol.AgentPersistenceHandle{Provider: "mock"})
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session")
	}
}

func TestMockAgentSessionRun(t *testing.T) {
	client := NewMockAgentClient()
	sess, _ := client.CreateSession(context.Background(), &protocol.AgentSessionConfig{Provider: "mock"})
	ms := sess.(*MockAgentSession)

	result, err := ms.Run(context.Background(), "test", nil, nil, "msg-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}
