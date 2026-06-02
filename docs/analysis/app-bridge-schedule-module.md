# Analysis: `app-bridge/src/server/schedule/`

## Overview

This directory defines the **type contract and RPC schema** for Solo's schedule subsystem — a cron/interval runner that periodically executes a prompt against an agent. It contains **zero runtime logic**; only Zod schemas and TypeScript types.

| File | Lines | Purpose |
|------|-------|---------|
| `types.ts` | 98 | Core domain models (`StoredSchedule`, `ScheduleRun`, `ScheduleCadence`, `ScheduleTarget`, etc.) |
| `rpc-schemas.ts` | 132 | Request/response message schemas for 7 RPC operations |

---

## Domain Model

### `StoredSchedule`

The canonical persisted schedule record.

| Field | Type | Notes |
|-------|------|-------|
| `prompt` | `string` | The instruction sent to the agent on each run |
| `cadence` | `ScheduleCadence` | Discriminated union: `every` or `cron` |
| `target` | `ScheduleTarget` | Discriminated union: `agent` or `new-agent` |
| `status` | `ScheduleStatus` | `active \| paused \| completed` |
| `nextRunAt` | `string \| null` | ISO-8601 timestamp of next scheduled execution |
| `lastRunAt` | `string \| null` | ISO-8601 timestamp of last execution |
| `pausedAt` | `string \| null` | When the schedule was paused |
| `expiresAt` | `string \| null` | Optional expiration deadline |
| `maxRuns` | `number \| null` | Optional execution cap |
| `runs` | `ScheduleRun[]` | Full execution history |

### `ScheduleCadence`

Discriminated union of two recurrence patterns with optional timezone support:

- **`{ type: "every", everyMs: number, timezone?: string }`** — Fixed interval in milliseconds.
- **`{ type: "cron", expression: string, timezone?: string }`** — Standard cron expression string.

**Timezone Field**:
- Optional IANA timezone name (e.g., "Asia/Shanghai", "America/New_York")
- Defaults to UTC if not specified
- Used for timezone-aware cron scheduling:
  - User enters cron in local time
  - Frontend converts to UTC for storage
  - Backend evaluates UTC expression directly
  - Display converts back to local time

### `ScheduleTarget`

Discriminated union defining what gets invoked:

- **`{ type: "agent", agentId: UUID }`** — Reuse an existing agent by UUID.
- **`{ type: "new-agent", config: AgentConfig }`** — Spawn a new agent per run with full configuration (provider, model, thinking option, approval policy, sandbox mode, MCP servers, system prompt, etc.).

### `ScheduleRun`

Single execution snapshot:

| Field | Type | Notes |
|-------|------|-------|
| `id` | `string` | Run identifier |
| `scheduledFor` | `string` | Planned execution time |
| `startedAt` | `string` | Actual start time |
| `endedAt` | `string \| null` | Completion time |
| `status` | `"running" \| "succeeded" \| "failed"` | Execution status |
| `agentId` | `UUID \| null` | Agent that executed the run |
| `output` | `string \| null` | Run output |
| `error` | `string \| null` | Error message if failed |

### `ScheduleSummary`

`StoredSchedule` with the `runs` array omitted. Used for list views to avoid transmitting full history.

---

## RPC Surface

Seven request/response pairs, all following the envelope pattern `{ requestId, ...payload, error: string | null }`.

| Request | Response | Payload Notes |
|---------|----------|---------------|
| `schedule/create` | `schedule/create/response` | Accepts `ScheduleCreateTargetSchema` (includes `self` alias); returns `ScheduleSummary` |
| `schedule/list` | `schedule/list/response` | Returns `ScheduleSummary[]` |
| `schedule/inspect` | `schedule/inspect/response` | Returns full `StoredSchedule` |
| `schedule/logs` | `schedule/logs/response` | Returns `ScheduleRun[]` for a given schedule |
| `schedule/pause` | `schedule/pause/response` | Toggles status to `paused` |
| `schedule/resume` | `schedule/resume/response` | Toggles status back to `active` |
| `schedule/delete` | `schedule/delete/response` | Returns deleted `scheduleId` |

Client timeout for all operations: **10 seconds**.

---

## Notable Design Details

### `self` Target Resolution

`ScheduleCreateTargetSchema` accepts `type: "self"` (with an `agentId`), but `ScheduleTargetSchema` (the storage model) only knows `agent` and `new-agent`. This implies `self` is a client-side convenience alias that is resolved to `agent` at creation time.

### Bounded Execution

`maxRuns` and `expiresAt` support finite/lifetime-bound schedules, not just infinite recurring jobs. A schedule naturally transitions to `completed` when either bound is reached.

### Agent Spawning Config Parity

The `new-agent` config shape mirrors the full agent creation options:

- `provider`, `model`, `modeId`, `thinkingOptionId`
- `approvalPolicy`, `sandboxMode`, `networkAccess`, `webSearch`
- `mcpServers`, `systemPrompt`
- Provider-specific `extra` fields (`codex`, `claude`)

This allows schedules to spin up fully-customized ephemeral agents with the same flexibility as interactive creation.

### Module Pattern Consistency

The directory mirrors `app-bridge/src/server/chat/` and `app-bridge/src/server/loop/`:

1. Export `types.ts` + `rpc-schemas.ts`
2. Import into `shared/messages.ts` to build the unified message union
3. Expose through `client/daemon-client.ts` as typed async methods

---

## Gaps & Observations

| # | Observation | Impact |
|---|-------------|--------|
| 1 | **No server-side handler** exists in `app-bridge/src/server/schedule/` or elsewhere under `app-bridge/src/server/`. The scheduling engine, persistence, and execution logic likely lives in the Go daemon or is not yet implemented. | This is purely a contract/schema layer. |
| 2 | `ScheduleRun.id` uses `z.string()` while `agentId` uses `z.string().uuid()`. If run IDs are UUIDs, the schema is slightly loose. | Minor inconsistency in validation strictness. |
| 3 | **No `update` or `edit` RPC**. Schedules appear immutable after creation aside from pause/resume. Any change requires delete + recreate. | Could be intentional (schedules are cheap to recreate) or a future feature gap. |
| 4 | All timestamps (`nextRunAt`, `lastRunAt`, etc.) are strings, not `Date` objects. | Consistent with bridge conventions, but consumers must parse for date arithmetic. |
| 5 | `runs` array lives inline in `StoredSchedule`. For schedules with high-frequency cadences and long lifetimes, this could become a large document. | Consider pagination or a separate `logs` query strategy if scale increases. |

---

## Verdict

A clean, well-typed schema layer. The design supports both interval and cron cadences, targets existing or newly-spawned agents, and includes lifecycle controls (pause / resume / expiration / max-runs). It is **contract-only** at this layer — the runtime implementation lives elsewhere in the system.
