// Package llm provides a minimal OpenAI-compatible chat completion client.
//
// It is used by the schedule assistant parse path: one non-streaming
// completion per parse, with the caller owning retry policy and per-call
// deadlines via context.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultTimeout is the overall per-request cap of the default HTTP client.
// Completions on slow endpoints can take tens of seconds; the design budget is
// 60s. Callers should bound each call with a context deadline — this timeout
// is only a backstop for contexts without one.
const DefaultTimeout = 60 * time.Second

// DefaultMaxTokens is sent when ChatRequest.MaxTokens <= 0.
const DefaultMaxTokens = 1024

// Sentinel errors for typed failure handling via errors.Is.
var (
	// ErrLLMAuth indicates the endpoint rejected the API key (401/403).
	ErrLLMAuth = errors.New("llm authentication failed")
	// ErrLLMRateLimited indicates the endpoint asked to slow down (429).
	ErrLLMRateLimited = errors.New("llm rate limited")
)

// ChatRequest describes one non-streaming chat completion call.
type ChatRequest struct {
	BaseURL      string // OpenAI-compatible base, e.g. "https://api.openai.com/v1"; a single trailing "/" is trimmed
	APIKey       string
	Model        string
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int // defaults to DefaultMaxTokens when <= 0
}

// Client performs chat completions against an OpenAI-compatible endpoint.
type Client struct {
	httpClient *http.Client
}

// NewClient returns a Client using httpClient; when nil, a default client with
// a DefaultTimeout overall timeout is used. The client never retries — retry
// policy lives in the caller.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{httpClient: httpClient}
}

// ChatCompletion POSTs {BaseURL}/chat/completions and returns the first
// choice's message content. The output is untrusted text; meaning is only
// given to it by the caller's own parsing/validation.
func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (string, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	body, err := json.Marshal(chatCompletionRequest{
		Model: req.Model,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
		Temperature: 0,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("marshal chat completion request: %w", err)
	}

	url := strings.TrimSuffix(req.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build chat completion request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("chat completion request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp.StatusCode)
	}

	var completion chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return "", fmt.Errorf("decode chat completion response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", errors.New("chat completion response has no choices")
	}
	return completion.Choices[0].Message.Content, nil
}

// statusError maps a non-200 response to a typed error carrying the status.
func statusError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: status %d", ErrLLMAuth, status)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: status %d", ErrLLMRateLimited, status)
	default:
		return fmt.Errorf("llm request failed: status %d", status)
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
