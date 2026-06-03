# Session Memory — Implementation Spec

> Implementation spec: Persist user input and agent output at turn granularity as project-level Markdown, laying the foundation for review, retrieval, and long-term agent memory.

- Upstream design: [`docs/architecture/session-memory-persistence.md`](../architecture/session-memory-persistence.md)
- Status: **Implemented — Phase 1 + Production Hardening**
- Author: Andy
- Created: 2026-05-28
- Last revised: 2026-05-29 (P1 path convergence + P2 streaming merge + P3 isolation + P4 regression)
- Phase: Phase 1 (FileTurnRecorder + SafeBridge)

## Revision Log

| Date | Revision |
|---|---|
| 2026-05-28 | M1–M6 initial version (TurnRecorder / FileTurnRecorder / Redactor / Bridge / MemoryConfig / wiring)|
| 2026-05-29 | **P1** Path convergence: `<projectRoot>/.solo/memory/...` → **`~/.solo/memory/...`** (fixed under SoloHome); `Enabled` changed to `*bool`, nil means enabled (opt-out); removed AutoGitignore (no longer writes to project directory); `RecordTurn` removed `projectRoot` parameter |
| 2026-05-29 | **P2** Streaming merge: `bridge.OnAssistantChunk` accumulates, flush on `turn_completed` / `turn_failed` / `turn_canceled`; `SafeBridge.Close` flushes unclosed buffers on shutdown |
| 2026-05-29 | **P3** Isolation: `bridge.SafeBridge` wraps panic recovery + failure counter + circuit breaker (default 3 consecutive failures / 30s cooldown)|
| 2026-05-29 | **P4** All 16 daemon packages pass `-race` regression; `golangci-lint` 0 new warnings |

## Implementation Status

| Milestone | Scope | Status |
|---|---|---|
| M1 | `TurnRecorder` 接口 + `Turn` 数据结构 + 单元测试 | ✅ |
| M2 | `FileTurnRecorder`（async channel writer、目录/文件布局、`sessions.jsonl`）| ✅ |
| M3 | `Redactor`（regex + env + multi + `BuildRedactor`）| ✅ |
| M4 | `Bridge`（turn builder + redactor + seq/parent chain）+ `Session` hook 注入 | ✅ |
| M5 | `MemoryConfig` + daemon wiring (`memorysetup`) + auto `.gitignore` | ✅ |
| M6 | E2E 测试（config → redactor → bridge → recorder → disk）+ doc sync | ✅ |

Phase 1 implementation is located in `daemon/internal/memory*` + `daemon/internal/memorysetup` + `daemon/internal/config/memoryconfig.go` + `daemon/internal/server/memorybridge*.go`.

## Where data lives

Turn files are written to **`~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md`** (fixed under SoloHome, multiple projects share a single memory directory). `sessions.jsonl` is also in the same directory. The project root directory is **no longer** written to by the memory feature.

### Default behavior

- **Enabled**: `*bool`, **nil means enabled** (opt-out model). Explicitly write `"memory": {"enabled": false}` in `~/.solo/config.json` to disable. On build failure: nil (auto mode) only warns and skips; explicit `true` is treated as fatal.
- **Backend**: default `"file"`, with `"sqlite"` / `"middleware"` reserved for future.
- **Root**: default `"memory"`, resolved relative to SoloHome → `~/.solo/memory`.
- **QueueSize**: default 1024; **Overflow**: default `"block"` (`"error"` optional).
- **RetentionDays**: default 90 (backend implementation responsible for pruning).

### Streaming output merge

A single assistant response from the server typically generates multiple streaming timeline events (`assistant_message` increments). **The Bridge accumulates these chunks in memory and only flushes them as a single `assistant` turn file when `turn_completed` / `turn_failed` / `turn_canceled` is received.** Therefore each logical turn strictly corresponds to one `user.md` + one `assistant.md`, and will not bloat into dozens of files due to streaming events. If the daemon shuts down while a turn is incomplete, `SafeBridge.Close` flushes each agent's in-progress buffer into a separate turn.

### Main flow isolation (SafeBridge)

`bridge.SafeBridge` wraps the outer layer of `Bridge`; the daemon main session loop only interacts with SafeBridge:

