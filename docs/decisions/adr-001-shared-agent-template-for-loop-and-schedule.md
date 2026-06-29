# ADR-001: Shared Agent Template for Loop and Schedule

|              |                                                               |
|--------------|---------------------------------------------------------------|
| **Status**   | Accepted                                                      |
| **Date**     | 2026-06-29                                                    |
| **Author**   | Solo Agent                                                    |
| **Scope**    | `protocol/`, `daemon/internal/loop/`, `daemon/internal/schedule/`, `daemon/internal/server/`, `app-bridge/src/shared/` |
| **Related**  | [Loop Schedule Spec](../product/loop-schedule-spec.md), [Loop Schedule Deep Dive](../product/loop-schedule-deep-dive.md), [App-Bridge Schedule Module](../analysis/app-bridge-schedule-module.md) |

---

## 1. Context

Solo has two independent subsystems that create and run AI agents on behalf of the user:

1. **Loop** (`daemon/internal/loop/`): repeatedly spawns worker and verifier agents until a goal is reached or a limit is hit.
2. **Schedule** (`daemon/internal/schedule/` + `daemon/internal/server/schedule_runner.go`): periodically triggers a single agent run.

Both subsystems ultimately call `agent.AgentManager.CreateAgent(ctx, *protocol.AgentSessionConfig, labels)`, but each carries its own configuration shape into that call:

- Loop stores agent parameters directly on `protocol.LoopRecord`: `Provider`, `Model`, `WorkerProvider`, `WorkerModel`, `VerifierProvider`, `VerifierModel`, `Cwd`.
- Schedule stores them in `protocol.ScheduleAgentConfig` inside `protocol.ScheduleTarget` for `"new-agent"` targets, and in `protocol.ProviderID` for `"provider"` targets.

`protocol.AgentSessionConfig` already contains the canonical agent configuration fields: `Provider`, `Cwd`, `Model`, `ModeID`, `ThinkingOptionID`, `ApprovalPolicy`, `SandboxMode`, `NetworkAccess`, `WebSearch`, `SystemPrompt`, `McpServers`, `Extra`, etc. Neither Loop nor Schedule uses the full surface today.

This ADR is a prerequisite for the broader Loop-as-a-Schedule-type unification documented in [Loop Schedule Spec](../product/loop-schedule-spec.md). Before we can merge the execution paths, we must first merge the *configuration* path.

---

## 2. Problem Statement (from first principles)

1. **Single responsibility / single source of truth.** The definition of "how to configure an agent" should live in one place. Today it is split across `LoopRecord`, `ScheduleAgentConfig`, and `AgentSessionConfig`.
2. **Don’t repeat yourself.** Adding a new agent capability—such as MCP servers, a system prompt, or sandbox mode—requires touching Loop, Schedule, and the conversion logic in at least two runners. In practice one path is often forgotten.
3. **Protocol-first.** The wire format should describe intent in the same vocabulary everywhere. A schedule "new-agent target" and a loop "worker agent" are both requests for a new agent with a given configuration; they should use the same message shape.
4. **Type erasure is a symptom, not a solution.** The current `ScheduleAgentConfig.Extra map[string]interface{}` and the Loop record’s flat fields are both partial workarounds for not committing to a shared typed configuration.

---

## 3. Decision

Adopt `protocol.AgentSessionConfig` as the **shared agent template** for both Loop and Schedule.

- A new exported type alias `AgentTemplate = AgentSessionConfig` is introduced in `protocol/` to make intent explicit on the wire: this is a *template* used to instantiate an agent, not an already-running session.
- `protocol.ScheduleAgentConfig` is **deprecated** and replaced by `AgentTemplate` inside `ScheduleTarget.Config`.
- `protocol.LoopRecord` gains explicit template fields for the worker and verifier, while the existing flat provider/model fields are retained for backward compatibility but marked deprecated.
- A single helper `agentTemplateToConfig(t AgentTemplate) AgentSessionConfig` (or direct use of the alias) is used by both `loop.Engine` and `schedule.daemonRunner` when calling `AgentManager.CreateAgent`.

### 3.1 Concrete shape

```go
// protocol/message_common.go

// AgentTemplate is a reusable configuration for instantiating an agent.
// It is intentionally identical to AgentSessionConfig so that any field
// available to a chat agent is also available to loop/schedule agents.
type AgentTemplate = AgentSessionConfig
```

