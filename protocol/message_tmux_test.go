package protocol

import (
	"encoding/json"
	"testing"
)

func TestTmuxListAgentsRequestRoundTrip(t *testing.T) {
	req := TmuxListAgentsRequest{Type: "tmux/list_agents", RequestID: "r1"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxListAgentsRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/list_agents" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/list_agents")
	}
	if decoded.RequestID != "r1" {
		t.Errorf("RequestID: got %q, want %q", decoded.RequestID, "r1")
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
			Error: nil,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxListAgentsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/list_agents/response" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/list_agents/response")
	}
	if len(decoded.Payload.Agents) != 1 {
		t.Fatalf("Agents: got %d, want 1", len(decoded.Payload.Agents))
	}
	a := decoded.Payload.Agents[0]
	if a.AgentName != "claude" {
		t.Errorf("AgentName: got %q, want %q", a.AgentName, "claude")
	}
	if a.SessionName != "dev" {
		t.Errorf("SessionName: got %q, want %q", a.SessionName, "dev")
	}
	if a.PaneID != "%0" {
		t.Errorf("PaneID: got %q, want %q", a.PaneID, "%0")
	}
	if a.PanePID != 1234 {
		t.Errorf("PanePID: got %d, want %d", a.PanePID, 1234)
	}
	if a.WorkingDir != "/home/user/project" {
		t.Errorf("WorkingDir: got %q, want %q", a.WorkingDir, "/home/user/project")
	}
	if decoded.Payload.Error != nil {
		t.Errorf("Error: got %v, want nil", decoded.Payload.Error)
	}
}

func TestTmuxListAgentsResponseWithError(t *testing.T) {
	errMsg := "tmux not found"
	resp := TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: TmuxListAgentsResponsePayload{
			RequestID: "r2",
			Agents:    nil,
			Error:     &errMsg,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxListAgentsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Payload.Error == nil {
		t.Fatal("Error: got nil, want non-nil")
	}
	if *decoded.Payload.Error != errMsg {
		t.Errorf("Error: got %q, want %q", *decoded.Payload.Error, errMsg)
	}
	if decoded.Payload.Agents != nil {
		t.Errorf("Agents: got %v, want nil", decoded.Payload.Agents)
	}
}

func TestTmuxListAgentsResponseMultipleAgents(t *testing.T) {
	resp := TmuxListAgentsResponse{
		Type: "tmux/list_agents/response",
		Payload: TmuxListAgentsResponsePayload{
			RequestID: "r3",
			Agents: []TmuxAgentInfo{
				{SessionName: "s1", WindowName: "w1", PaneID: "%0", AgentName: "claude", CurrentCmd: "claude"},
				{SessionName: "s1", WindowName: "w2", PaneID: "%1", AgentName: "pi", CurrentCmd: "pi"},
				{SessionName: "s2", WindowName: "w1", PaneID: "%0", AgentName: "kimi-cli", CurrentCmd: "kimi-cli"},
			},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxListAgentsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Payload.Agents) != 3 {
		t.Fatalf("Agents: got %d, want 3", len(decoded.Payload.Agents))
	}
	names := make([]string, 3)
	for i, a := range decoded.Payload.Agents {
		names[i] = a.AgentName
	}
	want := []string{"claude", "pi", "kimi-cli"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("Agent[%d].AgentName: got %q, want %q", i, names[i], w)
		}
	}
}

func TestTmuxCapturePaneRequestRoundTrip(t *testing.T) {
	req := TmuxCapturePaneRequest{Type: "tmux/capture_pane", PaneID: "%0", RequestID: "r4"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxCapturePaneRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/capture_pane" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/capture_pane")
	}
	if decoded.PaneID != "%0" {
		t.Errorf("PaneID: got %q, want %q", decoded.PaneID, "%0")
	}
	if decoded.RequestID != "r4" {
		t.Errorf("RequestID: got %q, want %q", decoded.RequestID, "r4")
	}
	if decoded.StartLine != nil {
		t.Errorf("StartLine: got %v, want nil", decoded.StartLine)
	}
}

func TestTmuxCapturePaneRequestWithStartLineRoundTrip(t *testing.T) {
	startLine := -400
	req := TmuxCapturePaneRequest{Type: "tmux/capture_pane", PaneID: "%0", StartLine: &startLine, RequestID: "r4a"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxCapturePaneRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/capture_pane" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/capture_pane")
	}
	if decoded.PaneID != "%0" {
		t.Errorf("PaneID: got %q, want %q", decoded.PaneID, "%0")
	}
	if decoded.StartLine == nil {
		t.Fatal("StartLine: got nil, want non-nil")
	}
	if *decoded.StartLine != -400 {
		t.Errorf("StartLine: got %d, want -400", *decoded.StartLine)
	}
	if decoded.RequestID != "r4a" {
		t.Errorf("RequestID: got %q, want %q", decoded.RequestID, "r4a")
	}
}

func TestTmuxCapturePaneResponseRoundTrip(t *testing.T) {
	resp := TmuxCapturePaneResponse{
		Type: "tmux/capture_pane/response",
		Payload: TmuxCapturePaneResponsePayload{
			RequestID: "r4",
			Content:   "$ ls\nfile1.txt\nfile2.txt\n$ _",
			Error:     nil,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxCapturePaneResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/capture_pane/response" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/capture_pane/response")
	}
	if decoded.Payload.Content != "$ ls\nfile1.txt\nfile2.txt\n$ _" {
		t.Errorf("Content: got %q, want %q", decoded.Payload.Content, "$ ls\nfile1.txt\nfile2.txt\n$ _")
	}
	if decoded.Payload.Error != nil {
		t.Errorf("Error: got %v, want nil", decoded.Payload.Error)
	}
}

