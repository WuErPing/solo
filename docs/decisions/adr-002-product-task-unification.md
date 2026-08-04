# ADR-002: Product + Task Unification

|              |                                                               |
|--------------|---------------------------------------------------------------|
| **Status**   | Proposed                                                      |
| **Date**     | 2026-07-31                                                    |
| **Author**   | Solo Team                                                     |
| **Scope**    | `protocol/`, `daemon/internal/`, `app-bridge/src/`, `app/src/` |
| **Related**  | [PRD: Product + Task Unification](../product/prd/product-task-unification.md), [ADR-001](adr-001-shared-agent-template-for-loop-and-schedule.md), [Loop Schedule Spec](../product/prd/loop-schedule-spec.md) |

---

## 1. Context

Solo has grown six user-facing concepts — Project, Workspace, Agent, Loop, Schedule, Tmux Pane — each with its own data model, RPC surface, store, and app screen. ADR-001 unified the *configuration* path (AgentTemplate), but the *orchestration* path remains split: Loop and Schedule are independent engines with independent stores, independent RPCs, and independent UIs.

Meanwhile, the product direction (informed by Multica's product-layer analysis) calls for:
- A unified "work object" that users interact with
- Agent demotion from first-class citizen to internal executor
- Knowledge compounding via Skills
- Passive awareness via notifications

The PRD proposes collapsing all six concepts into two: **Product** (what I'm building) and **Task** (what needs to be done). This ADR records the architectural decision to proceed with that unification.

---

## 2. Problem Statement

From first principles:

1. **Single abstraction for "work".** A user's intent is always "get this thing done." Whether it runs once, iterates with verification, or fires on a cron schedule are *execution parameters*, not different kinds of entities. Forcing users to choose "Loop vs Schedule" before they've even stated what they want done inverts the natural decision order.

2. **Agents are means, not ends.** Users don't want to "manage agents" — they want tasks completed. Exposing agent lifecycle (initializing/idle/running/error/closed) as a primary UI concern leaks implementation detail. The agent is an executor, like a CI runner.

3. **Tmux is plumbing.** Tmux provides process isolation and terminal multiplexing. Users care about *seeing the agent work*, not about managing panes as objects. The pane is a view into a task's execution, not an independent entity.

4. **RPC surface bloat.** Loop has 10 RPCs, Schedule has 10 RPCs, Agent has 15+ RPCs. Many are near-duplicates (list/inspect/logs/stop). A unified Task RPC set can serve all use cases with ~10 endpoints.

5. **Store fragmentation.** `loops.json`, `schedules.json`, and in-memory agent state are three separate persistence concerns for what is conceptually one thing: "work items and their execution history."

---

## 3. Decision

Adopt **Product + Task** as the two primary user-facing abstractions. All other concepts become internal implementation details or configuration parameters.

### 3.1 Concept reduction

| Current concept | Becomes | Rationale |
|-----------------|---------|-----------|
| Project | **Product** | Renamed; absorbs repo binding, gains config (skills, verify, env) |
| Workspace | Product's runtime tmux session | Not a separate user concept |
| Loop | Task with `execution.mode = "iterate"` | Iteration is an execution parameter |
| Schedule | Task with `trigger.type = "cron"` | Timing is a trigger parameter |
| Agent | Task's internal executor | Users interact with tasks, not agents |
| Tmux Pane | Task run's live view | A window into execution, not a managed object |

### 3.2 New primary types

```go
// protocol/message_product.go
type Product struct {
    ID        string        `json:"id"`
    Name      string        `json:"name"`
    Avatar    string        `json:"avatar,omitempty"`
    Repos     []string      `json:"repos"`
    Config    ProductConfig `json:"config"`
    CreatedAt time.Time     `json:"createdAt"`
    UpdatedAt time.Time     `json:"updatedAt"`
}

type ProductConfig struct {
    DefaultProfileID string            `json:"defaultProfileId,omitempty"`
    Skills           []string          `json:"skills,omitempty"`
    VerifyChecks     []string          `json:"verifyChecks,omitempty"`
    Env              map[string]string `json:"env,omitempty"`
}
```

```go
// protocol/message_task.go
type Task struct {
    ID        string     `json:"id"`
    ProductID string     `json:"productId"`
    Title     string     `json:"title"`
    Prompt    string     `json:"prompt"`
    Status    TaskStatus `json:"status"`
    Execution Execution  `json:"execution"`
    Trigger   Trigger    `json:"trigger"`
    ProfileID string     `json:"profileId,omitempty"`
    Skills    []string   `json:"skills,omitempty"`
    Runs      []Run      `json:"runs"`
    CreatedAt time.Time  `json:"createdAt"`
    UpdatedAt time.Time  `json:"updatedAt"`
}

type Execution struct {
    Mode          string   `json:"mode"` // "once" | "iterate" | "interactive"
    MaxIterations int      `json:"maxIterations,omitempty"`
    MaxTimeMs     int64    `json:"maxTimeMs,omitempty"`
    SleepMs       int      `json:"sleepMs,omitempty"`
    VerifyPrompt  string   `json:"verifyPrompt,omitempty"`
    VerifyChecks  []string `json:"verifyChecks,omitempty"`
}

type Trigger struct {
    Type  string `json:"type"` // "manual" | "cron" | "webhook" | "on_complete"
    Cron  string `json:"cron,omitempty"`
    Event string `json:"event,omitempty"`
}

type Run struct {
    ID         string      `json:"id"`
    Status     string      `json:"status"`
    AgentID    string      `json:"agentId"`
    Iterations []Iteration `json:"iterations,omitempty"`
    StartedAt  time.Time   `json:"startedAt"`
    FinishedAt *time.Time  `json:"finishedAt,omitempty"`
    Error      string      `json:"error,omitempty"`
}
```

```go
// protocol/message_profile.go
type AgentProfile struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Avatar      string        `json:"avatar,omitempty"`
    Description string        `json:"description,omitempty"`
    Template    AgentTemplate `json:"template"`
    Skills      []string      `json:"skills,omitempty"`
    Concurrency int           `json:"concurrency,omitempty"`
    CreatedAt   time.Time     `json:"createdAt"`
    UpdatedAt   time.Time     `json:"updatedAt"`
}
```

### 3.3 Execution modes

| Mode | Behaviour | Replaces |
|------|-----------|----------|
| `once` | Send prompt to agent, wait for completion, record run | Schedule's new-agent target |
| `iterate` | Prompt + verify loop until pass or budget exhausted | Loop engine |
| `interactive` | No prompt; user controls agent in real-time via key injection | Current Workspace/Agent experience |

### 3.4 Trigger types

| Type | Behaviour | Replaces |
|------|-----------|----------|
| `manual` | User clicks Run, or auto-run on creation | Loop manual run |
| `cron` | Daemon scheduler fires at cron/interval | Schedule executor |
| `webhook` | `POST /api/trigger/{taskID}` on daemon HTTP | New capability |
| `on_complete` | Fires when another task's run reaches `done` | New capability (task chain) |

### 3.5 Resolution chain

When the task engine needs an `AgentSessionConfig`:

```
1. Resolve profile: Task.ProfileID → Product.Config.DefaultProfileID → error
2. Load AgentProfile → base AgentTemplate
3. Apply task-level overrides (if Task has inline Template fields)
4. Merge skills: Product.Config.Skills ∪ Profile.Skills ∪ Task.Skills
5. Inject skills into provider-native paths
6. Apply Product.Config.Env to execution environment
7. Call agent.Manager.CreateAgent(ctx, resolvedConfig, labels)
```

### 3.6 Interactive session escape hatch

Interactive sessions are `Task(mode=interactive)` in the data model, but expose a separate low-latency RPC surface:

```
session/open   → creates task(mode=interactive) + agent + tmux pane
session/send   → key injection (bypasses task engine, direct to agent)
session/stream → ANSI output stream (direct from pane)
session/close  → stops agent, marks task done
```

This preserves the current sub-100ms key-to-screen latency that would be unacceptable through the task/run request-response path.

### 3.7 RPC surface

```
product/*    (5)  list, get, create, update, delete
task/*       (8)  create, list, get, run, stop, update, delete, logs
profile/*    (5)  list, get, create, update, delete
skill/*      (4)  list, read, write, delete
notification/* (4) list, read, read-all, unread-count
session/*    (4)  open, send, stream, close
─────────────────
Total:       30
```

### 3.8 Daemon module structure

```
daemon/internal/
├── task/              ← NEW: unified engine
│   ├── engine.go      # Run(), dispatch by execution.mode
│   ├── iterate.go     # iterate loop (absorbs loop/engine.go)
│   ├── trigger.go     # cron/webhook/on_complete scheduler (absorbs schedule/executor.go)
│   ├── store.go       # tasks.json persistence
│   └── resolve.go     # profile + skills + env resolution
├── product/           ← NEW
│   └── store.go       # products.json persistence
├── profile/           ← NEW
│   └── store.go       # profiles.json persistence
├── skill/             ← NEW
│   ├── reader.go      # filesystem skill loading
│   └── injector.go    # provider-aware injection
├── notification/      ← NEW
│   └── store.go       # notifications.json + push trigger
├── agent/             ← UNCHANGED (execution layer)
└── server/            ← MODIFIED (new RPC handlers, old handlers proxy)
```

---

## 4. Alternatives Considered

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| **A: Gradual evolution (PRD product-layer-evolution P0-P5)** | Low risk per step; no big-bang migration | Perpetuates Loop/Schedule split indefinitely; 6 concepts remain; two engines never truly merge | Rejected — kicks the can |
| **B: Product + Task unification (this ADR)** | Clean mental model; single engine; RPC convergence; natural extensibility (triggers, chains) | Larger migration surface; app rewrite; risk of regression during transition | **Chosen** |
| **C: Keep Loop/Schedule, add Task as a facade** | Minimal backend change; old code untouched | Facade over two engines = lowest-common-denominator API; doesn't reduce conceptual load; technical debt compounds | Rejected — shallow module |
| **D: Adopt Multica's model wholesale (Issue + Squad + Autopilot)** | Battle-tested abstractions; rich feature set | Multi-user concepts (squads, RBAC) don't fit Solo's single-user daemon; massive scope; loses terminal-native identity | Rejected — wrong fit |

---

## 5. Consequences

### Positive

- **Cognitive load**: 6 concepts → 2. New users can be productive in minutes.
- **Single engine**: One task engine with mode-dispatch replaces two independent engines. Bug fixes and features apply once.
- **RPC convergence**: ~35 RPCs → 30, with far less semantic overlap.
- **Extensibility**: New execution modes (e.g., `parallel` for multi-agent) or trigger types (e.g., `git_push`) are additive — no new top-level concept.
- **Data unification**: One `tasks.json` replaces `loops.json` + `schedules.json`. Run history is uniform.
- **App simplification**: 15+ screens → 8. Navigation is Product → Task → Live View.
- **Natural pipeline**: `on_complete` trigger enables task chains without new abstractions.
- **Agent demotion**: Users stop thinking about "which agent" and start thinking about "what task." Profile handles the "which agent" question once at config time.

### Negative / Risks

- **Migration complexity**: Two data files + in-memory state + app screens all change. Three-phase migration spans 3 releases (~12 weeks).
- **Interactive mode awkwardness**: Fitting real-time terminal control into "task" semantics requires the session/* escape hatch — a slight conceptual leak.
- **Regression risk**: Loop and Schedule engines are battle-tested. Rewriting them as task engine modes may introduce subtle behaviour differences (timing, error handling, retry logic).
- **App rewrite scope**: Nearly all screens change. This is the largest frontend effort in Solo's history.
- **Backward incompatibility**: Third-party integrations (if any) using loop/* or schedule/* RPCs will break after v0.15.x.

---

## 6. Tech Debt / Repayment Window

| Debt Item | Debt ID | Repayment Target |
|-----------|---------|------------------|
| Old RPC handlers (loop/*, schedule/*) kept as proxies during migration | DEBT-001 | v0.15.x (remove) |
| Legacy data files (loops.json, schedules.json) kept as migration source | DEBT-002 | v0.15.x (remove after confirmed migration) |
| Interactive session as separate RPC surface (conceptual leak) | DEBT-003 | Evaluate in v0.16.x — may unify if latency allows |
| Old app screens kept in "Legacy" section during Phase 2 | DEBT-004 | v0.15.x (remove) |

---

## 7. Acceptance Criteria

### 7.1 Protocol (Phase 1)

1. `TestProductRoundTrip` — Product serializes/deserializes with all config fields.
2. `TestTaskRoundTrip` — Task with each execution mode (once/iterate/interactive) round-trips.
3. `TestTaskTriggerTypes` — Task with each trigger type (manual/cron/webhook/on_complete) round-trips.
4. `TestRunHistory` — Task with multiple runs and iterations serializes correctly.
5. `TestAgentProfileRoundTrip` — Profile with template + skills round-trips.

### 7.2 Task Engine (Phase 1)

6. `TestTaskRunOnce` — task(mode=once) creates agent, sends prompt, waits, records run with status=done.
7. `TestTaskRunIterate` — task(mode=iterate) loops until verify passes or maxIterations reached.
8. `TestTaskRunIterateVerifyFail` — iteration records verify failure reason and continues.
9. `TestTaskCronTrigger` — task(trigger=cron) fires at the correct time via scheduler tick.
10. `TestTaskWebhookTrigger` — POST /api/trigger/{id} enqueues a run.
11. `TestTaskOnCompleteTrigger` — task B fires when task A's run reaches done.
12. `TestTaskChainDepthLimit` — chain deeper than 10 does not fire.
13. `TestTaskConcurrencySkip` — trigger with policy=skip does not enqueue if run in progress.
14. `TestProfileResolution` — task with ProfileID resolves provider/model/skills from profile.
15. `TestSkillInjection` — agent session created with skills has files at provider-native path.
16. `TestProductVerifyInheritance` — task without VerifyChecks inherits product-level checks.

### 7.3 Migration (Phase 1)

17. `TestMigrateLoopsToTasks` — loops.json entries become tasks with mode=iterate, all fields preserved.
18. `TestMigrateSchedulesToTasks` — schedules.json entries become tasks with trigger=cron, all fields preserved.
19. `TestMigrateProjectsToProducts` — projects.json entries become products with repos populated.
20. `TestOldRPCProxy` — loop/run, schedule/create still work and produce identical behaviour via task engine.

### 7.4 App (Phase 2)

21. Product list renders with name, repo count, active task count.
22. Task board shows tasks grouped by status (queued/running/done).
23. Task detail shows runs history with iteration results.
24. Live view streams ANSI output for a running task.
25. Inbox shows unread notifications with badge count.
26. New Task flow: title → prompt → mode → trigger → profile → create.

### 7.5 Cleanup (Phase 3)

27. Old RPCs (loop/*, schedule/*) return 410 Gone.
28. `daemon/internal/loop/` and `daemon/internal/schedule/` packages removed.
29. Old app screens removed from navigation.
30. `make ci` passes with no references to removed packages.

---

## 8. Implementation Plan

### Phase 1 — Data Layer + Engine (v0.13.x, ~3 weeks)

1. Add `protocol/message_product.go`, `message_task.go`, `message_profile.go`.
2. Implement `daemon/internal/task/` (engine, store, resolve, trigger, iterate).
3. Implement `daemon/internal/product/`, `profile/`, `skill/`, `notification/`.
4. Add new RPC handlers in `daemon/internal/server/`.
5. Wire old loop/* and schedule/* handlers as proxies to task engine.
6. Write migration functions (loops.json → tasks, schedules.json → tasks, projects.json → products).
7. All acceptance criteria §7.1–§7.3 pass.

### Phase 2 — App New UI (v0.14.x, ~3 weeks)

8. Implement Products tab (list + detail + task board).
9. Implement Task Detail (runs, live view, verification results).
10. Implement Inbox tab.
11. Implement Settings sub-pages (Profiles, Skills).
12. Implement New Task flow.
13. Keep old screens under "Legacy" settings entry.
14. All acceptance criteria §7.4 pass.

### Phase 3 — Cleanup (v0.15.x, ~1 week)

15. Remove old screens from navigation.
16. Remove old RPC handlers; return 410 for deprecated endpoints.
17. Remove `daemon/internal/loop/`, `daemon/internal/schedule/` packages.
18. Remove `protocol/message_loop.go`, `message_schedule.go`.
19. Remove migration code (one-time, already executed).
20. All acceptance criteria §7.5 pass.

---

## 9. Migration Path

### Data files

| File | Migration | Timing |
|------|-----------|--------|
| `~/.solo/loops.json` | → `tasks.json` (mode=iterate) + `products.json` (from Cwd) | First daemon start on v0.13.x |
| `~/.solo/schedules.json` | → `tasks.json` (trigger=cron) | First daemon start on v0.13.x |
| `~/.solo/projects.json` | → `products.json` | First daemon start on v0.13.x |
| `~/.solo/profiles.json` | New file, empty on first start | v0.13.x |
| `~/.solo/skills/` | New directory, empty on first start | v0.13.x |
| `~/.solo/notifications.json` | New file, empty on first start | v0.13.x |

### Wire clients (app-bridge)

- v0.13.x: app-bridge adds new task/product/profile/skill/notification schemas alongside existing loop/schedule schemas.
- v0.14.x: app switches to new schemas for all UI.
- v0.15.x: app-bridge removes loop/schedule schemas.

### Rollback safety

- Migration is **additive** in Phase 1: old files are not deleted, only read and transformed into new files.
- If v0.13.x is rolled back, old daemon reads old files (still present) and ignores new files.
- Old files are only deleted in Phase 3 (v0.15.x) after two release cycles of confirmed stability.

---

## 10. References

- `protocol/message_common.go` — AgentSessionConfig, AgentTemplate (ADR-001)
- `daemon/internal/loop/engine.go` — current loop execution (to be absorbed)
- `daemon/internal/schedule/executor.go` — current schedule execution (to be absorbed)
- `daemon/internal/agent/manager.go` — AgentClient/AgentSession interfaces (unchanged)
- `daemon/internal/agent/template.go` — template resolution helpers (extended by task/resolve.go)
- `app/src/screens/` — current screen inventory (to be restructured)
- [PRD: Product + Task Unification](../product/prd/product-task-unification.md) — full product spec
- [PRD: Product Layer Evolution](../product/prd/product-layer-evolution.md) — incremental alternative (superseded)
