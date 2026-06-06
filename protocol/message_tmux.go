package protocol

// TmuxAgentInfo represents a single AI agent detected in a tmux pane.
type TmuxAgentInfo struct {
	SessionName string `json:"sessionName"`
	WindowName  string `json:"windowName"`
	PaneID      string `json:"paneId"`
	PaneIndex   int    `json:"paneIndex"`
	PanePID     int    `json:"panePid"`
	AgentName   string `json:"agentName"`
	CurrentCmd  string `json:"currentCmd"`
	WorkingDir  string `json:"workingDir"`
}

// TmuxListAgentsRequest asks the daemon to scan tmux for AI agent panes.
type TmuxListAgentsRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m TmuxListAgentsRequest) MsgType() string { return "tmux/list_agents" }

// TmuxListAgentsResponse returns the detected agents.
type TmuxListAgentsResponse struct {
	Type    string                         `json:"type"`
	Payload TmuxListAgentsResponsePayload  `json:"payload"`
}

func (m TmuxListAgentsResponse) MsgType() string { return "tmux/list_agents/response" }

// TmuxListAgentsResponsePayload is the payload for TmuxListAgentsResponse.
type TmuxListAgentsResponsePayload struct {
	RequestID string          `json:"requestId"`
	Agents    []TmuxAgentInfo `json:"agents"`
	Error     *string         `json:"error"`
}

// TmuxCapturePaneRequest asks the daemon to capture the content of a tmux pane.
// StartLine is the negative offset from the bottom of the pane (e.g. -200 = last 200 lines).
// When nil the daemon defaults to -200.
type TmuxCapturePaneRequest struct {
	Type      string `json:"type"`
	PaneID    string `json:"paneId"`
	StartLine *int   `json:"startLine,omitempty"`
	RequestID string `json:"requestId"`
}

func (m TmuxCapturePaneRequest) MsgType() string { return "tmux/capture_pane" }

// TmuxCapturePaneResponse returns the captured pane content.
type TmuxCapturePaneResponse struct {
	Type    string                          `json:"type"`
	Payload TmuxCapturePaneResponsePayload  `json:"payload"`
}

func (m TmuxCapturePaneResponse) MsgType() string { return "tmux/capture_pane/response" }

// TmuxCapturePaneResponsePayload is the payload for TmuxCapturePaneResponse.
type TmuxCapturePaneResponsePayload struct {
	RequestID string  `json:"requestId"`
	Content   string  `json:"content"`
	Error     *string `json:"error"`
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
	RequestID string           `json:"requestId"`
	Theme     TmuxThemeColors  `json:"theme"`
	Error     *string          `json:"error"`
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
	Type    string                       `json:"type"`
	Payload TmuxSendKeysResponsePayload  `json:"payload"`
}

func (m TmuxSendKeysResponse) MsgType() string { return "tmux/send_keys/response" }

// TmuxSendKeysResponsePayload is the payload for TmuxSendKeysResponse.
type TmuxSendKeysResponsePayload struct {
	RequestID string  `json:"requestId"`
	Error     *string `json:"error"`
}
