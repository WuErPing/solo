package schedule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/llm"
	"github.com/WuErPing/solo/protocol"
)

type stubLLMResult struct {
	text string
	err  error
}

type stubLLMClient struct {
	mu       sync.Mutex
	queue    []stubLLMResult
	requests []llm.ChatRequest
	started  chan struct{}
	release  chan struct{}
}

func (s *stubLLMClient) ChatCompletion(ctx context.Context, req llm.ChatRequest) (string, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	res := stubLLMResult{}
	if len(s.queue) > 0 {
		res = s.queue[0]
		s.queue = s.queue[1:]
	}
	started, release := s.started, s.release
	s.mu.Unlock()

	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return res.text, res.err
}

func (s *stubLLMClient) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *stubLLMClient) request(i int) llm.ChatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[i]
}

func testLLMProviders() []config.LLMProviderConfig {
	return []config.LLMProviderConfig{
		{
			ID:      "test-provider",
			Label:   "Test Provider",
			BaseURL: "https://llm.example/v1",
			APIKey:  "k",
			Models:  []config.LLMModelConfig{{ID: "m1"}},
		},
	}
}

func newTestAssistant(store *Store, agents []AgentInfo, providers []config.LLMProviderConfig, client llmClient) *Assistant {
	return NewAssistant(AssistantConfig{
		Store:          store,
		AgentsFn:       func() []AgentInfo { return agents },
		LLMProvidersFn: func() []config.LLMProviderConfig { return providers },
		LLMClient:      client,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func testAssistRequest(message string) protocol.ScheduleAssistRequest {
	return protocol.ScheduleAssistRequest{
		Type:      "schedule/assist",
		RequestID: "req-1",
		Message:   message,
		Timezone:  "Asia/Shanghai",
		ClientNow: "2026-07-17T22:50:00+08:00",
	}
}

const validCreateLLMOutput = "```json\n" + validCreateJSON + "\n```"

func TestAssistant_CreateProposalHappyPath(t *testing.T) {
	client := &stubLLMClient{queue: []stubLLMResult{{text: validCreateLLMOutput}}}
	a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

	payload, err := a.Assist(context.Background(), testAssistRequest("every weekday at 9am summarize tests"))
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}

	if payload.Kind != "proposal" {
		t.Fatalf("kind = %q, want proposal (payload %+v)", payload.Kind, payload)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected error field: %s", *payload.Error)
	}
	if payload.Proposal == nil {
		t.Fatal("expected proposal")
	}
	if payload.Proposal.Op != "create" {
		t.Errorf("op = %q", payload.Proposal.Op)
	}
	if payload.Proposal.Cadence == nil || payload.Proposal.Cadence.Expression != "0 9 * * 1-5" {
		t.Errorf("cadence = %+v", payload.Proposal.Cadence)
	}
	if payload.Proposal.Summary == "" {
		t.Error("expected summary")
	}
	if payload.Proposal.NextRunAt == nil {
		t.Fatal("expected NextRunAt preview")
	}
	if _, err := time.Parse(time.RFC3339, *payload.Proposal.NextRunAt); err != nil {
		t.Errorf("NextRunAt not RFC3339: %q", *payload.Proposal.NextRunAt)
	}
	if payload.LLMProvider != "test-provider" {
		t.Errorf("llmProvider = %q", payload.LLMProvider)
	}
	if payload.Model != "m1" {
		t.Errorf("model = %q", payload.Model)
	}

	// The completion must carry system + user prompts and the resolved model.
	req := client.request(0)
	if req.Model != "m1" || req.BaseURL != "https://llm.example/v1" || req.APIKey != "k" {
		t.Errorf("unexpected chat request: %+v", req)
	}
	if req.SystemPrompt == "" || req.UserPrompt == "" {
		t.Error("expected system and user prompts")
	}
	if !strings.Contains(req.UserPrompt, "every weekday at 9am summarize tests") {
		t.Error("user prompt missing user message")
	}
}

func TestAssistant_ClarifyOnUnknownScheduleName(t *testing.T) {
	store := NewStore()
	createAssistTestSchedule(t, store, "DB backup")
	createAssistTestSchedule(t, store, "Files backup")

	client := &stubLLMClient{queue: []stubLLMResult{
		{text: `{"kind":"proposal","op":"pause","scheduleId":"backup","name":"backup","summary":"pause the backup"}`},
	}}
	a := newTestAssistant(store, nil, testLLMProviders(), client)

	payload, err := a.Assist(context.Background(), testAssistRequest("pause the backup"))
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}

	if payload.Kind != "clarify" {
		t.Fatalf("kind = %q, want clarify (payload %+v)", payload.Kind, payload)
	}
	if payload.Proposal != nil {
		t.Error("expected no proposal on clarify")
	}
	if !strings.Contains(payload.Message, "DB backup") || !strings.Contains(payload.Message, "Files backup") {
		t.Errorf("clarify message should list candidates, got %q", payload.Message)
	}
	if client.callCount() != 1 {
		t.Errorf("reference clarify should not retry, calls = %d", client.callCount())
	}
	if payload.LLMProvider != "test-provider" || payload.Model != "m1" {
		t.Error("expected provider/model echo on clarify")
	}
}

func TestAssistant_RetryOnceThenError(t *testing.T) {
	client := &stubLLMClient{queue: []stubLLMResult{
		{text: "total garbage"},
		{text: "still garbage"},
	}}
	a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

	payload, err := a.Assist(context.Background(), testAssistRequest("schedule something"))
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}

	if payload.Kind != "error" {
		t.Fatalf("kind = %q, want error", payload.Kind)
	}
	if payload.Error == nil || *payload.Error != "parse_failed" {
		t.Fatalf("error = %v, want parse_failed", payload.Error)
	}
	if !strings.Contains(payload.Message, "Try rephrasing") {
		t.Errorf("message = %q", payload.Message)
	}
	if client.callCount() != 2 {
		t.Errorf("expected exactly 2 completions (1 retry), got %d", client.callCount())
	}
}

