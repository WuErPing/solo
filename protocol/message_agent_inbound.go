package protocol

import (
	"encoding/json"
	"fmt"
)

// --- Inbound Message Structs ---

// PingMessage
type PingMessage struct {
	Type         string `json:"type"`
	RequestID    string `json:"requestId"`
	ClientSentAt *int64 `json:"clientSentAt,omitempty"`
}

func (m *PingMessage) MsgType() string { return "ping" }

// ClientHeartbeatMessage
type ClientHeartbeatMessage struct {
	Type                   string  `json:"type"`
	DeviceType             string  `json:"deviceType"`
	FocusedAgentID         *string `json:"focusedAgentId"`
	LastActivityAt         string  `json:"lastActivityAt"`
	AppVisible             bool    `json:"appVisible"`
	AppVisibilityChangedAt *string `json:"appVisibilityChangedAt,omitempty"`
}

func (m *ClientHeartbeatMessage) MsgType() string { return "client_heartbeat" }

// CreateAgentRequest
type CreateAgentRequest struct {
	Type            string                 `json:"type"`
	Config          AgentSessionConfig     `json:"config"`
	WorkspaceID     *string                `json:"workspaceId,omitempty"`
	WorktreeName    *string                `json:"worktreeName,omitempty"`
	InitialPrompt   *string                `json:"initialPrompt,omitempty"`
	ClientMessageID *string                `json:"clientMessageId,omitempty"`
	OutputSchema    map[string]interface{} `json:"outputSchema,omitempty"`
	Images          []ImageAttachment      `json:"images,omitempty"`
	Attachments     []AgentAttachment      `json:"attachments,omitempty"`
	Git             *GitSetupOptions       `json:"git,omitempty"`
	Labels          map[string]string      `json:"labels"`
	RequestID       string                 `json:"requestId"`
}

func (m *CreateAgentRequest) MsgType() string { return "create_agent_request" }

// ResumeAgentRequest
type ResumeAgentRequest struct {
	Type      string                 `json:"type"`
	Handle    AgentPersistenceHandle `json:"handle"`
	Overrides *AgentSessionConfig    `json:"overrides,omitempty"`
	RequestID string                 `json:"requestId"`
}

func (m *ResumeAgentRequest) MsgType() string { return "resume_agent_request" }

// RefreshAgentRequest
type RefreshAgentRequest struct {
	Type      string `json:"type"`
	AgentID   string `json:"agentId"`
	RequestID string `json:"requestId"`
}

func (m *RefreshAgentRequest) MsgType() string { return "refresh_agent_request" }

// CancelAgentRequest
type CancelAgentRequest struct {
	Type      string  `json:"type"`
	AgentID   string  `json:"agentId"`
	RequestID *string `json:"requestId,omitempty"`
}

func (m *CancelAgentRequest) MsgType() string { return "cancel_agent_request" }

// CancelAgentResponse
type CancelAgentResponse struct {
	Type    string                     `json:"type"`
	Payload CancelAgentResponsePayload `json:"payload"`
}

type CancelAgentResponsePayload struct {
	RequestID string  `json:"requestId"`
	AgentID   string  `json:"agentId"`
	Error     *string `json:"error,omitempty"`
}

func (m *CancelAgentResponse) MsgType() string { return "cancel_agent_response" }

// DeleteAgentRequest
type DeleteAgentRequest struct {
	Type      string `json:"type"`
	AgentID   string `json:"agentId"`
	RequestID string `json:"requestId"`
}

func (m *DeleteAgentRequest) MsgType() string { return "delete_agent_request" }

// ArchiveAgentRequest
type ArchiveAgentRequest struct {
	Type      string `json:"type"`
	AgentID   string `json:"agentId"`
	RequestID string `json:"requestId"`
}

func (m *ArchiveAgentRequest) MsgType() string { return "archive_agent_request" }

// CloseItemsRequest
type CloseItemsRequest struct {
	Type        string   `json:"type"`
	AgentIDs    []string `json:"agentIds"`
	TerminalIDs []string `json:"terminalIds"`
	RequestID   string   `json:"requestId"`
}

func (m *CloseItemsRequest) MsgType() string { return "close_items_request" }

