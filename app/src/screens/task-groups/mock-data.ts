/**
 * Mock fixtures for the Task Groups (multi-agent collaboration) prototype.
 *
 * All data is local and static — there is no backend/store wiring yet.
 * Types mirror the model from docs/design/multi-agent-collaboration.md.
 */

export type AgentRole = "worker" | "reviewer" | "coordinator";
export type AgentStatus = "busy" | "idle" | "waiting" | "done" | "error";
export type TaskGroupMode = "parallel" | "pipeline";
export type TaskGroupProvider = "claude" | "kimi" | "opencode" | "pi";

export interface TaskGroupMember {
  agentID: string;
  provider: TaskGroupProvider;
  role: AgentRole;
  scope?: string;
  status: AgentStatus;
  worktreeBranch: string;
  additions: number;
  deletions: number;
}

export interface TaskGroup {
  id: string;
  goal: string;
  projectName: string;
  mode: TaskGroupMode;
  members: TaskGroupMember[];
  tasksDone: number;
  tasksTotal: number;
  costUSD: number;
  tokensK: number;
  /** Diffs awaiting acceptance. */
  pendingReviewCount: number;
  updatedAt: string;
}

export type TaskGroupActivityKind = "handoff" | "started" | "finished" | "merged";

export interface TaskGroupActivityEvent {
  time: string;
  text: string;
  kind: TaskGroupActivityKind;
}

export interface TaskGroupChatMessage {
  id: string;
  /** Member agentID, or "user" for the human coordinator. */
  authorID: string;
  time: string;
  text: string;
}

export interface TaskGroupSharedFile {
  path: string;
  addedBy: string;
  updatedAt: string;
}

export interface TaskGroupDetailExtras {
  activity: TaskGroupActivityEvent[];
  chat: TaskGroupChatMessage[];
  sharedFiles: TaskGroupSharedFile[];
}

export const MOCK_TASK_GROUPS: TaskGroup[] = [
  {
    id: "tg-auth-refresh",
    goal: "Add token refresh flow to the auth module",
    projectName: "solo",
    mode: "pipeline",
    members: [
      {
        agentID: "claude-auth",
        provider: "claude",
        role: "worker",
        scope: "auth/*",
        status: "done",
        worktreeBranch: "tg/auth-refresh/auth",
        additions: 412,
        deletions: 87,
      },
      {
        agentID: "kimi-tokens",
        provider: "kimi",
        role: "worker",
        scope: "tokens/*",
        status: "busy",
        worktreeBranch: "tg/auth-refresh/tokens",
        additions: 268,
        deletions: 41,
      },
      {
        agentID: "opencode-review",
        provider: "opencode",
        role: "reviewer",
        scope: "auth/*, tokens/*",
        status: "waiting",
        worktreeBranch: "tg/auth-refresh/review",
        additions: 0,
        deletions: 0,
      },
    ],
    tasksDone: 5,
    tasksTotal: 8,
    costUSD: 3.42,
    tokensK: 812,
    pendingReviewCount: 0,
    updatedAt: "2026-08-05T14:52:00Z",
  },
  {
    id: "tg-billing-migration",
    goal: "Migrate billing pages to the new pricing table",
    projectName: "solo-web",
    mode: "pipeline",
    members: [
      {
        agentID: "kimi-schema",
        provider: "kimi",
        role: "worker",
        scope: "billing/schema/*",
        status: "done",
        worktreeBranch: "tg/billing/schema",
        additions: 190,
        deletions: 64,
      },
      {
        agentID: "claude-ui",
        provider: "claude",
        role: "worker",
        scope: "billing/ui/*",
        status: "done",
        worktreeBranch: "tg/billing/ui",
        additions: 524,
        deletions: 203,
      },
      {
        agentID: "pi-review",
        provider: "pi",
        role: "reviewer",
        scope: "billing/*",
        status: "done",
        worktreeBranch: "tg/billing/review",
        additions: 12,
        deletions: 3,
      },
    ],
    tasksDone: 12,
    tasksTotal: 12,
    costUSD: 7.18,
    tokensK: 1904,
    pendingReviewCount: 2,
    updatedAt: "2026-08-05T13:20:00Z",
  },
  {
    id: "tg-docs-sweep",
    goal: "Sweep docs: refresh provider guides and fix stale links",
    projectName: "solo",
    mode: "parallel",
    members: [
      {
        agentID: "claude-providers",
        provider: "claude",
        role: "worker",
        scope: "docs/providers/*",
        status: "busy",
        worktreeBranch: "tg/docs/providers",
        additions: 233,
        deletions: 96,
      },
      {
        agentID: "kimi-arch",
        provider: "kimi",
        role: "worker",
        scope: "docs/architecture/*",
        status: "idle",
        worktreeBranch: "tg/docs/architecture",
        additions: 58,
        deletions: 12,
      },
      {
        agentID: "pi-links",
        provider: "pi",
        role: "worker",
        scope: "docs/**/*.md links",
        status: "done",
        worktreeBranch: "tg/docs/links",
        additions: 41,
        deletions: 39,
      },
    ],
    tasksDone: 4,
    tasksTotal: 7,
    costUSD: 1.87,
    tokensK: 445,
    pendingReviewCount: 1,
    updatedAt: "2026-08-05T14:41:00Z",
  },
  {
    id: "tg-checkout-redesign",
    goal: "Redesign checkout diff view with split panes",
    projectName: "solo",
    mode: "pipeline",
    members: [
      {
        agentID: "opencode-plan",
        provider: "opencode",
        role: "coordinator",
        status: "done",
        worktreeBranch: "tg/checkout/plan",
        additions: 24,
        deletions: 2,
      },
      {
        agentID: "kimi-diff",
        provider: "kimi",
        role: "worker",
        scope: "components/checkout/*",
        status: "error",
        worktreeBranch: "tg/checkout/diff-view",
        additions: 342,
        deletions: 178,
      },
      {
        agentID: "claude-review",
        provider: "claude",
        role: "reviewer",
        scope: "components/checkout/*",
        status: "waiting",
        worktreeBranch: "tg/checkout/review",
        additions: 0,
        deletions: 0,
      },
    ],
    tasksDone: 3,
    tasksTotal: 9,
    costUSD: 2.64,
    tokensK: 698,
    pendingReviewCount: 0,
    updatedAt: "2026-08-05T11:05:00Z",
  },
];

