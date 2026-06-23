package base

import (
	"strings"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestExtractString(t *testing.T) {
	m := map[string]interface{}{"name": "value", "num": 42, "empty": ""}

	if got := ExtractString(m, "name"); got != "value" {
		t.Errorf("ExtractString(m, \"name\") = %q, want %q", got, "value")
	}
	if got := ExtractString(m, "missing"); got != "" {
		t.Errorf("ExtractString(m, \"missing\") = %q, want empty", got)
	}
	if got := ExtractString(m, "num"); got != "" {
		t.Errorf("ExtractString(m, \"num\") = %q, want empty (non-string)", got)
	}
	if got := ExtractString(m, "empty"); got != "" {
		t.Errorf("ExtractString(m, \"empty\") = %q, want empty", got)
	}
	if got := ExtractString(m, "missing", "name"); got != "value" {
		t.Errorf("ExtractString multi-key = %q, want %q", got, "value")
	}
}

func TestExtractNumber(t *testing.T) {
	m := map[string]interface{}{"count": 42.0, "name": "str", "nilval": nil}

	if got := ExtractNumber(m, "count"); got != 42.0 {
		t.Errorf("ExtractNumber(m, \"count\") = %v, want 42.0", got)
	}
	if got := ExtractNumber(m, "missing"); got != nil {
		t.Errorf("ExtractNumber(m, \"missing\") = %v, want nil", got)
	}
	if got := ExtractNumber(m, "nilval"); got != nil {
		t.Errorf("ExtractNumber(m, \"nilval\") = %v, want nil", got)
	}
	if got := ExtractNumber(m, "name"); got != "str" {
		t.Errorf("ExtractNumber(m, \"name\") = %v, want \"str\" (non-nil value)", got)
	}
}

func TestExtractStringOrJoinArray(t *testing.T) {
	m := map[string]interface{}{
		"cmd":   "ls -la",
		"parts": []interface{}{"echo", "hello", "world"},
		"empty": []interface{}{},
	}

	if got := ExtractStringOrJoinArray(m, "cmd"); got != "ls -la" {
		t.Errorf("string value = %q, want %q", got, "ls -la")
	}
	if got := ExtractStringOrJoinArray(m, "parts"); got != "echo hello world" {
		t.Errorf("array join = %q, want %q", got, "echo hello world")
	}
	if got := ExtractStringOrJoinArray(m, "empty"); got != "" {
		t.Errorf("empty array = %q, want empty", got)
	}
	if got := ExtractStringOrJoinArray(m, "missing"); got != "" {
		t.Errorf("missing key = %q, want empty", got)
	}
}

func TestExtractStringOrJoinArray_MixedTypes(t *testing.T) {
	m := map[string]interface{}{
		"mixed": []interface{}{"hello", 42, "world"},
	}
	got := ExtractStringOrJoinArray(m, "mixed")
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Errorf("mixed array = %q", got)
	}
}

func TestExtractNestedString(t *testing.T) {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"path": "/tmp/test",
			"num":  42,
		},
	}

	if got := ExtractNestedString(m, "metadata", "path"); got != "/tmp/test" {
		t.Errorf("nested string = %q, want %q", got, "/tmp/test")
	}
	if got := ExtractNestedString(m, "metadata", "num"); got != "" {
		t.Errorf("nested non-string = %q, want empty", got)
	}
	if got := ExtractNestedString(m, "missing", "path"); got != "" {
		t.Errorf("missing nested key = %q, want empty", got)
	}
}

func TestExtractNestedNumber(t *testing.T) {
	m := map[string]interface{}{
		"stats": map[string]interface{}{
			"exitCode": 1.0,
		},
	}

	if got := ExtractNestedNumber(m, "stats", "exitCode"); got != 1.0 {
		t.Errorf("nested number = %v, want 1.0", got)
	}
	if got := ExtractNestedNumber(m, "stats", "missing"); got != nil {
		t.Errorf("missing nested number = %v, want nil", got)
	}
}

func TestTruncateText(t *testing.T) {
	if got := TruncateText("hello", 10); got != "hello" {
		t.Errorf("short string = %q", got)
	}
	if got := TruncateText("hello world", 5); got != "hello..." {
		t.Errorf("truncated = %q, want %q", got, "hello...")
	}
	if got := TruncateText("", 5); got != "" {
		t.Errorf("empty string = %q", got)
	}
	if got := TruncateText("exact", 5); got != "exact" {
		t.Errorf("exact length = %q", got)
	}
}

