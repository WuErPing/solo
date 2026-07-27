package deepseek

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
	defaultEndpoint = "https://api.deepseek.com"
	httpTimeout     = 10 * time.Second
)

func init() {
	provider.Register("deepseek", func(cfg provider.Config) (provider.Provider, error) {
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
		return nil, fmt.Errorf("deepseek: apiKey is required (sk-xxx from platform.deepseek.com)")
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

func (c *Client) Name() string { return "deepseek" }

func (c *Client) Fetch(ctx context.Context) (*provider.Snapshot, error) {
	body, err := c.get(ctx, c.endpoint+"/user/balance")
	if err != nil {
		return nil, err
	}

	snap := &provider.Snapshot{
		Provider:  "deepseek",
		FetchedAt: time.Now(),
	}

	if !c.parseResponse(body, snap) {
		return nil, fmt.Errorf("deepseek: unrecognized response format")
	}
	return snap, nil
}

func (c *Client) get(ctx context.Context, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepseek: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deepseek: read body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return data, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("deepseek: unauthorized (401) — ensure key is a valid platform.deepseek.com API key")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("deepseek: rate limited (429) — retry later")
	default:
		return nil, fmt.Errorf("deepseek: unexpected status %d", resp.StatusCode)
	}
}

// numStr handles the API returning amounts as JSON strings ("9.90") or numbers (9.9).
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
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    numStr `json:"total_balance"`
		GrantedBalance  numStr `json:"granted_balance"`
		ToppedUpBalance numStr `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

// parseResponse maps balance info to quotas. DeepSeek exposes remaining balance
// (not usage against a limit), so Used carries the remaining amount and
// Limit/UsedPct stay nil.
func (c *Client) parseResponse(body json.RawMessage, snap *provider.Snapshot) bool {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	if len(resp.BalanceInfos) == 0 {
		return false
	}

	for _, info := range resp.BalanceInfos {
		currency := strings.ToUpper(info.Currency)
		if currency == "" {
			currency = "CNY"
		}
		snap.Quotas = append(snap.Quotas, provider.Quota{
			Name:  "balance_" + strings.ToLower(currency),
			Label: fmt.Sprintf("Balance (%s)", currency),
			Used:  info.TotalBalance.ptr(),
			Unit:  currency,
		})
		if info.GrantedBalance.isSet && info.GrantedBalance.val > 0 {
			snap.Quotas = append(snap.Quotas, provider.Quota{
				Name:  "granted_balance_" + strings.ToLower(currency),
				Label: fmt.Sprintf("Granted Balance (%s)", currency),
				Used:  info.GrantedBalance.ptr(),
				Unit:  currency,
			})
		}
	}

	return true
}
