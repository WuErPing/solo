package protocol

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
	Status        string                 `json:"status"`
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
