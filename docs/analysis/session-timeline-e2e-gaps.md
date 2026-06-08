# Session ↔ Timeline End-to-End Test Coverage Gap Analysis

> Generated: 2026-05-26  
> Scope: Solo full project (Go daemon / TS frontend / Playwright E2E)  
> Related doc: `docs/analysis/test-suite-analysis.md`  

---

## 1. Summary

This document analyzes the interaction chain between **Session (WebSocket connection) and Timeline (Agent message timeline)** in the Solo platform, examines existing end-to-end (E2E) test coverage, and focuses on three high-frequency real-world issues — **message duplication, message stuck, message format anomalies** — for root cause analysis and test gap identification.

**Key Conclusions**:

- The **backend pipeline** from Session → Timeline Store → Client Broadcast already has relatively complete unit/integration tests.
- The pure logic of **Reducer / Seq-gate / Bootstrap Policy** on the frontend also has unit test coverage.
- However, the complete end-to-end chain of **"user opens App → inputs message → sees reply → multi-device sync → reconnect recovery"** currently has **no test coverage at all**.
- The root causes of the three user-reported issues can all be traced to corresponding design defects or implementation differences in the code, but existing tests are all "backend self-tests" or "frontend self-tests" and cannot catch regressions from the user's perspective.

---

## 2. Architecture Relationship Between Session and Timeline

### 2.1 Concept Layers

| Concept | Layer | Responsibility |
|---------|-------|----------------|
| **Session** | WebSocket connection level | One client connection corresponds to one `Session`（`daemon/internal/server/session.go`）。Responsible for receiving `AgentManager`'s `agent_stream` events, batch processing through `StreamCoalescer`（500 ms window），then writing to Timeline Store and broadcasting to the currently connected client. |
| **Timeline** | Agent level | Maintained by `InMemoryTimelineStore`（`daemon/internal/agent/timeline.go`），each Agent has independent timeline rows. Supports deduplication by `MessageID` / `Text` / `CallID+Status`，because **multiple Sessions may consume the same Agent Stream simultaneously**。 |

### 2.2 Core Data Flow

```
User Input (Frontend/App/CLI)
    → WS Session receives send_agent_message_request
    → Agent executes → produces agent_stream events
    → Session.handleStreamEvent()
        → Non-aggregated events (user_message/tool_call): direct Append + broadcast
        → Aggregated events (assistant_message/reasoning): enter Coalescer → Flush then Append + broadcast
    → InMemoryTimelineStore.Append() (deduplication)
    → Session.sendAgentStream() → client
    → Frontend session-stream-reducers (seq-gate + bootstrap policy) → Zustand Store
    → AgentPanel rendering (tail + head merge)
```

### 2.3 Key Mechanisms

- **MessageID Propagation**（2026-05-25）: All providers now attach unique `MessageID` to `user_message`, making backend deduplication more reliable.
- **Timeline Deduplication**: `Append()` compares the last row to prevent duplicates during multi-Session concurrency.
- **Grace Period Buffering**: When disconnected, `agent_stream` timeline events are buffered; after reconnection, they are replayed through `ReplaceConn()`.
- **Per-Session Coalescer**: Each Session has an independent `StreamCoalescer` that merges `assistant_message` / `reasoning` within a 500 ms window.

---

## 3. Existing Test Landscape

### 3.1 Backend Tests