// UpdateAgentRequest
type UpdateAgentRequest struct {
	Type      string            `json:"type"`
	AgentID   string            `json:"agentId"`
	Name      *string           `json:"name,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	RequestID string            `json:"requestId"`
}

func (m *UpdateAgentRequest) MsgType() string { return "update_agent_request" }

// ClearAgentAttention
type ClearAgentAttention struct {
	Type      string   `json:"type"`
	AgentID   []string `json:"agentId"`
	RequestID *string  `json:"requestId,omitempty"`
}

func (m *ClearAgentAttention) MsgType() string { return "clear_agent_attention" }

func (m *ClearAgentAttention) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type      string          `json:"type"`
		AgentID   json.RawMessage `json:"agentId"`
		RequestID *string         `json:"requestId,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var single string
	if err := json.Unmarshal(raw.AgentID, &single); err == nil {
		m.Type = raw.Type
		m.AgentID = []string{single}
		m.RequestID = raw.RequestID
		return nil
	}

	var many []string
	if err := json.Unmarshal(raw.AgentID, &many); err != nil {
		return fmt.Errorf("agentId must be string or string[]: %w", err)
	}
	m.Type = raw.Type
	m.AgentID = many
	m.RequestID = raw.RequestID
	return nil
}

// SendAgentMessageRequest
type SendAgentMessageRequest struct {
	Type        string            `json:"type"`
	RequestID   string            `json:"requestId"`
	AgentID     string            `json:"agentId"`
	Text        string            `json:"text"`
	MessageID   *string           `json:"messageId,omitempty"`
	Images      []ImageAttachment `json:"images,omitempty"`
	Attachments []AgentAttachment `json:"attachments,omitempty"`
}

func (m *SendAgentMessageRequest) MsgType() string { return "send_agent_message_request" }

// WaitForFinishRequest
type WaitForFinishRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	AgentID   string `json:"agentId"`
	TimeoutMs *int   `json:"timeoutMs,omitempty"`
}

func (m *WaitForFinishRequest) MsgType() string { return "wait_for_finish_request" }

// FetchAgentsRequest
type FetchAgentsRequest struct {
	Type      string                `json:"type"`
	RequestID string                `json:"requestId"`
	Scope     *string               `json:"scope,omitempty"`
	Filter    *FetchAgentsFilter    `json:"filter,omitempty"`
	Sort      []FetchAgentsSortItem `json:"sort,omitempty"`
	Page      *FetchPage            `json:"page,omitempty"`
	Subscribe *FetchSubscribe       `json:"subscribe,omitempty"`
}

type FetchAgentsFilter struct {
	Labels            map[string]string      `json:"labels,omitempty"`
	ProjectKeys       []string               `json:"projectKeys,omitempty"`
	Statuses          []AgentLifecycleStatus `json:"statuses,omitempty"`
	IncludeArchived   *bool                  `json:"includeArchived,omitempty"`
	RequiresAttention *bool                  `json:"requiresAttention,omitempty"`
	ThinkingOptionID  *string                `json:"thinkingOptionId,omitempty"`
}

type FetchAgentsSortItem struct {
	Key       string `json:"key"`
	Direction string `json:"direction"` // "asc" | "desc"
}

type FetchPage struct {
	Limit  int     `json:"limit"`
	Cursor *string `json:"cursor,omitempty"`
}

type FetchSubscribe struct {
	SubscriptionID *string `json:"subscriptionId,omitempty"`
}

func (m *FetchAgentsRequest) MsgType() string { return "fetch_agents_request" }

// FetchAgentRequest
type FetchAgentRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	AgentID   string `json:"agentId"`
}

func (m *FetchAgentRequest) MsgType() string { return "fetch_agent_request" }

// FetchAgentHistoryRequest
type FetchAgentHistoryRequest struct {
	Type      string                `json:"type"`
	RequestID string                `json:"requestId"`
	Filter    *FetchAgentsFilter    `json:"filter,omitempty"`
	Sort      []FetchAgentsSortItem `json:"sort,omitempty"`
	Page      *FetchPage            `json:"page,omitempty"`
}

func (m *FetchAgentHistoryRequest) MsgType() string { return "fetch_agent_history_request" }

