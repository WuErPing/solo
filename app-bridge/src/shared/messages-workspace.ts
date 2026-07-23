import { z } from "zod";
import {
  WorkspaceDescriptorSchema,
  WorkspaceScriptSchema,
  WorkspaceGitRuntimeSchema,
  WorkspaceGitHubRuntimeSchema,
  type WorkspaceDescriptor,
  type WorkspaceScript,
  type WorkspaceGitRuntime,
  type WorkspaceGitHubRuntime,
} from "../generated/protocol-schemas.js";
import type { LiteralUnion } from "./literal-union.js";
import { CheckoutErrorSchema } from "./messages-git.js";
import { WorktreeSetupDetailPayloadSchema, AgentAttachmentsSchema } from "./messages-agent.js";

export {
  WorkspaceDescriptorSchema,
  WorkspaceScriptSchema,
  WorkspaceGitRuntimeSchema,
  WorkspaceGitHubRuntimeSchema,
  type WorkspaceDescriptor,
  type WorkspaceScript,
  type WorkspaceGitRuntime,
  type WorkspaceGitHubRuntime,
};

// Coerce null → undefined for Go pointer fields that marshal as JSON null.
const GitRuntimeSchemaNullSafe = WorkspaceGitRuntimeSchema.nullable()
  .optional()
  .transform((v) => v ?? undefined);
const GitHubRuntimeSchemaNullSafe = WorkspaceGitHubRuntimeSchema.nullable()
  .optional()
  .transform((v) => v ?? undefined);

// Extends the generated schema with the workspaceDirectory default that Go omits on the wire.
export const WorkspaceDescriptorSchemaWithDefault = WorkspaceDescriptorSchema.extend({
  gitRuntime: GitRuntimeSchemaNullSafe,
  githubRuntime: GitHubRuntimeSchemaNullSafe,
}).transform((wd) => ({
  ...wd,
  workspaceDirectory: wd.workspaceDirectory ?? wd.projectRootPath,
}));

const _WorkspaceStateBucketSchema = z.enum([
  "needs_input",
  "failed",
  "running",
  "attention",
  "done",
]);

export const FetchWorkspacesRequestMessageSchema = z.object({
  type: z.literal("fetch_workspaces_request"),
  requestId: z.string(),
  filter: z
    .object({
      query: z.string().optional(),
      projectId: z.string().optional(),
      idPrefix: z.string().optional(),
    })
    .optional(),
  sort: z
    .array(
      z.object({
        key: z.enum(["status_priority", "activity_at", "name", "project_id"]),
        direction: z.enum(["asc", "desc"]),
      }),
    )
    .optional(),
  page: z
    .object({
      limit: z.number().int().positive().max(200),
      cursor: z.string().min(1).optional(),
    })
    .optional(),
  subscribe: z
    .object({
      subscriptionId: z.string().optional(),
    })
    .optional(),
});

export const DirectorySuggestionsRequestSchema = z.object({
  type: z.literal("directory_suggestions_request"),
  query: z.string(),
  cwd: z.string().optional(),
  includeFiles: z.boolean().optional(),
  includeDirectories: z.boolean().optional(),
  limit: z.number().int().min(1).max(100).optional(),
  requestId: z.string(),
});

export const SoloWorktreeListRequestSchema = z.object({
  type: z.literal("solo_worktree_list_request"),
  cwd: z.string().optional(),
  repoRoot: z.string().optional(),
  requestId: z.string(),
});

export const SoloWorktreeArchiveRequestSchema = z.object({
  type: z.literal("solo_worktree_archive_request"),
  worktreePath: z.string().optional(),
  repoRoot: z.string().optional(),
  branchName: z.string().optional(),
  requestId: z.string(),
});

export const CreateSoloWorktreeRequestSchema = z.object({
  type: z.literal("create_solo_worktree_request"),
  cwd: z.string(),
  worktreeSlug: z.string().optional(),
  attachments: AgentAttachmentsSchema,
  refName: z.string().min(1).optional(),
  action: z.enum(["branch-off", "checkout"]).optional(),
  githubPrNumber: z.number().int().positive().optional(),
  requestId: z.string(),
});

