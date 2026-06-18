export interface TmuxPaneSource {
  serverId: string;
  workingDir: string;
  kind: "agent" | "pane";
}

export interface ProjectPaneCounts {
  agentCount: number;
  paneCount: number;
}

export interface ProjectPathSource {
  projectKey: string;
  serverId: string;
  projectRootPath?: string;
  workspaceDirectory?: string;
}

const SOLO_WORKTREE_SEGMENT = "/.solo/worktrees/";

function normalizePath(path: string): string {
  const trimmed = path.trim();
  if (trimmed === "/") return trimmed;
  return trimmed.endsWith("/") ? trimmed.slice(0, -1) : trimmed;
}

function isChildPath(child: string, parent: string): boolean {
  return child.startsWith(parent) && (child.length === parent.length || child[parent.length] === "/");
}

function extractWorktreeParent(path: string): string | null {
  const idx = path.indexOf(SOLO_WORKTREE_SEGMENT);
  if (idx <= 0) return null;
  return normalizePath(path.slice(0, idx));
}

function matchesProject(
  paneDir: string,
  project: ProjectPathSource,
): boolean {
  const rootPath = project.projectRootPath ? normalizePath(project.projectRootPath) : null;
  const wsDir = project.workspaceDirectory ? normalizePath(project.workspaceDirectory) : null;

  // Direct match against projectRootPath
  if (rootPath && isChildPath(paneDir, rootPath)) {
    return true;
  }

  // Direct match against workspaceDirectory
  if (wsDir && isChildPath(paneDir, wsDir)) {
    return true;
  }

  // Worktree backtracking: if pane is inside .solo/worktrees/, extract parent repo
  const worktreeParent = extractWorktreeParent(paneDir);
  if (worktreeParent) {
    if (rootPath && isChildPath(worktreeParent, rootPath)) {
      return true;
    }
    if (wsDir && isChildPath(worktreeParent, wsDir)) {
      return true;
    }
  }

  return false;
}

/**
 * Returns true if `dir` matches `filterDir` — either exact match or child path.
 * Handles trailing slashes and worktree backtracking.
 */
export function matchesWorkingDir(dir: string, filterDir: string): boolean {
  const a = normalizePath(dir);
  const b = normalizePath(filterDir);
  if (!a || !b) return false;
  if (isChildPath(a, b)) return true;
  const parent = extractWorktreeParent(a);
  if (parent && isChildPath(parent, b)) return true;
  return false;
}

export function matchTmuxToProjects(
  panes: TmuxPaneSource[],
  projects: ProjectPathSource[],
): Map<string, ProjectPaneCounts> {
  const counts = new Map<string, ProjectPaneCounts>();

  for (const p of panes) {
    const paneDir = normalizePath(p.workingDir);
    if (!paneDir) continue;

    for (const proj of projects) {
      if (proj.serverId !== p.serverId) continue;
      if (!proj.projectRootPath && !proj.workspaceDirectory) continue;

      if (matchesProject(paneDir, proj)) {
        const entry = counts.get(proj.projectKey) ?? { agentCount: 0, paneCount: 0 };
        entry.paneCount++;
        if (p.kind === "agent") {
          entry.agentCount++;
        }
        counts.set(proj.projectKey, entry);
      }
    }
  }

  return counts;
}
