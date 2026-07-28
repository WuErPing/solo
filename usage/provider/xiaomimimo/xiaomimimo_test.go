package xiaomimimo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

// Real response pasted from the console API (Lite annual plan, 2026-07).
const realResponse = `{
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

func TestFetchRealFormat(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tokenPlan/usage" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Cookie"); got != "api-platform_serviceToken=tok; userId=123" {
			t.Errorf("cookie header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(realResponse))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Provider != "xiaomimimo" {
		t.Errorf("provider = %q", snap.Provider)
	}
	// compensation_total_token (0/0) skipped
	if len(snap.Quotas) != 2 {
		t.Fatalf("quotas = %d, want 2 (zero compensation skipped)", len(snap.Quotas))
	}

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

	plan := snap.Quotas[1]
	if plan.Name != "plan_plan_total_token" {
		t.Errorf("quota[1].name = %q", plan.Name)
	}
	if plan.Label != "Plan Total" {
		t.Errorf("quota[1].label = %q", plan.Label)
	}
	if plan.UsedPct == nil || *plan.UsedPct != 8.0 {
		t.Errorf("quota[1].usedPct = %v, want 8.0", plan.UsedPct)
	}
}

func TestFetchKeepsNonZeroCompensation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"code": 0, "message": "",
			"data": {
				"monthUsage": {"percent": 0, "items": []},
				"usage": {"percent": 0.5, "items": [
					{"name": "plan_total_token", "used": 100, "limit": 200, "percent": 0.5},
					{"name": "compensation_total_token", "used": 10, "limit": 50, "percent": 0.2}
				]}
			}
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Quotas) != 2 {
		t.Fatalf("quotas = %d, want 2 (non-zero compensation kept)", len(snap.Quotas))
	}
	if snap.Quotas[1].Label != "Compensation" {
		t.Errorf("quota[1].label = %q", snap.Quotas[1].Label)
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
