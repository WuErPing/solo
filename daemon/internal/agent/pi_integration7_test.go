package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_FullPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	sess := newPiSession("/Users/wuerping/.bun/bin/pi", &protocol.AgentSessionConfig{Cwd: "/tmp"}, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	start := time.Now()
	result, err := sess.Run(ctx, "hi", nil, nil, "")
	t.Logf("run took %v, result=%+v, err=%v", time.Since(start), result, err)
}
