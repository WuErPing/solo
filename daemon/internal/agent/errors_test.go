package agent

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrProviderCrashed_IsSelf(t *testing.T) {
	if !errors.Is(ErrProviderCrashed, ErrProviderCrashed) {
		t.Error("ErrProviderCrashed should match itself")
	}
}

func TestErrProviderTimeout_IsSelf(t *testing.T) {
	if !errors.Is(ErrProviderTimeout, ErrProviderTimeout) {
		t.Error("ErrProviderTimeout should match itself")
	}
}

func TestErrProviderStreaming_IsSelf(t *testing.T) {
	if !errors.Is(ErrProviderStreaming, ErrProviderStreaming) {
		t.Error("ErrProviderStreaming should match itself")
	}
}

func TestErrProviderUnavailable_IsSelf(t *testing.T) {
	if !errors.Is(ErrProviderUnavailable, ErrProviderUnavailable) {
		t.Error("ErrProviderUnavailable should match itself")
	}
}

func TestErrProviderCrashed_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("claude process died: %w", ErrProviderCrashed)
	if !errors.Is(wrapped, ErrProviderCrashed) {
		t.Error("wrapped ErrProviderCrashed should be detectable with errors.Is")
	}
}

func TestErrProviderTimeout_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("kimi turn took too long: %w", ErrProviderTimeout)
	if !errors.Is(wrapped, ErrProviderTimeout) {
		t.Error("wrapped ErrProviderTimeout should be detectable with errors.Is")
	}
}

func TestErrProviderStreaming_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("SSE connection dropped: %w", ErrProviderStreaming)
	if !errors.Is(wrapped, ErrProviderStreaming) {
		t.Error("wrapped ErrProviderStreaming should be detectable with errors.Is")
	}
}

func TestErrProviderUnavailable_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("opencode server not running: %w", ErrProviderUnavailable)
	if !errors.Is(wrapped, ErrProviderUnavailable) {
		t.Error("wrapped ErrProviderUnavailable should be detectable with errors.Is")
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// Ensure none of the errors accidentally alias each other.
	all := []error{
		ErrProviderCrashed,
		ErrProviderTimeout,
		ErrProviderStreaming,
		ErrProviderUnavailable,
	}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("%v should not match %v", a, b)
			}
		}
	}
}
