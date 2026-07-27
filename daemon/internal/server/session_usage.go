package server

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/WuErPing/solo/protocol"
	usageconfig "github.com/WuErPing/solo/usage/config"
	usageprovider "github.com/WuErPing/solo/usage/provider"
	_ "github.com/WuErPing/solo/usage/provider/kimi" // register the kimi provider
)

const (
	// usageQuotaCacheTTL is how long a fetched snapshot set is reused before
	// providers are queried again.
	usageQuotaCacheTTL = 60 * time.Second
	// usageQuotaFetchTimeout bounds a single refresh across all providers.
	usageQuotaFetchTimeout = 15 * time.Second
)

// usageQuotas is the process-wide quota snapshot cache shared by all sessions.
var usageQuotas = newUsageQuotaService(usageconfig.DefaultPath(), usageQuotaCacheTTL)

// usageQuotaResult is the outcome of one quota refresh (or a cache hit).
type usageQuotaResult struct {
	snapshots []protocol.UsageQuotaSnapshot
	errs      map[string]string // per-provider errors, keyed by provider name
	cachedAt  time.Time
	topErr    error // fatal error (e.g. config parse failure); not cached per provider
}

// usageQuotaService fetches provider quota snapshots and caches the result for
// a short TTL. A nil cache entry or an expired one triggers a refresh;
// concurrent refreshes are coalesced via singleflight.
type usageQuotaService struct {
	configPath string
	ttl        time.Duration
	flight     singleflight.Group

	mu     sync.Mutex
	cached *usageQuotaResult
}

func newUsageQuotaService(configPath string, ttl time.Duration) *usageQuotaService {
	return &usageQuotaService{configPath: configPath, ttl: ttl}
}

func (u *usageQuotaService) list(ctx context.Context, forceRefresh bool) usageQuotaResult {
	u.mu.Lock()
	cached := u.cached
	u.mu.Unlock()
	if !forceRefresh && cached != nil && time.Since(cached.cachedAt) < u.ttl {
		return *cached
	}

	v, _, _ := u.flight.Do("refresh", func() (interface{}, error) {
		r := u.fetch(ctx)
		u.mu.Lock()
		u.cached = &r
		u.mu.Unlock()
		return r, nil
	})
	return v.(usageQuotaResult)
}

// fetch loads the usage config and queries every enabled provider
// concurrently. A missing config file or zero enabled providers is not an
// error — it yields an empty snapshot set.
func (u *usageQuotaService) fetch(ctx context.Context) usageQuotaResult {
	res := usageQuotaResult{cachedAt: time.Now()}

	cfg, err := usageconfig.Load(u.configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			res.topErr = err
		}
		return res
	}

	names := cfg.EnabledProviders()
	if len(names) == 0 {
		return res
	}
	sort.Strings(names)

	ctx, cancel := context.WithTimeout(ctx, usageQuotaFetchTimeout)
	defer cancel()

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs = map[string]string{}
	)
	for _, name := range names {
		pcfg, ok := cfg.ToProviderConfig(name)
		if !ok {
			continue
		}
		p, err := usageprovider.Create(name, pcfg)
		if err != nil {
			errs[name] = err.Error()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			snap, err := p.Fetch(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[name] = err.Error()
				return
			}
			res.snapshots = append(res.snapshots, toProtocolUsageSnapshot(snap))
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		res.errs = errs
	}
	sort.Slice(res.snapshots, func(i, j int) bool { return res.snapshots[i].Provider < res.snapshots[j].Provider })
	return res
}

func toProtocolUsageSnapshot(snap *usageprovider.Snapshot) protocol.UsageQuotaSnapshot {
	out := protocol.UsageQuotaSnapshot{
		Provider:  snap.Provider,
		FetchedAt: snap.FetchedAt.Format(time.RFC3339),
	}
	if snap.Plan != nil {
		out.Plan = &protocol.UsagePlan{Name: snap.Plan.Name, Tier: snap.Plan.Tier}
	}
	out.Quotas = make([]protocol.UsageQuota, 0, len(snap.Quotas))
	for _, q := range snap.Quotas {
		quota := protocol.UsageQuota{
			Name:    q.Name,
			Label:   q.Label,
			Used:    q.Used,
			Limit:   q.Limit,
			UsedPct: q.UsedPct,
			Unit:    q.Unit,
			ResetIn: q.ResetIn,
		}
		if q.ResetAt != nil {
			s := q.ResetAt.Format(time.RFC3339)
			quota.ResetAt = &s
		}
		out.Quotas = append(out.Quotas, quota)
	}
	return out
}

// --- Session handler ---

func (s *Session) handleUsageQuotaList(m *protocol.UsageQuotaListRequest) {
	res := usageQuotas.list(context.Background(), m.ForceRefresh)
	errMsg := ""
	if res.topErr != nil {
		s.logger.Error("usage quota list failed", "error", res.topErr)
		errMsg = res.topErr.Error()
	}
	s.sendUsageQuotaListResponse(m.RequestID, res, errMsg)
}

func (s *Session) sendUsageQuotaListResponse(requestID string, res usageQuotaResult, errMsg string) {
	snapshots := res.snapshots
	if snapshots == nil {
		snapshots = []protocol.UsageQuotaSnapshot{}
	}
	payload := protocol.UsageQuotaListResponsePayload{
		RequestID: requestID,
		Snapshots: snapshots,
		Errors:    res.errs,
		CachedAt:  res.cachedAt.Format(time.RFC3339),
	}
	if errMsg != "" {
		payload.Error = &errMsg
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.UsageQuotaListResponse{Type: "usage/quota/list/response", Payload: payload}))
}
