// Package protocol defines the WebSocket message types and protocol constants
// shared between the Solo daemon, relay, CLI, and clients.
package protocol

import (
	"encoding/json"
	"fmt"
	"sync"
)

// --- WebSocket Envelope Types ---

// WSInboundMessage is the top-level message from client to server.
// Discriminated on the Type field.
type WSInboundMessage struct {
	Type            string             `json:"type"`
	ClientID        string             `json:"clientId,omitempty"`
	ClientType      ClientType         `json:"clientType,omitempty"`
	ProtocolVersion int                `json:"protocolVersion,omitempty"`
	AppVersion      string             `json:"appVersion,omitempty"`
	Capabilities    *HelloCapabilities `json:"capabilities,omitempty"`
	IsRecording     *bool              `json:"isRecording,omitempty"`
	Message         json.RawMessage    `json:"message,omitempty"`
}

type HelloCapabilities struct {
	Voice             *bool `json:"voice,omitempty"`
	PushNotifications *bool `json:"pushNotifications,omitempty"`
}

// WSOutboundMessage is the top-level message from server to client.
type WSOutboundMessage struct {
	Type    string      `json:"type"`
	Message interface{} `json:"message,omitempty"`
}

// NewPongMessage creates a WS-level pong response.
func NewPongMessage() WSOutboundMessage {
	return WSOutboundMessage{Type: "pong"}
}

// NewSessionMessage wraps a session outbound message.
func NewSessionMessage(msg SessionOutboundMessage) WSOutboundMessage {
	return WSOutboundMessage{
		Type:    "session",
		Message: msg,
	}
}

// --- Session Inbound Message Types ---

// SessionInboundMessage is the interface for all client->server session messages.
type SessionInboundMessage interface {
	MsgType() string
}

// inboundFactory creates a new SessionInboundMessage for deserialization.
type inboundFactory func() SessionInboundMessage

var (
	inboundRegistry   map[string]inboundFactory
	inboundRegistryMu sync.RWMutex
)

// RegisterInbound registers a factory for an inbound message type.
func RegisterInbound(msgType string, factory inboundFactory) {
	inboundRegistryMu.Lock()
	defer inboundRegistryMu.Unlock()
	if inboundRegistry == nil {
		inboundRegistry = make(map[string]inboundFactory)
	}
	inboundRegistry[msgType] = factory
}

// DecodeSessionInboundMessage decodes a JSON raw message into a SessionInboundMessage.
func DecodeSessionInboundMessage(raw json.RawMessage) (SessionInboundMessage, error) {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return nil, fmt.Errorf("failed to peek message type: %w", err)
	}

	inboundRegistryMu.RLock()
	factory, ok := inboundRegistry[peek.Type]
	inboundRegistryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown inbound message type: %s", peek.Type)
	}

	msg := factory()
	if err := json.Unmarshal(raw, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", peek.Type, err)
	}
	return msg, nil
}

// --- Session Outbound Message Types ---

// SessionOutboundMessage is the interface for all server->client session messages.
type SessionOutboundMessage interface {
	MsgType() string
}

