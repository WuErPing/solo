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

// --- Common Sub-Types ---

// AgentCapabilityFlags describes what an agent provider supports.
type AgentCapabilityFlags struct {
	SupportsStreaming          bool `json:"supportsStreaming"`
	SupportsSessionPersistence bool `json:"supportsSessionPersistence"`
	SupportsDynamicModes       bool `json:"supportsDynamicModes"`
	SupportsMcpServers         bool `json:"supportsMcpServers"`
	SupportsReasoningStream    bool `json:"supportsReasoningStream"`
	SupportsToolInvocations    bool `json:"supportsToolInvocations"`
}

// AgentMode describes an available mode for an agent.
type AgentMode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	ColorTier   string `json:"colorTier,omitempty"`
}

// AgentSelectOption is a select option for agent features.
type AgentSelectOption struct {
	ID          string                 `json:"id"`
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	IsDefault   bool                   `json:"isDefault,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AgentModelDefinition describes a model available for a provider.
type AgentModelDefinition struct {
	Provider                string                 `json:"provider"`
	ID                      string                 `json:"id"`
	Label                   string                 `json:"label"`
	Description             string                 `json:"description,omitempty"`
	IsDefault               bool                   `json:"isDefault,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
	ThinkingOptions         []AgentSelectOption    `json:"thinkingOptions,omitempty"`
	DefaultThinkingOptionID string                 `json:"defaultThinkingOptionId,omitempty"`
}

// AgentUsage tracks token usage for an agent session.
type AgentUsage struct {
	InputTokens             *float64 `json:"inputTokens,omitempty"`
	CachedInputTokens       *float64 `json:"cachedInputTokens,omitempty"`
	OutputTokens            *float64 `json:"outputTokens,omitempty"`
	TotalCostUSD            *float64 `json:"totalCostUsd,omitempty"`
	ContextWindowMaxTokens  *float64 `json:"contextWindowMaxTokens,omitempty"`
	ContextWindowUsedTokens *float64 `json:"contextWindowUsedTokens,omitempty"`
}

// AgentFeatureToggle represents a toggle feature.
type AgentFeatureToggle struct {
	Type        string `json:"type"` // "toggle"
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Tooltip     string `json:"tooltip,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Value       bool   `json:"value"`
}

// AgentFeatureSelect represents a select feature.
type AgentFeatureSelect struct {
	Type        string              `json:"type"` // "select"
	ID          string              `json:"id"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	Tooltip     string              `json:"tooltip,omitempty"`
	Icon        string              `json:"icon,omitempty"`
	Value       *string             `json:"value"`
	Options     []AgentSelectOption `json:"options"`
}

// AgentFeature is either a toggle or select feature.
// The Type field discriminates between them.
type AgentFeature struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Tooltip     string `json:"tooltip,omitempty"`
	Icon        string `json:"icon,omitempty"`
	// Toggle fields
	Value bool `json:"value,omitempty"`
	// Select fields
	SelectValue *string             `json:"selectValue,omitempty"`
	Options     []AgentSelectOption `json:"options,omitempty"`
}

