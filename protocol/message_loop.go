package protocol

// --- Shared Data Structs ---

type LoopLogEntry struct {
	Seq       int    `json:"seq"`
	Timestamp string `json:"timestamp"`
	Iteration *int   `json:"iteration"`
	Source    string `json:"source"` // "loop" | "worker" | "verifier" | "verify-check"
	Level     string `json:"level"`  // "info" | "error"
	Text      string `json:"text"`
}

type LoopVerifyCheckResult struct {
	Command     string `json:"command"`
	ExitCode    int    `json:"exitCode"`
	Passed      bool   `json:"passed"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
}

type LoopVerifyPromptResult struct {
	Passed          bool    `json:"passed"`
	Reason          string  `json:"reason"`
	VerifierAgentID *string `json:"verifierAgentId"`
	StartedAt       string  `json:"startedAt"`
	CompletedAt     string  `json:"completedAt"`
}

type LoopIterationRecord struct {
	Index             int                     `json:"index"`
	WorkerAgentID     *string                 `json:"workerAgentId"`
	WorkerStartedAt   string                  `json:"workerStartedAt"`
	WorkerCompletedAt *string                 `json:"workerCompletedAt"`
	VerifierAgentID   *string                 `json:"verifierAgentId"`
	Status            string                  `json:"status"`        // "running" | "succeeded" | "failed" | "stopped"
	WorkerOutcome     *string                 `json:"workerOutcome"` // "completed" | "failed" | "canceled"
	FailureReason     *string                 `json:"failureReason"`
	VerifyChecks      []LoopVerifyCheckResult `json:"verifyChecks"`
	VerifyPrompt      *LoopVerifyPromptResult `json:"verifyPrompt"`
}

type LoopRecord struct {
	ID                    string                `json:"id"`
	TemplateID            string                `json:"templateID,omitempty"`
	Name                  *string               `json:"name"`
	Prompt                string                `json:"prompt"`
	Cwd                   string                `json:"cwd"`
	VerifyPrompt          *string               `json:"verifyPrompt"`
	VerifyChecks          []string              `json:"verifyChecks"`
	Archive               bool                  `json:"archive"`
	SleepMs               int                   `json:"sleepMs"`
	MaxIterations         *int                  `json:"maxIterations"`
	MaxTimeMs             *int                  `json:"maxTimeMs"`
	Status                string                `json:"status"` // "running" | "succeeded" | "failed" | "stopped"
	CreatedAt             string                `json:"createdAt"`
	UpdatedAt             string                `json:"updatedAt"`
	StartedAt             string                `json:"startedAt"`
	CompletedAt           *string               `json:"completedAt"`
	StopRequestedAt       *string               `json:"stopRequestedAt"`
	Iterations            []LoopIterationRecord `json:"iterations"`
	Logs                  []LoopLogEntry        `json:"logs"`
	NextLogSeq            int                   `json:"nextLogSeq"`
	ActiveIteration       *int                  `json:"activeIteration"`
	ActiveWorkerAgentID   *string               `json:"activeWorkerAgentId"`
	ActiveVerifierAgentID *string               `json:"activeVerifierAgentId"`

	// AgentTemplate is the base template for any agent this loop creates.
	// It replaces Provider/Model for new code.
	AgentTemplate *AgentTemplate `json:"agentTemplate,omitempty"`

	// WorkerAgentTemplate overrides AgentTemplate for the worker agent.
	WorkerAgentTemplate *AgentTemplate `json:"workerAgentTemplate,omitempty"`

	// VerifierAgentTemplate overrides AgentTemplate for the verifier agent.
	VerifierAgentTemplate *AgentTemplate `json:"verifierAgentTemplate,omitempty"`

	// Provider is the legacy default provider for the loop. It is ignored when
	// AgentTemplate or WorkerAgentTemplate is set.
	// Deprecated: use AgentTemplate instead.
	Provider string `json:"provider"`

	// Model is the legacy default model for the loop. It is ignored when
	// AgentTemplate or WorkerAgentTemplate is set.
	// Deprecated: use AgentTemplate instead.
	Model *string `json:"model,omitempty"`

	// WorkerProvider is the legacy worker provider override. It is ignored when
	// WorkerAgentTemplate is set.
	// Deprecated: use WorkerAgentTemplate instead.
	WorkerProvider *string `json:"workerProvider,omitempty"`

	// WorkerModel is the legacy worker model override. It is ignored when
	// WorkerAgentTemplate is set.
	// Deprecated: use WorkerAgentTemplate instead.
	WorkerModel *string `json:"workerModel,omitempty"`

	// VerifierProvider is the legacy verifier provider override. It is ignored
	// when VerifierAgentTemplate is set.
	// Deprecated: use VerifierAgentTemplate instead.
	VerifierProvider *string `json:"verifierProvider,omitempty"`

	// VerifierModel is the legacy verifier model override. It is ignored when
	// VerifierAgentTemplate is set.
	// Deprecated: use VerifierAgentTemplate instead.
	VerifierModel *string `json:"verifierModel,omitempty"`
}

type LoopListItem struct {
	ID              string  `json:"id"`
	TemplateID      string  `json:"templateID,omitempty"`
	Name            *string `json:"name"`
	Status          string  `json:"status"`
	Cwd             string  `json:"cwd"`
	Provider        string  `json:"provider"`
	Model           *string `json:"model,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	ActiveIteration *int    `json:"activeIteration"`
}

