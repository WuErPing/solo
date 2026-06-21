import type { ProjectPathSource } from "./tmux-project-matcher";

export interface CwdItem {
  cwd: string;
  serverId: string;
}

const SOLO_WORKTREE_SEGMENT = "/.solo/worktrees/";

function stripTrailingSlash(path: string): string {
  if (path === "/") return path;
  return path.endsWith("/") ? path.slice(0, -1) : path;
}

function normalizePath(path: string): string {
  return stripTrailingSlash(path.trim());
}

function isChildPath(child: string, parent: string): boolean {
  return child.startsWith(parent) && (child.length === parent.length || child[parent.length] === "/");
}

function extractWorktreeParent(path: string): string | null {
  const idx = path.indexOf(SOLO_WORKTREE_SEGMENT);
  if (idx <= 0) return null;
  return normalizePath(path.slice(0, idx));
}

/**
 * Expand a leading ~ in `cwd` by inferring the home directory from
 * `projectPaths`. Falls back to process.env.HOME (works in Node / tests).
 */
function expandTilde(cwd: string, projectPaths: string[]): string {
  if (cwd !== "~" && !cwd.startsWith("~/")) return cwd;

  const relPart = cwd === "~" ? "" : cwd.slice(2); // after "~/"
  const home = process.env.HOME ?? process.env.USERPROFILE;
  if (home) return home + "/" + relPart;

  // Infer from project paths by finding the longest prefix of relPart that
  // a project path ends with. E.g. project "/Users/wuerping/code/wuerping/solo"
  // ends with "/code/wuerping/solo" which is a prefix of
  // "code/wuerping/solo/src", so home = "/Users/wuerping".
  for (const p of projectPaths) {
    if (!relPart) continue;
    // Check full relPart, then progressively shorter prefixes
    if (p.endsWith("/" + relPart)) {
      return stripTrailingSlash(p.slice(0, p.length - relPart.length)) + "/" + relPart;
    }
    let pos = relPart.length;
    while (pos > 0) {
      pos = relPart.lastIndexOf("/", pos - 1);
      if (pos < 0) break;
      const prefix = relPart.slice(0, pos);
      if (p.endsWith("/" + prefix)) {
        return stripTrailingSlash(p.slice(0, p.length - prefix.length)) + "/" + relPart;
      }
    }
  }
  return cwd;
}

function matchesProject(
  cwd: string,
  project: ProjectPathSource,
): boolean {
  const rootPath = project.projectRootPath ? normalizePath(project.projectRootPath) : null;
  const wsDir = project.workspaceDirectory ? normalizePath(project.workspaceDirectory) : null;

  if (rootPath && isChildPath(cwd, rootPath)) {
    return true;
  }

  if (wsDir && isChildPath(cwd, wsDir)) {
    return true;
  }

  const worktreeParent = extractWorktreeParent(cwd);
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

export function matchCwdToProjects(
  items: CwdItem[],
  projects: ProjectPathSource[],
): Map<string, number> {
  const counts = new Map<string, number>();

  // Collect all absolute project paths for tilde inference.
  const projectPaths: string[] = [];
  for (const p of projects) {
    if (p.projectRootPath) projectPaths.push(normalizePath(p.projectRootPath));
    if (p.workspaceDirectory) projectPaths.push(normalizePath(p.workspaceDirectory));
  }

  for (const it of items) {
    const cwd = normalizePath(expandTilde(it.cwd, projectPaths));
    if (!cwd) continue;

    const matchedProjects = new Set<string>();
    for (const proj of projects) {
      if (proj.serverId !== it.serverId) continue;
      if (!proj.projectRootPath && !proj.workspaceDirectory) continue;
      if (matchedProjects.has(proj.projectKey)) continue;

      if (matchesProject(cwd, proj)) {
        matchedProjects.add(proj.projectKey);
        counts.set(proj.projectKey, (counts.get(proj.projectKey) ?? 0) + 1);
      }
    }
  }

  return counts;
}
