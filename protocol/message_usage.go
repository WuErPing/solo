package protocol

// --- Usage Quota Types ---

// genzod
// UsagePlan describes the subscription plan of a provider account.
type UsagePlan struct {
	Name string `json:"name"`
	Tier string `json:"tier,omitempty"`
}

// genzod
// UsageQuota is a single quota window reported by a provider.
// Numeric fields are nullable: providers may omit values they do not expose.
type UsageQuota struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Used        *float64 `json:"used"`
	Limit       *float64 `json:"limit"`
	UsedPct     *float64 `json:"usedPct"`
	Unit        string   `json:"unit,omitempty"`
	WindowStart *string  `json:"windowStart"` // RFC3339 timestamp; start of the current reset window
	ResetAt     *string  `json:"resetAt"`     // RFC3339 timestamp
	ResetIn     string   `json:"resetIn,omitempty"`
}

// genzod
// UsageQuotaSnapshot is a point-in-time quota snapshot for one provider.
type UsageQuotaSnapshot struct {
	Provider  string       `json:"provider"`
	Plan      *UsagePlan   `json:"plan"`
	Quotas    []UsageQuota `json:"quotas"`
	FetchedAt string       `json:"fetchedAt"` // RFC3339 timestamp
}

// --- Inbound Requests ---

// genzod
type UsageQuotaListRequest struct {
	Type         string `json:"type"`
	RequestID    string `json:"requestId"`
	ForceRefresh bool   `json:"forceRefresh,omitempty"`
}

func (m UsageQuotaListRequest) MsgType() string { return "usage/quota/list" }

// --- Outbound Responses ---

// genzod
type UsageQuotaListResponse struct {
	Type    string                        `json:"type"`
	Payload UsageQuotaListResponsePayload `json:"payload"`
}

func (m UsageQuotaListResponse) MsgType() string { return "usage/quota/list/response" }

// genzod
type UsageQuotaListResponsePayload struct {
	RequestID string               `json:"requestId"`
	Snapshots []UsageQuotaSnapshot `json:"snapshots"`
	Errors    map[string]string    `json:"errors,omitempty"` // per-provider error, keyed by provider name
	CachedAt  string               `json:"cachedAt"`         // RFC3339 timestamp of when the snapshots were fetched
	Error     *string              `json:"error"`
}
