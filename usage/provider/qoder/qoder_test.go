package qoder

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
		APIKey:   "qoder-test-key",
		Endpoint: srv.URL,
		Extra:    map[string]string{"organizationId": "org_xxx"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const realResponse = `{
	"resourcePackages": [
		{
			"id": "pkg-001",
			"name": "Enterprise Annual Pack",
			"source": "purchased",
			"status": "active",
			"activatedAt": "2025-01-01T00:00:00Z",
			"expiresAt": "2099-01-01T00:00:00Z",
			"limitValue": 3000.0,
			"usedValue": 800.0,
			"remainingValue": 2200.0,
			"unit": "credits"
		},
		{
			"id": "pkg-002",
			"name": "Trial Pack",
			"source": "trial",
			"status": "exhausted",
			"expiresAt": "2099-09-15T00:00:00Z",
			"limitValue": 500.0,
			"usedValue": 500.0,
			"remainingValue": 0.0,
			"unit": "credits"
		}
	],
	"maxResults": 20
}`

func TestFetchRealFormat(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/organizations/org_xxx/resource-packages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer qoder-test-key" {
			t.Errorf("auth header = %q", got)
		}
		if got := r.URL.Query().Get("status"); got != "active" {
			t.Errorf("status param = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(realResponse))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Provider != "qoder" {
		t.Errorf("provider = %q", snap.Provider)
	}
	// exhausted package filtered out
	if len(snap.Quotas) != 1 {
		t.Fatalf("quotas = %d, want 1 (exhausted filtered)", len(snap.Quotas))
	}

	q := snap.Quotas[0]
	if q.Name != "pkg_pkg-001" {
		t.Errorf("name = %q", q.Name)
	}
	if q.Label != "Enterprise Annual Pack" {
		t.Errorf("label = %q", q.Label)
	}
	if q.Used == nil || *q.Used != 800 {
		t.Errorf("used = %v, want 800", q.Used)
	}
	if q.Limit == nil || *q.Limit != 3000 {
		t.Errorf("limit = %v, want 3000", q.Limit)
	}
	if q.UsedPct == nil || *q.UsedPct != 800.0/3000.0*100 {
		t.Errorf("usedPct = %v, want %v", q.UsedPct, 800.0/3000.0*100)
	}
	if q.Unit != "credits" {
		t.Errorf("unit = %q", q.Unit)
	}
	if q.ResetAt == nil {
		t.Error("resetAt is nil")
	}
	if q.ResetIn == "" {
		t.Error("resetIn is empty")
	}
}

func TestFetchExpiredFiltered(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		// status=active but expiresAt in the past → not truly usable
		w.Write([]byte(`{
			"resourcePackages": [
				{"id": "pkg-old", "name": "Old Pack", "status": "active", "expiresAt": "2020-01-01T00:00:00Z",
				 "limitValue": 100.0, "usedValue": 10.0, "remainingValue": 90.0, "unit": "credits"}
			],
			"maxResults": 20
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Quotas) != 0 {
		t.Errorf("quotas = %d, want 0 (expired filtered)", len(snap.Quotas))
	}
}

func TestFetchPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("nextToken") == "" {
			w.Write([]byte(`{
				"resourcePackages": [
					{"id": "pkg-1", "name": "Pack 1", "status": "active", "expiresAt": "2099-01-01T00:00:00Z",
					 "limitValue": 100.0, "usedValue": 50.0, "remainingValue": 50.0, "unit": "credits"}
				],
				"maxResults": 1,
				"nextToken": "page2"
			}`))
			return
		}
		if got := r.URL.Query().Get("nextToken"); got != "page2" {
			t.Errorf("nextToken = %q, want page2", got)
		}
		w.Write([]byte(`{
			"resourcePackages": [
				{"id": "pkg-2", "name": "Pack 2", "status": "active", "expiresAt": "2099-06-01T00:00:00Z",
				 "limitValue": 200.0, "usedValue": 100.0, "remainingValue": 100.0, "unit": "credits"}
			],
			"maxResults": 1
		}`))
	}))
	t.Cleanup(srv.Close)
	c, err := New(provider.Config{
		APIKey:   "qoder-test-key",
		Endpoint: srv.URL,
		Extra:    map[string]string{"organizationId": "org_xxx"},
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Quotas) != 2 {
		t.Fatalf("quotas = %d, want 2 (two pages)", len(snap.Quotas))
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

func TestFetchForbidden(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestFetchOrgNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(provider.Config{Extra: map[string]string{"organizationId": "org_xxx"}})
	if err == nil {
		t.Fatal("expected error for empty apiKey")
	}
}

func TestNewRequiresOrganizationID(t *testing.T) {
	_, err := New(provider.Config{APIKey: "qoder-test-key"})
	if err == nil {
		t.Fatal("expected error for missing organizationId")
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

func TestComputePct(t *testing.T) {
	used, limit := 800.0, 3000.0
	q := &provider.Quota{Used: &used, Limit: &limit}
	computePct(q)
	want := 800.0 / 3000.0 * 100
	if q.UsedPct == nil || *q.UsedPct != want {
		t.Errorf("usedPct = %v, want %v", q.UsedPct, want)
	}

	zero := 0.0
	q = &provider.Quota{Used: &used, Limit: &zero}
	computePct(q)
	if q.UsedPct != nil {
		t.Errorf("usedPct = %v, want nil for zero limit", q.UsedPct)
	}
}

// --- personal (cookie) mode ---

func newPersonalTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(provider.Config{
		Endpoint: srv.URL,
		Extra:    map[string]string{"cookie": "session=abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Real response pasted from the web console API (personal Pro account, 2026-07).
const realPersonalResponse = `{
	"user_id": "ee479351-d7b8-40b7-9497-9b25deb69983",
	"quota_key": "big_model_credits",
	"status": "active",
	"plan_quota": {
		"quota_summary": {"used_value": 370, "limit_value": 6000, "remaining_value": 5630, "usage_percentage": 7, "unit": "credits"},
		"quota_detail": [{"id": "a6542750", "limit_value": 6000, "used_value": 370, "remaining_value": 5630, "unit": "credits", "is_active": true, "usage_percentage": 7, "expires_at": 0, "source": "PLAN", "status": "ACTIVE"}]
	},
	"resource_package_quota": {
		"quota_summary": {"used_value": 0, "limit_value": 0, "remaining_value": 0, "usage_percentage": 0, "unit": "credits"},
		"quota_detail": null
	},
	"total_quota": {
		"quota_summary": {"used_value": 370, "limit_value": 6000, "remaining_value": 5630, "usage_percentage": 7, "unit": "credits"},
		"quota_detail": [{"id": "a6542750", "limit_value": 6000, "used_value": 370, "remaining_value": 5630, "unit": "credits", "is_active": true, "usage_percentage": 7, "expires_at": 0, "source": "PLAN", "status": "ACTIVE"}]
	},
	"lastResetAt": 1768966445071,
	"nextResetAt": 1787587200000
}`

func TestFetchPersonalRealFormat(t *testing.T) {
	c := newPersonalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/me/usages/big_model_credits" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Cookie"); got != "session=abc" {
			t.Errorf("cookie header = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("authorization header should be empty in cookie mode, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(realPersonalResponse))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Provider != "qoder" {
		t.Errorf("provider = %q", snap.Provider)
	}
	// resource_package_quota is 0/0 → skipped
	if len(snap.Quotas) != 1 {
		t.Fatalf("quotas = %d, want 1 (empty resource package skipped)", len(snap.Quotas))
	}

	q := snap.Quotas[0]
	if q.Name != "plan_credits" {
		t.Errorf("name = %q", q.Name)
	}
	if q.Label != "Plan Credits" {
		t.Errorf("label = %q", q.Label)
	}
	if q.Used == nil || *q.Used != 370 {
		t.Errorf("used = %v, want 370", q.Used)
	}
	if q.Limit == nil || *q.Limit != 6000 {
		t.Errorf("limit = %v, want 6000", q.Limit)
	}
	if q.UsedPct == nil || *q.UsedPct != 7 {
		t.Errorf("usedPct = %v, want 7", q.UsedPct)
	}
	if q.Unit != "credits" {
		t.Errorf("unit = %q", q.Unit)
	}
	if q.ResetAt == nil {
		t.Fatal("resetAt is nil")
	}
	if got := q.ResetAt.UnixMilli(); got != 1787587200000 {
		t.Errorf("resetAt = %v, want 1787587200000", got)
	}
	if q.ResetIn == "" {
		t.Error("resetIn is empty")
	}
}

func TestFetchPersonalKeepsNonZeroResourcePackage(t *testing.T) {
	c := newPersonalTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"quota_key": "big_model_credits",
			"status": "active",
			"plan_quota": {"quota_summary": {"used_value": 370, "limit_value": 6000, "usage_percentage": 7, "unit": "credits"}},
			"resource_package_quota": {"quota_summary": {"used_value": 10, "limit_value": 100, "usage_percentage": 10, "unit": "credits"}},
			"nextResetAt": 1787587200000
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Quotas) != 2 {
		t.Fatalf("quotas = %d, want 2", len(snap.Quotas))
	}
	if snap.Quotas[1].Name != "resource_package_credits" {
		t.Errorf("quota[1].name = %q", snap.Quotas[1].Name)
	}
}

func TestFetchPersonalUnauthorized(t *testing.T) {
	c := newPersonalTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestFetchPersonalAntiBotHTML(t *testing.T) {
	c := newPersonalTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<script>sessionStorage.x5referer=...punish...</script>`))
	})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for anti-bot HTML response")
	}
}

func TestNewCookieModeTakesPrecedence(t *testing.T) {
	c, err := New(provider.Config{
		APIKey: "ignored-key",
		Extra:  map[string]string{"cookie": "session=abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.cookie != "session=abc" {
		t.Errorf("cookie = %q", c.cookie)
	}
	if c.endpoint != personalEndpoint {
		t.Errorf("endpoint = %q, want %q", c.endpoint, personalEndpoint)
	}
}
