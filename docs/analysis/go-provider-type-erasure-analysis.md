# Go Provider Type Erasure Analysis

**Date:** 2026-06-07
**Status:** Analysis Complete
**Priority:** P1 (Structural Risk)
**Source:** Architecture review `2026-06-07_main_68c66e5_qodercli_qodercli`

---

## Executive Summary

Go-side type erasure (`interface{}` and `map[string]interface{}`) is growing at ~25-30% per review cycle—far outpacing code growth (~5%). This has been flagged P1 for 5 consecutive reviews without execution. The structural risk is compounding: every new provider or event type deepens the dependency on dynamic typing, degrading AI inferability and pushing the codebase toward a "refactor cost exceeds rewrite cost" tipping point.

This document diagnoses the root cause, compares five remediation strategies, and recommends a phased **Boundary Isolation → Tagged Union** approach that balances risk, cost, and structural improvement.

---

## Current State

### Quantified Impact

| Metric | Count | Growth | Location |
|--------|-------|--------|----------|
| `interface{}` | 987 | +220 (+28.7%) | Protocol, dispatcher, provider events |
| `map[string]interface{}` | 754 | +152 (+25.2%) | Provider parsing, timeline detail, metadata |

### Module-Level Distribution

| Module | `map[string]interface{}` | Core Pattern |
|--------|--------------------------|--------------|
| **protocol** | 17 | `AgentStreamPayload.Event` (`interface{}`), `StatusMessage.Payload` |
| **provider_opencode** | ~120 | SSE event translation, tool-detail derivation |
| **provider_kimi** | ~35 | JSON-RPC Wire dynamic parsing |
| **provider_claude** | ~45 | Print-stream JSON dynamic parsing |
| **provider_pi** | ~20 | Legacy (removed from registry, code still in tests) |
| **timeline** | ~15 | `TimelineItem.Detail`, `Error`, `Metadata` |
| **dispatcher/base** | ~10 | `chan interface{}`, `Emit(interface{})` |
| **server/session** | ~80 | Event routing, message transformation |

### Quality Attribute Impact

- **AI-Inferability**: 1.8 → 1.6 (↓0.2), the only declining dimension in the latest review
- **Context-First sub-score**: 2.3 → 2.0 (↓0.3)
- **Modifiability/Maintainability**: Flat at 3.5; new module quality is offset by type-erasure degradation

---

## Root Cause Analysis

### The Type-Sink Pattern

The root cause is a single protocol definition that acts as a **type sink**:

```go
// protocol/message_agent_outbound.go
type AgentStreamPayload struct {
    AgentID   string      `json:"agentId"`
    Event     interface{} `json:"event"`  // ← type sink
    Timestamp string      `json:"timestamp"`
}
```

Because `Event` is `interface{}`, every provider emits events through the same untyped channel. The path of least resistance is to construct `map[string]interface{}` literals rather than define typed structs.

### Causal Chain

```
External API format uncertainty (OpenCode SSE / Kimi Wire / Claude Print)
                    ↓
Provider layer uses map[string]interface{} for "defensive parsing"
                    ↓
protocol.AgentStreamPayload.Event is interface{} (accepts everything)
                    ↓
Dispatcher pipes chan interface{}
                    ↓
Timeline / Session / Server layers consume via type assertions
                    ↓
New provider or event type → new map[string]interface{} construction
```

### Why Growth Is Compounding

1. **Network effect**: Each new provider adds its own `map[string]interface{}` construction points, plus cascading type assertions in timeline store, session handler, and test mocks.
2. **Copy-paste precedent**: New providers copy existing ones (especially OpenCode, with 120+ instances). The dominant pattern becomes the default pattern.
3. **AI-Inferability death spiral**: AI agents cannot infer event contracts from `map[string]interface{}` usage → generate more dynamic parsing code → further degrade inferability.

---

## Solution Comparison

### Option A: Gradual Strong-Typing (Per-Provider)

> "When modifying a provider, convert its event parsing from `map[string]interface{}` to strong-type structs."

| Dimension | Assessment |
|-----------|------------|
| **Effort** | ~2h extra per provider change; first protocol change ~10h |
| **Risk** | Low; isolated to single provider |
| **Root-cause fix?** | Partial—does not change `AgentStreamPayload.Event` |
| **AI-Inferability gain** | Medium (single provider improves, protocol layer still untyped) |
| **Execution feasibility** | **Poor**—flagged P1 for 5 cycles, never executed because it is always deprioritized behind feature delivery |
| **Total cost** | ~40-60h over 6-12 months (spread across iterations) |

**Verdict**: Palliative, not cure. History proves it will not be executed without dedicated time.

---

### Option B: Tagged Union (Sum Type) Event System

> Define a unified `AgentEvent` discriminated union. All events become strongly typed structs; serialization remains wire-compatible.

```go
type AgentEvent interface {
    EventType() string
}

type TimelineEvent struct {
    Type     string       `json:"type"`
    Item     TimelineItem `json:"item"`
    Provider string       `json:"provider"`
}

type TurnCompletedEvent struct {
    Type     string      `json:"type"`
    Provider string      `json:"provider"`
    Usage    *AgentUsage `json:"usage,omitempty"`
}
```

