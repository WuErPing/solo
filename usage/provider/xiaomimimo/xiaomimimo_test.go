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

	// currentPeriodEnd 2027-07-09 → quotas reset on the 9th of each month (UTC).
	wantStart, wantEnd := monthWindow(time.Now(), 9)

	month := snap.Quotas[0]
	if month.Name != "month_month_total_token" {
		t.Errorf("quota[0].name = %q", month.Name)
	}
	if month.Label != "Monthly Usage" {
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

func TestMonthWindow(t *testing.T) {
	utc := func(s string) time.Time {
		t.Helper()
		tt, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			t.Fatal(err)
		}
		return tt
	}
	cases := []struct {
		name      string
		now       string
		anchorDay int
		wantStart string
		wantEnd   string
	}{
		{"mid-cycle", "2026-07-30 12:00:00", 9, "2026-07-09 00:00:00", "2026-08-09 00:00:00"},
		{"before anchor", "2026-07-05 08:00:00", 9, "2026-06-09 00:00:00", "2026-07-09 00:00:00"},
		{"exactly on anchor", "2026-07-09 00:00:00", 9, "2026-07-09 00:00:00", "2026-08-09 00:00:00"},
		{"clamp to february", "2026-02-15 00:00:00", 31, "2026-01-31 00:00:00", "2026-02-28 00:00:00"},
		{"clamp end side", "2026-03-31 12:00:00", 31, "2026-03-31 00:00:00", "2026-04-30 00:00:00"},
		{"year boundary", "2026-01-05 00:00:00", 31, "2025-12-31 00:00:00", "2026-01-31 00:00:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := monthWindow(utc(tc.now), tc.anchorDay)
			if !start.Equal(utc(tc.wantStart)) {
				t.Errorf("start = %v, want %v", start, tc.wantStart)
			}
			if !end.Equal(utc(tc.wantEnd)) {
				t.Errorf("end = %v, want %v", end, tc.wantEnd)
			}
		})
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
		{"month_total_token", "Monthly Usage"},
		{"plan_total_token", "Plan Total"},
		{"compensation_total_token", "Compensation"},
		{"some_future_item", "Some future item"},
	}
	for _, tc := range cases {
		if got := itemLabel(tc.in); got != tc.want {
			t.Errorf("itemLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
