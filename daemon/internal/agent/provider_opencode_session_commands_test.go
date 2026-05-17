package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// TestOpenCodeSessionListCommandsReturnsCachedCommandsImmediately tests that
// ListCommands returns preloaded cached commands immediately without making
// an HTTP request.
func TestOpenCodeSessionListCommandsReturnsCachedCommandsImmediately(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a test server that tracks request count
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path == "/command" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"name":"help","description":"Show help","hints":[]}]`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	// Create session - this should trigger warmup in background
	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	// Wait for warmup to complete
	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete in time")
	}

	// First call - should return cached commands immediately
	start := time.Now()
	commands, err := session.ListCommands(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Name != "help" {
		t.Fatalf("expected command 'help', got %s", commands[0].Name)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("ListCommands took too long: %v (expected < 100ms for cached)", elapsed)
	}

	// Second call - should still use cache, no additional HTTP request
	requestCountBefore := requestCount
	commands2, err := session.ListCommands(context.Background())
	if err != nil {
		t.Fatalf("second ListCommands failed: %v", err)
	}
	if requestCount != requestCountBefore {
		t.Fatalf("second ListCommands made an HTTP request (count went from %d to %d)", requestCountBefore, requestCount)
	}
	if len(commands2) != 1 {
		t.Fatalf("expected 1 command on second call, got %d", len(commands2))
	}
}

// TestOpenCodeSessionListCommandsWaitsForWarmup tests that ListCommands waits
// for background warmup when called before warmup completes.
func TestOpenCodeSessionListCommandsWaitsForWarmup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a slow test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			// Simulate slow response
			time.Sleep(500 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"name":"help","description":"Show help"}]`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	// Create session - warmup starts in background
	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	// Call ListCommands immediately (before warmup completes)
	start := time.Now()
	commands, err := session.ListCommands(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ListCommands failed: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	// Should have waited for warmup (~500ms)
	if elapsed < 400*time.Millisecond {
		t.Fatalf("ListCommands returned too quickly: %v (expected to wait for warmup)", elapsed)
	}
}

// TestOpenCodeSessionListCommandsConcurrentAccess tests that concurrent
// ListCommands calls are safe and don't race.
func TestOpenCodeSessionListCommandsConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"name":"cmd1","description":"Command 1"},{"name":"cmd2","description":"Command 2"}]`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	// Wait for warmup
	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	// Concurrent calls
	const numGoroutines = 10
	results := make(chan []protocol.AgentSlashCommand, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			cmds, err := session.ListCommands(context.Background())
			if err != nil {
				errors <- err
				return
			}
			results <- cmds
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errors:
			t.Fatalf("concurrent ListCommands failed: %v", err)
		case cmds := <-results:
			if len(cmds) != 2 {
				t.Fatalf("expected 2 commands, got %d", len(cmds))
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent ListCommands")
		}
	}
}

// TestOpenCodeSessionListCommandsFallbackOnTimeout tests that ListCommands
// returns an empty list when warmup is slow and exceeds the wait timeout.
func TestOpenCodeSessionListCommandsFallbackOnTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Very slow server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			time.Sleep(10 * time.Second) // Very slow
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"name":"help","description":"Show help"}]`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	// Call ListCommands - should timeout and return empty list
	start := time.Now()
	commands, err := session.ListCommands(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ListCommands should not error on timeout fallback: %v", err)
	}
	if len(commands) != 0 {
		t.Fatalf("expected empty list on timeout, got %d commands", len(commands))
	}
	// Should return quickly (within 2s timeout + some margin)
	if elapsed > 3*time.Second {
		t.Fatalf("ListCommands took too long to fallback: %v", elapsed)
	}
}