// AgentPersistenceHandle describes how an agent session can be persisted.
type AgentPersistenceHandle struct {
	Provider     string                 `json:"provider"`
	SessionID    string                 `json:"sessionId"`
	NativeHandle string                 `json:"nativeHandle,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AgentRuntimeInfo contains runtime information about an agent session.
type AgentRuntimeInfo struct {
	Provider         string                 `json:"provider"`
	SessionID        *string                `json:"sessionId"`
	Model            *string                `json:"model,omitempty"`
	ThinkingOptionID *string                `json:"thinkingOptionId,omitempty"`
	ModeID           *string                `json:"modeId,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

// McpServerConfig describes an MCP server configuration.
type McpServerConfig struct {
	Type    string            `json:"type"` // "stdio", "http", "sse"
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// AgentSessionConfig is the configuration for creating an agent session.
type AgentSessionConfig struct {
	Provider         string                     `json:"provider"`
	Cwd              string                     `json:"cwd"`
	ModeID           *string                    `json:"modeId,omitempty"`
	Model            *string                    `json:"model,omitempty"`
	ThinkingOptionID *string                    `json:"thinkingOptionId,omitempty"`
	FeatureValues    map[string]interface{}     `json:"featureValues,omitempty"`
	Title            *string                    `json:"title,omitempty"`
	ApprovalPolicy   string                     `json:"approvalPolicy,omitempty"`
	SandboxMode      string                     `json:"sandboxMode,omitempty"`
	NetworkAccess    bool                       `json:"networkAccess,omitempty"`
	WebSearch        bool                       `json:"webSearch,omitempty"`
	Extra            map[string]interface{}     `json:"extra,omitempty"`
	SystemPrompt     string                     `json:"systemPrompt,omitempty"`
	McpServers       map[string]McpServerConfig `json:"mcpServers,omitempty"`
	OutputSchema     map[string]interface{}     `json:"outputSchema,omitempty"`
}

// ImageAttachment represents an image sent to an agent.
type ImageAttachment struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

// AgentAttachment represents an attachment (GitHub PR/Issue) sent to an agent.
type AgentAttachment struct {
	Type        string  `json:"type"`
	MimeType    string  `json:"mimeType"`
	Number      int     `json:"number,omitempty"`
	Title       string  `json:"title,omitempty"`
	URL         string  `json:"url,omitempty"`
	Body        *string `json:"body,omitempty"`
	BaseRefName *string `json:"baseRefName,omitempty"`
	HeadRefName *string `json:"headRefName,omitempty"`
}

// GitSetupOptions describes git setup for a new agent.
type GitSetupOptions struct {
	BaseBranch      *string `json:"baseBranch,omitempty"`
	CreateNewBranch *bool   `json:"createNewBranch,omitempty"`
	NewBranchName   *string `json:"newBranchName,omitempty"`
	CreateWorktree  *bool   `json:"createWorktree,omitempty"`
	WorktreeSlug    *string `json:"worktreeSlug,omitempty"`
	RefName         *string `json:"refName,omitempty"`
	Action          *string `json:"action,omitempty"` // "branch-off" | "checkout"
	GithubPRNumber  *int    `json:"githubPrNumber,omitempty"`
}

// AgentPermissionAction describes an action on a permission request.
type AgentPermissionAction struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Behavior string `json:"behavior"` // "allow" | "deny"
	Variant  string `json:"variant,omitempty"`
	Intent   string `json:"intent,omitempty"`
}

// AgentPermissionResponse is the client's response to a permission request.
type AgentPermissionResponse struct {
	Behavior           string                   `json:"behavior"` // "allow" | "deny"
	SelectedActionID   string                   `json:"selectedActionId,omitempty"`
	UpdatedInput       map[string]interface{}   `json:"updatedInput,omitempty"`
	UpdatedPermissions []map[string]interface{} `json:"updatedPermissions,omitempty"`
	Message            string                   `json:"message,omitempty"`
	Interrupt          bool                     `json:"interrupt,omitempty"`
}

// ProjectCheckoutLitePayload describes the checkout associated with an agent
// directory entry. The shape mirrors Solo's ProjectCheckoutLitePayloadSchema.
type ProjectCheckoutLitePayload struct {
	Cwd                  string  `json:"cwd"`
	IsGit                bool    `json:"isGit"`
	CurrentBranch        *string `json:"currentBranch"`
	RemoteURL            *string `json:"remoteUrl"`
	WorktreeRoot         *string `json:"worktreeRoot,omitempty"`
	IsSoloOwnedWorktree bool    `json:"isSoloOwnedWorktree"`
	MainRepoRoot         *string `json:"mainRepoRoot"`
}

// ProjectPlacementPayload groups an agent under a project in the directory UI.
type ProjectPlacementPayload struct {
	ProjectKey  string                     `json:"projectKey"`
	ProjectName string                     `json:"projectName"`
	Checkout    ProjectCheckoutLitePayload `json:"checkout"`
}

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

// --- Outbound Message Structs ---

// StatusMessage carries various status payloads (server_info, agent_created, etc.)
type StatusMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (m *StatusMessage) MsgType() string { return "status" }

// ServerInfoPayload is the first message sent after hello.
type ServerInfoPayload struct {
	Status       string              `json:"status"`
	ServerID     string              `json:"serverId"`
	Hostname     *string             `json:"hostname,omitempty"`
	Version      *string             `json:"version,omitempty"`
	Capabilities *ServerCapabilities `json:"capabilities,omitempty"`
	Features     *ServerFeatures     `json:"features,omitempty"`
}

type ServerCapabilities struct {
	Voice *VoiceCapabilities `json:"voice,omitempty"`
}

type VoiceCapabilities struct {
	Dictation VoiceFeatureStatus `json:"dictation"`
	Voice     VoiceFeatureStatus `json:"voice"`
}

type VoiceFeatureStatus struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

type ServerFeatures struct {
	ProvidersSnapshot *bool `json:"providersSnapshot,omitempty"`
}

// PongMessage
type PongMessage struct {
	Type    string      `json:"type"`
	Payload PongPayload `json:"payload"`
}

type PongPayload struct {
	RequestID        string `json:"requestId"`
	ClientSentAt     *int64 `json:"clientSentAt,omitempty"`
	ServerReceivedAt int64  `json:"serverReceivedAt"`
	ServerSentAt     int64  `json:"serverSentAt"`
}

func (m *PongMessage) MsgType() string { return "pong" }

// RpcErrorMessage
type RpcErrorMessage struct {
	Type    string          `json:"type"`
	Payload RpcErrorPayload `json:"payload"`
}

type RpcErrorPayload struct {
	RequestID   string  `json:"requestId"`
	RequestType *string `json:"requestType,omitempty"`
	Error       string  `json:"error"`
	Code        *string `json:"code,omitempty"`
}

func (m *RpcErrorMessage) MsgType() string { return "rpc_error" }

// AgentSnapshotPayload is the full state of an agent sent to clients.
type AgentSnapshotPayload struct {
	ID                        string                  `json:"id"`
	Provider                  string                  `json:"provider"`
	Cwd                       string                  `json:"cwd"`
	Model                     *string                 `json:"model"`
	Features                  []AgentFeature          `json:"features,omitempty"`
	ThinkingOptionID          *string                 `json:"thinkingOptionId,omitempty"`
	EffectiveThinkingOptionID *string                 `json:"effectiveThinkingOptionId,omitempty"`
	CreatedAt                 string                  `json:"createdAt"`
	UpdatedAt                 string                  `json:"updatedAt"`
	LastUserMessageAt         *string                 `json:"lastUserMessageAt"`
	Status                    AgentLifecycleStatus    `json:"status"`
	Capabilities              AgentCapabilityFlags    `json:"capabilities"`
	CurrentModeID             *string                 `json:"currentModeId"`
	AvailableModes            []AgentMode             `json:"availableModes"`
	PendingPermissions        []interface{}           `json:"pendingPermissions"`
	Persistence               *AgentPersistenceHandle `json:"persistence"`
	RuntimeInfo               *AgentRuntimeInfo       `json:"runtimeInfo,omitempty"`
	LastUsage                 *AgentUsage             `json:"lastUsage,omitempty"`
	LastError                 *string                 `json:"lastError,omitempty"`
	Title                     *string                 `json:"title"`
	Labels                    map[string]string       `json:"labels"`
	RequiresAttention         bool                    `json:"requiresAttention,omitempty"`
	AttentionReason           *string                 `json:"attentionReason,omitempty"`
	AttentionTimestamp        *string                 `json:"attentionTimestamp,omitempty"`
	ArchivedAt                *string                 `json:"archivedAt,omitempty"`
	ProviderUnavailable       bool                    `json:"providerUnavailable,omitempty"`
}

// AgentStreamMessage
type AgentStreamMessage struct {
	Type    string             `json:"type"`
	Payload AgentStreamPayload `json:"payload"`
}

type AgentStreamPayload struct {
	AgentID   string      `json:"agentId"`
	Event     interface{} `json:"event"`
	Timestamp string      `json:"timestamp"`
	Seq       *int        `json:"seq,omitempty"`
	Epoch     *string     `json:"epoch,omitempty"`
}

func (m *AgentStreamMessage) MsgType() string { return "agent_stream" }

// AgentUpdateMessage
type AgentUpdateMessage struct {
	Type    string             `json:"type"`
	Payload AgentUpdatePayload `json:"payload"`
}

type AgentUpdatePayload struct {
	Kind    string                   `json:"kind"` // "upsert" | "remove"
	Agent   *AgentSnapshotPayload    `json:"agent,omitempty"`
	AgentID string                   `json:"agentId,omitempty"` // for "remove" kind
	Project *ProjectPlacementPayload `json:"project,omitempty"`
}

func (m *AgentUpdateMessage) MsgType() string { return "agent_update" }

// FetchAgentsResponse
type FetchAgentsResponse struct {
	Type    string                     `json:"type"`
	Payload FetchAgentsResponsePayload `json:"payload"`
}

type FetchAgentsResponsePayload struct {
	RequestID      string             `json:"requestId"`
	SubscriptionID *string            `json:"subscriptionId,omitempty"`
	Entries        []FetchAgentsEntry `json:"entries"`
	PageInfo       FetchPageInfo      `json:"pageInfo"`
}

type FetchAgentsEntry struct {
	Agent   AgentSnapshotPayload    `json:"agent"`
	Project ProjectPlacementPayload `json:"project"`
}

type FetchPageInfo struct {
	NextCursor *string `json:"nextCursor"`
	PrevCursor *string `json:"prevCursor"`
	HasMore    bool    `json:"hasMore"`
}

func (m *FetchAgentsResponse) MsgType() string { return "fetch_agents_response" }

// FetchWorkspacesResponse
type FetchWorkspacesResponse struct {
	Type    string                         `json:"type"`
	Payload FetchWorkspacesResponsePayload `json:"payload"`
}

type FetchWorkspacesResponsePayload struct {
	RequestID string        `json:"requestId"`
	Entries   []interface{} `json:"entries"`
	PageInfo  FetchPageInfo `json:"pageInfo"`
}

func (m *FetchWorkspacesResponse) MsgType() string { return "fetch_workspaces_response" }

// WorkspaceUpdateMessage
type WorkspaceUpdateMessage struct {
	Type    string                 `json:"type"`
	Payload WorkspaceUpdatePayload `json:"payload"`
}

type WorkspaceUpdatePayload struct {
	Kind      string               `json:"kind"`
	Workspace *WorkspaceDescriptor `json:"workspace,omitempty"`
	ID        string               `json:"id,omitempty"`
}

func (m *WorkspaceUpdateMessage) MsgType() string { return "workspace_update" }

// GetProvidersSnapshotResponse
type GetProvidersSnapshotResponse struct {
	Type    string                              `json:"type"`
	Payload GetProvidersSnapshotResponsePayload `json:"payload"`
}

type GetProvidersSnapshotResponsePayload struct {
	RequestID   string                  `json:"requestId"`
	Entries     []ProviderSnapshotEntry `json:"entries"`
	GeneratedAt string                  `json:"generatedAt"`
}

func (m *GetProvidersSnapshotResponse) MsgType() string { return "get_providers_snapshot_response" }

// FetchAgentResponse
type FetchAgentResponse struct {
	Type    string                    `json:"type"`
	Payload FetchAgentResponsePayload `json:"payload"`
}

type FetchAgentResponsePayload struct {
	RequestID string                   `json:"requestId"`
	Agent     *AgentSnapshotPayload    `json:"agent"`
	Project   *ProjectPlacementPayload `json:"project,omitempty"`
	Error     *string                  `json:"error"`
}

func (m *FetchAgentResponse) MsgType() string { return "fetch_agent_response" }

// FetchAgentTimelineResponse
type FetchAgentTimelineResponse struct {
	Type    string                            `json:"type"`
	Payload FetchAgentTimelineResponsePayload `json:"payload"`
}

type FetchAgentTimelineResponsePayload struct {
	RequestID   string                    `json:"requestId"`
	AgentID     string                    `json:"agentId"`
	Agent       *AgentSnapshotPayload     `json:"agent"`
	Direction   string                    `json:"direction"`
	Projection  string                    `json:"projection"`
	Epoch       string                    `json:"epoch"`
	Reset       bool                      `json:"reset"`
	StaleCursor bool                      `json:"staleCursor"`
	Gap         bool                      `json:"gap"`
	Window      FetchAgentTimelineWindow  `json:"window"`
	StartCursor *AgentTimelineCursor      `json:"startCursor"`
	EndCursor   *AgentTimelineCursor      `json:"endCursor"`
	HasOlder    bool                      `json:"hasOlder"`
	HasNewer    bool                      `json:"hasNewer"`
	Entries     []FetchAgentTimelineEntry `json:"entries"`
	Error       *string                   `json:"error"`
}

type FetchAgentTimelineWindow struct {
	MinSeq  int `json:"minSeq"`
	MaxSeq  int `json:"maxSeq"`
	NextSeq int `json:"nextSeq"`
}

type FetchAgentTimelineEntry struct {
	Provider       string                 `json:"provider"`
	Item           map[string]interface{} `json:"item"`
	Timestamp      string                 `json:"timestamp"`
	SeqStart       int                    `json:"seqStart"`
	SeqEnd         int                    `json:"seqEnd"`
	SourceSeqRange []map[string]int       `json:"sourceSeqRanges"`
	Collapsed      []string               `json:"collapsed"`
}

func (m *FetchAgentTimelineResponse) MsgType() string { return "fetch_agent_timeline_response" }

// CreateAgentResponse (uses StatusMessage with status "agent_created" or "agent_create_failed")
// AgentCreatedPayload
type AgentCreatedPayload struct {
	Status    string               `json:"status"`
	Agent     AgentSnapshotPayload `json:"agent"`
	AgentID   string               `json:"agentId"`
	RequestID string               `json:"requestId"`
}

type AgentResumedPayload struct {
	Status       string               `json:"status"`
	Agent        AgentSnapshotPayload `json:"agent"`
	AgentID      string               `json:"agentId"`
	RequestID    string               `json:"requestId"`
	TimelineSize int                  `json:"timelineSize,omitempty"`
}

type AgentRefreshedPayload struct {
	Status       string `json:"status"`
	AgentID      string `json:"agentId"`
	RequestID    string `json:"requestId"`
	TimelineSize int    `json:"timelineSize,omitempty"`
}

// AgentCreateFailedPayload
type AgentCreateFailedPayload struct {
	Status    string  `json:"status"`
	RequestID string  `json:"requestId"`
	Error     string  `json:"error"`
	ErrorCode *string `json:"errorCode,omitempty"`
}

// AgentDeletedMessage
type AgentDeletedMessage struct {
	Type    string              `json:"type"`
	Payload AgentDeletedPayload `json:"payload"`
}

type AgentDeletedPayload struct {
	AgentID   string `json:"agentId"`
	RequestID string `json:"requestId"`
}

func (m *AgentDeletedMessage) MsgType() string { return "agent_deleted" }

// AgentArchivedMessage
type AgentArchivedMessage struct {
	Type    string               `json:"type"`
	Payload AgentArchivedPayload `json:"payload"`
}

type AgentArchivedPayload struct {
	AgentID    string `json:"agentId"`
	ArchivedAt string `json:"archivedAt"`
	RequestID  string `json:"requestId"`
}

func (m *AgentArchivedMessage) MsgType() string { return "agent_archived" }

// SendAgentMessageResponse
type SendAgentMessageResponse struct {
	Type    string                          `json:"type"`
	Payload SendAgentMessageResponsePayload `json:"payload"`
}

type SendAgentMessageResponsePayload struct {
	RequestID string  `json:"requestId"`
	AgentID   string  `json:"agentId"`
	Accepted  bool    `json:"accepted"`
	Error     *string `json:"error"`
}

func (m *SendAgentMessageResponse) MsgType() string { return "send_agent_message_response" }

// ClearAgentAttentionResponse
type ClearAgentAttentionResponse struct {
	Type    string                             `json:"type"`
	Payload ClearAgentAttentionResponsePayload `json:"payload"`
}

type ClearAgentAttentionResponsePayload struct {
	RequestID string                 `json:"requestId"`
	AgentID   interface{}            `json:"agentId"`
	Agents    []AgentSnapshotPayload `json:"agents"`
}

func (m *ClearAgentAttentionResponse) MsgType() string { return "clear_agent_attention_response" }

// WaitForFinishResponse
type WaitForFinishResponse struct {
	Type    string                       `json:"type"`
	Payload WaitForFinishResponsePayload `json:"payload"`
}

type WaitForFinishResponsePayload struct {
	RequestID   string                `json:"requestId"`
	Status      string                `json:"status"` // "idle" | "error" | "permission" | "timeout"
	Final       *AgentSnapshotPayload `json:"final"`
	Error       *string               `json:"error"`
	LastMessage *string               `json:"lastMessage"`
}

func (m *WaitForFinishResponse) MsgType() string { return "wait_for_finish_response" }

// SetAgentModeResponse
type SetAgentModeResponse struct {
	Type    string                      `json:"type"`
	Payload SetAgentModeResponsePayload `json:"payload"`
}

type SetAgentModeResponsePayload struct {
	RequestID string  `json:"requestId"`
	AgentID   string  `json:"agentId"`
	Accepted  bool    `json:"accepted"`
	Error     *string `json:"error"`
}

func (m *SetAgentModeResponse) MsgType() string { return "set_agent_mode_response" }

// SetAgentModelResponse
type SetAgentModelResponse struct {
	Type    string                       `json:"type"`
	Payload SetAgentModelResponsePayload `json:"payload"`
}

type SetAgentModelResponsePayload struct {
	RequestID string  `json:"requestId"`
	AgentID   string  `json:"agentId"`
	Accepted  bool    `json:"accepted"`
	Error     *string `json:"error"`
}

func (m *SetAgentModelResponse) MsgType() string { return "set_agent_model_response" }

// FetchAgentHistoryResponse
type FetchAgentHistoryResponse struct {
	Type    string                           `json:"type"`
	Payload FetchAgentHistoryResponsePayload `json:"payload"`
}

type FetchAgentHistoryResponsePayload struct {
	RequestID string             `json:"requestId"`
	Entries   []FetchAgentsEntry `json:"entries"`
	PageInfo  FetchPageInfo      `json:"pageInfo"`
}

func (m *FetchAgentHistoryResponse) MsgType() string { return "fetch_agent_history_response" }

// ProviderSnapshotEntry
type ProviderSnapshotEntry struct {
	Provider      string                 `json:"provider"`
	Status        ProviderStatus         `json:"status"`
	Enabled       bool                   `json:"enabled,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Models        []AgentModelDefinition `json:"models,omitempty"`
	Modes         []AgentMode            `json:"modes,omitempty"`
	FetchedAt     string                 `json:"fetchedAt,omitempty"`
	Label         string                 `json:"label,omitempty"`
	Description   string                 `json:"description,omitempty"`
	DefaultModeID *string                `json:"defaultModeId,omitempty"`
}

// ProvidersSnapshotUpdate
type ProvidersSnapshotUpdate struct {
	Type    string                   `json:"type"`
	Payload ProvidersSnapshotPayload `json:"payload"`
}

type ProvidersSnapshotPayload struct {
	Cwd         *string                 `json:"cwd,omitempty"`
	Entries     []ProviderSnapshotEntry `json:"entries"`
	GeneratedAt string                  `json:"generatedAt"`
}

func (m *ProvidersSnapshotUpdate) MsgType() string { return "providers_snapshot_update" }

// --- Editor Message Types ---

type ListAvailableEditorsRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *ListAvailableEditorsRequest) MsgType() string { return "list_available_editors_request" }

type ListAvailableEditorsResponse struct {
	Type    string                      `json:"type"`
	Payload ListAvailableEditorsPayload `json:"payload"`
}

type ListAvailableEditorsPayload struct {
	RequestID string         `json:"requestId"`
	Editors   []EditorTarget `json:"editors"`
	Error     *string        `json:"error"`
}

type EditorTarget struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (m *ListAvailableEditorsResponse) MsgType() string { return "list_available_editors_response" }

// --- Terminal Message Types ---

type ListTerminalsRequest struct {
	Type      string  `json:"type"`
	Cwd       *string `json:"cwd,omitempty"`
	RequestID string  `json:"requestId"`
}

func (m *ListTerminalsRequest) MsgType() string { return "list_terminals_request" }

type ListTerminalsResponse struct {
	Type    string               `json:"type"`
	Payload ListTerminalsPayload `json:"payload"`
}

type ListTerminalsPayload struct {
	Cwd       *string        `json:"cwd,omitempty"`
	Terminals []TerminalInfo `json:"terminals"`
	RequestID string         `json:"requestId"`
}

type TerminalInfo struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Cwd   string  `json:"cwd"`
	Title *string `json:"title,omitempty"`
}

func (m *ListTerminalsResponse) MsgType() string { return "list_terminals_response" }

type CreateTerminalRequest struct {
	Type      string   `json:"type"`
	Cwd       string   `json:"cwd"`
	Name      *string  `json:"name,omitempty"`
	AgentID   *string  `json:"agentId,omitempty"`
	Command   *string  `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	RequestID string   `json:"requestId"`
}

func (m *CreateTerminalRequest) MsgType() string { return "create_terminal_request" }

type CreateTerminalResponse struct {
	Type    string                `json:"type"`
	Payload CreateTerminalPayload `json:"payload"`
}

type CreateTerminalPayload struct {
	Terminal  *TerminalInfo `json:"terminal"`
	Error     *string       `json:"error"`
	RequestID string        `json:"requestId"`
}

func (m *CreateTerminalResponse) MsgType() string { return "create_terminal_response" }

type KillTerminalRequest struct {
	Type       string `json:"type"`
	TerminalID string `json:"terminalId"`
	RequestID  string `json:"requestId"`
}

func (m *KillTerminalRequest) MsgType() string { return "kill_terminal_request" }

type SubscribeTerminalsRequest struct {
	Type string `json:"type"`
	Cwd  string `json:"cwd"`
}

func (m *SubscribeTerminalsRequest) MsgType() string { return "subscribe_terminals_request" }

type UnsubscribeTerminalsRequest struct {
	Type string `json:"type"`
	Cwd  string `json:"cwd"`
}

func (m *UnsubscribeTerminalsRequest) MsgType() string { return "unsubscribe_terminals_request" }

type SubscribeTerminalRequest struct {
	Type       string `json:"type"`
	TerminalID string `json:"terminalId"`
	RequestID  string `json:"requestId"`
}

func (m *SubscribeTerminalRequest) MsgType() string { return "subscribe_terminal_request" }

type UnsubscribeTerminalRequest struct {
	Type       string `json:"type"`
	TerminalID string `json:"terminalId"`
}

func (m *UnsubscribeTerminalRequest) MsgType() string { return "unsubscribe_terminal_request" }

type TerminalInputMessage struct {
	Type       string          `json:"type"`
	TerminalID string          `json:"terminalId"`
	Message    json.RawMessage `json:"message"`
}

func (m *TerminalInputMessage) MsgType() string { return "terminal_input" }

type CaptureTerminalRequest struct {
	Type       string `json:"type"`
	TerminalID string `json:"terminalId"`
	Start      *int   `json:"start,omitempty"`
	End        *int   `json:"end,omitempty"`
	StripAnsi  *bool  `json:"stripAnsi,omitempty"`
	RequestID  string `json:"requestId"`
}

func (m *CaptureTerminalRequest) MsgType() string { return "capture_terminal_request" }

type StartWorkspaceScriptRequest struct {
	Type        string `json:"type"`
	WorkspaceID string `json:"workspaceId"`
	ScriptName  string `json:"scriptName"`
	RequestID   string `json:"requestId"`
}

func (m *StartWorkspaceScriptRequest) MsgType() string { return "start_workspace_script_request" }

type StartWorkspaceScriptResponse struct {
	Type    string                              `json:"type"`
	Payload StartWorkspaceScriptResponsePayload `json:"payload"`
}

type StartWorkspaceScriptResponsePayload struct {
	RequestID  string  `json:"requestId"`
	ScriptName string  `json:"scriptName"`
	Hostname   string  `json:"hostname"`
	Port       int     `json:"port"`
	ProxyURL   string  `json:"proxyUrl,omitempty"`
	TerminalID string  `json:"terminalId,omitempty"`
	Error      *string `json:"error"`
}

func (m *StartWorkspaceScriptResponse) MsgType() string { return "start_workspace_script_response" }

// FileExplorerRequest
type FileExplorerRequest struct {
	Type      string  `json:"type"`
	Cwd       string  `json:"cwd"`
	Path      *string `json:"path,omitempty"`
	Mode      string  `json:"mode"` // "list" or "file"
	RequestID string  `json:"requestId"`
}

func (m *FileExplorerRequest) MsgType() string { return "file_explorer_request" }

// ProjectIconRequest
type ProjectIconRequest struct {
	Type      string `json:"type"`
	Cwd       string `json:"cwd"`
	RequestID string `json:"requestId"`
}

func (m *ProjectIconRequest) MsgType() string { return "project_icon_request" }

// FileExplorerResponse
type FileExplorerResponse struct {
	Type    string                      `json:"type"`
	Payload FileExplorerResponsePayload `json:"payload"`
}

type FileExplorerResponsePayload struct {
	Cwd       string                 `json:"cwd"`
	Path      string                 `json:"path"`
	Mode      string                 `json:"mode"`
	Directory *FileExplorerDirectory `json:"directory"`
	File      *FileExplorerFile      `json:"file"`
	Error     *string                `json:"error"`
	RequestID string                 `json:"requestId"`
}

type FileExplorerDirectory struct {
	Path    string              `json:"path"`
	Entries []FileExplorerEntry `json:"entries"`
}

type FileExplorerEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"` // "file" or "directory"
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modifiedAt"`
}

