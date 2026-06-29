# ADR-001: Shared Agent Template for Loop and Schedule

## Status

Accepted

## Date

2026-06-28

## Context

Solo has two features that need to spin up ephemeral agents with pre-selected provider settings:

1. **Schedule** — runs a prompt on a cadence, optionally creating a brand-new agent per run via `ScheduleTarget.Type == "new-agent"`. Its agent configuration is captured today in `ScheduleAgentConfig` (`protocol/message_schedule.go`).
2. **Loop** — runs a worker agent repeatedly and optionally a verifier agent, retrying until success or max iterations. Its agent configuration is captured today as flat optional fields (`Provider`, `Model`, `WorkerProvider`, `WorkerModel`, `VerifierProvider`, `VerifierModel`) in `protocol/message_loop.go`.

Both features conceptually ask the same question: *"When I need a new agent, what provider/model/mode/approval policy/etc. should it use?"* However, they currently express this with different shapes and at different levels of fidelity:

- Schedule supports provider, model, mode, thinking option, approval policy, sandbox, network, web search, system prompt, MCP servers, and extra provider options.
- Loop supports only provider and model for worker/verifier.

Meanwhile, the runtime agent creation API uses `AgentSessionConfig` (`protocol/message_common.go`), which includes additional runtime-only fields such as `Cwd`, `FeatureValues`, and `OutputSchema`.

This duplication and mismatch creates several problems:

- **Feature parity gaps**: Loop cannot configure mode, approval policy, system prompt, or MCP servers for its worker/verifier agents.
- **Code duplication**: Provider/model/mode selection logic is implemented or partially implemented in multiple places (schedule create/edit modals, loop create screen, agent creation flow).
- **Inconsistent UX**: Users learn different configuration patterns for essentially the same "configure a new agent" task.
- **Maintenance burden**: Adding a new agent capability requires updating `AgentSessionConfig`, `ScheduleAgentConfig`, and then Loop's flat fields separately.

Related documents:

- [`docs/product/loop-schedule-spec.md`](../product/loop-schedule-spec.md)
- [`docs/product/loop-schedule-design.md`](../product/loop-schedule-design.md)
- [`docs/analysis/app-bridge-schedule-module.md`](../analysis/app-bridge-schedule-module.md)
- [`docs/architecture/components.md`](../architecture/components.md) § Agent / Daemon

## Decision

Introduce a shared, user-facing abstraction called **`AgentTemplate`** that represents a preset for creating a new agent. Both Loop and Schedule will use `AgentTemplate` to configure their agent targets. The template is converted into a runtime `AgentSessionConfig` at execution time.

### 1. `AgentTemplate` is distinct from `AgentSessionConfig`

| Concern | `AgentTemplate` | `AgentSessionConfig` |
|---|---|---|
| Semantics | User-defined preset for "what kind of agent to create" | Runtime parameter for `AgentManager.CreateAgent` |
| Scope | Shared by Loop, Schedule, and future features | Used by chat/agent creation and runtime execution |
| `cwd` | Not included; injected from Loop/Schedule context | Included; the actual working directory for this session |
| Lifecycle | Persisted in Loop/Schedule records | Ephemeral; built per execution |
| Fields | Provider, model, mode, thinking option, approval policy, sandbox, network, web search, system prompt, MCP servers, extra | Template-derived fields + `cwd` + `featureValues` + `outputSchema` + runtime labels |

### 2. `AgentTemplate` fields

```go
// protocol/agent_template.go
type AgentTemplate struct {
    Provider         string
    Model            *string
    ModeID           *string
    ThinkingOptionID *string
    Title            *string
    ApprovalPolicy   string
    SandboxMode      string
    NetworkAccess    bool
    WebSearch        bool
    SystemPrompt     string
    Extra            map[string]interface{}
    McpServers       map[string]McpServerConfig
}
```

`Cwd` is intentionally excluded because it is a property of the execution context (Loop or Schedule), not of the agent preset.

### 3. Runtime conversion

A single helper converts a template into a runtime session config:

```go
func NewAgentSessionConfigFromTemplate(t *AgentTemplate, cwd string) *AgentSessionConfig
```

Loop and Schedule call this helper before `agentMgr.CreateAgent`, passing their own `cwd`.

### 4. Loop uses `AgentTemplate` for worker and verifier

`LoopRecord` and `LoopRunRequest` replace the flat provider/model fields with:

```go
type LoopRecord struct {
    // ... prompt, cwd, maxIterations, etc.
    Worker   AgentTemplate  `json:"worker"`
    Verifier *AgentTemplate `json:"verifier,omitempty"`
}
```

The verifier is optional. When omitted, the engine may fall back to the worker template for prompt-based verification.

### 5. Schedule replaces `ScheduleAgentConfig` with `AgentTemplate`

```go
type ScheduleTarget struct {
    Type       string          `json:"type"` // "agent" | "new-agent" | "provider"
    AgentID    string          `json:"agentId,omitempty"`
    ProviderID string          `json:"providerId,omitempty"`
    Config     *AgentTemplate  `json:"config,omitempty"`
}
```

`ScheduleAgentConfig` is removed; `AgentTemplate` is its semantic replacement.

### 6. Shared UI component

The App layer introduces a reusable **`AgentTemplateEditor`** component that is used by:

- Loop create/edit screens (worker section + verifier section)
- Schedule create/edit modals for `new-agent` targets

The editor encapsulates provider, model, mode, thinking option, approval policy, sandbox, network, web search, system prompt, and MCP server selection. It operates on a partial `AgentTemplate` value.

### 7. Backwards compatibility

- **Schedule**: `AgentTemplate` is field-compatible with the old `ScheduleAgentConfig`, so existing persisted JSON deserializes without a breaking change. The old `config.cwd` field, if present, is migrated to `Schedule.Cwd`.
- **Loop**: Existing persisted `LoopRecord` entries with flat provider/model fields are migrated at load time into `Worker` and `Verifier` templates.

## Alternatives Considered

### Alternative A: Reuse `AgentSessionConfig` directly

Both Loop and Schedule would store `AgentSessionConfig` instead of introducing `AgentTemplate`.

- **Pros**: One fewer type; no conversion function.
- **Cons**:
  - `AgentSessionConfig` contains runtime-only concerns (`cwd`, `outputSchema`, `featureValues`) that do not belong in a persisted user preset.
  - It blurs the line between "what agent to create" and "how to create it this time", making future refactors harder.
  - Persisting runtime fields can lead to stale or context-leaking data.

**Rejected**: The distinction between user preset and runtime config is worth preserving.

### Alternative B: Keep Loop and Schedule separate

Leave Loop's flat fields and Schedule's `ScheduleAgentConfig` unchanged.

- **Pros**: No refactoring risk; minimal immediate churn.
- **Cons**:
  - Loop cannot support mode, approval policy, system prompt, MCP servers, etc.
  - Every new agent capability must be added in three places.
  - Users face inconsistent configuration UX.

**Rejected**: The duplication and feature-parity gap are unacceptable long term.

### Alternative C: Make `AgentTemplate` include `cwd`

Allow the template to override the working directory.

- **Pros**: More flexible; a single template can be reused across different directories.
- **Cons**:
  - Complicates the execution model: which wins, template `cwd` or Loop/Schedule `cwd`?
  - Makes templates less portable and harder to reason about.

**Rejected**: `cwd` is execution context, not agent preset. If directory-specific presets are needed later, they can be added as an explicit override without changing the core model.

## Consequences

### Positive

- **Feature parity**: Loop's worker and verifier agents gain access to the full set of agent capabilities (mode, approval policy, system prompt, MCP servers, etc.).
- **Single source of truth**: One protocol type, one app-bridge schema, and one UI component for "configure a new agent".
- **Easier maintenance**: Adding a new agent capability requires updating `AgentTemplate`, the conversion helper, and the editor — not three or more separate structs.
- **Consistent UX**: Users configure agents the same way in Loop, Schedule, and future features.
- **Clear separation of concerns**: User preset vs. runtime config is explicit.

### Negative / Risks

- **Migration cost**: Loop persisted records need a one-time migration from flat fields to `AgentTemplate`.
- **Coordination cost**: This change touches protocol, daemon, app-bridge, and app; it must land in a coordinated PR or feature branch.
- **UI scope**: `AgentTemplateEditor` is a non-trivial component; it must handle provider loading, model/mode availability, and validation.

## Implementation Notes

Recommended file changes:

- `protocol/agent_template.go` — new `AgentTemplate` type and conversion helper.
- `protocol/message_schedule.go` — replace `ScheduleAgentConfig` with `AgentTemplate`.
- `protocol/message_loop.go` — replace flat provider/model fields with `Worker`/`Verifier AgentTemplate`.
- `daemon/internal/server/schedule_runner.go` — convert template to `AgentSessionConfig`.
- `daemon/internal/loop/engine.go` — worker/verifier both use `NewAgentSessionConfigFromTemplate`.
- `daemon/internal/loop/store.go` — migrate old Loop records on load.
- `app-bridge/src/server/agent/agent-template-schema.ts` — shared Zod schema.
- `app-bridge/src/server/schedule/types.ts` — reuse `AgentTemplateSchema`.
- `app-bridge/src/server/loop/rpc-schemas.ts` — reuse `AgentTemplateSchema` for worker/verifier.
- `app/src/components/agent-template-editor.tsx` — new reusable editor.
- `app/src/screens/loop-create-screen.tsx` and `loop-detail-screen.tsx` — use editor.
- `app/src/components/schedule-create-modal.tsx` and `schedule-edit-modal.tsx` — use editor for `new-agent` targets.

## References

- [`protocol/message_common.go`](../../protocol/message_common.go) — `AgentSessionConfig`
- [`protocol/message_schedule.go`](../../protocol/message_schedule.go) — `ScheduleTarget`, `ScheduleAgentConfig`
- [`protocol/message_loop.go`](../../protocol/message_loop.go) — `LoopRecord`, `LoopRunRequest`
- [`daemon/internal/loop/engine.go`](../../daemon/internal/loop/engine.go) — Loop worker/verifier agent creation
- [`daemon/internal/server/schedule_runner.go`](../../daemon/internal/server/schedule_runner.go) — Schedule execution
- [`app-bridge/src/server/schedule/types.ts`](../../app-bridge/src/server/schedule/types.ts) — Schedule schemas
- [`app-bridge/src/server/loop/rpc-schemas.ts`](../../app-bridge/src/server/loop/rpc-schemas.ts) — Loop schemas