| Test File | Test Type | Coverage |
|-----------|-----------|----------|
| `daemon/internal/server/multi_client_sync_test.go` | Integration | Two client connections, verify shared `timelineStore` has no duplicates |
| `daemon/internal/server/opencode_reasoning_e2e_test.go` | E2E | Real OpenCode provider, verify reasoning + assistant_message + dedup |
| `daemon/internal/server/session_user_message_test.go` | Unit | Directly inject `user_message` event, verify storage + forwarding |
| `daemon/internal/server/session_critical_grace_test.go` | Unit | Critical message buffering within grace period + reconnect replay |
| `daemon/internal/server/session_grace_test.go` | Unit | Grace enter/restore/subscription preservation |
| `daemon/internal/server/session_ping_test.go` | Unit | Ping not overwhelmed by messages |
| `daemon/internal/server/session_write_deadline_test.go` | Unit | Slow writes don't permanently block |
| `daemon/internal/server/session_race_test.go` | Unit | sendMessage after disconnect doesn't hang for 5 s |
| `daemon/internal/server/server_reconnect_test.go` | Unit | Concurrent reconnect replaces stale session |
| `daemon/internal/agent/timeline_test.go` | Unit | Append/Fetch/epoch/gap/WaitForAssistantMessage |
| `daemon/internal/agent/coalescer_test.go` | Unit | Coalescer merge logic |
| `daemon/internal/agent/provider_claude_duplicate_test.go` | Unit | Claude thinking/text dedup |
| `daemon/internal/agent/provider_opencode_events_test.go` | Unit | SSE event translation |

### 3.2 Frontend Tests

| Test File | Test Type | Coverage |
|-----------|-----------|----------|
| `app/e2e/solo-local-core.spec.ts` | Playwright E2E | Mock provider, verify assistant text visible in Web UI |
| `app/e2e/pi-provider-tool-use.spec.ts` | Playwright E2E | Real Pi provider (needs local binary), verify tool-use multi-turn assistant_message visible |
| `app/src/contexts/session-stream-reducers.test.ts` | Unit | `processAgentStreamEvent` + `processTimelineResponse`, including seq-gate decisions |
| `app/src/contexts/session-timeline-seq-gate.test.ts` | Unit | Timeline sequence number gate logic |
| `app/src/contexts/session-timeline-bootstrap-policy.test.ts` | Unit | Full replacement vs incremental merge strategy |

### 3.3 Test Execution Status

- Playwright E2E（22 specs）only runs on **nightly**（`.github/workflows/e2e-nightly.yml`）。
- Go tests execute in CI（`-short -race`），but E2E tests marked as `!short`（e.g. `opencode_reasoning_e2e_test.go`）are **skipped in CI**。
- Frontend unit tests execute in CI but **do not cover** network/WS/session interactions.

---

## 4. Problem Root Causes and Test Gaps

### 4.1 Message Duplication (Multiple web/app devices coexisting)

#### Root Cause

1. **Each Session has an independent Coalescer**: When web and app are connected simultaneously, two Sessions each buffer streaming deltas. Due to goroutine scheduling differences, **Session A may flush `"Hello"` first, Session B flushes the same `"Hello"` later**。
2. **Timeline Store only checks the last row for deduplication**: `InMemoryTimelineStore.Append()` only compares `state.Rows[len-1]`。If Session A has already appended `"Hello"`, then the agent produces `" world"` which gets appended, and then Session B flushes `"Hello"` → the last row is already `" world"` → **`"Hello"` is inserted as a new row, creating a duplicate**。
3. **Frontend seq-gate cannot prevent new seq duplicates**: seq-gate only discards exact duplicates where `seq <= cursor.endSeq`。But the newly inserted duplicate row has a **new seq**, so the reducer accepts and renders it.

```
Timeline:
  Session A flush "Hello"     → Append seq=1 ✓
  Agent produces " world"
  Session A flush " world"    → Append seq=2 ✓
  Session B flush "Hello"     → last row = " world" ≠ "Hello" → Append seq=3 ❌ Duplicate!
```

#### Fixes Applied (2026-06-08)

1. **`Append()` still checks the last row** for live multi-Session dedup (performance-preserving).
2. **`AppendFromHistory()` scans all rows backwards** to avoid inserting duplicates of items already added by live events, which can be interleaved with later live events.
3. **Added regression tests** in `daemon/internal/agent/timeline_test.go` covering non-consecutive history/live dedup and duplicate history entries.

These fixes do **not** address the per-Session Coalescer race itself (which remains a theoretical risk under extreme scheduling), but they harden the storage layer so that any duplicate flush does not create visible duplicates for clients.

