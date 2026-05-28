package workspace

import (
	"os/exec"
	"sync"
)

// GitCommander abstracts raw git command execution so that functions in this
// package can be tested without a real git repository on disk.
type GitCommander interface {
	// Run executes a git command in dir and returns the exit error.
	Run(dir string, args ...string) error
	// Output executes a git command in dir and returns stdout.
	Output(dir string, args ...string) (string, error)
}

var (
	gitCmdMu  sync.RWMutex
	gitCmdVar GitCommander = &defaultGitCommander{}
)

// getGitCmd returns the current GitCommander under a read lock.
func getGitCmd() GitCommander {
	gitCmdMu.RLock()
	defer gitCmdMu.RUnlock()
	return gitCmdVar
}

// setGitCmd replaces the current GitCommander under a write lock.
func setGitCmd(c GitCommander) {
	gitCmdMu.Lock()
	defer gitCmdMu.Unlock()
	gitCmdVar = c
}

// defaultGitCommander is the production implementation backed by exec.Command.
type defaultGitCommander struct{}

func (defaultGitCommander) Run(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (defaultGitCommander) Output(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

