# Chat-Based Schedule Assistant — Product Design

> **Document type**: Product / architecture design
> **Date**: 2026-07-17
> **Baseline version**: Solo v0.10.0
> **Status**: Proposed (design review)
> **Audience**: Backend, frontend, product
> **Related docs**:
> - [Loop Schedule Implementation Spec](loop-schedule-spec.md)
> - [App-Bridge Schedule Module](../analysis/app-bridge-schedule-module.md)
> - [Create Schedule Flow](../analysis/create-schedule-flow.md)
> - [ADR-001: Shared Agent Template](../decisions/adr-001-shared-agent-template-for-loop-and-schedule.md)

---

## Executive Summary

Add a **Schedule Assistant**: a chat panel inside the Schedules area where users create and edit schedules in natural language ("every weekday at 9:00, summarize overnight agent activity"), powered by any of Solo's existing LLM providers.

Two locked decisions (confirmed with product owner):

1. **Dedicated assistant panel** in the Schedules area — not schedule tools inside existing agent chat. The assistant is scoped, predictable, and safe; agent-chat tool injection may follow later as a separate phase (out of scope here).
2. **Reuse agent providers** — the daemon parses natural language by spawning an ephemeral agent session through the existing provider registry (Claude / Kimi / OpenCode / Pi), forcing structured JSON output. No direct-API gateway, no new credentials, local-first/E2EE properties intact. This mirrors the `DecisionProvider` pattern already specced for the Loop Controller.

Core safety invariant: **the LLM never mutates schedules**. It only produces a *proposal*; the user confirms via a structured card, and confirmation flows through the existing, validated `schedule/create|update|pause|resume|delete` RPCs. Only **one new RPC pair** (`schedule/assist`) is added.

---

## 1. Problem & Goals

### 1.1 Problem

- Schedule creation today requires filling a form: prompt text, cadence type, cron expression or interval, target agent. Cron syntax is the main friction — most users cannot write `0 9 * * 1-5` and should not have to.
- Editing means re-opening the form and re-entering every field (`schedule/update` is full-replace).
- Schedules are per-host and often numerous; small tweaks ("move the daily report to 7:30", "pause everything on this host") require hunting through the list.

### 1.2 Goals

| # | Goal | Success signal |
|---|------|----------------|
| G1 | Create a schedule from one natural-language sentence | ≥80% of common requests produce a correct proposal on first attempt (mock + real-provider eval set) |
| G2 | Edit/pause/resume/delete existing schedules by referring to them by name | Correct schedule resolution for unambiguous names; clarify loop for ambiguous ones |
| G3 | Zero accidental mutations | Every mutation passes an explicit confirm card; no auto-apply path exists |
| G4 | Provider-agnostic | Feature works with all 4 built-in providers + Mock; no provider-specific UI |
| G5 | No regression to existing schedule stack | All existing schedule RPCs, UI, timezone pipeline unchanged and reused |

### 1.3 Non-goals (v1)

- Loop-type schedules (`type: "loop"`) via chat — wait for Loop Schedule spec to land.
- Schedule tools inside regular agent chat sessions (future phase).
- Daemon-side conversation persistence (transcripts live in the app).
- Direct HTTP API integration with OpenAI/Anthropic/etc.
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

- Provider returns unparseable output → daemon retries once with the validation error appended → still failing → error card: "Couldn't understand that as a schedule change. Try rephrasing, or use the form."
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

Schedules live on a specific daemon; the assistant is therefore **host-scoped**. The panel always displays which host it is talking to.

### 3.2 Panel layout

- Mobile: bottom sheet (existing `@gorhom/bottom-sheet` pattern) over the schedules screen.
- Web/desktop: right-side docked panel, ~380px.
- Contents: host chip, provider chip (see 3.3), scrollable message list, composer with send button.

Message types in the list:

| Type | Rendering |
|------|-----------|
| User text | Standard chat bubble |
| Assistant text (clarify/answer) | Standard bubble |
| **Proposal card** | Structured card, see 3.4 |
| Error card | Muted bubble with retry hint |
| Applied receipt | Collapsed card: "Created ✓ / Updated ✓ / Paused ✓" + link to schedule detail |

### 3.3 Provider selector

- Compact chip in the panel header showing the active provider (Claude / Kimi / OpenCode / Pi), tap to switch.
- Default: last-used provider for the assistant (persisted in app settings store); fallback: host's default provider.
- Optional model override deferred to v1.1 (provider default model is used).

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
- **Edit in form** opens the existing `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` pre-filled with proposal values — the user can adjust and save through the normal path. This guarantees a manual escape hatch that needs zero new form code.

### 3.5 Empty, loading, and offline states

- Empty conversation: 3 example prompts as suggestion chips ("Every morning at 9, run the daily standup summary", "Pause the disk cleanup", "What runs this week?").
- Sending: streaming-style pending bubble ("Thinking…" with provider name); parse timeout 90s → error card with Retry.
- Host disconnected: composer disabled with reconnect hint.

---

## 4. Architecture

### 4.1 Overview

```
┌──────────────────────────────────────────────────────────────┐
│ Solo App                                                      │
│  ScheduleAssistantPanel                                       │
│   ├─ message list (user / assistant / proposal / error)       │
│   ├─ composer + provider chip + host chip                     │
│   └─ ProposalCard → Confirm → existing schedule/* RPCs        │
└───────────────────────────┬──────────────────────────────────┘
                            │ WebSocket (E2EE via Relay, or local)
┌───────────────────────────▼──────────────────────────────────┐
│ Solo Daemon                                                   │
│                                                               │
│  session_schedule_assist.go   handleScheduleAssist()          │
│            │                                                  │
│  ┌─────────▼───────────────────────────────────────────┐     │
│  │ daemon/internal/schedule/ (assistant files)          │     │
│  │  · Assistant      — orchestration, rate limit, retry │     │
│  │  · PromptBuilder  — system prompt + context block    │     │
│  │  · Extractor      — JSON extraction + validation     │     │
│  │  · Context        — agents + schedules summaries     │     │
│  └─────────┬───────────────────────────────────────────┘     │
│            │ one-shot completion                              │
│  ┌─────────▼──────────┐     ┌──────────────────────────┐     │
│  │ AgentManager        │────▶ Provider harness          │     │
│  │ (ephemeral agent,   │     │ claude --print / kimi    │     │
│  │  labels: source=    │     │ --wire / opencode SSE /  │     │
│  │  schedule-assistant)│     │ pi                       │     │
│  └─────────────────────┘     └──────────────────────────┘     │
│            │ validated proposal only — NO mutation            │
│  schedule.Store (mutation happens only via existing RPCs)     │
└───────────────────────────────────────────────────────────────┘
```

### 4.2 Why reuse provider harnesses (decision rationale)

| Alternative | Verdict | Reason |
|-------------|---------|--------|
| **Ephemeral agent via AgentManager (chosen)** | ✅ | No new credentials; user's existing provider auth Just Works; E2EE/local-first intact; same seam the Loop Controller spec already establishes (`DecisionProvider`); works offline/local |
| Direct API gateway in daemon | ❌ | New API-key management surface; duplicates provider config; bypasses the CLI-harness model that is Solo's core architecture |
| Parse in the app | ❌ | App holds no provider credentials by design; would break the trust boundary |

### 4.3 Stateless parse with client-held transcript

Each `schedule/assist` request carries the bounded transcript (last ≤10 turns). The daemon keeps **no** assistant conversation state.

Rationale:

- All four provider harnesses support one-shot prompting; keeping ephemeral sessions alive across turns would add lifecycle, timeout, and orphan-cleanup complexity for marginal quality gain.
- Daemon restart loses nothing; crash mid-parse only fails one request.
- Context the LLM actually needs (agents, schedules, timezone, now) is re-injected fresh per request — always accurate, never stale.

