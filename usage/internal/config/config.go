package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/WuErPing/solo/usage/internal/provider"
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

var envPattern = regexp.MustCompile(`\$\{(\w+)\}`)

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
	f.expandEnv()
	return &f, nil
}

func (f *File) expandEnv() {
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
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := envPattern.FindStringSubmatch(match)[1]
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		return match
	})
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