Custom `UnmarshalJSON` on `AgentStreamPayload` dispatches by `"type"` field.

| Dimension | Assessment |
|-----------|------------|
| **Effort** | ~60-80h (1.5-2 weeks): design union (~4h) + protocol serialization (~8h) + pilot provider (~12h) + remaining providers (~8h each) + consumer adaptation (~16h) |
| **Risk** | Medium—core abstraction changes; must maintain 100% backward serialization compatibility |
| **Root-cause fix?** | **Complete**—eliminates the necessity for `map[string]interface{}` in the event pipeline |
| **AI-Inferability gain** | **High**—event contracts are explicit; type switches replace map assertions |
| **Execution feasibility** | Medium—requires dedicated sprint, cannot be done "alongside" feature work |
| **Total cost** | ~70h, 2-3 weeks concentrated |

**Verdict**: The structural fix. Directly breaks the compounding cycle.

---

### Option C: Code Generation / JSON Schema Driven

> Maintain JSON Schema per provider; auto-generate Go structs and parsers.

| Dimension | Assessment |
|-----------|------------|
| **Effort** | ~40h initial + ~2h/month maintenance |
| **Risk** | Medium-high—build-time dependency; schema-to-real-API drift |
| **Root-cause fix?** | Partial—auto-generates structs but handwritten translator remains a type-erasure hotspot |
| **AI-Inferability gain** | High for generated structs; low for hand-written glue |
| **Execution feasibility** | Low—adds build-pipeline complexity |
| **Total cost** | ~40h + ongoing maintenance |

**Verdict**: Over-engineered for Solo's current scale (4 providers, stable APIs). Revisit if provider count grows to 10+ or external APIs change frequently.

---

### Option D: Boundary Isolation (Adapter Pattern)

> **Accept** that provider internals need defensive parsing against external APIs. **Isolate** that dynamic typing at the **Provider → Dispatcher** boundary by forcing conversion to a strong-type `AgentEvent` before emission.

```go
// Provider internal: still uses map for parsing external SSE/JSON-RPC
func (s *openCodeSession) translateEvent(...) AgentEvent {
    // ... internal parsing via raw map[string]json.RawMessage
    return TimelineEvent{Item: ..., Provider: ...}  // ← boundary conversion
}
```

- Protocol `AgentStreamPayload.Event` keeps `interface{}` but is constrained to `AgentEvent` implementations.
- Dispatcher and consumers work with typed events via type switches.

| Dimension | Assessment |
|-----------|------------|
| **Effort** | ~45h (1-2 weeks): define `AgentEvent` interface + core structs (~8h) + change provider return signatures (~6h/provider) + adapt consumers (~12h) |
| **Risk** | **Low**—provider internal logic untouched; only the "exit gate" changes |
| **Root-cause fix?** | Strong—stops type erasure from spreading beyond provider internals |
| **AI-Inferability gain** | High for event pipeline; provider internals remain dynamic |
| **Execution feasibility** | **High**—lower cost and risk make it achievable in a single sprint |
| **Total cost** | ~45h, 1-2 weeks |

**Verdict**: The pragmatic compromise. Acknowledges reality (external APIs are uncontrollable) while preventing architectural contamination.

---

### Option E: Status Quo + Linter Gate

> Do not touch existing code. Add a linter rule forbidding new `map[string]interface{}`.

| Dimension | Assessment |
|-----------|------------|
| **Effort** | ~2h |
| **Risk** | Negligible |
| **Root-cause fix?** | None—754 existing instances untouched |
| **AI-Inferability gain** | None |
| **Execution feasibility** | High—but meaningless |
| **Total cost** | ~2h |

**Verdict**: Inadequate. The problem is compounding *stock*, not just flow.

---

## Comparison Matrix

| Option | Effort | Risk | Root-Cause Fix | AI-Inferability | Feasibility | Recommendation |
|--------|--------|------|----------------|-----------------|-------------|----------------|
| A Gradual | ~60h | Low | ⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⚠️ Fallback |
| B Tagged Union | ~70h | Medium | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ✅ Structural fix |
| C Code Gen | ~40h+ | Medium-High | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ | ❌ Over-engineered |
| D Boundary Isolation | ~45h | **Low** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ✅ **Recommended** |
| E Linter Only | ~2h | Negligible | ⭐ | ⭐ | ⭐⭐⭐⭐⭐ | ❌ Inadequate |

---

## Recommended Approach: D → B Staircase

Given that this issue has been **flagged P1 for 5 consecutive reviews without execution**, the strategy must optimize for **execution feasibility** while preserving a path to the structural fix.

### Phase 1: Boundary Isolation (D) — Weeks 1-2, "Stop the Bleeding"

