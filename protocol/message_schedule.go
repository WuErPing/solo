package protocol

// --- Schedule Cadence ---

type ScheduleCadence struct {
	Type       string `json:"type"` // "cron" | "every"
	Expression string `json:"expression,omitempty"`
	EveryMs    int    `json:"everyMs,omitempty"`
	Timezone   string `json:"timezone,omitempty"` // IANA timezone name, e.g. "Asia/Shanghai". Defaults to UTC.
}

// --- Schedule Target ---

type ScheduleTarget struct {
	Type    string               `json:"type"` // "agent" | "new-agent"
	AgentID string               `json:"agentId,omitempty"`
	Config  *ScheduleAgentConfig `json:"config,omitempty"`
}

type ScheduleAgentConfig struct {
	Provider         string                 `json:"provider"`
	Cwd              string                 `json:"cwd"`
	ModeID           *string                `json:"modeId,omitempty"`
	Model            *string                `json:"model,omitempty"`
	ThinkingOptionID *string                `json:"thinkingOptionId,omitempty"`
	Title            *string                `json:"title,omitempty"`
	ApprovalPolicy   string                 `json:"approvalPolicy,omitempty"`
	SandboxMode      string                 `json:"sandboxMode,omitempty"`
	NetworkAccess    bool                   `json:"networkAccess,omitempty"`
	WebSearch        bool                   `json:"webSearch,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
	SystemPrompt     string                 `json:"systemPrompt,omitempty"`
	McpServers       map[string]interface{} `json:"mcpServers,omitempty"`
}

// --- Schedule Run ---

type ScheduleRun struct {
	ID           string  `json:"id"`
	ScheduledFor string  `json:"scheduledFor"`
	StartedAt    string  `json:"startedAt"`
	EndedAt      *string `json:"endedAt"`
	Status       string  `json:"status"` // "running" | "succeeded" | "failed"
	AgentID      *string `json:"agentId"`
	Output       *string `json:"output"`
	Error        *string `json:"error"`
}

// --- Stored Schedule ---

type StoredSchedule struct {
	ID        string          `json:"id"`
	Name      *string         `json:"name"`
	Prompt    string          `json:"prompt"`
	Cadence   ScheduleCadence `json:"cadence"`
	Target    ScheduleTarget  `json:"target"`
	Cwd       *string         `json:"cwd,omitempty"`
	Status    string          `json:"status"` // "active" | "paused" | "completed"
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	NextRunAt *string         `json:"nextRunAt"`
	LastRunAt *string         `json:"lastRunAt"`
	PausedAt  *string         `json:"pausedAt"`
	ExpiresAt *string         `json:"expiresAt"`
	MaxRuns   *int            `json:"maxRuns"`
	Runs      []ScheduleRun   `json:"runs"`
}

// ScheduleSummary omits runs from StoredSchedule.
type ScheduleSummary struct {
	ID        string          `json:"id"`
	Name      *string         `json:"name"`
	Prompt    string          `json:"prompt"`
	Cadence   ScheduleCadence `json:"cadence"`
	Target    ScheduleTarget  `json:"target"`
	Cwd       *string         `json:"cwd,omitempty"`
	Status    string          `json:"status"` // "active" | "paused" | "completed"
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	NextRunAt *string         `json:"nextRunAt"`
	LastRunAt *string         `json:"lastRunAt"`
	PausedAt  *string         `json:"pausedAt"`
	ExpiresAt *string         `json:"expiresAt"`
	MaxRuns   *int            `json:"maxRuns"`
}

// --- Inbound Requests ---

type ScheduleCreateRequest struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId"`
	Prompt    string          `json:"prompt"`
	Name      string          `json:"name,omitempty"`
	Cadence   ScheduleCadence `json:"cadence"`
	Target    ScheduleTarget  `json:"target"`
	Cwd       *string         `json:"cwd,omitempty"`
	MaxRuns   *int            `json:"maxRuns,omitempty"`
	ExpiresAt string          `json:"expiresAt,omitempty"`
}

func (m ScheduleCreateRequest) MsgType() string { return "schedule/create" }

type ScheduleListRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m ScheduleListRequest) MsgType() string { return "schedule/list" }

type ScheduleInspectRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	ScheduleID string `json:"scheduleId"`
}

func (m ScheduleInspectRequest) MsgType() string { return "schedule/inspect" }

type ScheduleLogsRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	ScheduleID string `json:"scheduleId"`
}

func (m ScheduleLogsRequest) MsgType() string { return "schedule/logs" }

type SchedulePauseRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	ScheduleID string `json:"scheduleId"`
}