- **Panic recovery**: Any panic inside hooks (`OnUserTurn` / `OnAssistantTurn` / `OnAssistantChunk` / `OnAssistantTurnEnd` / `OnSystemTurn`) is caught by `recover()`, only emitting a `slog.Warn`; it never propagates to the session.
- **Circuit breaker**: When consecutive failures reach `FailureThreshold` (default 3), the circuit opens; all calls within `FailureCooldown` (default 30s) are short-circuited; after cooldown, a single probe call is allowed, and success closes the circuit.
- **Idempotent Close**: Multiple calls only pass `Close` through to the inner implementation once, so the daemon can safely `defer` multiple times.
- **nil inner**: Passing `nil` inner makes all hooks no-ops, facilitating testing and feature flag disable paths.

The hard guarantee of main flow isolation is guarded by the `TestSafeBridge_MainFlowNotBlocked` test: under 100 panic injections, the caller goroutine must return normally within 2s.

## 1. Overview

Solo's current session data resides only in memory and is lost when the daemon shuts down. This spec injects symmetric hooks (`OnUserTurn` / `OnAssistantTurn` + streaming `OnAssistantChunk` / `OnAssistantTurnEnd`) into the **`daemon/internal/server` session layer**, asynchronously writing each turn as a markdown file under `~/.solo/memory/`. The entire pipeline is wrapped by `SafeBridge` to ensure recorder failures do not affect the main session. The implementation strictly follows the `TurnRecorder` interface, facilitating future migration to SQLite or vector memory middleware.

## 2. Goals

| ID | Goal | Metric |
|---|---|---|
| G1 | Record 100% of user/assistant turns (no omissions) | In e2e tests, turn count = actual message count of the session |
| G2 | Zero blocking on main flow | Write path is fully async, agent loop latency < 1ms (P99) |
| G3 | Write failures do not affect the session | Failures only log + metric, main flow is unaffected |
| G4 | Smooth migration to DB / middleware | `TurnRecorder` interface is stable, new implementations are hot-swappable |
| G5 | Sensitive information never persisted to disk | `.env` fragments, API key patterns are 100% redacted |

## 3. Non-goals

- No RAG / vector retrieval implementation (Phase 2+)
- No recording of subagent / tool call internal traces
- No new external Go modules introduced (Phase 1)
- No cross-project memory aggregation
- No CLI query subcommand (separate spec)

## 4. Scope

### 4.1 Phase 1 (this spec)

- `TurnRecorder` interface + `Turn` data structure
- `FileTurnRecorder` implementation (async channel writer, ULID, YAML frontmatter)
- `daemon/internal/server` hook injection at two points
- Configuration items (`memory.*`)
- Redaction rule engine (reusable with existing `security` module)
- Auto `.gitignore` suggestion
- Unit tests + integration tests

### 4.2 Subsequent phases (not in this spec)

- SQLite + FTS5 backend
- mem0 / Letta / custom vector store adapter
- `solo memory search | export | prune` CLI subcommands
- Cross-project global memory storage

## 5. Detailed Requirements

### FR-1 Turn Capture

- After the session receives and validates an inbound `UserMessage`, `OnUserTurn` is triggered before dispatching to the agent.
- After the agent output finalizes and before pushing to the client, `OnAssistantTurn` is triggered.
- Each trigger produces one `Turn`; no merging, no splitting.
- Turns within the same session are strictly sequentially numbered (`0001-`, `0002-`...).

### FR-2 Data Structure

```go
type Turn struct {
    ID        string         `yaml:"id"`        // ULID（26 字符）
    SessionID string         `yaml:"sessionId"`
    Seq       uint64         `yaml:"seq"`       // 会话内单调递增序号
    Role      TurnRole       `yaml:"role"`      // "user" | "assistant" | "system"
    Ts        time.Time      `yaml:"ts"`        // UTC，RFC3339
    Source    TurnSource     `yaml:"source"`    // "cli" | "app" | "relay"
    Content   string         `yaml:"-"`         // markdown body，不进 frontmatter
    Metadata  TurnMetadata   `yaml:"metadata,omitempty"`
    ParentID  string         `yaml:"parent,omitempty"`
}

type TurnRole   string
type TurnSource string

type TurnMetadata struct {
    Model        string            `yaml:"model,omitempty"`
    Tokens       *TokenUsage       `yaml:"tokens,omitempty"`
    ToolCalls    []string          `yaml:"toolCalls,omitempty"`
    FinishReason string            `yaml:"finishReason,omitempty"`
    Attachments  []AttachmentRef   `yaml:"attachments,omitempty"`
    Extra        map[string]any    `yaml:"extra,omitempty"`
}

type TokenUsage struct {
    Prompt     int `yaml:"prompt"`
    Completion int `yaml:"completion"`
}

type AttachmentRef struct {
    Name string `yaml:"name"`
    Kind string `yaml:"kind"` // "image" | "file"
    Size int    `yaml:"size"`
}
```