// FetchAgentTimelineRequest
type FetchAgentTimelineRequest struct {
	Type       string               `json:"type"`
	AgentID    string               `json:"agentId"`
	RequestID  string               `json:"requestId"`
	Direction  *string              `json:"direction,omitempty"` // "tail"|"before"|"after"
	Cursor     *AgentTimelineCursor `json:"cursor,omitempty"`
	Limit      *int                 `json:"limit,omitempty"`
	Projection *string              `json:"projection,omitempty"`
}

type AgentTimelineCursor struct {
	Epoch string `json:"epoch"`
	Seq   int    `json:"seq"`
}

func (m *FetchAgentTimelineRequest) MsgType() string { return "fetch_agent_timeline_request" }

// FetchWorkspacesRequest
type FetchWorkspacesRequest struct {
	Type      string                    `json:"type"`
	RequestID string                    `json:"requestId"`
	Filter    *FetchWorkspacesFilter    `json:"filter,omitempty"`
	Sort      []FetchWorkspacesSortItem `json:"sort,omitempty"`
	Page      *FetchPage                `json:"page,omitempty"`
	Subscribe *FetchSubscribe           `json:"subscribe,omitempty"`
}

type FetchWorkspacesFilter struct {
	Query     *string `json:"query,omitempty"`
	ProjectID *string `json:"projectId,omitempty"`
	IDPrefix  *string `json:"idPrefix,omitempty"`
}

type FetchWorkspacesSortItem struct {
	Key       string `json:"key"`
	Direction string `json:"direction"`
}

func (m *FetchWorkspacesRequest) MsgType() string { return "fetch_workspaces_request" }

// GetProvidersSnapshotRequest
type GetProvidersSnapshotRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *GetProvidersSnapshotRequest) MsgType() string { return "get_providers_snapshot_request" }

// OpenProjectRequest
type OpenProjectRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	Cwd       string `json:"cwd"`
}

func (m *OpenProjectRequest) MsgType() string { return "open_project_request" }

// OpenProjectResponse
type OpenProjectResponse struct {
	Type    string                     `json:"type"`
	Payload OpenProjectResponsePayload `json:"payload"`
}

type OpenProjectResponsePayload struct {
	RequestID string               `json:"requestId"`
	Workspace *WorkspaceDescriptor `json:"workspace,omitempty"`
	Error     *string              `json:"error"`
}

func (m *OpenProjectResponse) MsgType() string { return "open_project_response" }

// WorkspaceDescriptor describes a workspace.
type WorkspaceDescriptor struct {
	ID                 string                  `json:"id"`
	ProjectID          string                  `json:"projectId"`
	ProjectDisplayName string                  `json:"projectDisplayName"`
	ProjectRootPath    string                  `json:"projectRootPath"`
	WorkspaceDirectory string                  `json:"workspaceDirectory,omitempty"`
	ProjectKind        string                  `json:"projectKind"`
	WorkspaceKind      string                  `json:"workspaceKind"`
	Name               string                  `json:"name"`
	Status             string                  `json:"status"`
	ActivityAt         *string                 `json:"activityAt"`
	Scripts            []WorkspaceScript       `json:"scripts,omitempty"`
	GitRuntime         *WorkspaceGitRuntime    `json:"gitRuntime,omitempty"`
	GitHubRuntime      *WorkspaceGitHubRuntime `json:"githubRuntime,omitempty"`
}

// WorkspaceScript describes a running script in a workspace.
type WorkspaceScript struct {
	ScriptName string  `json:"scriptName"`
	Type       string  `json:"type,omitempty"`
	Hostname   string  `json:"hostname"`
	Port       *int    `json:"port,omitempty"`
	ProxyURL   *string `json:"proxyUrl,omitempty"`
	Lifecycle  string  `json:"lifecycle"`
	Health     *string `json:"health,omitempty"`
	ExitCode   *int    `json:"exitCode,omitempty"`
	TerminalID *string `json:"terminalId,omitempty"`
}

