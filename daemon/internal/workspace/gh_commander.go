package workspace

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// GhCommander abstracts gh CLI execution so that GitHub-facing functions in
// this package can be tested without a real gh binary or network access.
type GhCommander interface {
	// Available reports whether the gh CLI is installed.
	Available() bool
	// Output executes a gh command in dir and returns stdout.
	Output(dir string, args ...string) (string, error)
}

var (
	ghCmdMu  sync.RWMutex
	ghCmdVar GhCommander = &defaultGhCommander{}
)

// getGhCmd returns the current GhCommander under a read lock.
func getGhCmd() GhCommander {
	ghCmdMu.RLock()
	defer ghCmdMu.RUnlock()
	return ghCmdVar
}

// setGhCmd replaces the current GhCommander under a write lock.
func setGhCmd(c GhCommander) {
	ghCmdMu.Lock()
	defer ghCmdMu.Unlock()
	ghCmdVar = c
}

// defaultGhCommander is the production implementation backed by exec.Command.
type defaultGhCommander struct{}

func (defaultGhCommander) Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (defaultGhCommander) Output(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), stderr)
			}
		}
		return "", err
	}
	return string(out), nil
}
