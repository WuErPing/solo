package server

import (
	"context"
	"strings"

	"github.com/WuErPing/solo/protocol"
)

// maxPlantUMLErrorMessage bounds the error text echoed to clients; renderer
// errors are already user-facing (install hints, server status, stderr tails).
const maxPlantUMLErrorMessage = 2000

// handlePlantUMLRender renders one diagram for a markdown preview. The work
// runs in a goroutine: a cold JVM start takes hundreds of milliseconds to
// seconds and must not block the session read loop.
func (s *Session) handlePlantUMLRender(m *protocol.PlantUMLRenderRequest) {
	go func() {
		payload := protocol.PlantUMLRenderResponsePayload{RequestID: m.RequestID}
		if s.plantUML == nil {
			message := "PlantUML renderer unavailable: the daemon was started without one"
			payload.Error = &message
		} else if svg, err := s.plantUML.Render(context.Background(), m.Source, m.Dark); err != nil {
			message := strings.TrimSpace(err.Error())
			if len(message) > maxPlantUMLErrorMessage {
				message = message[:maxPlantUMLErrorMessage]
			}
			payload.Error = &message
		} else {
			payload.SVG = svg
		}

		s.sendMessage(protocol.NewSessionMessage(&protocol.PlantUMLRenderResponse{
			Type:    "plantuml_render_response",
			Payload: payload,
		}))
	}()
}
