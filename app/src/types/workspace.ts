export const PROJECT_KINDS = ["git", "non_git"] as const;
export type ProjectKind = (typeof PROJECT_KINDS)[number];

export const WORKSPACE_KINDS = ["local_checkout", "worktree", "directory"] as const;
export type WorkspaceKind = (typeof WORKSPACE_KINDS)[number];

export const WORKSPACE_STATUSES = ["done", "running", "failed", "needs_input", "attention"] as const;
export type WorkspaceStatus = (typeof WORKSPACE_STATUSES)[number];