export const WorkspaceSetupStatusRequestSchema = z.object({
  type: z.literal("workspace_setup_status_request"),
  workspaceId: z.string(),
  requestId: z.string(),
});

// TODO(2026-07): Remove once most clients are on >=0.1.50 and support arbitrary editor ids.
export const LEGACY_EDITOR_TARGET_IDS = [
  "cursor",
  "vscode",
  "zed",
  "finder",
  "explorer",
  "file-manager",
] as const;

export const KNOWN_EDITOR_TARGET_IDS = [...LEGACY_EDITOR_TARGET_IDS, "webstorm"] as const;

export const KnownEditorTargetIdSchema = z.enum(KNOWN_EDITOR_TARGET_IDS);
export const LegacyEditorTargetIdSchema = z.enum(LEGACY_EDITOR_TARGET_IDS);
export const EditorTargetIdSchema = z.string().trim().min(1);

const KNOWN_EDITOR_TARGET_ID_SET = new Set<string>(KNOWN_EDITOR_TARGET_IDS);
const LEGACY_EDITOR_TARGET_ID_SET = new Set<string>(LEGACY_EDITOR_TARGET_IDS);

export function isKnownEditorTargetId(value: string): value is KnownEditorTargetId {
  return KNOWN_EDITOR_TARGET_ID_SET.has(value);
}

export function isLegacyEditorTargetId(value: string): value is LegacyEditorTargetId {
  return LEGACY_EDITOR_TARGET_ID_SET.has(value);
}

export const EditorTargetDescriptorPayloadSchema = z.object({
  id: EditorTargetIdSchema,
  label: z.string(),
});

export const ListAvailableEditorsRequestSchema = z.object({
  type: z.literal("list_available_editors_request"),
  requestId: z.string(),
});

export const OpenInEditorRequestSchema = z.object({
  type: z.literal("open_in_editor_request"),
  path: z.string(),
  editorId: EditorTargetIdSchema,
  requestId: z.string(),
});

export const OpenProjectRequestSchema = z.object({
  type: z.literal("open_project_request"),
  cwd: z.string(),
  requestId: z.string(),
});

export const ArchiveWorkspaceRequestSchema = z.object({
  type: z.literal("archive_workspace_request"),
  workspaceId: z.string(),
  requestId: z.string(),
});

export const RemoveProjectRequestSchema = z.object({
  type: z.literal("remove_project_request"),
  workspaceIds: z.array(z.string()),
  requestId: z.string(),
});

const FileExplorerEntrySchema = z.object({
  name: z.string(),
  path: z.string(),
  kind: z.enum(["file", "directory"]),
  size: z.number(),
  modifiedAt: z.string(),
});

const FileExplorerFileSchema = z.object({
  path: z.string(),
  kind: z.enum(["text", "image", "binary"]),
  encoding: z.enum(["utf-8", "base64", "none"]),
  content: z.string().optional(),
  mimeType: z.string().optional(),
  size: z.number(),
  modifiedAt: z.string(),
});

const FileExplorerDirectorySchema = z.object({
  path: z.string(),
  entries: z.array(FileExplorerEntrySchema),
});

export const FileExplorerRequestSchema = z.object({
  type: z.literal("file_explorer_request"),
  cwd: z.string(),
  path: z.string().optional(),
  mode: z.enum(["list", "file"]),
  requestId: z.string(),
});

export const ProjectIconRequestSchema = z.object({
  type: z.literal("project_icon_request"),
  cwd: z.string(),
  requestId: z.string(),
});

export const FileDownloadTokenRequestSchema = z.object({
  type: z.literal("file_download_token_request"),
  cwd: z.string(),
  path: z.string(),
  requestId: z.string(),
});

export const StartWorkspaceScriptRequestSchema = z.object({
  type: z.literal("start_workspace_script_request"),
  workspaceId: z.string(),
  scriptName: z.string(),
  requestId: z.string(),
});

