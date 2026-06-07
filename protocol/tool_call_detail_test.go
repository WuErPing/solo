package protocol

import (
	"encoding/json"
	"testing"
)

func TestToolCallDetailMarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		detail   ToolCallDetail
		wantJSON string
	}{
		{
			name:     "shell",
			detail:   ShellDetail{Type: "shell", Command: "ls", Cwd: "/tmp"},
			wantJSON: `{"type":"shell","command":"ls","cwd":"/tmp"}`,
		},
		{
			name:     "read",
			detail:   ReadDetail{Type: "read", FilePath: "/tmp/a.txt", Content: "hello"},
			wantJSON: `{"type":"read","filePath":"/tmp/a.txt","content":"hello"}`,
		},
		{
			name:     "write",
			detail:   WriteDetail{Type: "write", FilePath: "/tmp/b.txt", Content: "world"},
			wantJSON: `{"type":"write","filePath":"/tmp/b.txt","content":"world"}`,
		},
		{
			name:     "edit",
			detail:   EditDetail{Type: "edit", FilePath: "/tmp/c.go", OldString: "old", NewString: "new"},
			wantJSON: `{"type":"edit","filePath":"/tmp/c.go","oldString":"old","newString":"new"}`,
		},
		{
			name:     "search",
			detail:   SearchDetail{Type: "search", Query: "TODO"},
			wantJSON: `{"type":"search","query":"TODO"}`,
		},
		{
			name:     "fetch",
			detail:   FetchDetail{Type: "fetch", URL: "https://example.com"},
			wantJSON: `{"type":"fetch","url":"https://example.com"}`,
		},
		{
			name:     "unknown",
			detail:   UnknownDetail{Type: "unknown", Input: map[string]interface{}{"foo": "bar"}},
			wantJSON: `{"type":"unknown","input":{"foo":"bar"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.detail)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("marshal\ngot  %s\nwant %s", got, tt.wantJSON)
			}

			// Unmarshal via the wrapper to exercise dispatch
			var parsed ToolCallDetailWrapper
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if parsed.Detail == nil {
				t.Fatal("unmarshaled detail is nil")
			}
			if parsed.Detail.GetType() != tt.name {
				t.Errorf("type: got %q, want %q", parsed.Detail.GetType(), tt.name)
			}
		})
	}
}

func TestToolErrorMarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		err      *ToolError
		wantJSON string
	}{
		{
			name:     "string",
			err:      &ToolError{Message: "oops"},
			wantJSON: `"oops"`,
		},
		{
			name:     "object",
			err:      &ToolError{Message: "failed"},
			wantJSON: `"failed"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("marshal\ngot  %s\nwant %s", got, tt.wantJSON)
			}

			var parsed ToolError
			if err := json.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if parsed.Message != tt.err.Message {
				t.Errorf("message: got %q, want %q", parsed.Message, tt.err.Message)
			}
		})
	}
}

func TestToolErrorUnmarshalFromObject(t *testing.T) {
	jsonData := `{"message":"something went wrong"}`
	var parsed ToolError
	if err := json.Unmarshal([]byte(jsonData), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Message != "something went wrong" {
		t.Errorf("message: got %q, want %q", parsed.Message, "something went wrong")
	}
}

func TestTimelineItemWithTypedDetailError(t *testing.T) {
	item := TimelineItem{
		Type:   "tool_call",
		CallID: "call-1",
		Name:   "shell",
		Status: "failed",
		Detail: ShellDetail{Type: "shell", Command: "ls"},
		Error:  &ToolError{Message: "not found"},
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed TimelineItem
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.Type != "tool_call" {
		t.Errorf("type: got %q", parsed.Type)
	}
	if parsed.Detail == nil {
		t.Fatal("detail is nil")
	}
	shell, ok := parsed.Detail.(ShellDetail)
	if !ok {
		t.Fatalf("detail type: got %T, want ShellDetail", parsed.Detail)
	}
	if shell.Command != "ls" {
		t.Errorf("command: got %q, want ls", shell.Command)
	}
	if parsed.Error == nil {
		t.Fatal("error is nil")
	}
	if parsed.Error.Message != "not found" {
		t.Errorf("error message: got %q, want not found", parsed.Error.Message)
	}
}