export const MOCK_TASK_GROUP_DETAILS: Record<string, TaskGroupDetailExtras> = {
  "tg-auth-refresh": {
    activity: [
      { time: "14:02", kind: "started", text: "Group created — pipeline with 3 members" },
      { time: "14:05", kind: "started", text: "claude-auth started on auth/* (tg/auth-refresh/auth)" },
      { time: "14:31", kind: "finished", text: "claude-auth finished — +412 −87 across 9 files" },
      { time: "14:31", kind: "handoff", text: "Handoff: claude-auth → kimi-tokens (token store contract)" },
      { time: "14:32", kind: "started", text: "kimi-tokens started on tokens/* (tg/auth-refresh/tokens)" },
      { time: "14:52", kind: "handoff", text: "kimi-tokens posted status: refresh rotation implemented, tests running" },
    ],
    chat: [
      { id: "m1", authorID: "user", time: "14:02", text: "Pipeline: auth first, then tokens, then review. Keep the RefreshTokenStore interface stable." },
      { id: "m2", authorID: "claude-auth", time: "14:28", text: "auth/* done. RefreshTokenStore is in auth/refresh-store.ts — kimi-tokens can build tokens/* against it." },
      { id: "m3", authorID: "kimi-tokens", time: "14:32", text: "Pulled the contract. Implementing rotation + reuse detection in tokens/refresh.ts." },
      { id: "m4", authorID: "user", time: "14:40", text: "Make sure revoked-token reuse wipes the whole family, per OWASP guidance." },
      { id: "m5", authorID: "kimi-tokens", time: "14:52", text: "Rotation + family revocation implemented. Running the token test suite now." },
    ],
    sharedFiles: [
      { path: "auth/refresh-store.ts", addedBy: "claude-auth", updatedAt: "14:28" },
      { path: "auth/session.ts", addedBy: "claude-auth", updatedAt: "14:31" },
      { path: "tokens/refresh.ts", addedBy: "kimi-tokens", updatedAt: "14:52" },
      { path: "tokens/rotation.test.ts", addedBy: "kimi-tokens", updatedAt: "14:50" },
    ],
  },
  "tg-billing-migration": {
    activity: [
      { time: "09:12", kind: "started", text: "Group created — pipeline with 3 members" },
      { time: "10:47", kind: "finished", text: "kimi-schema finished — +190 −64" },
      { time: "10:48", kind: "handoff", text: "Handoff: kimi-schema → claude-ui (pricing table schema)" },
      { time: "12:56", kind: "finished", text: "claude-ui finished — +524 −203" },
      { time: "13:20", kind: "finished", text: "pi-review approved with 2 change requests" },
    ],
    chat: [
      { id: "m1", authorID: "user", time: "09:12", text: "Move all billing pages to the new PricingTable. Schema first, UI second, review last." },
      { id: "m2", authorID: "kimi-schema", time: "10:47", text: "Schema migration done — billing/schema/v3.ts with seat-based tiers." },
      { id: "m3", authorID: "claude-ui", time: "12:56", text: "All 4 billing pages rebuilt on PricingTable. Old plan cards removed." },
      { id: "m4", authorID: "pi-review", time: "13:20", text: "2 diffs need your acceptance: yearly-discount badge and the legacy redirect." },
    ],
    sharedFiles: [
      { path: "billing/schema/v3.ts", addedBy: "kimi-schema", updatedAt: "10:47" },
      { path: "billing/ui/pricing-table.tsx", addedBy: "claude-ui", updatedAt: "12:51" },
      { path: "billing/ui/plan-card.tsx", addedBy: "claude-ui", updatedAt: "12:56" },
    ],
  },
  "tg-docs-sweep": {
    activity: [
      { time: "13:58", kind: "started", text: "Group created — parallel with 3 workers" },
      { time: "14:00", kind: "started", text: "All workers started on disjoint doc scopes" },
      { time: "14:22", kind: "finished", text: "pi-links finished — fixed 39 stale links" },
      { time: "14:22", kind: "merged", text: "tg/docs/links queued for merge (pending review)" },
      { time: "14:41", kind: "handoff", text: "claude-providers reported: 3 of 5 provider guides refreshed" },
    ],
    chat: [
      { id: "m1", authorID: "user", time: "13:58", text: "Parallel sweep — no overlaps between scopes. pi-links, only touch links, no prose." },
      { id: "m2", authorID: "pi-links", time: "14:22", text: "Done. 39 stale links fixed across 21 files." },
      { id: "m3", authorID: "claude-providers", time: "14:41", text: "Claude/Kimi/OpenCode guides refreshed. Pi and Codex guides still in progress." },
    ],
    sharedFiles: [
      { path: "docs/providers/kimi.md", addedBy: "claude-providers", updatedAt: "14:40" },
      { path: "docs/architecture/tmux-pane-content-loading.md", addedBy: "kimi-arch", updatedAt: "14:18" },
      { path: "docs/README.md", addedBy: "pi-links", updatedAt: "14:21" },
    ],
  },
  "tg-checkout-redesign": {
    activity: [
      { time: "10:02", kind: "started", text: "Group created — pipeline with 3 members" },
      { time: "10:20", kind: "finished", text: "opencode-plan published the split-pane plan" },
      { time: "10:21", kind: "handoff", text: "Handoff: opencode-plan → kimi-diff" },
      { time: "11:05", kind: "finished", text: "kimi-diff crashed: sandbox OOM while running checkout tests" },
    ],
    chat: [
      { id: "m1", authorID: "user", time: "10:02", text: "Goal: split checkout diff into side-by-side panes with a unified fallback." },
      { id: "m2", authorID: "opencode-plan", time: "10:20", text: "Plan posted: reuse SplitContainer, keep DiffLine model unchanged." },
      { id: "m3", authorID: "kimi-diff", time: "11:05", text: "ERROR: worker crashed (OOM in test run). Work so far is committed on tg/checkout/diff-view." },
    ],
    sharedFiles: [
      { path: "components/checkout/split-diff-plan.md", addedBy: "opencode-plan", updatedAt: "10:20" },
      { path: "components/checkout/split-diff-view.tsx", addedBy: "kimi-diff", updatedAt: "11:04" },
    ],
  },
};

export function getTaskGroupById(id: string): TaskGroup | undefined {
  return MOCK_TASK_GROUPS.find((group) => group.id === id);
}

export function getTaskGroupDetails(id: string): TaskGroupDetailExtras {
  return (
    MOCK_TASK_GROUP_DETAILS[id] ?? { activity: [], chat: [], sharedFiles: [] }
  );
}
