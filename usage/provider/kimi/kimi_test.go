package kimi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/WuErPing/solo/usage/provider"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(provider.Config{APIKey: "sk-kimi-test", Endpoint: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const realResponse = `{
	"user": {
		"userId": "cnsjeqg3r07068eeuki0",
		"region": "REGION_CN",
		"membership": {"level": "LEVEL_INTERMEDIATE"}
	},
	"usage": {
		"limit": "100",
		"used": "12",
		"remaining": "88",
		"resetTime": "2026-07-28T06:15:37.520232Z"
	},
	"limits": [
		{
			"window": {"duration": 300, "timeUnit": "TIME_UNIT_MINUTE"},
			"detail": {
				"limit": "100",
				"used": "3",
				"remaining": "97",
				"resetTime": "2026-07-27T15:15:37.520232Z"
			}
		}
	],
	"parallel": {"limit": "20"}
}`

func TestFetchRealFormat(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/usages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-kimi-test" {
			t.Errorf("auth header = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("user-agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(realResponse))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Provider != "kimi" {
		t.Errorf("provider = %q", snap.Provider)
	}
	if snap.Plan == nil || snap.Plan.Name != "Intermediate" {
		t.Errorf("plan = %+v, want Intermediate", snap.Plan)
	}
	if len(snap.Quotas) != 3 {
		t.Fatalf("quotas = %d, want 3 (weekly + rate + parallel)", len(snap.Quotas))
	}

	weekly := snap.Quotas[0]
	if weekly.Name != "weekly_usage" {
		t.Errorf("quota[0].name = %q", weekly.Name)
	}
	if weekly.Used == nil || *weekly.Used != 12 {
		t.Errorf("quota[0].used = %v, want 12", weekly.Used)
	}
	if weekly.Limit == nil || *weekly.Limit != 100 {
		t.Errorf("quota[0].limit = %v, want 100", weekly.Limit)
	}
	if weekly.UsedPct == nil || *weekly.UsedPct != 12.0 {
		t.Errorf("quota[0].usedPct = %v, want 12.0", weekly.UsedPct)
	}
	if weekly.ResetAt == nil {
		t.Error("quota[0].resetAt is nil")
	}
	if weekly.ResetIn == "" {
		t.Error("quota[0].resetIn is empty")
	}

	rate := snap.Quotas[1]
	if rate.Name != "5h_limit" {
		t.Errorf("quota[1].name = %q, want 5h_limit", rate.Name)
	}
	if rate.Label != "5h" {
		t.Errorf("quota[1].label = %q, want 5h", rate.Label)
	}
	if rate.UsedPct == nil || *rate.UsedPct != 3.0 {
		t.Errorf("quota[1].usedPct = %v, want 3.0", rate.UsedPct)
	}

	parallel := snap.Quotas[2]
	if parallel.Name != "parallel" {
		t.Errorf("quota[2].name = %q", parallel.Name)
	}
	if parallel.Limit == nil || *parallel.Limit != 20 {
		t.Errorf("quota[2].limit = %v, want 20", parallel.Limit)
	}
}

func TestFetchNumericJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"usage": {"limit": 200, "used": 50, "remaining": 150, "resetTime": "2026-08-01T00:00:00Z"},
			"limits": []
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	q := snap.Quotas[0]
	if q.Used == nil || *q.Used != 50 {
		t.Errorf("used = %v, want 50", q.Used)
	}
	if q.UsedPct == nil || *q.UsedPct != 25.0 {
		t.Errorf("usedPct = %v, want 25.0", q.UsedPct)
	}
}

func TestFetchUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestFetchRateLimited(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 429")
	}
}

func TestFetchInvalidFormat(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"unknown": true}`))
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for unrecognized format")
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(provider.Config{})
	if err == nil {
		t.Fatal("expected error for empty apiKey")
	}
}

func TestNormalizeTimeUnit(t *testing.T) {
	cases := []struct{ in, want string }{
		{"TIME_UNIT_MINUTE", "MINUTE"},
		{"TIME_UNIT_HOUR", "HOUR"},
		{"MINUTE", "MINUTE"},
	}
	for _, tc := range cases {
		if got := normalizeTimeUnit(tc.in); got != tc.want {
			t.Errorf("normalizeTimeUnit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeLevel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"LEVEL_INTERMEDIATE", "Intermediate"},
		{"LEVEL_BASIC", "Basic"},
	}
	for _, tc := range cases {
		if got := normalizeLevel(tc.in); got != tc.want {
			t.Errorf("normalizeLevel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWindowLabel(t *testing.T) {
	cases := []struct {
		duration float64
		unit     string
		want     string
	}{
		{300, "MINUTE", "5h"},
		{60, "MINUTE", "1h"},
		{30, "MINUTE", "30m"},
		{5, "HOUR", "5h"},
		{1, "DAY", "1d"},
		{1, "MONTH", "1mo"},
	}
	for _, tc := range cases {
		if got := windowLabel(tc.duration, tc.unit); got != tc.want {
			t.Errorf("windowLabel(%v, %q) = %q, want %q", tc.duration, tc.unit, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{25*time.Hour + 30*time.Minute, "1d 1h 30m"},
		{2 * time.Hour, "2h 0m"},
		{5 * time.Minute, "5m"},
		{0, "0m"},
		{-time.Hour, "0m"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.d); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
