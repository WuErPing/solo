# Tmux-Project Matcher

Date: 2026-07-29
Status: Implemented
Supersedes: plan-tmux-project-matcher.md, spec-tmux-project-matcher.md

---

## Overview

The tmux-project matcher associates live tmux panes (and the agents running in
them) with sidebar projects by comparing each pane's working directory against
each project's known paths. The result drives the activity badges shown on
sidebar project rows, so a user can see at a glance which projects have active
tmux panes — and how many — without opening the tmux dashboard.

The design keeps two data flows orthogonal:

- **Workspace structure** (`useSidebarWorkspacesList` → projects/workspaces) is
  never polluted with pane counts.
- **Pane counts** are computed independently by a pure matcher plus a thin hook
  (`useTmuxProjectCounts`) and passed down the component tree as a separate map.

## Contract / API

Pure matching logic lives in `app/src/utils/tmux-project-matcher.ts`.

### Types

```typescript
export interface TmuxPaneSource {
  serverId: string;
  workingDir: string;
  kind: "agent" | "pane";
}

export interface ProjectPathSource {
  projectKey: string;
  serverId: string;
  projectRootPath?: string;
  workspaceDirectory?: string;
}

export interface ProjectPaneCounts {
  agentCount: number;
  paneCount: number;
  loopCount: number;
  scheduleCount: number;
}
```

### Functions

```typescript
export function matchTmuxToProjects(
  panes: TmuxPaneSource[],
  projects: ProjectPathSource[],
): Map<string, ProjectPaneCounts>;

export function matchesWorkingDir(dir: string, filterDir: string): boolean;
```

- `matchTmuxToProjects` returns a map keyed by `projectKey`. For each matched
  pane it increments `paneCount`, and additionally increments `agentCount` when
  the pane's `kind` is `"agent"`. (`loopCount` / `scheduleCount` are always 0
  here; they are populated elsewhere and merged in the sidebar — see below.)
- `matchesWorkingDir` is a standalone helper exposing the same path-matching
  rules for a single directory/filter pair.

### Matching rules

1. **Normalize** paths: trim whitespace and strip a single trailing `/`
   (root `/` is preserved).
2. A pane matches a project when its directory is the project path **or a child
   of it**, where "child" requires a `/` boundary (so `/repo-extended` does NOT
   match `/repo`).
3. Matching is attempted against **both** `projectRootPath` and
   `workspaceDirectory` (when present).
4. **Worktree backtracking:** if the pane directory contains
   `/.solo/worktrees/`, the segment before it is treated as the parent repo path
   and re-matched against the project paths. This lets a pane inside a solo
   worktree count toward the originating project.
5. **Host scoping:** a pane only matches a project when
   `pane.serverId === project.serverId` (no cross-host matches).
6. **Deduplication:** a single pane is counted at most once per `projectKey`,
   even if a project has multiple workspaces that both match.
7. Projects with neither `projectRootPath` nor `workspaceDirectory` are skipped.

Paths are compared case-sensitively (macOS/Linux semantics).

### Hook

`app/src/hooks/use-tmux-project-counts.ts`:

```typescript
export function useTmuxProjectCounts(
  projects: SidebarProjectEntry[],
  serverId: string | null,
  enabled = true,
): Map<string, ProjectPaneCounts>;
```

The hook also exports two pure helpers used internally (and covered by tests):

```typescript
export function buildProjectPathSources(
  projects: SidebarProjectEntry[],
  serverId: string,
): ProjectPathSource[];

export function buildPaneSources(
  agents: TmuxAgent[],
  otherPanes: TmuxPane[],
  serverId: string,
): TmuxPaneSource[];
```

Behavior:

- Pulls `agents` and `otherPanes` from `useAggregatedTmuxAgents({ enabled })`
  (existing ~5s polling source, data across all hosts).
- `buildPaneSources` keeps only entries for the active `serverId` with a
  non-empty `workingDir`; agents with `status === "exited"` are excluded. Agents
  become `kind: "agent"`, other panes become `kind: "pane"`.
- `buildProjectPathSources` emits one `ProjectPathSource` per workspace on the
  active host that has path information (a project with multiple workspaces
  yields multiple sources; the matcher dedupes per project).
- Runs `matchTmuxToProjects` inside `useMemo` and returns the count map. Returns
  an empty map when there is no `serverId`, no projects, or no pane sources.

## Implementation notes

- **No `paneCount` on `SidebarProjectEntry`.** Pane counts are a separate
  concern from workspace-structure data, so they flow through an independent
  hook and a separate prop (`paneCountMap`) rather than extending the project
  model.
- **Memoization:** matching runs in `useMemo` keyed on
  `[agents, otherPanes, projects, serverId]` to avoid recompute on unrelated
  renders. The sidebar additionally gates polling via the `enabled` flag
  (`isCompactLayout || isOpen`).
- **Badge rendering** (`app/src/components/sidebar-workspace-list.tsx`): the
  pane badge is an orange pill (`palette.orange[500]`, white text + Terminal
  icon) rendered as a `Pressable` that calls `onPaneBadgePress` with the
  project's icon working dir. It is shown only when `paneCount > 0`. A separate
  agent badge (Bot icon) renders from `agentCount`.
- **Count merging** (`app/src/components/left-sidebar.tsx`): the tmux map is
  merged with results from `useSoloAgentCounts`, `useLoopProjectCounts`, and
  `useScheduleProjectCounts` into a single `Map<string, ProjectPaneCounts>`. In
  the merged map, `agentCount` comes from the solo-agent hook and `paneCount`
  from the tmux matcher; `loopCount` / `scheduleCount` come from their
  respective hooks. The merged map is threaded through
  `SidebarWorkspaceList` → `ProjectBlock` → `ProjectHeaderRow`.

## Testing

Vitest, co-located with source:

- `app/src/utils/tmux-project-matcher.test.ts` — exact/child/worktree matching,
  `/`-boundary (no partial-name match), cross-host isolation, trailing slashes,
  multi-pane counts, agent vs. pane distinction, per-project dedup across
  multiple workspaces, empty inputs, and `matchesWorkingDir`.
- `app/src/hooks/use-tmux-project-counts.test.ts` — `buildProjectPathSources`
  (host filtering, workspaceDirectory passthrough, skipping pathless
  workspaces) and `buildPaneSources` (agent/pane kinds, excluding exited
  agents, host filtering, skipping empty working dirs).

Commands:

```
cd app && npx vitest run src/utils/tmux-project-matcher.test.ts
cd app && npx vitest run src/hooks/use-tmux-project-counts.test.ts
cd app && npx expo lint --max-warnings 0
```

## Files

- `app/src/utils/tmux-project-matcher.ts` — pure matcher + `matchesWorkingDir`.
- `app/src/utils/tmux-project-matcher.test.ts` — matcher unit tests.
- `app/src/hooks/use-tmux-project-counts.ts` — hook + source-builder helpers.
- `app/src/hooks/use-tmux-project-counts.test.ts` — helper unit tests.
- `app/src/components/left-sidebar.tsx` — calls the hook, merges count maps,
  threads `paneCountMap` into the list.
- `app/src/components/sidebar-workspace-list.tsx` — renders the pane/agent
  badges on project rows.

## Superseded documents

- `docs/analysis/plan-tmux-project-matcher.md`
- `docs/analysis/spec-tmux-project-matcher.md`
