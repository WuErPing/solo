// Package opencode implements the OpenCode SSE-based agent provider.
package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

const (
	opencodeProviderName               = "opencode"
	opencodeServerStartTimeout         = 30 * time.Second
	opencodeProviderListTimeout        = 30 * time.Second
	opencodeHTTPRequestTimeout         = 30 * time.Second
	opencodeServerShutdownTimeout      = 5 * time.Second
	opencodeMcpAddTimeout              = 10 * time.Second
	opencodeCommandListTimeout         = 30 * time.Second
	opencodeListCommandsAcquireTimeout = 30 * time.Second
	// SSE read idle timeout: if no SSE event is received within this window,
	// the connection is considered dead and the turn is failed. This prevents
	// consumeSSE from blocking indefinitely on a half-open TCP connection.
	opencodeSSEReadIdleTimeout = 120 * time.Second
)

var opencodeHeadersTimeoutTokens = []string{
	"headers timeout", "headers timeout error", "headers_timeout", "und_err_headers_timeout",
}

// --- OpenCode Agent Client ---

type Client struct {
	binaryPath          string
	logger              *slog.Logger
	runtimeSettings     *OpenCodeRuntimeSettings
	serverManager       *OpenCodeServerManager
	modelContextWindows map[string]int // "providerID/modelID" -> maxTokens
}

func NewClient(binaryPath string, logger *slog.Logger) *Client {
	return NewOpenCodeAgentClientWithSettings(binaryPath, logger, nil)
}

func NewOpenCodeAgentClientWithSettings(binaryPath string, logger *slog.Logger, settings *OpenCodeRuntimeSettings) *Client {
	if settings != nil && settings.BinaryPath != "" {
		binaryPath = settings.BinaryPath
	}
	if binaryPath == "" {
		if p, err := base.FindBinary("opencode", "OPENCODE_PATH", []string{
			"$HOME/.opencode/bin/opencode",
			"$HOME/.local/bin/opencode",
			"/usr/local/bin/opencode",
			"/opt/homebrew/bin/opencode",
		}); err == nil {
			binaryPath = p
		}
	}

	var extraEnv []string
	if settings != nil {
		extraEnv = settings.ExtraEnv
	}

	return &Client{
		binaryPath:      binaryPath,
		logger:          logger.With("provider", "opencode"),
		runtimeSettings: settings,
		serverManager:   GetOpenCodeServerManager(binaryPath, logger, extraEnv),
	}
}

func (c *Client) Provider() string { return opencodeProviderName }

func (c *Client) EnsureRunning(ctx context.Context) (string, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return "", err
	}
	return c.serverManager.EnsureRunning(ctx)
}

func (c *Client) IsAvailable(_ context.Context) error {
	if c.binaryPath == "" {
		return fmt.Errorf("opencode binary not found")
	}
	if _, err := os.Stat(c.binaryPath); err != nil {
		return fmt.Errorf("opencode binary not accessible: %w", err)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}

	server, release, err := c.serverManager.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("start opencode server: %w", err)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := opencodePost(ctx, server.url, "/session", config.Cwd, nil, &resp); err != nil {
		release()
		return nil, fmt.Errorf("create opencode session: %w", err)
	}

	return newOpenCodeSession(server.url, resp.ID, config, c.logger, release, c.modelContextWindows), nil
}

func (c *Client) ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (agent.AgentSession, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}

	server, release, err := c.serverManager.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	config := &protocol.AgentSessionConfig{
		Provider: opencodeProviderName,
	}
	var sessionID string

	if handle != nil {
		sessionID = handle.NativeHandle
		if sessionID == "" {
			sessionID = handle.SessionID
		}
		if v, ok := handle.Metadata["cwd"].(string); ok {
			config.Cwd = v
		}
		if v, ok := handle.Metadata["model"].(string); ok {
			config.Model = &v
		}
	}

	return newOpenCodeSession(server.url, sessionID, config, c.logger, release, c.modelContextWindows), nil
}