### FR-3 `TurnRecorder` Interface

```go
type TurnRecorder interface {
    // RecordTurn asynchronously enqueues a turn.
    // Implementations must be thread-safe and callable concurrently.
    // Returns nil on successful enqueue (does not mean persisted to disk);
    // Returns error on enqueue failure (e.g., channel full, recorder closed).
    RecordTurn(ctx context.Context, sessionID string, turn Turn) error

    // Flush synchronously waits for all queued turns to be persisted; used for testing and shutdown.
    Flush(ctx context.Context) error

    // Close flushes and releases resources; after calling, any RecordTurn must return ErrClosed.
    Close() error
}

var ErrClosed = errors.New("turn recorder closed")
```

### FR-4 `FileTurnRecorder` Behavior

- Internally starts **1 writer goroutine** that consumes `Turn` from a channel.
- Channel capacity defaults to **1024**; on full, handled per configured policy (`block` default / `error` optional).
- Write path: `~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md` (fixed under SoloHome)
  - Example: `~/.solo/memory/sessions/2026-05/01J.../turns/0003-user.md`
- Each turn written also appends a JSON line to `~/.solo/memory/sessions.jsonl`.
- **No overwrite** when file exists (ULID + seq are naturally unique).
- Creates directories on first write to a session, permission `0o755`; file permission `0o644`.

### FR-5 Turn File Format

```markdown
---
id: 01J5XQ8K9P...
sessionId: 01J5XQ...
seq: 3
role: assistant
ts: 2026-05-28T10:23:45Z
source: cli
metadata:
  model: solo-v1
  tokens:
    prompt: 1234
    completion: 567
  toolCalls: [Read, Bash]
  finishReason: stop
parent: 01J5XQ8K7M...
---

<raw agent output, preserving markdown>
```

- Frontmatter uses `gopkg.in/yaml.v3` (already in the project)
- Body is written as-is, with no escaping or processing
- **No** extra trailing newline (preserves original content)

### FR-6 Session Index `sessions.jsonl`

One session summary per line, **appended once on first turn write** (subsequent turns for the same session do not update the line). Index file location: `~/.solo/memory/sessions.jsonl`.

```jsonl
{"id":"01J5XQ...","startedAt":"2026-05-28T10:20:00Z","turnsCount":1,"source":"cli"}
```

Fields: `id` / `startedAt` / `turnsCount` / `source`.

### FR-7 Hook Integration

- `daemon/internal/server` adds a `MemoryBridge` interface, called directly by the session:
  - `OnUserTurn(sessionID, agentID, content)`
  - `OnAssistantTurn(sessionID, agentID, content)` (one-shot, for non-streaming scenarios like attention)
  - `OnAssistantChunk(agentID, sessionID, fragment)` (streaming accumulation)
  - `OnAssistantTurnEnd(agentID, sessionID)` (flush on `turn_completed` / `turn_failed` / `turn_canceled`)
  - `OnSystemTurn(sessionID, agentID, content)`
  - `Close() error` (flush unclosed buffers on shutdown)
- Implementation in `daemon/internal/memory/bridge.Bridge`, wrapped by `bridge.SafeBridge` before injection into the session, ensuring recorder failures do not propagate back to the session.
- Hooks perform four operations internally:
  1. Map inbound/outbound messages to `Turn`
  2. Call `Redactor.Redact(turn.Content)`
  3. Maintain `seq` / `parentID` chain within the session
  4. `recorder.RecordTurn(ctx, sess.ID, turn)`
- Hook error path: only `slog.Warn` + failure count; **never** propagates errors back to the session; consecutive failures trigger circuit breaker.

### FR-8 Redaction (Redactor)

```go
type Redactor interface {
    Redact(content string) string
}
```

Phase 1 implementation:
- `RegexRedactor`: Configurable regex list (defaults include `sk-[A-Za-z0-9]{32,}`, `ghp_[A-Za-z0-9]{36}`, `AKIA[0-9A-Z]{16}` and other common token patterns)
- `EnvFileRedactor`: Identifies `KEY=value` patterns where the key is in a known sensitive name list (`*_KEY`, `*_SECRET`, `*_TOKEN`, `PASSWORD`, `DATABASE_URL`...), replacing the entire line with `[redacted: KEY]`
- Combined as `MultiRedactor`, applied in sequence

