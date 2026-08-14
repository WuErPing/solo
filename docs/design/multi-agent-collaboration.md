# Multi-Agent Collaboration Design

> Date: 2026-08-05
> Status: Proposal
> Working title: **Task Groups**

## Context

Solo's positioning (per the 2026 Roadmap §1.7) is a **Meta-Agent orchestration layer**: Solo does not replace external coding agents (Claude Code, Kimi, OpenCode, Codex, …) — it is their orchestration, observability, intervention, acceptance, and memory layer. External agents are the execution units; Solo is the workbench.

```
   Human (Web/App)
       │ intent (text / voice / screenshot)
       ▼
┌────────────── Solo (Meta-Agent Layer) ──────────────┐
│ intent → orchestrate → observe → intervene → accept │
└─────────────────────────────────────────────────────┘
       │ structured instructions      ▲ execution stream + results
       ▼                              │
  External Agent                 External Agent
  (Claude edits code)            (Kimi reviews it)
```

Multi-agent collaboration is the natural next step of this positioning: once several heterogeneous agents run side by side under one daemon, the missing product capability is making them work *together* — dividing a goal, handing results between steps, and giving the human a single place to review and accept the combined output.

This document defines the collaboration model, the data/primitive design, the UI, and the phased rollout.

**References**:

- [Roadmap 2026 §1.7](../product/roadmap-2026.md) — Meta-Agent orchestration-layer positioning
- [ADR-002: Product + Task Unification](../decisions/adr-002-product-task-unification.md) — "work object" unification; TaskGroup is a Task-scoped composition, not a seventh concept
- [Session Memory Persistence](../architecture/session-memory-persistence.md) — per-session turn recording under `~/.solo/memory/`
- [ADR-001: Shared Agent Template](../decisions/adr-001-shared-agent-template-for-loop-and-schedule.md) — `AgentTemplate = AgentSessionConfig` unification reused by the Pipeline design

## Current State

What already exists in the codebase (with paths), and what is missing.

### Exists

| Capability | Where | Notes |
|---|---|---|
| Multiple concurrent heterogeneous agent sessions per daemon | `daemon/internal/agent/manager.go` (`agents map[string]*ManagedAgent`) | No per-project limit; providers live behind the `AgentClient` / `AgentSession` interface in `daemon/internal/agent/providers/{claude,kimi,opencode,pi}` |
| Project linkage of agents | `Cwd` string on agent config | String matching only — no `projectID` foreign key; the protocol agent registry is flat |
| Worktree creation | `daemon/internal/workspace/worktree.go`, `protocol/message_worktree.go` | Used by workspace setup; not yet tied to agents automatically |
| Loop engine (2-agent pipeline) | `daemon/internal/loop/engine.go` (`runWorker` / `runVerifier`) | Hardcoded worker → verifier with text handoff; failure feedback folded into the next prompt; templates unified via ADR-001 (`AgentTemplate`) |
| Schedules that run agents | `daemon/internal/server/schedule_runner.go` | Targets an existing agent or spawns a new one (`new-agent` / `provider` target) in a cwd, waits for terminal state, deletes. No fan-out, no chaining |
| Tmux external-agent detection + I/O | `daemon/internal/server/session_tmux_scan.go`, `protocol/message_tmux.go` (`TmuxAgentInfo`, `Activity` = busy/idle), `app/src/screens/tmux-dashboard/` | 3-layer detection; `capture_pane` + `send_keys` give unstructured observe/intervene |
| Per-session turn memory | `daemon/internal/memory/` under `~/.solo/memory/` | Strictly per-session, append-only; **no cross-session retrieval path** |
| Agent stall detection | `daemon/internal/agent/stall_monitor.go` | Per-agent inactivity/repetition detection |
| App surfaces | `app/src/screens/sessions-screen.tsx` (flat sessions list), `app/src/stores/workspace-tabs-store.ts` (tab kinds `{draft, agent, terminal, file, setup}`), tmux dashboard cards | All flat — no grouping concept anywhere |

### Missing

- Agent-to-agent messaging (no protocol message carries a payload from one agent session to another)
- Session discovery / peer awareness (agents cannot see each other)
- Project-scoped agent grouping (flat registry, `Cwd`-string linkage only)
- Cross-session memory retrieval (memory is write-only per session)
- Structured I/O for tmux-detected external agents (raw capture_pane/send_keys only)
- A coordinator / orchestrator role, and any Task DAG

## Collaboration Levels and the Prioritization Decision

Four levels of collaboration, ordered by roadmap ambition:

| Level | Name | Description | State of infra |
|---|---|---|---|
| **L1** | Parallel divide-and-conquer (并行分工) | N agents in separate worktrees on one project; unified review/accept | Mostly exists — needs productization |
| **L2** | Pipeline collaboration (流水线协作) | Generalize the loop engine into a step chain (worker → reviewer → fixer); steps may use different providers ("Claude implements → Kimi reviews → OpenCode fixes") | Loop engine is the seed; needs generalization |
| **L3** | Conversational collaboration (对话式协作) | A Coordinator agent dispatches to Specialists; agent-to-agent messages | Nothing exists; roadmap places it Q1 2027 |
| **L4** | External Agent Bus | Structured I/O for tmux-detected external agents | Detection exists; structured event extraction does not |

**Key decision — invert the roadmap order.**

- **Do L1 + L2 first.** The infrastructure (agent manager, worktrees, loop engine, stall monitor) already exists; these two levels have the highest ROI for the least new machinery.
- **Converge L3**: keep it on the roadmap but descope it to a *role* (`role: coordinator` inside a TaskGroup) rather than a new subsystem.
- **Defer L4 and any full A2A protocol.** Same-machine, same-account collaboration does not need cross-vendor A2A semantics. A simple `agent_handoff` protocol message suffices until real cross-organization needs arise.

## Design: Three Product Primitives

```
┌─────────────────────── TaskGroup ───────────────────────┐
│ goal · mode (parallel | pipeline) · projectID           │
│                                                         │
│  members ──► Primitive 1: data model + protocol fields  │
│  worktrees ─► Primitive 2: isolation + acceptance flow  │
│  handoff ──► Primitive 3: structured pipeline context   │
└─────────────────────────────────────────────────────────┘
```

### Primitive 1: TaskGroup data model

```
TaskGroup {
  id, projectID, goal,
  mode: parallel | pipeline,
  members: [{ agentID, role: worker|reviewer|coordinator,
              worktreePath, scope, status }],
  sharedContext: ContextBundleRef
}
```

- `scope` is a path/glob declaration of what a member is expected to touch (e.g. `src/api/**`) — used for conflict warnings, not enforcement.
- `status` reuses existing agent session states plus stall-monitor busy/idle semantics.

**Protocol changes**:

- Agents gain `projectID` / `groupID` fields (replacing `Cwd`-string inference as the grouping key).
- `FetchAgentsRequest` gains a `groupID` filter.
- New `agent_handoff` message type — added in a later phase (P3), deliberately minimal.

**App**: the workspace tab strip (`app/src/stores/workspace-tabs-store.ts`) is upgraded to a group view — a TaskGroup renders as one tab cluster rather than N loose `agent` tabs.

### Primitive 2: Automatic worktree isolation + acceptance flow

- Group creation auto-creates one worktree per member agent, reusing `daemon/internal/workspace/worktree.go`. The member's agent session is spawned with `Cwd = worktreePath`.
- Acceptance UI shows each member's diff side-by-side against main, with per-member **Merge** / **Drop** back to the base branch.
- Cross-member conflict warnings: files touched by more than one member worktree are flagged before merge.
- Optional **"Ask reviewer agent to check"** action on any diff — this is the bridge from L1 to L2: the acceptance screen can spin up a reviewer step without leaving the flow.

### Primitive 3: Structured handoff / Pipeline

Generalize the hardcoded worker/verifier loop (`daemon/internal/loop/engine.go`) into:

```
Pipeline = [ Step{ template: AgentTemplate,
                   role,
                   input: prevOutput | fileGlob | userPrompt } ]
```

- A handoff between steps passes a **context bundle**: file list + git diff + the previous step's markdown output. The bundle is persisted through the existing memory module (`daemon/internal/memory/`).
- Each step may specify a different provider via its `AgentTemplate` (ADR-001) — "Claude implements → Kimi reviews → OpenCode fixes" is just a 3-step pipeline.
- Schedules can target a pipeline: `daemon/internal/server/schedule_runner.go` gains a pipeline target type, extending the existing new-agent/spawn/wait/delete lifecycle to a step chain.

## UI Design

Three layers, ~2.5 new screens, heavy reuse of existing components.

### Entry

- Sidebar gains a **"Task Groups"** entry. The flat Sessions list stays for single-agent work.

### Layer 1 — Task Groups list

Cards reusing the tmux-dashboard aggregated-card style:

```
┌──────────────────────────────────────────────────────────┐
│ Refactor auth module                       [待验收 2]     │
│ solo · parallel · 3 agents                    $1.24       │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐                      │
│ │◆ Claude │ │◆ Kimi   │ │◆ OC     │                      │
│ │ worker  │ │ worker  │ │reviewer │                      │
│ │ ●busy   │ │ ○idle   │ │ ●busy   │                      │
│ │ api/**  │ │ web/**  │ │ —       │                      │
│ └─────────┘ └─────────┘ └─────────┘                      │
│ ▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░  66%                                   │
└──────────────────────────────────────────────────────────┘
```

- Each card: goal, project, member mini-cards (provider icon, role, busy/idle from stall-monitor semantics, scope), progress bar, cost.
- The **"待验收" (pending acceptance) badge** is the primary action driver — it is what pulls the human back in.

### Layer 2 — Task Group detail

Three columns:

```
┌─────────────┬───────────────────────────┬──────────────────┐
│ Members     │ Conversation (embedded    │ Shared Context   │
│ (pipeline   │  existing agent view,     │ Changes          │
│  order,     │  as-is)                   │  per-worktree    │
│  handoff    │                           │  diff + Merge/   │
│  arrows;    │  Human can talk to any    │  Drop            │
│  parallel:  │  agent at any time —      │ Activity         │
│  flat list) │  intervention preserved   │  structured      │
│             │                           │  event stream    │
└─────────────┴───────────────────────────┴──────────────────┘
```

- **Left**: member list in pipeline order with handoff arrows between steps (parallel mode: flat list).
- **Middle**: the existing agent conversation view embedded unchanged. The human can talk to any member agent at any time — intervention-as-primitive is preserved.
- **Right**: Shared Context files; Changes (per-worktree diff + Merge/Drop); Activity stream (structured events: handoff, step started, step finished).
- Mobile: the three columns become three tabs, conversation tab default.

### Layer 3 — Creation flow

Single screen: goal, project, mode (`parallel` | `pipeline` | `custom`), agent list rows (provider picker, role, scope glob), auto-worktree checkbox, notify-on-complete (Expo push). Schedule creation reuses the same editor.

### Acceptance screen

Diff view reusing the existing file pane; conflict warnings across member worktrees; actions **[Ask reviewer agent to check] / [Merge] / [Drop]**.

### Deliberately not doing

- **No multi-agent chatroom view.** Agents cannot freely message each other in the MVP; the Activity stream suffices for visibility.
- **No dedicated Coordinator page.** In L3 a coordinator is just a member card with `role=coordinator`.

## Phased Plan

| Phase | Content | Depends on |
|---|---|---|
| **P0** | TaskGroup data model + protocol fields (`projectID`/`groupID`, group filter) + App group view; schedule multi-agent fan-out | Existing agent manager + schedule runner |
| **P1** | Worktree auto-isolation + acceptance/merge UI; loop engine generalized to Pipeline | P0 |
| **P2** | Handoff context bundle + cross-session memory retrieval (the memory module is write-only per-session today; this phase adds the read path) | P1 |
| **P3** | Coordinator role + `agent_handoff` message; then re-evaluate the tmux Agent Bus (L4) and full A2A | P2 |

## Risks

- **Acceptance burden.** Parallel output multiplies review load — the P1 acceptance UI and the reviewer pipeline step are mandatory companions, not optional extras.
- **Cost runaway.** N agents burn N× tokens. Mitigation: group-level token budgets + extending `stall_monitor.go` semantics to the group level.
- **Diff conflicts on the same files.** Worktrees + `scope` declarations mitigate but do not eliminate conflicts; set expectations in product copy instead of promising isolation.

## Companion Prototype

A mock-data prototype page is being built in the app (`app/src/screens/task-groups/` + route) as a companion to this document. Status: **prototype, no backend** — it validates the Layer 1/2 information architecture against realistic data before P0 protocol work begins.

## Design Principles

1. **Productize what exists before building what's new** — L1/L2 reuse the agent manager, worktrees, and loop engine; new machinery is limited to grouping, handoff, and acceptance.
2. **The human stays the orchestrator of last resort** — every member conversation remains directly reachable; automation never hides intervention.
3. **Acceptance is a first-class screen, not an afterthought** — parallel agents are only viable if review cost stays bounded.
4. **One grouping concept, reused everywhere** — TaskGroup serves the sessions list, schedules, pipelines, and (later) coordinators; no parallel grouping abstractions.
5. **Minimal protocol, honest scope** — `agent_handoff` beats a full A2A protocol for same-machine collaboration; defer semantics until a real cross-vendor need exists.
