package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Listen != "127.0.0.1:17612" {
		t.Errorf("Listen: got %q, want %q", cfg.Listen, "127.0.0.1:17612")
	}
	if !cfg.RelayEnabled {
		t.Error("expected RelayEnabled to be true")
	}
	if cfg.RelayEndpoint != "relay.solo.sh:443" {
		t.Errorf("RelayEndpoint: got %q", cfg.RelayEndpoint)
	}
	if cfg.AppBaseURL != "https://solo.up2ai.top" {
		t.Errorf("AppBaseURL: got %q", cfg.AppBaseURL)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://solo.up2ai.top" || cfg.CORSOrigins[1] != "http://localhost:19000" {
		t.Errorf("CORSOrigins: got %v", cfg.CORSOrigins)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SoloHome == "" {
		t.Error("expected SoloHome to be resolved")
	}
	if cfg.ServerID == "" {
		t.Error("expected ServerID to be generated")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("SOLO_LISTEN", "127.0.0.1:9999")
	t.Setenv("SOLO_RELAY_ENABLED", "false")
	t.Setenv("SOLO_RELAY_ENDPOINT", "custom.relay.sh")
	t.Setenv("SOLO_CORS_ORIGINS", "http://localhost:3000,http://localhost:3001")
	t.Setenv("SOLO_HOSTNAMES", "host1,host2")
	t.Setenv("SOLO_APP_BASE_URL", "http://local.app")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1:9999" {
		t.Errorf("Listen override: got %q", cfg.Listen)
	}
	if cfg.RelayEnabled {
		t.Error("expected RelayEnabled to be false")
	}
	if cfg.RelayEndpoint != "custom.relay.sh" {
		t.Errorf("RelayEndpoint override: got %q", cfg.RelayEndpoint)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Errorf("CORSOrigins: expected 2, got %d", len(cfg.CORSOrigins))
	}
	if len(cfg.Hostnames) != 2 {
		t.Errorf("Hostnames: expected 2, got %d", len(cfg.Hostnames))
	}
	if cfg.AppBaseURL != "http://local.app" {
		t.Errorf("AppBaseURL override: got %q", cfg.AppBaseURL)
	}
}

func TestLoad_PORT_Env(t *testing.T) {
	t.Setenv("PORT", "8080")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1:8080" {
		t.Errorf("PORT override: got %q", cfg.Listen)
	}
}

func TestLoad_PersistedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLO_HOME", home)

	configData := []byte(`{"daemon":{"listen":"127.0.0.1:7777","hostnames":["h1"],"mcp":{"injectIntoAgents":true},"cors":{"origins":["http://example.com"]},"relay":{"enabled":false,"endpoint":"persisted.endpoint","publicEndpoint":"persisted.pub"},"providers":{"customModels":{"claude":[{"id":"custom1","label":"Custom"}]}}},"app":{"baseUrl":"http://persisted.app"}}`)
	_ = os.WriteFile(filepath.Join(home, "config.json"), configData, 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1:7777" {
		t.Errorf("persisted Listen: got %q", cfg.Listen)
	}
	if len(cfg.Hostnames) != 1 || cfg.Hostnames[0] != "h1" {
		t.Errorf("persisted Hostnames: got %v", cfg.Hostnames)
	}
	if !cfg.MCPInjectIntoAgents {
		t.Error("expected MCPInjectIntoAgents to be true")
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://example.com" {
		t.Errorf("persisted CORSOrigins: got %v", cfg.CORSOrigins)
	}
	if cfg.RelayEnabled {
		t.Error("expected persisted RelayEnabled to be false")
	}
	if cfg.RelayEndpoint != "persisted.endpoint" {
		t.Errorf("persisted RelayEndpoint: got %q", cfg.RelayEndpoint)
	}
	if cfg.AppBaseURL != "http://persisted.app" {
		t.Errorf("persisted AppBaseURL: got %q", cfg.AppBaseURL)
	}
	if cfg.CustomModels == nil {
		t.Fatal("expected CustomModels")
	}
	models, ok := cfg.CustomModels["claude"]
	if !ok || len(models) != 1 || models[0].ID != "custom1" {
		t.Errorf("custom models mismatch: %+v", cfg.CustomModels)
	}
}

func TestLoad_CORSOrigins_TrimsSpaces(t *testing.T) {
	t.Setenv("SOLO_CORS_ORIGINS", "https://a.com, https://b.com , https://c.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	expected := []string{"https://a.com", "https://b.com", "https://c.com"}
	if len(cfg.CORSOrigins) != len(expected) {
		t.Fatalf("CORSOrigins: expected %d, got %d", len(expected), len(cfg.CORSOrigins))
	}
	for i, want := range expected {
		if cfg.CORSOrigins[i] != want {
			t.Errorf("CORSOrigins[%d]: got %q, want %q", i, cfg.CORSOrigins[i], want)
		}
	}
}

func TestLoad_ServerID_Persistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLO_HOME", home)

	cfg1, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	id1 := cfg1.ServerID
	if id1 == "" {
		t.Fatal("expected ServerID to be generated")
	}

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load second: %v", err)
	}
	if cfg2.ServerID != id1 {
		t.Errorf("expected same ServerID, got %q vs %q", cfg2.ServerID, id1)
	}
}

