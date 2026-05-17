package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

const maxInitialAgentTitleChars = 60

func deriveInitialAgentTitle(prompt string) *string {
	for _, line := range strings.Split(prompt, "\n") {
		normalized := strings.Join(strings.Fields(line), " ")
		if normalized == "" {
			continue
		}
		if title := classifyInitialPromptTitle(normalized); title != nil {
			return title
		}
		if len(normalized) > maxInitialAgentTitleChars {
			normalized = strings.TrimSpace(normalized[:maxInitialAgentTitleChars])
		}
		if normalized == "" {
			return nil
		}
		return &normalized
	}
	return nil
}

func classifyInitialPromptTitle(prompt string) *string {
	normalized := strings.ToLower(strings.Trim(prompt, " \t\r\n?.!"))
	switch normalized {
	case "who are you", "what are you", "identify yourself":
		title := "Identity inquiry"
		return &title
	default:
		return nil
	}
}

func resolveCreateAgentTitles(configTitle *string, initialPrompt *string) (explicitTitle *string, provisionalTitle *string) {
	if configTitle != nil {
		title := strings.TrimSpace(*configTitle)
		if title != "" {
			return &title, &title
		}
	}
	if initialPrompt == nil {
		return nil, nil
	}
	prompt := strings.TrimSpace(*initialPrompt)
	if prompt == "" {
		return nil, nil
	}
	return nil, deriveInitialAgentTitle(prompt)
}

// relPath returns the path of target relative to base, using forward slashes.
// If target equals base, it returns ".". On error, it falls back to target.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

// isTextContent checks if data is likely text by looking for null bytes.
func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	// Check first 8KB for null bytes
	check := data
	if len(check) > 8192 {
		check = check[:8192]
	}
	for _, b := range check {
		if b == 0 {
			return false
		}
	}
	return true
}

func isTextMimeType(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	textTypes := []string{
		"application/json", "application/xml", "application/javascript",
		"application/typescript", "application/x-yaml", "application/yaml",
		"application/toml", "application/x-sh", "application/x-shellscript",
	}
	for _, t := range textTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}

func summarizeAgentIDMatches(ids []string) string {
	limit := len(ids)
	if limit > 5 {
		limit = 5
	}
	parts := make([]string, 0, limit)
	for _, id := range ids[:limit] {
		if len(id) > 8 {
			parts = append(parts, id[:8])
		} else {
			parts = append(parts, id)
		}
	}
	summary := strings.Join(parts, ", ")
	if len(ids) > limit {
		summary += ", ..."
	}
	return summary
}

func normalizeWaitForFinishStatus(status string) string {
	switch status {
	case "error", "permission", "timeout":
		return status
	default:
		return "idle"
	}
}

func strPtr(s string) *string { return &s }

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func textPrefix(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func snapshotPtr(s protocol.AgentSnapshotPayload) *protocol.AgentSnapshotPayload { return &s }

func normalizeProjectCwd(cwd string) string {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return "."
	}
	if abs, err := filepath.Abs(trimmed); err == nil {
		return abs
	}
	return trimmed
}

func deriveProjectGroupingKey(cwd string, remoteURL *string, mainRepoRoot *string) string {
	if remoteKey := deriveRemoteProjectKey(remoteURL); remoteKey != "" {
		return remoteKey
	}
	if mainRepoRoot != nil && strings.TrimSpace(*mainRepoRoot) != "" {
		return normalizeProjectCwd(*mainRepoRoot)
	}
	return normalizeProjectCwd(cwd)
}

func deriveRemoteProjectKey(remoteURL *string) string {
	if remoteURL == nil {
		return ""
	}
	trimmed := strings.TrimSpace(*remoteURL)
	if trimmed == "" {
		return ""
	}

	var host, remotePath string
	if at := strings.Index(trimmed, "@"); at >= 0 {
		afterAt := trimmed[at+1:]
		if colon := strings.Index(afterAt, ":"); colon > 0 {
			host = afterAt[:colon]
			remotePath = afterAt[colon+1:]
		}
	}
	if (host == "" || remotePath == "") && strings.Contains(trimmed, "://") {
		withoutScheme := trimmed
		if schemeIdx := strings.Index(withoutScheme, "://"); schemeIdx >= 0 {
			withoutScheme = withoutScheme[schemeIdx+3:]
		}
		if slash := strings.Index(withoutScheme, "/"); slash > 0 {
			host = withoutScheme[:slash]
			if at := strings.LastIndex(host, "@"); at >= 0 {
				host = host[at+1:]
			}
			remotePath = withoutScheme[slash+1:]
		}
	}
	if host == "" || remotePath == "" {
		return ""
	}

	remotePath = strings.Trim(remotePath, "/")
	remotePath = strings.TrimSuffix(remotePath, ".git")
	if !strings.Contains(remotePath, "/") {
		return ""
	}

	return "remote:" + strings.ToLower(host) + "/" + remotePath
}

func deriveProjectGroupingName(projectKey string) string {
	const githubRemotePrefix = "remote:github.com/"
	if strings.HasPrefix(projectKey, githubRemotePrefix) {
		if name := strings.TrimSpace(strings.TrimPrefix(projectKey, githubRemotePrefix)); name != "" {
			return name
		}
	}
	normalized := strings.ReplaceAll(projectKey, "\\", "/")
	segments := strings.FieldsFunc(normalized, func(r rune) bool { return r == '/' })
	if len(segments) == 0 {
		return projectKey
	}
	return segments[len(segments)-1]
}

