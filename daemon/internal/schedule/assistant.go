package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/llm"
	"github.com/WuErPing/solo/protocol"
)

// AgentInfo is the read-only agent view the Assistant needs for the context
// block and agent reference resolution. The parse path never creates,
// modifies, or deletes agents.
type AgentInfo struct {
	ID       string
	Title    string
	Provider string
	Cwd      string
	Status   string
}

// llmClient is the chat-completion seam; *llm.Client satisfies it.
type llmClient interface {
	ChatCompletion(ctx context.Context, req llm.ChatRequest) (string, error)
}

const (
	assistMaxMessageChars    = 2000
	assistRateLimitPerMinute = 10
	assistRateWindow         = time.Minute
	assistCompletionTimeout  = 60 * time.Second
)

// AssistantConfig wires an Assistant through narrow seams so it stays
// testable without the agent/config/server packages.
type AssistantConfig struct {
	Store          *Store
	AgentsFn       func() []AgentInfo                // read-only agent listing
	LLMProvidersFn func() []config.LLMProviderConfig // fresh read per request
	LLMClient      llmClient
	Logger         *slog.Logger
}

// Assistant orchestrates one stateless NL → proposal parse per request.
// It NEVER calls Store mutation methods; the LLM only ever produces a
// validated proposal the user confirms through the existing schedule RPCs.
type Assistant struct {
	store          *Store
	agentsFn       func() []AgentInfo
	llmProvidersFn func() []config.LLMProviderConfig
	llmClient      llmClient
	logger         *slog.Logger

	limiterMu sync.Mutex
	calls     []time.Time // sliding-window timestamps of accepted calls

	inflight atomic.Bool // single in-flight parse per Assistant
}

func NewAssistant(cfg AssistantConfig) *Assistant {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Assistant{
		store:          cfg.Store,
		agentsFn:       cfg.AgentsFn,
		llmProvidersFn: cfg.LLMProvidersFn,
		llmClient:      cfg.LLMClient,
		logger:         logger,
	}
}

// Assist parses one natural-language schedule request into a validated
// payload. Domain failures are reported in the payload (Kind "error" /
// "clarify"), not as Go errors; a non-nil Go error means a programming or
// wiring bug (e.g. no LLM client configured).
func (a *Assistant) Assist(ctx context.Context, req protocol.ScheduleAssistRequest) (*protocol.ScheduleAssistResponsePayload, error) {
	payload := &protocol.ScheduleAssistResponsePayload{RequestID: req.RequestID}

	// 1. Guards: request shape, then rate limit, then single in-flight.
	if code, msg := checkAssistGuards(req); code != "" {
		setAssistPayloadError(payload, code, msg)
		return payload, nil
	}
	if !a.allowAssistCall() {
		setAssistPayloadError(payload, "rate_limited", "Too many assist requests — wait a moment and try again.")
		return payload, nil
	}
	if !a.inflight.CompareAndSwap(false, true) {
		setAssistPayloadError(payload, "rate_limited", "Another assist request is still in progress — wait for it to finish.")
		return payload, nil
	}
	defer a.inflight.Store(false)

	if a.llmClient == nil {
		return nil, fmt.Errorf("schedule assistant: no LLM client configured")
	}

	// 2. Resolve the default LLM provider (fresh per request so settings
	// changes take effect immediately).
	var providers []config.LLMProviderConfig
	if a.llmProvidersFn != nil {
		providers = a.llmProvidersFn()
	}
	ep, ok := resolveDefaultLLMEndpoint(providers)
	if !ok {
		setAssistPayloadError(payload, "no_llm_provider",
			"No LLM provider is configured on this host. Add one with a default model in Settings → General → LLM Providers.")
		return payload, nil
	}
	payload.LLMProvider = ep.providerID
	payload.Model = ep.model
	label := ep.label
	if label == "" {
		label = ep.providerID
	}

	// 3. Build the prompt with fresh context.
	now := parseAssistClientNow(req.ClientNow)
	var agents []AgentInfo
	if a.agentsFn != nil {
		agents = a.agentsFn()
	}
	var schedules []protocol.ScheduleSummary
	if a.store != nil {
		schedules = a.store.List()
	}
	systemPrompt := assistSystemPrompt()
	userPrompt := buildAssistUserPrompt(assistPromptContext{
		now:        now,
		timezone:   req.Timezone,
		agents:     agents,
		schedules:  schedules,
		transcript: req.Transcript,
		message:    req.Message,
	})

	// 4-6. One-shot completion + extraction, with a single validation retry.
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			userPrompt = userPrompt + "\n\nYour previous output was invalid: " + lastErr.Error() +
				"\nOutput exactly one corrected JSON object."
		}
		callCtx, cancel := context.WithTimeout(ctx, assistCompletionTimeout)
		text, err := a.llmClient.ChatCompletion(callCtx, llm.ChatRequest{
			BaseURL:      ep.baseURL,
			APIKey:       ep.apiKey,
			Model:        ep.model,
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
		})
		cancel()
		if err != nil {
			mapAssistLLMError(payload, label, err)
			a.logger.Warn("schedule assist completion failed",
				"requestId", req.RequestID, "llmProvider", ep.providerID, "model", ep.model, "error", err)
			return payload, nil
		}

		intent, err := parseAssistIntent(text, a.store, agents, now)
		if err != nil {
			var refErr *assistRefError
			if errors.As(err, &refErr) {
				// Unknown/ambiguous reference → clarify, not retry.
				payload.Kind = "clarify"
				payload.Message = refErr.Error()
				a.logAssistDone(req.RequestID, ep, payload.Kind, attempt)
				return payload, nil
			}
			lastErr = err
			a.logger.Debug("schedule assist validation failed",
				"requestId", req.RequestID, "attempt", attempt, "error", err)
			continue
		}

		a.applyAssistIntent(payload, intent, now)
		a.logAssistDone(req.RequestID, ep, payload.Kind, attempt)
		return payload, nil
	}

	setAssistPayloadError(payload, "parse_failed",
		"Couldn't understand that as a schedule change. Try rephrasing, or use the form.")
	a.logger.Warn("schedule assist parse failed after retry",
		"requestId", req.RequestID, "llmProvider", ep.providerID, "model", ep.model, "error", lastErr)
	return payload, nil
}