// ============================================================================
// Workspace Outbound Messages
// ============================================================================

export const WorkspaceScriptLifecycleSchema = z.enum(["running", "stopped"]);
export const WorkspaceScriptHealthSchema = z.enum(["healthy", "unhealthy"]);

export const WorkspaceScriptPayloadSchema = z.object({
  scriptName: z.string(),
  type: z.enum(["script", "service"]).optional().default("service"),
  hostname: z.string(),
  port: z.number().int().positive().nullable(),
  proxyUrl: z.string().nullable().optional().default(null),
  lifecycle: WorkspaceScriptLifecycleSchema,
  health: WorkspaceScriptHealthSchema.nullable(),
  exitCode: z.number().nullable().optional().default(null),
  terminalId: z.string().nullable().optional().default(null),
});

export const FetchWorkspacesResponseMessageSchema = z.object({
  type: z.literal("fetch_workspaces_response"),
  payload: z.object({
    requestId: z.string(),
    subscriptionId: z.string().nullable().optional(),
    entries: z.array(WorkspaceDescriptorSchemaWithDefault),
    pageInfo: z.object({
      nextCursor: z.string().nullable(),
      prevCursor: z.string().nullable(),
      hasMore: z.boolean(),
    }),
  }),
});

export const WorkspaceUpdateMessageSchema = z.object({
  type: z.literal("workspace_update"),
  payload: z.discriminatedUnion("kind", [
    z.object({
      kind: z.literal("upsert"),
      workspace: WorkspaceDescriptorSchemaWithDefault,
    }),
    z.object({
      kind: z.literal("remove"),
      id: z.string(),
    }),
  ]),
});

export const ScriptStatusUpdateMessageSchema = z.object({
  type: z.literal("script_status_update"),
  payload: z.object({
    workspaceId: z.string(),
    scripts: z.array(WorkspaceScriptPayloadSchema),
  }),
});

export const WorkspaceSetupProgressMessageSchema = z.object({
  type: z.literal("workspace_setup_progress"),
  payload: z.object({
    workspaceId: z.string(),
    status: z.enum(["running", "completed", "failed"]),
    detail: WorktreeSetupDetailPayloadSchema,
    error: z.string().nullable(),
  }),
});

export const WorkspaceSetupSnapshotSchema = z.object({
  status: z.enum(["running", "completed", "failed"]),
  detail: WorktreeSetupDetailPayloadSchema,
  error: z.string().nullable(),
});

export const WorkspaceSetupStatusResponseMessageSchema = z.object({
  type: z.literal("workspace_setup_status_response"),
  payload: z.object({
    requestId: z.string(),
    workspaceId: z.string(),
    snapshot: WorkspaceSetupSnapshotSchema.nullable(),
  }),
});

export const OpenProjectResponseMessageSchema = z.object({
  type: z.literal("open_project_response"),
  payload: z.object({
    requestId: z.string(),
    workspace: WorkspaceDescriptorSchemaWithDefault.nullable(),
    error: z.string().nullable(),
  }),
});

export const StartWorkspaceScriptResponseMessageSchema = z.object({
  type: z.literal("start_workspace_script_response"),
  payload: z.object({
    requestId: z.string(),
    workspaceId: z.string(),
    scriptName: z.string(),
    terminalId: z.string().nullable(),
    error: z.string().nullable(),
  }),
});

export const ListAvailableEditorsResponseMessageSchema = z.object({
  type: z.literal("list_available_editors_response"),
  payload: z.object({
    requestId: z.string(),
    editors: z.array(EditorTargetDescriptorPayloadSchema),
    error: z.string().nullable(),
  }),
});

export const OpenInEditorResponseMessageSchema = z.object({
  type: z.literal("open_in_editor_response"),
  payload: z.object({
    requestId: z.string(),
    error: z.string().nullable(),
  }),
});

