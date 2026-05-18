package protocol

// --- Common Sub-Types ---

// AgentCapabilityFlags describes what an agent provider supports.
type AgentCapabilityFlags struct {
	SupportsStreaming          bool `json:"supportsStreaming"`
	SupportsSessionPersistence bool `json:"supportsSessionPersistence"`
	SupportsDynamicModes       bool `json:"supportsDynamicModes"`
	SupportsMcpServers         bool `json:"supportsMcpServers"`
	SupportsReasoningStream    bool `json:"supportsReasoningStream"`
	SupportsToolInvocations    bool `json:"supportsToolInvocations"`
}

// AgentMode describes an available mode for an agent.
type AgentMode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	ColorTier   string `json:"colorTier,omitempty"`
}

// AgentSelectOption is a select option for agent features.
type AgentSelectOption struct {
	ID          string                 `json:"id"`
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	IsDefault   bool                   `json:"isDefault,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AgentModelDefinition describes a model available for a provider.
type AgentModelDefinition struct {
	Provider                string                 `json:"provider"`
	ID                      string                 `json:"id"`
	Label                   string                 `json:"label"`
	Description             string                 `json:"description,omitempty"`
	IsDefault               bool                   `json:"isDefault,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
	ThinkingOptions         []AgentSelectOption    `json:"thinkingOptions,omitempty"`
	DefaultThinkingOptionID string                 `json:"defaultThinkingOptionId,omitempty"`
}

// AgentUsage tracks token usage for an agent session.
type AgentUsage struct {
	InputTokens             *float64 `json:"inputTokens,omitempty"`
	CachedInputTokens       *float64 `json:"cachedInputTokens,omitempty"`
	OutputTokens            *float64 `json:"outputTokens,omitempty"`
	TotalCostUSD            *float64 `json:"totalCostUsd,omitempty"`
	ContextWindowMaxTokens  *float64 `json:"contextWindowMaxTokens,omitempty"`
	ContextWindowUsedTokens *float64 `json:"contextWindowUsedTokens,omitempty"`
}

// AgentFeatureToggle represents a toggle feature.
type AgentFeatureToggle struct {
	Type        string `json:"type"` // "toggle"
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Tooltip     string `json:"tooltip,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Value       bool   `json:"value"`
}

// AgentFeatureSelect represents a select feature.
type AgentFeatureSelect struct {
	Type        string              `json:"type"` // "select"
	ID          string              `json:"id"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	Tooltip     string              `json:"tooltip,omitempty"`
	Icon        string              `json:"icon,omitempty"`
	Value       *string             `json:"value"`
	Options     []AgentSelectOption `json:"options"`
}

// AgentFeature is either a toggle or select feature.
// The Type field discriminates between them.
type AgentFeature struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Tooltip     string `json:"tooltip,omitempty"`
	Icon        string `json:"icon,omitempty"`
	// Toggle fields
	Value bool `json:"value,omitempty"`
	// Select fields
	SelectValue *string             `json:"selectValue,omitempty"`
	Options     []AgentSelectOption `json:"options,omitempty"`
}

