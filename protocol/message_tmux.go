package protocol

// TmuxPaneInfo represents any tmux pane (not necessarily running an agent).
type TmuxPaneInfo struct {
	SessionName             string `json:"sessionName"`
	WindowName              string `json:"windowName"`
	PaneID                  string `json:"paneId"`
	PaneIndex               int    `json:"paneIndex"`
	PanePID                 int    `json:"panePid"`
	CurrentCmd              string `json:"currentCmd"`
	WorkingDir              string `json:"workingDir"`
	Title                   string `json:"title,omitempty"`
	LastContentChange       int64  `json:"lastContentChange"`
	LastContentChangeHHMM   string `json:"lastContentChangeHHMM,omitempty"`
	LastContentChangeAgo    string `json:"lastContentChangeAgo,omitempty"`
}

// TmuxAgentInfo represents a single AI agent detected in a tmux pane.
type TmuxAgentInfo struct {
	SessionName             string `json:"sessionName"`
	WindowName              string `json:"windowName"`
	PaneID                  string `json:"paneId"`
	PaneIndex               int    `json:"paneIndex"`
	PanePID                 int    `json:"panePid"`
	AgentName               string `json:"agentName"`
	CurrentCmd              string `json:"currentCmd"`
	WorkingDir              string `json:"workingDir"`
	Title                   string `json:"title,omitempty"`
	GitCommit               string `json:"gitCommit,omitempty"`
	Status                  string `json:"status,omitempty"`   // "active" (default/omitted) or "exited"
	Activity                string `json:"activity,omitempty"` // "busy", "idle", or "" (unknown)
	LaunchCmd               string `json:"launchCmd,omitempty"`
	LastContentChange       int64  `json:"lastContentChange"`
	LastContentChangeHHMM   string `json:"lastContentChangeHHMM,omitempty"`
	LastContentChangeAgo    string `json:"lastContentChangeAgo,omitempty"`
}

// TmuxListAgentsRequest asks the daemon to scan tmux for AI agent panes.
type TmuxListAgentsRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m TmuxListAgentsRequest) MsgType() string { return "tmux/list_agents" }

// TmuxListAgentsResponse returns the detected agents.
type TmuxListAgentsResponse struct {
	Type    string                        `json:"type"`
	Payload TmuxListAgentsResponsePayload `json:"payload"`
}

func (m TmuxListAgentsResponse) MsgType() string { return "tmux/list_agents/response" }

// AgentCommandEntry represents a deduplicated coding agent launch command.
type AgentCommandEntry struct {
	AgentName string `json:"agentName"`
	LaunchCmd string `json:"launchCmd"`
	LastSeen  string `json:"lastSeen"`
}

// TmuxListAgentsResponsePayload is the payload for TmuxListAgentsResponse.
type TmuxListAgentsResponsePayload struct {
	RequestID      string              `json:"requestId"`
	Agents         []TmuxAgentInfo     `json:"agents"`
	OtherPanes     []TmuxPaneInfo      `json:"otherPanes"`
	CommandHistory []AgentCommandEntry `json:"commandHistory,omitempty"`
	Error          *string             `json:"error"`
}

// TmuxCapturePaneRequest asks the daemon to capture the content of a tmux pane.
// StartLine is the negative offset from the bottom of the pane (e.g. -200 = last 200 lines).
// When nil the daemon defaults to -200.
// LastContentHash, when set, lets the daemon skip returning content if the hash matches.
// Cols, when set, requests the content cropped to the specified width via capture-pane -C.
type TmuxCapturePaneRequest struct {
	Type            string  `json:"type"`
	PaneID          string  `json:"paneId"`
	StartLine       *int    `json:"startLine,omitempty"`
	LastContentHash *string `json:"lastContentHash,omitempty"`
	Cols            *int    `json:"cols,omitempty"`
	RequestID       string  `json:"requestId"`
}

func (m TmuxCapturePaneRequest) MsgType() string { return "tmux/capture_pane" }

// TmuxCapturePaneResponse returns the captured pane content.
type TmuxCapturePaneResponse struct {
	Type    string                         `json:"type"`
	Payload TmuxCapturePaneResponsePayload `json:"payload"`
}

func (m TmuxCapturePaneResponse) MsgType() string { return "tmux/capture_pane/response" }

// TmuxCapturePaneResponsePayload is the payload for TmuxCapturePaneResponse.
// Changed indicates whether content differs from the client's lastContentHash.
// ContentHash is the hash of the current content (always set when no error).
// When Changed is false, Content is empty (client should keep its cached version).
// PaneCols is the original column width of the tmux pane, useful when the
// client wants to display content at the pane's native width.
type TmuxCapturePaneResponsePayload struct {
	RequestID   string  `json:"requestId"`
	Content     string  `json:"content"`
	Changed     *bool   `json:"changed,omitempty"`
	ContentHash *string `json:"contentHash,omitempty"`
	PaneCols    *int    `json:"paneCols,omitempty"`
	Error       *string `json:"error"`
}

// TmuxThemeColors holds the extracted tmux theme colors.
type TmuxThemeColors struct {
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	// Pane colors
	PaneActiveBorder   string `json:"paneActiveBorder,omitempty"`
	PaneInactiveBorder string `json:"paneInactiveBorder,omitempty"`
	// Status bar colors
	StatusBackground string `json:"statusBackground,omitempty"`
	StatusForeground string `json:"statusForeground,omitempty"`
	// Message colors
	MessageBackground string `json:"messageBackground,omitempty"`
	MessageForeground string `json:"messageForeground,omitempty"`
	// Window status colors
	WindowStatusCurrentBg string `json:"windowStatusCurrentBg,omitempty"`
	WindowStatusCurrentFg string `json:"windowStatusCurrentFg,omitempty"`
}

