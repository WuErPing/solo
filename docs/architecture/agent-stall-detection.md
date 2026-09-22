# Agent Stall Detection

> **Back to**: [Architecture Overview](README.md) · [Documentation Index](../README.md)

## Problem Statement

Long-running AI agents can get stuck in several ways that the previous architecture could not detect or recover from:

1. **Repetition loops** — The LLM emits the same sentence over and over (e.g. "There are 5 agent.py files in this project:").
2. **Silent stalls** — The provider connection stays open but no new events arrive for minutes.
3. **Grace-period leakage** — A stuck agent keeps `LifecycleRunning`, causing the session grace timer to extend indefinitely (up to 10 × 90 s = 15 min).
4. **Late watchdog** — The only hard timeout was `maxAgentRunDuration = 35 min`, far too long for obvious failure modes.

**Incident**: 2026-05-29, OpenCode / DeepSeek V4 Pro, task "移除 Agent 对话模拟". The agent looped the same output, triggered a `question.asked` permission request, and continued streaming identical content while the grace period auto-extended 8 times.

---

## Design Goals

| Goal | Constraint |
|------|------------|
| Detect stuck agents early | < 3 min for obvious stalls |
| Provider-agnostic | Works for Claude, Kimi, OpenCode, Codex, Mock |
| Non-intrusive | No changes to provider implementations |
| Grace-safe | Stuck agents must not extend session grace |
| Configurable | Thresholds overridable per environment |
| Testable | Race-free, deterministic under test clocks |

---

## Architecture

### Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      AgentManager                            │
│  ┌─────────────┐  ┌─────────────────────────────────────┐  │
│  │ AgentLifecycle│  │         StallMonitor                │  │
│  │  (Running)   │  │  ┌─────────────┐ ┌───────────────┐ │  │
│  └──────┬───────┘  │  │ Inactivity  │ │  Repetition   │ │  │
│         │          │  │  Detector   │ │   Detector    │ │  │
│    stream events   │  │  (2 min)    │ │ (6/10 identical│ │  │
│         │          │  └──────┬──────┘ └───────┬───────┘ │  │
│         ▼          │         └────────────────┘         │  │
│  RecordEvent() ───►│                   │                │  │
│                    │            Interrupt()             │  │
│                    │                   │                │  │
│                    │         session.Interrupt()        │  │
│                    │                   │                │  │
│                    │         turn_failed event          │  │
│                    └─────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Session                                 │
│  expireGrace() ──► hasRunningAgentsWithProgress()?          │
│                    └─ NO → do NOT extend grace               │
└─────────────────────────────────────────────────────────────┘
```

### Components

#### 1. `StallMonitor` (`daemon/internal/agent/stall_monitor.go`)

A standalone component owned by `AgentManager`. It tracks per-agent progress state and fires `Interrupt()` when thresholds are breached.

**State per agent:**

```go
type stallState struct {
    lastEventTime time.Time   // timestamp of last stream event
    recentTexts   []string    // rolling window of normalized assistant_message texts
}
```

**Detection modes:**

| Mode | Trigger | Action |
|------|---------|--------|
| **Inactivity** | `time.Since(lastEventTime) > 2 min` | `Interrupt()` + log |
| **Repetition** | `≥ 6` identical texts in last `10` assistant messages | `Interrupt()` + log |

**Why these numbers:**
- **2 min inactivity**: Long enough to survive slow tool calls (e.g. large `git clone`), short enough to catch silent SSE hangs early.
- **6/10 repetition**: Tolerates 4 varied messages in a window; catches pure repetition loops common in reasoning models that lose context.

**Configuration:**

```go
// Production defaults
const defaultStallCheckInterval       = 30 * time.Second
const defaultInactivityThreshold      = 2 * time.Minute
const defaultRepetitionWindow         = 10
const defaultRepetitionThreshold      = 6

// Override via functional options
NewStallMonitor(logger, interruptFn,
    WithCheckInterval(50*time.Millisecond),      // test only
    WithInactivityThreshold(150*time.Millisecond),// test only
    WithRepetitionThreshold(5, 3),                // window, threshold
)
```

#### 2. AgentManager Integration

Lifecycle hooks ensure the monitor only tracks agents that are actually running:

| Lifecycle Point | Monitor Action |
|-----------------|----------------|
| `SendAgentMessage()` starts a turn | `RegisterAgent(agentID)` |
| Every stream event | `RecordEvent(agentID, event)` |
| `turn_completed` / `turn_failed` / `turn_canceled` | `UnregisterAgent(agentID)` |
| `DeleteAgent()` / `ArchiveAgent()` | `UnregisterAgent(agentID)` |

#### 3. Grace Period Fix

**Before:**
```go
// session.go
func (s *Session) expireGrace() {
    if s.hasRunningAgents() && s.graceExtensions < maxGraceExtensions {
        // extend grace...
    }
}
```

**After:**
```go
func (s *Session) hasRunningAgentsWithProgress() bool {
    return s.agentMgr.HasRunningAgentsWithRecentProgress()
}

func (s *Session) expireGrace() {
    if s.hasRunningAgentsWithProgress() && s.graceExtensions < maxGraceExtensions {
        // extend grace only if agent is actually producing output
    }
}
```

An agent that is `LifecycleRunning` but hasn't emitted an event in 2 minutes is treated as **not running** for grace-extension purposes.

---

## Data Flow

### Normal Turn (Healthy)

```
User sends message
  → AgentManager.SendAgentMessage()
    → stallMonitor.RegisterAgent("agent-1")
    → session.Run() (background goroutine)
      → SSE events arrive every few seconds
        → stallMonitor.RecordEvent("agent-1", event)
        → lastEventTime updated continuously
      → turn_completed
        → stallMonitor.UnregisterAgent("agent-1")
```

### Stuck Turn (Repetition Loop)

```
User sends message
  → AgentManager.SendAgentMessage()
    → stallMonitor.RegisterAgent("agent-1")
    → session.Run()
      → message.part.delta: "There are 5 agent.py files..."
      → message.part.delta: "There are 5 agent.py files..."
      → message.part.delta: "There are 5 agent.py files..."
      → ... (6 identical in window)
      → StallMonitor.checkAgent() fires
        → interruptFn("agent-1")
          → session.Interrupt()
            → OpenCode session.cancel()
            → turn_canceled / turn_failed emitted
        → stallMonitor.UnregisterAgent("agent-1")
```

### Silent Stall (No Events)

```
User sends message
  → AgentManager.SendAgentMessage()
    → stallMonitor.RegisterAgent("agent-1")
    → session.Run()
      → No SSE events for 2 minutes
      → StallMonitor.checkAgent() fires (inactivity)
        → interruptFn("agent-1")
          → session.Interrupt()
```

---

## Implementation Details

### Thread Safety

- `StallMonitor` uses an internal `sync.Mutex` for all state.
- `RecordEvent()` is called from the hot event path (`handleStreamEvent`) — it acquires the lock, updates state, and returns immediately.
- `checkAll()` runs on the monitor's own goroutine (ticker-driven), so it never blocks event delivery.

### Stop / Start Lifecycle

```go
type StallMonitor struct {
    stopCh   chan struct{}
    stopOnce sync.Once   // prevents double-close panic
    ticker   *time.Ticker
}
```

- `Start()` is idempotent (no-op if ticker already exists).
- `Stop()` uses `sync.Once` to close `stopCh` exactly once, avoiding the panic from `close()` on an already-closed channel.

### Why Not Put Detection in the Provider?

- **Provider-agnostic**: OpenCode, Claude, Kimi all emit different event shapes. A unified monitor at the `AgentStreamEvent` level works for all.
- **Non-intrusive**: No changes to provider implementations (`providers/opencode/client.go`, `providers/claude/client.go`, etc.).
- **Testable**: Can test with mock events without spinning up real provider processes.

---

## Testing

**File:** `daemon/internal/agent/stall_monitor_test.go`

| Test | What it verifies |
|------|-----------------|
| `TestStallMonitor_Inactivity` | No events → interrupt fires |
| `TestStallMonitor_ActivityResetsInactivity` | Varied events keep agent alive |
| `TestStallMonitor_Repetition` | 5 identical messages → interrupt |
| `TestStallMonitor_NoFalsePositiveForVariedOutput` | 10 different messages → no interrupt |
| `TestStallMonitor_UnregisterStopsTracking` | Unregister → no interrupt even with inactivity |
| `TestStallMonitor_HasRecentProgress` | API returns true/false correctly |
| `TestStallMonitor_RecordEventCreatesState` | Lazy state creation on first event |
| `TestStallMonitor_ReasoningEventsCountAsActivity` | `reasoning` timeline items count as progress |
| `TestStallMonitor_RepetitionIgnoresWhitespace` | Whitespace differences normalized |
| `TestStallMonitor_MapShapedEventsDoNotFeedRepetition` | Map-shaped `item` payloads do not feed the repetition detector |

All tests run with `-race` and pass under 4s.

---

## Operational Notes

### Logs to Watch

```
level=WARN msg="stall detected, interrupting agent" component=stall-monitor agentId=... reason=inactivity detail="no events for 2m5s"
level=WARN msg="stall detected, interrupting agent" component=stall-monitor agentId=... reason=repetition detail="6/10 identical messages"
```

### Tuning Thresholds

If legitimate long-running tool calls are being interrupted:

1. Increase `inactivityThreshold` (e.g. 5 min for CI/CD agents).
2. Increase `repetitionWindow` / `repetitionThreshold` for providers that naturally repeat system prompts.

These are compile-time constants today; future work could expose them in `~/.solo/config.json`.

---

## Future Work

1. **Config file integration** — Expose thresholds in `daemon/internal/config/config.go` so users can tune per-project.
2. **Adaptive thresholds** — Shorter thresholds for lightweight providers (Mock) and longer for heavy providers (Claude with large context).
3. **Token-velocity detection** — Detect stalls even when events arrive but contain zero new tokens (e.g. heartbeat deltas).
4. **Frontend stall indicator** — Show a "Agent appears stuck" banner in the UI before auto-interrupt, giving the user a chance to cancel manually.

---

## Changelog

| Date | Change |
|------|--------|
| 2026-05-29 | Introduced `StallMonitor` with inactivity + repetition detection; tightened grace-period condition. |