// AgentPersistenceHandle describes how an agent session can be persisted.
type AgentPersistenceHandle struct {
	Provider     string                 `json:"provider"`
	SessionID    string                 `json:"sessionId"`
	NativeHandle string                 `json:"nativeHandle,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AgentRuntimeInfo contains runtime information about an agent session.
type AgentRuntimeInfo struct {
	Provider         string                 `json:"provider"`
	SessionID        *string                `json:"sessionId"`
	Model            *string                `json:"model,omitempty"`
	ThinkingOptionID *string                `json:"thinkingOptionId,omitempty"`
	ModeID           *string                `json:"modeId,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

// McpServerConfig describes an MCP server configuration.
type McpServerConfig struct {
	Type    string            `json:"type"` // "stdio", "http", "sse"
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// AgentSessionConfig is the configuration for creating an agent session.
type AgentSessionConfig struct {
	Provider         string                     `json:"provider"`
	Cwd              string                     `json:"cwd"`
	ModeID           *string                    `json:"modeId,omitempty"`
	Model            *string                    `json:"model,omitempty"`
	ThinkingOptionID *string                    `json:"thinkingOptionId,omitempty"`
	FeatureValues    map[string]interface{}     `json:"featureValues,omitempty"`
	Title            *string                    `json:"title,omitempty"`
	ApprovalPolicy   string                     `json:"approvalPolicy,omitempty"`
	SandboxMode      string                     `json:"sandboxMode,omitempty"`
	NetworkAccess    bool                       `json:"networkAccess,omitempty"`
	WebSearch        bool                       `json:"webSearch,omitempty"`
	Extra            map[string]interface{}     `json:"extra,omitempty"`
	SystemPrompt     string                     `json:"systemPrompt,omitempty"`
	McpServers       map[string]McpServerConfig `json:"mcpServers,omitempty"`
	OutputSchema     map[string]interface{}     `json:"outputSchema,omitempty"`
}

// ImageAttachment represents an image sent to an agent.
type ImageAttachment struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

// AgentAttachment represents an attachment (GitHub PR/Issue) sent to an agent.
type AgentAttachment struct {
	Type        string  `json:"type"`
	MimeType    string  `json:"mimeType"`
	Number      int     `json:"number,omitempty"`
	Title       string  `json:"title,omitempty"`
	URL         string  `json:"url,omitempty"`
	Body        *string `json:"body,omitempty"`
	BaseRefName *string `json:"baseRefName,omitempty"`
	HeadRefName *string `json:"headRefName,omitempty"`
}

// GitSetupOptions describes git setup for a new agent.
type GitSetupOptions struct {
	BaseBranch      *string `json:"baseBranch,omitempty"`
	CreateNewBranch *bool   `json:"createNewBranch,omitempty"`
	NewBranchName   *string `json:"newBranchName,omitempty"`
	CreateWorktree  *bool   `json:"createWorktree,omitempty"`
	WorktreeSlug    *string `json:"worktreeSlug,omitempty"`
	RefName         *string `json:"refName,omitempty"`
	Action          *string `json:"action,omitempty"` // "branch-off" | "checkout"
	GithubPRNumber  *int    `json:"githubPrNumber,omitempty"`
}

// AgentPermissionAction describes an action on a permission request.
type AgentPermissionAction struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Behavior string `json:"behavior"` // "allow" | "deny"
	Variant  string `json:"variant,omitempty"`
	Intent   string `json:"intent,omitempty"`
}

// AgentPermissionResponse is the client's response to a permission request.
type AgentPermissionResponse struct {
	Behavior           string                   `json:"behavior"` // "allow" | "deny"
	SelectedActionID   string                   `json:"selectedActionId,omitempty"`
	UpdatedInput       map[string]interface{}   `json:"updatedInput,omitempty"`
	UpdatedPermissions []map[string]interface{} `json:"updatedPermissions,omitempty"`
	Message            string                   `json:"message,omitempty"`
	Interrupt          bool                     `json:"interrupt,omitempty"`
}

// ProjectCheckoutLitePayload describes the checkout associated with an agent
// directory entry. The shape mirrors Solo's ProjectCheckoutLitePayloadSchema.
type ProjectCheckoutLitePayload struct {
	Cwd                 string  `json:"cwd"`
	IsGit               bool    `json:"isGit"`
	CurrentBranch       *string `json:"currentBranch"`
	RemoteURL           *string `json:"remoteUrl"`
	WorktreeRoot        *string `json:"worktreeRoot,omitempty"`
	IsSoloOwnedWorktree bool    `json:"isSoloOwnedWorktree"`
	MainRepoRoot        *string `json:"mainRepoRoot"`
}

// ProjectPlacementPayload groups an agent under a project in the directory UI.
type ProjectPlacementPayload struct {
	ProjectKey  string                     `json:"projectKey"`
	ProjectName string                     `json:"projectName"`
	Checkout    ProjectCheckoutLitePayload `json:"checkout"`
}
