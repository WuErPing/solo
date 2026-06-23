package base

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/WuErPing/solo/protocol"
)

// TruncateText truncates a string to maxLen, appending "..." if truncated.
func TruncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DeriveToolCallDetail parses tool input/output into typed detail.
func DeriveToolCallDetail(toolName string, input, output interface{}) protocol.ToolCallDetail {
	switch toolName {
	case "shell", "bash", "exec_command":
		return deriveShellDetail(input, output)
	case "read", "read_file":
		return deriveReadDetail(input, output)
	case "write", "write_file", "create_file":
		return deriveWriteDetail(input, output)
	case "edit", "apply_patch", "apply_diff":
		return deriveEditDetail(input, output)
	case "search", "web_search", "grep", "glob":
		return deriveSearchDetail(input, output)
	case "fetch", "web_fetch":
		return deriveFetchDetail(input, output)
	default:
		if input != nil || output != nil {
			return protocol.UnknownDetail{
				Type:   "unknown",
				Input:  input,
				Output: output,
			}
		}
		return nil
	}
}

func deriveShellDetail(input, output interface{}) protocol.ToolCallDetail {
	detail := protocol.ShellDetail{
		Type:    "shell",
		Command: "",
	}

	if input != nil {
		if m, ok := input.(map[string]interface{}); ok {
			if cmd := ExtractStringOrJoinArray(m, "command", "cmd"); cmd != "" {
				detail.Command = cmd
			}
			if cwd := ExtractString(m, "cwd", "directory"); cwd != "" {
				detail.Cwd = cwd
			}
		}
	}
	if output != nil {
		if m, ok := output.(map[string]interface{}); ok {
			if out := ExtractString(m, "output", "text", "content", "aggregated_output", "aggregatedOutput"); out != "" {
				detail.Output = TruncateText(out, 2000)
			}
			if out := ExtractNestedString(m, "structuredContent", "structured_content", "result"); out != "" {
				if detail.Output == "" {
					detail.Output = TruncateText(out, 2000)
				}
			}
			if ec := ExtractInt(m, "exitCode", "exit_code"); ec != nil {
				detail.ExitCode = ec
			}
			if ec := ExtractNestedInt(m, "metadata", "exitCode", "exit_code"); ec != nil {
				if detail.ExitCode == nil {
					detail.ExitCode = ec
				}
			}
		} else if s, ok := output.(string); ok {
			detail.Output = TruncateText(s, 2000)
		}
	}
	return detail
}

func deriveReadDetail(input, output interface{}) protocol.ToolCallDetail {
	detail := protocol.ReadDetail{
		Type:     "read",
		FilePath: "",
	}

	if input != nil {
		if m, ok := input.(map[string]interface{}); ok {
			if fp := ExtractString(m, "path", "file_path", "filePath"); fp != "" {
				detail.FilePath = fp
			}
			if offset := ExtractInt(m, "offset"); offset != nil {
				detail.Offset = offset
			}
			if limit := ExtractInt(m, "limit"); limit != nil {
				detail.Limit = limit
			}
		}
	}
	if output != nil {
		if m, ok := output.(map[string]interface{}); ok {
			if content := ExtractString(m, "content", "text", "output"); content != "" {
				detail.Content = TruncateText(content, 2000)
			}
			if content := ExtractNestedString(m, "data", "structuredContent", "structured_content"); content != "" {
				if detail.Content == "" {
					detail.Content = TruncateText(content, 2000)
				}
			}
		} else if s, ok := output.(string); ok {
			detail.Content = TruncateText(s, 2000)
		}
	}
	return detail
}

func deriveWriteDetail(input, _ interface{}) protocol.ToolCallDetail {
	detail := protocol.WriteDetail{
		Type:     "write",
		FilePath: "",
	}

	if input != nil {
		if m, ok := input.(map[string]interface{}); ok {
			if fp := ExtractString(m, "path", "file_path", "filePath"); fp != "" {
				detail.FilePath = fp
			}
			if content := ExtractString(m, "content", "new_content", "newContent"); content != "" {
				detail.Content = TruncateText(content, 2000)
			}
		}
	}
	return detail
}