func TestHumanReadablePermission(t *testing.T) {
	tests := []struct {
		perm string
		want string
	}{
		{"bash", "Run shell command"},
		{"read_file", "Read files"},
		{"write_file", "Write files"},
		{"edit", "Edit files"},
		{"custom_permission", "Custom Permission"},
		{"unknownThing", "UnknownThing"},
	}
	for _, tt := range tests {
		t.Run(tt.perm, func(t *testing.T) {
			if got := HumanReadablePermission(tt.perm); got != tt.want {
				t.Errorf("HumanReadablePermission(%q) = %q, want %q", tt.perm, got, tt.want)
			}
		})
	}
}

func TestDeriveToolCallDetail(t *testing.T) {
	if got := DeriveToolCallDetail("unknown", nil, nil); got != nil {
		t.Errorf("nil input/output should return nil, got %v", got)
	}

	input := map[string]interface{}{"command": "ls -la"}
	output := map[string]interface{}{"exit_code": 0.0}
	detail := DeriveToolCallDetail("shell", input, output)
	shell, ok := detail.(protocol.ShellDetail)
	if !ok {
		t.Fatalf("expected ShellDetail, got %T", detail)
	}
	if shell.Type != "shell" {
		t.Errorf("shell type = %v", shell.Type)
	}
	if shell.Command != "ls -la" {
		t.Errorf("shell command = %v", shell.Command)
	}

	readInput := map[string]interface{}{"file_path": "/tmp/test.txt"}
	readDetail := DeriveToolCallDetail("read", readInput, nil)
	read, ok := readDetail.(protocol.ReadDetail)
	if !ok {
		t.Fatalf("expected ReadDetail, got %T", readDetail)
	}
	if read.Type != "read" {
		t.Errorf("read type = %v", read.Type)
	}
	if read.FilePath != "/tmp/test.txt" {
		t.Errorf("read filePath = %v", read.FilePath)
	}

	unknownInput := map[string]interface{}{"foo": "bar"}
	unknownDetail := DeriveToolCallDetail("custom_tool", unknownInput, nil)
	unk, ok := unknownDetail.(protocol.UnknownDetail)
	if !ok {
		t.Fatalf("expected UnknownDetail, got %T", unknownDetail)
	}
	if unk.Type != "unknown" {
		t.Errorf("unknown tool type = %v", unk.Type)
	}
}

func TestDeriveToolCallDetailEdit(t *testing.T) {
	input := map[string]interface{}{
		"file_path": "/tmp/test.go",
		"old_str":   "func old()",
		"new_str":   "func new()",
	}
	detail := DeriveToolCallDetail("edit", input, nil).(protocol.EditDetail)
	if detail.Type != "edit" {
		t.Errorf("type = %v", detail.Type)
	}
	if detail.FilePath != "/tmp/test.go" {
		t.Errorf("filePath = %v", detail.FilePath)
	}
	if detail.OldString != "func old()" {
		t.Errorf("oldString = %v", detail.OldString)
	}
	if detail.NewString != "func new()" {
		t.Errorf("newString = %v", detail.NewString)
	}
}

func TestDeriveToolCallDetailSearch(t *testing.T) {
	input := map[string]interface{}{
		"query":     "func main",
		"tool_name": "grep",
	}
	detail := DeriveToolCallDetail("search", input, nil).(protocol.SearchDetail)
	if detail.Query != "func main" {
		t.Errorf("query = %v", detail.Query)
	}
}

func TestDeriveToolCallDetailFetch(t *testing.T) {
	input := map[string]interface{}{
		"url": "https://example.com",
	}
	output := map[string]interface{}{
		"statusCode": 200.0,
	}
	detail := DeriveToolCallDetail("fetch", input, output).(protocol.FetchDetail)
	if detail.URL != "https://example.com" {
		t.Errorf("url = %v", detail.URL)
	}
	if detail.Code == nil || *detail.Code != 200 {
		t.Errorf("code = %v", detail.Code)
	}
}

func TestDeriveShellDetail(t *testing.T) {
	input := map[string]interface{}{
		"command": "ls -la",
		"cwd":     "/tmp",
	}
	output := map[string]interface{}{
		"output":    "file1\nfile2",
		"exit_code": 0.0,
	}
	detail := DeriveToolCallDetail("shell", input, output).(protocol.ShellDetail)
	if detail.Command != "ls -la" {
		t.Errorf("command = %v", detail.Command)
	}
	if detail.Cwd != "/tmp" {
		t.Errorf("cwd = %v", detail.Cwd)
	}
	if detail.ExitCode == nil || *detail.ExitCode != 0 {
		t.Errorf("exitCode = %v", detail.ExitCode)
	}
}

