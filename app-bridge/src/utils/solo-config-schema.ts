import { z } from "zod";

export function normalizeLifecycleCommands(commands: unknown): string[] {
  if (typeof commands === "string") {
    return commands.trim().length > 0 ? [commands] : [];
  }
  if (!Array.isArray(commands)) {
    return [];
  }
  return commands.filter((command): command is string => {
    return typeof command === "string" && command.trim().length > 0;
  });
}

export const SoloLifecycleCommandRawSchema = z.union([z.string(), z.array(z.string())]);

export const SoloScriptEntryRawSchema = z
  .object({
    type: z.unknown().optional(),
    command: z.unknown().optional(),
    port: z.unknown().optional(),
  })
  .passthrough();

export const SoloWorktreeConfigRawSchema = z
  .object({
    setup: SoloLifecycleCommandRawSchema.optional(),
    teardown: SoloLifecycleCommandRawSchema.optional(),
    terminals: z.unknown().optional(),
  })
  .passthrough();

export const SoloConfigRawSchema = z
  .object({
    worktree: SoloWorktreeConfigRawSchema.optional(),
    scripts: z.record(z.string(), SoloScriptEntryRawSchema).optional(),
  })
  .passthrough();

export const WorktreeConfigSchema = SoloWorktreeConfigRawSchema.extend({
  setup: z.unknown().transform(normalizeLifecycleCommands),
  teardown: z.unknown().transform(normalizeLifecycleCommands),
})
  .passthrough()
  .catch({ setup: [], teardown: [] });

export const ScriptEntrySchema = SoloScriptEntryRawSchema.catch({});

export const SoloConfigSchema = SoloConfigRawSchema.extend({
  worktree: WorktreeConfigSchema.optional(),
  scripts: z.record(z.string(), ScriptEntrySchema).optional().catch({}),
})
  .passthrough()
  .catch({});

export const SoloConfigRevisionSchema = z.object({
  mtimeMs: z.number(),
  size: z.number(),
});

export const ProjectConfigRpcErrorSchema = z.discriminatedUnion("code", [
  z.object({ code: z.literal("project_not_found") }),
  z.object({ code: z.literal("invalid_project_config") }),
  z.object({
    code: z.literal("stale_project_config"),
    currentRevision: SoloConfigRevisionSchema.nullable(),
  }),
  z.object({ code: z.literal("write_failed") }),
]);

export type SoloScriptEntryRaw = z.infer<typeof SoloScriptEntryRawSchema>;
export type SoloConfigRaw = z.infer<typeof SoloConfigRawSchema>;
export type SoloConfig = z.infer<typeof SoloConfigSchema>;
export type SoloConfigRevision = z.infer<typeof SoloConfigRevisionSchema>;
export type ProjectConfigRpcError = z.infer<typeof ProjectConfigRpcErrorSchema>;