type FileExplorerFile struct {
	Path       string  `json:"path"`
	Kind       string  `json:"kind"`     // "text", "image", "binary"
	Encoding   string  `json:"encoding"` // "utf-8", "base64", "none"
	Content    *string `json:"content,omitempty"`
	MimeType   *string `json:"mimeType,omitempty"`
	Size       int64   `json:"size"`
	ModifiedAt string  `json:"modifiedAt"`
}

func (m *FileExplorerResponse) MsgType() string { return "file_explorer_response" }

// ProjectIcon
type ProjectIcon struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

// ProjectIconResponse
type ProjectIconResponse struct {
	Type    string                     `json:"type"`
	Payload ProjectIconResponsePayload `json:"payload"`
}

type ProjectIconResponsePayload struct {
	Cwd       string       `json:"cwd"`
	Icon      *ProjectIcon `json:"icon"`
	Error     *string      `json:"error"`
	RequestID string       `json:"requestId"`
}

// DirectorySuggestionsRequest
type DirectorySuggestionsRequest struct {
	Type               string `json:"type"`
	Query              string `json:"query"`
	Cwd                string `json:"cwd,omitempty"`
	IncludeFiles       *bool  `json:"includeFiles,omitempty"`
	IncludeDirectories *bool  `json:"includeDirectories,omitempty"`
	Limit              *int   `json:"limit,omitempty"`
	RequestID          string `json:"requestId"`
}

