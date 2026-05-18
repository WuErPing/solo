package agent

import (
	"encoding/json"

	"github.com/WuErPing/solo/protocol"
)

type opencodeProvidersResponse struct {
	Connected []string `json:"connected"`
	All       []struct {
		ID     string                       `json:"id"`
		Name   string                       `json:"name"`
		Models map[string]opencodeModelInfo `json:"models"`
	} `json:"all"`
}

type opencodeModelInfo struct {
	Name         string                 `json:"name"`
	Family       json.RawMessage        `json:"family"`
	ReleaseDate  string                 `json:"release_date,omitempty"`
	Capabilities *opencodeCapabilities  `json:"capabilities,omitempty"`
	Cost         interface{}            `json:"cost,omitempty"`
	Limit        *opencodeModelLimit    `json:"limit,omitempty"`
	Variants     map[string]interface{} `json:"variants,omitempty"`
}

type opencodeCapabilities struct {
	Temperature bool `json:"temperature"`
	Reasoning   bool `json:"reasoning"`
	Attachment  bool `json:"attachment"`
	ToolCall    bool `json:"toolcall"`
}

type opencodeModelLimit struct {
	Context *float64 `json:"context,omitempty"`
	Input   *float64 `json:"input,omitempty"`
	Output  *float64 `json:"output,omitempty"`
}

func buildOpenCodeModelDefinition(providerID, providerName, modelID string, model opencodeModelInfo) protocol.AgentModelDefinition {
	rawVariants := make([]string, 0, len(model.Variants))
	for k := range model.Variants {
		rawVariants = append(rawVariants, k)
	}

	var thinkingOptions []protocol.AgentSelectOption
	for i, vID := range rawVariants {
		opt := protocol.AgentSelectOption{ID: vID, Label: vID, IsDefault: i == 0}
		thinkingOptions = append(thinkingOptions, opt)
	}

	var defaultThinking *string
	if len(thinkingOptions) > 0 {
		defaultThinking = &thinkingOptions[0].ID
	}

	def := protocol.AgentModelDefinition{
		Provider:                opencodeProviderName,
		ID:                      providerID + "/" + modelID,
		Label:                   model.Name,
		Description:             providerName + " - " + stringOrNil(model.Family),
		ThinkingOptions:         thinkingOptions,
		DefaultThinkingOptionID: derefString(defaultThinking),
	}

	// Build metadata
	metadata := map[string]interface{}{
		"providerId":   providerID,
		"providerName": providerName,
		"modelId":      modelID,
	}
	if model.Family != nil {
		metadata["family"] = stringOrNil(model.Family)
	}
	if model.ReleaseDate != "" {
		metadata["releaseDate"] = model.ReleaseDate
	}
	if model.Capabilities != nil {
		metadata["supportsAttachments"] = model.Capabilities.Attachment
		metadata["supportsReasoning"] = model.Capabilities.Reasoning
		metadata["supportsToolCall"] = model.Capabilities.ToolCall
	}
	if model.Cost != nil {
		metadata["cost"] = model.Cost
	}
	if model.Limit != nil {
		metadata["limit"] = model.Limit
		if model.Limit.Context != nil {
			metadata["contextWindowMaxTokens"] = int(*model.Limit.Context)
		}
	}
	def.Metadata = metadata

	return def
}

func extractModelContextWindow(model opencodeModelInfo) *int {
	if model.Limit != nil && model.Limit.Context != nil {
		v := int(*model.Limit.Context)
		return &v
	}
	return nil
}