func TestAssistant_RetrySecondCallSucceeds(t *testing.T) {
	client := &stubLLMClient{queue: []stubLLMResult{
		{text: "garbage, no json here"},
		{text: validCreateLLMOutput},
	}}
	a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

	payload, err := a.Assist(context.Background(), testAssistRequest("every weekday at 9am summarize tests"))
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}
	if payload.Kind != "proposal" {
		t.Fatalf("kind = %q, want proposal after retry", payload.Kind)
	}
	if client.callCount() != 2 {
		t.Fatalf("expected 2 completions, got %d", client.callCount())
	}
	retryPrompt := client.request(1).UserPrompt
	if !strings.Contains(retryPrompt, "invalid") {
		t.Error("retry prompt should include the previous validation error")
	}
}

func TestAssistant_NoLLMProvider(t *testing.T) {
	client := &stubLLMClient{}
	a := newTestAssistant(NewStore(), nil, nil, client)

	payload, err := a.Assist(context.Background(), testAssistRequest("schedule something"))
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}

	if payload.Kind != "error" {
		t.Fatalf("kind = %q, want error", payload.Kind)
	}
	if payload.Error == nil || *payload.Error != "no_llm_provider" {
		t.Fatalf("error = %v, want no_llm_provider", payload.Error)
	}
	if !strings.Contains(payload.Message, "Settings") {
		t.Errorf("message should point to settings, got %q", payload.Message)
	}
	if client.callCount() != 0 {
		t.Error("LLM client must not be called without a provider")
	}
}

func TestAssistant_RateLimit(t *testing.T) {
	client := &stubLLMClient{}
	for i := 0; i < 10; i++ {
		client.queue = append(client.queue, stubLLMResult{text: validCreateLLMOutput})
	}
	a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

	for i := 0; i < 10; i++ {
		payload, err := a.Assist(context.Background(), testAssistRequest(fmt.Sprintf("req %d", i)))
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if payload.Kind != "proposal" {
			t.Fatalf("call %d kind = %q, want proposal", i, payload.Kind)
		}
	}

	payload, err := a.Assist(context.Background(), testAssistRequest("one too many"))
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}
	if payload.Kind != "error" || payload.Error == nil || *payload.Error != "rate_limited" {
		t.Fatalf("11th call: kind = %q, error = %v; want rate_limited", payload.Kind, payload.Error)
	}
	if client.callCount() != 10 {
		t.Errorf("LLM calls = %d, want 10 (rate-limited call must not hit the LLM)", client.callCount())
	}
}

