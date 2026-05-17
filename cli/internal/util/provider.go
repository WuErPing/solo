package util

import (
	"fmt"
	"strings"

	"github.com/WuErPing/solo/protocol"
)

// ResolvedProviderModel holds the resolved provider and optional model.
type ResolvedProviderModel struct {
	Provider string
	Model    string // empty string means use default
}

// ResolveProviderModel resolves --provider and --model flags.
// Supports --provider <provider>/<model> slash notation.
func ResolveProviderModel(providerFlag, modelFlag string, snapshot *protocol.ProvidersSnapshotPayload) (*ResolvedProviderModel, error) {
	providerInput := strings.TrimSpace(providerFlag)
	modelInput := strings.TrimSpace(modelFlag)

	if providerInput == "" {
		return nil, &ProviderModelError{
			Code:    "MISSING_PROVIDER",
			Message: "Provider is required",
			Details: "Pass --provider <provider> or --provider <provider>/<model>. Use `solo provider ls` to see providers.",
		}
	}

	if modelFlag != "" && modelInput == "" {
		return nil, &ProviderModelError{
			Code:    "INVALID_MODEL",
			Message: "--model cannot be empty",
		}
	}

	// Check for slash notation: provider/model
	if slashIdx := strings.Index(providerInput, "/"); slashIdx != -1 {
		provider := strings.TrimSpace(providerInput[:slashIdx])
		model := strings.TrimSpace(providerInput[slashIdx+1:])
		if provider == "" || model == "" {
			return nil, &ProviderModelError{
				Code:    "INVALID_PROVIDER",
				Message: "Invalid --provider value",
				Details: "Use --provider <provider> or --provider <provider>/<model>",
			}
		}
		if modelInput != "" && modelInput != model {
			return nil, &ProviderModelError{
				Code:    "CONFLICTING_MODEL",
				Message: "Conflicting model values",
				Details: fmt.Sprintf("--provider specifies model %s, but --model specifies %s", model, modelInput),
			}
		}
		return &ResolvedProviderModel{Provider: provider, Model: model}, nil
	}

	return &ResolvedProviderModel{Provider: providerInput, Model: modelInput}, nil
}

// ProviderModelError is an error from provider/model resolution.
type ProviderModelError struct {
	Code    string
	Message string
	Details string
}

func (e *ProviderModelError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s\n%s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