func (a *Assistant) logAssistDone(requestID string, ep resolvedLLMEndpoint, kind string, attempt int) {
	a.logger.Info("schedule assist completed",
		"requestId", requestID, "llmProvider", ep.providerID, "model", ep.model,
		"kind", kind, "retries", attempt)
}

func checkAssistGuards(req protocol.ScheduleAssistRequest) (code, msg string) {
	n := utf8.RuneCountInString(req.Message)
	if n == 0 || n > assistMaxMessageChars {
		return "invalid_request", fmt.Sprintf("message must be 1-%d characters", assistMaxMessageChars)
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		return "invalid_request", fmt.Sprintf("invalid timezone %q", req.Timezone)
	}
	if len(req.Transcript) > maxAssistTranscriptTurns {
		return "invalid_request", fmt.Sprintf("transcript must be at most %d turns", maxAssistTranscriptTurns)
	}
	return "", ""
}

// allowAssistCall implements the 10/min sliding-window rate limit.
// Only accepted calls consume budget.
func (a *Assistant) allowAssistCall() bool {
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-assistRateWindow)
	kept := a.calls[:0]
	for _, ts := range a.calls {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	a.calls = kept
	if len(a.calls) >= assistRateLimitPerMinute {
		return false
	}
	a.calls = append(a.calls, now)
	return true
}

func mapAssistLLMError(payload *protocol.ScheduleAssistResponsePayload, label string, err error) {
	switch {
	case errors.Is(err, llm.ErrLLMAuth):
		setAssistPayloadError(payload, "llm_auth",
			fmt.Sprintf("Authentication failed for LLM provider %q — check the API key in Settings → General → LLM Providers.", label))
	case errors.Is(err, llm.ErrLLMRateLimited):
		setAssistPayloadError(payload, "rate_limited",
			fmt.Sprintf("LLM provider %q is rate limiting requests — wait a moment and try again.", label))
	default:
		setAssistPayloadError(payload, "llm_unreachable",
			fmt.Sprintf("Could not reach LLM provider %q — check the provider configuration in Settings → General → LLM Providers and try again.", label))
	}
}

func (a *Assistant) applyAssistIntent(payload *protocol.ScheduleAssistResponsePayload, intent *assistIntent, now time.Time) {
	payload.Kind = intent.Kind
	switch intent.Kind {
	case "clarify", "answer":
		payload.Message = intent.Message
	case "proposal":
		proposal := &protocol.ScheduleAssistProposal{
			Op:         intent.Op,
			ScheduleID: intent.ScheduleID,
			Name:       intent.Name,
			Prompt:     intent.Prompt,
			Cadence:    intent.Cadence,
			Target:     intent.Target,
			Cwd:        intent.Cwd,
			MaxRuns:    intent.MaxRuns,
			ExpiresAt:  intent.ExpiresAt,
			Summary:    intent.Summary,
			Warnings:   intent.Warnings,
		}
		// Lifecycle proposals need a name on the card; fill it from the store
		// when the LLM omitted it.
		if proposal.Name == "" && proposal.ScheduleID != "" && a.store != nil {
			if sched, ok := a.store.Get(proposal.ScheduleID); ok && sched.Name != nil {
				proposal.Name = *sched.Name
			}
		}
		if proposal.Cadence != nil {
			if next := NextRunAt(*proposal.Cadence, now); next != nil {
				s := next.Format(time.RFC3339)
				proposal.NextRunAt = &s
			}
		}
		payload.Proposal = proposal
	}
}

func setAssistPayloadError(payload *protocol.ScheduleAssistResponsePayload, code, message string) {
	payload.Kind = "error"
	payload.Error = &code
	payload.Message = message
}

func parseAssistClientNow(clientNow string) time.Time {
	if t, err := time.Parse(time.RFC3339, clientNow); err == nil {
		return t
	}
	return time.Now()
}