Rejected alternative: daemon-side conversation store (adds persistence, migration, and privacy surface for little benefit).

### 4.4 Context enrichment happens daemon-side

The app sends only `{message, provider, timezone, clientNow, transcript, contextScheduleId?}`. The daemon injects:

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
    Provider string `json:"provider"`           // "claude" | "kimi" | "opencode" | "pi"
    Model    string `json:"model,omitempty"`    // optional provider-specific override (v1.1)

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
    Error     *string                 `json:"error,omitempty"`    // transport-level failure (rate limit, provider down)
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
- `Cadence` in a proposal is expressed in the **client timezone** (local cron). The daemon validates it parses (via existing `cron.go`) and computes `NextRunAt`. On confirm, the **app** converts to UTC with the existing `cronToUTC()` before calling `schedule/create` / `schedule/update` — the storage convention ("frontend converts local → UTC; backend evaluates UTC") stays exactly as today.
- `pause` / `resume` / `delete` proposals carry only `Op + ScheduleID + Name + Summary`.
- Client timeout: 120s (parse can take tens of seconds on slower providers). All other schedule RPCs stay at 10s.

### 5.3 Confirm path — no new mutation RPC

| Proposal op | App calls on Confirm |
|-------------|----------------------|
| `create` | `schedule/create` (payload mapped 1:1, cadence → UTC) |
| `update` | `schedule/update` (full record: proposal fields merged over the inspected current record) |
| `pause` | `schedule/pause` |
| `resume` | `schedule/resume` |
| `delete` | `schedule/delete` |

For `update`, the app first fetches `schedule/inspect` and merges, because `schedule/update` is full-replace; the diff shown on the card is computed from the same inspect result.

---

## 6. Daemon Design

### 6.1 New files

```
daemon/internal/schedule/
├── assistant.go           # Assistant: orchestration, rate limit, retry, guards
├── assistant_prompt.go    # PromptBuilder: system prompt + context rendering
├── assistant_extract.go   # Extractor: JSON extraction + schema validation
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
    store     *Store
    agents    assistantAgentManager // same shape as scheduleAgentManager in schedule_runner.go
    limiter   *rateLimiter          // per-connection
    logger    *slog.Logger
}

func (a *Assistant) Assist(ctx context.Context, req protocol.ScheduleAssistRequest) (*protocol.ScheduleAssistResponsePayload, error)
```

Flow:

1. **Guard**: validate `Message` non-empty/≤2000 chars, `Timezone` valid IANA, `Provider` registered; enforce rate limit and single in-flight parse per connection.
2. **Build prompt** (§6.3): system prompt + context block (agents, schedules, timezone, clientNow) + transcript + user message.
3. **One-shot completion** (§6.4): spawn ephemeral agent, collect final assistant text.
4. **Extract + validate** (§6.5): JSON → typed intent → semantic validation against store/agent state. On failure, **one retry** with the validation error appended to the prompt.
5. **Resolve references**: schedule/agent names → ids; unknown → `clarify` with candidate list.
6. **Enrich**: compute `NextRunAt` preview via existing `NextRunAt(cadence, now)`; attach warnings.
7. Return typed payload. The Assistant **never** calls `Store.Create/Update/...`.

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

### 6.4 One-shot completion via AgentManager

```go
func (a *Assistant) runCompletion(ctx context.Context, provider, systemPrompt, userPrompt string) (string, error)
```

- Build `protocol.AgentSessionConfig` for the chosen provider with `SystemPrompt` set (shared `AgentTemplate` fields per ADR-001), a neutral `cwd` (daemon home), and **tools disabled where the provider supports it** (e.g. Claude print mode with empty allowed-tools) — the parse agent needs no tools and must not execute any.
- `CreateAgent` with labels `{source: "schedule-assistant", requestId: …}` → `SendAgentMessage` → accumulate assistant text from agent stream/timeline events (same accumulation pattern as `memory.Bridge`) → wait for terminal status (same wait pattern as `daemonRunner.waitForAgent`) → `DeleteAgent` in all paths (defer).
- Timeout: 90s (context deadline), distinct from the 5-minute schedule-run budget.
- Output treated as **untrusted text**; only the Extractor gives it meaning.

