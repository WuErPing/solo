package protocol

import "encoding/json"

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

