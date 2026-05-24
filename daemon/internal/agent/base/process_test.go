package base

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestProcessManager_Start_ProcessExitsImmediately_NoZombie verifies that
// when a process exits immediately, Wait() is called so it doesn't become
// a zombie. Before the fix, Start() never called Wait(), leaving defunct
// processes.
func TestProcessManager_Start_ProcessExitsImmediately_NoZombie(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	ctx := context.Background()
	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "exit 42"}, "", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdout.Close()
	defer stderr.Close()
	defer stdin.Close()

	// Wait for process to exit
	exitCode, waitErr := pm.WaitForExit(cmd)
	if waitErr == nil {
		t.Fatal("expected non-zero exit")
	}
	if exitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", exitCode)
	}

	// Verify process state is set
	if cmd.ProcessState == nil {
		t.Fatal("ProcessState should be set after Wait()")
	}
	if !cmd.ProcessState.Exited() {
		t.Fatal("process should be marked as exited")
	}
}

// TestProcessManager_Start_ReadsStderrOnFailure verifies stderr is captured
// when a process fails immediately.
func TestProcessManager_Start_ReadsStderrOnFailure(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pm := NewProcessManager("/bin/sh", logger)

	ctx := context.Background()
	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "echo 'fatal error' >&2; exit 1"}, "", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdout.Close()
	defer stdin.Close()

	// Drain stderr like production code does
	pm.DrainStderr(stderr)

	// Wait for process
	pm.WaitForExit(cmd)

	// Give DrainStderr time to process
	time.Sleep(100 * time.Millisecond)

	logs := logBuf.String()
	if !strings.Contains(logs, "fatal error") {
		t.Fatalf("expected stderr 'fatal error' in logs, got: %s", logs)
	}
}

// TestProcessManager_Start_LongRunning_CanBeStopped verifies a long-running
// process can be gracefully stopped.
func TestProcessManager_Start_LongRunning_CanBeStopped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	ctx := context.Background()
	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "sleep 30"}, "", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdout.Close()
	defer stderr.Close()
	defer stdin.Close()

	// Stop should send signal and wait
	err = pm.Stop(cmd, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// After Stop, the shell process should be gone (it may have orphaned
	// the sleep child, but the cmd itself should be reaped).
	// We verify by checking Stop returned without error and ProcessState
	// is set (meaning Wait() was called).
	if cmd.ProcessState == nil {
		t.Fatal("ProcessState should be set after Stop (Wait was called)")
	}
}

// TestProcessManager_Start_ContextCancellation_KillsProcess verifies that
// cancelling the context kills the process.
func TestProcessManager_Start_ContextCancellation_KillsProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "sleep 30"}, "", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdout.Close()
	defer stderr.Close()
	defer stdin.Close()

	cancel()

	// Wait should complete quickly after context cancellation
	done := make(chan struct{})
	go func() {
		pm.WaitForExit(cmd)
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForExit did not return after context cancellation")
	}
}

// TestProcessManager_Start_MultipleProcesses_NoZombies verifies multiple
// concurrent processes are all properly reaped.
func TestProcessManager_Start_MultipleProcesses_NoZombies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "echo hello"}, "", nil)
			if err != nil {
				t.Errorf("Start failed: %v", err)
				return
			}
			defer stdout.Close()
			defer stderr.Close()
			defer stdin.Close()

			// Read stdout to avoid blocking
			io.ReadAll(stdout)

			// Wait for process to exit
			if _, err := pm.WaitForExit(cmd); err != nil {
				// echo exits 0, so this should be nil
				t.Errorf("WaitForExit failed: %v", err)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent starts did not complete")
	}
}

// TestProcessManager_DrainStderr_CapturesAllLines verifies DrainStderr reads
// all stderr lines from a process.
func TestProcessManager_DrainStderr_CapturesAllLines(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	pm := NewProcessManager("/bin/sh", logger)

	ctx := context.Background()
	_, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "echo 'line 1' >&2; echo 'line 2' >&2"}, "", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdin.Close()

	// DrainStderr closes stderr when done
	pm.DrainStderr(stderr)

	// Wait for process to finish
	pm.WaitForExit(cmd)

	// Give DrainStderr time to process
	time.Sleep(100 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "line 1") {
		t.Errorf("expected 'line 1' in logs, got: %s", output)
	}
	if !strings.Contains(output, "line 2") {
		t.Errorf("expected 'line 2' in logs, got: %s", output)
	}
}

// TestProcessManager_Stop_NilCmd_NoPanic verifies Stop handles nil cmd
// gracefully.
func TestProcessManager_Stop_NilCmd_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	err := pm.Stop(nil, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Stop with nil cmd should return nil, got: %v", err)
	}
}

// TestProcessManager_Kill_NilCmd_NoPanic verifies Kill handles nil cmd
// gracefully.
func TestProcessManager_Kill_NilCmd_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	err := pm.Kill(nil)
	if err != nil {
		t.Fatalf("Kill with nil cmd should return nil, got: %v", err)
	}
}

// TestProcessManager_Interrupt_NilCmd_NoPanic verifies Interrupt handles nil
// cmd gracefully.
func TestProcessManager_Interrupt_NilCmd_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	err := pm.Interrupt(nil)
	if err != nil {
		t.Fatalf("Interrupt with nil cmd should return nil, got: %v", err)
	}
}

// TestProcessManager_WaitForExit_NilCmd_ReturnsError verifies WaitForExit
// handles nil cmd.
func TestProcessManager_WaitForExit_NilCmd_ReturnsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	exitCode, err := pm.WaitForExit(nil)
	if err == nil {
		t.Fatal("expected error for nil cmd")
	}
	if exitCode != -1 {
		t.Fatalf("expected exit code -1 for nil cmd, got %d", exitCode)
	}
}

// TestProcessManager_WaitForExit_CleanExit_ReturnsZero verifies WaitForExit
// returns 0 for successful process.
func TestProcessManager_WaitForExit_CleanExit_ReturnsZero(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pm := NewProcessManager("/bin/sh", logger)

	ctx := context.Background()
	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-c", "echo hello"}, "", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdout.Close()
	defer stderr.Close()
	defer stdin.Close()

	// Read stdout to completion
	io.ReadAll(stdout)

	exitCode, waitErr := pm.WaitForExit(cmd)
	if waitErr != nil {
		t.Fatalf("expected clean exit, got: %v", waitErr)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}