```go
// protocol/message_schedule.go

type ScheduleTarget struct {
    Type       string         `json:"type"`                 // "agent" | "new-agent" | "provider"
    AgentID    string         `json:"agentId,omitempty"`    // existing agent id when Type == "agent"
    ProviderID string         `json:"providerId,omitempty"` // provider id when Type == "provider"
    Config     *AgentTemplate `json:"config,omitempty"`     // template when Type == "new-agent"
}

// ScheduleAgentConfig is kept as a deprecated alias for one release cycle.
// Deprecated: use AgentTemplate instead.
type ScheduleAgentConfig = AgentTemplate
```

```go
// protocol/message_loop.go

type LoopRecord struct {
    // ... existing fields ...

    // AgentTemplate is the base template for any agent this loop creates.
    // It replaces Provider/Model for new code.
    AgentTemplate *AgentTemplate `json:"agentTemplate,omitempty"`

    // WorkerAgentTemplate overrides AgentTemplate for the worker agent.
    WorkerAgentTemplate *AgentTemplate `json:"workerAgentTemplate,omitempty"`

    // VerifierAgentTemplate overrides AgentTemplate for the verifier agent.
    VerifierAgentTemplate *AgentTemplate `json:"verifierAgentTemplate,omitempty"`

    // Legacy fields retained for backward compatibility. They are ignored
    // when the corresponding *AgentTemplate field is non-nil.
    Provider         string  `json:"provider"`
    Model            *string `json:"model,omitempty"`
    WorkerProvider   *string `json:"workerProvider,omitempty"`
    WorkerModel      *string `json:"workerModel,omitempty"`
    VerifierProvider *string `json:"verifierProvider,omitempty"`
    VerifierModel    *string `json:"verifierModel,omitempty"`
}
```

### 3.2 Resolution rules

When Loop or Schedule needs an `AgentSessionConfig`:

1. If an explicit template is provided, use it as-is.
2. Else, fall back to legacy flat fields (`Provider`, `Model`, etc.) converted into an `AgentSessionConfig`.
3. Else, for Schedule `"provider"` targets, use `ProviderID` with an empty/default `AgentSessionConfig`.
4. Else, fail fast with a clear validation error.

This preserves existing persisted data and wire clients while making the new template shape the source of truth going forward.

---

## 4. Consequences

### Positive

- One place to add new agent capabilities.
- Loop and Schedule can both set system prompts, MCP servers, sandbox mode, approval policy, network access, etc.
- Existing tests and persisted JSON continue to work during the deprecation window.
- Unblocks the Loop-as-Schedule-type work described in the Loop Schedule Spec.

### Negative / Risks

- `AgentTemplate` and `AgentSessionConfig` are the same type; callers must not confuse a not-yet-created template with a live session config. The alias name makes intent explicit, but the compiler will not enforce it.
- Schedule target validation changes shape; older app versions that send `ScheduleAgentConfig` will still work because the alias preserves JSON field names, but new fields will be ignored by old code.
- Loop records grow new optional fields; we must be careful that old `loops.json` files deserialize cleanly.

---

## 5. TDD-First Acceptance Criteria

The following tests must exist and pass **before** the implementation is considered complete. They are listed in dependency order.

### 5.1 Protocol unit tests

1. `TestAgentTemplateAlias` — `protocol.AgentTemplate` serializes/deserializes as `AgentSessionConfig`.
2. `TestScheduleTargetWithAgentTemplate` — a `ScheduleTarget{Type:"new-agent", Config: &AgentTemplate{...}}` round-trips through JSON and all `AgentSessionConfig` fields are preserved.
3. `TestScheduleAgentConfigDeprecatedAlias` — a JSON object previously decoded as `ScheduleAgentConfig` still decodes into the new `AgentTemplate` field.
4. `TestLoopRecordLegacyCompatibility` — a `LoopRecord` JSON without `agentTemplate`/`workerAgentTemplate`/`verifierAgentTemplate` decodes successfully and legacy provider/model fields are populated.
5. `TestLoopRecordTemplateRoundTrip` — a `LoopRecord` with `AgentTemplate`/`WorkerAgentTemplate`/`VerifierAgentTemplate` round-trips through JSON preserving all template fields.

### 5.2 Daemon unit tests

6. `TestLoopEngineUsesAgentTemplate` — given a `LoopRecord` with `WorkerAgentTemplate`, the engine calls `AgentManager.CreateAgent` with an `AgentSessionConfig` that has `Provider`, `Cwd`, `Model`, `SystemPrompt`, and `McpServers` matching the template.
7. `TestLoopEngineFallsBackToLegacyProviderModel` — given a `LoopRecord` without templates but with `Provider`/`Model`, the engine still creates an agent with the correct provider/model.
8. `TestScheduleRunnerUsesAgentTemplate` — given a `StoredSchedule` with `Target.Type == "new-agent"` and a full `AgentTemplate`, `daemonRunner.Run` creates an agent using all template fields.
9. `TestScheduleRunnerRejectsEmptyTemplate` — a `"new-agent"` target with a nil/empty config returns a failed `RunResult` without panicking.

