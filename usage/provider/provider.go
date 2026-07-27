package provider

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type Quota struct {
	Name    string     `json:"name"`
	Label   string     `json:"label"`
	Used    *float64   `json:"used,omitempty"`
	Limit   *float64   `json:"limit,omitempty"`
	UsedPct *float64   `json:"usedPct,omitempty"`
	Unit    string     `json:"unit,omitempty"`
	ResetAt *time.Time `json:"resetAt,omitempty"`
	ResetIn string     `json:"resetIn,omitempty"`
}

type Plan struct {
	Name string `json:"name"`
	Tier string `json:"tier,omitempty"`
}

type Snapshot struct {
	Provider  string    `json:"provider"`
	Plan      *Plan     `json:"plan,omitempty"`
	Quotas    []Quota   `json:"quotas"`
	FetchedAt time.Time `json:"fetchedAt"`
}

type Provider interface {
	Name() string
	Fetch(ctx context.Context) (*Snapshot, error)
}

type Config struct {
	APIKey   string            `json:"apiKey"`
	Endpoint string            `json:"endpoint,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

type Factory func(cfg Config) (Provider, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) {
	registry[name] = f
}

func Create(name string, cfg Config) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return f(cfg)
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