// LoopTemplateSummary represents a reusable loop configuration.
// Multiple LoopRecords (instances) can share the same TemplateID.
type LoopTemplateSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Cwd           string `json:"cwd"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	InstanceCount int    `json:"instanceCount"`
	LastRunAt     string `json:"lastRunAt,omitempty"`
	LatestStatus  string `json:"latestStatus,omitempty"`
}

// --- Inbound Requests ---

type LoopRunRequest struct {
	Type          string   `json:"type"`
	RequestID     string   `json:"requestId"`
	Prompt        string   `json:"prompt"`
	Cwd           string   `json:"cwd"`
	VerifyPrompt  *string  `json:"verifyPrompt,omitempty"`
	VerifyChecks  []string `json:"verifyChecks,omitempty"`
	Archive       *bool    `json:"archive,omitempty"`
	Name          *string  `json:"name,omitempty"`
	SleepMs       *int     `json:"sleepMs,omitempty"`
	MaxIterations *int     `json:"maxIterations,omitempty"`
	MaxTimeMs     *int     `json:"maxTimeMs,omitempty"`
	TemplateID    string   `json:"templateID,omitempty"`

	// AgentTemplate is the base template for any agent this loop creates.
	AgentTemplate *AgentTemplate `json:"agentTemplate,omitempty"`

	// WorkerAgentTemplate overrides AgentTemplate for the worker agent.
	WorkerAgentTemplate *AgentTemplate `json:"workerAgentTemplate,omitempty"`

	// VerifierAgentTemplate overrides AgentTemplate for the verifier agent.
	VerifierAgentTemplate *AgentTemplate `json:"verifierAgentTemplate,omitempty"`

	// Provider is the legacy default provider for the loop. It is ignored when
	// AgentTemplate or WorkerAgentTemplate is set.
	// Deprecated: use AgentTemplate instead.
	Provider *string `json:"provider,omitempty"`

	// Model is the legacy default model for the loop. It is ignored when
	// AgentTemplate or WorkerAgentTemplate is set.
	// Deprecated: use AgentTemplate instead.
	Model *string `json:"model,omitempty"`

	// WorkerProvider is the legacy worker provider override. It is ignored when
	// WorkerAgentTemplate is set.
	// Deprecated: use WorkerAgentTemplate instead.
	WorkerProvider *string `json:"workerProvider,omitempty"`

	// WorkerModel is the legacy worker model override. It is ignored when
	// WorkerAgentTemplate is set.
	// Deprecated: use WorkerAgentTemplate instead.
	WorkerModel *string `json:"workerModel,omitempty"`

	// VerifierProvider is the legacy verifier provider override. It is ignored
	// when VerifierAgentTemplate is set.
	// Deprecated: use VerifierAgentTemplate instead.
	VerifierProvider *string `json:"verifierProvider,omitempty"`

	// VerifierModel is the legacy verifier model override. It is ignored when
	// VerifierAgentTemplate is set.
	// Deprecated: use VerifierAgentTemplate instead.
	VerifierModel *string `json:"verifierModel,omitempty"`
}

func (m LoopRunRequest) MsgType() string { return "loop/run" }

type LoopListRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m LoopListRequest) MsgType() string { return "loop/list" }

type LoopInspectRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	ID        string `json:"id"`
}

func (m LoopInspectRequest) MsgType() string { return "loop/inspect" }

type LoopLogsRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	ID        string `json:"id"`
	AfterSeq  *int   `json:"afterSeq,omitempty"`
}

func (m LoopLogsRequest) MsgType() string { return "loop/logs" }

type LoopStopRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	ID        string `json:"id"`
}

func (m LoopStopRequest) MsgType() string { return "loop/stop" }

type LoopUpdateRequest struct {
	Type                  string         `json:"type"`
	RequestID             string         `json:"requestId"`
	ID                    string         `json:"id"`
	Name                  *string        `json:"name,omitempty"`
	Archive               *bool          `json:"archive,omitempty"`
	Prompt                *string        `json:"prompt,omitempty"`
	Cwd                   *string        `json:"cwd,omitempty"`
	VerifyChecks          *[]string      `json:"verifyChecks,omitempty"`
	MaxIterations         *int           `json:"maxIterations,omitempty"`
	AgentTemplate         *AgentTemplate `json:"agentTemplate,omitempty"`
	WorkerAgentTemplate   *AgentTemplate `json:"workerAgentTemplate,omitempty"`
	VerifierAgentTemplate *AgentTemplate `json:"verifierAgentTemplate,omitempty"`
}

func (m LoopUpdateRequest) MsgType() string { return "loop/update" }

type LoopDeleteRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	ID        string `json:"id"`
}

func (m LoopDeleteRequest) MsgType() string { return "loop/delete" }

// --- Outbound Responses ---

type LoopRunResponse struct {
	Type    string                 `json:"type"`
	Payload LoopRunResponsePayload `json:"payload"`
}

func (m LoopRunResponse) MsgType() string { return "loop/run/response" }

type LoopRunResponsePayload struct {
	RequestID string      `json:"requestId"`
	Loop      *LoopRecord `json:"loop"`
	Error     *string     `json:"error"`
}

type LoopListResponse struct {
	Type    string                  `json:"type"`
	Payload LoopListResponsePayload `json:"payload"`
}

func (m LoopListResponse) MsgType() string { return "loop/list/response" }

type LoopListResponsePayload struct {
	RequestID string         `json:"requestId"`
	Loops     []LoopListItem `json:"loops"`
	Error     *string        `json:"error"`
}

type LoopInspectResponse struct {
	Type    string                     `json:"type"`
	Payload LoopInspectResponsePayload `json:"payload"`
}

func (m LoopInspectResponse) MsgType() string { return "loop/inspect/response" }

type LoopInspectResponsePayload struct {
	RequestID string      `json:"requestId"`
	Loop      *LoopRecord `json:"loop"`
	Error     *string     `json:"error"`
}

type LoopLogsResponse struct {
	Type    string                  `json:"type"`
	Payload LoopLogsResponsePayload `json:"payload"`
}

func (m LoopLogsResponse) MsgType() string { return "loop/logs/response" }

type LoopLogsResponsePayload struct {
	RequestID  string         `json:"requestId"`
	Loop       *LoopRecord    `json:"loop"`
	Entries    []LoopLogEntry `json:"entries"`
	NextCursor int            `json:"nextCursor"`
	Error      *string        `json:"error"`
}

type LoopStopResponse struct {
	Type    string                  `json:"type"`
	Payload LoopStopResponsePayload `json:"payload"`
}

func (m LoopStopResponse) MsgType() string { return "loop/stop/response" }

type LoopStopResponsePayload struct {
	RequestID string      `json:"requestId"`
	Loop      *LoopRecord `json:"loop"`
	Error     *string     `json:"error"`
}

type LoopUpdateResponse struct {
	Type    string                    `json:"type"`
	Payload LoopUpdateResponsePayload `json:"payload"`
}

func (m LoopUpdateResponse) MsgType() string { return "loop/update/response" }

type LoopUpdateResponsePayload struct {
	RequestID string      `json:"requestId"`
	Loop      *LoopRecord `json:"loop"`
	Error     *string     `json:"error"`
}

type LoopDeleteResponse struct {
	Type    string                    `json:"type"`
	Payload LoopDeleteResponsePayload `json:"payload"`
}

func (m LoopDeleteResponse) MsgType() string { return "loop/delete/response" }

type LoopDeleteResponsePayload struct {
	RequestID string  `json:"requestId"`
	ID        string  `json:"id"`
	Error     *string `json:"error"`
}

// --- Template RPC Types ---

type LoopTemplateListRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m LoopTemplateListRequest) MsgType() string { return "loop/template/list" }

type LoopTemplateListResponse struct {
	Type    string                         `json:"type"`
	Payload LoopTemplateListResponsePayload `json:"payload"`
}

func (m LoopTemplateListResponse) MsgType() string { return "loop/template/list/response" }

type LoopTemplateListResponsePayload struct {
	RequestID string                  `json:"requestId"`
	Templates []LoopTemplateSummary   `json:"templates"`
	Error     *string                 `json:"error"`
}

type LoopTemplateGetRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	TemplateID string `json:"templateID"`
}

func (m LoopTemplateGetRequest) MsgType() string { return "loop/template/get" }

type LoopTemplateGetResponse struct {
	Type    string                       `json:"type"`
	Payload LoopTemplateGetResponsePayload `json:"payload"`
}

func (m LoopTemplateGetResponse) MsgType() string { return "loop/template/get/response" }

type LoopTemplateGetResponsePayload struct {
	RequestID    string                  `json:"requestId"`
	Template     *LoopTemplateSummary    `json:"template"`
	Instances    []LoopListItem          `json:"instances"`
	LatestRecord *LoopRecord             `json:"latestRecord,omitempty"`
	Error        *string                 `json:"error"`
}

type LoopTemplateDeleteRequest struct {
	Type       string `json:"type"`
	RequestID  string `json:"requestId"`
	TemplateID string `json:"templateID"`
}

func (m LoopTemplateDeleteRequest) MsgType() string { return "loop/template/delete" }

type LoopTemplateDeleteResponse struct {
	Type    string                        `json:"type"`
	Payload LoopTemplateDeleteResponsePayload `json:"payload"`
}

func (m LoopTemplateDeleteResponse) MsgType() string { return "loop/template/delete/response" }

type LoopTemplateDeleteResponsePayload struct {
	RequestID  string  `json:"requestId"`
	TemplateID string  `json:"templateID"`
	Error      *string `json:"error"`
}
