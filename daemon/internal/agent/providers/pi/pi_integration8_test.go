//go:build !short && !external_api

package pi

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_InstrumentedRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	sess := newPiSession("/Users/wuerping/.bun/bin/pi", &protocol.AgentSessionConfig{Cwd: "/tmp"}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	t.Logf("start process at %v", time.Now().Format("15:04:05.000"))
	start := time.Now()
	err := sess.startProcessLocked(runCtx, "hi")
	t.Logf("startProcessLocked took %v, err=%v", time.Since(start), err)

	if err != nil {
		t.Fatalf("startProcessLocked failed: %v", err)
	}

	stdoutPipe := sess.stdoutPipe

	// Read manually to see what happens
	scanner := bufio.NewScanner(stdoutPipe)
	start = time.Now()
	count := 0
	for scanner.Scan() {
		count++
	}
	t.Logf("manual scan took %v, lines=%d, err=%v", time.Since(start), count, scanner.Err())
}
