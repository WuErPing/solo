package push

import (
	"strings"
	"testing"
)

func TestBuildAttentionNotification_Finished(t *testing.T) {
	payload := BuildAttentionNotification("agent1", "finished", "Hello world")

	if payload.Title != "Agent finished" {
		t.Errorf("expected title 'Agent finished', got %q", payload.Title)
	}
	if payload.Body != "Hello world" {
		t.Errorf("expected body 'Hello world', got %q", payload.Body)
	}
	if payload.Data.AgentID != "agent1" {
		t.Errorf("expected agentId 'agent1', got %q", payload.Data.AgentID)
	}
	if payload.Data.Reason != "finished" {
		t.Errorf("expected reason 'finished', got %q", payload.Data.Reason)
	}
}

func TestBuildAttentionNotification_Error(t *testing.T) {
	payload := BuildAttentionNotification("agent2", "error", "")

	if payload.Title != "Agent needs attention" {
		t.Errorf("expected title 'Agent needs attention', got %q", payload.Title)
	}
	if payload.Body != "Encountered an error." {
		t.Errorf("expected body 'Encountered an error.', got %q", payload.Body)
	}
}

func TestBuildAttentionNotification_Permission(t *testing.T) {
	payload := BuildAttentionNotification("agent3", "permission", "")

	if payload.Title != "Agent needs permission" {
		t.Errorf("expected title 'Agent needs permission', got %q", payload.Title)
	}
	if payload.Body != "Permission requested." {
		t.Errorf("expected body 'Permission requested.', got %q", payload.Body)
	}
}

func TestBuildAttentionNotification_MarkdownStripping(t *testing.T) {
	input := "# Title\n\nThis is **bold** and _italic_.\n\n```go\nfmt.Println(\"hello\")\n```\n\n[Link](http://example.com)"
	payload := BuildAttentionNotification("agent1", "finished", input)

	// Should not contain markdown markers
	if strings.Contains(payload.Body, "#") {
		t.Error("body should not contain markdown headers")
	}
	if strings.Contains(payload.Body, "**") {
		t.Error("body should not contain bold markers")
	}
	if strings.Contains(payload.Body, "```") {
		t.Error("body should not contain code blocks")
	}
	if strings.Contains(payload.Body, "[") {
		t.Error("body should not contain link markers")
	}
}

func TestBuildAttentionNotification_Truncation(t *testing.T) {
	// Create a message longer than 220 chars
	longMessage := strings.Repeat("a", 250)
	payload := BuildAttentionNotification("agent1", "finished", longMessage)

	if len(payload.Body) > 223 { // 220 + "..."
		t.Errorf("body too long: %d chars", len(payload.Body))
	}
	if !strings.HasSuffix(payload.Body, "...") {
		t.Error("body should be truncated with '...'")
	}
}

func TestBuildAttentionNotification_EmptyMessage(t *testing.T) {
	payload := BuildAttentionNotification("agent1", "finished", "")

	if payload.Body != "Finished working." {
		t.Errorf("expected fallback body 'Finished working.', got %q", payload.Body)
	}
}

func TestBuildAttentionNotification_WhitespaceNormalization(t *testing.T) {
	input := "Hello\n\n\nWorld   with   spaces"
	payload := BuildAttentionNotification("agent1", "finished", input)

	if strings.Contains(payload.Body, "\n\n") {
		t.Error("body should normalize whitespace")
	}
	if strings.Contains(payload.Body, "   ") {
		t.Error("body should collapse multiple spaces")
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "headers",
			input:    "# Header 1\n## Header 2",
			expected: "Header 1\nHeader 2",
		},
		{
			name:     "bold",
			input:    "**bold text**",
			expected: "bold text",
		},
		{
			name:     "italic",
			input:    "*italic text*",
			expected: "italic text",
		},
		{
			name:     "code inline",
			input:    "use `code` here",
			expected: "use code here",
		},
		{
			name:     "code block",
			input:    "```go\nfmt.Println()\n```",
			expected: "go\nfmt.Println()\n",
		},
		{
			name:     "link",
			input:    "[text](http://example.com)",
			expected: "text",
		},
		{
			name:     "blockquote",
			input:    "> quoted text",
			expected: "quoted text",
		},
		{
			name:     "list",
			input:    "- item 1\n- item 2",
			expected: "item 1\nitem 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
