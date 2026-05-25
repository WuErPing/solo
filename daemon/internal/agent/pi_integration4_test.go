package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_TimedSteps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewPiAgentClient("", logger)

	if err := client.IsAvailable(context.Background()); err != nil {
		t.Skipf("pi not available: %v", err)
	}

	config := &protocol.AgentSessionConfig{
		Provider: "pi",
		Cwd:      "/tmp",
	}

	start := time.Now()
	sess, err := client.CreateSession(context.Background(), config)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Logf("create session: %v", time.Since(start))
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start = time.Now()
	result, err := sess.Run(ctx, "hi", nil, nil, "")
	t.Logf("run took %v, result=%+v, err=%v", time.Since(start), result, err)
}