func deriveEditDetail(input, _ interface{}) protocol.ToolCallDetail {
	detail := protocol.EditDetail{
		Type:     "edit",
		FilePath: "",
	}

	if input != nil {
		if m, ok := input.(map[string]interface{}); ok {
			if fp := ExtractString(m, "path", "file_path", "filePath"); fp != "" {
				detail.FilePath = fp
			}
			if old := ExtractString(m, "old_string", "old_str", "oldContent", "old_content"); old != "" {
				detail.OldString = old
			}
			if newStr := ExtractString(m, "new_string", "new_str", "newContent", "new_content", "content"); newStr != "" {
				detail.NewString = TruncateText(newStr, 2000)
			}
			if diff := ExtractString(m, "patch", "diff", "unified_diff", "unifiedDiff"); diff != "" {
				detail.UnifiedDiff = TruncateText(diff, 2000)
			}
		}
	}
	return detail
}

func deriveSearchDetail(input, _ interface{}) protocol.ToolCallDetail {
	detail := protocol.SearchDetail{
		Type:  "search",
		Query: "",
	}

	if input != nil {
		if m, ok := input.(map[string]interface{}); ok {
			if q := ExtractString(m, "query", "q", "pattern"); q != "" {
				detail.Query = q
			}
			if toolName := ExtractString(m, "toolName", "tool_name"); toolName != "" {
				detail.ToolName = toolName
			}
		}
	}
	return detail
}

func deriveFetchDetail(input, output interface{}) protocol.ToolCallDetail {
	detail := protocol.FetchDetail{
		Type: "fetch",
		URL:  "",
	}

	if input != nil {
		if m, ok := input.(map[string]interface{}); ok {
			if url := ExtractString(m, "url"); url != "" {
				detail.URL = url
			}
			if prompt := ExtractString(m, "prompt"); prompt != "" {
				detail.Prompt = prompt
			}
		}
	}
	if output != nil {
		if m, ok := output.(map[string]interface{}); ok {
			if result := ExtractString(m, "result", "content", "text"); result != "" {
				detail.Result = TruncateText(result, 2000)
			}
			if code := ExtractInt(m, "code", "statusCode"); code != nil {
				detail.Code = code
			}
			if bytes := ExtractInt(m, "bytes", "size"); bytes != nil {
				detail.Bytes = bytes
			}
			if duration := ExtractInt(m, "durationMs", "duration_ms"); duration != nil {
				detail.DurationMs = duration
			}
		}
	}
	return detail
}

// --- Map extraction utilities ---

func ExtractString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func ExtractNumber(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if v != nil {
				return v
			}
		}
	}
	return nil
}

func ExtractInt(m map[string]interface{}, keys ...string) *int {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch n := v.(type) {
			case int:
				return &n
			case int64:
				i := int(n)
				return &i
			case float64:
				i := int(n)
				return &i
			case json.Number:
				if i, err := n.Int64(); err == nil {
					vi := int(i)
					return &vi
				}
			}
		}
	}
	return nil
}

func ExtractNestedInt(m map[string]interface{}, nestedKey string, keys ...string) *int {
	nested, ok := m[nestedKey].(map[string]interface{})
	if !ok {
		return nil
	}
	return ExtractInt(nested, keys...)
}

func ExtractStringOrJoinArray(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				var parts []string
				for _, item := range arr {
					if s, ok := item.(string); ok {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					return strings.Join(parts, " ")
				}
			}
		}
	}
	return ""
}

func ExtractNestedString(m map[string]interface{}, nestedKey string, keys ...string) string {
	nested, ok := m[nestedKey].(map[string]interface{})
	if !ok {
		return ""
	}
	return ExtractString(nested, keys...)
}

func ExtractNestedNumber(m map[string]interface{}, nestedKey string, keys ...string) interface{} {
	nested, ok := m[nestedKey].(map[string]interface{})
	if !ok {
		return nil
	}
	return ExtractNumber(nested, keys...)
}

// --- Permission title mapping ---

var permissionTitleMap = map[string]string{
	"bash": "Run shell command", "read": "Read files", "read_file": "Read files",
	"write": "Write files", "write_file": "Write files", "create_file": "Write files",
	"edit": "Edit files", "apply_patch": "Edit files", "apply_diff": "Edit files",
	"external_directory": "Access external directory",
}

// HumanReadablePermission converts a permission identifier to a human-readable title.
func HumanReadablePermission(permission string) string {
	if mapped, ok := permissionTitleMap[permission]; ok {
		return mapped
	}
	re := regexp.MustCompile(`[_\s]+`)
	parts := re.Split(permission, -1)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	result := strings.Join(parts, " ")
	if result == "" {
		return "Permission request"
	}
	return result
}
