package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
	usageprovider "github.com/WuErPing/solo/usage/provider"
)

// stubUsageProvider is a fake quota provider registered via the usage
// provider registry for tests.
type stubUsageProvider struct {
	name  string
	err   error
	calls *int32
}

func (p *stubUsageProvider) Name() string { return p.name }

func (p *stubUsageProvider) Fetch(_ context.Context) (*usageprovider.Snapshot, error) {
	if p.calls != nil {
		atomic.AddInt32(p.calls, 1)
	}
	if p.err != nil {
		return nil, p.err
	}
	return &usageprovider.Snapshot{
		Provider:  p.name,
		Plan:      &usageprovider.Plan{Name: "Pro", Tier: "pro"},
		Quotas:    []usageprovider.Quota{{Name: "weekly", Label: "Weekly usage"}},
		FetchedAt: time.Now(),
	}, nil
}

func registerStubUsageProvider(name string, err error, calls *int32) {
	usageprovider.Register(name, func(_ usageprovider.Config) (usageprovider.Provider, error) {
		return &stubUsageProvider{name: name, err: err, calls: calls}, nil
	})
}

func writeUsageConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestUsageQuotaServiceList(t *testing.T) {
	registerStubUsageProvider("stub-ok", nil, nil)
	registerStubUsageProvider("stub-fail", errors.New("boom"), nil)

	tests := []struct {
		name          string
		config        string // empty means the file does not exist
		wantSnapshots int
		wantErrKeys   []string
		wantTopErr    bool
	}{
		{
			name:          "success",
			config:        `{"providers":{"stub-ok":{"enabled":true,"apiKey":"x"}}}`,
			wantSnapshots: 1,
		},
		{
			name: "partial failure",
			config: `{"providers":{
				"stub-ok":{"enabled":true,"apiKey":"x"},
				"stub-fail":{"enabled":true,"apiKey":"x"}
			}}`,
			wantSnapshots: 1,
			wantErrKeys:   []string{"stub-fail"},
		},
		{
			name:          "all providers fail still returns a result",
			config:        `{"providers":{"stub-fail":{"enabled":true,"apiKey":"x"}}}`,
			wantSnapshots: 0,
			wantErrKeys:   []string{"stub-fail"},
		},
		{
			name:          "config missing is not an error",
			config:        "",
			wantSnapshots: 0,
		},
		{
			name:          "no enabled providers is not an error",
			config:        `{"providers":{"stub-ok":{"enabled":false,"apiKey":"x"}}}`,
			wantSnapshots: 0,
		},
		{
			name:          "unknown provider is reported per provider",
			config:        `{"providers":{"nope":{"enabled":true,"apiKey":"x"}}}`,
			wantSnapshots: 0,
			wantErrKeys:   []string{"nope"},
		},
		{
			name:       "malformed config is a top-level error",
			config:     `{not json`,
			wantTopErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "usage.json")
			if tc.config != "" {
				configPath = writeUsageConfig(t, tc.config)
			}
			svc := newUsageQuotaService(configPath, time.Minute)

			res := svc.list(context.Background(), false)

			if tc.wantTopErr {
				if res.topErr == nil {
					t.Fatal("expected a top-level error")
				}
				return
			}
			if res.topErr != nil {
				t.Fatalf("unexpected top-level error: %v", res.topErr)
			}
			if len(res.snapshots) != tc.wantSnapshots {
				t.Errorf("snapshots: got %d, want %d", len(res.snapshots), tc.wantSnapshots)
			}
			for _, key := range tc.wantErrKeys {
				if _, ok := res.errs[key]; !ok {
					t.Errorf("expected per-provider error for %q, got %v", key, res.errs)
				}
			}
			if len(tc.wantErrKeys) == 0 && len(res.errs) != 0 {
				t.Errorf("unexpected per-provider errors: %v", res.errs)
			}
			if res.cachedAt.IsZero() {
				t.Error("expected cachedAt to be set")
			}
		})
	}
}

func TestUsageQuotaServiceCache(t *testing.T) {
	var calls int32
	registerStubUsageProvider("stub-cache", nil, &calls)
	configPath := writeUsageConfig(t, `{"providers":{"stub-cache":{"enabled":true,"apiKey":"x"}}}`)
	svc := newUsageQuotaService(configPath, time.Minute)

	svc.list(context.Background(), false)
	svc.list(context.Background(), false)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected cached second call, fetch count = %d", got)
	}

	// forceRefresh bypasses the cache.
	svc.list(context.Background(), true)
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected forceRefresh to refetch, fetch count = %d", got)
	}

	// An expired entry is refetched.
	expiring := newUsageQuotaService(configPath, 20*time.Millisecond)
	expiring.list(context.Background(), false)
	time.Sleep(30 * time.Millisecond)
	expiring.list(context.Background(), false)
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("expected refetch after TTL expiry, fetch count = %d", got)
	}
}

func TestHandleUsageQuotaList(t *testing.T) {
	registerStubUsageProvider("stub-ws", nil, nil)
	configPath := writeUsageConfig(t, `{"providers":{"stub-ws":{"enabled":true,"apiKey":"x"}}}`)

	old := usageQuotas
	usageQuotas = newUsageQuotaService(configPath, time.Minute)
	defer func() { usageQuotas = old }()

	_, ts := newTestWSServer(t)
	conn := dialAndHello(t, ts.URL, "client-usage-quota-list")
	defer conn.Close()
	readInitialMessages(t, conn)

	conn.WriteJSON(protocol.WSInboundMessage{
		Type: "session",
		Message: mustMarshal(map[string]interface{}{
			"type":      "usage/quota/list",
			"requestId": "req-usage-1",
		}),
	})

	resp := readUntilType(t, conn, "usage/quota/list/response")
	payload := decodeSessionPayload[protocol.UsageQuotaListResponsePayload](t, resp)
	if payload.Error != nil {
		t.Fatalf("unexpected error: %s", *payload.Error)
	}
	if len(payload.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(payload.Snapshots))
	}
	snap := payload.Snapshots[0]
	if snap.Provider != "stub-ws" {
		t.Errorf("provider mismatch: got %q", snap.Provider)
	}
	if snap.Plan == nil || snap.Plan.Name != "Pro" {
		t.Errorf("plan mismatch: got %+v", snap.Plan)
	}
	if len(snap.Quotas) != 1 || snap.Quotas[0].Name != "weekly" {
		t.Errorf("quotas mismatch: got %+v", snap.Quotas)
	}
	if snap.FetchedAt == "" || payload.CachedAt == "" {
		t.Errorf("expected RFC3339 timestamps, got fetchedAt=%q cachedAt=%q", snap.FetchedAt, payload.CachedAt)
	}
	fmt.Println("snapshot:", snap.Provider, "quotas:", len(snap.Quotas))
}
