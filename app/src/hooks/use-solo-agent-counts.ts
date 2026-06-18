import { useMemo } from "react";
import { useSessionStore, type Agent } from "@/stores/session-store";
import { deriveProjectKey } from "@/utils/agent-grouping";

interface SoloAgentEntry {
  serverId: string;
  cwd: string;
  projectKey?: string | null;
  archivedAt?: Date | null;
}

export function countSoloAgentsByProject(
  agents: SoloAgentEntry[],
  serverId: string,
): Map<string, number> {
  const counts = new Map<string, number>();
  for (const agent of agents) {
    if (agent.serverId !== serverId) continue;
    if (agent.archivedAt) continue;
    const key = agent.projectKey || deriveProjectKey(agent.cwd);
    if (!key) continue;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return counts;
}

export function useSoloAgentCounts(
  serverId: string | null,
): Map<string, number> {
  const agentsMap = useSessionStore((state) =>
    serverId ? (state.sessions[serverId]?.agents ?? null) : null,
  );

  return useMemo(() => {
    if (!serverId || !agentsMap) return new Map<string, number>();
    const entries: SoloAgentEntry[] = Array.from(agentsMap.values()).map((a: Agent) => ({
      serverId: a.serverId,
      cwd: a.cwd,
      projectKey: a.projectPlacement?.projectKey ?? null,
      archivedAt: a.archivedAt ?? null,
    }));
    return countSoloAgentsByProject(entries, serverId);
  }, [agentsMap, serverId]);
}