func (m SchedulePauseRequest) MsgType() string { return "schedule/pause" }

type ScheduleResumeRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	ScheduleID string `json:"scheduleId"`
}

func (m ScheduleResumeRequest) MsgType() string { return "schedule/resume" }

type ScheduleDeleteRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	ScheduleID string `json:"scheduleId"`
}

func (m ScheduleDeleteRequest) MsgType() string { return "schedule/delete" }

type ScheduleUpdateRequest struct {
	Type       string          `json:"type"`
	RequestID  string          `json:"requestId"`
	ScheduleID string          `json:"scheduleId"`
	Prompt     string          `json:"prompt"`
	Name       string          `json:"name,omitempty"`
	Cadence    ScheduleCadence `json:"cadence"`
	Target     ScheduleTarget  `json:"target"`
	Cwd        *string         `json:"cwd,omitempty"`
	MaxRuns    *int            `json:"maxRuns,omitempty"`
	ExpiresAt  string          `json:"expiresAt,omitempty"`
}

func (m ScheduleUpdateRequest) MsgType() string { return "schedule/update" }

// --- Outbound Responses ---

type ScheduleCreateResponse struct {
	Type    string                        `json:"type"`
	Payload ScheduleCreateResponsePayload `json:"payload"`
}

func (m ScheduleCreateResponse) MsgType() string { return "schedule/create/response" }

type ScheduleCreateResponsePayload struct {
	RequestID string           `json:"requestId"`
	Schedule  *ScheduleSummary `json:"schedule"`
	Error     *string          `json:"error"`
}

type ScheduleUpdateResponse struct {
	Type    string                        `json:"type"`
	Payload ScheduleUpdateResponsePayload `json:"payload"`
}

func (m ScheduleUpdateResponse) MsgType() string { return "schedule/update/response" }

type ScheduleUpdateResponsePayload struct {
	RequestID  string           `json:"requestId"`
	ScheduleID string           `json:"scheduleId"`
	Schedule   *ScheduleSummary `json:"schedule"`
	Error      *string          `json:"error"`
}

type ScheduleListResponse struct {
	Type    string                      `json:"type"`
	Payload ScheduleListResponsePayload `json:"payload"`
}

func (m ScheduleListResponse) MsgType() string { return "schedule/list/response" }

type ScheduleListResponsePayload struct {
	RequestID string            `json:"requestId"`
	Schedules []ScheduleSummary `json:"schedules"`
	Error     *string           `json:"error"`
}

type ScheduleInspectResponse struct {
	Type    string                         `json:"type"`
	Payload ScheduleInspectResponsePayload `json:"payload"`
}

func (m ScheduleInspectResponse) MsgType() string { return "schedule/inspect/response" }

type ScheduleInspectResponsePayload struct {
	RequestID string          `json:"requestId"`
	Schedule  *StoredSchedule `json:"schedule"`
	Error     *string         `json:"error"`
}

type ScheduleLogsResponse struct {
	Type    string                      `json:"type"`
	Payload ScheduleLogsResponsePayload `json:"payload"`
}

func (m ScheduleLogsResponse) MsgType() string { return "schedule/logs/response" }

type ScheduleLogsResponsePayload struct {
	RequestID string        `json:"requestId"`
	Runs      []ScheduleRun `json:"runs"`
	Error     *string       `json:"error"`
}

type SchedulePauseResponse struct {
	Type    string                       `json:"type"`
	Payload SchedulePauseResponsePayload `json:"payload"`
}

func (m SchedulePauseResponse) MsgType() string { return "schedule/pause/response" }

type SchedulePauseResponsePayload struct {
	RequestID string           `json:"requestId"`
	Schedule  *ScheduleSummary `json:"schedule"`
	Error     *string          `json:"error"`
}

type ScheduleResumeResponse struct {
	Type    string                        `json:"type"`
	Payload ScheduleResumeResponsePayload `json:"payload"`
}

func (m ScheduleResumeResponse) MsgType() string { return "schedule/resume/response" }

type ScheduleResumeResponsePayload struct {
	RequestID string           `json:"requestId"`
	Schedule  *ScheduleSummary `json:"schedule"`
	Error     *string          `json:"error"`
}

type ScheduleDeleteResponse struct {
	Type    string                        `json:"type"`
	Payload ScheduleDeleteResponsePayload `json:"payload"`
}

func (m ScheduleDeleteResponse) MsgType() string { return "schedule/delete/response" }

type ScheduleDeleteResponsePayload struct {
	RequestID  string  `json:"requestId"`
	ScheduleID string  `json:"scheduleId"`
	Error      *string `json:"error"`
}
