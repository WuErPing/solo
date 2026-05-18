package output

import (
	"fmt"
	"io"
	"os"
)

// OutputFormat determines how command results are rendered.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
	FormatQuiet OutputFormat = "quiet"
)

// ParseOutputFormat validates and returns an OutputFormat.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch OutputFormat(s) {
	case FormatTable, "cli":
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatYAML:
		return FormatYAML, nil
	case FormatQuiet:
		return FormatQuiet, nil
	default:
		return "", fmt.Errorf("unsupported output format: %s (valid: table, json, yaml, quiet)", s)
	}
}

// OutputOptions controls how command output is rendered.
type OutputOptions struct {
	Format    OutputFormat
	NoColor   bool
	NoHeaders bool
	Quiet     bool
}

// ColumnDef defines a table column for rendering.
type ColumnDef struct {
	Header    string
	FieldFunc func(item interface{}) string
	Width     int                                         // minimum width; 0 = auto
	Align     string                                      // "left" (default) or "right"
	ColorFunc func(value string, item interface{}) string // returns color name or ""
}

// Schema describes how to render command output.
type Schema struct {
	IDField func(item interface{}) string // extracts ID for quiet mode
	Columns []ColumnDef
	// Serialize transforms data before JSON/YAML output. Optional.
	Serialize func(data interface{}) interface{}
}

// CommandResult holds the result of a command for rendering.
type CommandResult struct {
	IsSingle bool
	Single   interface{}
	List     []interface{}
	Schema   *Schema
}

// SingleResult creates a CommandResult for a single item.
func SingleResult(item interface{}, schema *Schema) *CommandResult {
	return &CommandResult{IsSingle: true, Single: item, Schema: schema}
}

// ListResult creates a CommandResult for a list of items.
func ListResult(items []interface{}, schema *Schema) *CommandResult {
	return &CommandResult{IsSingle: false, List: items, Schema: schema}
}

// CommandError is a structured error for CLI output.
type CommandError struct {
	Code    string
	Message string
	Details string
}

func (e *CommandError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s\n%s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Stdout and Stderr can be overridden in tests.
var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)
