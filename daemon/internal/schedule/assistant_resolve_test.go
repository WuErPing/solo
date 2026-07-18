package schedule

import (
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
)

func TestResolveDefaultLLMEndpoint(t *testing.T) {
	enabled := func(b bool) *bool { return &b }

	tests := []struct {
		name      string
		providers []config.LLMProviderConfig
		wantOK    bool
		wantID    string
		wantLabel string
		wantBase  string
		wantKey   string
		wantModel string
	}{
		{
			name:      "no providers",
			providers: nil,
			wantOK:    false,
		},
		{
			name: "first enabled provider wins by array order",
			providers: []config.LLMProviderConfig{
				{ID: "p1", Label: "First", BaseURL: "https://a.example/v1", APIKey: "k1", Models: []config.LLMModelConfig{{ID: "m1"}}},
				{ID: "p2", Label: "Second", BaseURL: "https://b.example/v1", APIKey: "k2", Models: []config.LLMModelConfig{{ID: "m2"}}},
			},
			wantOK: true, wantID: "p1", wantLabel: "First", wantBase: "https://a.example/v1", wantKey: "k1", wantModel: "m1",
		},
		{
			name: "nil Enabled treated as enabled",
			providers: []config.LLMProviderConfig{
				{ID: "p1", Enabled: nil, BaseURL: "https://a.example/v1", APIKey: "k1", Models: []config.LLMModelConfig{{ID: "m1"}}},
			},
			wantOK: true, wantID: "p1", wantModel: "m1",
		},
		{
			name: "disabled provider skipped",
			providers: []config.LLMProviderConfig{
				{ID: "p1", Enabled: enabled(false), BaseURL: "https://a.example/v1", APIKey: "k1", Models: []config.LLMModelConfig{{ID: "m1"}}},
				{ID: "p2", Enabled: enabled(true), BaseURL: "https://b.example/v1", APIKey: "k2", Models: []config.LLMModelConfig{{ID: "m2"}}},
			},
			wantOK: true, wantID: "p2", wantBase: "https://b.example/v1", wantKey: "k2", wantModel: "m2",
		},
		{
			name: "empty baseURL skipped",
			providers: []config.LLMProviderConfig{
				{ID: "p1", BaseURL: "", APIKey: "k1", Models: []config.LLMModelConfig{{ID: "m1"}}},
				{ID: "p2", BaseURL: "https://b.example/v1", APIKey: "k2", Models: []config.LLMModelConfig{{ID: "m2"}}},
			},
			wantOK: true, wantID: "p2", wantModel: "m2",
		},
		{
			name: "empty apiKey skipped",
			providers: []config.LLMProviderConfig{
				{ID: "p1", BaseURL: "https://a.example/v1", APIKey: "", Models: []config.LLMModelConfig{{ID: "m1"}}},
				{ID: "p2", BaseURL: "https://b.example/v1", APIKey: "k2", Models: []config.LLMModelConfig{{ID: "m2"}}},
			},
			wantOK: true, wantID: "p2", wantModel: "m2",
		},
		{
			name: "isDefault model preferred over first model",
			providers: []config.LLMProviderConfig{
				{ID: "p1", BaseURL: "https://a.example/v1", APIKey: "k1", Models: []config.LLMModelConfig{
					{ID: "m1"},
					{ID: "m2", IsDefault: enabled(true)},
				}},
			},
			wantOK: true, wantModel: "m2",
		},
		{
			name: "explicit isDefault false falls back to first model",
			providers: []config.LLMProviderConfig{
				{ID: "p1", BaseURL: "https://a.example/v1", APIKey: "k1", Models: []config.LLMModelConfig{
					{ID: "m1", IsDefault: enabled(false)},
					{ID: "m2"},
				}},
			},
			wantOK: true, wantModel: "m1",
		},
		{
			name: "empty models not ok",
			providers: []config.LLMProviderConfig{
				{ID: "p1", BaseURL: "https://a.example/v1", APIKey: "k1"},
			},
			wantOK: false,
		},
		{
			name: "provider without models skipped for one with models",
			providers: []config.LLMProviderConfig{
				{ID: "p1", BaseURL: "https://a.example/v1", APIKey: "k1"},
				{ID: "p2", BaseURL: "https://b.example/v1", APIKey: "k2", Models: []config.LLMModelConfig{{ID: "m2"}}},
			},
			wantOK: true, wantID: "p2", wantModel: "m2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveDefaultLLMEndpoint(tt.providers)
			if ok != tt.wantOK {
				t.Fatalf("resolveDefaultLLMEndpoint() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if tt.wantID != "" && got.providerID != tt.wantID {
				t.Errorf("providerID = %q, want %q", got.providerID, tt.wantID)
			}
			if tt.wantLabel != "" && got.label != tt.wantLabel {
				t.Errorf("label = %q, want %q", got.label, tt.wantLabel)
			}
			if tt.wantBase != "" && got.baseURL != tt.wantBase {
				t.Errorf("baseURL = %q, want %q", got.baseURL, tt.wantBase)
			}
			if tt.wantKey != "" && got.apiKey != tt.wantKey {
				t.Errorf("apiKey = %q, want %q", got.apiKey, tt.wantKey)
			}
			if tt.wantModel != "" && got.model != tt.wantModel {
				t.Errorf("model = %q, want %q", got.model, tt.wantModel)
			}
		})
	}
}
