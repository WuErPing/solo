package protocol

import (
	"testing"
)

func TestAllOutboundMessageTypes(t *testing.T) {
	tests := []struct {
		msg  SessionOutboundMessage
		want string
	}{
		// message_agent_outbound.go
		{&StatusMessage{}, "status"},
		{&PongMessage{}, "pong"},
		{&RpcErrorMessage{}, "rpc_error"},
		{&AgentStreamMessage{}, "agent_stream"},
		{&AgentUpdateMessage{}, "agent_update"},
		{&FetchAgentsResponse{}, "fetch_agents_response"},
		{&FetchWorkspacesResponse{}, "fetch_workspaces_response"},
		{&WorkspaceUpdateMessage{}, "workspace_update"},
		{&GetProvidersSnapshotResponse{}, "get_providers_snapshot_response"},
		{&FetchAgentResponse{}, "fetch_agent_response"},
		{&FetchAgentTimelineResponse{}, "fetch_agent_timeline_response"},
		{&AgentDeletedMessage{}, "agent_deleted"},
		{&AgentArchivedMessage{}, "agent_archived"},
		{&SendAgentMessageResponse{}, "send_agent_message_response"},
		{&ClearAgentAttentionResponse{}, "clear_agent_attention_response"},
		{&WaitForFinishResponse{}, "wait_for_finish_response"},
		{&SetAgentModeResponse{}, "set_agent_mode_response"},
		{&SetAgentModelResponse{}, "set_agent_model_response"},
		{&FetchAgentHistoryResponse{}, "fetch_agent_history_response"},
		{&ProvidersSnapshotUpdate{}, "providers_snapshot_update"},

		// message_agent_inbound.go (response types)
		{&CancelAgentResponse{}, "cancel_agent_response"},
		{&OpenProjectResponse{}, "open_project_response"},
		{&GetDaemonConfigResponse{}, "get_daemon_config_response"},
		{&SetDaemonConfigResponse{}, "set_daemon_config_response"},

		// message_worktree.go
		{&CreateSoloWorktreeResponse{}, "create_solo_worktree_response"},
		{&WorkspaceSetupProgressMessage{}, "workspace_setup_progress"},
		{&WorkspaceSetupStatusResponse{}, "workspace_setup_status_response"},

		// message_editor.go
		{&ListAvailableEditorsResponse{}, "list_available_editors_response"},

		// message_solo_compat.go
		{&ReadProjectConfigResponse{}, "read_project_config_response"},
		{&WriteProjectConfigResponse{}, "write_project_config_response"},

		// message_terminal_msg.go
		{&ListTerminalsResponse{}, "list_terminals_response"},
		{&CreateTerminalResponse{}, "create_terminal_response"},
		{&StartWorkspaceScriptResponse{}, "start_workspace_script_response"},
		{&FileExplorerResponse{}, "file_explorer_response"},
		{&ProjectIconResponse{}, "project_icon_response"},
		{&DirectorySuggestionsResponse{}, "directory_suggestions_response"},
		{&ListProviderFeaturesResponse{}, "list_provider_features_response"},
		{&CheckoutStatusResponse{}, "checkout_status_response"},
		{&ListCommandsResponse{}, "list_commands_response"},
	}

	for _, tt := range tests {
		got := tt.msg.MsgType()
		if got != tt.want {
			t.Errorf("%T.MsgType() = %q, want %q", tt.msg, got, tt.want)
		}
	}
}