func TestTmuxCapturePaneResponseWithError(t *testing.T) {
	errMsg := "no pane %99"
	resp := TmuxCapturePaneResponse{
		Type: "tmux/capture_pane/response",
		Payload: TmuxCapturePaneResponsePayload{
			RequestID: "r5",
			Content:   "",
			Error:     &errMsg,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxCapturePaneResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Payload.Error == nil {
		t.Fatal("Error: got nil, want non-nil")
	}
	if *decoded.Payload.Error != errMsg {
		t.Errorf("Error: got %q, want %q", *decoded.Payload.Error, errMsg)
	}
}

func TestTmuxSendKeysRequestRoundTrip(t *testing.T) {
	req := TmuxSendKeysRequest{Type: "tmux/send_keys", PaneID: "%0", Keys: "ls -la", RequestID: "r6"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxSendKeysRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/send_keys" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/send_keys")
	}
	if decoded.PaneID != "%0" {
		t.Errorf("PaneID: got %q, want %q", decoded.PaneID, "%0")
	}
	if decoded.Keys != "ls -la" {
		t.Errorf("Keys: got %q, want %q", decoded.Keys, "ls -la")
	}
	if decoded.RequestID != "r6" {
		t.Errorf("RequestID: got %q, want %q", decoded.RequestID, "r6")
	}
}

func TestTmuxSendKeysResponseRoundTrip(t *testing.T) {
	resp := TmuxSendKeysResponse{
		Type: "tmux/send_keys/response",
		Payload: TmuxSendKeysResponsePayload{
			RequestID: "r6",
			Error:     nil,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxSendKeysResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MsgType() != "tmux/send_keys/response" {
		t.Errorf("MsgType: got %q, want %q", decoded.MsgType(), "tmux/send_keys/response")
	}
	if decoded.Payload.Error != nil {
		t.Errorf("Error: got %v, want nil", decoded.Payload.Error)
	}
}

func TestTmuxSendKeysResponseWithError(t *testing.T) {
	errMsg := "no pane %99"
	resp := TmuxSendKeysResponse{
		Type: "tmux/send_keys/response",
		Payload: TmuxSendKeysResponsePayload{
			RequestID: "r7",
			Error:     &errMsg,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxSendKeysResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Payload.Error == nil {
		t.Fatal("Error: got nil, want non-nil")
	}
	if *decoded.Payload.Error != errMsg {
		t.Errorf("Error: got %q, want %q", *decoded.Payload.Error, errMsg)
	}
}

func TestTmuxSendKeysRequestSendEnterFalse(t *testing.T) {
	sendEnter := false
	req := TmuxSendKeysRequest{
		Type:      "tmux/send_keys",
		PaneID:    "%0",
		Keys:      "Up",
		SendEnter: &sendEnter,
		RequestID: "r8",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxSendKeysRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SendEnter == nil {
		t.Fatal("SendEnter: got nil, want non-nil")
	}
	if *decoded.SendEnter != false {
		t.Errorf("SendEnter: got %v, want false", *decoded.SendEnter)
	}
	if decoded.Keys != "Up" {
		t.Errorf("Keys: got %q, want %q", decoded.Keys, "Up")
	}
}

func TestTmuxGetThemeRequestRoundTrip(t *testing.T) {
	req := TmuxGetThemeRequest{
		Type:      "tmux/get_theme",
		SessionID: "my-session",
		RequestID: "r9",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxGetThemeRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "tmux/get_theme" {
		t.Errorf("Type: got %q, want %q", decoded.Type, "tmux/get_theme")
	}
	if decoded.SessionID != "my-session" {
		t.Errorf("SessionID: got %q, want %q", decoded.SessionID, "my-session")
	}
	if decoded.RequestID != "r9" {
		t.Errorf("RequestID: got %q, want %q", decoded.RequestID, "r9")
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
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxGetThemeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Payload.Theme.Background != "#181825" {
		t.Errorf("Background: got %q, want %q", decoded.Payload.Theme.Background, "#181825")
	}
	if decoded.Payload.Theme.Foreground != "#cdd6f4" {
		t.Errorf("Foreground: got %q, want %q", decoded.Payload.Theme.Foreground, "#cdd6f4")
	}
	if decoded.Payload.Theme.PaneActiveBorder != "#89b4fa" {
		t.Errorf("PaneActiveBorder: got %q, want %q", decoded.Payload.Theme.PaneActiveBorder, "#89b4fa")
	}
}

func TestTmuxGetThemeResponseWithError(t *testing.T) {
	errMsg := "session not found"
	resp := TmuxGetThemeResponse{
		Type: "tmux/get_theme/response",
		Payload: TmuxGetThemeResponsePayload{
			RequestID: "r11",
			Error:     &errMsg,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TmuxGetThemeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Payload.Error == nil {
		t.Fatal("Error: got nil, want non-nil")
	}
	if *decoded.Payload.Error != errMsg {
		t.Errorf("Error: got %q, want %q", *decoded.Payload.Error, errMsg)
	}
}
