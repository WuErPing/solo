package protocol

// TmuxPaneChangedNotification is a server-push message (no requestId) that
// tells clients one or more tmux panes have new content. Clients should
// refetch the affected panes immediately.
type TmuxPaneChangedNotification struct {
	Type    string                 `json:"type"`
	Payload TmuxPaneChangedPayload `json:"payload"`
}

func (m *TmuxPaneChangedNotification) MsgType() string { return "tmux/pane_changed" }

type TmuxPaneChangedPayload struct {
	PaneIDs []string `json:"paneIds"`
}