// TmuxGetThemeRequest asks the daemon to extract tmux theme colors for a session.
type TmuxGetThemeRequest struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	RequestID string `json:"requestId"`
}

func (m TmuxGetThemeRequest) MsgType() string { return "tmux/get_theme" }

// TmuxGetThemeResponse returns the extracted tmux theme colors.
type TmuxGetThemeResponse struct {
	Type    string                      `json:"type"`
	Payload TmuxGetThemeResponsePayload `json:"payload"`
}

func (m TmuxGetThemeResponse) MsgType() string { return "tmux/get_theme/response" }

// TmuxGetThemeResponsePayload is the payload for TmuxGetThemeResponse.
type TmuxGetThemeResponsePayload struct {
	RequestID string          `json:"requestId"`
	Theme     TmuxThemeColors `json:"theme"`
	Error     *string         `json:"error"`
}

// TmuxStatusLineRequest asks the daemon to read the tmux status line content.
type TmuxStatusLineRequest struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	RequestID string `json:"requestId"`
}

func (m TmuxStatusLineRequest) MsgType() string { return "tmux/status_line" }

// TmuxStatusLineResponse returns the status line content.
type TmuxStatusLineResponse struct {
	Type    string                        `json:"type"`
	Payload TmuxStatusLineResponsePayload `json:"payload"`
}

func (m TmuxStatusLineResponse) MsgType() string { return "tmux/status_line/response" }

// TmuxStatusLineResponsePayload is the payload for TmuxStatusLineResponse.
type TmuxStatusLineResponsePayload struct {
	RequestID    string  `json:"requestId"`
	StatusLeft   string  `json:"statusLeft"`
	StatusCenter string  `json:"statusCenter"`
	StatusRight  string  `json:"statusRight"`
	Error        *string `json:"error"`
}

// TmuxNewSessionRequest asks the daemon to create a new tmux session.
type TmuxNewSessionRequest struct {
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	WorkingDir *string `json:"workingDir,omitempty"`
	Command    *string `json:"command,omitempty"`
	RequestID  string  `json:"requestId"`
}

func (m TmuxNewSessionRequest) MsgType() string { return "tmux/new_session" }

// TmuxNewSessionResponse confirms the session was created.
type TmuxNewSessionResponse struct {
	Type    string                        `json:"type"`
	Payload TmuxNewSessionResponsePayload `json:"payload"`
}

func (m TmuxNewSessionResponse) MsgType() string { return "tmux/new_session/response" }

// TmuxNewSessionResponsePayload is the payload for TmuxNewSessionResponse.
type TmuxNewSessionResponsePayload struct {
	RequestID   string  `json:"requestId"`
	SessionName string  `json:"sessionName"`
	Error       *string `json:"error"`
}

// TmuxKillSessionRequest asks the daemon to kill a tmux session.
type TmuxKillSessionRequest struct {
	Type        string `json:"type"`
	SessionName string `json:"sessionName"`
	RequestID   string `json:"requestId"`
}

func (m TmuxKillSessionRequest) MsgType() string { return "tmux/kill_session" }

// TmuxKillSessionResponse confirms the session was killed.
type TmuxKillSessionResponse struct {
	Type    string                         `json:"type"`
	Payload TmuxKillSessionResponsePayload `json:"payload"`
}

func (m TmuxKillSessionResponse) MsgType() string { return "tmux/kill_session/response" }

// TmuxKillSessionResponsePayload is the payload for TmuxKillSessionResponse.
type TmuxKillSessionResponsePayload struct {
	RequestID string  `json:"requestId"`
	Error     *string `json:"error"`
}

// TmuxDeleteCommandHistoryRequest asks the daemon to delete a persisted launch command.
type TmuxDeleteCommandHistoryRequest struct {
	Type      string `json:"type"`
	LaunchCmd string `json:"launchCmd"`
	RequestID string `json:"requestId"`
}

func (m TmuxDeleteCommandHistoryRequest) MsgType() string {
	return "tmux/delete_command_history"
}

// TmuxDeleteCommandHistoryResponse confirms the command was deleted.
type TmuxDeleteCommandHistoryResponse struct {
	Type    string                                  `json:"type"`
	Payload TmuxDeleteCommandHistoryResponsePayload `json:"payload"`
}

func (m TmuxDeleteCommandHistoryResponse) MsgType() string {
	return "tmux/delete_command_history/response"
}

// TmuxDeleteCommandHistoryResponsePayload is the payload for TmuxDeleteCommandHistoryResponse.
type TmuxDeleteCommandHistoryResponsePayload struct {
	RequestID string  `json:"requestId"`
	Error     *string `json:"error"`
}

// TmuxSendKeysRequest asks the daemon to send keystrokes to a tmux pane.
type TmuxSendKeysRequest struct {
	Type      string `json:"type"`
	PaneID    string `json:"paneId"`
	Keys      string `json:"keys"`
	SendEnter *bool  `json:"sendEnter,omitempty"`
	RequestID string `json:"requestId"`
}

func (m TmuxSendKeysRequest) MsgType() string { return "tmux/send_keys" }

// TmuxSendKeysResponse confirms the keys were sent.
type TmuxSendKeysResponse struct {
	Type    string                      `json:"type"`
	Payload TmuxSendKeysResponsePayload `json:"payload"`
}

func (m TmuxSendKeysResponse) MsgType() string { return "tmux/send_keys/response" }

// TmuxSendKeysResponsePayload is the payload for TmuxSendKeysResponse.
type TmuxSendKeysResponsePayload struct {
	RequestID string  `json:"requestId"`
	Error     *string `json:"error"`
}
