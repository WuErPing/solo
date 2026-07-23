package base

import "errors"

// ErrProviderCrashed indicates the provider subprocess ended the event stream
// without reaching a terminal state (crashed or exited unexpectedly mid-turn).
// Defined here (not in the agent package) so the event pump can wrap it
// without an import cycle; agent.ErrProviderCrashed aliases this value.
var ErrProviderCrashed = errors.New("provider process crashed")
