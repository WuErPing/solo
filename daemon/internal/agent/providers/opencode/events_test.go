package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// TestConsumeSSE_IdleTimeoutTriggersTurnFailed verifies that consumeSSE
// detects a dead SSE connection (no events received within the idle timeout)
// and synthesizes a turn_failed error.
func TestConsumeSSE_IdleTimeoutTriggersTurnFailed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Mock SSE server: sends 2 events, then hangs until client disconnects.
	// /command returns 503 so the heartbeat ping fails, letting the hard
	// timeout trigger.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/global/event" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// Send a few SSE events in /global/event format (wrapped in payload)
			for i := 0; i < 2; i++ {
				fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"test-session\"}}}\n\n")
				flusher.Flush()
			}

			// Then hang until client disconnects (r.Context() is cancelled when
			// the HTTP connection is closed), simulating a dead OpenCode server.
			<-r.Context().Done()
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

	// Wait for command warmup
	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	// Use a short idle timeout for testing
	session.sseReadIdleTimeout = 3 * time.Second

	// consumeSSE should detect the idle connection and return an error
	start := time.Now()
	_, err := session.consumeSSE(context.Background(), "test-turn-1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from consumeSSE when SSE stream goes idle, got nil")
	}

	// Should return within idle timeout + watchdog ticker margin
	if elapsed < 2*time.Second {
		t.Fatalf("consumeSSE returned too quickly (%v), idle timeout may not be working", elapsed)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("consumeSSE took too long (%v), idle timeout should have triggered by now", elapsed)
	}
}

// TestConsumeSSE_ActiveStreamDoesNotTimeout verifies that consumeSSE
// completes normally when SSE events arrive within the idle timeout window.
func TestConsumeSSE_ActiveStreamDoesNotTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Mock SSE server: sends events periodically, then a terminal event
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[]`)
			return
		}
		if r.URL.Path == "/global/event" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// Send events every 500ms, 4 events total, then a terminal event
			for i := 0; i < 4; i++ {
				time.Sleep(500 * time.Millisecond)
				fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"test-session\"}}}\n\n")
				flusher.Flush()
			}

			// Terminal event: session.status idle (wrapped in payload)
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"session.status\",\"properties\":{\"sessionID\":\"test-session\",\"status\":{\"type\":\"idle\"}}}}\n\n")
			flusher.Flush()
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	// Use a short idle timeout that should NOT trigger (events every 500ms)
	session.sseReadIdleTimeout = 5 * time.Second

	start := time.Now()
	result, err := session.consumeSSE(context.Background(), "test-turn-2")
	elapsed := time.Since(start)

	// Should complete successfully (session.status idle = turn_completed)
	if err != nil {
		t.Fatalf("expected no error from consumeSSE with active stream, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from consumeSSE")
	}

	// Should take roughly 2s (4 events * 500ms), definitely less than idle timeout
	if elapsed > 6*time.Second {
		t.Fatalf("consumeSSE took too long (%v), idle timeout may have triggered incorrectly", elapsed)
	}
}

// TestConsumeSSE_KeepAliveDoesNotResetIdleWatchdog verifies that SSE comment
// lines (": keep-alive") do NOT reset lastEventTime, and that when the
// heartbeat ping fails the hard timeout still triggers. During a long thinking
// phase OpenCode streams keep-alive lines but no data events. The idle watchdog
// must still fire when no real data arrives and the server is unresponsive.
func TestConsumeSSE_KeepAliveDoesNotResetIdleWatchdog(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Mock SSE server: sends one data event, then floods with keep-alive comments
	// every 200ms (well within the 3s idle timeout), then hangs.
	// The /command endpoint returns 503 so the heartbeat ping fails.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/global/event" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// One real data event at the start (in /global/event payload format)
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"test-session-keepalive\"}}}\n\n")
			flusher.Flush()

			// Then keep-alive comment lines every 200ms — these must NOT reset the timer
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					fmt.Fprintf(w, ": keep-alive\n\n")
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession(ts.URL, "test-session-keepalive", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	// Use a short idle timeout: 3s. Keep-alives arrive every 200ms.
	// Because /command returns 503, the heartbeat ping fails; the hard
	// timeout (3s) should still trigger despite keep-alive traffic.
	session.sseReadIdleTimeout = 3 * time.Second

	start := time.Now()
	_, err := session.consumeSSE(context.Background(), "test-turn-keepalive")
	elapsed := time.Since(start)

	// Must return an error (idle timeout synthesizes turn_failed)
	if err == nil {
		t.Fatal("expected error from consumeSSE when only keep-alives arrive, got nil")
	}

	// Must trigger the idle watchdog (3s) not run forever; allow 5s margin.
	if elapsed > 12*time.Second {
		t.Fatalf("consumeSSE took %v — watchdog never fired", elapsed)
	}

	// Must not return immediately (the keep-alive server is still running)
	if elapsed < 2*time.Second {
		t.Fatalf("consumeSSE returned too quickly (%v)", elapsed)
	}
}

// TestConsumeSSE_ContextCancelStopsWatchdog verifies that cancelling the
// parent context stops the idle watchdog goroutine and consumeSSE returns.
func TestConsumeSSE_ContextCancelStopsWatchdog(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Mock SSE server: sends one event then hangs until client disconnects
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[]`)
			return
		}
		if r.URL.Path == "/global/event" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// Send one event then hang (in /global/event payload format)
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"test-session\"}}}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	config := &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      "/tmp/test",
	}

	session := newOpenCodeSession(ts.URL, "test-session", config, logger, func() {}, nil)

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	// Use a long idle timeout so context cancellation wins the race
	session.sseReadIdleTimeout = 60 * time.Second

	// Cancel context after 1 second
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	start := time.Now()
	_, err := session.consumeSSE(ctx, "test-turn-3")
	elapsed := time.Since(start)

	// Should return promptly after context cancellation
	if elapsed > 3*time.Second {
		t.Fatalf("consumeSSE took too long after context cancel (%v), watchdog may not respect parent context", elapsed)
	}

	// Should return an error (stream ended prematurely since we cancelled)
	if err == nil {
		t.Fatal("expected error from consumeSSE when context is cancelled")
	}
}

