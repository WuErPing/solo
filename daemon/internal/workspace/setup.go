package workspace

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maxCommandOutputBytes = 64 * 1024 // 64KB per command

// SetupProgressEvent is emitted during setup command execution.
type SetupProgressEvent struct {
	WorkspaceID string
	Worktree    WorktreeConfig
	Status      string // "running" | "completed" | "failed"
	Commands    []SetupCommandSnapshot
	Error       *string
}

// SetupCommandSnapshot represents the state of a single setup command.
type SetupCommandSnapshot struct {
	Index      int    `json:"index"`
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	Log        string `json:"log"`
	Status     string `json:"status"` // "running" | "completed" | "failed"
	ExitCode   *int   `json:"exitCode"`
	DurationMs *int   `json:"durationMs,omitempty"`
}

// SetupProgressFunc is called to emit progress events.
type SetupProgressFunc func(event SetupProgressEvent)

// RunWorktreeSetup reads setup commands from solo.json/solo.json and executes them.
// Progress is streamed via the onProgress callback.
func RunWorktreeSetup(
	worktreePath string,
	worktree WorktreeConfig,
	workspaceID string,
	repoRoot string,
	onProgress SetupProgressFunc,
) error {
	cfg, err := ReadProjectConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("read project config: %w", err)
	}
	if cfg == nil || cfg.Worktree == nil || len(cfg.Worktree.Setup) == 0 {
		// No setup commands - emit completed immediately
		onProgress(SetupProgressEvent{
			WorkspaceID: workspaceID,
			Worktree:    worktree,
			Status:      "completed",
			Commands:    nil,
		})
		return nil
	}

	commands := cfg.Worktree.Setup
	snapshots := make([]SetupCommandSnapshot, len(commands))
	for i, cmd := range commands {
		snapshots[i] = SetupCommandSnapshot{
			Index:   i + 1,
			Command: cmd,
			Cwd:     worktreePath,
			Status:  "running",
		}
	}

	env := buildSetupEnv(repoRoot, worktreePath, worktree.BranchName)

	for i, cmd := range commands {
		// Emit running state
		snapshots[i].Status = "running"
		onProgress(SetupProgressEvent{
			WorkspaceID: workspaceID,
			Worktree:    worktree,
			Status:      "running",
			Commands:    copySnapshots(snapshots),
		})

		// Execute command
		start := time.Now()
		log, exitCode, execErr := execSetupCommand(cmd, worktreePath, env)
		duration := int(time.Since(start).Milliseconds())

		snapshots[i].Log = log
		snapshots[i].ExitCode = &exitCode
		snapshots[i].DurationMs = &duration

		if execErr != nil || exitCode != 0 {
			snapshots[i].Status = "failed"
			errMsg := fmt.Sprintf("setup command failed: %s (exit %d)", cmd, exitCode)
			if execErr != nil && exitCode == 0 {
				errMsg = fmt.Sprintf("setup command error: %s: %v", cmd, execErr)
			}
			onProgress(SetupProgressEvent{
				WorkspaceID: workspaceID,
				Worktree:    worktree,
				Status:      "failed",
				Commands:    copySnapshots(snapshots),
				Error:       &errMsg,
			})
			return fmt.Errorf("%s", errMsg)
		}

		snapshots[i].Status = "completed"
	}

	// Emit completed
	onProgress(SetupProgressEvent{
		WorkspaceID: workspaceID,
		Worktree:    worktree,
		Status:      "completed",
		Commands:    copySnapshots(snapshots),
	})
	return nil
}

// execSetupCommand runs a single setup command and captures its output.
func execSetupCommand(command, cwd string, env []string) (log string, exitCode int, err error) {
	cmd := exec.Command("/bin/bash", "-lc", command)
	cmd.Dir = cwd
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", -1, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", -1, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", -1, fmt.Errorf("start command: %w", err)
	}

	// Read combined output with truncation
	var builder strings.Builder
	combined := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(combined)
	for scanner.Scan() {
		line := scanner.Text()
		if builder.Len()+len(line)+1 > maxCommandOutputBytes {
			builder.WriteString("\n... [output truncated]\n")
			// Drain remaining to prevent pipe stall
			io.Copy(io.Discard, combined)
			break
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	waitErr := cmd.Wait()
	log = builder.String()

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return log, exitErr.ExitCode(), nil
		}
		return log, -1, waitErr
	}

	return log, 0, nil
}

// buildSetupEnv constructs environment variables for setup commands.
// Inherits the current process environment and appends Solo-compatible vars.
func buildSetupEnv(repoRoot, worktreePath, branchName string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"SOLO_SOURCE_CHECKOUT_PATH="+repoRoot,
		"SOLO_ROOT_PATH="+repoRoot,
		"SOLO_WORKTREE_PATH="+worktreePath,
		"SOLO_BRANCH_NAME="+branchName,
	)
	return env
}

func copySnapshots(src []SetupCommandSnapshot) []SetupCommandSnapshot {
	dst := make([]SetupCommandSnapshot, len(src))
	copy(dst, src)
	return dst
}
