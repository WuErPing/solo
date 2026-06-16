package agent

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

func TestPiIntegration_RawPump(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pm := base.NewProcessManager("pi", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-p", "hi", "--mode", "json", "--work-dir", "/tmp"}, "/tmp", os.Environ())
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer stdout.Close()
	defer stderr.Close()
	stdin.Close()

	go pm.DrainStderr(stderr)

	// Monitor process exit
	go func() {
		cmd.Wait()
		t.Logf("process exited with code %d", cmd.ProcessState.ExitCode())
	}()

	scanner := bufio.NewScanner(stdout)
	start := time.Now()
	count := 0
	for scanner.Scan() {
		count++
		if count <= 3 {
			t.Logf("line %d: %s", count, scanner.Text())
		}
	}
	elapsed := time.Since(start)
	t.Logf("scanner done: lines=%d elapsed=%v err=%v", count, elapsed, scanner.Err())

	if elapsed > 2*time.Second {
		t.Fatalf("scanner took too long: %v", elapsed)
	}
}
