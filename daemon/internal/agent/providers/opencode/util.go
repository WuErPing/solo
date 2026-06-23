package opencode

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
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
func buildToolCallTimelineItem(callID, toolName, status string, input, output, errorVal interface{}) protocol.TimelineItem {
	item := protocol.TimelineItem{
		Type:   "tool_call",
		CallID: callID,
		Name:   toolName,
		Status: status,
		Detail: base.DeriveToolCallDetail(toolName, input, output),
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
