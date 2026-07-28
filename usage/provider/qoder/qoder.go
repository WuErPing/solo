package qoder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WuErPing/solo/usage/provider"
)

const (
	defaultEndpoint = "https://api.qoder.com"
	httpTimeout     = 10 * time.Second
	maxPages        = 10
)

func init() {
	provider.Register("qoder", func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}

type Client struct {
	apiKey         string
	endpoint       string
	cookie         string
	organizationID string
	http           *http.Client
}

// personalEndpoint is the web console origin used in cookie mode.
const personalEndpoint = "https://qoder.com"

// New supports two auth modes:
//   - personal mode: extra.cookie set (browser session cookie from qoder.com)
//   - org mode: apiKey + extra.organizationId (teams OpenAPI, org admin only)
func New(cfg provider.Config) (*Client, error) {
	c := &Client{http: &http.Client{Timeout: httpTimeout}}

	if cookie := strings.TrimSpace(cfg.Extra["cookie"]); cookie != "" {
		c.cookie = cookie
		c.endpoint = cfg.Endpoint
		if c.endpoint == "" {
			c.endpoint = personalEndpoint
		}
		return c, nil
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("qoder: apiKey is required (see https://docs.qoder.com/zh/account/teams/openapi/usage), or set extra.cookie for personal accounts")
	}
	orgID := cfg.Extra["organizationId"]
	if orgID == "" {
		return nil, fmt.Errorf("qoder: extra.organizationId is required")
	}
	c.apiKey = cfg.APIKey
	c.organizationID = orgID
	c.endpoint = cfg.Endpoint
	if c.endpoint == "" {
		c.endpoint = defaultEndpoint
	}
	return c, nil
}

func (c *Client) Name() string { return "qoder" }

func (c *Client) Fetch(ctx context.Context) (*provider.Snapshot, error) {
	if c.cookie != "" {
		return c.fetchPersonal(ctx)
	}
	return c.fetchOrg(ctx)
}

// fetchPersonal queries the personal credits usage via the web console API
// (GET /api/v2/me/usages/big_model_credits) with the session cookie.
func (c *Client) fetchPersonal(ctx context.Context) (*provider.Snapshot, error) {
	body, err := c.get(ctx, c.endpoint+"/api/v2/me/usages/big_model_credits")
	if err != nil {
		return nil, err
	}

	snap := &provider.Snapshot{
		Provider:  "qoder",
		FetchedAt: time.Now(),
	}
	if !c.parsePersonal(body, snap) {
		return nil, fmt.Errorf("qoder: unrecognized response format (cookie expired or blocked by anti-bot — re-copy it from the browser)")
	}
	return snap, nil
}

type personalUsage struct {
	QuotaKey  string `json:"quota_key"`
	Status    string `json:"status"`
	PlanQuota struct {
		QuotaSummary quotaSummary `json:"quota_summary"`
	} `json:"plan_quota"`
	ResourcePackageQuota struct {
		QuotaSummary quotaSummary `json:"quota_summary"`
	} `json:"resource_package_quota"`
	NextResetAt int64 `json:"nextResetAt"` // unix ms
}

type quotaSummary struct {
	UsedValue       float64 `json:"used_value"`
	LimitValue      float64 `json:"limit_value"`
	UsagePercentage float64 `json:"usage_percentage"` // already 0-100
	Unit            string  `json:"unit"`
}

func (c *Client) parsePersonal(body json.RawMessage, snap *provider.Snapshot) bool {
	var resp personalUsage
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	if resp.QuotaKey == "" {
		return false
	}

	add := func(name, label string, s quotaSummary, resetMs int64) {
		if s.UsedValue == 0 && s.LimitValue == 0 {
			return
		}
		unit := s.Unit
		if unit == "" {
			unit = "credits"
		}
		q := provider.Quota{
			Name:    name,
			Label:   label,
			Used:    &s.UsedValue,
			Limit:   &s.LimitValue,
			UsedPct: &s.UsagePercentage,
			Unit:    unit,
		}
		if resetMs > 0 {
			t := time.UnixMilli(resetMs)
			q.ResetAt = &t
			q.ResetIn = formatDuration(time.Until(t))
		}
		snap.Quotas = append(snap.Quotas, q)
	}

	add("plan_credits", "Plan Credits", resp.PlanQuota.QuotaSummary, resp.NextResetAt)
	add("resource_package_credits", "Resource Package", resp.ResourcePackageQuota.QuotaSummary, 0)

	return len(snap.Quotas) > 0
}

func (c *Client) fetchOrg(ctx context.Context) (*provider.Snapshot, error) {
	packages, err := c.listResourcePackages(ctx)
	if err != nil {
		return nil, err
	}

	snap := &provider.Snapshot{
		Provider:  "qoder",
		FetchedAt: time.Now(),
	}

	now := time.Now()
	for _, pkg := range packages {
		// 真实可用 = status == "active" AND expiresAt > now
		if pkg.Status != "active" {
			continue
		}
		expires, err := time.Parse(time.RFC3339, pkg.ExpiresAt)
		if err != nil || !expires.After(now) {
			continue
		}
		unit := pkg.Unit
		if unit == "" {
			unit = "credits"
		}
		q := provider.Quota{
			Name:  "pkg_" + pkg.ID,
			Label: pkg.Name,
			Used:  &pkg.UsedValue,
			Limit: &pkg.LimitValue,
			Unit:  unit,
		}
		computePct(&q)
		q.ResetAt = &expires
		q.ResetIn = formatDuration(time.Until(expires))
		snap.Quotas = append(snap.Quotas, q)
	}

	return snap, nil
}

type resourcePackage struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	ExpiresAt      string  `json:"expiresAt"`
	LimitValue     float64 `json:"limitValue"`
	UsedValue      float64 `json:"usedValue"`
	RemainingValue float64 `json:"remainingValue"`
	Unit           string  `json:"unit"`
}

