package opencode

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

// --- Tier 1: Leaf functions with zero dependencies ---

func TestReadPositiveFloat(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  *float64
	}{
		{"nil", nil, nil},
		{"positive float64", 3.14, ptrFloat(3.14)},
		{"zero float64", 0.0, nil},
		{"negative float64", -1.0, nil},
		{"json.Number positive", json.Number("2.5"), ptrFloat(2.5)},
		{"json.Number zero", json.Number("0"), nil},
		{"json.Number negative", json.Number("-1"), nil},
		{"json.Number invalid", json.Number("abc"), nil},
		{"string", "3.14", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readPositiveFloat(tt.input)
			if (got == nil) != (tt.want == nil) {
				t.Errorf("readPositiveFloat(%v) = %v, want %v", tt.input, got, tt.want)
			} else if got != nil && *got != *tt.want {
				t.Errorf("readPositiveFloat(%v) = %v, want %v", tt.input, *got, *tt.want)
			}
		})
	}
}

func ptrFloat(f float64) *float64 { return &f }

func TestCapitalizeFirst(t *testing.T) {
	if got := capitalizeFirst("hello"); got != "Hello" {
		t.Errorf("capitalizeFirst(\"hello\") = %q, want %q", got, "Hello")
	}
	if got := capitalizeFirst(""); got != "" {
		t.Errorf("capitalizeFirst(\"\") = %q, want empty", got)
	}
	if got := capitalizeFirst("A"); got != "A" {
		t.Errorf("capitalizeFirst(\"A\") = %q, want %q", got, "A")
	}
}

func TestDerefString(t *testing.T) {
	if got := derefString(nil); got != "" {
		t.Errorf("derefString(nil) = %q, want empty", got)
	}
	s := "hello"
	if got := derefString(&s); got != "hello" {
		t.Errorf("derefString(&\"hello\") = %q, want %q", got, "hello")
	}
}

func TestStringOrNil(t *testing.T) {
	if got := stringOrNil(nil); got != "" {
		t.Errorf("stringOrNil(nil) = %q, want empty", got)
	}
	raw := json.RawMessage(`"hello"`)
	if got := stringOrNil(raw); got != "hello" {
		t.Errorf("stringOrNil quoted = %q, want %q", got, "hello")
	}
	raw2 := json.RawMessage(`42`)
	if got := stringOrNil(raw2); got != "42" {
		t.Errorf("stringOrNil non-string falls back to raw = %q", got)
	}
}

func TestNormalizeError(t *testing.T) {
	if got := normalizeError(nil); got != "unknown error" {
		t.Errorf("nil = %q", got)
	}
	if got := normalizeError("something failed"); got != "something failed" {
		t.Errorf("string = %q", got)
	}
	if got := normalizeError(42); got != "42" {
		t.Errorf("number = %q", got)
	}
	if got := normalizeError(map[string]string{"err": "bad"}); !strings.Contains(got, "bad") {
		t.Errorf("map = %q", got)
	}
}

func TestStringifyStructuredMessage(t *testing.T) {
	if got := stringifyStructuredMessage(nil); got != "" {
		t.Errorf("nil = %q", got)
	}
	if got := stringifyStructuredMessage("hello"); got != "hello" {
		t.Errorf("string = %q", got)
	}
	if got := stringifyStructuredMessage("  "); got != "" {
		t.Errorf("whitespace = %q, want empty", got)
	}
	if got := stringifyStructuredMessage("{}"); got != "{}" {
		t.Errorf("empty object string = %q, want \"{}\"", got)
	}
	if got := stringifyStructuredMessage(map[string]interface{}{"key": "value"}); !strings.Contains(got, "key") {
		t.Errorf("object = %q", got)
	}
}

