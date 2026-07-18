package protocol

// --- Schedule Assistant (natural-language -> proposal RPC) ---

type ScheduleAssistRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	Message   string `json:"message"`   // user natural-language input, ≤ 2000 chars
	Timezone  string `json:"timezone"`  // IANA, required — e.g. "Asia/Shanghai"
	ClientNow string `json:"clientNow"` // RFC3339, client wall clock (relative times: "tomorrow")

	ContextScheduleID string               `json:"contextScheduleId,omitempty"` // opened from a detail screen
	Transcript        []ScheduleAssistTurn `json:"transcript,omitempty"`        // ≤ 10 turns, oldest first
}

type ScheduleAssistTurn struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"` // plain-text rendering of the turn (proposals summarized)
}

func (m ScheduleAssistRequest) MsgType() string { return "schedule/assist" }

type ScheduleAssistResponse struct {
	Type    string                        `json:"type"`
	Payload ScheduleAssistResponsePayload `json:"payload"`
}

func (m ScheduleAssistResponse) MsgType() string { return "schedule/assist/response" }

type ScheduleAssistResponsePayload struct {
	RequestID string                  `json:"requestId"`
	Kind      string                  `json:"kind"` // "proposal" | "clarify" | "answer" | "error"
	Message   string                  `json:"message,omitempty"`
	Proposal  *ScheduleAssistProposal `json:"proposal,omitempty"`
	Error     *string                 `json:"error"` // code e.g. "no_llm_provider"; nil = success

	LLMProvider string `json:"llmProvider,omitempty"` // resolved provider config id — for the panel indicator
	Model       string `json:"model,omitempty"`       // resolved model id — for the panel indicator
}

type ScheduleAssistProposal struct {
	Op         string           `json:"op"` // "create" | "update" | "pause" | "resume" | "delete"
	ScheduleID string           `json:"scheduleId,omitempty"`
	Name       string           `json:"name,omitempty"`
	Prompt     string           `json:"prompt,omitempty"`
	Cadence    *ScheduleCadence `json:"cadence,omitempty"` // LOCAL cron/interval in request timezone
	Target     *ScheduleTarget  `json:"target,omitempty"`
	Cwd        string           `json:"cwd,omitempty"`
	MaxRuns    *int             `json:"maxRuns,omitempty"`
	ExpiresAt  string           `json:"expiresAt,omitempty"`

	Summary   string   `json:"summary"`            // one-line human description from the LLM
	Warnings  []string `json:"warnings,omitempty"` // e.g. "interpreted 'morning' as 09:00"
	NextRunAt *string  `json:"nextRunAt,omitempty"`
}
