package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/WuErPing/solo/protocol"
)

func readPositiveFloat(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	var f float64
	switch val := v.(type) {
	case float64:
		f = val
	case json.Number:
		n, err := val.Float64()
		if err != nil {
			return nil
		}
		f = n
	default:
		return nil
	}
	if f <= 0 {
		return nil
	}
	return &f
}

// --- Tool Call Detail Mapping (gap #1, matches Solo's deriveOpencodeToolDetail) ---

func normalizeToolStatus(status string) string {
	lower := strings.ToLower(strings.TrimSpace(status))
	switch lower {
	case "complete", "completed", "success", "succeeded", "done":
		return "completed"
	case "error", "failed", "failure":
		return "failed"
	case "canceled", "cancelled", "aborted", "interrupted":
		return "canceled"
	default:
		return "running"
	}
}
func buildToolCallTimelineItem(callID, toolName, status string, input, output, errorVal interface{}) TimelineItem {
	item := TimelineItem{
		Type:   "tool_call",
		CallID: callID,
		Name:   toolName,
		Status: status,
		Detail: deriveToolCallDetail(toolName, input, output),
	}

	if status == "failed" {
		if errorVal != nil {
			item.Error = &protocol.ToolError{Message: normalizeError(errorVal)}
		} else {
			item.Error = &protocol.ToolError{Message: "Tool call failed"}
		}
	}

	return item
}

// deriveToolCallDetail parses tool input/output into typed detail (matches Solo's deriveOpencodeToolDetail).
func deriveToolCallDetail(toolName string, input, output interface{}) protocol.ToolCallDetail {
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
			// Command can be string or string array
			if cmd := extractStringOrJoinArray(m, "command", "cmd"); cmd != "" {
				detail.Command = cmd
			}
			if cwd := extractString(m, "cwd", "directory"); cwd != "" {
				detail.Cwd = cwd
			}
		}
	}
	if output != nil {
		if m, ok := output.(map[string]interface{}); ok {
			if out := extractString(m, "output", "text", "content", "aggregated_output", "aggregatedOutput"); out != "" {
				detail.Output = truncateText(out, 2000)
			}
			// Also check structuredContent/structured_content/result
			if out := extractNestedString(m, "structuredContent", "structured_content", "result"); out != "" {
				if detail.Output == "" {
					detail.Output = truncateText(out, 2000)
				}
			}
			if ec := extractInt(m, "exitCode", "exit_code"); ec != nil {
				detail.ExitCode = ec
			}
			// Also check metadata.exitCode
			if ec := extractNestedInt(m, "metadata", "exitCode", "exit_code"); ec != nil {
				if detail.ExitCode == nil {
					detail.ExitCode = ec
				}
			}
		} else if s, ok := output.(string); ok {
			detail.Output = truncateText(s, 2000)
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
			if fp := extractString(m, "path", "file_path", "filePath"); fp != "" {
				detail.FilePath = fp
			}
			if offset := extractInt(m, "offset"); offset != nil {
				detail.Offset = offset
			}
			if limit := extractInt(m, "limit"); limit != nil {
				detail.Limit = limit
			}
		}
	}
	if output != nil {
		if m, ok := output.(map[string]interface{}); ok {
			if content := extractString(m, "content", "text", "output"); content != "" {
				detail.Content = truncateText(content, 2000)
			}
			// Check nested data/structuredContent
			if content := extractNestedString(m, "data", "structuredContent", "structured_content"); content != "" {
				if detail.Content == "" {
					detail.Content = truncateText(content, 2000)
				}
			}
		} else if s, ok := output.(string); ok {
			detail.Content = truncateText(s, 2000)
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
			if fp := extractString(m, "path", "file_path", "filePath"); fp != "" {
				detail.FilePath = fp
			}
			if content := extractString(m, "content", "new_content", "newContent"); content != "" {
				detail.Content = truncateText(content, 2000)
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
			if fp := extractString(m, "path", "file_path", "filePath"); fp != "" {
				detail.FilePath = fp
			}
			if old := extractString(m, "old_string", "old_str", "oldContent", "old_content"); old != "" {
				detail.OldString = old
			}
			if newStr := extractString(m, "new_string", "new_str", "newContent", "new_content", "content"); newStr != "" {
				detail.NewString = truncateText(newStr, 2000)
			}
			if diff := extractString(m, "patch", "diff", "unified_diff", "unifiedDiff"); diff != "" {
				detail.UnifiedDiff = truncateText(diff, 2000)
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
			if q := extractString(m, "query", "q", "pattern"); q != "" {
				detail.Query = q
			}
			if toolName := extractString(m, "toolName", "tool_name"); toolName != "" {
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
			if url := extractString(m, "url"); url != "" {
				detail.URL = url
			}
			if prompt := extractString(m, "prompt"); prompt != "" {
				detail.Prompt = prompt
			}
		}
	}
	if output != nil {
		if m, ok := output.(map[string]interface{}); ok {
			if result := extractString(m, "result", "content", "text"); result != "" {
				detail.Result = truncateText(result, 2000)
			}
			if code := extractInt(m, "code", "statusCode"); code != nil {
				detail.Code = code
			}
			if bytes := extractInt(m, "bytes", "size"); bytes != nil {
				detail.Bytes = bytes
			}
			if duration := extractInt(m, "durationMs", "duration_ms"); duration != nil {
				detail.DurationMs = duration
			}
		}
	}
	return detail
}
func extractString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
func extractNumber(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if v != nil {
				return v
			}
		}
	}
	return nil
}

