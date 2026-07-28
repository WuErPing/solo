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
		Usage      usageSection `json:"usage"`
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

	sections := []struct {
		namePrefix string
		section    usageSection
	}{
		{"month", env.Data.MonthUsage},
		{"plan", env.Data.Usage},
	}
	for _, s := range sections {
		for _, item := range s.section.Items {
			// skip empty buckets (e.g. compensation with 0/0)
			if item.Used == 0 && item.Limit == 0 {
				continue
			}
			pct := item.Percent * 100
			snap.Quotas = append(snap.Quotas, provider.Quota{
				Name:    s.namePrefix + "_" + item.Name,
				Label:   itemLabel(item.Name),
				Used:    &item.Used,
				Limit:   &item.Limit,
				UsedPct: &pct,
				Unit:    "credits",
			})
		}
	}

	return len(snap.Quotas) > 0
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
