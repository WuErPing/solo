package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestScheduleAssistRequestRoundTrip(t *testing.T) {
	req := ScheduleAssistRequest{
		Type:              "schedule/assist",
		RequestID:         "req-1",
		Message:           "every weekday at 9am, summarize overnight agent activity",
		Timezone:          "Asia/Shanghai",
		ClientNow:         "2026-07-18T09:00:00+08:00",
		ContextScheduleID: "sched-1",
		Transcript: []ScheduleAssistTurn{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ScheduleAssistRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, req)
	}
}

func TestScheduleAssistRequestOmitsOptionalFields(t *testing.T) {
	req := ScheduleAssistRequest{
		Type:      "schedule/assist",
		RequestID: "req-2",
		Message:   "what runs today?",
		Timezone:  "UTC",
		ClientNow: "2026-07-18T01:00:00Z",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"contextScheduleId"`, `"transcript"`} {
		if strings.Contains(string(data), key) {
			t.Errorf("expected %s to be omitted, got %s", key, data)
		}
	}
}

func TestScheduleAssistResponseRoundTrip(t *testing.T) {
	maxRuns := 30
	nextRun := "2026-07-21T09:00:00+08:00"
	resp := ScheduleAssistResponse{
		Type: "schedule/assist/response",
		Payload: ScheduleAssistResponsePayload{
			RequestID: "req-1",
			Kind:      "proposal",
			Proposal: &ScheduleAssistProposal{
				Op:        "create",
				Name:      "Nightly test summary",
				Prompt:    "Summarize the nightly test runs",
				Cadence:   &ScheduleCadence{Type: "cron", Expression: "0 9 * * 1-5", Timezone: "Asia/Shanghai"},
				Target:    &ScheduleTarget{Type: "agent", AgentID: "a1b2"},
				Cwd:       "~/work/backend",
				MaxRuns:   &maxRuns,
				ExpiresAt: "2026-08-31T00:00:00+08:00",
				Summary:   "Create 'Nightly test summary' every weekday at 09:00",
				Warnings:  []string{"interpreted 'morning' as 09:00"},
				NextRunAt: &nextRun,
			},
			LLMProvider: "openai",
			Model:       "gpt-4o",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ScheduleAssistResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, resp)
	}
}

func TestScheduleAssistResponseErrorField(t *testing.T) {
	t.Run("null when absent", func(t *testing.T) {
		resp := ScheduleAssistResponse{
			Type: "schedule/assist/response",
			Payload: ScheduleAssistResponsePayload{
				RequestID: "req-1",
				Kind:      "answer",
				Message:   "3 schedules run today",
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(data), `"error":null`) {
			t.Errorf("payload must carry \"error\":null when absent, got %s", data)
		}
	})

	t.Run("code round trip", func(t *testing.T) {
		code := "no_llm_provider"
		resp := ScheduleAssistResponse{
			Type: "schedule/assist/response",
			Payload: ScheduleAssistResponsePayload{
				RequestID: "req-2",
				Kind:      "error",
				Message:   "no LLM provider configured",
				Error:     &code,
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded ScheduleAssistResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Payload.Error == nil || *decoded.Payload.Error != code {
			t.Errorf("Error: got %v, want %q", decoded.Payload.Error, code)
		}
	})
}

func TestScheduleAssistRequestDecodesViaRegistry(t *testing.T) {
	raw := `{"type":"schedule/assist","requestId":"req-9","message":"pause the nightly summary","timezone":"Asia/Shanghai","clientNow":"2026-07-18T22:50:00+08:00","contextScheduleId":"sched-7","transcript":[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"}]}`

	msg, err := DecodeSessionInboundMessage(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	req, ok := msg.(*ScheduleAssistRequest)
	if !ok {
		t.Fatalf("decoded type: got %T, want *ScheduleAssistRequest", msg)
	}
	if msg.MsgType() != "schedule/assist" {
		t.Errorf("MsgType: got %q, want %q", msg.MsgType(), "schedule/assist")
	}
	if req.RequestID != "req-9" || req.Message != "pause the nightly summary" {
		t.Errorf("request fields: got %+v", req)
	}
	if req.Timezone != "Asia/Shanghai" || req.ClientNow != "2026-07-18T22:50:00+08:00" {
		t.Errorf("time fields: got %+v", req)
	}
	if req.ContextScheduleID != "sched-7" {
		t.Errorf("ContextScheduleID: got %q, want %q", req.ContextScheduleID, "sched-7")
	}
	if len(req.Transcript) != 2 || req.Transcript[0].Role != "user" || req.Transcript[1].Role != "assistant" {
		t.Errorf("Transcript: got %+v", req.Transcript)
	}
}

func TestScheduleAssistMsgTypes(t *testing.T) {
	if got := (ScheduleAssistRequest{}).MsgType(); got != "schedule/assist" {
		t.Errorf("ScheduleAssistRequest.MsgType: got %q, want %q", got, "schedule/assist")
	}
	if got := (ScheduleAssistResponse{}).MsgType(); got != "schedule/assist/response" {
		t.Errorf("ScheduleAssistResponse.MsgType: got %q, want %q", got, "schedule/assist/response")
	}
}
