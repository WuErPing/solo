# Chat-Based Schedule Assistant — Product Design

> **Document type**: Product / architecture design
> **Date**: 2026-07-17
> **Last revised**: 2026-07-18 — NL parse switched from ephemeral agent harness to the host's configured LLM Providers (Settings → General); schedule execution via Target Agent confirmed unchanged.
> **Baseline version**: Solo v0.10.0
> **Status**: Implemented (2026-07-18) — protocol, daemon, app-bridge, app UI, E2E specs landed; see §10 for the test surface.
> **Audience**: Backend, frontend, product
> **Related docs**:
> - [Loop Schedule Implementation Spec](loop-schedule-spec.md)
> - [App-Bridge Schedule Module](../analysis/app-bridge-schedule-module.md)
> - [Create Schedule Flow](../analysis/create-schedule-flow.md)
> - [ADR-001: Shared Agent Template](../decisions/adr-001-shared-agent-template-for-loop-and-schedule.md)

---

## Executive Summary

Add a **Schedule Assistant**: a chat panel inside the Schedules area where users create and edit schedules in natural language ("every weekday at 9:00, summarize overnight agent activity"), powered by the OpenAI-compatible LLM providers the user configures under **Settings → General → LLM Providers**.

Three locked decisions (confirmed with product owner):

1. **Dedicated assistant panel** in the Schedules area — not schedule tools inside existing agent chat. The assistant is scoped, predictable, and safe; agent-chat tool injection may follow later as a separate phase (out of scope here).
2. **Parse via the daemon's configured LLM Providers (default config)** — the daemon parses natural language with a direct, stateless chat-completion call to the host's default LLM provider, resolved from `config.llmProviders` (first enabled provider; its `isDefault` model, else first model). No agent session is spawned, no CLI harness is involved, no new credentials are introduced — this consumes the existing LLM Providers settings backend, which today has no runtime consumer.
3. **Execution unchanged — Target Agent** — confirmed proposals flow through the existing, validated `schedule/create|update|pause|resume|delete` RPCs, and execution at fire time keeps the current runner behavior: target type `agent` messages a running agent; `new-agent`/`provider` spawn an ephemeral agent via AgentManager. The LLM only ever *proposes* a `ScheduleTarget`; it never touches the execution path.

Core safety invariant: **the LLM never mutates schedules**. It only produces a *proposal*; the user confirms via a structured card, and confirmation flows through the existing schedule RPCs. Only **one new RPC pair** (`schedule/assist`) is added.

---

## 1. Problem & Goals

### 1.1 Problem

- Schedule creation today requires filling a form: prompt text, cadence type, cron expression or interval, target agent. Cron syntax is the main friction — most users cannot write `0 9 * * 1-5` and should not have to.
- Editing means re-opening the form and re-entering every field (`schedule/update` is full-replace).
- Schedules are per-host and often numerous; small tweaks ("move the daily report to 7:30", "pause everything on this host") require hunting through the list.

### 1.2 Goals

| # | Goal | Success signal |
|---|------|----------------|
| G1 | Create a schedule from one natural-language sentence | ≥80% of common requests produce a correct proposal on first attempt (stub-endpoint + real-provider eval set) |
| G2 | Edit/pause/resume/delete existing schedules by referring to them by name | Correct schedule resolution for unambiguous names; clarify loop for ambiguous ones |
| G3 | Zero accidental mutations | Every mutation passes an explicit confirm card; no auto-apply path exists |
| G4 | Provider-agnostic | Feature works with any OpenAI-compatible endpoint configured in LLM Providers; no provider-specific code or UI |
| G5 | No regression to existing schedule stack | All existing schedule RPCs, UI, timezone pipeline, and Target Agent execution unchanged and reused |

### 1.3 Non-goals (v1)

- Loop-type schedules (`type: "loop"`) via chat — wait for Loop Schedule spec to land.
- Schedule tools inside regular agent chat sessions (future phase).
- Daemon-side conversation persistence (transcripts live in the app).
- In-panel LLM provider/model switching — v1 always uses the host default (per-request override is a v1.1 candidate, see §13).
- Managing LLM provider configs — the existing Settings → General → LLM Providers UI and daemon config backend are reused as-is.
- Multi-step autonomous schedule management (the assistant proposes; it never chains mutations).

---

## 2. User Stories & Flows

### 2.1 Create

```
User: "Every weekday at 9am, have the backend agent summarize the nightly test runs"
Assistant: [Proposal card]
           CREATE  "Nightly test summary"
           Prompt: "Summarize the nightly test runs"
           Cadence: Every weekday at 09:00  (cron 0 9 * * 1-5, Asia/Shanghai)
           Target: agent "backend-worker"
           Next run: Tue 2026-07-21 09:00
           [Confirm]  [Edit in form]  [Cancel]
User: [Confirm] → schedule/create → "Created ✓ — view schedule"
```

### 2.2 Edit by reference

