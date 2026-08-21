package server

import (
	"encoding/json"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/protocol"
)

func TestTypeHandler(t *testing.T) {
	var received *protocol.PingMessage
	handler := typeHandler(func(m *protocol.PingMessage) {
		received = m
	})

	msg := &protocol.PingMessage{}
	handler(nil, msg)

	if received != msg {
		t.Error("typeHandler did not forward the typed message")
	}
}

func TestRegisterHandlers_RegistersAllTypes(t *testing.T) {
	cfg := &config.Config{
		SoloHome:   t.TempDir(),
		ServerID:   "test-server",
		Version:    "0.1.0",
		AppBaseURL: "https://solo.up2ai.top",
	}
	logger := newTestLogger()

	s := &Session{
		cfg:             cfg,
		logger:          logger,
		handlerRegistry: newMessageHandlerRegistry(),
	}

	s.registerHandlers()

	expectedTypes := []string{
		"ping",
		"client_heartbeat",
		"create_agent_request",
		"fetch_agents_request",
		"fetch_agent_request",
		"fetch_agent_timeline_request",
		"send_agent_message_request",
		"cancel_agent_request",
		"delete_agent_request",
		"archive_agent_request",
		"resume_agent_request",
		"wait_for_finish_request",
		"agent_permission_response",
		"set_agent_mode_request",
		"set_agent_model_request",
		"set_agent_thinking_request",
		"set_agent_feature_request",
		"update_agent_request",
		"clear_agent_attention",
		"refresh_agent_request",
		"close_items_request",
		"list_commands_request",
		"list_provider_features_request",
		"list_available_editors_request",
		"fetch_agent_history_request",
		"get_daemon_config_request",
		"set_daemon_config_request",
		"get_providers_snapshot_request",
		"restart_server_request",
		"shutdown_server_request",
		"list_daemon_versions_request",
		"switch_daemon_version_request",
		"register_push_token",
		"list_terminals_request",
		"create_terminal_request",
		"kill_terminal_request",
		"subscribe_terminals_request",
		"unsubscribe_terminals_request",
		"subscribe_terminal_request",
		"unsubscribe_terminal_request",
		"terminal_input",
		"capture_terminal_request",
		"start_workspace_script_request",
		"open_project_request",
		"fetch_workspaces_request",
		"file_explorer_request",
		"project_icon_request",
		"directory_suggestions_request",
		"create_solo_worktree_request",
		"workspace_setup_status_request",
		"archive_workspace_request",
		"solo_worktree_archive_request",
		"remove_project_request",
		"checkout_pr_status_request",
		"read_project_config_request",
		"write_project_config_request",
		"checkout_status_request",
		"branch_suggestions_request",
		"github_search_request",
		"schedule/create",
		"schedule/list",
		"schedule/inspect",
		"schedule/logs",
		"schedule/pause",
		"schedule/resume",
		"schedule/delete",
		"schedule/update",
		"schedule/assist",
		"tmux/list_agents",
		"tmux/capture_pane",
		"tmux/send_keys",
		"tmux/status_line",
	}

	for _, msgType := range expectedTypes {
		if !s.handlerRegistry.HasHandler(msgType) {
			t.Errorf("expected handler for message type %q", msgType)
		}
	}
}

// Regression test: an rpc_error reply must carry the caller's requestId (and
// the request type), otherwise the client's pending request never resolves
// and times out — this is what happens when a newer app talks to an older
// daemon that doesn't know the message type yet.
func TestHandleSessionMessage_UnknownType_CorrelatesRPCError(t *testing.T) {
	s, q := newCaptureSession()
	s.handlerRegistry = newMessageHandlerRegistry()

	s.handleSessionMessage(json.RawMessage(`{"type":"future_feature_request","requestId":"req-42"}`))

	msgs := drainMessages(q)
	resp := findSessionMessage(msgs, "rpc_error")
	if resp == nil {
		t.Fatalf("expected rpc_error, got %v", msgs)
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["requestId"] != "req-42" {
		t.Errorf("requestId = %v, want req-42", payload["requestId"])
	}
	if payload["requestType"] != "future_feature_request" {
		t.Errorf("requestType = %v, want future_feature_request", payload["requestType"])
	}
}

// Same correlation guarantee for a message that decodes fine but has no
// registered handler on this daemon.
func TestHandleSessionMessage_UnhandledType_CorrelatesRPCError(t *testing.T) {
	s, q := newCaptureSession()
	s.handlerRegistry = newMessageHandlerRegistry()

	s.handleSessionMessage(json.RawMessage(`{"type":"ping","requestId":"req-7"}`))

	msgs := drainMessages(q)
	resp := findSessionMessage(msgs, "rpc_error")
	if resp == nil {
		t.Fatalf("expected rpc_error, got %v", msgs)
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["requestId"] != "req-7" {
		t.Errorf("requestId = %v, want req-7", payload["requestId"])
	}
	if payload["requestType"] != "ping" {
		t.Errorf("requestType = %v, want ping", payload["requestType"])
	}
}