### 5.3 Integration tests

10. `TestScheduleRunWithSystemPrompt` — a schedule run whose template contains `SystemPrompt` produces an agent whose snapshot/config reflects that system prompt.
11. `TestLoopRunWithMcpServers` — a loop run whose `WorkerAgentTemplate` contains `McpServers` creates a worker agent with those servers configured.

### 5.4 App-Bridge type tests

12. `TestScheduleAgentConfigSchemaMatchesAgentSessionConfig` — the Zod/schema shape for schedule new-agent config in `app-bridge/src/server/schedule/rpc-schemas.ts` includes the same fields as `AgentSessionConfigSchema`.
13. `TestLoopRecordSchemaIncludesAgentTemplate` — the loop record schema in `app-bridge/src/server/loop/rpc-schemas.ts` includes optional `agentTemplate`, `workerAgentTemplate`, and `verifierAgentTemplate` fields typed as `AgentSessionConfigSchema`.

---

## 6. Implementation Plan

### Phase 1 — Protocol (1 day)

1. Add `type AgentTemplate = AgentSessionConfig` to `protocol/message_common.go`.
2. Change `ScheduleTarget.Config` from `*ScheduleAgentConfig` to `*AgentTemplate` in `protocol/message_schedule.go`.
3. Add `// Deprecated: use AgentTemplate.` alias `type ScheduleAgentConfig = AgentTemplate`.
4. Add `AgentTemplate`, `WorkerAgentTemplate`, `VerifierAgentTemplate` to `protocol.LoopRecord` in `protocol/message_loop.go`.
5. Mark legacy `Provider`/`Model`/`WorkerProvider`/`WorkerModel`/`VerifierProvider`/`VerifierModel` as deprecated in comments.
6. Write the protocol unit tests from §5.1.

### Phase 2 — Daemon helpers (1 day)

7. Add a small package `daemon/internal/agent/template` (or a helper in `daemon/internal/agent`) with:
   - `func FromTemplate(t protocol.AgentTemplate) protocol.AgentSessionConfig`
   - `func LoopRecordToWorkerConfig(r protocol.LoopRecord) (protocol.AgentSessionConfig, error)`
   - `func LoopRecordToVerifierConfig(r protocol.LoopRecord) (protocol.AgentSessionConfig, error)`
   - `func ScheduleTargetToConfig(t protocol.ScheduleTarget, fallbackCwd string) (protocol.AgentSessionConfig, error)`
8. Write unit tests for each helper covering template precedence and legacy fallback.

### Phase 3 — Adopt in runners (1 day)

9. Update `daemon/internal/loop/engine.go`:
   - `runWorker` uses `LoopRecordToWorkerConfig`.
   - `runVerifyPrompt` uses `LoopRecordToVerifierConfig`.
10. Update `daemon/internal/server/schedule_runner.go`:
    - `"new-agent"` path uses `ScheduleTargetToConfig`.
    - `"provider"` path continues to use `ProviderID` with an empty/default config.
11. Run all daemon tests with `-short -race`.

### Phase 4 — App-Bridge types (1 day)

12. Mirror the new optional fields in `app-bridge/src/shared/messages.ts` and the loop/schedule RPC schemas.
13. Add schema tests that assert `AgentTemplateSchema` equals `AgentSessionConfigSchema`.
14. Run `npm test` in `app-bridge` and `cd app && npx expo lint && npx tsc --noEmit`.

### Phase 5 — Documentation and deprecation notice (0.5 day)

15. Update this ADR status to **Accepted** once CI is green.
16. Add a note to `docs/product/loop-schedule-spec.md` §3.1/§3.2 that agent configuration now uses the shared `AgentTemplate`.
17. Open a follow-up ticket to remove `ScheduleAgentConfig` and Loop legacy provider fields after the deprecation window.

---

## 7. Migration Path

### Server-side persisted data

- `~/.solo/schedules.json`: existing `"config"` objects deserialize as `AgentTemplate` because the JSON field names and types are unchanged. No migration needed.
- `~/.solo/loops.json`: existing records have no template fields, so they fall back to legacy provider/model fields. No migration needed.

### Wire clients

- Old clients sending `ScheduleAgentConfig` continue to work.
- New clients may send `AgentTemplate` with extra fields; old servers that do not yet understand those fields will ignore them at the JSON level.

### Code cleanup timeline

