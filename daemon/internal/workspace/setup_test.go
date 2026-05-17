package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWorktreeSetup_NoCommands(t *testing.T) {
	dir := t.TempDir()
	var events []SetupProgressEvent
	onProgress := func(e SetupProgressEvent) {
		events = append(events, e)
	}

	err := RunWorktreeSetup(dir, WorktreeConfig{WorktreePath: dir, BranchName: "main"}, "ws1", dir, onProgress)
	if err != nil {
		t.Fatalf("RunWorktreeSetup: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Status != "completed" {
		t.Errorf("expected completed, got %q", events[0].Status)
	}
}

func TestRunWorktreeSetup_SuccessfulCommands(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	data := []byte(`{"worktree":{"setup":["echo hello","echo world"]}}`)
	_ = os.WriteFile(filepath.Join(repoRoot, "solo.json"), data, 0644)

	var events []SetupProgressEvent
	onProgress := func(e SetupProgressEvent) {
		events = append(events, e)
	}

	err := RunWorktreeSetup(dir, WorktreeConfig{WorktreePath: dir, BranchName: "main"}, "ws1", repoRoot, onProgress)
	if err != nil {
		t.Fatalf("RunWorktreeSetup: %v", err)
	}

	// Should have running + completed events
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.Status != "completed" {
		t.Errorf("expected final status completed, got %q", last.Status)
	}
	if len(last.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(last.Commands))
	}
}

func TestRunWorktreeSetup_FailingCommand(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	data := []byte(`{"worktree":{"setup":["exit 1"]}}`)
	_ = os.WriteFile(filepath.Join(repoRoot, "solo.json"), data, 0644)

	var events []SetupProgressEvent
	onProgress := func(e SetupProgressEvent) {
		events = append(events, e)
	}

	err := RunWorktreeSetup(dir, WorktreeConfig{WorktreePath: dir, BranchName: "main"}, "ws1", repoRoot, onProgress)
	if err == nil {
		t.Fatal("expected error for failing command")
	}

	last := events[len(events)-1]
	if last.Status != "failed" {
		t.Errorf("expected final status failed, got %q", last.Status)
	}
	if last.Error == nil {
		t.Error("expected error message in event")
	}
}

func TestBuildSetupEnv(t *testing.T) {
	env := buildSetupEnv("/repo", "/worktree", "feat-x")
	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	if envMap["SOLO_SOURCE_CHECKOUT_PATH"] != "/repo" {
		t.Errorf("SOLO_SOURCE_CHECKOUT_PATH: got %q, want %q", envMap["SOLO_SOURCE_CHECKOUT_PATH"], "/repo")
	}
	if envMap["SOLO_WORKTREE_PATH"] != "/worktree" {
		t.Errorf("SOLO_WORKTREE_PATH: got %q, want %q", envMap["SOLO_WORKTREE_PATH"], "/worktree")
	}
	if envMap["SOLO_BRANCH_NAME"] != "feat-x" {
		t.Errorf("SOLO_BRANCH_NAME: got %q, want %q", envMap["SOLO_BRANCH_NAME"], "feat-x")
	}
}

func TestExecSetupCommand_CaptureOutput(t *testing.T) {
	log, exitCode, err := execSetupCommand("echo hello", t.TempDir(), os.Environ())
	if err != nil {
		t.Fatalf("execSetupCommand: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(log, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", log)
	}
}

func TestExecSetupCommand_ExitCode(t *testing.T) {
	log, exitCode, err := execSetupCommand("exit 42", t.TempDir(), os.Environ())
	if err != nil {
		t.Fatalf("execSetupCommand: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
	_ = log
}