func (m *DirectorySuggestionsRequest) MsgType() string { return "directory_suggestions_request" }

type DirectorySuggestionsResponse struct {
	Type    string                      `json:"type"`
	Payload DirectorySuggestionsPayload `json:"payload"`
}

type DirectorySuggestionsPayload struct {
	Directories []string                   `json:"directories"`
	Entries     []DirectorySuggestionEntry `json:"entries,omitempty"`
	Error       *string                    `json:"error"`
	RequestID   string                     `json:"requestId"`
}

type DirectorySuggestionEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // "file" or "directory"
}

func (m *DirectorySuggestionsResponse) MsgType() string { return "directory_suggestions_response" }

func (m *ProjectIconResponse) MsgType() string { return "project_icon_response" }

// ListCommandsRequest
type ListCommandsRequest struct {
	Type        string                   `json:"type"`
	AgentID     string                   `json:"agentId"`
	DraftConfig *ListCommandsDraftConfig `json:"draftConfig,omitempty"`
	RequestID   string                   `json:"requestId"`
}

type ListCommandsDraftConfig struct {
	Provider         string                 `json:"provider"`
	Cwd              string                 `json:"cwd"`
	ModeID           string                 `json:"modeId,omitempty"`
	Model            string                 `json:"model,omitempty"`
	ThinkingOptionID string                 `json:"thinkingOptionId,omitempty"`
	FeatureValues    map[string]interface{} `json:"featureValues,omitempty"`
}