// TestConsumeSSE_HeartbeatResetExtendsTimeout verifies that a successful
// server ping resets the idle timer, preventing false-positive timeout on
// long-running tool calls.
func TestConsumeSSE_HeartbeatResetExtendsTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Track whether the ping endpoint was hit
	var pingCount atomic.Int64

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			pingCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[]`)
			return
		}
		if r.URL.Path == "/global/event" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// Send one event immediately
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"test-session\"}}}\n\n")
			flusher.Flush()

			// Wait longer than the hard timeout but the heartbeat should reset
			time.Sleep(4 * time.Second)

			// Terminal event
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"session.status\",\"properties\":{\"sessionID\":\"test-session\",\"status\":{\"type\":\"idle\"}}}}\n\n")
			flusher.Flush()
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

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	// Hard timeout 3s, soft timeout ~2s. The 4s gap would normally trigger
	// timeout, but heartbeat ping should reset the timer.
	session.sseReadIdleTimeout = 3 * time.Second

	start := time.Now()
	result, err := session.consumeSSE(context.Background(), "test-turn-hb")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success with heartbeat reset, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if pingCount.Load() == 0 {
		t.Fatal("expected at least one heartbeat ping")
	}
	if elapsed < 3*time.Second {
		t.Fatalf("completed too quickly (%v), heartbeat may not have extended timeout", elapsed)
	}
}

// TestConsumeSSE_HeartbeatFailRespectsHardTimeout verifies that when the
// heartbeat ping fails, the hard timeout still triggers and fails the turn.
func TestConsumeSSE_HeartbeatFailRespectsHardTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			// Ping fails
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/global/event" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer does not support flushing")
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher.Flush()

			// Send one event, then hang
			fmt.Fprintf(w, "data: {\"payload\":{\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"test-session\"}}}\n\n")
			flusher.Flush()

			<-r.Context().Done()
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

	select {
	case <-session.commandsReadyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("warmup did not complete")
	}

	session.sseReadIdleTimeout = 3 * time.Second

	start := time.Now()
	_, err := session.consumeSSE(context.Background(), "test-turn-hb-fail")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when heartbeat fails and hard timeout triggers")
	}
	if elapsed < 2*time.Second {
		t.Fatalf("returned too quickly (%v), hard timeout may not have triggered", elapsed)
	}
	if elapsed > 8*time.Second {
		t.Fatalf("took too long (%v), hard timeout should have triggered earlier", elapsed)
	}
}
