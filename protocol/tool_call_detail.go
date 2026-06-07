package protocol

import (
	"encoding/json"
	"fmt"
)

// ToolCallDetail is a sealed union of all tool-call detail variants.
type ToolCallDetail interface {
	toolCallDetail()
	GetType() string
}

// --- Detail Variants ---

type ShellDetail struct {
	Type     string `json:"type"`
	Command  string `json:"command"`
	Cwd      string `json:"cwd,omitempty"`
	Output   string `json:"output,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
}

func (ShellDetail) toolCallDetail()   {}
func (d ShellDetail) GetType() string { return d.Type }

type ReadDetail struct {
	Type     string `json:"type"`
	FilePath string `json:"filePath"`
	Content  string `json:"content,omitempty"`
	Offset   *int   `json:"offset,omitempty"`
	Limit    *int   `json:"limit,omitempty"`
}

func (ReadDetail) toolCallDetail()   {}
func (d ReadDetail) GetType() string { return d.Type }

type WriteDetail struct {
	Type     string `json:"type"`
	FilePath string `json:"filePath"`
	Content  string `json:"content,omitempty"`
}

func (WriteDetail) toolCallDetail()   {}
func (d WriteDetail) GetType() string { return d.Type }

type EditDetail struct {
	Type        string `json:"type"`
	FilePath    string `json:"filePath"`
	OldString   string `json:"oldString,omitempty"`
	NewString   string `json:"newString,omitempty"`
	UnifiedDiff string `json:"unifiedDiff,omitempty"`
}

func (EditDetail) toolCallDetail()   {}
func (d EditDetail) GetType() string { return d.Type }

type SearchDetail struct {
	Type     string `json:"type"`
	Query    string `json:"query"`
	ToolName string `json:"toolName,omitempty"`
}

func (SearchDetail) toolCallDetail()   {}
func (d SearchDetail) GetType() string { return d.Type }

type FetchDetail struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	Prompt     string `json:"prompt,omitempty"`
	Result     string `json:"result,omitempty"`
	Code       *int   `json:"code,omitempty"`
	Bytes      *int   `json:"bytes,omitempty"`
	DurationMs *int   `json:"durationMs,omitempty"`
}

func (FetchDetail) toolCallDetail()   {}
func (d FetchDetail) GetType() string { return d.Type }

type PlainTextDetail struct {
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
	Text  string `json:"text,omitempty"`
}

func (PlainTextDetail) toolCallDetail()   {}
func (d PlainTextDetail) GetType() string { return d.Type }

type UnknownDetail struct {
	Type   string      `json:"type"`
	Input  interface{} `json:"input,omitempty"`
	Output interface{} `json:"output,omitempty"`
}

func (UnknownDetail) toolCallDetail()   {}
func (d UnknownDetail) GetType() string { return d.Type }

// ToolCallDetailWrapper is a helper for JSON unmarshalling of ToolCallDetail.
type ToolCallDetailWrapper struct {
	Detail ToolCallDetail
}

// UnmarshalJSON dispatches to the correct detail variant based on the "type" field.
func (w *ToolCallDetailWrapper) UnmarshalJSON(data []byte) error {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return fmt.Errorf("peek detail type: %w", err)
	}

	switch peek.Type {
	case "shell":
		var d ShellDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	case "read":
		var d ReadDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	case "write":
		var d WriteDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	case "edit":
		var d EditDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	case "search":
		var d SearchDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	case "fetch":
		var d FetchDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	case "plain_text":
		var d PlainTextDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	default:
		var d UnknownDetail
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		w.Detail = d
	}
	return nil
}

// MarshalJSON delegates to the concrete detail type.
func (w ToolCallDetailWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Detail)
}

// --- ToolError ---

// ToolError represents an error on a tool_call timeline item.
type ToolError struct {
	Message string `json:"message"`
}

func (e *ToolError) String() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// MarshalJSON serializes ToolError as a string for backward compatibility.
func (e *ToolError) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	return json.Marshal(e.Message)
}

// UnmarshalJSON accepts either a plain string or a {"message":...} object.
func (e *ToolError) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.Message = s
		return nil
	}
	// Fallback to object
	var obj struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		e.Message = obj.Message
		return nil
	}
	return fmt.Errorf("cannot unmarshal ToolError from %s", string(data))
}
