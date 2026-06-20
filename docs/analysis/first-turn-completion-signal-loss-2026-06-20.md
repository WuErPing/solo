# First-Turn Completion Signal Loss — Root Cause Analysis

**Date**: 2026-06-20
**Symptom**: New agent → first conversation turn → content arrives but UI stays in loading animation. Affects multiple providers.
**Severity**: High — blocks core user flow

---

## Executive Summary

Two independent bugs combine to cause the symptom:

1. **Systemic app-side race condition** (affects ALL providers): During agent initialization, `agent_update` messages are buffered. If `turn_completed` arrives while the agent is not yet in the Zustand store, the optimistic status update is **silently skipped** due to a `currentAgent` null guard. The UI stays in `status === "running"` until the timeline fetch completes and flushes the buffer — which can take seconds or never happen if the fetch fails.

2. **OpenCode provider bug** (affects OpenCode only): `finishForegroundTurn()` in `provider_opencode_events.go:250` has a missing `case protocol.TurnCompletedStreamEvent` branch. The `turn_completed` stream event is **silently dropped** — never emitted to the dispatcher, never sent to the client. The `agent_update(idle)` is still sent via the Run-return path, but the timeline lacks the terminal event.

---

## Part 1: First Principles — The Turn Completion Signal Path

### What is the "end signal"?

The app's loading animation (`WorkingIndicator` — 3 bouncing dots) is controlled by **exactly one condition**:

```typescript
// app/src/components/agent-stream-view.tsx:592
const showWorkingIndicator = agent.status === "running";
```

For the animation to stop, `agent.status` must transition from `"running"` to `"idle"`. There are **two paths** that can do this:

| Path | Trigger | Mechanism |
|------|---------|-----------|
| **A: Optimistic** (stream event) | `turn_completed` / `turn_failed` arrives via `agent_stream` | Reducer calls `deriveOptimisticLifecycleStatus()` → sets status to `"idle"` |
| **B: Authoritative** (state event) | `agent_update` with `status: "idle"` arrives | `applyAgentUpdatePayload()` → `applyAuthoritativeAgentSnapshot()` → sets status |

Both paths must work for reliable turn completion. If both fail, the UI is permanently stuck.

### The daemon sends BOTH signals on turn completion

```
Provider emits TurnCompletedStreamEvent
         │
         ├──→ Event pipeline → handleStreamEvent → applyTerminalStreamState
         │         │                                  ├── SetLifecycle(AgentIdle)
         │         │                                  └── emitState(agent) → sends agent_update(idle) [Path B]
         │         └── agent.Emit + m.emit → session → sendAgentStream
         │                                        └── sends agent_stream(turn_completed) [Path A]
         │
         └──→ session.Run() returns
                  └── manager.go:561-566 → SetLifecycle(AgentIdle) + emitState(agent)
                      └── sends agent_update(idle) again [Path B, duplicate]
```

**Key insight**: Path A (stream event) and Path B (state event) are **independent**. Either one can transition the status to `"idle"`. But Path A is **faster** (no need to wait for `Run()` to return), and Path B is **more reliable** (always fires on the Run-return path).

---

## Part 2: Root Cause #1 — App-Side First-Turn Race Condition

### The initialization buffer

When a new agent is created, the app calls `ensureAgentIsInitialized(agentId)`:

```typescript
// app/src/hooks/use-agent-initialization.ts:79
setAgentInitializing(agentId, true);  // sets initializingAgents[agentId] = true
// ...
client.fetchAgentTimeline(agentId, timelineRequest);  // async RPC
```

While `initializingAgents[agentId]` is `true`, the `agent_update` handler **buffers** all updates instead of applying them:

```typescript
// app/src/contexts/session-context.tsx:1069-1085
const isSyncingHistory =
  session?.initializingAgents.get(agentId) === true && Boolean(getInitDeferred(initKey));

if (isSyncingHistory) {
  bufferPendingAgentUpdate(serverId, agentId, update);  // ← BUFFERED, not applied
  return;
}
applyAgentUpdatePayload(update);  // ← only reached if NOT syncing
```

