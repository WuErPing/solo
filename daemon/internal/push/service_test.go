package push

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/httpx"
)

func TestExpoPushService_Send(t *testing.T) {
	var receivedMessages []expoPushMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var messages []expoPushMessage
		if err := json.NewDecoder(r.Body).Decode(&messages); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		receivedMessages = append(receivedMessages, messages...)

		tickets := make([]expoPushTicket, len(messages))
		for i := range tickets {
			tickets[i] = expoPushTicket{Status: "ok"}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": tickets})
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	service := NewExpoPushService(server.URL, tokenStore, slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := NotificationPayload{
		Title: "Test Title",
		Body:  "Test Body",
		Data:  NotificationData{AgentID: "agent1", Reason: "finished"},
	}

	tokens := []string{"token1", "token2"}
	err := service.Send(tokens, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(receivedMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(receivedMessages))
	}

	msg := receivedMessages[0]
	if msg.To != "token1" {
		t.Errorf("expected token1, got %s", msg.To)
	}
	if msg.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %s", msg.Title)
	}
	if msg.Body != "Test Body" {
		t.Errorf("expected body 'Test Body', got %s", msg.Body)
	}
	if msg.Sound != "default" {
		t.Errorf("expected sound 'default', got %s", msg.Sound)
	}
	if msg.Data.AgentID != "agent1" {
		t.Errorf("expected agentId 'agent1', got %s", msg.Data.AgentID)
	}
}

func TestExpoPushService_Send_DeviceNotRegistered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var messages []expoPushMessage
		json.NewDecoder(r.Body).Decode(&messages)

		tickets := make([]expoPushTicket, len(messages))
		for i := range tickets {
			tickets[i] = expoPushTicket{
				Status:  "error",
				Details: &expoErrorDetails{Error: "DeviceNotRegistered"},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": tickets})
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	tokenStore.Register("bad-token")

	service := NewExpoPushService(server.URL, tokenStore, slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := NotificationPayload{Title: "Test", Body: "Test"}
	service.Send([]string{"bad-token"}, payload)

	tokens := tokenStore.GetAll()
	if len(tokens) != 0 {
		t.Errorf("expected token to be removed, got %d tokens", len(tokens))
	}
}

func TestExpoPushService_Send_InvalidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var messages []expoPushMessage
		json.NewDecoder(r.Body).Decode(&messages)

		tickets := make([]expoPushTicket, len(messages))
		for i := range tickets {
			tickets[i] = expoPushTicket{
				Status:  "error",
				Details: &expoErrorDetails{Error: "InvalidCredentials"},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": tickets})
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	tokenStore.Register("bad-token")

	service := NewExpoPushService(server.URL, tokenStore, slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := NotificationPayload{Title: "Test", Body: "Test"}
	service.Send([]string{"bad-token"}, payload)

	tokens := tokenStore.GetAll()
	if len(tokens) != 0 {
		t.Errorf("expected token to be removed, got %d tokens", len(tokens))
	}
}

func TestExpoPushService_Send_Batching(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var messages []expoPushMessage
		json.NewDecoder(r.Body).Decode(&messages)

		if len(messages) > 100 {
			t.Errorf("batch size %d exceeds limit of 100", len(messages))
		}

		tickets := make([]expoPushTicket, len(messages))
		for i := range tickets {
			tickets[i] = expoPushTicket{Status: "ok"}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": tickets})
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	service := NewExpoPushService(server.URL, tokenStore, slog.New(slog.NewTextHandler(io.Discard, nil)))

	tokens := make([]string, 250)
	for i := range tokens {
		tokens[i] = "token-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}

	payload := NotificationPayload{Title: "Test", Body: "Test"}
	err := service.Send(tokens, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount != 3 {
		t.Errorf("expected 3 requests for 250 tokens, got %d", requestCount)
	}
}

func TestExpoPushService_Send_EmptyTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("should not call Expo API with empty tokens")
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	service := NewExpoPushService(server.URL, tokenStore, slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := NotificationPayload{Title: "Test", Body: "Test"}
	err := service.Send([]string{}, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestService(serverURL string, tokenStore TokenStore) *ExpoPushService {
	svc := NewExpoPushService(serverURL, tokenStore, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.RetryDelay = 10 * time.Millisecond
	return svc
}

func TestExpoPushService_RetriesOn429(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := calls.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []expoPushTicket{{Status: "ok"}}})
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	svc := newTestService(server.URL, tokenStore)

	err := svc.Send([]string{"tok-1"}, NotificationPayload{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("expected no error after retries, got: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", calls.Load())
	}
}

func TestExpoPushService_RetriesOn5xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := calls.Add(1)
		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []expoPushTicket{{Status: "ok"}}})
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	svc := newTestService(server.URL, tokenStore)

	err := svc.Send([]string{"tok-1"}, NotificationPayload{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls (1 failure + 1 success), got %d", calls.Load())
	}
}

func TestExpoPushService_NoRetryOn4xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	svc := newTestService(server.URL, tokenStore)

	err := svc.Send([]string{"tok-1"}, NotificationPayload{Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error for 4xx response")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call (no retry for 4xx), got %d", calls.Load())
	}
}

func TestExpoPushService_TimeoutDoesNotLeak(t *testing.T) {
	// Simulate a target that accepts the TCP connection but never sends a
	// response. With the old bare &http.Client{}, Send would hang forever
	// and leak a goroutine per batch; with httpx.Standard() the overall
	// request timeout aborts it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Accept one connection and do nothing so the client waits for headers.
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			connCh <- conn
		}
	}()
	defer func() {
		select {
		case c := <-connCh:
			_ = c.Close()
		default:
		}
	}()

	tokenStore := NewInMemoryTokenStore()
	svc := newTestService("http://"+ln.Addr().String(), tokenStore)
	svc.MaxRetries = 1
	svc.RetryDelay = 10 * time.Millisecond

	// Use a short-timeout client so the suite stays fast; the production
	// client is verified by TestExpoPushService_UsesStandardClient and the
	// httpx package tests.
	svc.client = httpx.NewClient(httpx.Config{
		ConnectTimeout:        50 * time.Millisecond,
		ResponseHeaderTimeout: 100 * time.Millisecond,
		IdleConnTimeout:       90 * time.Second,
		RequestTimeout:        200 * time.Millisecond,
	})

	start := time.Now()
	payload := NotificationPayload{Title: "T", Body: "B"}
	_ = svc.Send([]string{"tok-1"}, payload)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Send hung too long; elapsed=%v (want ≤ 2s)", elapsed)
	}
}

func TestExpoPushService_UsesStandardClient(t *testing.T) {
	svc := NewExpoPushService("", NewInMemoryTokenStore(), nil)
	if svc.client == nil {
		t.Fatal("ExpoPushService.client is nil")
	}
	if svc.client != httpx.Standard() {
		t.Errorf("ExpoPushService should use httpx.Standard()")
	}
}

func TestExpoPushService_MaxRetriesExhausted(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	tokenStore := NewInMemoryTokenStore()
	svc := newTestService(server.URL, tokenStore)
	svc.MaxRetries = 3

	err := svc.Send([]string{"tok-1"}, NotificationPayload{Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error when all retries exhausted")
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls (max retries), got %d", calls.Load())
	}
}
