package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestServerManager(t *testing.T) *OpenCodeServerManager {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return &OpenCodeServerManager{
		logger:         logger.With("component", "opencode-server-manager"),
		binaryPath:     "echo", // Use echo as a safe no-op binary
		env:            []string{},
		retiredServers: make(map[*OpenCodeServerGeneration]struct{}),
	}
}

func TestOpenCodeServerManager_buildEnv(t *testing.T) {
	m := newTestServerManager(t)
	m.env = []string{"CUSTOM=1"}

	env := m.buildEnv()
	hasCustom := false
	for _, e := range env {
		if e == "CUSTOM=1" {
			hasCustom = true
		}
	}
	if !hasCustom {
		t.Error("expected custom env to be present")
	}
}

func TestOpenCodeServerManager_buildEnv_FiltersParentSession(t *testing.T) {
	m := newTestServerManager(t)
	// Simulate a parent session env var
	original := os.Environ()
	defer func() {
		for _, e := range original {
			parts := splitEnv(e)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()
	os.Setenv("CLAUDECODE", "1")

	env := m.buildEnv()
	for _, e := range env {
		if e == "CLAUDECODE=1" {
			t.Error("expected CLAUDECODE to be filtered out")
		}
	}
}

func TestIsParentSessionEnvVar(t *testing.T) {
	if !isParentSessionEnvVar("CLAUDECODE=1") {
		t.Error("expected CLAUDECODE to be detected")
	}
	if isParentSessionEnvVar("PATH=/usr/bin") {
		t.Error("expected PATH to not be detected")
	}
}

func TestOpenCodeServerManager_Shutdown(t *testing.T) {
	m := newTestServerManager(t)
	m.Shutdown()
	if m.currentServer != nil {
		t.Error("expected currentServer to be nil after shutdown")
	}
}

func TestOpenCodeServerManager_AcquireNormal_WithExistingServer(t *testing.T) {
	m := newTestServerManager(t)
	m.currentServer = &OpenCodeServerGeneration{
		port: 12345,
		url:  "http://127.0.0.1:12345",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	server, release, err := m.Acquire(ctx, false)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if server == nil {
		t.Fatal("expected server")
	}
	if server.refCount != 1 {
		t.Errorf("refCount: got %d, want 1", server.refCount)
	}
	release()

	if server.refCount != 0 {
		t.Errorf("refCount after release: got %d, want 0", server.refCount)
	}
}

func TestOpenCodeServerManager_Acquire_ContextCancelled(t *testing.T) {
	m := newTestServerManager(t)
	m.startPromise = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := m.Acquire(ctx, false)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestOpenCodeServerManager_killServer_Nil(t *testing.T) {
	m := newTestServerManager(t)
	// Should not panic for empty generation
	m.killServer(&OpenCodeServerGeneration{})
}

func TestOpenCodeServerManager_cleanupRetiredServers(t *testing.T) {
	m := newTestServerManager(t)
	retired := &OpenCodeServerGeneration{refCount: 0}
	m.retiredServers[retired] = struct{}{}
	m.cleanupRetiredServers()
	if len(m.retiredServers) != 0 {
		t.Error("expected retired server to be cleaned up")
	}
}

func TestOpenCodeServerManager_cleanupRetiredServers_SkipsReferenced(t *testing.T) {
	m := newTestServerManager(t)
	retired := &OpenCodeServerGeneration{refCount: 1}
	m.retiredServers[retired] = struct{}{}
	m.cleanupRetiredServers()
	if len(m.retiredServers) != 1 {
		t.Error("expected retired server with refCount>0 to remain")
	}
}

func splitEnv(e string) []string {
	for i := 0; i < len(e); i++ {
		if e[i] == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return []string{e}
}