func (m *ListCommandsRequest) MsgType() string { return "list_commands_request" }

// ListProviderFeaturesRequest
type ListProviderFeaturesRequest struct {
	Type        string                  `json:"type"`
	DraftConfig ListCommandsDraftConfig `json:"draftConfig"`
	RequestID   string                  `json:"requestId"`
}

func (m *ListProviderFeaturesRequest) MsgType() string { return "list_provider_features_request" }

type ListProviderFeaturesResponse struct {
	Type    string                      `json:"type"`
	Payload ListProviderFeaturesPayload `json:"payload"`
}

type ListProviderFeaturesPayload struct {
	Provider  string         `json:"provider"`
	Features  []AgentFeature `json:"features,omitempty"`
	Error     *string        `json:"error,omitempty"`
	FetchedAt string         `json:"fetchedAt"`
	RequestID string         `json:"requestId"`
}

func (m *ListProviderFeaturesResponse) MsgType() string { return "list_provider_features_response" }

// CheckoutStatusRequest
type CheckoutStatusRequest struct {
	Type      string `json:"type"`
	Cwd       string `json:"cwd"`
	RequestID string `json:"requestId"`
}

func (m *CheckoutStatusRequest) MsgType() string { return "checkout_status_request" }

type CheckoutStatusResponse struct {
	Type    string                `json:"type"`
	Payload CheckoutStatusPayload `json:"payload"`
}

