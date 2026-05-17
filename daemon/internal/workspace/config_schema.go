package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectConfig represents the parsed solo.json config.
type ProjectConfig struct {
	Worktree *WorktreeLifecycleConfig `json:"worktree,omitempty"`
	Scripts  map[string]*ScriptConfig `json:"scripts,omitempty"`
}

// WorktreeLifecycleConfig contains lifecycle commands for worktrees.
type WorktreeLifecycleConfig struct {
	Setup    []string `json:"setup,omitempty"`
	Teardown []string `json:"teardown,omitempty"`
}

// ScriptConfig represents a script entry in the config.
type ScriptConfig struct {
	Type    string `json:"type,omitempty"` // "service" for service scripts
	Command string `json:"command,omitempty"`
	Port    *int   `json:"port,omitempty"`
}

type ProjectConfigRevision struct {
	MtimeMs float64 `json:"mtimeMs"`
	Size    int64   `json:"size"`
}

type RawProjectConfigReadResult struct {
	Config   map[string]interface{}
	Revision *ProjectConfigRevision
}

type RawProjectConfigWriteResult struct {
	Config   map[string]interface{}
	Revision *ProjectConfigRevision
}

type StaleProjectConfigError struct {
	CurrentRevision *ProjectConfigRevision
}

func (e *StaleProjectConfigError) Error() string {
	return "stale project config"
}

// ReadProjectConfig reads solo.json from the repo root.
func ReadProjectConfig(repoRoot string) (*ProjectConfig, error) {
	cfg, err := readConfigFile(filepath.Join(repoRoot, "solo.json"))
	if err == nil && cfg != nil {
		return cfg, nil
	}
	return nil, nil
}

func ReadRawSoloConfigForEdit(repoRoot string) (*RawProjectConfigReadResult, error) {
	configPath := filepath.Join(repoRoot, "solo.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &RawProjectConfigReadResult{Config: nil, Revision: nil}, nil
		}
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	revision, err := statProjectConfig(configPath)
	if err != nil {
		return nil, err
	}
	return &RawProjectConfigReadResult{Config: config, Revision: revision}, nil
}

func WriteRawSoloConfigForEdit(repoRoot string, config map[string]interface{}, expectedRevision *ProjectConfigRevision) (*RawProjectConfigWriteResult, error) {
	configPath := filepath.Join(repoRoot, "solo.json")
	if config == nil {
		config = map[string]interface{}{}
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	currentRevision, err := statProjectConfig(configPath)
	if err != nil {
		return nil, err
	}
	if !projectConfigRevisionsEqual(currentRevision, expectedRevision) {
		return nil, &StaleProjectConfigError{CurrentRevision: currentRevision}
	}

	tempPath, err := os.CreateTemp(repoRoot, ".solo.json.*.tmp")
	if err != nil {
		return nil, err
	}
	tempName := tempPath.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := tempPath.Write(data); err != nil {
		_ = tempPath.Close()
		return nil, err
	}
	if err := tempPath.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tempName, configPath); err != nil {
		return nil, err
	}
	cleanupTemp = false

	revision, err := statProjectConfig(configPath)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, fmt.Errorf("project config revision missing after write")
	}
	return &RawProjectConfigWriteResult{Config: config, Revision: revision}, nil
}

func statProjectConfig(configPath string) (*ProjectConfigRevision, error) {
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &ProjectConfigRevision{
		MtimeMs: float64(info.ModTime().UnixNano()) / 1e6,
		Size:    info.Size(),
	}, nil
}

func projectConfigRevisionsEqual(left, right *ProjectConfigRevision) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.MtimeMs == right.MtimeMs && left.Size == right.Size
}

func readConfigFile(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse into raw map to handle flexible types
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cfg := &ProjectConfig{}

	// Parse worktree section
	if wtRaw, ok := raw["worktree"]; ok {
		wt, err := parseWorktreeConfig(wtRaw)
		if err != nil {
			return nil, err
		}
		cfg.Worktree = wt
	}

	// Parse scripts section
	if scriptsRaw, ok := raw["scripts"]; ok {
		var scripts map[string]*ScriptConfig
		if err := json.Unmarshal(scriptsRaw, &scripts); err != nil {
			return nil, err
		}
		cfg.Scripts = scripts
	}

	return cfg, nil
}

func parseWorktreeConfig(raw json.RawMessage) (*WorktreeLifecycleConfig, error) {
	var wt struct {
		Setup    json.RawMessage `json:"setup"`
		Teardown json.RawMessage `json:"teardown"`
	}
	if err := json.Unmarshal(raw, &wt); err != nil {
		return nil, err
	}

	result := &WorktreeLifecycleConfig{}
	result.Setup = normalizeLifecycleCommands(wt.Setup)
	result.Teardown = normalizeLifecycleCommands(wt.Teardown)
	return result, nil
}

// normalizeLifecycleCommands normalizes a string or string[] to string[].
func normalizeLifecycleCommands(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s != "" {
			return []string{s}
		}
		return nil
	}

	// Try string[]
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}

	return nil
}