**Consequence**: During initialization, the agent is **never added to the Zustand store** via `agent_update`. The agent only enters the store when:
- The timeline response arrives and includes `payload.agent` (line 968-974), OR
- The `flush_pending_updates` side effect fires (line 299-303) and applies the buffered update

### The `currentAgent` null guard

The stream event reducer has a **null guard** that prevents the optimistic status update when the agent is not in the store:

```typescript
// app/src/contexts/session-stream-reducers.ts:522-542
if (
  currentAgent &&                    // ← THIS CHECK
  (event.type === "turn_completed" ||
   event.type === "turn_canceled" ||
   event.type === "turn_failed")
) {
  const optimisticStatus = deriveOptimisticLifecycleStatus(currentAgent.status, event);
  // ... sets agentPatch to { status: "idle", ... }
}
```

If `currentAgent` is `null` (agent not in store), the entire block is skipped. The `turn_completed` event is still added to the timeline, but **the agent status is NOT updated**.

### The race condition timeline

```
Time │ Event
─────┼────────────────────────────────────────────────────────────────
 T0  │ User creates agent with initialPrompt
 T1  │ Daemon creates agent, sends agent_update(idle)  ← may or may not arrive before T3
 T2  │ Daemon starts first turn, sends agent_update(running)
 T3  │ App receives createAgent response, navigates to agent panel
 T4  │ Agent panel calls ensureAgentIsInitialized(agentId)
     │   → initializingAgents[agentId] = true
     │   → fetchAgentTimeline(agentId, ...) starts
 T5  │ agent_update(running) arrives → BUFFERED (initializingAgents is true)
     │   → agent NOT in store
 T6  │ Stream events (timeline, content) arrive → processed by reducer
     │   → currentAgent is null → optimistic updates have no effect on status
 T7  │ turn_completed stream event arrives → enqueued in reducer
 T8  │ Reducer flushes (setTimeout) → currentAgent is still null
     │   → ❌ Optimistic status update SKIPPED
     │   → agent.status stays "running" (or undefined)
 T9  │ agent_update(idle) arrives → BUFFERED (overwrites running in buffer)
 T10 │ Timeline response arrives
     │   → payload.agent added to store (status from daemon)
     │   → flush_pending_updates fires → flushes buffered agent_update(idle)
     │   → ✅ agent.status = "idle" → animation stops
     │   → initializingAgents cleared
```

**The window between T8 and T10 is the stuck state.** During this window:
- Content has been received and rendered
- `turn_completed` has been processed (added to timeline)
- But `agent.status` is NOT `"idle"` because:
  - Path A (optimistic) was skipped at T8 due to `currentAgent` being null
  - Path B (authoritative) is buffered at T9, not yet flushed

### Why this affects ALL providers

The race condition is in the **app's initialization logic**, not in any provider. Every new agent goes through `ensureAgentIsInitialized`, which triggers the buffering. The `currentAgent` null guard then blocks the optimistic update for any provider.

### Why it only affects the FIRST turn

On subsequent turns:
- The agent is already in the store (from previous initialization)
- `initializingAgents[agentId]` is `false`
- `agent_update(running)` is applied immediately → agent in store with `status: "running"`
- `turn_completed` arrives → `currentAgent` exists → optimistic update works → `status: "idle"` ✓

### When does the stuck state become PERMANENT?

The stuck state is **temporary** if the timeline fetch succeeds — the `flush_pending_updates` side effect (line 329) is always produced, and the buffered `agent_update(idle)` is flushed.

However, the stuck state can become **permanent** if:

1. **Timeline fetch fails**: `client.fetchAgentTimeline()` rejects → `setAgentInitializing(agentId, false)` + `rejectInitDeferred()` are called → `initializingAgents` is cleared → subsequent `agent_update` messages are applied directly. But if the `agent_update(idle)` already arrived and was buffered BEFORE the rejection, it's stuck in the buffer. The buffer is only flushed by `flush_pending_updates` (which requires a timeline response) or by `deletePendingAgentUpdate` (which is called when `isSyncingHistory` is false). After rejection, `isSyncingHistory` becomes false, so the NEXT `agent_update` would call `deletePendingAgentUpdate` (clearing the buffer without applying it) and then `applyAgentUpdatePayload` with the new update. But if there's no new `agent_update` after the rejection, the buffered (idle) update is lost.

2. **Timeline fetch is slow**: The stuck state persists for the duration of the fetch. If the fetch takes 5+ seconds, the user perceives it as "stuck."

3. **No `agent_update(idle)` is sent**: For OpenCode, the event stream path doesn't fire (bug #2), so the `agent_update(idle)` is only sent via the Run-return path. If the Run-return path also fails (e.g., `session.Run()` panics), no `agent_update(idle)` is sent, and the agent stays "running" forever.

---

## Part 3: Root Cause #2 — OpenCode `finishForegroundTurn` Drops `turn_completed`

### The bug

```go
// daemon/internal/agent/provider_opencode_events.go:241-286
func (s *openCodeSession) finishForegroundTurn(evt AgentStreamEvent, turnID string) {
    // ...
    evtType := ""
    switch e := evt.Event.(type) {
    case protocol.TurnCanceledStreamEvent:    // ← handled
        evtType = e.StreamEventType()
    case protocol.TurnFailedStreamEvent:      // ← handled
        evtType = e.StreamEventType()
    case map[string]interface{}:              // ← handled (legacy)
        evtType, _ = e["type"].(string)
    // ❌ MISSING: case protocol.TurnCompletedStreamEvent
    }

    // ...
    if evtType != "" {          // ← FALSE for TurnCompletedStreamEvent
        s.dispatcher.Emit(evt)  // ← NEVER CALLED
    }
}
```

### Impact

For OpenCode, the `turn_completed` stream event is **never emitted to the dispatcher**. This means:
- **Path A (optimistic) is completely broken** for OpenCode — the stream event never reaches the client
- **Path B (authoritative) still works** — `session.Run()` returns (line 196), the Run-return path fires (manager.go:561-566), and `agent_update(idle)` is sent

However, the timeline is missing the `turn_completed` terminal event, which may affect:
- Timeline finalization (the turn boundary is missing)
- End-of-turn UI elements (copy button, which checks `agent.status !== "running"`)
- Session memory recording (if it depends on the stream event)

### Why the OpenCode bug + app race = worse symptom

For OpenCode on the first turn:
- Path A is broken (stream event dropped) AND `currentAgent` is null (optimistic update skipped even if the event arrived)
- Path B is the ONLY path, but `agent_update(idle)` is BUFFERED during initialization
- The buffer is only flushed when the timeline response arrives
- If the timeline fetch is slow, the user sees a long stuck state

For Claude/Kimi/Pi on the first turn:
- Path A: `turn_completed` stream event arrives, but `currentAgent` is null → optimistic update skipped
- Path B: `agent_update(idle)` is BUFFERED → flushed when timeline response arrives
- Same stuck state, but typically shorter because Path A would have worked if `currentAgent` existed

---

## Part 4: The Fix

### Fix 1: OpenCode — Add missing `TurnCompletedStreamEvent` case

**File**: `daemon/internal/agent/provider_opencode_events.go`
**Line**: 250

```go
// Before:
switch e := evt.Event.(type) {
case protocol.TurnCanceledStreamEvent:
    evtType = e.StreamEventType()
case protocol.TurnFailedStreamEvent:
    evtType = e.StreamEventType()
case map[string]interface{}:
    evtType, _ = e["type"].(string)
}

// After:
switch e := evt.Event.(type) {
case protocol.TurnCompletedStreamEvent:    // ← ADD THIS
    evtType = e.StreamEventType()
case protocol.TurnCanceledStreamEvent:
    evtType = e.StreamEventType()
case protocol.TurnFailedStreamEvent:
    evtType = e.StreamEventType()
case map[string]interface{}:
    evtType, _ = e["type"].(string)
}
```

