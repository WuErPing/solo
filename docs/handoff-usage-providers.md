# Handoff: Usage Providers — Xiaomi MiMo (blocked) & Qoder (org-only)

> Context: the `usage/` module (`solo-usage` CLI + daemon `usage/quota/list` RPC) fetches
> subscription quota snapshots per provider. Implemented so far: `kimi`, `deepseek`, `qoder`.
> This document records the Xiaomi MiMo and Qoder API investigations so the work can be
> resumed without re-doing the reconnaissance.

## Xiaomi MiMo (Token Plan) — IMPLEMENTED (cookie auth), detail endpoint pending

### Status update (2026-07-27)
Provider `usage/provider/xiaomimimo/` implemented and tested (7 tests). Auth: browser
session cookie via `extra.cookie` in `~/.solo/usage.json`. Only `/tokenPlan/usage` is
consumed so far; `/tokenPlan/detail` (plan name, expiry, auto-renew) response shape is
still unknown — a real sample is needed to add the Plan badge and ResetAt.
Registered in CLI + daemon; `solo-usage providers` lists it.

Real `/tokenPlan/usage` response (Lite annual plan, confirmed):
```json
{"code":0,"message":"","data":{
  "monthUsage":{"percent":0.0782,"items":[{"name":"month_total_token","used":3846407684,"limit":49200000000,"percent":0.0782}]},
  "usage":{"percent":0.08,"items":[{"name":"plan_total_token","used":3846407684,"limit":49200000000,"percent":0.08},{"name":"compensation_total_token","used":0,"limit":0,"percent":0}]}}}
```
Notes: envelope is `{code,message,data}` (`code != 0` → error); `percent` is a fraction
(0.0782 = 7.82%, provider converts to 0-100); items with used=0 AND limit=0 are skipped;
cookie fields needed: `api-platform_ph`, `api-platform_serviceToken`, `api-platform_slh`,
`userId`; HTTP 401 → cookie expired (re-copy from browser).

### Original investigation

### Goal
Show Token Plan usage (example from the user's console: Lite 年度套餐, used
3,846,407,684 / 49,200,000,000 credits, 8.0%, 有效期至 2027-07-09) as a usage snapshot.

