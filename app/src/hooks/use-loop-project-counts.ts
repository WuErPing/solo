import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import type { LoopTemplateSummary } from "@server/server/loop/rpc-schemas";
import type { SidebarProjectEntry } from "./use-sidebar-workspaces-list";
import {
  getHostRuntimeStore,
  useHosts,
  isHostRuntimeConnected,
} from "@/runtime/host-runtime";
import { loopTemplatesQueryKey } from "./use-loop-templates";
import {
  matchCwdToProjects,
  type CwdItem,
} from "@/utils/cwd-project-matcher";
import {
  buildProjectPathSources,
} from "./use-tmux-project-counts";

interface AggregatedTemplate {
  template: LoopTemplateSummary;
  serverId: string;
}

function buildTemplateCwdItems(items: AggregatedTemplate[]): CwdItem[] {
  return items.map(({ template, serverId }) => ({ cwd: template.cwd, serverId }));
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
        queryKey: loopTemplatesQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        staleTime: 30_000,
        queryFn: async () => {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          const payload = await client.loopTemplateList();
          return {
            templates: payload.templates ?? [],
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

    const allTemplates: AggregatedTemplate[] = [];
    for (let i = 0; i < queries.length; i++) {
      const query = queries[i];
      if (!query?.data?.templates) continue;
      const host = connectedHosts[i];
      if (!host) continue;
      for (const template of query.data.templates) {
        allTemplates.push({ template, serverId: host.serverId });
      }
    }

    if (allTemplates.length === 0) {
      return new Map<string, number>();
    }

    const cwdItems = buildTemplateCwdItems(allTemplates);
    const projectSources = buildProjectPathSources(projects, serverId);
    return matchCwdToProjects(cwdItems, projectSources);
  }, [queries, connectedHosts, projects, serverId]);
}