**Risk**: Very low. The `evtType` is used to determine whether to synthesize failed tool call events (line 260) and whether to emit the event (line 283). Adding the `turn_completed` case ensures the event is emitted. The tool call synthesis only fires for `turn_canceled` and `turn_failed`, so `turn_completed` is unaffected.

**Test**: Add a test that verifies `TurnCompletedStreamEvent` is emitted by `finishForegroundTurn`.

### Fix 2: App — Handle `currentAgent` null case in stream reducer

**File**: `app/src/contexts/session-stream-reducers.ts`
**Line**: 522

The optimistic status update should NOT require `currentAgent` to be non-null. When `turn_completed` arrives and the agent is not in the store (first-turn race), the reducer should still produce a status patch.

```typescript
// Before:
if (
  currentAgent &&
  (event.type === "turn_completed" ||
   event.type === "turn_canceled" ||
   event.type === "turn_failed")
) {
  const optimisticStatus = deriveOptimisticLifecycleStatus(currentAgent.status, event);
  if (optimisticStatus) {
    // ... set agentPatch
  }
}

// After:
if (
  event.type === "turn_completed" ||
  event.type === "turn_canceled" ||
  event.type === "turn_failed"
) {
  const currentStatus = currentAgent?.status;
  const optimisticStatus = currentAgent
    ? deriveOptimisticLifecycleStatus(currentAgent.status, event)
    : (event.type === "turn_completed" ? "idle"
       : event.type === "turn_failed" ? "error"
       : null);
  if (optimisticStatus) {
    const nextUpdatedAtMs = currentAgent
      ? Math.max(currentAgent.updatedAt.getTime(), timestamp.getTime())
      : timestamp.getTime();
    const nextLastActivityAtMs = currentAgent
      ? Math.max(currentAgent.lastActivityAt.getTime(), timestamp.getTime())
      : timestamp.getTime();
    agentPatch = {
      status: optimisticStatus,
      updatedAt: new Date(nextUpdatedAtMs),
      lastActivityAt: new Date(nextLastActivityAtMs),
    };
    agentChanged = true;
  }
}
```

**Risk**: Low-medium. The `commit` function calls `setAgents`, which needs to handle the case where the agent is not yet in the store. The `setAgents` callback should use `new Map(prev)` and set the agent entry, which works even if the agent doesn't exist yet. However, the agent patch only contains `status`, `updatedAt`, and `lastActivityAt` — it doesn't have `id`, `provider`, `cwd`, etc. The `commit` function needs to merge the patch with the existing agent or create a minimal agent entry.

**Alternative (safer) fix**: Instead of changing the reducer, ensure the agent is in the store BEFORE stream events are processed. This can be done by:
1. Adding the agent to the store from the `createAgent` RPC response (before `ensureAgentIsInitialized` is called)
2. OR not buffering the FIRST `agent_update` (the one that adds the agent to the store), only buffering subsequent updates

### Fix 3 (recommended): Don't buffer `agent_update` that adds the agent to the store

**File**: `app/src/contexts/session-context.tsx`
**Line**: 1078

```typescript
// Before:
if (isSyncingHistory) {
  bufferPendingAgentUpdate(serverId, agentId, update);
  return;
}

// After:
const agentExistsInStore = session?.agents.has(agentId) === true;
if (isSyncingHistory && agentExistsInStore) {
  bufferPendingAgentUpdate(serverId, agentId, update);
  return;
}
// If agent doesn't exist in store yet, apply the update immediately
// so that stream events (turn_completed) can use the optimistic path.
```

**Risk**: Low. The purpose of buffering is to prevent `agent_update` from overwriting the timeline response's authoritative state during history sync. But if the agent doesn't exist in the store yet, there's nothing to overwrite. Applying the first `agent_update` immediately ensures the agent is in the store for the stream reducer.

