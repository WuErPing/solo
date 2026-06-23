package opencode

import (
	"context"
	"fmt"
	"strings"

	"github.com/WuErPing/solo/protocol"
)

var mcpAlreadyPresentTokens = []string{"already", "exists", "connected"}

// --- MCP Configuration (gap #4) ---

func (s *openCodeSession) ensureMcpConfigured(ctx context.Context) error {
	if s.mcpConfigured {
		return nil
	}
	if len(s.base.Config().McpServers) == 0 {
		s.mcpConfigured = true
		return nil
	}

	s.mu.Lock()
	if s.mcpSetupPromise != nil {
		ch := s.mcpSetupPromise
		s.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
		return s.mcpSetupErr
	}
	ch := make(chan struct{})
	s.mcpSetupPromise = ch
	s.mu.Unlock()

	err := s.configureMcpServers(ctx)

	s.mu.Lock()
	s.mcpSetupErr = err
	if err == nil {
		s.mcpConfigured = true
	}
	close(ch)
	s.mcpSetupPromise = nil
	s.mu.Unlock()
	return err
}

func (s *openCodeSession) configureMcpServers(ctx context.Context) error {
	if len(s.base.Config().McpServers) == 0 {
		return nil
	}
	// Register MCP servers in parallel (matches Solo's Promise.all)
	type result struct {
		name string
		err  error
	}
	ch := make(chan result, len(s.base.Config().McpServers))
	for name, serverConfig := range s.base.Config().McpServers {
		go func(n string, cfg protocol.McpServerConfig) {
			ch <- result{n, s.registerMcpServer(ctx, n, cfg)}
		}(name, serverConfig)
	}
	var errs []string
	for range s.base.Config().McpServers {
		r := <-ch
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.name, r.err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("MCP registration errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *openCodeSession) registerMcpServer(ctx context.Context, name string, config protocol.McpServerConfig) error {
	mcpConfig := toOpenCodeMcpConfig(config)

	// mcp.add
	httpCtx, cancel := context.WithTimeout(ctx, opencodeMcpAddTimeout)
	defer cancel()
	addBody := map[string]interface{}{
		"name":   name,
		"config": mcpConfig,
	}
	if err := opencodePostJSON(httpCtx, s.baseURL, "/mcp", s.base.Config().Cwd, addBody, nil); err != nil {
		if isMcpAlreadyPresentError(err) {
			return nil
		}
		return fmt.Errorf("mcp.add: %w", err)
	}

	// mcp.connect
	connCtx, connCancel := context.WithTimeout(ctx, opencodeMcpAddTimeout)
	defer connCancel()
	connPath := "/mcp/" + name + "/connect"
	if err := opencodePostJSON(connCtx, s.baseURL, connPath, s.base.Config().Cwd, nil, nil); err != nil {
		if isMcpAlreadyPresentError(err) {
			return nil
		}
		return fmt.Errorf("mcp.connect: %w", err)
	}
	return nil
}

func toOpenCodeMcpConfig(config protocol.McpServerConfig) map[string]interface{} {
	if config.Type == "stdio" {
		cmd := []string{config.Command}
		cmd = append(cmd, config.Args...)
		mcp := map[string]interface{}{
			"type":    "local",
			"command": cmd,
		}
		if len(config.Env) > 0 {
			mcp["environment"] = config.Env
		}
		return mcp
	}
	mcp := map[string]interface{}{
		"type": "remote",
		"url":  config.URL,
	}
	if len(config.Headers) > 0 {
		mcp["headers"] = config.Headers
	}
	return mcp
}

func isMcpAlreadyPresentError(err error) bool {
	lower := strings.ToLower(err.Error())
	for _, token := range mcpAlreadyPresentTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}
