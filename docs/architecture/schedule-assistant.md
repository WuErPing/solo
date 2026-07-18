# Chat-Based Schedule Assistant

> Natural-language schedule creation and editing from a chat panel in the Schedules area — the daemon parses requests with the host's configured LLM provider, the LLM only ever *proposes* changes, and confirmed proposals flow through the existing schedule RPCs.

- Status: **Implemented** (2026-07-18)
- Created: 2026-07-18
- Design doc: [`../product/chat-schedule-assistant-design.md`](../product/chat-schedule-assistant-design.md) (product flows, UX, rationale)

## 1. Overview

The Schedule Assistant lets users manage cron schedules in plain language ("every weekday at 9am, summarize the nightly test runs") instead of writing cron expressions by hand. A chat panel in the app sends each request to the daemon over the new `schedule/assist` RPC; the daemon resolves the host's default LLM provider from `config.llmProviders` (Settings → General → LLM Providers — previously a config with no runtime consumer), makes one stateless chat completion, validates the output, and returns a typed proposal, clarify question, answer, or error.

**Safety invariant: the LLM never mutates schedules.** The daemon parse path has no code path to the schedule store. The LLM output is treated as untrusted text, validated against live state, and rendered as a proposal card; only an explicit user confirm calls the existing, validated `schedule/create|update|pause|resume|delete` RPCs. Execution at fire time is entirely unchanged — Target Agent resolution in `daemonRunner` (`agent` → message to the running agent; `new-agent`/`provider` → ephemeral agent via AgentManager).

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
6. **Extract + validate** — fenced ```` ```json ```` block else balanced-brace extraction; per-op schema validation; semantic validation against live state (cron parses, `everyMs ≥ 60000`, prompt ≤4000 chars, referenced ids resolved name→id with ≤5 fuzzy candidates → `clarify`).
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

## 11. Testing Surface

- **Go (daemon, `-short -race`)**: `internal/llm` client against `httptest.Server` (auth header, 401/429/5xx mapping, malformed body, timeout); table-driven resolver / prompt / extractor / orchestration tests; WebSocket round-trip integration against a stub chat-completions endpoint (proposal happy path, garbage → retry → error, empty config → `no_llm_provider`).
- **App-bridge (Vitest)**: schema round-trip for the assist request/response and union registration; client RPC.
- **App (Vitest, 75 tests)**: store, hooks, and components — op→RPC mapping incl. `cronToUTC` and update-merge-via-inspect, proposal card variants, clarify/answer/error rendering, `no_llm_provider` deep link.
- **E2E (Playwright, nightly)**: `app/e2e/schedule-assistant.spec.ts` with `app/e2e/helpers/stub-llm-server.ts` — the daemon under test is configured with a local stub LLM endpoint in `llmProviders`. Four specs: no-provider setup card, create-with-confirm (incl. UTC conversion), update-with-diff, ambiguity → clarify.

## 12. Related Docs

- [Chat-Based Schedule Assistant — Product Design](../product/chat-schedule-assistant-design.md) — flows, UX, decision rationale, rollout
- [App-Bridge Schedule Module](../analysis/app-bridge-schedule-module.md) — schedule RPC type contract
- [Create Schedule Flow](../analysis/create-schedule-flow.md) — form-based creation path and timezone pipeline