// WorkspaceGitRuntime describes git state in a workspace.
type WorkspaceGitRuntime struct {
	CurrentBranch        *string `json:"currentBranch,omitempty"`
	RemoteURL            *string `json:"remoteUrl,omitempty"`
	IsSoloOwnedWorktree *bool   `json:"isSoloOwnedWorktree,omitempty"`
	IsDirty              *bool   `json:"isDirty,omitempty"`
}

// WorkspaceGitHubRuntime describes GitHub state in a workspace.
type WorkspaceGitHubRuntime struct {
	FeaturesEnabled *bool `json:"featuresEnabled,omitempty"`
}

// SetAgentModeRequest
type SetAgentModeRequest struct {
	Type      string `json:"type"`
	AgentID   string `json:"agentId"`
	ModeID    string `json:"modeId"`
	RequestID string `json:"requestId"`
}

func (m *SetAgentModeRequest) MsgType() string { return "set_agent_mode_request" }

// SetAgentModelRequest
type SetAgentModelRequest struct {
	Type      string  `json:"type"`
	AgentID   string  `json:"agentId"`
	ModelID   *string `json:"modelId"`
	RequestID string  `json:"requestId"`
}

func (m *SetAgentModelRequest) MsgType() string { return "set_agent_model_request" }

// SetAgentThinkingRequest
type SetAgentThinkingRequest struct {
	Type             string  `json:"type"`
	AgentID          string  `json:"agentId"`
	ThinkingOptionID *string `json:"thinkingOptionId"`
	RequestID        string  `json:"requestId"`
}

func (m *SetAgentThinkingRequest) MsgType() string { return "set_agent_thinking_request" }

// SetAgentFeatureRequest
type SetAgentFeatureRequest struct {
	Type      string      `json:"type"`
	AgentID   string      `json:"agentId"`
	FeatureID string      `json:"featureId"`
	Value     interface{} `json:"value"`
	RequestID string      `json:"requestId"`
}

func (m *SetAgentFeatureRequest) MsgType() string { return "set_agent_feature_request" }

// AgentPermissionResponseMessage
type AgentPermissionResponseMessage struct {
	Type      string                  `json:"type"`
	AgentID   string                  `json:"agentId"`
	RequestID string                  `json:"requestId"`
	Response  AgentPermissionResponse `json:"response"`
}

func (m *AgentPermissionResponseMessage) MsgType() string { return "agent_permission_response" }

// RegisterPushTokenMessage
type RegisterPushTokenMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

func (m *RegisterPushTokenMessage) MsgType() string { return "register_push_token" }

// RestartServerRequest
type RestartServerRequest struct {
	Type      string  `json:"type"`
	Reason    *string `json:"reason,omitempty"`
	RequestID string  `json:"requestId"`
}

func (m *RestartServerRequest) MsgType() string { return "restart_server_request" }

// ShutdownServerRequest
type ShutdownServerRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *ShutdownServerRequest) MsgType() string { return "shutdown_server_request" }

// GetDaemonConfigRequest
type GetDaemonConfigRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *GetDaemonConfigRequest) MsgType() string { return "get_daemon_config_request" }

// SetDaemonConfigRequest
type SetDaemonConfigRequest struct {
	Type      string                 `json:"type"`
	RequestID string                 `json:"requestId"`
	Config    map[string]interface{} `json:"config"`
}

func (m *SetDaemonConfigRequest) MsgType() string { return "set_daemon_config_request" }

// GetDaemonConfigResponse
type GetDaemonConfigResponse struct {
	Type    string                         `json:"type"`
	Payload GetDaemonConfigResponsePayload `json:"payload"`
}

type GetDaemonConfigResponsePayload struct {
	RequestID string                 `json:"requestId"`
	Config    map[string]interface{} `json:"config"`
}

func (m *GetDaemonConfigResponse) MsgType() string { return "get_daemon_config_response" }

// SetDaemonConfigResponse
type SetDaemonConfigResponse struct {
	Type    string                         `json:"type"`
	Payload SetDaemonConfigResponsePayload `json:"payload"`
}

type SetDaemonConfigResponsePayload struct {
	RequestID string                 `json:"requestId"`
	Config    map[string]interface{} `json:"config"`
}

func (m *SetDaemonConfigResponse) MsgType() string { return "set_daemon_config_response" }