func extractInt(m map[string]interface{}, keys ...string) *int {
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

func extractNestedInt(m map[string]interface{}, nestedKey string, keys ...string) *int {
	nested, ok := m[nestedKey].(map[string]interface{})
	if !ok {
		return nil
	}
	return extractInt(nested, keys...)
}

// extractStringOrJoinArray extracts a string value, or joins an array of strings with spaces.
func extractStringOrJoinArray(m map[string]interface{}, keys ...string) string {
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

// extractNestedString extracts a string from a nested map structure.
func extractNestedString(m map[string]interface{}, nestedKey string, keys ...string) string {
	nested, ok := m[nestedKey].(map[string]interface{})
	if !ok {
		return ""
	}
	return extractString(nested, keys...)
}

// extractNestedNumber extracts a number from a nested map structure.
func extractNestedNumber(m map[string]interface{}, nestedKey string, keys ...string) interface{} {
	nested, ok := m[nestedKey].(map[string]interface{})
	if !ok {
		return nil
	}
	return extractNumber(nested, keys...)
}

// --- Prompt Parts Builder (gap #10, matches Solo's buildOpenCodePromptParts) ---

func buildOpenCodePromptParts(text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) []map[string]interface{} {
	parts := []map[string]interface{}{
		{"type": "text", "text": text},
	}

	// Add images as file parts
	for i, img := range images {
		mimeType := img.MimeType
		if mimeType == "" {
			mimeType = "image/png"
		}
		ext := getAttachmentExtension(mimeType)
		url := img.Data
		if !strings.HasPrefix(url, "data:") {
			url = fmt.Sprintf("data:%s;base64,%s", mimeType, img.Data)
		}
		parts = append(parts, map[string]interface{}{
			"type":     "file",
			"mime":     mimeType,
			"filename": fmt.Sprintf("image-%d.%s", i+1, ext),
			"url":      url,
		})
	}

	// Add attachments
	for i, att := range attachments {
		if att.Type == "github_pr" || att.Type == "github_issue" {
			text := fmt.Sprintf("[%s] %s", att.Type, att.URL)
			if att.Title != "" {
				text = fmt.Sprintf("[%s] %s: %s", att.Type, att.Title, att.URL)
			}
			parts = append(parts, map[string]interface{}{"type": "text", "text": text})
			continue
		}
		// Binary attachment
		mimeType := att.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		ext := getAttachmentExtension(mimeType)
		url := att.URL
		if !strings.HasPrefix(url, "data:") && !strings.HasPrefix(url, "http") {
			url = fmt.Sprintf("data:%s;base64,%s", mimeType, url)
		}
		parts = append(parts, map[string]interface{}{
			"type":     "file",
			"mime":     mimeType,
			"filename": fmt.Sprintf("attachment-%d.%s", i+1, ext),
			"url":      url,
		})
	}

	return parts
}
func getAttachmentExtension(mimeType string) string {
	switch mimeType {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "image/svg+xml":
		return "svg"
	default:
		return "bin"
	}
}

// extractQuestionAnswers parses the response to build answers array for question.reply.
func extractQuestionAnswers(pendingInput map[string]interface{}, response protocol.AgentPermissionResponse) [][]string {
	questions, _ := pendingInput["questions"].([]map[string]interface{})
	var answers [][]string
	for _, q := range questions {
		header, _ := q["header"].(string)
		if header == "" {
			answers = append(answers, []string{})
			continue
		}
		// Try to get answer from response
		if response.UpdatedInput != nil {
			if answersMap, ok := response.UpdatedInput["answers"].(map[string]interface{}); ok {
				if val, ok := answersMap[header].(string); ok && val != "" {
					parts := strings.Split(val, ",")
					var cleaned []string
					for _, p := range parts {
						p = strings.TrimSpace(p)
						if p != "" {
							cleaned = append(cleaned, p)
						}
					}
					answers = append(answers, cleaned)
					continue
				}
			}
		}
		answers = append(answers, []string{})
	}
	return answers
}
func parseOpenCodeModel(model string) (providerID, modelID string) {
	if model == "" {
		return "", ""
	}
	idx := strings.Index(model, "/")
	if idx <= 0 || idx == len(model)-1 {
		return "", model
	}
	return model[:idx], model[idx+1:]
}
func stringOrNil(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw)
	}
	return s
}
func stringifyStructuredMessage(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
		return ""
	}
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if s == "" || s == "{}" || s == "null" {
		return ""
	}
	return s
}
func normalizeError(err interface{}) string {
	if err == nil {
		return "unknown error"
	}
	if s, ok := err.(string); ok && s != "" {
		return s
	}
	if b, err := json.Marshal(err); err == nil {
		return string(b)
	}
	return "unknown error"
}
func extractPermissionField(metadata json.RawMessage, keys []string) string {
	if metadata == nil {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(metadata, &obj); err != nil {
		return ""
	}
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if input, ok := obj["input"].(map[string]interface{}); ok {
		for _, key := range keys {
			if v, ok := input[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}
func parseSlashCommandInput(text string) (name, args string) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") || len(trimmed) <= 1 {
		return "", ""
	}
	withoutPrefix := trimmed[1:]
	idx := strings.IndexAny(withoutPrefix, " \t")
	if idx == -1 {
		return withoutPrefix, ""
	}
	name = withoutPrefix[:idx]
	args = strings.TrimSpace(withoutPrefix[idx+1:])
	if name == "" || strings.Contains(name, "/") {
		return "", ""
	}
	return name, args
}
func isHeadersTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, token := range opencodeHeadersTimeoutTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

var opencodeDefaultModeDescriptions = map[string]string{
	"build": "Allows edits and tool execution for implementation work",
	"plan":  "Read-only planning mode that avoids file edits",
}

func opencodeDefaultModes() []protocol.AgentMode {
	return []protocol.AgentMode{
		{ID: "build", Label: "Build", Description: "Allows edits and tool execution for implementation work"},
		{ID: "plan", Label: "Plan", Description: "Read-only planning mode that avoids file edits"},
	}
}
func sortOpenCodeModes(modes []protocol.AgentMode) []protocol.AgentMode {
	order := map[string]int{"build": 0, "plan": 1}
	sorted := make([]protocol.AgentMode, len(modes))
	copy(sorted, modes)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			oi, oki := order[sorted[i].ID]
			oj, okj := order[sorted[j].ID]
			if !oki {
				oi = 100
			}
			if !okj {
				oj = 100
			}
			if oi > oj || (oi == oj && sorted[i].ID > sorted[j].ID) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

var fatalRetryTokens = []string{
	"insufficient balance", "no resource package", "please recharge",
	"invalid api key", "unauthorized", "authentication",
	"model not found", "unknown model", "does not exist", "unsupported model",
}

func isFatalRetryMessage(msg string) bool {
	lower := strings.ToLower(msg)
	for _, token := range fatalRetryTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

var permissionTitleMap = map[string]string{
	"bash": "Run shell command", "read": "Read files", "read_file": "Read files",
	"write": "Write files", "write_file": "Write files", "create_file": "Write files",
	"edit": "Edit files", "apply_patch": "Edit files", "apply_diff": "Edit files",
	"external_directory": "Access external directory",
}

func humanReadablePermission(permission string) string {
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
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
