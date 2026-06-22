package protocol

// Protocol constants matching the TypeScript implementation.
const (
	WSProtocolVersion int = 2

	HelloTimeoutMs           = 15000
	SessionDisconnectGraceMs = 90000

	WSCloseHelloTimeout         = 4001
	WSCloseInvalidHello         = 4002
	WSCloseIncompatibleProtocol = 4003

	WSEndpoint           = "/ws"
	RelayProtocolVersion = "2"
)

// AgentStatus represents the possible lifecycle states of an agent.
// This is the canonical type used across all layers.
type AgentStatus string

const (
	AgentInitializing AgentStatus = "initializing"
	AgentIdle         AgentStatus = "idle"
	AgentRunning      AgentStatus = "running"
	AgentError        AgentStatus = "error"
	AgentClosed       AgentStatus = "closed"
)

// IsTerminal returns true for states that cannot transition to other states.
func (s AgentStatus) IsTerminal() bool {
	return s == AgentError || s == AgentClosed
}

// IsActive returns true for states where the agent is ready to perform work.
func (s AgentStatus) IsActive() bool {
	return s == AgentRunning || s == AgentIdle
}

// AgentLifecycleStatus is a backward-compatible alias for AgentStatus.
// Deprecated: use AgentStatus directly.
type AgentLifecycleStatus = AgentStatus

// ClientType identifies the type of client connecting.
type ClientType string

const (
	ClientMobile  ClientType = "mobile"
	ClientBrowser ClientType = "browser"
	ClientCLI     ClientType = "cli"
	ClientMCP     ClientType = "mcp"
)

// Provider availability constants (used as plain strings in ProviderSnapshotEntry).
const (
	ProviderReady       = "ready"
	ProviderLoading     = "loading"
	ProviderError       = "error"
	ProviderUnavailable = "unavailable"
)
