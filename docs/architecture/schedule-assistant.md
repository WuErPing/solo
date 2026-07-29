# Chat-Based Schedule Assistant

> Natural-language schedule creation and editing from a chat panel in the Schedules area — the daemon parses requests with the host's configured LLM provider, the LLM only ever *proposes* changes, and confirmed proposals flow through the existing schedule RPCs.

- Status: **Implemented** (2026-07-18)
- Created: 2026-07-18

## Product Context

**Why chat-based scheduling.** Cron is the main friction in the existing form: most users cannot write `0 9 * * 1-5` and should not have to. Creating a schedule means filling several fields (prompt, cadence type, cron/interval, target agent); editing is heavier still because `schedule/update` is full-replace, so re-opening the form re-enters every field. Schedules are per-host and often numerous, so small tweaks ("move the daily report to 7:30", "pause everything on this host") mean hunting through the list. The assistant replaces this with one plain-language sentence and the ability to refer to existing schedules by name.

**Interaction model — propose, then confirm.** The LLM only ever *proposes*; nothing applies silently. Every mutation requires an explicit confirm on a structured proposal card (goal: zero accidental mutations) — there is no auto-apply path and no multi-step autonomous chaining. Four response kinds drive the conversation (protocol in §4):

- **proposal** — a confirmable card (create / update / pause / resume / delete).
- **clarify** — an ambiguous reference or missing info; the assistant never guesses (e.g. "which schedule — 'Nightly test summary' or 'Disk cleanup'?").
- **answer** — informational, no mutation (e.g. "what runs today?").
- **error** — an actionable failure card.

Representative flows: create from a sentence; edit by name with a per-field old→new diff; pause/resume/delete by name; clarify on ambiguity; plain answers for read-only questions.

**Locked product decisions.**

- A dedicated, scoped assistant panel in the Schedules area — not schedule tools injected into existing agent chat (a possible later phase).
- Parse with the host's configured LLM Providers via a stateless daemon-side completion (§5–§6): credentials stay on the daemon, no agent session or CLI harness is spawned, and it works identically for local and relay/E2EE clients.
- Execution is unchanged — confirmed proposals reuse the existing schedule RPCs and Target Agent runner behavior (§8).

**User-facing surface.**

- Entry points: "Ask AI" header button on the per-host schedules screen; "Edit with AI" on the schedule detail screen (sets `contextScheduleId`); an assistant FAB on the schedules dashboard, with a required host chip selector when more than one host is connected.
- The panel is **host-scoped** — it parses with the LLM provider configured on the same host whose schedules it manages — and always shows the host and a display-only provider/model indicator (no in-panel switching in v1; the host default is resolved per §5).
- Proposal card: op badge, the proposed fields (update shows a diff), cadence rendered human-readable via the existing `describeCron()`, a local next-run preview, any warnings, and Confirm / Edit in form / Cancel. Delete is destructive-styled and names the schedule + cadence; there is no bulk delete in v1. After a successful confirm the card collapses to an applied receipt (§8).
- States: an empty conversation offers example suggestion chips; sending shows a "Thinking…" pending bubble (120s budget); a missing provider is known up front and shown as a setup card deep-linking to Settings → General → LLM Providers; a disconnected host disables the composer. Config/endpoint errors render as a card naming the provider with a settings deep link and Retry; an invalid cadence or unknown reference becomes a clarify card stating the constraint or listing candidates.

**Scope boundaries (v1 non-goals).** No loop-type schedules via chat; no schedule tools inside regular agent chat; no daemon-side transcript persistence (transcripts live in the app, session-only); no in-panel provider/model switching (host default only); no managing provider configs beyond the existing settings UI; no multi-step autonomous schedule management.

## 1. Overview

The Schedule Assistant lets users manage cron schedules in plain language ("every weekday at 9am, summarize the nightly test runs") instead of writing cron expressions by hand. A chat panel in the app sends each request to the daemon over the new `schedule/assist` RPC; the daemon resolves the host's default LLM provider from `config.llmProviders` (Settings → General → LLM Providers — previously a config with no runtime consumer), makes one stateless chat completion, validates the output, and returns a typed proposal, clarify question, answer, or error.

**Safety invariant: the LLM never mutates schedules.** The daemon parse path has no code path to the schedule store. The LLM output is treated as untrusted text, validated against live state, and rendered as a proposal card; only an explicit user confirm calls the existing, validated `schedule/create|update|pause|resume|delete` RPCs. Execution at fire time is entirely unchanged — Target Agent resolution in `daemonRunner` (`agent` → message to the running agent; `new-agent`/`provider` → ephemeral agent via AgentManager).

