package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestParseOutputFormat_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected OutputFormat
	}{
		{"table", FormatTable},
		{"cli", FormatTable},
		{"json", FormatJSON},
		{"yaml", FormatYAML},
		{"quiet", FormatQuiet},
	}
	for _, tc := range tests {
		got, err := ParseOutputFormat(tc.input)
		if err != nil {
			t.Errorf("ParseOutputFormat(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.expected {
			t.Errorf("ParseOutputFormat(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestParseOutputFormat_Invalid(t *testing.T) {
	_, err := ParseOutputFormat("xml")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestCommandError_Error(t *testing.T) {
	e := &CommandError{Code: "ERR", Message: "something failed", Details: "try again"}
	if !strings.Contains(e.Error(), "ERR") {
		t.Error("expected Code in error")
	}
	if !strings.Contains(e.Error(), "something failed") {
		t.Error("expected Message in error")
	}
	if !strings.Contains(e.Error(), "try again") {
		t.Error("expected Details in error")
	}

	e2 := &CommandError{Code: "ERR", Message: "fail"}
	if strings.Contains(e2.Error(), "try again") {
		t.Error("expected no details")
	}
}

func TestRender_Nil(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, nil, OutputOptions{Format: FormatTable}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestRender_Table(t *testing.T) {
	var buf bytes.Buffer
	schema := &Schema{
		Columns: []ColumnDef{
			{Header: "NAME", FieldFunc: func(i interface{}) string { return i.(map[string]string)["name"] }, Width: 5},
			{Header: "VAL", FieldFunc: func(i interface{}) string { return i.(map[string]string)["val"] }, Width: 5, Align: "right"},
		},
	}
	result := ListResult([]interface{}{map[string]string{"name": "a", "val": "1"}}, schema)
	if err := Render(&buf, result, OutputOptions{Format: FormatTable, NoColor: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected header in output, got: %q", out)
	}
	if !strings.Contains(out, "a") {
		t.Errorf("expected data in output, got: %q", out)
	}
}

func TestRender_Table_NoHeaders(t *testing.T) {
	var buf bytes.Buffer
	schema := &Schema{
		Columns: []ColumnDef{
			{Header: "NAME", FieldFunc: func(i interface{}) string { return i.(map[string]string)["name"] }, Width: 5},
		},
	}
	result := ListResult([]interface{}{map[string]string{"name": "a"}}, schema)
	if err := Render(&buf, result, OutputOptions{Format: FormatTable, NoHeaders: true, NoColor: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "NAME") {
		t.Errorf("expected no header, got: %q", out)
	}
}

func TestRender_Table_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	schema := &Schema{
		Columns: []ColumnDef{{Header: "NAME", FieldFunc: func(_ interface{}) string { return "" }, Width: 5}},
	}
	result := ListResult([]interface{}{}, schema)
	if err := Render(&buf, result, OutputOptions{Format: FormatTable, NoColor: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty list, got: %q", buf.String())
	}
}

func TestRender_Table_NoSchema(t *testing.T) {
	var buf bytes.Buffer
	result := ListResult([]interface{}{"x"}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatTable, NoColor: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output without schema, got: %q", buf.String())
	}
}

func TestRender_JSON(t *testing.T) {
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"key": "value"}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatJSON}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}
	if parsed["key"] != "value" {
		t.Errorf("expected value, got %q", parsed["key"])
	}
}

func TestRender_JSON_List(t *testing.T) {
	var buf bytes.Buffer
	result := ListResult([]interface{}{map[string]string{"k": "v"}}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatJSON}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0]["k"] != "v" {
		t.Errorf("unexpected JSON output: %s", buf.String())
	}
}

func TestRender_JSON_Serialize(t *testing.T) {
	var buf bytes.Buffer
	schema := &Schema{
		Serialize: func(_ interface{}) interface{} {
			return map[string]string{"transformed": "yes"}
		},
	}
	result := SingleResult(map[string]string{"key": "value"}, schema)
	if err := Render(&buf, result, OutputOptions{Format: FormatJSON}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if parsed["transformed"] != "yes" {
		t.Errorf("expected transformed data, got %v", parsed)
	}
}

func TestRender_YAML(t *testing.T) {
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"key": "value"}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatYAML}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "key:") {
		t.Errorf("expected YAML output, got: %q", buf.String())
	}
}

func TestRender_Quiet_Single(t *testing.T) {
	var buf bytes.Buffer
	schema := &Schema{
		IDField: func(i interface{}) string { return i.(map[string]string)["id"] },
	}
	result := SingleResult(map[string]string{"id": "abc"}, schema)
	if err := Render(&buf, result, OutputOptions{Format: FormatQuiet}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "abc" {
		t.Errorf("expected abc, got: %q", buf.String())
	}
}

func TestRender_Quiet_List(t *testing.T) {
	var buf bytes.Buffer
	schema := &Schema{
		IDField: func(i interface{}) string { return i.(map[string]string)["id"] },
	}
	result := ListResult([]interface{}{
		map[string]string{"id": "a"},
		map[string]string{"id": "b"},
	}, schema)
	if err := Render(&buf, result, OutputOptions{Format: FormatQuiet}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" {
		t.Errorf("unexpected quiet output: %q", buf.String())
	}
}

func TestRender_Quiet_NoSchema(t *testing.T) {
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"id": "abc"}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatQuiet}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output without schema, got: %q", buf.String())
	}
}

