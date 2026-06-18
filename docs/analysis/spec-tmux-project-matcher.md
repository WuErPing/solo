# Spec: Tmux-Project Matcher (Steps 1-2)

## Objective

Associate tmux panes with sidebar projects via working directory path matching, then surface that association as a visual badge on sidebar project rows.

**User story:** As a user, when I look at my sidebar project list, I want to see which projects have active tmux panes running, so I can quickly gauge project activity without opening the tmux dashboard.

**Success criteria:**
1. A pure function `matchTmuxToProjects()` correctly maps tmux agents/panes to sidebar projects by path
2. Sidebar project rows display a badge showing the count of associated tmux panes
3. All matching logic is covered by unit tests

## Tech Stack

- TypeScript (existing app codebase)
- Vitest for unit tests (existing test framework)
- React Native + Unistyles for badge rendering

## Commands

```
Test:    cd app && npx vitest run src/utils/tmux-project-matcher.test.ts
Lint:    cd app && npx expo lint --max-warnings 0
Full:    cd app && npm test
```

## Project Structure

```
app/src/utils/tmux-project-matcher.ts        ← Pure matching function (Step 1)
app/src/utils/tmux-project-matcher.test.ts   ← Unit tests (Step 1)
app/src/hooks/use-tmux-project-counts.ts     ← Hook combining matcher + data (Step 2)
app/src/components/sidebar-workspace-list.tsx ← Badge rendering (Step 2, existing file)
```

## Code Style

Follow existing patterns in the codebase:

```typescript
// Pure functions with explicit input/output types (like buildProjects in utils/projects.ts)
export interface TmuxProjectMatchInput {
  agents: TmuxAgent[];
  otherPanes: TmuxPane[];
  projects: SidebarProjectEntry[];
}

export interface TmuxProjectMatchResult {
  matchCountByProjectKey: Map<string, number>;
}

export function matchTmuxToProjects(input: TmuxProjectMatchInput): TmuxProjectMatchResult {
  // ...
}
```

## Testing Strategy

- **Framework:** Vitest (existing)
- **Test location:** `app/src/utils/tmux-project-matcher.test.ts` (co-located with source)
- **Test cases:**
  - Exact path match
  - Child path match (with `/` boundary)
  - Worktree path backtracking (`.solo/worktrees/` → parent repo)
  - No match (unrelated paths)
  - Different serverId → no cross-host match
  - Empty inputs (no agents, no projects)
  - Case sensitivity (paths are case-sensitive on macOS/Linux)

## Boundaries

- **Always do:** Pure function tests, follow existing naming conventions, use `Map<string, number>` for results
- **Ask first:** Changing sidebar layout/spacing, adding new UI components
- **Never do:** Daemon changes, symlink resolution, modifying tmux data fetching logic

## Open Questions

None — requirements are clear from the prior analysis.