func matchesFetchAgentsFilter(agent protocol.AgentSnapshotPayload, project protocol.ProjectPlacementPayload, filter *protocol.FetchAgentsFilter, defaultIncludeArchived bool) bool {
	includeArchived := defaultIncludeArchived
	if filter != nil && filter.IncludeArchived != nil {
		includeArchived = *filter.IncludeArchived
	}
	if !includeArchived && agent.ArchivedAt != nil {
		return false
	}
	if filter == nil {
		return true
	}
	for key, value := range filter.Labels {
		if agent.Labels[key] != value {
			return false
		}
	}
	if len(filter.ProjectKeys) > 0 {
		found := false
		for _, key := range filter.ProjectKeys {
			if strings.TrimSpace(key) != "" && key == project.ProjectKey {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Statuses) > 0 {
		found := false
		for _, status := range filter.Statuses {
			if status == agent.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.RequiresAttention != nil && agent.RequiresAttention != *filter.RequiresAttention {
		return false
	}
	return true
}

// extractTimelineItem converts a timeline item from either agent.TimelineItem
// or map[string]interface{} (fallback) to agent.TimelineItem.
func extractTimelineItem(v interface{}) agent.TimelineItem {
	if item, ok := v.(agent.TimelineItem); ok {
		return item
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return agent.TimelineItem{}
	}
	item := agent.TimelineItem{}
	if t, ok := m["type"].(string); ok {
		item.Type = t
	}
	if t, ok := m["text"].(string); ok {
		item.Text = t
	}
	if t, ok := m["messageId"].(string); ok {
		item.MessageID = t
	}
	if t, ok := m["callId"].(string); ok {
		item.CallID = t
	}
	if t, ok := m["name"].(string); ok {
		item.Name = t
	}
	item.Detail = m["detail"]
	if t, ok := m["status"].(string); ok {
		item.Status = t
	}
	item.Error = m["error"]
	if md, ok := m["metadata"].(map[string]interface{}); ok {
		item.Metadata = md
	}
	if items, ok := m["items"].([]interface{}); ok {
		for _, it := range items {
			if im, ok := it.(map[string]interface{}); ok {
				ti := agent.TodoItem{}
				if t, ok := im["text"].(string); ok {
					ti.Text = t
				}
				if t, ok := im["completed"].(bool); ok {
					ti.Completed = t
				}
				item.TodoItems = append(item.TodoItems, ti)
			}
		}
	}
	if t, ok := m["message"].(string); ok {
		item.Message = t
	}
	if t, ok := m["compactionStatus"].(string); ok {
		item.CompactionStatus = t
	}
	if t, ok := m["trigger"].(string); ok {
		item.Trigger = t
	}
	if t, ok := m["preTokens"].(float64); ok {
		item.PreTokens = int(t)
	}
	return item
}

func workspaceRootMatchesConfigRequest(ws *protocol.WorkspaceDescriptor, requested string) bool {
	if ws == nil {
		return false
	}
	return requested == canonicalizeConfigRoot(ws.ProjectRootPath) ||
		requested == canonicalizeConfigRoot(ws.WorkspaceDirectory)
}

func canonicalizeConfigRoot(repoRoot string) string {
	trimmed := strings.TrimSpace(repoRoot)
	if trimmed == "" {
		return ""
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		resolved = filepath.Clean(trimmed)
	}
	if realPath, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = realPath
	}
	return stripTrailingPathSeparators(resolved)
}

func stripTrailingPathSeparators(path string) string {
	for len(path) > 1 && strings.HasSuffix(path, string(filepath.Separator)) {
		path = strings.TrimSuffix(path, string(filepath.Separator))
	}
	return path
}

func protocolRevisionFromWorkspace(revision *workspace.ProjectConfigRevision) *protocol.ProjectConfigRevision {
	if revision == nil {
		return nil
	}
	return &protocol.ProjectConfigRevision{
		MtimeMs: revision.MtimeMs,
		Size:    revision.Size,
	}
}

func workspaceRevisionFromProtocol(revision *protocol.ProjectConfigRevision) *workspace.ProjectConfigRevision {
	if revision == nil {
		return nil
	}
	return &workspace.ProjectConfigRevision{
		MtimeMs: revision.MtimeMs,
		Size:    revision.Size,
	}
}

// buildSetupLog formats a log string from command snapshots.
func buildSetupLog(commands []workspace.SetupCommandSnapshot) string {
	var builder strings.Builder
	for _, cmd := range commands {
		if cmd.Status == "running" {
			builder.WriteString(fmt.Sprintf("==> [%d] Running: %s\n", cmd.Index, cmd.Command))
		} else {
			exitCode := -1
			if cmd.ExitCode != nil {
				exitCode = *cmd.ExitCode
			}
			duration := 0
			if cmd.DurationMs != nil {
				duration = *cmd.DurationMs
			}
			builder.WriteString(fmt.Sprintf("==> [%d] Running: %s\n", cmd.Index, cmd.Command))
			if cmd.Log != "" {
				builder.WriteString(cmd.Log)
			}
			builder.WriteString(fmt.Sprintf("<== [%d] Exit %d in %dms\n", cmd.Index, exitCode, duration))
		}
	}
	return builder.String()
}

// convertCommandSnapshots converts workspace snapshots to protocol snapshots.
func convertCommandSnapshots(commands []workspace.SetupCommandSnapshot) []protocol.WorktreeSetupCommandSnapshot {
	result := make([]protocol.WorktreeSetupCommandSnapshot, len(commands))
	for i, cmd := range commands {
		result[i] = protocol.WorktreeSetupCommandSnapshot{
			Index:      cmd.Index,
			Command:    cmd.Command,
			Cwd:        cmd.Cwd,
			Log:        cmd.Log,
			Status:     cmd.Status,
			ExitCode:   cmd.ExitCode,
			DurationMs: cmd.DurationMs,
		}
	}
	return result
}
