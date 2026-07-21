// Package push sends push notifications to clients.
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

// fencedCodeMarkers are removed (content kept) before other processing.
var fencedCodeMarkers = []string{"```", "~~~"}

// inlineMarkers are removed in order; multi-char markers precede their
// single-char subsets so "**" is stripped before "*" and "__" before "_".
var inlineMarkers = []string{"**", "__", "*", "_", "~~", "`"}

// linePrefixStrippers run in order on each line to remove leading markdown
// syntax (headers, blockquotes, list bullets).
var linePrefixStrippers = []func(string) string{
	stripHeaderPrefix,
	stripBlockquotePrefix,
	stripListPrefix,
}

func stripMarkdown(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = stripFencedCodeMarkers(text)
	text = stripMarkdownLinks(text)
	text = strings.ReplaceAll(text, "!", "")
	text = stripLinePrefixes(text)
	text = stripInlineFormatting(text)
	text = stripHorizontalRules(text)
	return text
}

func stripFencedCodeMarkers(text string) string {
	for _, m := range fencedCodeMarkers {
		text = strings.ReplaceAll(text, m, "")
	}
	return text
}

// stripMarkdownLinks replaces [text](url) (and ![alt](url), once the leading
// "!" is later removed) with just the link text.
func stripMarkdownLinks(text string) string {
	for {
		start := strings.Index(text, "[")
		if start == -1 {
			return text
		}
		end := strings.Index(text[start:], "]")
		if end == -1 {
			return text
		}
		linkEnd := strings.Index(text[start+end:], ")")
		if linkEnd == -1 {
			return text
		}
		linkText := text[start+1 : start+end]
		text = text[:start] + linkText + text[start+end+linkEnd+1:]
	}
}

func stripLinePrefixes(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		for _, strip := range linePrefixStrippers {
			line = strip(line)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// stripHeaderPrefix removes up to six leading '#' markers plus the following
// space. A '#' run not followed by a space (e.g. "#Hello") is left intact.
func stripHeaderPrefix(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	for j := 0; j < len(trimmed) && j < 6; j++ {
		if trimmed[j] != '#' {
			if j > 0 && trimmed[j] == ' ' {
				return trimmed[j+1:]
			}
			return line
		}
	}
	return line
}

// stripBlockquotePrefix removes a single leading '>' (and surrounding spaces).
func stripBlockquotePrefix(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmed, ">") {
		return strings.TrimLeft(strings.TrimPrefix(trimmed, ">"), " ")
	}
	return line
}

// stripListPrefix removes a leading bullet ('-', '*', or '+') followed by a space.
func stripListPrefix(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) > 1 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
		return trimmed[2:]
	}
	return line
}

func stripInlineFormatting(text string) string {
	for _, m := range inlineMarkers {
		text = strings.ReplaceAll(text, m, "")
	}
	return text
}

func stripHorizontalRules(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if isHorizontalRule(line) {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// isHorizontalRule reports whether a line is a markdown horizontal rule: three
// or more of a single '-', '*', or '_' character (after trimming whitespace).
func isHorizontalRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	first := trimmed[0]
	if first != '-' && first != '*' && first != '_' {
		return false
	}
	for i := 1; i < len(trimmed); i++ {
		if trimmed[i] != first {
			return false
		}
	}
	return true
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