```
User: "Move the nightly summary to 7:30"
Assistant: [Proposal card]
           UPDATE  "Nightly test summary"
           Cadence: Every weekday at 09:00 → Every weekday at 07:30
           Next run: Wed 2026-07-22 07:30
           [Confirm]  [Edit in form]  [Cancel]
```

### 2.3 Lifecycle ops

```
User: "Pause the nightly summary until further notice"
Assistant: [Proposal card]  PAUSE "Nightly test summary"  [Confirm] [Cancel]
```

### 2.4 Clarify loop

```
User: "Change it to every 2 hours"
Assistant: [Clarify card] "Which schedule do you mean — 'Nightly test summary'
           or 'Disk cleanup'? (or say 'both')"
```

### 2.5 Informational answer (no mutation)

```
User: "What runs today?"
Assistant: [Answer] "3 schedules run today: Nightly test summary (09:00), …"
```

### 2.6 Failure paths

- **No LLM provider configured on the host** (none enabled, or no model resolvable) → error card with deep link: "Add an LLM provider with a default model in Settings → General → LLM Providers."
- Endpoint returns unparseable output → daemon retries once with the validation error appended → still failing → error card: "Couldn't understand that as a schedule change. Try rephrasing, or use the form."
- Endpoint unreachable / auth failure (401/403/429/5xx) → error card naming the configured provider, with a settings deep link and Retry.
- Cadence invalid (cron doesn't parse, interval < 60s) → clarify card stating the constraint.
- Referenced agent/schedule not found → clarify card listing candidates.

---

## 3. UX Design

### 3.1 Entry points

| Surface | Entry | Host scoping |
|---------|-------|--------------|
| Per-host schedules screen (`app/src/screens/schedules-screen.tsx`) | "Ask AI" header button (sparkle icon) | Host implied by screen |
| Schedule detail screen (`schedule-detail-screen.tsx`) | "Edit with AI" button | Host + `contextScheduleId` implied |
| Schedules dashboard (`schedules/schedules-dashboard-screen.tsx`) | Assistant FAB | If >1 host connected, first message composer shows a host chip selector (required, defaults to last-used host) |

Schedules live on a specific daemon — and so does the `llmProviders` config. The assistant is therefore **host-scoped**: it parses with the LLM provider configured on the same host whose schedules it manages. The panel always displays which host it is talking to.

### 3.2 Panel layout

- Mobile: bottom sheet (existing `@gorhom/bottom-sheet` pattern) over the schedules screen.
- Web/desktop: right-side docked panel, ~380px.
- Contents: host chip, LLM provider indicator (see 3.3), scrollable message list, composer with send button.

Message types in the list:

| Type | Rendering |
|------|-----------|
| User text | Standard chat bubble |
| Assistant text (clarify/answer) | Standard bubble |
| **Proposal card** | Structured card, see 3.4 |
| Error card | Muted bubble with retry hint (+ settings deep link for config errors) |
| Applied receipt | Collapsed card: "Created ✓ / Updated ✓ / Paused ✓" + link to schedule detail |

### 3.3 LLM provider indicator

- Display-only chip in the panel header showing the resolved provider label + model (echoed back in each `schedule/assist` response).
- **v1 has no in-panel switching.** The parse always uses the host default: the first enabled provider in Settings → General → LLM Providers, with its `isDefault` model (else first model). To change it, the user edits the provider list in settings (reorder, disable, or change the default model).
- Per-request provider/model override deferred to v1.1 (see §13).

### 3.4 Proposal card anatomy

```
┌────────────────────────────────────────────┐
│ ● CREATE                          (op badge)│
│ Nightly test summary               (name)  │
│ ─────────────────────────────────────────  │
│ Prompt   Summarize the nightly test runs   │
│ Cadence  Every weekday at 09:00            │
│ Target   agent · backend-worker            │
│ Cwd      ~/work/backend                    │
│ Max runs 30 · Expires 2026-08-31 (if set)  │
│ ─────────────────────────────────────────  │
│ Next run: Tue 2026-07-21 09:00 (local)     │
│ ⚠ warnings, if any                         │
│ [ Confirm ] [ Edit in form ] [ Cancel ]    │
└────────────────────────────────────────────┘
```

- Op badge colors: create=green, update=blue, pause=amber, resume=green, delete=red.
- **Update** cards show a per-field diff (old → new) instead of the full field list; unchanged fields collapsed.
- **Delete** cards name the schedule and its cadence; confirm button is destructive-styled. There is no bulk-delete op in v1.
- Cadence line is rendered with the existing `describeCron()` so the user reads exactly what will be stored.
- **Target** is rendered from the proposed `ScheduleTarget` — the same field the execution runner will resolve at fire time (existing agent / new agent template / provider).
- **Edit in form** opens the existing `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` pre-filled with proposal values — the user can adjust and save through the normal path. This guarantees a manual escape hatch that needs zero new form code.

### 3.5 Empty, loading, and offline states

- Empty conversation: 3 example prompts as suggestion chips ("Every morning at 9, run the daily standup summary", "Pause the disk cleanup", "What runs this week?").
- Sending: pending bubble ("Thinking…" with the resolved provider label); timeout 120s → error card with Retry.
- No LLM provider configured on the host: empty state replaced by a setup card deep-linking to Settings → General → LLM Providers (known before sending, from daemon config).
- Host disconnected: composer disabled with reconnect hint.

---

## 4. Architecture

### 4.1 Overview

```
┌──────────────────────────────────────────────────────────────┐
│ Solo App                                                      │
│  ScheduleAssistantPanel                                       │
│   ├─ message list (user / assistant / proposal / error)       │
│   ├─ composer + host chip + LLM provider indicator            │
│   └─ ProposalCard → Confirm → existing schedule/* RPCs        │
└───────────────────────────┬──────────────────────────────────┘
                            │ WebSocket (E2EE via Relay, or local)
┌───────────────────────────▼──────────────────────────────────┐
│ Solo Daemon (per host)                                        │
│                                                               │
│  session_schedule_assist.go   handleScheduleAssist()          │
│            │                                                  │
│  ┌─────────▼───────────────────────────────────────────┐     │
│  │ daemon/internal/schedule/ (assistant files)          │     │
│  │  · Assistant      — orchestration, rate limit, retry │     │
│  │  · PromptBuilder  — system prompt + context block    │     │
│  │  · Extractor      — JSON extraction + validation     │     │
│  │  · Context        — agents + schedules summaries     │     │
│  │  · Resolver       — default provider/model from      │     │
│  │                     config.LLMProviders              │     │
│  └─────────┬───────────────────────────────────────────┘     │
│            │ one-shot HTTPS chat completion                   │
│  ┌─────────▼──────────┐     ┌───────────────────────────┐    │
│  │ daemon/internal/llm │────▶ OpenAI-compatible endpoint │    │
│  │ chat client         │     │ baseURL + apiKey + model   │    │
│  └─────────────────────┘     │ (Settings → General →      │    │
│                              │  LLM Providers)            │    │
│                              └───────────────────────────┘    │
│            │ validated proposal only — NO mutation             │
│  schedule.Store ── Executor ── daemonRunner ──▶ Target Agent  │
│  (mutation via existing RPCs; execution path unchanged:       │
│   existing agent / ephemeral provider agent via AgentManager) │
└───────────────────────────────────────────────────────────────┘
```

### 4.2 Why a direct call to the configured LLM provider (decision rationale)

| Alternative | Verdict | Reason |
|-------------|---------|--------|
| **Direct chat-completion to the configured LLM provider (chosen)** | ✅ | Consumes the LLM Providers config users already maintain (Settings → General) — today it has no runtime consumer; one small HTTP client in the daemon; no CLI-harness dependency on the daemon host; credentials stay in daemon config; works identically for local and relay/E2EE clients |
| Ephemeral agent via AgentManager (previous revision of this doc) | ❌ superseded | Requires an installed + authenticated CLI harness per provider on the daemon host; process spawn + session lifecycle overhead for a one-shot parse; harness stream wrapping adds failure modes to a strict JSON contract; ignores the LLM Providers config entirely |
| Parse in the app | ❌ | Would ship apiKey to every client and hit CORS/network limits from web builds; duplicates the parse pipeline per client; breaks the "daemon owns provider credentials" boundary |

### 4.3 Stateless parse with client-held transcript

Each `schedule/assist` request carries the bounded transcript (last ≤10 turns). The daemon keeps **no** assistant conversation state — one HTTP completion carries the full prompt (system + context + transcript + user message).

Rationale:

- A single stateless completion needs no session lifecycle, timeout, or orphan cleanup at all.
- Daemon restart loses nothing; crash mid-parse only fails one request.
- Context the LLM actually needs (agents, schedules, timezone, now) is re-injected fresh per request — always accurate, never stale.

Rejected alternative: daemon-side conversation store (adds persistence, migration, and privacy surface for little benefit).

### 4.4 Context enrichment happens daemon-side

The app sends only `{message, timezone, clientNow, transcript, contextScheduleId?}` — no provider choice in v1. The daemon injects:

- `agents`: id, name, provider, cwd, status (so "the backend agent" resolves to a target)
- `schedules`: id, name, cadence, status, nextRunAt (so "the nightly summary" resolves to an id)
- Capability constraints: allowed ops, cadence rules, target rules

This keeps request payloads tiny and makes name resolution deterministic — the daemon validates every referenced id before returning the proposal.

---

## 5. Protocol Changes

One new RPC pair in a new file `protocol/message_schedule_assist.go`; mirrored in `app-bridge/src/server/schedule/rpc-schemas.ts` + `types.ts` + `shared/messages.ts`. No changes to existing schedule messages.

### 5.1 Request

```go
type ScheduleAssistRequest struct {
    Type      string `json:"type"`
    RequestID string `json:"requestId"`

    Message  string `json:"message"`            // user natural-language input, ≤ 2000 chars

    Timezone  string `json:"timezone"`           // IANA, required — e.g. "Asia/Shanghai"
    ClientNow string `json:"clientNow"`          // RFC3339, client wall clock (relative times: "tomorrow")

    ContextScheduleID string                `json:"contextScheduleId,omitempty"` // opened from a detail screen
    Transcript        []ScheduleAssistTurn  `json:"transcript,omitempty"`        // ≤ 10 turns, oldest first
}

type ScheduleAssistTurn struct {
    Role    string `json:"role"`    // "user" | "assistant"
    Content string `json:"content"` // plain-text rendering of the turn (proposals summarized)
}

func (m ScheduleAssistRequest) MsgType() string { return "schedule/assist" }
```

**No provider field in v1**: the daemon always parses with the host's default LLM provider resolved from `config.llmProviders` (§6.4). A per-request `llmProviderId`/`model` override is a v1.1 candidate (§13).

### 5.2 Response

```go
type ScheduleAssistResponse struct {
    Type    string                          `json:"type"` // "schedule/assist/response"
    Payload ScheduleAssistResponsePayload   `json:"payload"`
}

type ScheduleAssistResponsePayload struct {
    RequestID string                  `json:"requestId"`
    Kind      string                  `json:"kind"` // "proposal" | "clarify" | "answer" | "error"
    Message   string                  `json:"message,omitempty"`  // clarify question / answer text / error detail
    Proposal  *ScheduleAssistProposal `json:"proposal,omitempty"`
    Error     *string                 `json:"error,omitempty"`    // transport/config failure code, e.g. "no_llm_provider", "llm_unreachable", "rate_limited"

    LLMProvider string `json:"llmProvider,omitempty"` // resolved provider config id — for the panel indicator
    Model       string `json:"model,omitempty"`       // resolved model id — for the panel indicator
}

type ScheduleAssistProposal struct {
    Op         string             `json:"op"` // "create" | "update" | "pause" | "resume" | "delete"
    ScheduleID string             `json:"scheduleId,omitempty"` // resolved id for update/pause/resume/delete
    Name       string             `json:"name,omitempty"`
    Prompt     string             `json:"prompt,omitempty"`
    Cadence    *ScheduleCadence   `json:"cadence,omitempty"`    // LOCAL cron/interval in request timezone
    Target     *ScheduleTarget    `json:"target,omitempty"`
    Cwd        string             `json:"cwd,omitempty"`
    MaxRuns    *int               `json:"maxRuns,omitempty"`
    ExpiresAt  string             `json:"expiresAt,omitempty"`

    Summary   string   `json:"summary"`           // one-line human description from the LLM
    Warnings  []string `json:"warnings,omitempty"` // e.g. "interpreted 'morning' as 09:00"
    NextRunAt *string  `json:"nextRunAt,omitempty"` // daemon-computed preview (RFC3339)
}
```

Semantics:

- `Kind` is driven by the daemon-validated LLM output, never asserted by the LLM alone.
- `Error == "no_llm_provider"` when no enabled provider with a resolvable model exists in `config.llmProviders`; the app renders a settings deep link (§3.5).
- `Cadence` in a proposal is expressed in the **client timezone** (local cron). The daemon validates it parses (via existing `cron.go`) and computes `NextRunAt`. On confirm, the **app** converts to UTC with the existing `cronToUTC()` before calling `schedule/create` / `schedule/update` — the storage convention ("frontend converts local → UTC; backend evaluates UTC") stays exactly as today.
- `Target` in a proposal is a plain `ScheduleTarget` (`agent` / `new-agent` / `provider`) — the same shape `schedule/create` already validates via `validateScheduleTarget`.
- `pause` / `resume` / `delete` proposals carry only `Op + ScheduleID + Name + Summary`.
- Client timeout: 120s (parse can take tens of seconds on slower endpoints). All other schedule RPCs stay at 10s.

### 5.3 Confirm path — no new mutation RPC

| Proposal op | App calls on Confirm |
|-------------|----------------------|
| `create` | `schedule/create` (payload mapped 1:1, cadence → UTC) |
| `update` | `schedule/update` (full record: proposal fields merged over the inspected current record) |
| `pause` | `schedule/pause` |
| `resume` | `schedule/resume` |
| `delete` | `schedule/delete` |

For `update`, the app first fetches `schedule/inspect` and merges, because `schedule/update` is full-replace; the diff shown on the card is computed from the same inspect result.

Execution at fire time is entirely the existing runner behavior — Target Agent resolution (`agent` → message to the running agent; `new-agent`/`provider` → ephemeral agent via AgentManager). The assistant introduces nothing new into that path.

---

## 6. Daemon Design

### 6.1 New files

```
daemon/internal/llm/
├── client.go              # OpenAI-compatible chat completion client
└── client_test.go

daemon/internal/schedule/
├── assistant.go           # Assistant: orchestration, rate limit, retry, guards
├── assistant_prompt.go    # PromptBuilder: system prompt + context rendering
├── assistant_extract.go   # Extractor: JSON extraction + schema validation
├── assistant_resolve.go   # Resolver: default LLM provider/model from config
└── assistant_test.go      # unit tests

daemon/internal/server/
└── session_schedule_assist.go   # handleScheduleAssist + response sender
```

Handler registration in `session_register_handlers.go`:

```go
r.Register("schedule/assist", typeHandler(s.handleScheduleAssist))
```

### 6.2 Assistant orchestration

```go
type Assistant struct {
    store   *Store
    agents  assistantAgentLister // read-only: list agents for the context block
    llm     *llm.Client
    cfg     llmConfigSource      // resolves default provider/model (config.LLMProviders)
    limiter *rateLimiter         // per-connection
    logger  *slog.Logger
}

func (a *Assistant) Assist(ctx context.Context, req protocol.ScheduleAssistRequest) (*protocol.ScheduleAssistResponsePayload, error)
```

Unlike the runner's `scheduleAgentManager` seam (which creates/deletes agents to *execute* schedules), the Assistant's agent seam is **read-only** — it lists agents for context. The parse path never creates an agent session.

Flow:

1. **Guard**: validate `Message` non-empty/≤2000 chars, `Timezone` valid IANA; enforce rate limit and single in-flight parse per connection.
2. **Resolve** default LLM provider + model from `config.llmProviders` (§6.4); unresolvable → `Kind: "error"`, `Error: "no_llm_provider"`.
3. **Build prompt** (§6.3): system prompt + context block (agents, schedules, timezone, clientNow) + transcript + user message.
4. **One-shot completion** (§6.4): single HTTPS chat completion, collect the response text.
5. **Extract + validate** (§6.5): JSON → typed intent → semantic validation against store/agent state. On failure, **one retry** with the validation error appended to the prompt.
6. **Resolve references**: schedule/agent names → ids; unknown → `clarify` with candidate list.
7. **Enrich**: compute `NextRunAt` preview via existing `NextRunAt(cadence, now)`; attach warnings; echo resolved `llmProvider`/`model` in the response.
8. Return typed payload. The Assistant **never** calls `Store.Create/Update/...`.

### 6.3 Prompt construction

System prompt (abridged; full template in Appendix A):

- Role: "You convert user requests about recurring tasks into a single JSON object."
- Output contract: *only* JSON, one of `{kind:"proposal", ...}`, `{kind:"clarify", ...}`, `{kind:"answer", ...}`; exact field schema; no prose outside JSON.
- Cadence rules: prefer `cron` for wall-clock times, `every` for pure intervals; minimum interval 60000ms; express cron in the given timezone.
- Target rules: `agent` only when a matching existing agent is listed; otherwise `new-agent` with provider + cwd when inferable, else `clarify`.
- Grounding rules: use only agents/schedules from the context block; quote ids exactly; when a reference is ambiguous, ask — never guess.
- Reply language: match the user's language.

Context block:

```
Current time (client): 2026-07-17T22:50:00+08:00
Client timezone: Asia/Shanghai

Existing agents:
- id=a1b2… name="backend-worker" provider=claude cwd=~/work/backend status=running

Existing schedules:
- id=f3e9… name="Nightly test summary" cadence=cron "0 9 * * 1-5" status=active nextRunAt=…
```

Transcript: last ≤10 turns rendered as `User:` / `Assistant:` lines (proposals summarized to one line, e.g. `Assistant: [proposal] update "Nightly test summary" cadence → 07:30`).

Prompt size guard: context block + transcript capped (~8k chars); schedules/agents lists truncated to 50 entries each with a "…and N more" note.

### 6.4 Default provider resolution & one-shot completion

**Resolution** (per request, read fresh from daemon config — settings changes take effect immediately):

1. Candidates = `cfg.LLMProviders` entries with `enabled != false`, in array order (array order = user priority, matching the settings list order).
2. Provider = first candidate with non-empty `baseURL` and `apiKey`.
3. Model = that provider's `models` entry with `isDefault == true`; else the first entry of `models`.
4. No candidate or no model → `Kind: "error"`, `Error: "no_llm_provider"`, with a message guiding the user to Settings → General → LLM Providers.

**Completion:**

```go
func (a *Assistant) runCompletion(ctx context.Context, p resolvedProvider, systemPrompt, userPrompt string) (string, error)
```

- `llm.Client.ChatCompletion`: `POST {baseURL}/chat/completions` (OpenAI-compatible), `Authorization: Bearer <apiKey>`, body `{model, messages: [system, user], temperature: 0, max_tokens: 1024}`.
- `response_format: {"type":"json_object"}` is **not** sent in v1 — support varies across "OpenAI-compatible" endpoints; the prompt contract + tolerant Extractor + one validation retry carry the JSON guarantee. Revisit in v1.1.
- Timeout: 60s (context deadline). Transport errors (401/403/429/5xx, network) surface immediately as error cards (`llm_unreachable` / `llm_auth` / `rate_limited`) — no silent retry; the user retries from the UI. The single retry in the Assistant flow is reserved for *validation* failures (§6.5).
- Output treated as **untrusted text**; only the Extractor gives it meaning.

Endpoint compatibility stance (v1):

| Aspect | v1 stance |
|--------|-----------|
| API shape | OpenAI chat completions (`POST {baseURL}/chat/completions`) — the common denominator of configured providers |
| Auth | `Authorization: Bearer <apiKey>` only |
| Streaming | No — single non-streaming response |
| JSON mode | Not requested (compatibility); extractor handles fenced/prose-wrapped JSON |
| Models | Whatever the configured default model is; no capability probing |

### 6.5 Extraction & validation

Extractor stages:

1. Locate JSON: prefer a ```json fenced block; else the outermost balanced `{…}` span.
2. Decode into typed `assistIntent` struct (kind + optional fields).
3. **Schema validation**: required fields per op (create → prompt + cadence + target; update → scheduleRef + ≥1 changed field; pause/resume/delete → scheduleRef).
4. **Semantic validation** against live state:
   - cron parses (`cron.go`), `everyMs ≥ 60000`, prompt ≤ 4000 chars
   - referenced schedule id exists (else clarify with ≤5 name candidates, fuzzy-matched)
   - referenced agent id exists and target rules from `validateScheduleTarget` hold
   - `expiresAt` in the future; `maxRuns > 0`
5. Any failure → one retry round-trip including the error; second failure → `Kind: "error"` with a user-actionable message.

### 6.6 Rate limits & resource guards

| Guard | Value | Scope |
|-------|-------|-------|
| Rate limit | 10 assist requests / minute | per connection |
| Concurrency | 1 in-flight parse | per connection (`rate_limited` error otherwise) |
| LLM call timeout | 60s per completion (client RPC budget 120s covers one retry) | per request |
| Daemon egress | 1 HTTPS call per parse, +1 only on validation retry | per request |
| Message size | ≤ 2000 chars user message; transcript ≤ 10 turns | per request |

### 6.7 Metrics & logging

```
solo_schedule_assist_requests_total{llmProvider, kind}
solo_schedule_assist_parse_failures_total{llmProvider, stage}
solo_schedule_assist_duration_seconds{llmProvider}
solo_schedule_assist_confirms_total{op}        // reported by app via existing telemetry path
```

(`llmProvider` label = the config id of the resolved provider, e.g. `"openai"`.)

slog: request id, provider id, model, kind, retry count, validation errors, token-ish sizes. **Never log raw user prompts or API keys at any level**; prompt logging is debug-gated and off by default (privacy).

---

## 7. App Design

### 7.1 New components & hooks

```
app/src/components/schedule-assistant/
├── schedule-assistant-panel.tsx    # sheet/dock container, host chip + LLM indicator
├── assistant-message-list.tsx      # bubbles + cards
├── proposal-card.tsx               # op badge, fields/diff, actions
└── assistant-composer.tsx          # input + send + suggestion chips

app/src/hooks/
├── use-schedule-assist.ts          # mutation: client.scheduleAssist()
└── use-assistant-thread.ts         # thread state, transcript windowing
```

- State: per-host thread in a small Zustand store (`useAssistantStore`, keyed by `serverId`); session-only persistence in v1 (no disk).
- `useScheduleAssist` wraps the bridge call with React Query mutation; on `proposal`, pushes a card message; on `clarify`/`answer`, pushes a bubble; on `error` with `no_llm_provider`, pushes an error card with a deep link to `/settings/general`.
- The LLM provider indicator chip reads `llmProvider`/`model` from the latest assist response (and can pre-check config via `useDaemonConfig` for the empty state).
- Confirm handler: maps op → existing hooks (`useCreateSchedule`, update/pause/resume/delete equivalents), invalidating schedule queries on success; card collapses to an applied receipt.

### 7.2 App-bridge additions

- `scheduleAssist(options)` in `client/daemon-client.ts` — correlated request, response type `schedule/assist/response`, timeout 120s.
- Zod schemas mirroring §5 in `server/schedule/rpc-schemas.ts`; types in `types.ts`; union registration in `shared/messages.ts`. Request has no provider field (host default is resolved daemon-side); response carries the `llmProvider`/`model` echo.

### 7.3 Reused, unchanged

- `cron-timezone.ts` (`cronToUTC`, `describeCron`, `detectTimezone`) — confirm path + card rendering.
- `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` — "Edit in form" prefill (add optional `initialValues` prop; default behavior unchanged).
- Existing schedule hooks/stores — list invalidation after confirm.
- `llm-providers-section.tsx` (Settings → General) — the assistant consumes whatever is configured there, via the existing `get/set_daemon_config` RPCs and `config.llmProviders` storage. The provider form edits the `models` list (comma-separated IDs; an existing `isDefault` marker is preserved, otherwise the first model is marked default), so every provider can be made assistant-usable entirely from the UI.
- Schedule execution (`daemonRunner`, `Executor`, Target Agent resolution) — untouched.

---

## 8. Timezone Handling

| Stage | Convention (unchanged from today) |
|-------|-----------------------------------|
| Parse | LLM produces cron in **client timezone**, using `timezone` + `clientNow` from the request |
| Validate | Daemon parses expression (`cron.go`) and computes `nextRunAt` preview |
| Display | Card shows `describeCron(expression, timezone)` + local next-run preview |
| Store | App converts via `cronToUTC` on confirm; daemon stores/evaluates UTC |
| Relative times | "tomorrow 7am" resolved against `clientNow`, never the daemon clock |

This reuses the tested timezone pipeline end-to-end; the assistant adds no new time logic except the LLM's local-cron generation, which is double-checked by daemon validation and the human-readable preview.

---

## 9. Safety, Security & Cost

1. **Confirm-before-apply (invariant)**: the daemon parse path has no code path to `Store` mutation; proposals apply only via user confirm through existing validated RPCs.
2. **Untrusted LLM output**: never executed, never rendered as HTML/markdown-with-links; proposal fields rendered as plain text; schema + semantic validation gate everything.
3. **Prompt-injection containment**: context data (schedule names, prompts of existing schedules) is quoted/escaped in the prompt; even if a malicious schedule name manipulates output, the blast radius is a wrong proposal on a confirm card — nothing applies silently.
4. **No agent session, no tools**: the parse path is a pure HTTPS chat completion — no process spawn, no tool-execution surface, nothing to clean up after a parse. Schedule execution (Target Agent) is unchanged and still governed by the existing runner rules.
5. **E2EE & egress**: the app↔daemon channel is unchanged (E2EE via relay, or local). The parse request (user sentence + schedule/agent summaries) then egresses **from the daemon host** to the user-configured LLM endpoint over HTTPS — the same trust posture as today's CLI-harness providers, which also call cloud APIs. The endpoint is explicitly user-configured in settings.
6. **Cost control**: per-connection rate limit + single in-flight + 60s timeout; each parse is one bounded completion (context ≤ ~8k chars, `max_tokens` 1024); metrics expose usage per configured provider.
7. **Privacy & credentials**: transcripts stay in app memory; `apiKey` lives only in daemon config (the existing `llmProviders` storage), is never logged and never echoed in assist responses; daemon logs metadata only, not prompts.
8. **Delete discipline**: delete proposals always carry name + cadence in the card; destructive-styled confirm; no bulk delete in v1.

---

## 10. Testing Strategy

### 10.1 Go (daemon, `-short -race`)

- `assistant_resolve_test.go`: table-driven resolver — enabled ordering, skip disabled/missing baseURL/apiKey, `isDefault` model vs first model, empty config → `no_llm_provider`.
- `client_test.go` (`internal/llm`): against `httptest.Server` — success body, auth header sent, 401/429/500 mapping, malformed JSON body, timeout.
- `assistant_prompt_test.go`: golden tests — context rendering, transcript windowing, size caps.
- `assistant_extract_test.go`: table-driven — fenced JSON, raw JSON, JSON with prose around it, truncated JSON, invalid schema, each op's required fields, semantic failures (unknown schedule, bad cron, interval too small).
- `assistant_test.go`: orchestration — rate limit, single-flight, retry-once-then-error, reference resolution, next-run preview, provider/model echo.
- Integration: stub chat-completions server returns fixture JSON → full `schedule/assist` round trip → assert proposal payload; stub returning garbage → retry → error kind; config with no providers → `no_llm_provider` error.
- Registration test: add `"schedule/assist"` to `session_register_handlers_test.go`.

### 10.2 App (Vitest)

- `use-schedule-assist` mapping tests: each op → correct existing RPC with correct payload; cadence passed through `cronToUTC`; update-merge via inspect.
- `proposal-card` render tests: create/update(diff)/pause/delete variants; warnings; disabled confirm while applying.
- Clarify/answer/error rendering; timeout → error card; `no_llm_provider` → error card with `/settings/general` deep link.
- LLM indicator chip shows resolved provider label + model.

### 10.3 Bridge (Vitest)

- Schema round-trip for assist request/response; union registration.

### 10.4 E2E (Playwright, nightly)

- Daemon under test is configured with a **stub LLM endpoint** (local test server in `llmProviders` config). Send "every weekday at 9am summarize tests" → proposal card → Confirm → schedule appears in list with expected cadence.
- Edit flow: "move it to 7:30" → update card with diff → Confirm → cadence updated.
- Ambiguity: two similarly-named schedules → clarify card.
- No-provider state: empty `llmProviders` → setup card with settings deep link.

### 10.5 Eval set (manual, pre-release)

~30 canonical utterances (create/edit/lifecycle/ambiguous/relative-time/zh+en) run against the team's real configured endpoint(s); record first-attempt correctness toward the G1 ≥80% target.

---

## 11. Rollout Phases

| Phase | Scope | Success criteria |
|-------|-------|------------------|
| **P1 — Daemon parse path** (1 wk) | Protocol types, `internal/llm` client + resolver, Assistant + prompt/extract, rate limits, Go tests | `schedule/assist` returns validated proposals for create/update/pause/resume/delete against a stub endpoint + one real configured provider |
| **P2 — App panel + create** (1 wk) | Panel UI, LLM indicator chip, proposal card, confirm → create, "Edit in form" prefill, `no_llm_provider` deep link | End-to-end NL create on web + mobile; unit tests green |
| **P3 — Edit & lifecycle ops** (1 wk) | Update diff card, clarify loop, pause/resume/delete, answer kind | All §2 flows work; E2E stub-endpoint specs pass |
| **P4 — Endpoint compatibility & hardening** (1 wk) | Verify against 2–3 real OpenAI-compatible endpoints, metrics, eval set, docs | Eval ≥80% first-attempt; metrics live; E2E nightly green |

Total: ~4 weeks. P1–P2 deliver the vertical slice (NL create) that de-risks the rest.

---

## 12. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM mis-parses cadence (e.g. wrong day) | High | Confirm card with `describeCron` + next-run preview; daemon cron validation; eval set tracking |
| Endpoint incompatibility (auth quirks, non-JSON, unsupported params) | Medium | Minimal request shape (no `response_format`, no streaming); tolerant extractor + one retry; error card naming the provider; eval per endpoint |
| No default LLM provider configured (first-run UX) | Medium | `no_llm_provider` error + setup card deep-linking to Settings → General → LLM Providers; docs; settings-UI model-editing gap called out in §13 |
| Ambiguous schedule reference | Medium | Never guess: clarify card with candidates; `contextScheduleId` bias from detail screens |
| Prompt injection via schedule/agent names | Low | Quoted context block; output untrusted; confirm gate; no tool execution |
| Parse latency (slow endpoint) | Medium | 60s call timeout / 120s client budget; pending UI with provider label |
| Token cost abuse | Low | Rate limit, single-flight, bounded context + `max_tokens`, metrics |
| Timezone confusion | Medium | Explicit tz+now in prompt; local preview on card; existing UTC storage pipeline untouched |

---

## 13. Open Questions

1. **Transcript persistence**: keep threads across app restarts (AsyncStorage) or session-only? v1: session-only.
2. **Low-risk auto-apply**: allow pause/resume without confirm for a "fast mode" toggle? v1: always confirm.
3. **Origin metadata**: tag assistant-created schedules (e.g. optional `origin: "assistant"` field) for audit/filtering? Deferred — needs protocol field; not required for v1.
4. **CLI surface**: `solo schedule assist "..."` reusing the same daemon path? Nice follow-up, not v1.
5. **Provider/model override & default semantics**: per-request `llmProviderId`/`model` override (v1.1), and/or a provider-level `isDefault` flag vs. v1's array-order precedence — decide together with settings UX. ~~Settings UI cannot edit models~~ — resolved: `llm-providers-section.tsx` now edits the models list with default-model handling.

---

## Appendix A — Parse Prompt Template (abridged)

```
You are the Solo Schedule Assistant. Convert the user's request into ONE JSON
object and output ONLY that JSON (optionally in a ```json fence). No prose.

Output shapes:
{"kind":"proposal","op":"create","name":string,"prompt":string,
 "cadence":{"type":"cron","expression":string} | {"type":"every","everyMs":number},
 "target":{"type":"agent","agentId":string} | {"type":"new-agent","config":{"provider":string,"cwd":string}},
 "maxRuns":number?,"expiresAt":string?,"summary":string,"warnings":string[]?}
{"kind":"proposal","op":"update","scheduleId":string, ...changed fields..., "summary":string}
{"kind":"proposal","op":"pause"|"resume"|"delete","scheduleId":string,"name":string,"summary":string}
{"kind":"clarify","message":string}
{"kind":"answer","message":string}

Rules:
- Cron expressions are in the client timezone given below. Minimum interval: 60000ms.
- Prefer "cron" for clock times ("at 9", "weekday mornings"); "every" for pure intervals.
- Reference agents/schedules ONLY from the context block; copy ids exactly.
- Ambiguous reference or missing required info (target agent, time) → kind="clarify".
- Pure questions about schedules → kind="answer".
- Reply in the user's language.
```

## Appendix B — Example Exchanges

**B.1 Relative time + interval**
```
User: "Ping the staging health check every 15 minutes"
→ proposal create, cadence {type:"every","everyMs":900000},
  warnings: ["no agent specified — using default new-agent with provider claude"]
```

**B.2 Ambiguous reference**
```
User: "Pause the backup"
Context: schedules "DB backup", "Files backup"
→ clarify: "Which one — 'DB backup' or 'Files backup'?"
```

**B.3 Update with diff**
```
User: "Run the nightly summary at 7:30 instead"
→ proposal update id=f3e9…, cadence {cron "30 7 * * 1-5"},
  summary: "Move 'Nightly test summary' from 09:00 to 07:30 on weekdays"
```
