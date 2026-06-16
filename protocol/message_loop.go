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
	Index             int                      `json:"index"`
	WorkerAgentID     *string                  `json:"workerAgentId"`
	WorkerStartedAt   string                   `json:"workerStartedAt"`
	WorkerCompletedAt *string                  `json:"workerCompletedAt"`
	VerifierAgentID   *string                  `json:"verifierAgentId"`
	Status            string                   `json:"status"`        // "running" | "succeeded" | "failed" | "stopped"
	WorkerOutcome     *string                  `json:"workerOutcome"` // "completed" | "failed" | "canceled"
	FailureReason     *string                  `json:"failureReason"`
	VerifyChecks      []LoopVerifyCheckResult  `json:"verifyChecks"`
	VerifyPrompt      *LoopVerifyPromptResult  `json:"verifyPrompt"`
}

type LoopRecord struct {
	ID                  string                  `json:"id"`
	Name                *string                 `json:"name"`
	Prompt              string                  `json:"prompt"`
	Cwd                 string                  `json:"cwd"`
	Provider            string                  `json:"provider"`
	Model               *string                 `json:"model"`
	WorkerProvider      *string                 `json:"workerProvider"`
	WorkerModel         *string                 `json:"workerModel"`
	VerifierProvider    *string                 `json:"verifierProvider"`
	VerifierModel       *string                 `json:"verifierModel"`
	VerifyPrompt        *string                 `json:"verifyPrompt"`
	VerifyChecks        []string                `json:"verifyChecks"`
	Archive             bool                    `json:"archive"`
	SleepMs             int                     `json:"sleepMs"`
	MaxIterations       *int                    `json:"maxIterations"`
	MaxTimeMs           *int                    `json:"maxTimeMs"`
	Status              string                  `json:"status"` // "running" | "succeeded" | "failed" | "stopped"
	CreatedAt           string                  `json:"createdAt"`
	UpdatedAt           string                  `json:"updatedAt"`
	StartedAt           string                  `json:"startedAt"`
	CompletedAt         *string                 `json:"completedAt"`
	StopRequestedAt     *string                 `json:"stopRequestedAt"`
	Iterations          []LoopIterationRecord   `json:"iterations"`
	Logs                []LoopLogEntry          `json:"logs"`
	NextLogSeq          int                     `json:"nextLogSeq"`
	ActiveIteration     *int                    `json:"activeIteration"`
	ActiveWorkerAgentID *string                 `json:"activeWorkerAgentId"`
	ActiveVerifierAgentID *string               `json:"activeVerifierAgentId"`
}

type LoopListItem struct {
	ID              string  `json:"id"`
	Name            *string `json:"name"`
	Status          string  `json:"status"`
	Cwd             string  `json:"cwd"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	ActiveIteration *int    `json:"activeIteration"`
}

// --- Inbound Requests ---

type LoopRunRequest struct {
	Type             string   `json:"type"`
	RequestID        string   `json:"requestId"`
	Prompt           string   `json:"prompt"`
	Cwd              string   `json:"cwd"`
	Provider         *string  `json:"provider,omitempty"`
	Model            *string  `json:"model,omitempty"`
	WorkerProvider   *string  `json:"workerProvider,omitempty"`
	WorkerModel      *string  `json:"workerModel,omitempty"`
	VerifierProvider *string  `json:"verifierProvider,omitempty"`
	VerifierModel    *string  `json:"verifierModel,omitempty"`
	VerifyPrompt     *string  `json:"verifyPrompt,omitempty"`
	VerifyChecks     []string `json:"verifyChecks,omitempty"`
	Archive          *bool    `json:"archive,omitempty"`
	Name             *string  `json:"name,omitempty"`
	SleepMs          *int     `json:"sleepMs,omitempty"`
	MaxIterations    *int     `json:"maxIterations,omitempty"`
	MaxTimeMs        *int     `json:"maxTimeMs,omitempty"`
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
	Type      string  `json:"type"`
	RequestID string  `json:"requestId"`
	ID        string  `json:"id"`
	Name      *string `json:"name,omitempty"`
	Archive   *bool   `json:"archive,omitempty"`
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
	Type    string                  `json:"type"`
	Payload LoopRunResponsePayload  `json:"payload"`
}

func (m LoopRunResponse) MsgType() string { return "loop/run/response" }

type LoopRunResponsePayload struct {
	RequestID string      `json:"requestId"`
	Loop      *LoopRecord `json:"loop"`
	Error     *string     `json:"error"`
}

type LoopListResponse struct {
	Type    string                   `json:"type"`
	Payload LoopListResponsePayload  `json:"payload"`
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
