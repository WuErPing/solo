import { useMemo } from "react";
import { useQueries, useQueryClient } from "@tanstack/react-query";
import type { ScheduleSummary } from "@server/server/schedule/types";
import type { SidebarProjectEntry } from "./use-sidebar-workspaces-list";
import {
  getHostRuntimeStore,
  useHosts,
  isHostRuntimeConnected,
} from "@/runtime/host-runtime";
import { schedulesQueryKey } from "./use-schedules";
import {
  matchCwdToProjects,
  type CwdItem,
} from "@/utils/cwd-project-matcher";
import {
  buildProjectPathSources,
} from "./use-tmux-project-counts";

export interface AggregatedScheduleForCounts extends ScheduleSummary {
  serverId: string;
  serverLabel: string;
}

export function buildScheduleCwdItems(
  schedules: AggregatedScheduleForCounts[],
): CwdItem[] {
  const items: CwdItem[] = [];
  for (const schedule of schedules) {
    if (schedule.target.type !== "new-agent") continue;
    const cwd = schedule.target.config.cwd?.trim();
    if (!cwd) continue;
    items.push({ cwd, serverId: schedule.serverId });
  }
  return items;
}

export function useScheduleProjectCounts(
  projects: SidebarProjectEntry[],
  serverId: string | null,
): Map<string, number> {
  const hosts = useHosts();
  const queryClient = useQueryClient();

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
        queryKey: schedulesQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        staleTime: 30_000,
        queryFn: async () => {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          const payload = await client.scheduleList();
          return {
            schedules: payload.schedules ?? [],
            serverId: host.serverId,
            serverLabel: host.label,
          };
        },
      };
    }),
  });

  return useMemo(() => {
    if (!serverId || projects.length === 0) {
      return new Map<string, number>();
    }

    const allSchedules: AggregatedScheduleForCounts[] = [];
    for (let i = 0; i < queries.length; i++) {
      const query = queries[i];
      if (!query?.data?.schedules) continue;
      const host = connectedHosts[i];
      if (!host) continue;
      for (const schedule of query.data.schedules) {
        allSchedules.push({
          ...schedule,
          serverId: host.serverId,
          serverLabel: host.label,
        });
      }
    }

    if (allSchedules.length === 0) {
      return new Map<string, number>();
    }

    const cwdItems = buildScheduleCwdItems(allSchedules);
    if (cwdItems.length === 0) {
      return new Map<string, number>();
    }

    const projectSources = buildProjectPathSources(projects, serverId);
    return matchCwdToProjects(cwdItems, projectSources);
  }, [queries, connectedHosts, projects, serverId]);
}