func init() {
	// Register all inbound message types
	RegisterInbound("ping", func() SessionInboundMessage { return &PingMessage{} })
	RegisterInbound("client_heartbeat", func() SessionInboundMessage { return &ClientHeartbeatMessage{} })
	RegisterInbound("create_agent_request", func() SessionInboundMessage { return &CreateAgentRequest{} })
	RegisterInbound("resume_agent_request", func() SessionInboundMessage { return &ResumeAgentRequest{} })
	RegisterInbound("refresh_agent_request", func() SessionInboundMessage { return &RefreshAgentRequest{} })
	RegisterInbound("cancel_agent_request", func() SessionInboundMessage { return &CancelAgentRequest{} })
	RegisterInbound("delete_agent_request", func() SessionInboundMessage { return &DeleteAgentRequest{} })
	RegisterInbound("archive_agent_request", func() SessionInboundMessage { return &ArchiveAgentRequest{} })
	RegisterInbound("close_items_request", func() SessionInboundMessage { return &CloseItemsRequest{} })
	RegisterInbound("update_agent_request", func() SessionInboundMessage { return &UpdateAgentRequest{} })
	RegisterInbound("clear_agent_attention", func() SessionInboundMessage { return &ClearAgentAttention{} })
	RegisterInbound("send_agent_message_request", func() SessionInboundMessage { return &SendAgentMessageRequest{} })
	RegisterInbound("wait_for_finish_request", func() SessionInboundMessage { return &WaitForFinishRequest{} })
	RegisterInbound("fetch_agents_request", func() SessionInboundMessage { return &FetchAgentsRequest{} })
	RegisterInbound("fetch_agent_request", func() SessionInboundMessage { return &FetchAgentRequest{} })
	RegisterInbound("fetch_agent_history_request", func() SessionInboundMessage { return &FetchAgentHistoryRequest{} })
	RegisterInbound("fetch_agent_timeline_request", func() SessionInboundMessage { return &FetchAgentTimelineRequest{} })
	RegisterInbound("fetch_workspaces_request", func() SessionInboundMessage { return &FetchWorkspacesRequest{} })
	RegisterInbound("get_providers_snapshot_request", func() SessionInboundMessage { return &GetProvidersSnapshotRequest{} })
	RegisterInbound("set_agent_mode_request", func() SessionInboundMessage { return &SetAgentModeRequest{} })
	RegisterInbound("set_agent_model_request", func() SessionInboundMessage { return &SetAgentModelRequest{} })
	RegisterInbound("set_agent_thinking_request", func() SessionInboundMessage { return &SetAgentThinkingRequest{} })
	RegisterInbound("set_agent_feature_request", func() SessionInboundMessage { return &SetAgentFeatureRequest{} })
	RegisterInbound("agent_permission_response", func() SessionInboundMessage { return &AgentPermissionResponseMessage{} })
	RegisterInbound("register_push_token", func() SessionInboundMessage { return &RegisterPushTokenMessage{} })
	RegisterInbound("restart_server_request", func() SessionInboundMessage { return &RestartServerRequest{} })
	RegisterInbound("shutdown_server_request", func() SessionInboundMessage { return &ShutdownServerRequest{} })
	RegisterInbound("get_daemon_config_request", func() SessionInboundMessage { return &GetDaemonConfigRequest{} })
	RegisterInbound("set_daemon_config_request", func() SessionInboundMessage { return &SetDaemonConfigRequest{} })
	RegisterInbound("open_project_request", func() SessionInboundMessage { return &OpenProjectRequest{} })
	RegisterInbound("list_available_editors_request", func() SessionInboundMessage { return &ListAvailableEditorsRequest{} })
	RegisterInbound("list_terminals_request", func() SessionInboundMessage { return &ListTerminalsRequest{} })
	RegisterInbound("create_terminal_request", func() SessionInboundMessage { return &CreateTerminalRequest{} })
	RegisterInbound("kill_terminal_request", func() SessionInboundMessage { return &KillTerminalRequest{} })
	RegisterInbound("subscribe_terminals_request", func() SessionInboundMessage { return &SubscribeTerminalsRequest{} })
	RegisterInbound("unsubscribe_terminals_request", func() SessionInboundMessage { return &UnsubscribeTerminalsRequest{} })
	RegisterInbound("subscribe_terminal_request", func() SessionInboundMessage { return &SubscribeTerminalRequest{} })
	RegisterInbound("unsubscribe_terminal_request", func() SessionInboundMessage { return &UnsubscribeTerminalRequest{} })
	RegisterInbound("terminal_input", func() SessionInboundMessage { return &TerminalInputMessage{} })
	RegisterInbound("capture_terminal_request", func() SessionInboundMessage { return &CaptureTerminalRequest{} })
	RegisterInbound("start_workspace_script_request", func() SessionInboundMessage { return &StartWorkspaceScriptRequest{} })
	RegisterInbound("file_explorer_request", func() SessionInboundMessage { return &FileExplorerRequest{} })
	RegisterInbound("project_icon_request", func() SessionInboundMessage { return &ProjectIconRequest{} })
	RegisterInbound("directory_suggestions_request", func() SessionInboundMessage { return &DirectorySuggestionsRequest{} })
	RegisterInbound("list_commands_request", func() SessionInboundMessage { return &ListCommandsRequest{} })
	RegisterInbound("list_provider_features_request", func() SessionInboundMessage { return &ListProviderFeaturesRequest{} })
	RegisterInbound("checkout_status_request", func() SessionInboundMessage { return &CheckoutStatusRequest{} })
	RegisterInbound("checkout_pr_status_request", func() SessionInboundMessage { return &CheckoutPrStatusRequest{} })
	RegisterInbound("workspace_setup_status_request", func() SessionInboundMessage { return &WorkspaceSetupStatusRequest{} })
	RegisterInbound("read_project_config_request", func() SessionInboundMessage { return &ReadProjectConfigRequest{} })
	RegisterInbound("write_project_config_request", func() SessionInboundMessage { return &WriteProjectConfigRequest{} })
	RegisterInbound("create_solo_worktree_request", func() SessionInboundMessage { return &CreateSoloWorktreeRequest{} })
	RegisterInbound("archive_workspace_request", func() SessionInboundMessage { return &ArchiveWorkspaceRequest{} })
	RegisterInbound("schedule/create", func() SessionInboundMessage { return &ScheduleCreateRequest{} })
	RegisterInbound("schedule/list", func() SessionInboundMessage { return &ScheduleListRequest{} })
	RegisterInbound("schedule/inspect", func() SessionInboundMessage { return &ScheduleInspectRequest{} })
	RegisterInbound("schedule/logs", func() SessionInboundMessage { return &ScheduleLogsRequest{} })
	RegisterInbound("schedule/pause", func() SessionInboundMessage { return &SchedulePauseRequest{} })
	RegisterInbound("schedule/resume", func() SessionInboundMessage { return &ScheduleResumeRequest{} })
	RegisterInbound("schedule/delete", func() SessionInboundMessage { return &ScheduleDeleteRequest{} })
	RegisterInbound("schedule/update", func() SessionInboundMessage { return &ScheduleUpdateRequest{} })
	RegisterInbound("loop/run", func() SessionInboundMessage { return &LoopRunRequest{} })
	RegisterInbound("loop/list", func() SessionInboundMessage { return &LoopListRequest{} })
	RegisterInbound("loop/inspect", func() SessionInboundMessage { return &LoopInspectRequest{} })
	RegisterInbound("loop/logs", func() SessionInboundMessage { return &LoopLogsRequest{} })
	RegisterInbound("loop/stop", func() SessionInboundMessage { return &LoopStopRequest{} })
	RegisterInbound("loop/update", func() SessionInboundMessage { return &LoopUpdateRequest{} })
	RegisterInbound("loop/delete", func() SessionInboundMessage { return &LoopDeleteRequest{} })
	RegisterInbound("tmux/list_agents", func() SessionInboundMessage { return &TmuxListAgentsRequest{} })
	RegisterInbound("tmux/capture_pane", func() SessionInboundMessage { return &TmuxCapturePaneRequest{} })
	RegisterInbound("tmux/send_keys", func() SessionInboundMessage { return &TmuxSendKeysRequest{} })
	RegisterInbound("tmux/new_session", func() SessionInboundMessage { return &TmuxNewSessionRequest{} })
	RegisterInbound("tmux/kill_session", func() SessionInboundMessage { return &TmuxKillSessionRequest{} })
	RegisterInbound("tmux/delete_command_history", func() SessionInboundMessage { return &TmuxDeleteCommandHistoryRequest{} })
	RegisterInbound("tmux/get_theme", func() SessionInboundMessage { return &TmuxGetThemeRequest{} })
	RegisterInbound("tmux/status_line", func() SessionInboundMessage { return &TmuxStatusLineRequest{} })
}