type CheckoutStatusPayload struct {
	RequestID string  `json:"requestId"`
	Branch    *string `json:"branch,omitempty"`
	Ahead     *int    `json:"ahead,omitempty"`
	Behind    *int    `json:"behind,omitempty"`
	Error     *string `json:"error,omitempty"`
}

func (m *CheckoutStatusResponse) MsgType() string { return "checkout_status_response" }

type ListCommandsResponse struct {
	Type    string              `json:"type"`
	Payload ListCommandsPayload `json:"payload"`
}

type ListCommandsPayload struct {
	AgentID   string              `json:"agentId"`
	Commands  []AgentSlashCommand `json:"commands"`
	Error     *string             `json:"error"`
	RequestID string              `json:"requestId"`
}

type AgentSlashCommand struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ArgumentHint string `json:"argumentHint"`
}

func (m *ListCommandsResponse) MsgType() string { return "list_commands_response" }

// --- Worktree Setup Sub-Types ---

type WorktreeSetupCommandSnapshot struct {
	Index      int    `json:"index"`
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	Log        string `json:"log"`
	Status     string `json:"status"` // "running" | "completed" | "failed"
	ExitCode   *int   `json:"exitCode"`
	DurationMs *int   `json:"durationMs,omitempty"`
}

type WorktreeSetupDetailPayload struct {
	Type         string                         `json:"type"` // "worktree_setup"
	WorktreePath string                         `json:"worktreePath"`
	BranchName   string                         `json:"branchName"`
	Log          string                         `json:"log"`
	Commands     []WorktreeSetupCommandSnapshot `json:"commands"`
	Truncated    *bool                          `json:"truncated,omitempty"`
}