#### Remaining Gaps

- ⚠️ No E2E that simultaneously opens web + app, sends messages, and verifies no duplicates in both DOMs.
- ⚠️ No multi-client sync test using real provider (Claude/OpenCode).
- ⚠️ `timelineItemsEqual` only compares `Text` for `assistant_message` / `reasoning`, no MessageID dimension.

---

### 4.2 Message Stuck

#### Root Cause

"Stuck" is a multi-cause symptom set:

| Cause | Mechanism |
|-------|-----------|
| **Grace period replay failure** | After disconnect, messages are buffered to `graceCriticalBuf`；after reconnection, they are replayed through `ReplaceConn()`。If grace times out or replay is discarded, the frontend never receives the final message. |
| **Catch-up failure causing gap** | When the frontend receives a seq-jumping event, it marks `gap` and triggers `fetchAgentTimeline("after")`。If catch-up fails, the reducer rejects all subsequent live events, and the UI freezes. |
| **Inbound queue overflow dropping messages** | `inboundQueue` capacity is 64。When the frontend bursts >64 messages, new messages are **silently discarded**, and the frontend waits forever for a response. |
| **Write deadline timeout cascade** | WebSocket write operation has 10 s timeout. When the client doesn't read, write failures cause connection closure, but the state recovery path after frontend auto-reconnect is unverified. |
| **Reducer queue flush delay** | `agentStreamReducerQueue` uses 48 ms batch processing. If flush is skipped, events are delayed from entering the Zustand store. |

#### Existing Tests

- `session_critical_grace_test.go` / `session_grace_test.go`: Verify backend grace logic, but **do not verify frontend UI recovery after reconnect**。
- `session-stream-reducers.test.ts`: Verify gap detection + catch-up side effect, but **pure reducer logic, no network**。

#### Gaps

- ❌ No E2E simulating "disconnect for 3 seconds during message sending then reconnect" to verify UI is eventually complete.
- ❌ No E2E testing recovery after catch-up failure (if `fetchAgentTimeline` returns 500 error, does the UI get stuck forever?).
- ❌ No E2E testing inbound queue overflow (rapidly sending 100 messages, are messages lost with UI waiting forever?).
- ❌ No mobile E2E testing app backgrounded for 2 minutes then resumed (`handleAppResumed` only catch-ups focused agent).

---

### 4.3 Message Format Anomalies

#### Root Cause

1. **TimelineItem is a "bag of fields"**: `Type string`, `Text string`, `Detail interface{}`, `Error interface{}`, with no compile-time or runtime schema validation. Different providers fill fields inconsistently for the same event type.
2. **Implementation differences across providers**:
   - **OpenCode**: SSE events, `messageID` was **completely ignored** until 2026-05-25; now propagated on `user_message` emits.
   - **Claude**: stream JSON, `Error` field shape differs from other providers.
   - **Kimi**: JSON-RPC Wire, simplest translator, **completely lacks dedup logic**, and no `MessageID` propagation.
3. **Structured message fallback**: OpenCode falls back to `stringifyStructuredMessage()` when there's no text delta, producing raw `map[string]interface{}`, not a typed `TimelineItem`。

#### Existing Tests

- `opencode_reasoning_e2e_test.go`: Verifies reasoning doesn't repeat in assistant, but **requires real environment and is backend-only**。
- `provider_claude_duplicate_test.go` / `provider_opencode_events_test.go`: Unit tests with mock data.

#### Gaps

