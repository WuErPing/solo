package protocol

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
	Type    string                            `json:"type"`
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