---

## Part 5: Verification Plan

### For Fix 1 (OpenCode):
1. Add test: `finishForegroundTurn` with `TurnCompletedStreamEvent` → verify `dispatcher.Emit` is called
2. Manual test: Create OpenCode agent → send message → verify `turn_completed` stream event arrives
3. Check timeline: verify `turn_completed` entry appears in the timeline

### For Fix 2/3 (App race condition):
1. Add test: `processAgentStreamEvent` with `currentAgent = null` and `turn_completed` event → verify `agentPatch` is non-null
2. Manual test: Create new agent with initial prompt → verify loading animation stops after content arrives
3. Test with slow network (throttle WebSocket) → verify animation still stops
4. Test with all providers (Claude, Kimi, OpenCode, Pi) → verify animation stops for all

### Regression tests:
1. Subsequent turns still work (agent already in store)
2. Reconnection during turn works (grace period buffering)
3. Agent creation without initial prompt works (no first-turn race)

---

## Part 6: Architecture Improvement — Eliminate the Dual-Path Race

The fundamental issue is that the daemon has **two independent paths** for signaling turn completion, and the app has **two independent paths** for processing it. This creates a complex state machine with race conditions.

### Recommendation: Make Path A (stream event) the primary signal

The `turn_completed` stream event is the **semantically correct** signal for turn completion. It carries usage data, arrives faster, and is part of the event timeline. The `agent_update(idle)` is a secondary, redundant signal.

To make Path A reliable:
1. **Fix the OpenCode bug** (Fix 1) — ensure all providers emit `turn_completed`
2. **Fix the dispatcher timeout** — ensure critical events are never dropped (consider unbounded buffering for critical events, or a dedicated critical-event channel)
3. **Fix the app reducer** (Fix 2/3) — ensure `turn_completed` is processed even when the agent is not in the store

To make Path B a reliable fallback:
1. **Keep the Run-return path** as a safety net
2. **Add `TouchUpdatedAt()`** to the Run-return path (manager.go:561) to ensure the `agent_update(idle)` always has a newer timestamp
3. **Ensure `flush_pending_updates` always fires** even on timeline fetch errors

---

## Appendix: Key File References

| File | Line(s) | Role |
|------|---------|------|
| `daemon/internal/agent/provider_opencode_events.go` | 250-285 | **BUG #2**: Missing `TurnCompletedStreamEvent` case |
| `daemon/internal/agent/manager.go` | 498-566 | Turn lifecycle: start → run → return → emitState |
| `daemon/internal/agent/manager.go` | 900-950 | workCh critical event path with fallback |
| `daemon/internal/agent/manager.go` | 1019-1068 | `applyTerminalStreamState` — event stream path |
| `daemon/internal/agent/manager.go` | 1098-1106 | `emitState` — sends `agent_update` |
| `daemon/internal/agent/agent.go` | 81-86 | `SetLifecycle` — updates `UpdatedAt = time.Now()` |
| `app/src/contexts/session-stream-reducers.ts` | 522-542 | **BUG #1**: `currentAgent` null guard |
| `app/src/contexts/session-stream-lifecycle.ts` | 4-30 | `deriveOptimisticLifecycleStatus` |
| `app/src/contexts/session-context.tsx` | 1069-1085 | `agent_update` handler with initialization buffering |
| `app/src/contexts/session-context.tsx` | 287-306 | `executeTimelineSideEffects` — `flush_pending_updates` |
| `app/src/contexts/session-context.tsx` | 329 | `flush_pending_updates` side effect (always produced) |
| `app/src/hooks/use-agent-initialization.ts` | 49-95 | `ensureAgentIsInitialized` — sets `initializingAgents` |
| `app/src/components/agent-stream-view.tsx` | 592 | `showWorkingIndicator = agent.status === "running"` |
| `app/src/contexts/session-context.tsx` | 514-533 | `applyAuthoritativeAgentSnapshot` — `updatedAt` check |
