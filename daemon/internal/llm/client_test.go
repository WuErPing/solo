package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capturedRequest struct {
	method        string
	path          string
	authorization string
	contentType   string
	body          map[string]any
}

// newChatServer starts a test server that captures the incoming request and
// responds with status and body.
func newChatServer(t *testing.T, status int, body string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.authorization = r.Header.Get("Authorization")
		captured.contentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&captured.body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func successBody(content string) string {
	return `{"choices":[{"message":{"role":"assistant","content":` + strconvQuote(content) + `}}]}`
}

func strconvQuote(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

func TestChatCompletionSuccess(t *testing.T) {
	srv, captured := newChatServer(t, http.StatusOK, successBody(`{"kind":"answer","message":"3 schedules"}`))

	client := NewClient(nil)
	got, err := client.ChatCompletion(context.Background(), ChatRequest{
		BaseURL:      srv.URL,
		APIKey:       "sk-test",
		Model:        "gpt-4o",
		SystemPrompt: "you are a parser",
		UserPrompt:   "what runs today?",
		MaxTokens:    512,
	})
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}

	if got != `{"kind":"answer","message":"3 schedules"}` {
		t.Errorf("content: got %q", got)
	}
	if captured.method != http.MethodPost {
		t.Errorf("method: got %q, want POST", captured.method)
	}
	if captured.path != "/chat/completions" {
		t.Errorf("path: got %q, want /chat/completions", captured.path)
	}
	if captured.authorization != "Bearer sk-test" {
		t.Errorf("authorization: got %q, want %q", captured.authorization, "Bearer sk-test")
	}
	if captured.contentType != "application/json" {
		t.Errorf("content-type: got %q, want application/json", captured.contentType)
	}
	if captured.body["model"] != "gpt-4o" {
		t.Errorf("model: got %v, want gpt-4o", captured.body["model"])
	}
	messages, ok := captured.body["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages: got %#v, want 2 entries", captured.body["messages"])
	}
	system, _ := messages[0].(map[string]any)
	user, _ := messages[1].(map[string]any)
	if system["role"] != "system" || system["content"] != "you are a parser" {
		t.Errorf("system message: got %#v", system)
	}
	if user["role"] != "user" || user["content"] != "what runs today?" {
		t.Errorf("user message: got %#v", user)
	}
	if captured.body["temperature"] != float64(0) {
		t.Errorf("temperature: got %v, want 0", captured.body["temperature"])
	}
	if captured.body["max_tokens"] != float64(512) {
		t.Errorf("max_tokens: got %v, want 512", captured.body["max_tokens"])
	}
	if _, sent := captured.body["response_format"]; sent {
		t.Error("response_format must not be sent (endpoint compatibility)")
	}
}

func TestChatCompletionDefaultMaxTokens(t *testing.T) {
	tests := []struct {
		name      string
		maxTokens int
		want      float64
	}{
		{"zero", 0, 1024},
		{"negative", -5, 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, captured := newChatServer(t, http.StatusOK, successBody("ok"))

			client := NewClient(nil)
			_, err := client.ChatCompletion(context.Background(), ChatRequest{
				BaseURL:   srv.URL,
				APIKey:    "sk-test",
				Model:     "gpt-4o",
				MaxTokens: tt.maxTokens,
			})
			if err != nil {
				t.Fatalf("ChatCompletion: %v", err)
			}
			if captured.body["max_tokens"] != tt.want {
				t.Errorf("max_tokens: got %v, want %v", captured.body["max_tokens"], tt.want)
			}
		})
	}
}

func TestChatCompletionBaseURLHandling(t *testing.T) {
	tests := []struct {
		name        string
		baseURLPath string
		wantPath    string
	}{
		{"bare", "", "/chat/completions"},
		{"trailing slash", "/", "/chat/completions"},
		{"versioned", "/v1", "/v1/chat/completions"},
		{"versioned trailing slash", "/v1/", "/v1/chat/completions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, captured := newChatServer(t, http.StatusOK, successBody("ok"))

			client := NewClient(nil)
			_, err := client.ChatCompletion(context.Background(), ChatRequest{
				BaseURL: srv.URL + tt.baseURLPath,
				APIKey:  "sk-test",
				Model:   "gpt-4o",
			})
			if err != nil {
				t.Fatalf("ChatCompletion: %v", err)
			}
			if captured.path != tt.wantPath {
				t.Errorf("path: got %q, want %q", captured.path, tt.wantPath)
			}
		})
	}
}

func TestChatCompletionHTTPStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantErr    error
		wantStatus string
	}{
		{"unauthorized", http.StatusUnauthorized, ErrLLMAuth, "401"},
		{"forbidden", http.StatusForbidden, ErrLLMAuth, "403"},
		{"rate limited", http.StatusTooManyRequests, ErrLLMRateLimited, "429"},
		{"server error", http.StatusInternalServerError, nil, "500"},
		{"bad gateway", http.StatusBadGateway, nil, "502"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newChatServer(t, tt.status, `{"error":{"message":"nope"}}`)

			client := NewClient(nil)
			_, err := client.ChatCompletion(context.Background(), ChatRequest{
				BaseURL: srv.URL,
				APIKey:  "sk-test",
				Model:   "gpt-4o",
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error: got %v, want errors.Is %v", err, tt.wantErr)
				}
			} else {
				if errors.Is(err, ErrLLMAuth) || errors.Is(err, ErrLLMRateLimited) {
					t.Errorf("error must not match auth/rate-limit sentinels: %v", err)
				}
			}
			if !strings.Contains(err.Error(), tt.wantStatus) {
				t.Errorf("error must include status %s: %v", tt.wantStatus, err)
			}
		})
	}
}

func TestChatCompletionMalformedResponseJSON(t *testing.T) {
	srv, _ := newChatServer(t, http.StatusOK, `not json at all`)

	client := NewClient(nil)
	_, err := client.ChatCompletion(context.Background(), ChatRequest{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4o",
	})
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestChatCompletionEmptyChoices(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty array", `{"choices":[]}`},
		{"missing field", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newChatServer(t, http.StatusOK, tt.body)

			client := NewClient(nil)
			_, err := client.ChatCompletion(context.Background(), ChatRequest{
				BaseURL: srv.URL,
				APIKey:  "sk-test",
				Model:   "gpt-4o",
			})
			if err == nil {
				t.Fatal("expected error for empty choices, got nil")
			}
		})
	}
}

func TestChatCompletionContextCanceled(t *testing.T) {
	srv, _ := newChatServer(t, http.StatusOK, successBody("ok"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(nil)
	_, err := client.ChatCompletion(ctx, ChatRequest{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4o",
	})
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error: got %v, want errors.Is context.Canceled", err)
	}
}
