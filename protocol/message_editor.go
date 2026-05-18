package protocol

// --- Editor Message Types ---

type ListAvailableEditorsRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *ListAvailableEditorsRequest) MsgType() string { return "list_available_editors_request" }

type ListAvailableEditorsResponse struct {
	Type    string                      `json:"type"`
	Payload ListAvailableEditorsPayload `json:"payload"`
}

type ListAvailableEditorsPayload struct {
	RequestID string         `json:"requestId"`
	Editors   []EditorTarget `json:"editors"`
	Error     *string        `json:"error"`
}

type EditorTarget struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (m *ListAvailableEditorsResponse) MsgType() string { return "list_available_editors_response" }

