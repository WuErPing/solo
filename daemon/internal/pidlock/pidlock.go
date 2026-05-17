package pidlock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Acquire tries to write a PID file. Returns a release function on success.
func Acquire(soloHome string) (func(), error) {
	if err := os.MkdirAll(soloHome, 0755); err != nil {
		return nil, fmt.Errorf("cannot create solo home: %w", err)
	}

	pidPath := filepath.Join(soloHome, "solo.pid")

	// Check for existing PID file
	if data, err := os.ReadFile(pidPath); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Check if the process is still running
			if p, err := os.FindProcess(pid); err == nil {
				if err := p.Signal(syscall.Signal(0)); err == nil {
					return nil, fmt.Errorf("another solo daemon is already running (pid %d)", pid)
				}
			}
			// Stale PID file, remove it
			os.Remove(pidPath)
		}
	}

	// Write our PID
	pid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return nil, fmt.Errorf("cannot write PID file: %w", err)
	}

	release := func() {
		os.Remove(pidPath)
	}
	return release, nil
}