func TestDeriveReadDetail(t *testing.T) {
	input := map[string]interface{}{
		"file_path": "/tmp/test.go",
		"offset":    10.0,
		"limit":     50.0,
	}
	detail := DeriveToolCallDetail("read", input, nil).(protocol.ReadDetail)
	if detail.FilePath != "/tmp/test.go" {
		t.Errorf("filePath = %v", detail.FilePath)
	}
	if detail.Offset == nil || *detail.Offset != 10 {
		t.Errorf("offset = %v", detail.Offset)
	}
}

func TestDeriveWriteDetail(t *testing.T) {
	input := map[string]interface{}{
		"file_path": "/tmp/out.txt",
		"content":   "hello world",
	}
	detail := DeriveToolCallDetail("write", input, nil).(protocol.WriteDetail)
	if detail.FilePath != "/tmp/out.txt" {
		t.Errorf("filePath = %v", detail.FilePath)
	}
	if detail.Content != "hello world" {
		t.Errorf("content = %v", detail.Content)
	}
}

func TestDeriveEditDetail(t *testing.T) {
	input := map[string]interface{}{
		"file_path": "/tmp/test.go",
		"old_str":   "foo",
		"new_str":   "bar",
	}
	detail := DeriveToolCallDetail("edit", input, nil).(protocol.EditDetail)
	if detail.FilePath != "/tmp/test.go" {
		t.Errorf("filePath = %v", detail.FilePath)
	}
	if detail.OldString != "foo" {
		t.Errorf("oldString = %v", detail.OldString)
	}
	if detail.NewString != "bar" {
		t.Errorf("newString = %v", detail.NewString)
	}
}

func TestDeriveSearchDetail(t *testing.T) {
	input := map[string]interface{}{
		"query": "TODO",
	}
	detail := DeriveToolCallDetail("search", input, nil).(protocol.SearchDetail)
	if detail.Query != "TODO" {
		t.Errorf("query = %v", detail.Query)
	}
}

func TestDeriveFetchDetail(t *testing.T) {
	input := map[string]interface{}{
		"url": "https://example.com",
	}
	output := map[string]interface{}{
		"statusCode": 200.0,
		"content":    "<html>hello</html>",
	}
	detail := DeriveToolCallDetail("fetch", input, output).(protocol.FetchDetail)
	if detail.URL != "https://example.com" {
		t.Errorf("url = %v", detail.URL)
	}
	if detail.Code == nil || *detail.Code != 200 {
		t.Errorf("code = %v", detail.Code)
	}
}

func TestDeriveShellDetailWithNilInput(t *testing.T) {
	detail := DeriveToolCallDetail("shell", nil, nil).(protocol.ShellDetail)
	if detail.Type != "shell" {
		t.Errorf("type = %v, want shell", detail.Type)
	}
	if detail.Command != "" {
		t.Errorf("command field missing: expected empty string default for client schema compatibility")
	}
}

func TestDeriveReadDetailWithNilInput(t *testing.T) {
	detail := DeriveToolCallDetail("read", nil, nil).(protocol.ReadDetail)
	if detail.Type != "read" {
		t.Errorf("type = %v, want read", detail.Type)
	}
	if detail.FilePath != "" {
		t.Errorf("filePath field missing: expected empty string default for client schema compatibility")
	}
}

func TestDeriveWriteDetailWithNilInput(t *testing.T) {
	detail := DeriveToolCallDetail("write", nil, nil).(protocol.WriteDetail)
	if detail.Type != "write" {
		t.Errorf("type = %v, want write", detail.Type)
	}
	if detail.FilePath != "" {
		t.Errorf("filePath field missing: expected empty string default for client schema compatibility")
	}
}

func TestDeriveEditDetailWithNilInput(t *testing.T) {
	detail := DeriveToolCallDetail("edit", nil, nil).(protocol.EditDetail)
	if detail.Type != "edit" {
		t.Errorf("type = %v, want edit", detail.Type)
	}
	if detail.FilePath != "" {
		t.Errorf("filePath field missing: expected empty string default for client schema compatibility")
	}
}

func TestDeriveSearchDetailWithNilInput(t *testing.T) {
	detail := DeriveToolCallDetail("search", nil, nil).(protocol.SearchDetail)
	if detail.Type != "search" {
		t.Errorf("type = %v, want search", detail.Type)
	}
	if detail.Query != "" {
		t.Errorf("query field missing: expected empty string default for client schema compatibility")
	}
}

func TestDeriveFetchDetailWithNilInput(t *testing.T) {
	detail := DeriveToolCallDetail("fetch", nil, nil).(protocol.FetchDetail)
	if detail.Type != "fetch" {
		t.Errorf("type = %v, want fetch", detail.Type)
	}
	if detail.URL != "" {
		t.Errorf("url field missing: expected empty string default for client schema compatibility")
	}
}