func TestRenderError_JSON(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, &CommandError{Code: "ERR", Message: "fail"}, OutputOptions{Format: FormatJSON})
	if !strings.Contains(buf.String(), "ERR") {
		t.Errorf("expected error code in JSON stderr, got: %q", buf.String())
	}
}

func TestRenderError_YAML(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, &CommandError{Code: "ERR", Message: "fail"}, OutputOptions{Format: FormatYAML})
	if !strings.Contains(buf.String(), "error:") {
		t.Errorf("expected YAML error output, got: %q", buf.String())
	}
}

func TestRenderError_Default(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, &CommandError{Code: "ERR", Message: "fail", Details: "do this"}, OutputOptions{Format: FormatTable})
	if !strings.Contains(buf.String(), "fail") {
		t.Errorf("expected error message, got: %q", buf.String())
	}
}

func TestPrintResult_Success(t *testing.T) {
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"k": "v"}, nil)
	code := PrintResult(&buf, result, OutputOptions{Format: FormatJSON})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if buf.Len() == 0 {
		t.Error("expected output")
	}
}

func TestPrintResult_Nil(t *testing.T) {
	var buf bytes.Buffer
	code := PrintResult(&buf, nil, OutputOptions{Format: FormatTable})
	if code != 0 {
		t.Errorf("expected exit code 0 for nil result, got %d", code)
	}
}

func TestColorize(t *testing.T) {
	// Disable color for predictable strings
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	tests := []struct {
		name     string
		expected string
	}{
		{"red", "text"},
		{"green", "text"},
		{"yellow", "text"},
		{"cyan", "text"},
		{"blue", "text"},
		{"magenta", "text"},
		{"dim", "text"},
		{"bold", "text"},
		{"unknown", "text"},
	}
	for _, tc := range tests {
		got := Colorize(tc.name, "text")
		if got != tc.expected {
			t.Errorf("Colorize(%q) = %q, want %q", tc.name, got, tc.expected)
		}
	}
}

func TestColorHelpers(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = oldNoColor }()

	if Bold("x") != "x" {
		t.Error("Bold mismatch")
	}
	if Red("x") != "x" {
		t.Error("Red mismatch")
	}
	if Green("x") != "x" {
		t.Error("Green mismatch")
	}
	if Yellow("x") != "x" {
		t.Error("Yellow mismatch")
	}
	if Cyan("x") != "x" {
		t.Error("Cyan mismatch")
	}
	if Dim("x") != "x" {
		t.Error("Dim mismatch")
	}
	if !strings.Contains(ErrorText("oops"), "oops") {
		t.Error("ErrorText mismatch")
	}
}

func TestDisableColor(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = oldNoColor }()

	DisableColor()
	if !color.NoColor {
		t.Error("expected NoColor to be true")
	}
}

func TestPadCell_Left(t *testing.T) {
	if got := padCell("hi", 5, "left"); got != "hi   " {
		t.Errorf("expected 'hi   ', got %q", got)
	}
}

func TestPadCell_Right(t *testing.T) {
	if got := padCell("hi", 5, "right"); got != "   hi" {
		t.Errorf("expected '   hi', got %q", got)
	}
}

func TestPadCell_Center(t *testing.T) {
	if got := padCell("hi", 6, "center"); got != "  hi  " {
		t.Errorf("expected '  hi  ', got %q", got)
	}
}

func TestPadCell_NoPadding(t *testing.T) {
	if got := padCell("hello", 3, "left"); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestStripAnsi(t *testing.T) {
	// Build a string with ANSI codes
	colored := "\x1b[31mred\x1b[0m"
	if got := stripAnsi(colored); got != "red" {
		t.Errorf("expected 'red', got %q", got)
	}
	if got := stripAnsi("plain"); got != "plain" {
		t.Errorf("expected 'plain', got %q", got)
	}
	if got := stripAnsi(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestRenderToWriter verifies Render writes to the provided writer instead of globals.
func TestRenderToWriter(t *testing.T) {
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"key": "value"}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatJSON}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("expected value, got %q", parsed["key"])
	}
}

// TestRenderErrorToWriter verifies RenderError writes to the provided writer.
func TestRenderErrorToWriter(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, &CommandError{Code: "TEST", Message: "msg"}, OutputOptions{Format: FormatJSON})
	if buf.Len() == 0 {
		t.Error("expected RenderError to write to provided writer")
	}
}

// TestPrintResultToWriter verifies PrintResult writes to the provided writer.
func TestPrintResultToWriter(t *testing.T) {
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"id": "x"}, nil)
	code := PrintResult(&buf, result, OutputOptions{Format: FormatJSON})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if buf.Len() == 0 {
		t.Error("expected PrintResult to write to provided writer")
	}
}

// TestRenderDoesNotDependOnGlobalStdout ensures the global Stdout is never consulted.
func TestRenderDoesNotDependOnGlobalStdout(t *testing.T) {
	// Intentionally leave global Stdout as nil; Render should ignore it.
	var buf bytes.Buffer
	result := SingleResult(map[string]string{"k": "v"}, nil)
	if err := Render(&buf, result, OutputOptions{Format: FormatJSON}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected output even when global Stdout is untouched")
	}
}
