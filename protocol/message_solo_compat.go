package protocol

import "encoding/json"

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

type ArchiveWorkspaceResponse struct {
	Type    string                          `json:"type"`
	Payload ArchiveWorkspaceResponsePayload `json:"payload"`
}

type ArchiveWorkspaceResponsePayload struct {
	RequestID   string  `json:"requestId"`
	WorkspaceID string  `json:"workspaceId"`
	ArchivedAt  *string `json:"archivedAt,omitempty"`
	Error       *string `json:"error,omitempty"`
}

func (m *ArchiveWorkspaceResponse) MsgType() string { return "archive_workspace_response" }

// --- Solo worktree archive ---

type SoloWorktreeArchiveRequest struct {
	Type         string `json:"type"`
	WorktreePath string `json:"worktreePath,omitempty"`
	RepoRoot     string `json:"repoRoot,omitempty"`
	BranchName   string `json:"branchName,omitempty"`
	RequestID    string `json:"requestId"`
}

func (m *SoloWorktreeArchiveRequest) MsgType() string { return "solo_worktree_archive_request" }

type SoloWorktreeArchiveResponse struct {
	Type    string                             `json:"type"`
	Payload SoloWorktreeArchiveResponsePayload `json:"payload"`
}

type SoloWorktreeArchiveResponsePayload struct {
	RequestID     string         `json:"requestId"`
	Success       bool           `json:"success"`
	RemovedAgents []string       `json:"removedAgents,omitempty"`
	Error         *CheckoutError `json:"error,omitempty"`
}

// CheckoutError mirrors the app-bridge checkout error schema.
type CheckoutError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (m *SoloWorktreeArchiveResponse) MsgType() string { return "solo_worktree_archive_response" }

// --- Remove project ---

type RemoveProjectRequest struct {
	Type         string   `json:"type"`
	WorkspaceIDs []string `json:"workspaceIds"`
	RequestID    string   `json:"requestId"`
}

func (m *RemoveProjectRequest) MsgType() string { return "remove_project_request" }

type RemoveProjectResponse struct {
	Type    string                       `json:"type"`
	Payload RemoveProjectResponsePayload `json:"payload"`
}

type RemoveProjectResponsePayload struct {
	RequestID    string   `json:"requestId"`
	WorkspaceIDs []string `json:"workspaceIds"`
	RemovedCount int      `json:"removedCount"`
	Error        *string  `json:"error,omitempty"`
}

func (m *RemoveProjectResponse) MsgType() string { return "remove_project_response" }