1. **Define `AgentEvent` interface** and 5-8 core event structs:
   - `TimelineEvent`
   - `TurnCompletedEvent`
   - `TurnFailedEvent`
   - `TurnCanceledEvent`
   - `PermissionRequestedEvent`
   - `UsageUpdatedEvent`
   - `ThreadStartedEvent`
2. **Add custom `MarshalJSON`/`UnmarshalJSON`** on `AgentStreamPayload` to support tagged-union serialization without breaking wire format.
3. **Pilot with OpenCode provider** (largest contributor, ~120 instances, code memory fresh).
4. **Adapt Timeline + Server consumers** to type-switch on `AgentEvent` instead of asserting `map[string]interface{}`.
5. **Validation**: After this phase, `map[string]interface{}` must be zero outside provider internal files.

### Phase 2: Tagged Union (B) — Follows naturally after Phase 1

Once the event pipeline and all consumers are typed:
- Converting provider internals from `map[string]interface{}` to typed structs becomes a **pure internal refactor** with no cross-module impact.
- Each remaining provider migration is a ~6-8h isolated task that can be scheduled independently.

### Why This Order Works

- **Phase 1 delivers visible ROI fast**: AI-Inferability score rebounds, new providers have a typed contract to follow.
- **Phase 2 is de-risked**: By the time you touch provider internals, the boundary is already proving the contract works.
- **Fits the review cycle**: Phase 1 can land in a single sprint; reviewers see measurable improvement in the next maturity assessment.

---

## Implementation Plan

| Step | Task | Effort | Owner | Verification |
|------|------|--------|-------|--------------|
| 1 | Design `AgentEvent` interface + event structs in `protocol/` | 4h | — | Design doc review |
| 2 | Implement `MarshalJSON`/`UnmarshalJSON` for backward-compatible serialization | 8h | — | Unit tests: round-trip all event shapes |
| 3 | Migrate OpenCode provider to return `AgentEvent` | 12h | — | `grep -r 'map\[string\]interface{}' provider_opencode*.go` shows only internal parsing |
| 4 | Adapt `timeline.go` consumers (`AppendFromHistory`, `timelineItemFromMap`) | 6h | — | Existing timeline tests pass |
| 5 | Adapt `server/session_agent_stream.go` and related handlers | 10h | — | Integration tests pass (`agent_integration_test.go`) |
| 6 | Migrate Kimi provider | 8h | — | Kimi wire tests pass |
| 7 | Migrate Claude provider | 8h | — | Claude stream tests pass |
| 8 | Add linter rule / code-review checklist forbidding `map[string]interface{}` in new event emission code | 2h | — | CI enforces |
| 9 | Update provider documentation with typed-event examples | 2h | — | Docs reviewed |

**Total**: ~60h over 2 sprints.

---

## Critical Success Factors

1. **Dedicated time window**: Do not attempt this "alongside" feature work. Schedule it as a focused technical-debt sprint.
2. **Backward compatibility**: The wire protocol between daemon and app must not change. `AgentEvent` structs must serialize to the exact same JSON shape as today's `map[string]interface{}` events.
3. **Test coverage**: Run the full `daemon/internal/server` integration test suite after each provider migration. The `agent_integration_test.go` and `session_agent_stream.go` tests are the safety net.
4. **Code review gate**: After Phase 1, any new provider PR must demonstrate event construction using `AgentEvent` types, not `map[string]interface{}`.

---

## Related Files

| File | Role |
|------|------|
| `protocol/message_agent_outbound.go` | `AgentStreamPayload.Event` (the type sink) |
| `protocol/message_common.go` | `AgentSessionConfig`, `AgentPermissionResponse` with `map[string]interface{}` fields |
| `daemon/internal/agent/base/dispatcher.go` | `EventDispatcher.Emit(interface{})`, `chan interface{}` |
| `daemon/internal/agent/provider_opencode_events.go` | Largest `map[string]interface{}` contributor (~120 instances) |
| `daemon/internal/agent/provider_opencode_util.go` | Tool-detail derivation functions returning `map[string]interface{}` |
| `daemon/internal/agent/provider_kimi.go` | JSON-RPC Wire provider with dynamic parsing |
| `daemon/internal/agent/provider_claude.go` | Print-stream provider with dynamic parsing |
| `daemon/internal/agent/timeline.go` | `TimelineItem.Detail` (`interface{}`), `timelineItemFromMap` |
| `daemon/internal/agent/agent.go` | `PendingPermissions map[string]interface{}` |
| `daemon/internal/server/session_agent_stream.go` | Event routing with type assertions |

---

## Appendix: Review Excerpt

> *"A structural risk is solidifying: Go-side type erasure is growing at ~25-30% per cycle, flagged P1 for 5 consecutive reviews, never executed. Every new provider or event type deepens the `map[string]interface{}` dependency. Code AI-inferability continues to decline. Eventually: refactor cost exceeds rewrite cost."*
>
> — Architecture Review `2026-06-07_main_68c66e5`, Section 7.1

---

## Document History

| Date | Version | Change |
|------|---------|--------|
| 2026-06-07 | v1.0 | Initial analysis and solution comparison |
