package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

type fakePlantUMLRenderer struct {
	svg    string
	err    error
	source string
	dark   bool
}

func (f *fakePlantUMLRenderer) Render(_ context.Context, source string, dark bool) (string, error) {
	f.source = source
	f.dark = dark
	return f.svg, f.err
}

func sendPlantUMLRequest(t *testing.T, wsURL, requestID, source string, dark bool) protocol.PlantUMLRenderResponsePayload {
	t.Helper()
	conn := dialAndHello(t, wsURL, "test-plantuml-"+requestID)
	defer conn.Close()
	readInitialMessages(t, conn)

	req := protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "plantuml_render_request",
			"requestId": requestID,
			"source":    source,
			"dark":      dark,
		}),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write plantuml_render_request: %v", err)
	}

	resp := readUntilType(t, conn, "plantuml_render_response")
	return decodeSessionPayload[protocol.PlantUMLRenderResponsePayload](t, resp)
}

func TestPlantUMLRenderSuccess(t *testing.T) {
	ws, ts := newTestWSServer(t)
	fake := &fakePlantUMLRenderer{svg: `<svg xmlns="http://www.w3.org/2000/svg"></svg>`}
	ws.SetPlantUMLRenderer(fake)

	source := "@startuml\nA -> B\n@enduml"
	payload := sendPlantUMLRequest(t, ts.URL, "req-puml-ok", source, true)

	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if payload.SVG != fake.svg {
		t.Errorf("svg: got %q, want %q", payload.SVG, fake.svg)
	}
	if payload.RequestID != "req-puml-ok" {
		t.Errorf("requestId: got %q, want req-puml-ok", payload.RequestID)
	}
	if fake.source != source {
		t.Errorf("source passed to renderer: got %q, want %q", fake.source, source)
	}
	if !fake.dark {
		t.Error("expected dark flag to be forwarded to the renderer")
	}
}

func TestPlantUMLRenderRendererError(t *testing.T) {
	ws, ts := newTestWSServer(t)
	ws.SetPlantUMLRenderer(&fakePlantUMLRenderer{err: errors.New("PlantUML renderer unavailable: install PlantUML")})

	payload := sendPlantUMLRequest(t, ts.URL, "req-puml-err", "@startuml\nbogus ->\n@enduml", false)

	if payload.Error == nil {
		t.Fatal("expected error payload")
	}
	if !strings.Contains(*payload.Error, "install PlantUML") {
		t.Errorf("error should surface renderer message, got %q", *payload.Error)
	}
	if payload.SVG != "" {
		t.Errorf("expected empty svg on error, got %q", payload.SVG)
	}
	if payload.RequestID != "req-puml-err" {
		t.Errorf("requestId: got %q, want req-puml-err", payload.RequestID)
	}
}

func TestPlantUMLRenderWithoutRenderer(t *testing.T) {
	_, ts := newTestWSServer(t)

	payload := sendPlantUMLRequest(t, ts.URL, "req-puml-nil", "@startuml\nA -> B\n@enduml", false)

	if payload.Error == nil {
		t.Fatal("expected error payload when no renderer is configured")
	}
	if !strings.Contains(*payload.Error, "unavailable") {
		t.Errorf("error should explain the renderer is unavailable, got %q", *payload.Error)
	}
}
