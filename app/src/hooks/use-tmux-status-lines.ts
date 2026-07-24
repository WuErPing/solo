import { useQueries } from "@tanstack/react-query";
import { useMemo } from "react";
import { getHostRuntimeStore, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";
import { useAppVisible } from "@/hooks/use-app-visible";
import { computeAdaptiveQueryInterval } from "@/hooks/use-tmux-capture-pane";
import type { TmuxAgent } from "./use-tmux-agents";

export interface TmuxStatusLineInfo {
  sessionName: string;
  serverId: string;
  statusLeft: string;
  statusCenter: string;
  statusRight: string;
}

export function tmuxStatusLineQueryKey(serverId: string, sessionName: string): readonly string[] {
  return ["tmux-status-line", serverId, sessionName];
}

export function useTmuxStatusLines(agents: TmuxAgent[]): TmuxStatusLineInfo[] {
  const isAppVisible = useAppVisible();
  // Dedupe by (serverId, sessionName)
  const uniqueSessions = useMemo(() => {
    const seen = new Set<string>();
    const result: { serverId: string; sessionName: string }[] = [];
    for (const agent of agents) {
      const key = `${agent.serverId}:${agent.sessionName}`;
      if (!seen.has(key)) {
        seen.add(key);
        result.push({ serverId: agent.serverId, sessionName: agent.sessionName });
      }
    }
    return result;
  }, [agents]);

  const queries = useQueries({
    queries: uniqueSessions.map((session) => {
      const store = getHostRuntimeStore();
      const client = store.getClient(session.serverId);
      const snapshot = store.getSnapshot(session.serverId);
      const isConnected = isHostRuntimeConnected(snapshot);

      return {
        queryKey: tmuxStatusLineQueryKey(session.serverId, session.sessionName),
        enabled: Boolean(client && isConnected),
        staleTime: 10000,
        refetchInterval: isAppVisible
          ? (query: { state: { data: unknown } }) =>
              computeAdaptiveQueryInterval(query, Date.now())
          : false,
        queryFn: async () => {
          const payload = await withLiveTmuxClient(session.serverId, (c) =>
            c.tmuxStatusLine(session.sessionName),
          );
          return {
            sessionName: session.sessionName,
            serverId: session.serverId,
            statusLeft: payload.statusLeft ?? "",
            statusCenter: payload.statusCenter ?? "",
            statusRight: payload.statusRight ?? "",
          };
        },
      };
    }),
  });

  return useMemo(() => {
    const result: TmuxStatusLineInfo[] = [];
    for (const query of queries) {
      if (query.data) {
        result.push(query.data);
      }
    }
    return result;
  }, [queries]);
}
