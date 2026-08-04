package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestUsageQuotaListRequestOmitsForceRefresh(t *testing.T) {
	req := UsageQuotaListRequest{Type: "usage/quota/list", RequestID: "req-2"}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), `"forceRefresh"`) {
		t.Errorf("expected forceRefresh to be omitted, got %s", data)
	}
}

func TestUsageQuotaListRequestDecode(t *testing.T) {
	msg, err := DecodeSessionInboundMessage(json.RawMessage(`{"type":"usage/quota/list","requestId":"req-3","forceRefresh":true}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	req, ok := msg.(*UsageQuotaListRequest)
	if !ok {
		t.Fatalf("expected *UsageQuotaListRequest, got %T", msg)
	}
	if req.RequestID != "req-3" || !req.ForceRefresh {
		t.Errorf("decoded request mismatch: %+v", req)
	}
}

func TestUsageQuotaListResponseRoundTrip(t *testing.T) {
	used := 42.0
	limit := 100.0
	pct := 42.0
	resetAt := "2026-07-28T00:00:00Z"
	resp := UsageQuotaListResponse{
		Type: "usage/quota/list/response",
		Payload: UsageQuotaListResponsePayload{
			RequestID: "req-1",
			Snapshots: []UsageQuotaSnapshot{
				{
					Provider: "kimi",
					Plan:     &UsagePlan{Name: "Pro", Tier: "pro"},
					Quotas: []UsageQuota{
						{
							Name:    "weekly",
							Label:   "Weekly usage",
							Used:    &used,
							Limit:   &limit,
							UsedPct: &pct,
							Unit:    "%",
							ResetAt: &resetAt,
							ResetIn: "3d",
						},
					},
					FetchedAt: "2026-07-27T15:00:00Z",
				},
			},
			Errors:   map[string]string{"other": "unauthorized"},
			CachedAt: "2026-07-27T15:00:00Z",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded UsageQuotaListResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", decoded, resp)
	}
}

func TestUsageQuotaListResponseNullableFields(t *testing.T) {
	resp := UsageQuotaListResponse{
		Type: "usage/quota/list/response",
		Payload: UsageQuotaListResponsePayload{
			RequestID: "req-1",
			Snapshots: []UsageQuotaSnapshot{
				{
					Provider:  "kimi",
					Quotas:    []UsageQuota{{Name: "weekly", Label: "Weekly usage"}},
					FetchedAt: "2026-07-27T15:00:00Z",
				},
			},
			CachedAt: "2026-07-27T15:00:00Z",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	// Nullable numbers and plan/resetAt must serialize as explicit null.
	for _, key := range []string{`"used":null`, `"limit":null`, `"usedPct":null`, `"resetAt":null`, `"plan":null`, `"error":null`} {
		if !strings.Contains(s, key) {
			t.Errorf("expected %s in %s", key, s)
		}
	}
	// errors is omitted when empty.
	if strings.Contains(s, `"errors"`) {
		t.Errorf("expected errors to be omitted, got %s", s)
	}
}
