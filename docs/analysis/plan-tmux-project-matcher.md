# Implementation Plan: Tmux-Project Matcher (Steps 1-2)

## Architecture Decision

**Do NOT add `paneCount` to `SidebarProjectEntry`.** That type flows through `useSidebarWorkspacesList` → `buildSidebarProjectsFromStructure`, which is workspace-structure data. Tmux pane counts are a separate concern and would pollute the workspace data model.

Instead, create an independent hook `useTmuxProjectCounts` that:
- Consumes `useAggregatedTmuxAgents()` (existing, 5s polling)
- Consumes projects from `useSidebarWorkspacesList` (already available in `LeftSidebar`)
- Runs the pure matcher function
- Returns `Map<string, number>`

The count map is passed as a separate prop through the component tree, keeping the two data flows orthogonal.

## Tasks

### Task 1: Pure matching function

**File:** `app/src/utils/tmux-project-matcher.ts`

```typescript
export interface TmuxPaneSource {
  serverId: string;
  workingDir: string;
}

export interface ProjectPathSource {
  projectKey: string;
  serverId: string;
  projectRootPath?: string;
  workspaceDirectory?: string;
}

export function matchTmuxToProjects(
  panes: TmuxPaneSource[],
  projects: ProjectPathSource[],
): Map<string, number>
```

**Matching logic:**
1. Normalize paths (trim trailing `/`)
2. For each pane, check each project:
   - Exact match: `pane.workingDir === project.projectRootPath`
   - Child path: `pane.workingDir.startsWith(project.projectRootPath + "/")`
   - Worktree: if path contains `/.solo/worktrees/`, extract the parent repo path and re-match
3. Only match when `pane.serverId === project.serverId`
4. Count matches per `projectKey`

**Acceptance:** Function handles exact, child, worktree, no-match, cross-host, empty inputs.
**Verify:** `npx vitest run src/utils/tmux-project-matcher.test.ts`

### Task 2: Unit tests for matching function

**File:** `app/src/utils/tmux-project-matcher.test.ts`

Test cases:
- Exact path match → count 1
- Child path (`/repo/src` under `/repo`) → count 1
- Worktree path (`.solo/worktrees/feat-x` under `/repo`) → count 1
- Unrelated path → no match
- Different serverId → no match
- Multiple panes in same project → count > 1
- Empty panes → empty map
- Empty projects → empty map
- Trailing slashes handled correctly

**Acceptance:** All test cases pass.
**Verify:** `npx vitest run src/utils/tmux-project-matcher.test.ts`

### Task 3: Hook that combines matcher with live data

**File:** `app/src/hooks/use-tmux-project-counts.ts`

```typescript
export function useTmuxProjectCounts(
  projects: SidebarProjectEntry[],
  serverId: string | null,
): Map<string, number>
```

- Calls `useAggregatedTmuxAgents()` to get `agents` and `otherPanes`
- Maps agents/panes to `TmuxPaneSource[]` (using `serverId` + `workingDir`)
- Maps projects to `ProjectPathSource[]` (using `serverId` from the caller context + `projectRootPath`/`workspaceDirectory` from each workspace)
- Calls `matchTmuxToProjects()` inside `useMemo`
- Returns the count map

**Note:** `useAggregatedTmuxAgents()` returns data across ALL hosts. The hook filters by the active `serverId` to match the sidebar's current host context.

**Acceptance:** Hook returns correct counts, memoized, no unnecessary re-renders.
**Verify:** `npx vitest run src/hooks/use-tmux-project-counts.test.ts`

### Task 4: Badge rendering in sidebar

**Files:**
- `app/src/components/sidebar-workspace-list.tsx` — add `paneCount?: number` to `ProjectHeaderRowProps`, render badge in `projectTitleGroup`
- `app/src/components/left-sidebar.tsx` — call `useTmuxProjectCounts`, pass counts to `SidebarWorkspaceList`
- `app/src/components/sidebar-workspace-list.tsx` — thread `paneCountMap` through `SidebarWorkspaceList` → `ProjectBlock` → `ProjectHeaderRow`

**Badge rendering:**
```tsx
{paneCount != null && paneCount > 0 ? (
  <View style={styles.paneBadge}>
    <Text style={styles.paneBadgeText}>{paneCount}</Text>
  </View>
) : null}
```

**Badge style:** Small pill, muted background (`theme.colors.surface2`), muted foreground text, compact size. Matches the aesthetic of existing sidebar indicators (status dots, shortcut badges).

**Acceptance:** Badge appears on projects with active tmux panes, hidden for 0.
**Verify:** `cd app && npx expo lint --max-warnings 0 && npm test`
