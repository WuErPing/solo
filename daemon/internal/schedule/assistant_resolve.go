package schedule

import (
	"github.com/WuErPing/solo/daemon/internal/config"
)

// resolvedLLMEndpoint describes the default LLM provider endpoint resolved
// from config.LLMProviders (Settings → General → LLM Providers).
type resolvedLLMEndpoint struct {
	providerID string
	label      string
	baseURL    string
	apiKey     string
	model      string
}

// resolveDefaultLLMEndpoint picks the default LLM provider + model:
// candidates are providers with Enabled != false, in array order (the settings
// list order is the user's priority); the first candidate with a non-empty
// baseURL and apiKey wins. The model is the provider's isDefault model, else
// its first model. ok is false when nothing resolvable is configured.
func resolveDefaultLLMEndpoint(providers []config.LLMProviderConfig) (resolved resolvedLLMEndpoint, ok bool) {
	for _, p := range providers {
		if p.Enabled != nil && !*p.Enabled {
			continue
		}
		if p.BaseURL == "" || p.APIKey == "" {
			continue
		}
		model := ""
		for _, m := range p.Models {
			if m.IsDefault != nil && *m.IsDefault {
				model = m.ID
				break
			}
		}
		if model == "" && len(p.Models) > 0 {
			model = p.Models[0].ID
		}
		if model == "" {
			continue
		}
		return resolvedLLMEndpoint{
			providerID: p.ID,
			label:      p.Label,
			baseURL:    p.BaseURL,
			apiKey:     p.APIKey,
			model:      model,
		}, true
	}
	return resolvedLLMEndpoint{}, false
}
