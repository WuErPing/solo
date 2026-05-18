package push

import (
	"strings"
	"unicode/utf8"
)

const notificationPreviewLimit = 220

// NotificationPayload represents the data sent to Expo Push API.
type NotificationPayload struct {
	Title string           `json:"title"`
	Body  string           `json:"body"`
	Data  NotificationData `json:"data"`
}

// NotificationData contains routing information for the client.
type NotificationData struct {
	AgentID  string `json:"agentId"`
	Reason   string `json:"reason"`
	ServerID string `json:"serverId,omitempty"`
}

// BuildAttentionNotification creates a notification payload for an agent attention event.
func BuildAttentionNotification(agentID string, reason string, assistantMessage string) NotificationPayload {
	return buildAttentionNotificationWithServerID(agentID, reason, assistantMessage, "")
}

// BuildAttentionNotificationWithServerID creates a notification payload including the server ID.
func BuildAttentionNotificationWithServerID(agentID string, reason string, assistantMessage string, serverID string) NotificationPayload {
	return buildAttentionNotificationWithServerID(agentID, reason, assistantMessage, serverID)
}

func buildAttentionNotificationWithServerID(agentID string, reason string, assistantMessage string, serverID string) NotificationPayload {
	title := resolveTitle(reason)
	body := resolveBody(reason, assistantMessage)

	return NotificationPayload{
		Title: title,
		Body:  body,
		Data: NotificationData{
			AgentID:  agentID,
			Reason:   reason,
			ServerID: serverID,
		},
	}
}

func resolveTitle(reason string) string {
	switch reason {
	case "permission":
		return "Agent needs permission"
	case "error":
		return "Agent needs attention"
	default:
		return "Agent finished"
	}
}

func resolveBody(reason string, assistantMessage string) string {
	switch reason {
	case "finished":
		if assistantMessage == "" {
			return "Finished working."
		}
		preview := buildNotificationPreview(assistantMessage)
		if preview != "" {
			return preview
		}
		return "Finished working."
	case "permission":
		return "Permission requested."
	case "error":
		return "Encountered an error."
	default:
		return "Finished working."
	}
}

func buildNotificationPreview(text string) string {
	if text == "" {
		return ""
	}

	stripped := stripMarkdown(text)
	normalized := normalizeWhitespace(stripped)
	if normalized == "" {
		return ""
	}

	return truncateText(normalized, notificationPreviewLimit)
}

func stripMarkdown(text string) string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")

	// Strip fenced code markers but keep content
	text = strings.ReplaceAll(text, "```", "")
	text = strings.ReplaceAll(text, "~~~", "")

	// Markdown links/images: [text](url) or ![alt](url)
	// Simple regex-like replacement for common patterns
	for {
		start := strings.Index(text, "[")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "]")
		if end == -1 {
			break
		}
		linkEnd := strings.Index(text[start+end:], ")")
		if linkEnd == -1 {
			break
		}
		// Extract link text
		linkText := text[start+1 : start+end]
		text = text[:start] + linkText + text[start+end+linkEnd+1:]
	}

	// Remove image markers ![alt](url)
	text = strings.ReplaceAll(text, "!", "")

	// Headers
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		for j := 0; j < len(trimmed) && j < 6; j++ {
			if trimmed[j] != '#' {
				if j > 0 && trimmed[j] == ' ' {
					lines[i] = trimmed[j+1:]
				}
				break
			}
		}
	}
	text = strings.Join(lines, "\n")

	// Blockquotes
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, ">") {
			lines[i] = strings.TrimPrefix(trimmed, ">")
			lines[i] = strings.TrimLeft(lines[i], " ")
		}
	}
	text = strings.Join(lines, "\n")

	// Lists
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if len(trimmed) > 0 {
			// Check for bullet or number list
			if trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+' {
				if len(trimmed) > 1 && trimmed[1] == ' ' {
					lines[i] = trimmed[2:]
				}
			}
		}
	}
	text = strings.Join(lines, "\n")

	// Inline formatting
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, "~~", "")
	text = strings.ReplaceAll(text, "`", "")

	// Horizontal rules
	lines = strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "---") || strings.Contains(trimmed, "***") || strings.Contains(trimmed, "___") {
			if len(trimmed) >= 3 {
				allSame := true
				first := trimmed[0]
				for _, ch := range trimmed {
					if ch != rune(first) {
						allSame = false
						break
					}
				}
				if allSame && (first == '-' || first == '*' || first == '_') {
					lines[i] = ""
				}
			}
		}
	}
	text = strings.Join(lines, "\n")

	return text
}

func normalizeWhitespace(text string) string {
	// Replace newlines with spaces
	text = strings.ReplaceAll(text, "\n", " ")
	// Collapse multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return strings.TrimSpace(text)
}

func truncateText(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	trimmed := string(runes[:limit-3])
	trimmed = strings.TrimRight(trimmed, " ")
	if trimmed != "" {
		return trimmed + "..."
	}
	return string(runes[:limit])
}