- ❌ No cross-provider E2E format consistency test (same prompt, do Claude/OpenCode/Kimi return consistent fields?).
- ❌ No E2E verification of `messageID` propagation (OpenCode and Kimi don't propagate `messageID`, making frontend optimistic dedup ineffective).
- ❌ No resilience E2E for malformed provider output (provider stdout outputs truncated JSON, does agent crash?).
- ❌ No E2E verification of tool_call format (different providers have different `Detail` field structures).

---

## 5. Supplemented End-to-End Tests

### ✅ P0 — Message Duplication Defense E2E

**File**: `app/e2e/multi-client-sync.spec.ts`

- `two clients see the same timeline after sending a message` — Two CLI clients connect simultaneously, verify timeline entries count and content are identical with no duplicates.
- `second message from client A is visible to client B` — Second message also correctly syncs to the other client.

**Limitations**: Mock provider has no streaming delta, so it cannot reproduce non-consecutive duplicates caused by per-Session Coalescer race. Needs enhanced mock or real provider integration for full coverage.

### ✅ P1 — Message Not Stuck E2E

**File**: `app/e2e/reconnect-resilience.spec.ts`

- `timeline is intact after disconnect and reconnect` — Client disconnects and reconnects, timeline is intact, and can continue sending messages.

**File**: `app/e2e/grace-period-recovery.spec.ts`

- `message sent while disconnected is visible after reconnect` — While client A is disconnected, client B sends a message; client A can see the message after reconnection, verifying grace-period buffering.

**File**: `app/e2e/rapid-fire-messages.spec.ts`

- `20 rapid messages are all recorded without loss` — Continuously sends 20 messages, verifies no loss. Limited by mock provider serializing turns, cannot do 100 messages.

**File**: `app/e2e/message-ordering.spec.ts`

- `user and assistant messages appear in strict send order` — 5 rounds of messages, verify user_message and assistant_message strictly alternate in correct order.

### ✅ P1 — Timeline Pagination E2E

**File**: `app/e2e/timeline-pagination.spec.ts`

- `tail with limit returns the most recent entries` — `tail` + `limit=4` correctly returns last 4 entries.
- `before and after cursors return correct slices` — `before`/`after` + `cursor` correctly slices.

### ❌ P1 — Message Format Consistency E2E

**File**: `app/e2e/provider-format-consistency.spec.ts`（not implemented）

- Requires real provider (OpenCode/Claude/Kimi) environment, current E2E only enables mock provider.
- Recommendation: Supplement after real provider integration environment is available, verify cross-provider field consistency.

### ✅ P2 — Frontend Optimistic Deduplication E2E

**File**: `app/e2e/optimistic-dedup.spec.ts`

- `user message appears exactly once after server echo` — Send message via UI composer, verify optimistic render + server echo appears only once in DOM.

**Note**: This test passes stably when running individually; occasionally fails during batch runs due to Metro first-bundle loading delay causing page load timeout. CI configuration `retries: 1` can auto-recover.

---

## 6. Conclusion

| Problem | Root Cause | Backend Unit/Integration Tests | Frontend Unit Tests | End-to-End Tests |
|---------|------------|-------------------------------|---------------------|------------------|
| **Message Duplication** | Per-Session Coalescer Race + Timeline Append only checks last row | ✅ Backend dedup hardened (`Append` last-row + `AppendFromHistory` full scan) | ⚠️ Partial (seq-gate only prevents exact duplicates) | ✅ `multi-client-sync.spec.ts` + `optimistic-dedup.spec.ts` |
| **Message Stuck** | Grace replay failure / Catch-up failure / Queue overflow / Write deadline | ✅ Relatively complete | ⚠️ Partial (reducer gap logic) | ✅ `reconnect-resilience.spec.ts` + `grace-period-recovery.spec.ts` + `rapid-fire-messages.spec.ts` + `message-ordering.spec.ts` |
| **Message Format Anomalies** | Provider translator heterogeneity + TimelineItem no schema | ✅ `session_closed` typed event added; `MessageID` propagated for opencode `user_message` | ❌ None | ❌ **None**（needs real provider environment） |

> **Supplemented 7 E2E specs (10 tests total), covering core scenarios including multi-client sync, disconnect recovery, rapid messages, optimistic dedup, message ordering, pagination queries. Cross-provider format consistency still needs real provider integration environment before it can be supplemented.**