Redacted text uses `[redacted:<reason>]` placeholders for auditability.

### FR-9 Configuration

Added in `daemon/internal/config`:

```go
type MemoryConfig struct {
    Enabled       *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"` // nil == enabled (opt-out)
    Backend       string            `yaml:"backend" json:"backend"`                     // "file" | "sqlite" | "middleware"
    RetentionDays int               `yaml:"retention_days" json:"retention_days"`       // default 90
    QueueSize     int               `yaml:"queue_size" json:"queue_size"`               // default 1024
    Overflow      string            `yaml:"overflow" json:"overflow"`                   // "block" (default) | "error"
    Root          string            `yaml:"root" json:"root"`                           // default "memory" → ~/.solo/memory
    Redact        RedactConfig      `yaml:"redact" json:"redact"`
    Safe          SafeBridgeConfig  `yaml:"safe" json:"safe"`
    SoloHome      string            `yaml:"-" json:"-"` // runtime-only, set by daemon
}

// IsEnabled reports whether the feature should run. nil or true → enabled.
func (c MemoryConfig) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

type RedactConfig struct {
    EnvFiles      bool     `yaml:"env_files" json:"env_files"`
    APIKeys       bool     `yaml:"api_keys" json:"api_keys"`
    CustomRegexes []string `yaml:"custom_regexes" json:"custom_regexes"`
    SensitiveKeys []string `yaml:"sensitive_keys" json:"sensitive_keys"`
}

type SafeBridgeConfig struct {
    FailureThreshold int           `yaml:"failure_threshold" json:"failure_threshold"` // default 3
    FailureCooldown  time.Duration `yaml:"failure_cooldown" json:"failure_cooldown"`   // default 30s
}
```

`~/.solo/config.json` example:

```json
{
  "memory": {
    "enabled": false
  }
}
```

Build failure strategy:
- `Enabled == nil` (auto): `slog.Warn` + skip feature, daemon startup is unaffected.
- `Enabled == true` (explicit): fatal, daemon refuses to start.

Defaults:
- `enabled`: `*bool`, nil means enabled (opt-out)
- `backend: "file"`
- `retention_days: 90`
- `queue_size: 1024`
- `overflow: "block"` (optional `"error"`)
- `root: "memory"` → actual path `~/.solo/memory`
- `safe.failure_threshold: 3`
- `safe.failure_cooldown: 30s`

### FR-11 Lifecycle

- On daemon startup:
  1. `cfg.Memory.SoloHome = cfg.SoloHome`
  2. If `cfg.Memory.IsEnabled()`, call `memorysetup.Build(cfg.Memory)` to get `{Bridge, Recorder}`
  3. `Bridge` is wrapped by `bridge.NewSafeBridge` (panic recovery + circuit breaker)
  4. Inject into `DaemonConfig.MemoryBridge` / `MemoryRecorder`; on session creation call `SetMemoryBridge(safeBridge)`
- On daemon shutdown (SIGTERM/SIGINT, `Daemon.Stop`):
  1. `safeBridge.Close()` — flush each agent's in-progress chunk buffer into a separate turn
  2. `recorder.Flush(ctx)` — drain the channel queue
  3. `recorder.Close()` — release resources
  - All three steps are best-effort; a single step failure only emits `slog.Warn` and does not affect the shutdown process.
- Configuration changes: Phase 1 **does not** support hot reload; daemon restart is required.

## 6. Non-functional Requirements

### NFR-1 Performance

- `RecordTurn` enqueue latency P99 < 100 μs (under 100k turns/hour load).
- Single turn write latency P99 < 5 ms (typical SSD).
- Daemon resident memory overhead < 10 MB (< 20 MB when queue is full).

### NFR-2 Reliability

- Write failures do not panic; all IO errors in `persistTurn` / `maybeWriteSessionIndex` are silently swallowed (recorder layer does not retry, does not propagate back to caller). Failure metrics go through `slog.Warn`.
- When the queue is full, behavior follows `cfg.Overflow`: `"block"` (default, controlled by `ctx` cancellation) or `"error"` (immediate return).
- Any panic is caught by `recover()` in the `SafeBridge` layer, only emitting a `slog.Warn`; consecutive failures ≥ `FailureThreshold` (default 3) trigger circuit breaker, all hook calls within `FailureCooldown` (default 30s) are short-circuited.
- On daemon shutdown, the three steps `safeBridge.Close()` / `recorder.Flush()` / `recorder.Close()` are all best-effort; a single step failure does not affect the shutdown process.

### NFR-3 Security

- All writes go through `Redactor`, enabled by default.
- File permission `0o644`, directory `0o755`; no shared permissions beyond world-readable are created.
- `turn.Content` is not printed in logs (only `turn.ID` + `turn.Seq` + byte count).

### NFR-4 Observability

Exposed metrics (reusing `daemon/internal/metrics`):
- `memory.turns_recorded_total{role,source}` (counter)
- `memory.turns_written_total{role}` (counter)
- `memory.write_errors_total{reason}` (counter)
- `memory.queue_depth` (gauge)
- `memory.queue_overflows_total{policy}` (counter)
- `memory.write_duration_seconds` (histogram)
- `memory.flush_duration_seconds` (histogram)

### NFR-5 Testability

- `TurnRecorder` interface accepts mock injection.
- `FileTurnRecorder` accepts `fs.FS` abstraction (uses `os` default, replaceable in tests).
- Time is injected via `Clock` interface (can be fixed in tests).

## 7. Data Flow Diagram

```
  User/App/Relay
        │
        ▼
