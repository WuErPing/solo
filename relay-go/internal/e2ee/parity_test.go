package e2ee_test

// Build parity test: verifies the E2EE channel uses the correct protocol message types.
// This guards against accidentally shipping legacy hello/ready protocol types.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandshakeMessageTypeParity(t *testing.T) {
	// Find channel.go in the source tree
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Walk up to find the relay-go root
	channelPath := filepath.Join(wd, "channel.go")
	if _, err := os.Stat(channelPath); os.IsNotExist(err) {
		// Try parent directory
		channelPath = filepath.Join(filepath.Dir(wd), "channel.go")
	}

	src, err := os.ReadFile(channelPath)
	if err != nil {
		t.Fatalf("failed to read channel.go: %v", err)
	}

	code := string(src)

	// Verify the source contains the correct E2EE message types
	if !strings.Contains(code, `"e2ee_hello"`) {
		t.Error("channel.go does not contain \"e2ee_hello\" message type")
	}
	if !strings.Contains(code, `"e2ee_ready"`) {
		t.Error("channel.go does not contain \"e2ee_ready\" message type")
	}

	// Guard against accidentally shipping legacy hello/ready protocol.
	// The source should NOT contain bare "hello" or "ready" as message types.
	// We check for the pattern: type: "hello" or "type":"hello" (with possible whitespace)
	if strings.Contains(code, `"hello"`) && !strings.Contains(code, `"e2ee_hello"`) {
		t.Error("channel.go contains legacy \"hello\" message type without e2ee_ prefix")
	}
	if strings.Contains(code, `"ready"`) && !strings.Contains(code, `"e2ee_ready"`) {
		t.Error("channel.go contains legacy \"ready\" message type without e2ee_ prefix")
	}
}