| Release | Action |
|---------|--------|
| v0.7.x (this ADR) | Introduce `AgentTemplate`; keep deprecated aliases and legacy fields. |
| v0.8.x | Migrate UI and CLI to emit `AgentTemplate` only. |
| v0.9.x | Remove deprecated `ScheduleAgentConfig` alias and Loop legacy provider/model fields. |

---

## 8. Implementation Notes

The decision has been implemented as described above. Key files and deviations:

- `protocol/message_common.go` — added `type AgentTemplate = AgentSessionConfig`.
- `protocol/message_schedule.go` — `ScheduleTarget.Config` is now `*AgentTemplate`; `ScheduleAgentConfig` is a deprecated alias.
- `protocol/message_loop.go` — `LoopRecord` and `LoopRunRequest` gained `AgentTemplate`, `WorkerAgentTemplate`, and `VerifierAgentTemplate` while retaining legacy provider/model fields.
- `daemon/internal/agent/template.go` — new helper functions `AgentTemplate`, `LoopRecordToWorkerConfig`, `LoopRecordToVerifierConfig`, and `ScheduleTargetToConfig` centralize template resolution.
- `daemon/internal/loop/engine.go` — worker and verifier agents now use the helpers; `Engine` accepts an `agentManager` interface to enable unit testing.
- `daemon/internal/loop/store.go` — `Create` populates both template and legacy fields for backward compatibility during the deprecation window.
- `daemon/internal/server/schedule_runner.go` — `"new-agent"` and `"provider"` targets now share a single code path via `ScheduleTargetToConfig`.
- `app-bridge/src/shared/agent-session-config.ts` — extracted `AgentSessionConfigSchema` and `McpServerConfigSchema` to avoid a circular import with loop/schedule schemas.
- `app-bridge/src/client/daemon-client.ts` / `schedule-rpc.ts` — `RunLoopOptions` and `UpdateLoopOptions` expose template fields.
- `app/src/screens/loop-create-screen.tsx` — added an expandable "Agent Template" section with provider, model, and system prompt.
- `app/src/screens/loop-detail-screen.tsx` — displays the resolved agent and system prompt, and the edit modal supports updating the agent template.
- `app/src/screens/loops-screen.tsx` — loop cards show provider and model.
- `app-bridge/src/server/schedule/types.ts` — `ScheduleNewAgentConfigSchema` is now `AgentSessionConfigSchema`.
- `app-bridge/src/server/loop/rpc-schemas.ts` — `LoopRecordSchema` and `LoopRunRequestSchema` include optional nullable template fields.

### Test coverage

- Protocol: `protocol/agent_template_test.go` covers alias identity, schedule target round-trip, deprecated alias decoding, and loop record legacy/template compatibility.
- Daemon helpers: `daemon/internal/agent/template_test.go` covers all resolution paths and error cases.
- Loop engine: `daemon/internal/loop/engine_test.go` covers template use and legacy fallback with a fake agent manager.
- Schedule runner: `daemon/internal/server/schedule_runner_test.go` covers full template propagation and empty-template rejection.
- Loop store: extended `daemon/internal/loop/store_test.go` covers template updates and list-item provider/model propagation.
- App-Bridge: `app-bridge/src/server/loop/rpc-schemas.test.ts` and extended `app-bridge/src/server/schedule/rpc-schemas.test.ts` cover schema shape and parsing.

### Known pre-existing test flakiness

`app/src/terminal/runtime/terminal-emulator-runtime.browser.test.ts` occasionally times out in local runs. It is unrelated to this change.

---

## 9. References

- `protocol/message_common.go` — `AgentSessionConfig`
- `protocol/message_schedule.go` — `ScheduleTarget`, `ScheduleAgentConfig`
- `protocol/message_loop.go` — `LoopRecord`
- `daemon/internal/agent/template.go` — template resolution helpers
- `daemon/internal/loop/engine.go` — worker/verifier agent creation
- `daemon/internal/loop/store.go` — loop record creation
- `daemon/internal/server/schedule_runner.go` — schedule agent creation
- `daemon/internal/schedule/executor.go` — schedule executor tick
- `app-bridge/src/shared/agent-session-config.ts` — canonical agent config schema
- `app/src/screens/loop-create-screen.tsx` — loop creation UI with agent template
- `app/src/screens/loop-detail-screen.tsx` — loop detail/edit UI with agent template
- `app/src/screens/loops-screen.tsx` — loop list UI showing agent provider/model
- `docs/product/loop-schedule-spec.md` — Loop-as-Schedule unification spec
- `docs/product/loop-schedule-deep-dive.md` — Loop Controller and Step Executor deep dive