┌─────────────────────────┐
│ session (server)        │
│   ├─ OnUserTurn ──────┐ │
│   └─ OnAssistantTurn ─┤ │
└─────────────────────────┼─┘
                          │
                          ▼
                  ┌──────────────┐
                  │ Hook bridge  │
                  │  - Map→Turn  │
                  │  - Redact    │
                  └──────┬───────┘
                         │
                         ▼
              ┌────────────────────┐
              │ TurnRecorder       │
              │  RecordTurn() ──┐  │
              └─────────────────┼──┘
                                │ channel (cap 1024)
                                ▼
                  ┌────────────────────────┐
                  │ writer goroutine       │
                  │  - mkdir -p            │
                  │  - write <seq>-<role>.md │
                  │  - append sessions.jsonl │
                  └──────────┬─────────────┘
                             ▼
                    <project>/.solo/memory/
```

## 8. Error Handling Matrix

| Scenario | Behavior | Observable |
|---|---|---|
| Channel full on enqueue | Handled per `overflow_policy` | `memory.queue_overflows_total{policy}` +1 |
| Directory creation failure | Backoff retry 3 times, discard + log on failure | `memory.write_errors_total{reason="mkdir"}` |
| File write failure | Backoff retry 3 times, discard + log on failure | `memory.write_errors_total{reason="write"}` |
| YAML serialization failure | Skip this turn + log (extremely rare) | `memory.write_errors_total{reason="marshal"}` |
| `Close()` timeout | Force exit + log undflushed count | `memory.flush_timeout_total` +1 |
| `RecordTurn` on closed | Returns `ErrClosed` | — |
| Redaction regex compilation failure | Fail-fast at startup | Daemon startup fails |

## 9. Test Strategy

### 9.1 Unit Tests (`daemon/internal/memory`)

- `TestTurnRecorder_RecordTurn_Success`: 1000 turns written sequentially, verify file count, content, frontmatter.
- `TestTurnRecorder_RecordTurn_Concurrent`: 10 goroutines concurrently write 1000 turns, verify no loss, no duplicates.
- `TestTurnRecorder_QueueOverflow_DropOldest`: queue_size=8, feed in 20 turns, verify latest 8 are persisted.
- `TestTurnRecorder_Flush`: Write then immediately `Flush`, verify all are persisted.
- `TestTurnRecorder_Close_Idempotent`: Repeated `Close` does not panic.
- `TestTurnRecorder_RecordAfterClose`: Returns `ErrClosed`.
- `TestRedactor_ApiKeys`: Cover OpenAI / GitHub / AWS / Anthropic / custom tokens.
- `TestRedactor_EnvFile`: Cover common sensitive key names.
- `TestSessionIndex_AppendAndUpdate`: Verify `sessions.jsonl` write frequency.

### 9.2 Integration Tests (`daemon/internal/server`)

- `TestSession_HooksFireOnBothTurns`: Simulate a complete session, verify `MockRecorder` receives 2N turns.
- `TestSession_HookErrorDoesNotAffectSession`: Inject fail-recorder, verify session completes normally.

### 9.3 E2E Tests

- Start a real daemon + CLI, run a `solo agent run`, assert that `.solo/memory/sessions/{YYYY-MM-DD}/*/turns/` contains ≥2 turn files, frontmatter is parseable, content is non-empty.

## 10. Acceptance Criteria

| AC | Verification Method |
|---|---|
| AC-1: After any Solo session ends, the corresponding `.solo/memory/` turn file count = session user + assistant total message count | E2E test |
| AC-2: No perceptible degradation in agent response latency during writes (P99 increment < 1ms) | Benchmark test |
| AC-3: Writing 100k turns without loss or duplication | Integration test |
| AC-4: Under default configuration, `.env` content, OpenAI/ GitHub tokens do not appear in any turn file | Redaction test + grep scan |
| AC-5: After `Close()`, daemon exits cleanly (no dangling goroutines) | Goroutine leak detection (`goleak`) |
| AC-6: After daemon restart, new turns for old sessions can still be appended with sequential numbering | Integration test |
| AC-7: All metrics are visible at the `/metrics` endpoint | Smoke test |
| AC-8: When a turn contains `[redacted:...]`, the original content has been replaced | Redaction unit test |

## 11. Migration Path

After Phase 1:

```
FileTurnRecorder  ─(interface unchanged)─►  SQLiteTurnRecorder
                                   │
                                   └─► One-time import script:
                                       Scan .solo/memory/sessions/*/*/turns/*.md
                                       → Parse frontmatter + body
                                       → Write to sqlite + FTS5 index
```

After Phase 2:

```
SQLiteTurnRecorder ─(interface unchanged)─► MemoryMiddlewareRecorder
                                    │
                                    ├─► mem0 adapter
                                    ├─► Letta adapter
                                    └─► Custom vector store adapter
```

`TurnRecorder` is a **stable contract**; backend implementations can evolve independently and coexist (multi-write mode).

## 12. Open Questions (Resolved in Phase 1)

| ID | Question | Resolution |
|---|---|---|
| Q1 | Should system prompt be included in a turn? | ✅ **Yes**: `bridge.OnSystemTurn` provides `role: "system"`, same chain as user/assistant. Whether it actually triggers is determined by the server hook (Phase 1 only triggers for `assistant_message` in `sendAgentStream`; system prompt trigger point deferred to a later iteration).|
| Q2 | Should `session.md` (concatenated view) be implemented in Phase 1? | ✅ **Deferred**: Phase 1 only writes per-turn files; `session.md` will be replaced by a query view in the SQLite phase.|
| Q3 | Where to store cross-project "global memory"? | ✅ **Not implemented in Phase 1**: Separate spec (suggested `~/.solo/global-memory/`).|
| Q4 | How to rebuild parent chain with multiple concurrent agents? | ✅ **Implemented**: `bridge.Bridge` has built-in `sessionState` map + per-session `sync.Mutex`, holding the lock throughout assign→record→update; seq/ParentID is strictly monotonic.|
| Q5 | Should CLI also write independently (separate from daemon)? | ✅ **No**: Only triggered in daemon layer `Session.handleSendAgentMessage` / `sendAgentStream`; CLI goes through daemon WS interface.|
| Q6 | Should redaction failure block writing? | ⚠️ **Phase 1 deviation**: Current `RegexRedactor` / `EnvFileRedactor` validate regex at compile time (invalid regex → Build failure); runtime `ReplaceAllString` cannot fail; thus there is no "redaction runtime failure" path — the original text is safe text. The `[redacted:failed]` placeholder will be added when external redactors (LLM-based / remote services) are introduced.|

## 13. Dependencies

- Internal:
  - `daemon/internal/server` (hook injection)
  - `daemon/internal/config` (configuration)
  - `daemon/internal/metrics` (metrics)
  - `daemon/internal/security` (existing redaction rule reuse)
- External (already present):
  - `gopkg.in/yaml.v3`
  - `github.com/oklog/ulid/v2` (if already in project; otherwise use `github.com/google/uuid` + sequence)
- No new external modules introduced.

## 14. Milestones

| Milestone | Deliverable | Estimate |
|---|---|---|
| M1 | `TurnRecorder` interface + `Turn` data structure + unit tests | 0.5 days |
| M2 | `FileTurnRecorder` (with async writer, directory/file layout) + unit tests | 1 day |
| M3 | `Redactor` (regex + env) + unit tests | 0.5 days |
| M4 | Session hook integration + integration tests | 0.5 days |
| M5 | Configuration + `.gitignore` auto suggestion + metrics | 0.5 days |
| M6 | E2E tests + documentation (update `docs/architecture/session-memory-persistence.md`) | 0.5 days |
| **Total** | — | **~3.5 days** |

---

**Signature**: Spec enters implementation after review approval. Recommend architecture/security reviewer to sign off on §5 / §6.3 / §12 first.