func TestAssistant_SingleFlight(t *testing.T) {
	client := &stubLLMClient{
		queue: []stubLLMResult{
			{text: validCreateLLMOutput},
			{text: validCreateLLMOutput},
		},
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

	var wg sync.WaitGroup
	var first *protocol.ScheduleAssistResponsePayload
	var firstErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		first, firstErr = a.Assist(context.Background(), testAssistRequest("first"))
	}()

	<-client.started

	second, err := a.Assist(context.Background(), testAssistRequest("second"))
	if err != nil {
		t.Fatalf("second Assist: %v", err)
	}
	if second.Kind != "error" || second.Error == nil || *second.Error != "rate_limited" {
		t.Fatalf("concurrent call: kind = %q, error = %v; want rate_limited", second.Kind, second.Error)
	}

	close(client.release)
	wg.Wait()

	if firstErr != nil {
		t.Fatalf("first Assist: %v", firstErr)
	}
	if first.Kind != "proposal" {
		t.Fatalf("first call kind = %q, want proposal", first.Kind)
	}
}

func TestAssistant_LLMErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		stubErr  error
		wantCode string
	}{
		{name: "auth", stubErr: fmt.Errorf("%w: status 401", llm.ErrLLMAuth), wantCode: "llm_auth"},
		{name: "endpoint rate limited", stubErr: fmt.Errorf("%w: status 429", llm.ErrLLMRateLimited), wantCode: "rate_limited"},
		{name: "unreachable", stubErr: errors.New("connection refused"), wantCode: "llm_unreachable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubLLMClient{queue: []stubLLMResult{{err: tt.stubErr}}}
			a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

			payload, err := a.Assist(context.Background(), testAssistRequest("schedule something"))
			if err != nil {
				t.Fatalf("Assist: %v", err)
			}
			if payload.Kind != "error" {
				t.Fatalf("kind = %q, want error", payload.Kind)
			}
			if payload.Error == nil || *payload.Error != tt.wantCode {
				t.Fatalf("error = %v, want %q", payload.Error, tt.wantCode)
			}
			if !strings.Contains(payload.Message, "Test Provider") {
				t.Errorf("message should name the provider, got %q", payload.Message)
			}
			if payload.LLMProvider != "test-provider" || payload.Model != "m1" {
				t.Error("expected provider/model echo on LLM error")
			}
			if client.callCount() != 1 {
				t.Errorf("transport errors must not retry, calls = %d", client.callCount())
			}
		})
	}
}

func TestAssistant_Guards(t *testing.T) {
	turn := protocol.ScheduleAssistTurn{Role: "user", Content: "x"}

	tests := []struct {
		name   string
		mutate func(req *protocol.ScheduleAssistRequest)
	}{
		{name: "empty message", mutate: func(req *protocol.ScheduleAssistRequest) { req.Message = "" }},
		{name: "message too long", mutate: func(req *protocol.ScheduleAssistRequest) { req.Message = strings.Repeat("x", 2001) }},
		{name: "invalid timezone", mutate: func(req *protocol.ScheduleAssistRequest) { req.Timezone = "Not/AZone" }},
		{
			name: "transcript too long",
			mutate: func(req *protocol.ScheduleAssistRequest) {
				for i := 0; i < 11; i++ {
					req.Transcript = append(req.Transcript, turn)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubLLMClient{}
			a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

			req := testAssistRequest("schedule something")
			tt.mutate(&req)

			payload, err := a.Assist(context.Background(), req)
			if err != nil {
				t.Fatalf("Assist: %v", err)
			}
			if payload.Kind != "error" || payload.Error == nil || *payload.Error != "invalid_request" {
				t.Fatalf("kind = %q, error = %v; want invalid_request", payload.Kind, payload.Error)
			}
			if client.callCount() != 0 {
				t.Error("guard failures must not call the LLM")
			}
		})
	}
}

func TestAssistant_InvalidClientNowFallsBack(t *testing.T) {
	client := &stubLLMClient{queue: []stubLLMResult{{text: validCreateLLMOutput}}}
	a := newTestAssistant(NewStore(), nil, testLLMProviders(), client)

	req := testAssistRequest("every weekday at 9am summarize tests")
	req.ClientNow = "not-a-timestamp"

	payload, err := a.Assist(context.Background(), req)
	if err != nil {
		t.Fatalf("Assist: %v", err)
	}
	if payload.Kind != "proposal" {
		t.Fatalf("kind = %q, want proposal despite bad clientNow", payload.Kind)
	}
	if payload.Proposal.NextRunAt == nil {
		t.Error("expected NextRunAt preview computed from fallback now")
	}
}
