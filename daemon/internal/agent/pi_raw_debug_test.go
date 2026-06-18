//go:build !short && !external_api

package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

func TestPiIntegration_RawDebugToolCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pm := base.NewProcessManager("pi", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-p", "list files in current dir", "--mode", "json"}, "/tmp", os.Environ())
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	stdin.Close()

	// Read stdout
	scanner := bufio.NewScanner(stdout)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if len(line) > 300 {
			line = line[:300] + "..."
		}
		fmt.Printf("%3d: %s\n", lineNum, line)
	}
	if err := scanner.Err(); err != nil {
		t.Logf("scanner error: %v", err)
	}
	fmt.Printf("total stdout lines: %d\n", lineNum)

	// Read stderr
	stderrBytes, _ := io.ReadAll(stderr)
	if len(stderrBytes) > 0 {
		fmt.Printf("stderr: %s\n", string(stderrBytes))
	}

	// Wait for exit
	if cmd.Process != nil {
		cmd.Wait()
		fmt.Printf("exit code: %d\n", cmd.ProcessState.ExitCode())
	}
}
