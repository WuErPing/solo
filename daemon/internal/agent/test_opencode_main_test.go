//go:build ignore

package agent_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestOpenCodeCreateSessionDirect(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewOpenCodeAgentClient("", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.IsAvailable(ctx); err != nil {
		t.Skipf("opencode not available: %v", err)
	}

	cwd := t.TempDir()
	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      cwd,
	}

	t.Log("Creating session...")
	sess, err := client.CreateSession(ctx, config)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Log("Session created OK")
	defer sess.Close()

	modes, err := sess.GetAvailableModes(ctx)
	if err != nil {
		t.Logf("GetAvailableModes err: %v", err)
	} else {
		t.Logf("Modes: %v", modes)
	}

	ri, err := sess.GetRuntimeInfo(ctx)
	if err != nil {
		t.Logf("GetRuntimeInfo err: %v", err)
	} else {
		t.Logf("RuntimeInfo: %+v", ri)
	}

	fmt.Println("All OK")
}