**Security posture.**

- LLM output is never executed and never rendered as HTML/markdown-with-links; proposal fields render as plain text and pass schema + semantic validation before reaching the card.
- Prompt-injection containment: context data (schedule/agent names, existing prompts) is quoted in the prompt; even a manipulated output yields at most a wrong proposal on a confirm card — nothing applies silently, and the parse path has no tool-execution surface (no agent session, no process spawn).
- Credentials & privacy: `apiKey` lives only in daemon config, is never logged and never echoed in assist responses; transcripts stay in app memory; daemon logs metadata, not prompts.
- Egress: the parse request egresses from the daemon host to the user-configured endpoint over HTTPS (the same trust posture as existing CLI-harness providers); the app↔daemon channel is unchanged (E2EE via relay, or local).

## 2. Component Map

```
┌──────────────────────────────────────────────────────────────────┐
│ App                                                               │
│  components/schedule-assistant/                                   │
│   ├─ schedule-assistant-panel.tsx   AdaptiveModalSheet container  │
│   ├─ assistant-message-list.tsx     bubbles + proposal/error cards│
│   ├─ proposal-card.tsx              op badge, fields/diff, actions│
│   └─ assistant-composer.tsx         input + suggestion chips      │
│  hooks/use-schedule-assist.ts       scheduleAssist() mutation     │
│  hooks/use-assistant-thread.ts      thread state, transcript ≤10  │
│  hooks/use-proposal-confirm.ts      confirm → schedule/* RPCs     │
│  stores/schedule-assistant-store.ts session-only, keyed by host   │
└───────────────────────────────┬──────────────────────────────────┘
                                │ WebSocket: schedule/assist (120s timeout)
┌───────────────────────────────▼──────────────────────────────────┐
│ App-Bridge                                                        │
│  client/schedule-rpc.ts            scheduleAssist()               │
│  server/schedule/rpc-schemas.ts    Zod schemas                    │
│  shared/messages.ts                union registration             │
└───────────────────────────────┬──────────────────────────────────┘
                                │ WebSocket (E2EE via Relay, or local)
┌───────────────────────────────▼──────────────────────────────────┐
│ Daemon (per host)                                                 │
│  server/session_schedule_assist.go handleScheduleAssist()         │
│            │  per-session Assistant, built lazily (sync.Once)     │
│  ┌─────────▼──────────────────────────────────────────────┐      │
│  │ schedule/assistant*.go                                  │      │
│  │  assistant.go          orchestration, guards, retry     │      │
│  │  assistant_resolve.go  default provider/model resolution│      │
│  │  assistant_prompt.go   system prompt + context block    │      │
│  │  assistant_extract.go  JSON extraction + validation     │      │
│  └─────────┬──────────────────────────────────────────────┘      │
│            │ one-shot HTTPS chat completion                      │
│  ┌─────────▼──────────┐     ┌───────────────────────────────┐    │
│  │ internal/llm        │────▶ OpenAI-compatible endpoint    │    │
│  │ chat client         │     │ baseURL + apiKey + model       │    │
│  └─────────────────────┘     │ (config.llmProviders)          │    │
│                              └───────────────────────────────┘    │
│            │ validated proposal only — NO store mutation          │
│  schedule.Store ─ Executor ─ daemonRunner ─▶ Target Agent         │
│  (mutation via existing RPCs; execution path unchanged)           │
└───────────────────────────────────────────────────────────────────┘
```

The Assistant's agent seam is **read-only** (`ListAgentsWithPersisted` → snapshots, for the context block); the config seam reads `s.cfg.LLMProviders`. Unlike the runner's agent manager seam, the parse path never creates an agent session.

## 3. Parse Pipeline

Each `schedule/assist` request runs through:

1. **Guards** — `message` non-empty and ≤2000 chars, valid IANA `timezone`, transcript ≤10 turns; violations return `invalid_request`.
2. **Rate limit / single-flight** — 10 requests/min sliding window and one in-flight parse per session/connection; excess returns `rate_limited`.
3. **Resolve default provider** — read fresh from `config.llmProviders` per request (settings changes take effect immediately); unresolvable → `no_llm_provider` (§5).
4. **Build prompt** — system prompt with a JSON-only output contract plus a context block: agents and schedules (≤50 entries each), timezone, clientNow, transcript (≤10 turns); capped at ~8k chars.
5. **One completion** — a single non-streaming chat completion via `internal/llm` (§6).
6. **Extract + validate** — fenced ```` ```json ```` block else balanced-brace extraction; per-op schema validation; semantic validation against live state (cron parses, `everyMs ≥ 60000`, prompt ≤4000 chars, `expiresAt` in the future, `maxRuns > 0`, referenced ids resolved name→id with ≤5 fuzzy candidates → `clarify`).
7. **One retry** — on validation failure only, the error is appended to the prompt and the completion repeated once; a second failure returns `parse_failed`.
8. **Enrich** — compute the `nextRunAt` preview via the existing `NextRunAt`, attach warnings, echo the resolved `llmProvider`/`model`, and return the typed payload.

The daemon keeps no assistant conversation state: the client-held transcript rides each request, so a daemon restart loses nothing.

## 4. Protocol Shape

One new RPC pair in `protocol/message_schedule_assist.go`, mirrored in `app-bridge/src/server/schedule/rpc-schemas.ts` and registered in `app-bridge/src/shared/messages.ts`. Client method: `scheduleAssist()` in `app-bridge/src/client/schedule-rpc.ts` with a 120s timeout (other schedule RPCs stay at 10s).

**Request** (`schedule/assist`): `message` (≤2000 chars), `timezone` (IANA), `clientNow` (RFC3339 client wall clock, for relative times), optional `contextScheduleId` (set by "Edit with AI" on the detail screen), optional `transcript` (≤10 turns, oldest first). **No provider field** — the daemon always uses the host default.

**Response** (`schedule/assist/response`): `kind` = `proposal | clarify | answer | error`, plus:

- `proposal`: `op` = `create | update | pause | resume | delete`, `scheduleId` (for update/pause/resume/delete), `name`, `prompt`, `cadence` (local cron/interval in the request timezone), `target` (a plain `ScheduleTarget`), `cwd`, `maxRuns`, `expiresAt`, `summary`, `warnings`, `nextRunAt` (daemon-computed preview).
- `message`: clarify question / answer text / error detail.
- `error`: failure code (see below).
- `llmProvider` + `model`: echo of the resolved provider config id and model, driving the panel's provider indicator chip.

| Error code | Meaning |
|------------|---------|
| `no_llm_provider` | No enabled provider with baseURL + apiKey and a resolvable model in `config.llmProviders` |
| `llm_auth` | Endpoint rejected credentials (401/403) |
| `llm_unreachable` | Network failure or 5xx from the configured endpoint |
| `rate_limited` | Per-connection rate limit or concurrent-parse guard tripped (also endpoint 429) |
| `parse_failed` | LLM output failed extraction/validation twice |
| `invalid_request` | Request guards failed (message/timezone/transcript) |

`kind` is always driven by daemon-validated output, never asserted by the LLM alone.

## 5. Default Provider / Model Resolution

Resolved per request, read fresh from daemon config:

1. Candidates = `config.llmProviders` entries with `enabled != false`, in array order (array order = user priority, matching the settings list).
2. Provider = first candidate with non-empty `baseURL` and `apiKey`.
3. Model = that provider's `models` entry with `isDefault == true`; else the first entry.
4. No candidate or no model → `kind: "error"`, `error: "no_llm_provider"`; the app renders a setup card deep-linking to `/settings/general`.

There is no in-panel provider switching in v1; to change the parse provider the user reorders/edits the list in Settings → General → LLM Providers.

## 6. LLM Client (`daemon/internal/llm`)

A minimal OpenAI-compatible chat completion client:

- `POST {baseURL}/chat/completions` with `Authorization: Bearer <apiKey>`
- Body `{model, messages: [system, user], temperature: 0, max_tokens: 1024}`
- Non-streaming; `response_format` is **not** sent (support varies across "OpenAI-compatible" endpoints — the prompt contract + tolerant extractor + one validation retry carry the JSON guarantee)
- 60s default timeout (the 120s client budget covers one validation retry)
- Sentinel errors `ErrLLMAuth` / `ErrLLMRateLimited` mapped to the response error codes; transport errors surface immediately — no silent retry
- LLM output is treated as untrusted text; only the extractor gives it meaning

## 7. Timezone Convention

The existing storage convention is untouched — the assistant adds no new time logic:

| Stage | Convention |
|-------|------------|
| Parse | LLM produces cron in the **client timezone**, using `timezone` + `clientNow` from the request |
| Validate | Daemon parses the expression and computes the `nextRunAt` preview |
| Confirm | **App** converts local → UTC via the existing `cronToUTC()` before calling `schedule/create` / `schedule/update` |
| Store / evaluate | Daemon stores and evaluates UTC, as today |

Relative times ("tomorrow 7am") are resolved against `clientNow`, never the daemon clock.

## 8. Confirm Path — No New Mutation RPC

| Proposal op | App calls on Confirm |
|-------------|----------------------|
| `create` | `schedule/create` (payload mapped 1:1, cadence → UTC) |
| `update` | `schedule/update` (proposal fields merged over the `schedule/inspect` result — update is full-replace; the card diff comes from the same inspect) |
| `pause` | `schedule/pause` |
| `resume` | `schedule/resume` |
| `delete` | `schedule/delete` |

"Edit in form" opens the existing create modal via a new optional `initialValues` prop, so the manual form path remains a zero-new-code escape hatch. After a successful confirm the card collapses to an applied receipt.

## 9. Settings UI — Models Editing

The LLM Providers config (Settings → General → LLM Providers, daemon `config.llmProviders`) gained its first runtime consumer with this feature. The settings form (`app/src/screens/settings/llm-providers-section.tsx`) now edits the `models` list (comma-separated IDs; an existing `isDefault` marker is preserved, otherwise the first model is marked default) — previously it preserved but could not edit models, which made providers impossible to make assistant-usable from the UI.

Bundled fix: daemon config responses emitted `tmuxAgentNames: null`, which the app-bridge schema rejected — silently breaking `useDaemonConfig` on fresh installs (the assistant's no-provider pre-check depends on it). The daemon now emits `[]` (`daemon/internal/server/session_agent.go`).

## 10. Rate Limits & Resource Guards

| Guard | Value | Scope |
|-------|-------|-------|
| Rate limit | 10 assist requests / minute (sliding window) | per session/connection |
| Concurrency | 1 in-flight parse | per session/connection |
| LLM call timeout | 60s per completion (120s client RPC budget) | per request |
| Daemon egress | 1 HTTPS call per parse, +1 only on validation retry | per request |
| Message size | ≤2000 chars user message; transcript ≤10 turns; context block ~8k chars | per request |

## 11. Observability

Metrics (the `llmProvider` label is the resolved provider's config id, e.g. `"openai"`):

| Metric | Labels |
|--------|--------|
| `solo_schedule_assist_requests_total` | `llmProvider`, `kind` |
| `solo_schedule_assist_parse_failures_total` | `llmProvider`, `stage` |
| `solo_schedule_assist_duration_seconds` | `llmProvider` |
| `solo_schedule_assist_confirms_total` | `op` (reported by the app via the existing telemetry path) |

Structured logs carry request id, provider id, model, result kind, retry count, validation errors, and approximate sizes. Raw user prompts and API keys are never logged at any level; prompt logging is debug-gated and off by default.

## 12. Testing Surface

- **Go (daemon, `-short -race`)**: `internal/llm` client against `httptest.Server` (auth header, 401/429/5xx mapping, malformed body, timeout); table-driven resolver / prompt / extractor / orchestration tests; WebSocket round-trip integration against a stub chat-completions endpoint (proposal happy path, garbage → retry → error, empty config → `no_llm_provider`).
- **App-bridge (Vitest)**: schema round-trip for the assist request/response and union registration; client RPC.
- **App (Vitest, 75 tests)**: store, hooks, and components — op→RPC mapping incl. `cronToUTC` and update-merge-via-inspect, proposal card variants, clarify/answer/error rendering, `no_llm_provider` deep link.
- **E2E (Playwright, nightly)**: `app/e2e/schedule-assistant.spec.ts` with `app/e2e/helpers/stub-llm-server.ts` — the daemon under test is configured with a local stub LLM endpoint in `llmProviders`. Four specs: no-provider setup card, create-with-confirm (incl. UTC conversion), update-with-diff, ambiguity → clarify.

## 13. Related Docs

- [App-Bridge Schedule Module](../analysis/app-bridge-schedule-module.md) — schedule RPC type contract
- [Create Schedule Flow](../analysis/create-schedule-flow.md) — form-based creation path and timezone pipeline