func TestResolveListenTarget_TCP(t *testing.T) {
	cfg := &Config{Listen: "127.0.0.1:17612"}
	target, err := cfg.ResolveListenTarget()
	if err != nil {
		t.Fatalf("ResolveListenTarget: %v", err)
	}
	if target.Type != "tcp" || target.Host != "127.0.0.1" || target.Port != 17612 {
		t.Errorf("unexpected target: %+v", target)
	}
}

func TestResolveListenTarget_Unix(t *testing.T) {
	cfg := &Config{Listen: "unix:///tmp/solo.sock"}
	target, err := cfg.ResolveListenTarget()
	if err != nil {
		t.Fatalf("ResolveListenTarget: %v", err)
	}
	if target.Type != "socket" || target.Path != "/tmp/solo.sock" {
		t.Errorf("unexpected target: %+v", target)
	}
}

func TestResolveListenTarget_Pipe(t *testing.T) {
	cfg := &Config{Listen: "pipe:///tmp/solo.pipe"}
	target, err := cfg.ResolveListenTarget()
	if err != nil {
		t.Fatalf("ResolveListenTarget: %v", err)
	}
	if target.Type != "pipe" || target.Path != "/tmp/solo.pipe" {
		t.Errorf("unexpected target: %+v", target)
	}
}

func TestResolveListenTarget_IPv6(t *testing.T) {
	cfg := &Config{Listen: "[::1]:8080"}
	target, err := cfg.ResolveListenTarget()
	if err != nil {
		t.Fatalf("ResolveListenTarget: %v", err)
	}
	if target.Type != "tcp" || target.Host != "[::1]" || target.Port != 8080 {
		t.Errorf("unexpected target: %+v", target)
	}
}

func TestGetTmuxAgentNames_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	names := cfg.GetTmuxAgentNames()
	expected := []string{"claude", "opencode", "qodercli", "pi", "cursor", "kimi", "kimi-cli", "codex"}
	for _, want := range expected {
		if !names[want] {
			t.Errorf("expected built-in agent %q in result", want)
		}
	}
	if len(names) != len(expected) {
		t.Errorf("expected %d names, got %d: %v", len(expected), len(names), names)
	}
}

func TestGetTmuxAgentNames_WithCustom(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmuxAgentNames = []string{"aider", "cody"}
	names := cfg.GetTmuxAgentNames()
	if !names["aider"] {
		t.Error("expected custom agent 'aider'")
	}
	if !names["cody"] {
		t.Error("expected custom agent 'cody'")
	}
	if !names["claude"] {
		t.Error("expected built-in agent 'claude' still present")
	}
	if len(names) != 10 {
		t.Errorf("expected 10 names (8 built-in + 2 custom), got %d: %v", len(names), names)
	}
}

func TestGetTmuxAgentNames_Dedup(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmuxAgentNames = []string{"claude", "aider"}
	names := cfg.GetTmuxAgentNames()
	if !names["claude"] {
		t.Error("expected 'claude' present")
	}
	if !names["aider"] {
		t.Error("expected 'aider' present")
	}
	if len(names) != 9 {
		t.Errorf("expected 9 names (8 built-in + 1 custom, 'claude' deduped), got %d: %v", len(names), names)
	}
}

func TestGetTmuxAgentNames_CaseInsensitive(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmuxAgentNames = []string{"Aider", "CODY"}
	names := cfg.GetTmuxAgentNames()
	if !names["aider"] {
		t.Error("expected 'aider' (lowercased from 'Aider')")
	}
	if !names["cody"] {
		t.Error("expected 'cody' (lowercased from 'CODY')")
	}
}

func TestLoad_PersistedConfig_TmuxAgentNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLO_HOME", home)

	configData := []byte(`{"daemon":{"tmuxAgentNames":["aider","cody"]}}`)
	_ = os.WriteFile(filepath.Join(home, "config.json"), configData, 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TmuxAgentNames) != 2 {
		t.Fatalf("expected 2 persisted tmux agent names, got %d: %v", len(cfg.TmuxAgentNames), cfg.TmuxAgentNames)
	}
	if cfg.TmuxAgentNames[0] != "aider" || cfg.TmuxAgentNames[1] != "cody" {
		t.Errorf("persisted TmuxAgentNames: got %v", cfg.TmuxAgentNames)
	}

	names := cfg.GetTmuxAgentNames()
	if !names["aider"] || !names["cody"] {
		t.Error("custom names should be in merged result")
	}
	if !names["claude"] {
		t.Error("built-in names should still be present")
	}
}

func TestResolveListenTarget_Invalid(t *testing.T) {
	cases := []string{
		"no-port",
		"[::1",
	}
	for _, addr := range cases {
		cfg := &Config{Listen: addr}
		_, err := cfg.ResolveListenTarget()
		if err == nil {
			t.Errorf("expected error for %q", addr)
		}
	}
}
