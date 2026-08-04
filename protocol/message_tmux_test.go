package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

// roundTripJSON marshals v and unmarshals it back, failing the test on error.
func roundTripJSON[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded T
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return decoded
}

func TestTmuxListAgentsRequestRoundTrip(t *testing.T) {
	req := TmuxListAgentsRequest{Type: "tmux/list_agents", RequestID: "r1"}
	decoded := roundTripJSON(t, req)
	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, req)
	}
}

func TestTmuxListAgentsResponseRoundTrip(t *testing.T) {
	resp := TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: TmuxListAgentsResponsePayload{
			RequestID: "r1",
			Agents: []TmuxAgentInfo{
				{
					SessionName: "dev",
					WindowName:  "main",
					PaneID:      "%0",
					PaneIndex:   0,
					PanePID:     1234,
					AgentName:   "claude",
					CurrentCmd:  "claude",
					WorkingDir:  "/home/user/project",
				},
			},
		},
	}
	decoded := roundTripJSON(t, resp)
	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, resp)
	}
}

func TestTmuxListAgentsResponseWithError(t *testing.T) {
	errMsg := "tmux not found"
	resp := TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: TmuxListAgentsResponsePayload{
			RequestID: "r2",
			Error:     &errMsg,
		},
	}
	decoded := roundTripJSON(t, resp)
	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, resp)
	}
}

func TestTmuxListAgentsResponseWithExitedStatus(t *testing.T) {
	resp := TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: TmuxListAgentsResponsePayload{
			RequestID: "r3a",
			Agents: []TmuxAgentInfo{
				{SessionName: "s1", WindowName: "w1", PaneID: "%0", AgentName: "claude", CurrentCmd: "claude"},
				{SessionName: "s1", WindowName: "w2", PaneID: "%1", AgentName: "pi", CurrentCmd: "bash", Status: "exited"},
			},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// "active" should be omitted (zero value) but "exited" should be present
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	payload := raw["payload"].(map[string]interface{})
	agents := payload["agents"].([]interface{})
	a0 := agents[0].(map[string]interface{})
	a1 := agents[1].(map[string]interface{})
	if _, ok := a0["status"]; ok {
		t.Error("active status should be omitted from JSON")
	}
	if a1["status"] != "exited" {
		t.Errorf("exited status: got %v, want %q", a1["status"], "exited")
	}

	var decoded TmuxListAgentsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Payload.Agents[0].Status != "" {
		t.Errorf("active agent Status: got %q, want empty", decoded.Payload.Agents[0].Status)
	}
	if decoded.Payload.Agents[1].Status != "exited" {
		t.Errorf("exited agent Status: got %q, want %q", decoded.Payload.Agents[1].Status, "exited")
	}
}

func TestTmuxCapturePaneRequestRoundTrip(t *testing.T) {
	req := TmuxCapturePaneRequest{Type: "tmux/capture_pane", PaneID: "%0", RequestID: "r4"}
	decoded := roundTripJSON(t, req)
	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, req)
	}
}

func TestTmuxCapturePaneRequestWithStartLineRoundTrip(t *testing.T) {
	startLine := -400
	req := TmuxCapturePaneRequest{Type: "tmux/capture_pane", PaneID: "%0", StartLine: &startLine, RequestID: "r4a"}
	decoded := roundTripJSON(t, req)
	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, req)
	}
}

func TestTmuxCapturePaneResponseRoundTrip(t *testing.T) {
	resp := TmuxCapturePaneResponse{
		Type: "tmux/capture_pane/response",
		Payload: TmuxCapturePaneResponsePayload{
			RequestID: "r4",
			Content:   "$ ls\nfile1.txt\nfile2.txt\n$ _",
		},
	}
	decoded := roundTripJSON(t, resp)
	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, resp)
	}
}

func TestTmuxSendKeysRequestRoundTrip(t *testing.T) {
	req := TmuxSendKeysRequest{Type: "tmux/send_keys", PaneID: "%0", Keys: "ls -la", RequestID: "r6"}
	decoded := roundTripJSON(t, req)
	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, req)
	}
}

func TestTmuxSendKeysRequestSendEnterFalse(t *testing.T) {
	// An explicit false must survive the round trip (pointer field, not omitempty-collapsed).
	sendEnter := false
	req := TmuxSendKeysRequest{
		Type:      "tmux/send_keys",
		PaneID:    "%0",
		Keys:      "Up",
		SendEnter: &sendEnter,
		RequestID: "r8",
	}
	decoded := roundTripJSON(t, req)
	if decoded.SendEnter == nil {
		t.Fatal("SendEnter: got nil, want non-nil")
	}
	if *decoded.SendEnter != false {
		t.Errorf("SendEnter: got %v, want false", *decoded.SendEnter)
	}
}

func TestTmuxGetThemeRequestRoundTrip(t *testing.T) {
	req := TmuxGetThemeRequest{
		Type:      "tmux/get_theme",
		SessionID: "my-session",
		RequestID: "r9",
	}
	decoded := roundTripJSON(t, req)
	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, req)
	}
}

func TestTmuxGetThemeResponseRoundTrip(t *testing.T) {
	resp := TmuxGetThemeResponse{
		Type: "tmux/get_theme/response",
		Payload: TmuxGetThemeResponsePayload{
			RequestID: "r10",
			Theme: TmuxThemeColors{
				Background:       "#181825",
				Foreground:       "#cdd6f4",
				StatusBackground: "#181825",
				StatusForeground: "#cdd6f4",
				PaneActiveBorder: "#89b4fa",
			},
		},
	}
	decoded := roundTripJSON(t, resp)
	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, resp)
	}
}