type WorkspaceSetupSnapshot struct {
	Status string                     `json:"status"` // "running" | "completed" | "failed"
	Detail WorktreeSetupDetailPayload `json:"detail"`
	Error  *string                    `json:"error"`
}

// --- Worktree Outbound Messages ---

type CreateSoloWorktreeResponse struct {
	Type    string                             `json:"type"`
	Payload CreateSoloWorktreeResponsePayload `json:"payload"`
}

type CreateSoloWorktreeResponsePayload struct {
	Workspace       *WorkspaceDescriptor `json:"workspace"`
	Error           *string              `json:"error"`
	ErrorCode       *string              `json:"errorCode,omitempty"`
	SetupTerminalID *string              `json:"setupTerminalId"`
	RequestID       string               `json:"requestId"`
}

func (m *CreateSoloWorktreeResponse) MsgType() string { return "create_solo_worktree_response" }

type WorkspaceSetupProgressMessage struct {
	Type    string                        `json:"type"`
	Payload WorkspaceSetupProgressPayload `json:"payload"`
}

type WorkspaceSetupProgressPayload struct {
	WorkspaceID string                     `json:"workspaceId"`
	Status      string                     `json:"status"` // "running" | "completed" | "failed"
	Detail      WorktreeSetupDetailPayload `json:"detail"`
	Error       *string                    `json:"error"`
}

