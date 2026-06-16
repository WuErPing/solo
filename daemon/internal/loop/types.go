package loop

import "errors"

// ErrNoProviderAvailable is returned when a loop cannot resolve a provider.
var ErrNoProviderAvailable = errors.New("no provider available")

// Status represents the lifecycle state of a loop.
type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
)

// UpdateInput contains the mutable fields for Update.
type UpdateInput struct {
	Name    *string
	Archive *bool
}

// VerifyResult is the parsed outcome of a verifier prompt.
type VerifyResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}