type resourcePackagesResponse struct {
	ResourcePackages []resourcePackage `json:"resourcePackages"`
	NextToken        string            `json:"nextToken"`
}

// listResourcePackages fetches all active resource packages, following pagination.
func (c *Client) listResourcePackages(ctx context.Context) ([]resourcePackage, error) {
	var all []resourcePackage
	nextToken := ""
	for page := 0; page < maxPages; page++ {
		u := fmt.Sprintf("%s/v1/organizations/%s/resource-packages?status=active&maxResults=100",
			c.endpoint, url.PathEscape(c.organizationID))
		if nextToken != "" {
			u += "&nextToken=" + url.QueryEscape(nextToken)
		}

		body, err := c.get(ctx, u)
		if err != nil {
			return nil, err
		}
		var resp resourcePackagesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("qoder: unrecognized response format")
		}
		all = append(all, resp.ResourcePackages...)
		if resp.NextToken == "" {
			return all, nil
		}
		nextToken = resp.NextToken
	}
	return all, nil
}

func (c *Client) get(ctx context.Context, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qoder: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qoder: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return data, nil
	case http.StatusUnauthorized:
		if c.cookie != "" {
			return nil, fmt.Errorf("qoder: unauthorized (401) — session cookie expired, re-copy it from the browser")
		}
		return nil, fmt.Errorf("qoder: unauthorized (401) — invalid API key")
	case http.StatusForbidden:
		return nil, fmt.Errorf("qoder: forbidden (403) — API key is not associated with the organization")
	case http.StatusNotFound:
		return nil, fmt.Errorf("qoder: organization not found or not accessible (404)")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("qoder: rate limited (429) — retry later")
	default:
		return nil, fmt.Errorf("qoder: unexpected status %d", resp.StatusCode)
	}
}

func computePct(q *provider.Quota) {
	if q.Used != nil && q.Limit != nil && *q.Limit > 0 {
		pct := *q.Used / *q.Limit * 100
		q.UsedPct = &pct
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
