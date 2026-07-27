package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/WuErPing/solo/usage/provider"
)

const (
	defaultEndpoint = "https://api.kimi.com/coding/v1"
	userAgent       = "KimiCLI/1.6"
	httpTimeout     = 10 * time.Second
)

func init() {
	provider.Register("kimi", func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}

type Client struct {
	apiKey   string
	endpoint string
	http     *http.Client
}

func New(cfg provider.Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("kimi: apiKey is required (sk-kimi-xxx)")
	}
	ep := cfg.Endpoint
	if ep == "" {
		ep = defaultEndpoint
	}
	return &Client{
		apiKey:   cfg.APIKey,
		endpoint: ep,
		http:     &http.Client{Timeout: httpTimeout},
	}, nil
}

func (c *Client) Name() string { return "kimi" }

func (c *Client) Fetch(ctx context.Context) (*provider.Snapshot, error) {
	body, err := c.get(ctx, c.endpoint+"/usages")
	if err != nil {
		return nil, err
	}

	snap := &provider.Snapshot{
		Provider:  "kimi",
		FetchedAt: time.Now(),
	}

	if !c.parseResponse(body, snap) {
		return nil, fmt.Errorf("kimi: unrecognized response format")
	}
	return snap, nil
}

func (c *Client) get(ctx context.Context, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kimi: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kimi: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return data, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("kimi: unauthorized (401) — ensure key is sk-kimi-xxx coding plan key")
	case http.StatusForbidden:
		return nil, fmt.Errorf("kimi: forbidden (403) — quota exhausted or insufficient permissions")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("kimi: rate limited (429) — retry later")
	default:
		return nil, fmt.Errorf("kimi: unexpected status %d", resp.StatusCode)
	}
}

// numStr handles the API returning numbers as JSON strings ("100") or numbers (100).
type numStr struct {
	val   float64
	isSet bool
}

func (n *numStr) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		n.val = v
		n.isSet = true
		return nil
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.val = v
	n.isSet = true
	return nil
}

func (n *numStr) ptr() *float64 {
	if !n.isSet {
		return nil
	}
	return &n.val
}

type apiResponse struct {
	User struct {
		Membership struct {
			Level string `json:"level"`
		} `json:"membership"`
	} `json:"user"`
	Usage struct {
		Limit     numStr `json:"limit"`
		Used      numStr `json:"used"`
		Remaining numStr `json:"remaining"`
		ResetTime string `json:"resetTime"`
	} `json:"usage"`
	Limits []struct {
		Window struct {
			Duration float64 `json:"duration"`
			TimeUnit string  `json:"timeUnit"`
		} `json:"window"`
		Detail struct {
			Limit     numStr `json:"limit"`
			Used      numStr `json:"used"`
			Remaining numStr `json:"remaining"`
			ResetTime string `json:"resetTime"`
		} `json:"detail"`
	} `json:"limits"`
	Parallel struct {
		Limit numStr `json:"limit"`
	} `json:"parallel"`
}

func (c *Client) parseResponse(body json.RawMessage, snap *provider.Snapshot) bool {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	if !resp.Usage.Used.isSet && !resp.Usage.Limit.isSet {
		return false
	}

	if resp.User.Membership.Level != "" {
		snap.Plan = &provider.Plan{
			Name: normalizeLevel(resp.User.Membership.Level),
			Tier: "member",
		}
	}

	weekly := provider.Quota{
		Name:  "weekly_usage",
		Label: "Weekly Usage",
		Used:  resp.Usage.Used.ptr(),
		Limit: resp.Usage.Limit.ptr(),
	}
	computePct(&weekly)
	applyResetTime(&weekly, resp.Usage.ResetTime)
	snap.Quotas = append(snap.Quotas, weekly)

	for _, lim := range resp.Limits {
		unit := normalizeTimeUnit(lim.Window.TimeUnit)
		lq := provider.Quota{
			Name:  windowName(lim.Window.Duration, unit),
			Label: windowLabel(lim.Window.Duration, unit),
			Used:  lim.Detail.Used.ptr(),
			Limit: lim.Detail.Limit.ptr(),
		}
		computePct(&lq)
		applyResetTime(&lq, lim.Detail.ResetTime)
		snap.Quotas = append(snap.Quotas, lq)
	}

	if resp.Parallel.Limit.isSet {
		pq := provider.Quota{
			Name:  "parallel",
			Label: "Parallel Limit",
			Limit: resp.Parallel.Limit.ptr(),
		}
		snap.Quotas = append(snap.Quotas, pq)
	}

	return true
}

func computePct(q *provider.Quota) {
	if q.Used != nil && q.Limit != nil && *q.Limit > 0 {
		pct := *q.Used / *q.Limit * 100
		q.UsedPct = &pct
	}
}

func applyResetTime(q *provider.Quota, resetTime string) {
	if resetTime == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, resetTime)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, resetTime)
		if err != nil {
			return
		}
	}
	q.ResetAt = &t
	q.ResetIn = formatDuration(time.Until(t))
}

// normalizeTimeUnit converts "TIME_UNIT_MINUTE" → "MINUTE".
func normalizeTimeUnit(unit string) string {
	return strings.TrimPrefix(unit, "TIME_UNIT_")
}

// normalizeLevel converts "LEVEL_INTERMEDIATE" → "Intermediate".
func normalizeLevel(level string) string {
	s := strings.TrimPrefix(level, "LEVEL_")
	s = strings.ToLower(s)
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func windowName(duration float64, unit string) string {
	return fmt.Sprintf("%s_limit", windowLabel(duration, unit))
}

func windowLabel(duration float64, unit string) string {
	switch unit {
	case "MINUTE":
		if duration >= 60 {
			return fmt.Sprintf("%.0fh", duration/60)
		}
		return fmt.Sprintf("%.0fm", duration)
	case "HOUR":
		return fmt.Sprintf("%.0fh", duration)
	case "DAY":
		return fmt.Sprintf("%.0fd", duration)
	case "MONTH":
		return fmt.Sprintf("%.0fmo", duration)
	default:
		return fmt.Sprintf("%.0f%s", duration, unit)
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
