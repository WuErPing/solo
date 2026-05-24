package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_NoSubscribe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewPiAgentClient("", logger)

	if err := client.IsAvailable(context.Background()); err != nil {
		t.Skipf("pi not available: %v", err)
	}

	config := &protocol.AgentSessionConfig{
		Provider: "pi",
		Cwd:      "/tmp",
	}

	sess, err := client.CreateSession(context.Background(), config)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := sess.Run(ctx, "hi", nil, nil)
	t.Logf("result=%+v err=%v", result, err)
}
