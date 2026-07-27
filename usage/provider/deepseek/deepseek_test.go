package deepseek

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
	c, err := New(provider.Config{APIKey: "sk-deepseek-test", Endpoint: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const realResponse = `{
	"is_available": true,
	"balance_infos": [
		{
			"currency": "CNY",
			"total_balance": "9.90",
			"granted_balance": "1.00",
			"topped_up_balance": "8.90"
		}
	]
}`

func TestFetchRealFormat(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-deepseek-test" {
			t.Errorf("auth header = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("accept header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(realResponse))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Provider != "deepseek" {
		t.Errorf("provider = %q", snap.Provider)
	}
	if snap.Plan != nil {
		t.Errorf("plan = %+v, want nil", snap.Plan)
	}
	if len(snap.Quotas) != 2 {
		t.Fatalf("quotas = %d, want 2 (balance + granted)", len(snap.Quotas))
	}

	balance := snap.Quotas[0]
	if balance.Name != "balance_cny" {
		t.Errorf("quota[0].name = %q", balance.Name)
	}
	if balance.Label != "Balance (CNY)" {
		t.Errorf("quota[0].label = %q", balance.Label)
	}
	if balance.Used == nil || *balance.Used != 9.90 {
		t.Errorf("quota[0].used = %v, want 9.90", balance.Used)
	}
	if balance.Limit != nil {
		t.Errorf("quota[0].limit = %v, want nil", balance.Limit)
	}
	if balance.Unit != "CNY" {
		t.Errorf("quota[0].unit = %q", balance.Unit)
	}

	granted := snap.Quotas[1]
	if granted.Name != "granted_balance_cny" {
		t.Errorf("quota[1].name = %q", granted.Name)
	}
	if granted.Used == nil || *granted.Used != 1.00 {
		t.Errorf("quota[1].used = %v, want 1.00", granted.Used)
	}
}

func TestFetchZeroGrantedOmitted(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"is_available": true,
			"balance_infos": [
				{"currency": "CNY", "total_balance": "5.00", "granted_balance": "0.00", "topped_up_balance": "5.00"}
			]
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Quotas) != 1 {
		t.Fatalf("quotas = %d, want 1 (zero granted omitted)", len(snap.Quotas))
	}
}

func TestFetchNumericJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"is_available": true,
			"balance_infos": [
				{"currency": "USD", "total_balance": 12.5, "granted_balance": 0, "topped_up_balance": 12.5}
			]
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	q := snap.Quotas[0]
	if q.Used == nil || *q.Used != 12.5 {
		t.Errorf("used = %v, want 12.5", q.Used)
	}
	if q.Unit != "USD" {
		t.Errorf("unit = %q, want USD", q.Unit)
	}
}

func TestFetchMultiCurrency(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"is_available": true,
			"balance_infos": [
				{"currency": "CNY", "total_balance": "9.90", "granted_balance": "0.00", "topped_up_balance": "9.90"},
				{"currency": "USD", "total_balance": "1.50", "granted_balance": "0.00", "topped_up_balance": "1.50"}
			]
		}`))
	})

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Quotas) != 2 {
		t.Fatalf("quotas = %d, want 2 (one per currency)", len(snap.Quotas))
	}
	if snap.Quotas[0].Unit != "CNY" || snap.Quotas[1].Unit != "USD" {
		t.Errorf("units = %q, %q", snap.Quotas[0].Unit, snap.Quotas[1].Unit)
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