Provider matrix (v1):

| Provider | Mode | Notes |
|----------|------|-------|
| Claude | print/stream-json | Strong JSON compliance; primary target |
| Kimi | wire (JSON-RPC) | Works; verify system-prompt plumbing |
| OpenCode | SSE | Works; verify |
| Pi | terminal harness | Best-effort; weakest JSON compliance — tolerated via retry, documented |
| Mock | dev-only | Returns canned JSON from fixtures; basis of integration tests |

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
| Concurrency | 1 in-flight parse | per connection (429-style error otherwise) |
| Parse timeout | 90s | per request |
| Ephemeral agent TTL | killed at parse end (defer DeleteAgent) | always |
| Message size | ≤ 2000 chars user message; transcript ≤ 10 turns | per request |

### 6.7 Metrics & logging

```
solo_schedule_assist_requests_total{provider, kind}
solo_schedule_assist_parse_failures_total{provider, stage}
solo_schedule_assist_duration_seconds{provider}
solo_schedule_assist_confirms_total{op}        // reported by app via existing telemetry path
```

slog: request id, provider, kind, retry count, validation errors, token-ish sizes. **Never log raw user prompts at info level** (privacy); debug-gated only.

---

## 7. App Design

### 7.1 New components & hooks

```
app/src/components/schedule-assistant/
├── schedule-assistant-panel.tsx    # sheet/dock container, host+provider chips
├── assistant-message-list.tsx      # bubbles + cards
├── proposal-card.tsx               # op badge, fields/diff, actions
└── assistant-composer.tsx          # input + send + suggestion chips

app/src/hooks/
├── use-schedule-assist.ts          # mutation: client.scheduleAssist()
└── use-assistant-thread.ts         # thread state, transcript windowing
```

- State: per-host thread in a small Zustand store (`useAssistantStore`, keyed by `serverId`); session-only persistence in v1 (no disk).
- `useScheduleAssist` wraps the bridge call with React Query mutation; on `proposal`, pushes a card message; on `clarify`/`answer`, pushes a bubble.
- Confirm handler: maps op → existing hooks (`useCreateSchedule`, update/pause/resume/delete equivalents), invalidating schedule queries on success; card collapses to an applied receipt.

### 7.2 App-bridge additions

- `scheduleAssist(options)` in `client/daemon-client.ts` — correlated request, response type `schedule/assist/response`, timeout 120s.
- Zod schemas mirroring §5 in `server/schedule/rpc-schemas.ts`; types in `types.ts`; union registration in `shared/messages.ts`.

### 7.3 Reused, unchanged

- `cron-timezone.ts` (`cronToUTC`, `describeCron`, `detectTimezone`) — confirm path + card rendering.
- `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` — "Edit in form" prefill (add optional `initialValues` prop; default behavior unchanged).
- Existing schedule hooks/stores — list invalidation after confirm.

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
4. **No tool execution**: parse sessions run tool-disabled where supported; ephemeral agents are destroyed after each parse.
5. **E2EE**: unchanged — assist RPC rides the existing encrypted session.
6. **Cost control**: per-connection rate limit + single in-flight + 90s timeout; each parse is one bounded LLM call (context ≤ ~8k chars); metrics expose usage.
7. **Privacy**: transcripts stay in app memory; daemon logs metadata only, not prompts.
8. **Delete discipline**: delete proposals always carry name + cadence in the card; destructive-styled confirm; no bulk delete in v1.

---

## 10. Testing Strategy

### 10.1 Go (daemon, `-short -race`)