### What was tried (all confirmed by probing)
1. **Public API for `tp-` keys does not exist.** Token Plan keys (`tp-xxxxx`) authenticate
   against the OpenAI/Anthropic-compatible inference gateways only:
   - `https://token-plan-cn.xiaomimimo.com/v1` (also `-sgp`, `-ams` clusters)
   - Candidate usage paths probed on all clusters + `api.xiaomimimo.com`
     (`/v1/usages`, `/v1/usage`, `/v1/credits`, `/v1/quota`, `/v1/plan`, `/v1/user/plan`,
     `/v1/subscription`, `/v1/billing/usage`, `/v1/dashboard/billing/usage`, `/v1/me`)
     → all **404** (control: `/v1/models` → 401, so 404 = truly absent).
   - The console endpoints `platform.xiaomimimo.com/api/v1/tokenPlan/usage` and
     `/api/v1/tokenPlan/detail` exist but reject `Authorization: Bearer tp-...` with
     **401 + Xiaomi SSO loginUrl** (verified with the user's real tp- key).
2. **Official agent has no usage query either.** `XiaomiMiMo/MiMo-Code` source
   (`packages/opencode/src/cli/cmd/tui/component/dialog-token-plan.tsx`) only links to
   `https://platform.xiaomimimo.com/token-plan` — no programmatic usage fetch.
3. **Untested fallback**: inference-gateway response *headers* may carry quota fields
   (`x-credits-*`, `x-ratelimit-*`). Not yet checked with a real key:
   `curl -s -D - -o /dev/null "https://token-plan-cn.xiaomimimo.com/v1/models" -H "Authorization: Bearer tp-KEY"`

### Console API details (confirmed by reverse-engineering the SPA)
- Endpoints (exist, require Xiaomi account SSO **session cookie**):
  - `GET https://platform.xiaomimimo.com/api/v1/tokenPlan/usage`
  - `GET https://platform.xiaomimimo.com/api/v1/tokenPlan/detail`
  - Unauthenticated → `401 {"code":401,"loginUrl":"https://account.xiaomi.com/pass/serviceLogin?..."}`
- These paths were found in the console bundle `https://platform.xiaomimimo.com/static/main.*.chunk.js`
  (`url:"/tokenPlan/usage"`, `url:"/tokenPlan/detail"`; the request wrapper prefixes `/api/v1`).
- **Response JSON field names are still unknown** — they are consumed by a minified MobX
  store (`tokenPlanDetail` / `tokenPlanUsage`); extracting them from the bundle was abandoned
  as too costly. Next step requires one real response sample.

### Agreed next step (user chose "方案 B": cookie auth)
1. User runs, with the `Cookie` header copied from a logged-in browser session
   (DevTools → Network → `usage`/`detail` request on `/console/plan-manage`):
   ```bash
   curl -s "https://platform.xiaomimimo.com/api/v1/tokenPlan/usage"  -H "Cookie: <cookie>"
   curl -s "https://platform.xiaomimimo.com/api/v1/tokenPlan/detail" -H "Cookie: <cookie>"
   ```
2. Implement `usage/provider/xiaomimimo/` modeled on `kimi`/`deepseek`:
   cookie from `extra.cookie` in `~/.solo/usage.json` (`${ENV}` expansion supported),
   default endpoint `https://platform.xiaomimimo.com/api/v1`.
3. Caveat to surface to the user: Xiaomi SSO cookies expire (weeks–months) and must be
   re-copied periodically.

## Qoder — IMPLEMENTED, two auth modes (personal cookie + org OpenAPI)

### Personal mode (cookie) — added 2026-07-27, works for the current user
- Reverse-engineered from the `qoder-web` Next.js bundle (public CDN chunks; route
  probing is impossible — the API gateway blanket-401s everything unauthenticated, and
  an Alibaba WAF serves x5sec "punish" interstitials).
- Endpoint: `GET https://qoder.com/api/v2/me/usages/big_model_credits` with the browser
  session cookie (no CSRF header needed for GETs). Related: `GET /api/v1/me/userplan`
  (plan name/auto-renew — not yet consumed, no sample), quota history at
  `/api/v1/me/quotas/big_model_credits/histories`.
- Real response shape (personal Pro account):
  ```json
  {"user_id":"...","quota_key":"big_model_credits","status":"active",
   "plan_quota":{"quota_summary":{"used_value":370,"limit_value":6000,"remaining_value":5630,"usage_percentage":7,"unit":"credits"},"quota_detail":[...]},
   "resource_package_quota":{"quota_summary":{"used_value":0,"limit_value":0,...},"quota_detail":null},
   "total_quota":{...},"lastResetAt":1768966445071,"nextResetAt":1787587200000}
  ```
  `usage_percentage` is already 0-100; `nextResetAt`/`lastResetAt` are unix **ms**
  (monthly reset). Mapping: plan_quota → "Plan Credits" quota (ResetAt=nextResetAt);
  resource_package_quota → "Resource Package" (skipped when 0/0).
- Config: `"qoder": {"enabled": true, "extra": {"cookie": "${QODER_COOKIE}"}}`
  (cookie mode takes precedence when `extra.cookie` is set).
- Caveats: cookie expiry; WAF anti-bot may return 200 HTML → provider reports
  "unrecognized response format (cookie expired or blocked by anti-bot)".

### Org mode (OpenAPI) — implemented earlier, org-admin accounts only

### What was built (commit pending at time of writing)
- `usage/provider/qoder/qoder.go` — calls
  `GET {endpoint}/v1/organizations/{orgId}/resource-packages?status=active&maxResults=100`
  (paginates via `nextToken`, cap 10 pages). Maps each *truly usable* package
  (`status == "active"` AND `expiresAt > now`, per the doc's guidance) to a `Quota`:
  used/limit/usedPct, `expiresAt` → ResetAt/ResetIn, unit `credits`.
- `usage/provider/qoder/qoder_test.go` — 10 tests (real format, expired/exhausted
  filtering, pagination, 401/403/404, missing key/orgId). All green; CLI rebuilt and
  lists `deepseek / kimi / qoder`.
- Registered in `usage/cmd/providers_import.go` and `daemon/internal/server/session_usage.go`.
- Config shape:
  ```json
  "qoder": { "enabled": true, "apiKey": "${QODER_API_KEY}", "extra": { "organizationId": "org_xxx" } }
  ```
- Source: official docs <https://docs.qoder.com/zh/account/teams/openapi/usage>.

### Blocker for the current user
The OpenAPI is a **teams/organization** feature: API Keys are created by an **org admin**
at `https://qoder.com/organizations/{orgid}/settings` → 设置 → 高级 → API 密钥
(<https://docs.qoder.com/zh/account/teams/openapi/get-api-key>). The user's account has no
organization, so they cannot obtain an API Key — the provider is unusable for them until:
1. they join/create an org and get an admin-issued key, or
2. a personal-account path is reverse-engineered (Qoder web/IDE personal Credits balance;
   same cookie-style approach as Xiaomi MiMo — not yet investigated, no known endpoint).

### Other Qoder API facts (from the docs, for future use)
- Base URL `https://api.qoder.com`, `Authorization: Bearer <api_key>`.
- Member usage: `GET /v1/organizations/{org}/members/{member}/usage-events` and
  `usage-summary` (≤7-day range, groupBy source|operation) — richer than resource-packages
  but requires a `member_id` and only reports consumption, not quota limits.
- Error envelope: `{"requestId","code","message"}`; 400/401/403/404/500.

## Files touched (Qoder only; MiMo produced no code changes)
- `usage/provider/qoder/qoder.go`, `usage/provider/qoder/qoder_test.go` (new)
- `usage/cmd/providers_import.go`, `daemon/internal/server/session_usage.go` (registration)
- `docs/handoff-usage-providers.md` (this file)
