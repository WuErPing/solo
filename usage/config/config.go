package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WuErPing/solo/usage/provider"
)

type File struct {
	Providers map[string]ProviderEntry `json:"providers"`
}

type ProviderEntry struct {
	Enabled  bool              `json:"enabled"`
	APIKey   string            `json:"apiKey"`
	Endpoint string            `json:"endpoint,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// varPattern matches ${...} placeholders in config values.
// Supported forms:
//   - ${VAR}        → environment variable VAR (kept as-is when unset)
//   - ${file:/path} → contents of the file (leading ~ supported, trimmed;
//     kept as-is when unreadable). Useful for rotating secrets like session
//     cookies: pbpaste > ~/.solo/xiaomimimo.cookie, no daemon restart needed.
var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".solo", "usage.json")
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	f.expandVars()
	return &f, nil
}

func (f *File) expandVars() {
	for name, entry := range f.Providers {
		entry.APIKey = expandVar(entry.APIKey)
		entry.Endpoint = expandVar(entry.Endpoint)
		for k, v := range entry.Extra {
			entry.Extra[k] = expandVar(v)
		}
		f.Providers[name] = entry
	}
}

func expandVar(s string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		body := varPattern.FindStringSubmatch(match)[1]
		if path, ok := strings.CutPrefix(body, "file:"); ok {
			if v, err := readSecretFile(path); err == nil {
				return v
			}
			return match
		}
		if v, ok := os.LookupEnv(body); ok {
			return v
		}
		return match
	})
}

func readSecretFile(path string) (string, error) {
	if rest, ok := strings.CutPrefix(path, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home + rest
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (f *File) ToProviderConfig(name string) (provider.Config, bool) {
	entry, ok := f.Providers[name]
	if !ok || !entry.Enabled {
		return provider.Config{}, false
	}
	return provider.Config{
		APIKey:   entry.APIKey,
		Endpoint: entry.Endpoint,
		Extra:    entry.Extra,
	}, true
}

func (f *File) EnabledProviders() []string {
	var names []string
	for name, entry := range f.Providers {
		if entry.Enabled {
			names = append(names, name)
		}
	}
	return names
}