export const ArchiveWorkspaceResponseMessageSchema = z.object({
  type: z.literal("archive_workspace_response"),
  payload: z.object({
    requestId: z.string(),
    workspaceId: z.string(),
    archivedAt: z.string().nullable(),
    error: z.string().nullable(),
  }),
});

export const RemoveProjectResponseMessageSchema = z.object({
  type: z.literal("remove_project_response"),
  payload: z.object({
    requestId: z.string(),
    workspaceIds: z.array(z.string()),
    removedCount: z.number(),
    error: z.string().nullable(),
  }),
});

export const DirectorySuggestionsResponseSchema = z.object({
  type: z.literal("directory_suggestions_response"),
  payload: z.object({
    directories: z.array(z.string()),
    entries: z
      .array(
        z.object({
          path: z.string(),
          kind: z.enum(["file", "directory"]),
        }),
      )
      .optional()
      .default([]),
    error: z.string().nullable(),
    requestId: z.string(),
  }),
});

const SoloWorktreeSchema = z.object({
  worktreePath: z.string(),
  createdAt: z.string(),
  branchName: z.string().nullable().optional(),
  head: z.string().nullable().optional(),
});

export const SoloWorktreeListResponseSchema = z.object({
  type: z.literal("solo_worktree_list_response"),
  payload: z.object({
    worktrees: z.array(SoloWorktreeSchema),
    error: CheckoutErrorSchema.nullable(),
    requestId: z.string(),
  }),
});

export const SoloWorktreeArchiveResponseSchema = z.object({
  type: z.literal("solo_worktree_archive_response"),
  payload: z.object({
    success: z.boolean(),
    removedAgents: z.array(z.string()).optional(),
    error: CheckoutErrorSchema.nullable(),
    requestId: z.string(),
  }),
});

export const CreateSoloWorktreeResponseSchema = z.object({
  type: z.literal("create_solo_worktree_response"),
  payload: z.object({
    workspace: WorkspaceDescriptorSchemaWithDefault.nullable(),
    error: z.string().nullable(),
    errorCode: z.string().optional(),
    setupTerminalId: z.string().nullable(),
    requestId: z.string(),
  }),
});

export const FileExplorerResponseSchema = z.object({
  type: z.literal("file_explorer_response"),
  payload: z.object({
    cwd: z.string(),
    path: z.string(),
    mode: z.enum(["list", "file"]),
    directory: FileExplorerDirectorySchema.nullable(),
    file: FileExplorerFileSchema.nullable(),
    error: z.string().nullable(),
    requestId: z.string(),
  }),
});

const ProjectIconSchema = z.object({
  data: z.string(),
  mimeType: z.string(),
});

export const ProjectIconResponseSchema = z.object({
  type: z.literal("project_icon_response"),
  payload: z.object({
    cwd: z.string(),
    icon: ProjectIconSchema.nullable(),
    error: z.string().nullable(),
    requestId: z.string(),
  }),
});

export const FileDownloadTokenResponseSchema = z.object({
  type: z.literal("file_download_token_response"),
  payload: z.object({
    cwd: z.string(),
    path: z.string(),
    token: z.string().nullable(),
    fileName: z.string().nullable(),
    mimeType: z.string().nullable(),
    size: z.number().nullable(),
    error: z.string().nullable(),
    requestId: z.string(),
  }),
});

// ---------------------------------------------------------------------------
// Type exports
// ---------------------------------------------------------------------------

