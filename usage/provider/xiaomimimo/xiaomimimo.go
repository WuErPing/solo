package xiaomimimo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/WuErPing/solo/usage/provider"
)

const (
	defaultEndpoint = "https://platform.xiaomimimo.com/api/v1"
	httpTimeout     = 10 * time.Second
)

func init() {
	provider.Register("xiaomimimo", func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}

type Client struct {
	cookie   string
	endpoint string
	http     *http.Client
}

// New requires the Xiaomi account session cookie (api-platform_ph,
// api-platform_serviceToken, api-platform_slh, userId) in cfg.Extra["cookie"].
// Token Plan API keys (tp-xxx) are NOT accepted by these console endpoints.
func New(cfg provider.Config) (*Client, error) {
	cookie := strings.TrimSpace(cfg.Extra["cookie"])
	if cookie == "" {
		return nil, fmt.Errorf("xiaomimimo: extra.cookie is required (browser session cookie from platform.xiaomimimo.com; tp- API keys are not supported)")
	}
	ep := cfg.Endpoint
	if ep == "" {
		ep = defaultEndpoint
	}
	return &Client{
		cookie:   cookie,
		endpoint: ep,
		http:     &http.Client{Timeout: httpTimeout},
	}, nil
}

func (c *Client) Name() string { return "xiaomimimo" }

func (c *Client) Fetch(ctx context.Context) (*provider.Snapshot, error) {
	body, err := c.get(ctx, c.endpoint+"/tokenPlan/usage")
	if err != nil {
		return nil, err
	}

	snap := &provider.Snapshot{
		Provider:  "xiaomimimo",
		FetchedAt: time.Now(),
	}

	if !c.parseResponse(body, snap) {
		return nil, fmt.Errorf("xiaomimimo: unrecognized response format")
	}
	c.applyPlanWindow(ctx, snap)
	return snap, nil
}

func (c *Client) get(ctx context.Context, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xiaomimimo: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xiaomimimo: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return data, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("xiaomimimo: unauthorized (401) — session cookie expired, re-copy it from the browser")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("xiaomimimo: rate limited (429) — retry later")
	default:
		return nil, fmt.Errorf("xiaomimimo: unexpected status %d", resp.StatusCode)
	}
}

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		MonthUsage usageSection `json:"monthUsage"`
	} `json:"data"`
}

type usageSection struct {
	Percent float64     `json:"percent"`
	Items   []usageItem `json:"items"`
}

type usageItem struct {
	Name    string  `json:"name"`
	Used    float64 `json:"used"`
	Limit   float64 `json:"limit"`
	Percent float64 `json:"percent"` // fraction, 0.0782 = 7.82%
}

func (c *Client) parseResponse(body json.RawMessage, snap *provider.Snapshot) bool {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return false
	}
	if env.Code != 0 {
		return false
	}

	// Only the monthUsage section is exposed; the usage section
	// (plan_total/compensation) aggregates the same monthly bucket.
	for _, item := range env.Data.MonthUsage.Items {
		// skip empty buckets (e.g. 0/0)
		if item.Used == 0 && item.Limit == 0 {
			continue
		}
		pct := item.Percent * 100
		snap.Quotas = append(snap.Quotas, provider.Quota{
			Name:    "month_" + item.Name,
			Label:   itemLabel(item.Name),
			Used:    &item.Used,
			Limit:   &item.Limit,
			UsedPct: &pct,
			Unit:    "credits",
		})
	}

	return len(snap.Quotas) > 0
}

type planEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CurrentPeriodEnd string `json:"currentPeriodEnd"`
		Expired          bool   `json:"expired"`
	} `json:"data"`
}

// applyPlanWindow derives the monthly reset window from the plan detail:
// quotas reset on the same day-of-month as the plan's currentPeriodEnd.
// Best-effort: a missing or expired plan leaves quotas without a window.
func (c *Client) applyPlanWindow(ctx context.Context, snap *provider.Snapshot) {
	body, err := c.get(ctx, c.endpoint+"/tokenPlan/detail")
	if err != nil {
		return
	}
	var env planEnvelope
	if err := json.Unmarshal(body, &env); err != nil || env.Code != 0 || env.Data.Expired {
		return
	}
	periodEnd, err := time.ParseInLocation("2006-01-02 15:04:05", env.Data.CurrentPeriodEnd, time.UTC)
	if err != nil {
		return
	}
	start, end := monthWindow(time.Now(), periodEnd.Day())
	for i := range snap.Quotas {
		q := &snap.Quotas[i]
		q.WindowStart = &start
		q.ResetAt = &end
		q.ResetIn = formatDuration(time.Until(end))
	}
}

func monthWindow(now time.Time, anchorDay int) (start, end time.Time) {
	y, m, _ := now.UTC().Date()
	anchor := func(yr int, mo time.Month) time.Time {
		d := anchorDay
		if dim := time.Date(yr, mo+1, 0, 0, 0, 0, 0, time.UTC).Day(); d > dim {
			d = dim
		}
		return time.Date(yr, mo, d, 0, 0, 0, 0, time.UTC)
	}
	thisMonth := anchor(y, m)
	if thisMonth.After(now) {
		return anchor(y, m-1), thisMonth
	}
	return thisMonth, anchor(y, m+1)
}

func itemLabel(name string) string {
	switch name {
	case "month_total_token":
		return "Monthly Usage"
	case "plan_total_token":
		return "Plan Total"
	case "compensation_total_token":
		return "Compensation"
	default:
		label := strings.ReplaceAll(name, "_", " ")
		if len(label) > 0 {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		return label
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
