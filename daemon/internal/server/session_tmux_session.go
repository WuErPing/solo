package server

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// createTmuxSession creates a new detached tmux session.
func createTmuxSession(name string, workingDir *string, command *string) error {
	args := []string{"new-session", "-d", "-s", name}
	if workingDir != nil {
		args = append(args, "-c", *workingDir)
	}
	if command != nil {
		args = append(args, *command)
	}
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// killTmuxSession kills a tmux session by name.
func killTmuxSession(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func sendKeysToTmuxPane(paneID, keys string, sendEnter bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	if sendEnter {
		return exec.CommandContext(ctx, "tmux", "send-keys", "-t", paneID, keys, "Enter").Run()
	}
	return exec.CommandContext(ctx, "tmux", "send-keys", "-t", paneID, keys).Run()
}

func extractTmuxStatusLine(sessionID string) (string, string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()

	var left, right, center string
	var wg sync.WaitGroup

	// Fetch and expand status-left in its own goroutine (2 sequential tmux calls)
	wg.Go(func() {
		leftFmt, err := exec.CommandContext(ctx, "tmux", "show-options", "-gv", "-t", sessionID, "status-left").Output()
		if err != nil {
			return
		}
		fmt := strings.TrimSpace(string(leftFmt))
		if fmt == "" {
			return
		}
		out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", sessionID, fmt).Output()
		if err == nil {
			left = strings.TrimSpace(string(out))
		}
	})

	// Fetch and expand status-right in its own goroutine (2 sequential tmux calls)
	wg.Go(func() {
		rightFmt, err := exec.CommandContext(ctx, "tmux", "show-options", "-gv", "-t", sessionID, "status-right").Output()
		if err != nil {
			return
		}
		fmt := strings.TrimSpace(string(rightFmt))
		if fmt == "" {
			return
		}
		out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", sessionID, fmt).Output()
		if err == nil {
			right = strings.TrimSpace(string(out))
		}
	})

	// Window list is independent — run in parallel
	wg.Go(func() {
		center = extractWindowList(sessionID)
	})

	wg.Wait()
	return left, center, right, nil
}

func extractWindowList(sessionID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-t", sessionID, "-F", "#{window_index}:#{window_name}#{window_flags}").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.Join(lines, " ")
}