export type WorkspaceStateBucket = z.infer<typeof _WorkspaceStateBucketSchema>;
export type WorkspaceDescriptorPayload = z.infer<typeof WorkspaceDescriptorSchemaWithDefault>;
export type WorkspaceScriptLifecycle = z.infer<typeof WorkspaceScriptLifecycleSchema>;
export type WorkspaceScriptHealth = z.infer<typeof WorkspaceScriptHealthSchema>;
export type WorkspaceScriptPayload = z.infer<typeof WorkspaceScriptPayloadSchema>;
export type KnownEditorTargetId = z.infer<typeof KnownEditorTargetIdSchema>;
export type LegacyEditorTargetId = z.infer<typeof LegacyEditorTargetIdSchema>;
export type EditorTargetId = LiteralUnion<KnownEditorTargetId, string>;
export type EditorTargetDescriptorPayload = z.infer<typeof EditorTargetDescriptorPayloadSchema>;
export type FetchWorkspacesRequestMessage = z.infer<typeof FetchWorkspacesRequestMessageSchema>;
export type FetchWorkspacesResponseMessage = z.infer<typeof FetchWorkspacesResponseMessageSchema>;
export type ScriptStatusUpdateMessage = z.infer<typeof ScriptStatusUpdateMessageSchema>;
export type WorkspaceSetupProgressMessage = z.infer<typeof WorkspaceSetupProgressMessageSchema>;
export type WorkspaceSetupSnapshot = z.infer<typeof WorkspaceSetupSnapshotSchema>;
export type WorkspaceSetupStatusResponseMessage = z.infer<
  typeof WorkspaceSetupStatusResponseMessageSchema
>;
export type OpenProjectResponseMessage = z.infer<typeof OpenProjectResponseMessageSchema>;
export type StartWorkspaceScriptResponseMessage = z.infer<
  typeof StartWorkspaceScriptResponseMessageSchema
>;
export type ListAvailableEditorsResponseMessage = z.infer<
  typeof ListAvailableEditorsResponseMessageSchema
>;
export type OpenInEditorResponseMessage = z.infer<typeof OpenInEditorResponseMessageSchema>;
export type ArchiveWorkspaceResponseMessage = z.infer<typeof ArchiveWorkspaceResponseMessageSchema>;
export type RemoveProjectResponseMessage = z.infer<typeof RemoveProjectResponseMessageSchema>;
export type DirectorySuggestionsRequest = z.infer<typeof DirectorySuggestionsRequestSchema>;
export type DirectorySuggestionsResponse = z.infer<typeof DirectorySuggestionsResponseSchema>;
export type SoloWorktreeListRequest = z.infer<typeof SoloWorktreeListRequestSchema>;
export type SoloWorktreeListResponse = z.infer<typeof SoloWorktreeListResponseSchema>;
export type SoloWorktreeArchiveRequest = z.infer<typeof SoloWorktreeArchiveRequestSchema>;
export type SoloWorktreeArchiveResponse = z.infer<typeof SoloWorktreeArchiveResponseSchema>;
export type CreateSoloWorktreeRequest = z.infer<typeof CreateSoloWorktreeRequestSchema>;
export type WorkspaceSetupStatusRequest = z.infer<typeof WorkspaceSetupStatusRequestSchema>;
export type ListAvailableEditorsRequest = z.infer<typeof ListAvailableEditorsRequestSchema>;
export type OpenInEditorRequest = z.infer<typeof OpenInEditorRequestSchema>;
export type OpenProjectRequest = z.infer<typeof OpenProjectRequestSchema>;
export type ArchiveWorkspaceRequest = z.infer<typeof ArchiveWorkspaceRequestSchema>;
export type RemoveProjectRequest = z.infer<typeof RemoveProjectRequestSchema>;
export type FileExplorerRequest = z.infer<typeof FileExplorerRequestSchema>;
export type FileExplorerResponse = z.infer<typeof FileExplorerResponseSchema>;
export type ProjectIconRequest = z.infer<typeof ProjectIconRequestSchema>;
export type ProjectIconResponse = z.infer<typeof ProjectIconResponseSchema>;
export type ProjectIcon = z.infer<typeof ProjectIconSchema>;
export type FileDownloadTokenRequest = z.infer<typeof FileDownloadTokenRequestSchema>;
export type FileDownloadTokenResponse = z.infer<typeof FileDownloadTokenResponseSchema>;
export type StartWorkspaceScriptRequest = z.infer<typeof StartWorkspaceScriptRequestSchema>;
export type StartWorkspaceScriptResponse = z.infer<
  typeof StartWorkspaceScriptResponseMessageSchema
>;