func TestParseOpenCodeModel(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"openai/gpt-4o", "openai", "gpt-4o"},
		{"anthropic/claude-3-opus", "anthropic", "claude-3-opus"},
		{"gpt-4o", "", "gpt-4o"},
		{"", "", ""},
		{"/", "", "/"},
		{"provider/", "", "provider/"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, m := parseOpenCodeModel(tt.input)
			if p != tt.wantProvider || m != tt.wantModel {
				t.Errorf("parseOpenCodeModel(%q) = (%q, %q), want (%q, %q)", tt.input, p, m, tt.wantProvider, tt.wantModel)
			}
		})
	}
}

func TestParseSlashCommandInput(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantArgs string
	}{
		{"/commit fix bug", "commit", "fix bug"},
		{"/test", "test", ""},
		{"/review  ", "review", ""},
		{"no-slash", "", ""},
		{"/", "", ""},
		{"  /foo bar  ", "foo", "bar"},
		{"", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, args := parseSlashCommandInput(tt.input)
			if name != tt.wantName || args != tt.wantArgs {
				t.Errorf("parseSlashCommandInput(%q) = (%q, %q), want (%q, %q)", tt.input, name, args, tt.wantName, tt.wantArgs)
			}
		})
	}
}

func TestGetAttachmentExtension(t *testing.T) {
	tests := []struct {
		mimeType string
		want     string
	}{
		{"image/png", "png"},
		{"image/jpeg", "jpg"},
		{"image/gif", "gif"},
		{"image/webp", "webp"},
		{"application/pdf", "bin"},
		{"", "bin"},
	}
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			if got := getAttachmentExtension(tt.mimeType); got != tt.want {
				t.Errorf("getAttachmentExtension(%q) = %q, want %q", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestNormalizeToolStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"complete", "completed"},
		{"completed", "completed"},
		{"success", "completed"},
		{"done", "completed"},
		{"error", "failed"},
		{"failed", "failed"},
		{"failure", "failed"},
		{"canceled", "canceled"},
		{"cancelled", "canceled"},
		{"aborted", "canceled"},
		{"running", "running"},
		{"unknown", "running"},
		{"", "running"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeToolStatus(tt.input); got != tt.want {
				t.Errorf("normalizeToolStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsHeadersTimeoutError(t *testing.T) {
	if isHeadersTimeoutError(nil) {
		t.Error("nil should not be timeout")
	}
}

func TestIsFatalRetryMessage(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"insufficient balance", true},
		{"Invalid API Key provided", true},
		{"model not found", true},
		{"temporary network error", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			if got := isFatalRetryMessage(tt.msg); got != tt.want {
				t.Errorf("isFatalRetryMessage(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestIsMcpAlreadyPresentError(t *testing.T) {
	// Function doesn't handle nil (no nil guard), test with real errors
	if isMcpAlreadyPresentError(fmt.Errorf("connection failed")) {
		t.Error("non-matching error should return false")
	}
}

func TestExtractPermissionField(t *testing.T) {
	// nil metadata
	if got := extractPermissionField(nil, []string{"command"}); got != "" {
		t.Errorf("nil metadata = %q", got)
	}

	// Direct field
	meta := json.RawMessage(`{"command": "ls -la", "cwd": "/tmp"}`)
	if got := extractPermissionField(meta, []string{"command"}); got != "ls -la" {
		t.Errorf("direct field = %q, want %q", got, "ls -la")
	}

	// Nested under "input"
	meta2 := json.RawMessage(`{"input": {"command": "echo hi"}}`)
	if got := extractPermissionField(meta2, []string{"command"}); got != "echo hi" {
		t.Errorf("nested input field = %q, want %q", got, "echo hi")
	}

	// Multiple keys (first match wins)
	meta3 := json.RawMessage(`{"cmd": "git status"}`)
	if got := extractPermissionField(meta3, []string{"command", "cmd"}); got != "git status" {
		t.Errorf("multi-key = %q, want %q", got, "git status")
	}
}

func TestExtractQuestionAnswers(t *testing.T) {
	pending := map[string]interface{}{
		"questions": []map[string]interface{}{
			{"header": "Framework"},
		},
	}
	resp := protocol.AgentPermissionResponse{
		UpdatedInput: map[string]interface{}{
			"answers": map[string]interface{}{
				"Framework": "React, Vue",
			},
		},
	}
	answers := extractQuestionAnswers(pending, resp)
	if len(answers) != 1 {
		t.Fatalf("expected 1 answer group, got %d", len(answers))
	}
	if len(answers[0]) != 2 {
		t.Errorf("expected 2 answers, got %d", len(answers[0]))
	}
}

// --- Tier 2: Functions composing Tier 1 ---

func TestBuildToolCallTimelineItem(t *testing.T) {
	input := map[string]interface{}{"command": "echo hello"}
	item := buildToolCallTimelineItem("call-1", "shell", "completed", input, nil, nil)

	if item.Type != "tool_call" {
		t.Errorf("type = %q", item.Type)
	}
	if item.CallID != "call-1" {
		t.Errorf("callID = %q", item.CallID)
	}
	if item.Name != "shell" {
		t.Errorf("name = %q", item.Name)
	}
	if item.Status != "completed" {
		t.Errorf("status = %q", item.Status)
	}

	// Failed status should set error
	failedItem := buildToolCallTimelineItem("call-2", "shell", "failed", input, nil, nil)
	if failedItem.Error == nil {
		t.Error("failed item should have error")
	}
}

// --- Tier 3: Functions with protocol type dependencies ---

func TestOpencodeDefaultModes(t *testing.T) {
	modes := opencodeDefaultModes()
	if len(modes) != 2 {
		t.Fatalf("expected 2 modes, got %d", len(modes))
	}
	if modes[0].ID != "build" || modes[1].ID != "plan" {
		t.Errorf("modes = %v, %v", modes[0].ID, modes[1].ID)
	}
}

func TestSortOpenCodeModes(t *testing.T) {
	modes := []protocol.AgentMode{
		{ID: "custom"},
		{ID: "plan"},
		{ID: "build"},
	}
	sorted := sortOpenCodeModes(modes)
	if sorted[0].ID != "build" || sorted[1].ID != "plan" || sorted[2].ID != "custom" {
		t.Errorf("sorted order = %v, %v, %v", sorted[0].ID, sorted[1].ID, sorted[2].ID)
	}
	// Original slice should not be mutated
	if modes[0].ID != "custom" {
		t.Error("original slice was mutated")
	}
}

func TestBuildToolCallTimelineItemWithError(t *testing.T) {
	item := buildToolCallTimelineItem("c1", "shell", "failed", nil, nil, "custom error")
	if item.Error == nil || item.Error.Message != "custom error" {
		t.Errorf("custom error = %v", item.Error)
	}
}

// --- Tier 4: HTTP transport helpers ---

func TestDecodeOpencodeResponse(t *testing.T) {
	// Wrapped format: {data: ...}
	body := strings.NewReader(`{"data": {"status": "ok"}}`)
	var result map[string]interface{}
	if err := decodeOpencodeResponse(body, &result); err != nil {
		t.Fatalf("decodeOpencodeResponse wrapped: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("wrapped result = %v", result)
	}

	// Direct JSON format
	body2 := strings.NewReader(`{"name": "test"}`)
	var result2 map[string]interface{}
	if err := decodeOpencodeResponse(body2, &result2); err != nil {
		t.Fatalf("decodeOpencodeResponse direct: %v", err)
	}
	if result2["name"] != "test" {
		t.Errorf("direct result = %v", result2)
	}

	// Error response
	body3 := strings.NewReader(`{"error": "not found"}`)
	var result3 map[string]interface{}
	if err := decodeOpencodeResponse(body3, &result3); err != nil {
		t.Fatalf("decodeOpencodeResponse error: %v", err)
	}
	if result3["error"] != "not found" {
		t.Errorf("error result = %v", result3)
	}
}

func TestFindAvailablePort(t *testing.T) {
	port, err := findAvailablePort()
	if err != nil {
		t.Fatalf("findAvailablePort: %v", err)
	}
	if port <= 0 {
		t.Errorf("port = %d, want > 0", port)
	}
}
