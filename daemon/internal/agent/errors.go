package agent

import (
	"errors"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

// Sentinel errors for provider-level failures.
// These enable upstream code (e.g. manager.go) to use errors.Is() for
// typed error handling and provider-level retry policies.
var (
	// ErrProviderCrashed indicates the provider subprocess crashed or exited
	// unexpectedly during a turn. Canonical value lives in the base package
	// (where the event pump wraps it); this alias keeps errors.Is working for
	// callers that import either package.
	ErrProviderCrashed = base.ErrProviderCrashed

	// ErrProviderTimeout indicates a provider operation exceeded its deadline.
	ErrProviderTimeout = errors.New("provider operation timed out")

	// ErrProviderStreaming indicates the provider's event stream was
	// interrupted before reaching a terminal state.
	ErrProviderStreaming = errors.New("provider streaming interrupted")

	// ErrProviderUnavailable indicates the provider binary or service is
	// not available (not installed, not authenticated, server not running).
	ErrProviderUnavailable = errors.New("provider unavailable")
)
