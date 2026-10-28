package protocol

// PlantUMLRenderRequest asks the daemon to render one PlantUML diagram to SVG.
// Rendering happens on the daemon host (local plantuml CLI or a configured
// PlantUML-compatible server), so diagram source never leaves the machine.
type PlantUMLRenderRequest struct {
	Type      string `json:"type"`
	Source    string `json:"source"`
	Dark      bool   `json:"dark"`
	RequestID string `json:"requestId"`
}

func (m *PlantUMLRenderRequest) MsgType() string { return "plantuml_render_request" }

type PlantUMLRenderResponse struct {
	Type    string                        `json:"type"`
	Payload PlantUMLRenderResponsePayload `json:"payload"`
}

func (m *PlantUMLRenderResponse) MsgType() string { return "plantuml_render_response" }

type PlantUMLRenderResponsePayload struct {
	// SVG is the rendered diagram markup; empty when Error is set.
	SVG string `json:"svg"`
	// Error is nil on success and carries an actionable message otherwise.
	Error     *string `json:"error"`
	RequestID string  `json:"requestId"`
}
