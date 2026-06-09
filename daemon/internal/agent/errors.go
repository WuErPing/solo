package agent

import "errors"

// Sentinel errors for provider-level failures.
// These enable upstream code (e.g. manager.go) to use errors.Is() for
// typed error handling and provider-level retry policies.
var (
	// ErrProviderCrashed indicates the provider subprocess crashed or exited
	// unexpectedly during a turn.
	ErrProviderCrashed = errors.New("provider process crashed")

	// ErrProviderTimeout indicates a provider operation exceeded its deadline.
	ErrProviderTimeout = errors.New("provider operation timed out")

	// ErrProviderStreaming indicates the provider's event stream was
	// interrupted before reaching a terminal state.
	ErrProviderStreaming = errors.New("provider streaming interrupted")

	// ErrProviderUnavailable indicates the provider binary or service is
	// not available (not installed, not authenticated, server not running).
	ErrProviderUnavailable = errors.New("provider unavailable")
)
