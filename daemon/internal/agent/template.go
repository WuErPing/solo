package agent

import (
	"fmt"

	"github.com/WuErPing/solo/protocol"
)

// AgentTemplate resolves a protocol.AgentTemplate into a concrete
// AgentSessionConfig. A nil template returns a zero value config.
func AgentTemplate(t *protocol.AgentTemplate) protocol.AgentSessionConfig {
	if t == nil {
		return protocol.AgentSessionConfig{}
	}
	return *t
}

// loopAutonomyModePreference lists permission-mode IDs from most to least
// autonomous. A loop runs unattended with no one to answer permission prompts,
// so its worker defaults to the most autonomous mode the provider offers — the
// one that lets it edit files and run actions without approval. The IDs are
// matched against each provider's declared Modes (see BuiltinProviderDefinitions),
// so the resolved mode is always valid for that provider:
//
//	claude/kimi -> bypassPermissions, codex -> full-access, opencode -> build, pi -> default.
var loopAutonomyModePreference = []string{
	"bypassPermissions",
	"full-access",
	"build",
	"auto",
	"default",
}

// autonomousModeForProvider returns the most autonomous permission mode the
// given provider declares, or "" when the provider is unknown or declares no
// matching mode (in which case no mode is forced and the provider's own default
// applies).
func autonomousModeForProvider(provider string) string {
	for _, def := range BuiltinProviderDefinitions() {
		if def.ID != provider {
			continue
		}
		for _, pref := range loopAutonomyModePreference {
			for _, m := range def.Modes {
				if m.ID == pref {
					return pref
				}
			}
		}
		return ""
	}
	return ""
}

// LoopRecordToWorkerConfig resolves the worker AgentSessionConfig for a loop
// record. It prefers WorkerAgentTemplate, then AgentTemplate, then the legacy
// WorkerProvider/WorkerModel fields, then the legacy Provider/Model fields.
// The loop record's Cwd is always authoritative for the agent's working
// directory. When no permission mode is configured it defaults to the
// provider's most autonomous mode so the unattended worker can actually edit
// files and run actions.
func LoopRecordToWorkerConfig(r protocol.LoopRecord) protocol.AgentSessionConfig {
	var cfg protocol.AgentSessionConfig
	switch {
	case r.WorkerAgentTemplate != nil:
		cfg = AgentTemplate(r.WorkerAgentTemplate)
	case r.AgentTemplate != nil:
		cfg = AgentTemplate(r.AgentTemplate)
	default:
		provider := r.Provider
		model := r.Model
		if r.WorkerProvider != nil && *r.WorkerProvider != "" {
			provider = *r.WorkerProvider
		}
		if r.WorkerModel != nil && *r.WorkerModel != "" {
			model = r.WorkerModel
		}
		cfg = protocol.AgentSessionConfig{
			Provider: provider,
			Model:    model,
		}
	}
	cfg.Cwd = r.Cwd
	if cfg.ModeID == nil || *cfg.ModeID == "" {
		if mode := autonomousModeForProvider(cfg.Provider); mode != "" {
			cfg.ModeID = &mode
		}
	}
	return cfg
}

// LoopRecordToVerifierConfig resolves the verifier AgentSessionConfig for a
// loop record. It prefers VerifierAgentTemplate, then AgentTemplate, then the
// legacy VerifierProvider/VerifierModel fields, then the legacy Provider/Model
// fields. The loop record's Cwd is always authoritative for the agent's
// working directory.
func LoopRecordToVerifierConfig(r protocol.LoopRecord) protocol.AgentSessionConfig {
	var cfg protocol.AgentSessionConfig
	switch {
	case r.VerifierAgentTemplate != nil:
		cfg = AgentTemplate(r.VerifierAgentTemplate)
	case r.AgentTemplate != nil:
		cfg = AgentTemplate(r.AgentTemplate)
	default:
		provider := r.Provider
		model := r.Model
		if r.VerifierProvider != nil && *r.VerifierProvider != "" {
			provider = *r.VerifierProvider
		}
		if r.VerifierModel != nil && *r.VerifierModel != "" {
			model = r.VerifierModel
		}
		cfg = protocol.AgentSessionConfig{
			Provider: provider,
			Model:    model,
		}
	}
	cfg.Cwd = r.Cwd
	return cfg
}

// ScheduleTargetToConfig resolves a schedule target into an AgentSessionConfig.
// For "new-agent" targets it uses Config. For "provider" targets it uses
// ProviderID. For "agent" targets it returns an error because the caller must
// send a message to the existing agent rather than create a new one.
func ScheduleTargetToConfig(t protocol.ScheduleTarget, fallbackCwd string) (protocol.AgentSessionConfig, error) {
	switch t.Type {
	case "agent":
		return protocol.AgentSessionConfig{}, fmt.Errorf("agent target does not create a new config")
	case "new-agent":
		if t.Config == nil {
			return protocol.AgentSessionConfig{}, fmt.Errorf("new-agent target requires config")
		}
		cfg := AgentTemplate(t.Config)
		// Schedule-level Cwd, when present, overrides the template Cwd.
		if fallbackCwd != "" {
			cfg.Cwd = fallbackCwd
		}
		return cfg, nil
	case "provider":
		if t.ProviderID == "" {
			return protocol.AgentSessionConfig{}, fmt.Errorf("provider target requires providerId")
		}
		return protocol.AgentSessionConfig{
			Provider: t.ProviderID,
			Cwd:      fallbackCwd,
		}, nil
	default:
		return protocol.AgentSessionConfig{}, fmt.Errorf("unsupported schedule target type: %s", t.Type)
	}
}
