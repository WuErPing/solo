package base

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ProcessManager handles subprocess lifecycle for stdio-based providers.
type ProcessManager struct {
	binaryPath string
	logger     *slog.Logger
}

// NewProcessManager creates a new process manager.
func NewProcessManager(binaryPath string, logger *slog.Logger) *ProcessManager {
	return &ProcessManager{
		binaryPath: binaryPath,
		logger:     logger,
	}
}

// Start launches a subprocess with the given arguments and working directory.
// Returns stdout, stderr, stdin, and the command.
//
// IMPORTANT: The caller MUST call cmd.Wait() when the process exits to reap
// it and prevent zombie/defunct processes. Use WaitForExit() for a convenient
// helper.
func (pm *ProcessManager) Start(ctx context.Context, args []string, cwd string, env []string) (io.ReadCloser, io.ReadCloser, io.WriteCloser, *exec.Cmd, error) {
	if pm.binaryPath == "" {
		return nil, nil, nil, nil, fmt.Errorf("binary path is empty")
	}

	cmd := exec.CommandContext(ctx, pm.binaryPath, args...)
	cmd.Dir = cwd
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("start process: %w", err)
	}

	return stdout, stderr, stdin, cmd, nil
}

// WaitForExit waits for a process to exit and returns its exit code.
// Returns (exitCode, error) where error is nil for clean exit (code 0).
func (pm *ProcessManager) WaitForExit(cmd *exec.Cmd) (int, error) {
	if cmd == nil {
		return -1, fmt.Errorf("nil cmd")
	}
	err := cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), err
	}
	return -1, err
}

// Stop gracefully stops the process with a timeout.
// Sends SIGTERM first, waits for graceful exit, then SIGKILL if needed.
func (pm *ProcessManager) Stop(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Try SIGTERM first for graceful shutdown
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails (e.g., process already exited), try SIGKILL
		_ = cmd.Process.Kill()
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		// Timeout: force kill
		_ = cmd.Process.Kill()
		// Wait again after kill
		select {
		case <-done:
			return nil
		case <-time.After(2 * time.Second):
			return fmt.Errorf("process did not exit after SIGKILL")
		}
	}
}

// Interrupt sends an interrupt signal to the process.
func (pm *ProcessManager) Interrupt(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(os.Interrupt)
}

// Kill forcefully kills the process and waits for it to exit.
func (pm *ProcessManager) Kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	// Wait for process to actually exit to avoid zombies
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("process did not exit after SIGKILL")
	}
}

// DrainStderr reads stderr and logs it at debug level.
func (pm *ProcessManager) DrainStderr(stderr io.ReadCloser) {
	if stderr == nil {
		return
	}
	defer func() { _ = stderr.Close() }()

	buf := make([]byte, 1024)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			pm.logger.Debug("stderr", "output", string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

// FindBinary locates a binary using environment variable, common paths, and PATH.
func FindBinary(name string, envVar string, commonPaths []string) (string, error) {
	// Check environment variable first
	if p := os.Getenv(envVar); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Check common locations
	home, _ := os.UserHomeDir()
	for _, p := range commonPaths {
		if home != "" {
			p = os.ExpandEnv(p)
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Fall back to PATH lookup
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s binary not found in PATH or common locations", name)
	}
	return p, nil
}
