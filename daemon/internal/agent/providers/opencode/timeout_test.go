package opencode

import (
	"testing"
	"time"
)

// TestOpenCodeHTTPRequestTimeoutAlignsWithSolo verifies that the HTTP request
// timeout matches Solo's 30s default. Solo uses the OpenCode SDK whose HTTP
// client defaults to 30s+; Solo must align to avoid spurious timeouts when the
// OpenCode server is busy processing a prompt.
func TestOpenCodeHTTPRequestTimeoutAlignsWithSolo(t *testing.T) {
	expected := 30 * time.Second
	if opencodeHTTPRequestTimeout != expected {
		t.Errorf("opencodeHTTPRequestTimeout = %v, want %v (Solo alignment)", opencodeHTTPRequestTimeout, expected)
	}
}

// TestOpenCodeCommandListTimeoutAlignsWithSolo verifies that the command list
// timeout matches Solo's 30s client timeout. A 10s value causes "/" command
// loading to fail when the OpenCode server is under load.
func TestOpenCodeCommandListTimeoutAlignsWithSolo(t *testing.T) {
	expected := 30 * time.Second
	if opencodeCommandListTimeout != expected {
		t.Errorf("opencodeCommandListTimeout = %v, want %v (Solo alignment)", opencodeCommandListTimeout, expected)
	}
}

// TestOpenCodeListClientCommandsAcquireTimeoutAlignsWithSolo verifies that
// ListClientCommands uses a 30s acquire+HTTP timeout rather than 8s. When the
// OpenCode server is cold-starting, 8s is insufficient and returns an empty
// command list prematurely.
func TestOpenCodeListClientCommandsAcquireTimeoutAlignsWithSolo(t *testing.T) {
	expected := 30 * time.Second
	if opencodeListCommandsAcquireTimeout != expected {
		t.Errorf("opencodeListCommandsAcquireTimeout = %v, want %v (Solo alignment)", opencodeListCommandsAcquireTimeout, expected)
	}
}

// TestOpenCodeSSEReadIdleTimeoutValue verifies the SSE idle timeout is set
// to 120s. This prevents consumeSSE from blocking indefinitely on a half-open
// TCP connection when the OpenCode server stops sending events but does not
// close the connection.
func TestOpenCodeSSEReadIdleTimeoutValue(t *testing.T) {
	expected := 120 * time.Second
	if opencodeSSEReadIdleTimeout != expected {
		t.Errorf("opencodeSSEReadIdleTimeout = %v, want %v", opencodeSSEReadIdleTimeout, expected)
	}
}
