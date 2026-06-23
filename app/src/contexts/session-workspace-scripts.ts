import equal from "fast-deep-equal";
import type { ScriptStatusUpdateMessage } from "@server/shared/messages";
import type { WorkspaceDescriptor } from "@/stores/session-store";
import { resolveWorkspaceMapKeyByIdentity } from "@/utils/workspace-execution";

function normalizeScript(
  s: ScriptStatusUpdateMessage["payload"]["scripts"][number],
): NonNullable<WorkspaceDescriptor["scripts"]>[number] {
  return {
    ...s,
    terminalId: s.terminalId ?? undefined,
    exitCode: s.exitCode ?? undefined,
    port: s.port ?? undefined,
    proxyUrl: s.proxyUrl ?? undefined,
    health: s.health ?? undefined,
  };
}

export function patchWorkspaceScripts(
  workspaces: Map<string, WorkspaceDescriptor>,
  update: ScriptStatusUpdateMessage["payload"],
): Map<string, WorkspaceDescriptor> {
  const workspaceKey = resolveWorkspaceMapKeyByIdentity({
    workspaces,
    workspaceId: update.workspaceId,
  });
  if (!workspaceKey) {
    return workspaces;
  }

  const existing = workspaces.get(workspaceKey);
  if (!existing) {
    return workspaces;
  }

  const normalized = update.scripts.map(normalizeScript);
  if (equal(existing.scripts, normalized)) {
    return workspaces;
  }

  const next = new Map(workspaces);
  next.set(workspaceKey, { ...existing, scripts: normalized });
  return next;
}
