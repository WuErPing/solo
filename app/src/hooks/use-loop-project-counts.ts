import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import type { LoopListItem } from "@server/server/loop/rpc-schemas";
import type { SidebarProjectEntry } from "./use-sidebar-workspaces-list";
import {
  getHostRuntimeStore,
  useHosts,
  isHostRuntimeConnected,
} from "@/runtime/host-runtime";
import { loopsQueryKey } from "./use-loops";
import {
  matchCwdToProjects,
  type CwdItem,
} from "@/utils/cwd-project-matcher";
import {
  buildProjectPathSources,
} from "./use-tmux-project-counts";

export interface AggregatedLoop extends LoopListItem {
  serverId: string;
}

export function buildLoopCwdItems(loops: AggregatedLoop[]): CwdItem[] {
  return loops.map((loop) => ({ cwd: loop.cwd, serverId: loop.serverId }));
}

export function useLoopProjectCounts(
  projects: SidebarProjectEntry[],
  serverId: string | null,
): Map<string, number> {
  const hosts = useHosts();

  const connectedHosts = useMemo(() => {
    const store = getHostRuntimeStore();
    return hosts.filter((host) => {
      const snapshot = store.getSnapshot(host.serverId);
      return isHostRuntimeConnected(snapshot);
    });
  }, [hosts]);

  const queries = useQueries({
    queries: connectedHosts.map((host) => {
      const store = getHostRuntimeStore();
      const client = store.getClient(host.serverId);
      const snapshot = store.getSnapshot(host.serverId);
      const isConnected = isHostRuntimeConnected(snapshot);

      return {
        queryKey: loopsQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        staleTime: 30_000,
        queryFn: async () => {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          const payload = await client.loopList();
          return {
            loops: payload.loops ?? [],
            serverId: host.serverId,
          };
        },
      };
    }),
  });

  return useMemo(() => {
    if (!serverId || projects.length === 0) {
      return new Map<string, number>();
    }

    const allLoops: AggregatedLoop[] = [];
    for (let i = 0; i < queries.length; i++) {
      const query = queries[i];
      if (!query?.data?.loops) continue;
      const host = connectedHosts[i];
      if (!host) continue;
      for (const loop of query.data.loops) {
        allLoops.push({ ...loop, serverId: host.serverId });
      }
    }

    if (allLoops.length === 0) {
      return new Map<string, number>();
    }

    const cwdItems = buildLoopCwdItems(allLoops);
    const projectSources = buildProjectPathSources(projects, serverId);
    return matchCwdToProjects(cwdItems, projectSources);
  }, [queries, connectedHosts, projects, serverId]);
}