func (m *WorkspaceSetupProgressMessage) MsgType() string { return "workspace_setup_progress" }

type WorkspaceSetupStatusResponse struct {
	Type    string                              `json:"type"`
	Payload WorkspaceSetupStatusResponsePayload `json:"payload"`
}

type WorkspaceSetupStatusResponsePayload struct {
	RequestID   string                  `json:"requestId"`
	WorkspaceID string                  `json:"workspaceId"`
	Snapshot    *WorkspaceSetupSnapshot `json:"snapshot"`
}

func (m *WorkspaceSetupStatusResponse) MsgType() string { return "workspace_setup_status_response" }

// --- Solo-compat inbound messages ---

type CheckoutPrStatusRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *CheckoutPrStatusRequest) MsgType() string { return "checkout_pr_status_request" }

type WorkspaceSetupStatusRequest struct {
	Type        string `json:"type"`
	WorkspaceID string `json:"workspaceId"`
	RequestID   string `json:"requestId"`
}

func (m *WorkspaceSetupStatusRequest) MsgType() string { return "workspace_setup_status_request" }

type ReadProjectConfigRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	RepoRoot  string `json:"repoRoot"`
}

func (m *ReadProjectConfigRequest) MsgType() string { return "read_project_config_request" }

type WriteProjectConfigRequest struct {
	Type             string                 `json:"type"`
	RequestID        string                 `json:"requestId"`
	RepoRoot         string                 `json:"repoRoot"`
	Config           map[string]interface{} `json:"config"`
	ExpectedRevision *ProjectConfigRevision `json:"expectedRevision"`
}

func (m *WriteProjectConfigRequest) MsgType() string { return "write_project_config_request" }

type ProjectConfigRevision struct {
	MtimeMs float64 `json:"mtimeMs"`
	Size    int64   `json:"size"`
}

type ProjectConfigRPCError struct {
	Code            string                 `json:"code"`
	CurrentRevision *ProjectConfigRevision `json:"currentRevision,omitempty"`
}

type ReadProjectConfigResponse struct {
	Type    string                           `json:"type"`
	Payload ReadProjectConfigResponsePayload `json:"payload"`
}

type ReadProjectConfigResponsePayload struct {
	RequestID string                 `json:"requestId"`
	RepoRoot  string                 `json:"repoRoot"`
	OK        bool                   `json:"ok"`
	Config    map[string]interface{} `json:"config"`
	Revision  *ProjectConfigRevision `json:"revision"`
	Error     *ProjectConfigRPCError `json:"error,omitempty"`
}

func (m *ReadProjectConfigResponse) MsgType() string { return "read_project_config_response" }

type WriteProjectConfigResponse struct {
	Type    string                            `json:"type"`
	Payload WriteProjectConfigResponsePayload `json:"payload"`
}

type WriteProjectConfigResponsePayload struct {
	RequestID string                 `json:"requestId"`
	RepoRoot  string                 `json:"repoRoot"`
	OK        bool                   `json:"ok"`
	Config    map[string]interface{} `json:"config,omitempty"`
	Revision  *ProjectConfigRevision `json:"revision,omitempty"`
	Error     *ProjectConfigRPCError `json:"error,omitempty"`
}

func (m *WriteProjectConfigResponse) MsgType() string { return "write_project_config_response" }

type CreateSoloWorktreeRequest struct {
	Type           string          `json:"type"`
	Cwd            string          `json:"cwd"`
	WorktreeSlug   *string         `json:"worktreeSlug,omitempty"`
	Attachments    json.RawMessage `json:"attachments,omitempty"`
	RefName        *string         `json:"refName,omitempty"`
	Action         *string         `json:"action,omitempty"` // "branch-off" | "checkout"
	GithubPRNumber *int            `json:"githubPrNumber,omitempty"`
	RequestID      string          `json:"requestId"`
}

func (m *CreateSoloWorktreeRequest) MsgType() string { return "create_solo_worktree_request" }

type ArchiveWorkspaceRequest struct {
	Type        string `json:"type"`
	WorkspaceID string `json:"workspaceId"`
	RequestID   string `json:"requestId"`
}

func (m *ArchiveWorkspaceRequest) MsgType() string { return "archive_workspace_request" }

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
}
