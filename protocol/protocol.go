package protocol

// Protocol constants matching the TypeScript implementation.
const (
	WSProtocolVersion int = 1

	HelloTimeoutMs           = 15000
	SessionDisconnectGraceMs = 90000

	WSCloseHelloTimeout         = 4001
	WSCloseInvalidHello         = 4002
	WSCloseIncompatibleProtocol = 4003

	WSEndpoint           = "/ws"
	RelayProtocolVersion = "2"
)

// AgentLifecycleStatus represents the possible lifecycle states of an agent.
type AgentLifecycleStatus string

const (
	AgentInitializing AgentLifecycleStatus = "initializing"
	AgentIdle         AgentLifecycleStatus = "idle"
	AgentRunning      AgentLifecycleStatus = "running"
	AgentError        AgentLifecycleStatus = "error"
	AgentClosed       AgentLifecycleStatus = "closed"
)

// ClientType identifies the type of client connecting.
type ClientType string

const (
	ClientMobile  ClientType = "mobile"
	ClientBrowser ClientType = "browser"
	ClientCLI     ClientType = "cli"
	ClientMCP     ClientType = "mcp"
)

// ProviderStatus represents the availability status of an agent provider.
type ProviderStatus string

const (
	ProviderReady       ProviderStatus = "ready"
	ProviderLoading     ProviderStatus = "loading"
	ProviderError       ProviderStatus = "error"
	ProviderUnavailable ProviderStatus = "unavailable"
)