func (c *Client) ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}

	// Cap total time for acquire + HTTP call so slow server startup does not
	// block the session handler. If the opencode server is still cold-starting
	// and we time out, return an empty list rather than hanging.
	totalCtx, cancel := context.WithTimeout(ctx, opencodeListCommandsAcquireTimeout)
	defer cancel()

	server, release, err := c.serverManager.Acquire(totalCtx)
	if err != nil {
		if totalCtx.Err() == context.DeadlineExceeded {
			c.logger.Warn("list_commands timed out waiting for opencode server, returning empty", "cwd", cwd)
			return []protocol.AgentSlashCommand{}, nil
		}
		return nil, err
	}
	defer release()

	var commands []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Hints       []string `json:"hints"`
	}
	if err := opencodeGet(totalCtx, server.url, "/command", cwd, &commands); err != nil {
		return nil, err
	}
	result := make([]protocol.AgentSlashCommand, 0, len(commands))
	for _, cmd := range commands {
		entry := protocol.AgentSlashCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
		}
		if len(cmd.Hints) > 0 {
			entry.ArgumentHint = strings.Join(cmd.Hints, " ")
		}
		result = append(result, entry)
	}
	return result, nil
}

func (c *Client) ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return nil, err
	}

	server, release, err := c.serverManager.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	var providers opencodeProvidersResponse

	httpCtx, cancel := context.WithTimeout(ctx, opencodeProviderListTimeout)
	defer cancel()

	if err := opencodeGetContext(httpCtx, server.url, "/provider", cwd, &providers); err != nil {
		return nil, fmt.Errorf("list opencode providers: %w", err)
	}

	connectedSet := make(map[string]bool, len(providers.Connected))
	for _, id := range providers.Connected {
		connectedSet[id] = true
	}

	if len(connectedSet) == 0 {
		return nil, fmt.Errorf("opencode has no connected providers")
	}

	var models []protocol.AgentModelDefinition
	ctxWindows := make(map[string]int)
	for _, provider := range providers.All {
		if !connectedSet[provider.ID] {
			continue
		}
		for modelID, model := range provider.Models {
			def := buildOpenCodeModelDefinition(provider.ID, provider.Name, modelID, model)
			models = append(models, def)
			if cw := extractModelContextWindow(model); cw != nil {
				key := provider.ID + "/" + modelID
				ctxWindows[key] = *cw
			}
		}
	}

	// Populate client-level context window cache
	c.modelContextWindows = ctxWindows

	return models, nil
}

func (c *Client) ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error) {
	if err := c.IsAvailable(ctx); err != nil {
		return opencodeDefaultModes(), nil
	}

	server, release, err := c.serverManager.Acquire(ctx)
	if err != nil {
		return opencodeDefaultModes(), nil
	}
	defer release()

	var agents []struct {
		Name        string `json:"name"`
		Mode        string `json:"mode"`
		Hidden      bool   `json:"hidden"`
		Description string `json:"description"`
	}

	if err := opencodeGet(ctx, server.url, "/agent", cwd, &agents); err != nil {
		return opencodeDefaultModes(), nil
	}

	var modes []protocol.AgentMode
	for _, a := range agents {
		if a.Mode != "primary" || a.Hidden {
			continue
		}
		label := capitalizeFirst(a.Name)
		desc := a.Description
		if desc == "" {
			if d, ok := opencodeDefaultModeDescriptions[a.Name]; ok {
				desc = d
			}
		}
		modes = append(modes, protocol.AgentMode{
			ID:          a.Name,
			Label:       label,
			Description: desc,
		})
	}

	if len(modes) == 0 {
		return opencodeDefaultModes(), nil
	}
	return sortOpenCodeModes(modes), nil
}

func (c *Client) GetDiagnostic() string {
	var parts []string

	binary := c.binaryPath
	if binary == "" {
		binary = "not found"
	}
	parts = append(parts, fmt.Sprintf("Binary: %s", binary))

	if c.binaryPath != "" {
		if version, err := resolveOpenCodeVersion(c.binaryPath); err == nil {
			parts = append(parts, fmt.Sprintf("Version: %s", version))
		} else {
			parts = append(parts, "Version: unknown")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url, err := c.serverManager.EnsureRunning(ctx)
	if err != nil {
		parts = append(parts, fmt.Sprintf("Server: unavailable (%v)", err))
	} else {
		parts = append(parts, fmt.Sprintf("Server: running (%s)", url))
	}

	if err := c.IsAvailable(context.Background()); err != nil {
		parts = append(parts, fmt.Sprintf("Status: unavailable (%v)", err))
	} else {
		parts = append(parts, "Status: available")
	}

	// Models count
	models, err := c.ListModels(context.Background(), "")
	if err == nil {
		parts = append(parts, fmt.Sprintf("Models: %d", len(models)))
	}

	return strings.Join(parts, "\n")
}

func resolveOpenCodeVersion(binaryPath string) (string, error) {
	cmd := exec.Command(binaryPath, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// --- Model Definition Builder (matches Solo's buildOpenCodeModelDefinition) ---
