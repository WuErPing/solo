package xiaomimimo

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
	c, err := New(provider.Config{
		Endpoint: srv.URL,
		Extra:    map[string]string{"cookie": "api-platform_serviceToken=tok; userId=123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Real responses pasted from the console API (Lite annual plan, 2026-07).
const realUsageResponse = `{
	"code": 0,
	"message": "",
	"data": {
		"monthUsage": {
			"percent": 0.0782,
			"items": [
				{"name": "month_total_token", "used": 3846407684, "limit": 49200000000, "percent": 0.0782}
			]
		},
		"usage": {
			"percent": 0.08,
			"items": [
				{"name": "plan_total_token", "used": 3846407684, "limit": 49200000000, "percent": 0.08},
				{"name": "compensation_total_token", "used": 0, "limit": 0, "percent": 0}
			]
		}
	}
}`

const realDetailResponse = `{
	"code": 0,
	"message": "",
	"data": {
		"planCode": "lite:year",
		"planName": "Lite",
		"currentPeriodEnd": "2027-07-09 23:59:59",
		"expired": false,
		"enableAutoRenew": true
	}
}`

func usageAndDetailHandler(t *testing.T, detailStatus int, detailBody string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "api-platform_serviceToken=tok; userId=123" {
			t.Errorf("cookie header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/tokenPlan/usage":
			w.Write([]byte(realUsageResponse))
		case "/tokenPlan/detail":
			w.WriteHeader(detailStatus)
			w.Write([]byte(detailBody))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}
}

func TestFetchRealFormat(t *testing.T) {
	c := newTestClient(t, usageAndDetailHandler(t, http.StatusOK, realDetailResponse))

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Provider != "xiaomimimo" {
		t.Errorf("provider = %q", snap.Provider)
	}
	// usage section (plan_total/compensation) is not exposed
	if len(snap.Quotas) != 1 {
		t.Fatalf("quotas = %d, want 1 (only monthUsage exposed)", len(snap.Quotas))
	}

	// lite:year plan → the quota covers the whole annual period ending at
	// currentPeriodEnd (2027-07-09 23:59:59 UTC).
	wantEnd := time.Date(2027, 7, 9, 23, 59, 59, 0, time.UTC)
	wantStart := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	month := snap.Quotas[0]
	if month.Name != "month_month_total_token" {
		t.Errorf("quota[0].name = %q", month.Name)
	}
	if month.Label != "Annual Usage" {
		t.Errorf("quota[0].label = %q", month.Label)
	}
	if month.Used == nil || *month.Used != 3846407684 {
		t.Errorf("quota[0].used = %v", month.Used)
	}
	if month.Limit == nil || *month.Limit != 49200000000 {
		t.Errorf("quota[0].limit = %v", month.Limit)
	}
	if month.UsedPct == nil || *month.UsedPct < 7.81 || *month.UsedPct > 7.83 {
		t.Errorf("quota[0].usedPct = %v, want ~7.82", month.UsedPct)
	}
	if month.Unit != "credits" {
		t.Errorf("quota[0].unit = %q", month.Unit)
	}
	if month.WindowStart == nil || !wantStart.Equal(*month.WindowStart) {
		t.Errorf("quota[0].windowStart = %v, want %v", month.WindowStart, wantStart)
	}
	if month.ResetAt == nil || !wantEnd.Equal(*month.ResetAt) {
		t.Errorf("quota[0].resetAt = %v, want %v", month.ResetAt, wantEnd)
	}
	if month.ResetIn == "" {
		t.Error("quota[0].resetIn is empty")
	}
}

func TestFetchDetailFailureDegrades(t *testing.T) {
	c := newTestClient(t, usageAndDetailHandler(t, http.StatusInternalServerError, `{}`))

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("detail failure must not fail the fetch: %v", err)
	}
	for i, q := range snap.Quotas {
		if q.WindowStart != nil || q.ResetAt != nil {
			t.Errorf("quota[%d] window = %v/%v, want nil (detail unavailable)", i, q.WindowStart, q.ResetAt)
		}
	}
}

func TestFetchExpiredPlanNoWindow(t *testing.T) {
	expired := `{"code":0,"message":"","data":{"planCode":"lite:year","currentPeriodEnd":"2025-07-09 23:59:59","expired":true}}`
	c := newTestClient(t, usageAndDetailHandler(t, http.StatusOK, expired))

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range snap.Quotas {
		if q.WindowStart != nil || q.ResetAt != nil {
			t.Errorf("quota[%d] window = %v/%v, want nil (plan expired)", i, q.WindowStart, q.ResetAt)
		}
	}
}

func TestPlanPeriod(t *testing.T) {
	end := time.Date(2027, 7, 9, 23, 59, 59, 0, time.UTC)

	start, label, ok := planPeriod("lite:year", end)
	if !ok {
		t.Fatal("lite:year not recognized")
	}
	if want := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("year start = %v, want %v", start, want)
	}
	if label != "Annual Usage" {
		t.Errorf("year label = %q", label)
	}

	monthEnd := time.Date(2026, 8, 9, 23, 59, 59, 0, time.UTC)
	start, label, ok = planPeriod("pro:month", monthEnd)
	if !ok {
		t.Fatal("pro:month not recognized")
	}
	if want := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("month start = %v, want %v", start, want)
	}
	if label != "Monthly Usage" {
		t.Errorf("month label = %q", label)
	}

	if _, _, ok := planPeriod("lite:lifetime", end); ok {
		t.Error("unknown plan code must yield ok=false")
	}
}

func TestFetchUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":401,"loginUrl":"https://account.xiaomi.com/pass/serviceLogin?..."}`))
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestFetchAPIErrorCode(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"code": 500, "message": "internal error", "data": {}}`))
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for non-zero code")
	}
}

func TestFetchAllZeroItems(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"code": 0, "message": "",
			"data": {
				"monthUsage": {"percent": 0, "items": [{"name": "month_total_token", "used": 0, "limit": 0, "percent": 0}]},
				"usage": {"percent": 0, "items": []}
			}
		}`))
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error when all items are zero (likely no subscription)")
	}
}

func TestNewRequiresCookie(t *testing.T) {
	_, err := New(provider.Config{})
	if err == nil {
		t.Fatal("expected error for missing cookie")
	}
}

func TestItemLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"month_total_token", "Plan Usage"},
		{"some_future_item", "Some future item"},
	}
	for _, tc := range cases {
		if got := itemLabel(tc.in); got != tc.want {
			t.Errorf("itemLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