- `assistant_prompt_test.go`: golden tests — context rendering, transcript windowing, size caps.
- `assistant_extract_test.go`: table-driven — fenced JSON, raw JSON, JSON with prose around it, truncated JSON, invalid schema, each op's required fields, semantic failures (unknown schedule, bad cron, interval too small).
- `assistant_test.go`: orchestration — rate limit, single-flight, retry-once-then-error, reference resolution, next-run preview.
- Integration: Mock provider returns fixture JSON → full `schedule/assist` round trip → assert proposal payload; mock returning garbage → retry → error kind.
- Registration test: add `"schedule/assist"` to `session_register_handlers_test.go`.

### 10.2 App (Vitest)

- `use-schedule-assist` mapping tests: each op → correct existing RPC with correct payload; cadence passed through `cronToUTC`; update-merge via inspect.
- `proposal-card` render tests: create/update(diff)/pause/delete variants; warnings; disabled confirm while applying.
- Clarify/answer/error rendering; timeout → error card.

### 10.3 Bridge (Vitest)

- Schema round-trip for assist request/response; union registration.

### 10.4 E2E (Playwright, nightly)

- Against Mock provider: send "every weekday at 9am summarize tests" → proposal card → Confirm → schedule appears in list with expected cadence.
- Edit flow: "move it to 7:30" → update card with diff → Confirm → cadence updated.
- Ambiguity: two similarly-named schedules → clarify card.

### 10.5 Eval set (manual, pre-release)

~30 canonical utterances (create/edit/lifecycle/ambiguous/relative-time/zh+en) run against Claude and Kimi; record first-attempt correctness toward the G1 ≥80% target.

---

## 11. Rollout Phases

| Phase | Scope | Success criteria |
|-------|-------|------------------|
| **P1 — Daemon parse path** (1 wk) | Protocol types, Assistant + prompt/extract, Claude + Mock, rate limits, Go tests | `schedule/assist` returns validated proposals for create/update/pause/resume/delete with mock + Claude |
| **P2 — App panel + create** (1 wk) | Panel UI, provider chip, proposal card, confirm → create, "Edit in form" prefill | End-to-end NL create on web + mobile; unit tests green |
| **P3 — Edit & lifecycle ops** (1 wk) | Update diff card, clarify loop, pause/resume/delete, answer kind | All §2 flows work; E2E mock specs pass |
| **P4 — Provider coverage & hardening** (1 wk) | Kimi/OpenCode/Pi verification, metrics, eval set, docs | Eval ≥80% first-attempt on Claude+Kimi; metrics live; E2E nightly green |

Total: ~4 weeks. P1–P2 deliver the vertical slice (NL create) that de-risks the rest.

---

## 12. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM mis-parses cadence (e.g. wrong day) | High | Confirm card with `describeCron` + next-run preview; daemon cron validation; eval set tracking |
| Provider emits non-JSON | Medium | Tolerant extractor + one retry + graceful error card; Mock fixtures lock the contract |
| Ambiguous schedule reference | Medium | Never guess: clarify card with candidates; `contextScheduleId` bias from detail screens |
| Prompt injection via schedule/agent names | Low | Quoted context block; output untrusted; confirm gate; no tool execution |
| Parse latency (slow provider) | Medium | 120s client timeout, pending UI with provider name, one-click provider switch |
| Token cost abuse | Low | Rate limit, single-flight, bounded context, metrics |
| Timezone confusion | Medium | Explicit tz+now in prompt; local preview on card; existing UTC storage pipeline untouched |
| Weaker providers (Pi) produce poor JSON | Low | Documented capability note; retry; user can switch provider chip |

---

## 13. Open Questions

1. **Transcript persistence**: keep threads across app restarts (AsyncStorage) or session-only? v1: session-only.
2. **Low-risk auto-apply**: allow pause/resume without confirm for a "fast mode" toggle? v1: always confirm.
3. **Origin metadata**: tag assistant-created schedules (e.g. optional `origin: "assistant"` field) for audit/filtering? Deferred — needs protocol field; not required for v1.
4. **CLI surface**: `solo schedule assist "..."` reusing the same daemon path? Nice follow-up, not v1.
5. **Model override** per request (provider chip already allows switching providers; model selection deferred to v1.1).

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
