import { useMemo } from "react";
import { useAggregatedTmuxAgents, type TmuxAgent, type TmuxPane } from "./use-tmux-agents";
import type { SidebarProjectEntry } from "./use-sidebar-workspaces-list";
import {
  matchTmuxToProjects,
  type ProjectPaneCounts,
  type ProjectPathSource,
  type TmuxPaneSource,
} from "@/utils/tmux-project-matcher";

export function buildProjectPathSources(
  projects: SidebarProjectEntry[],
  serverId: string,
): ProjectPathSource[] {
  const sources: ProjectPathSource[] = [];
  for (const project of projects) {
    for (const ws of project.workspaces) {
      if (ws.serverId !== serverId) continue;
      if (!ws.projectRootPath && !ws.workspaceDirectory) continue;
      sources.push({
        projectKey: project.projectKey,
        serverId: ws.serverId,
        projectRootPath: ws.projectRootPath,
        workspaceDirectory: ws.workspaceDirectory,
      });
    }
  }
  return sources;
}

export function buildPaneSources(
  agents: TmuxAgent[],
  otherPanes: TmuxPane[],
  serverId: string,
): TmuxPaneSource[] {
  const paneSources: TmuxPaneSource[] = [];
  for (const agent of agents) {
    if (agent.serverId === serverId && agent.workingDir && agent.status !== "exited") {
      paneSources.push({ serverId: agent.serverId, workingDir: agent.workingDir, kind: "agent" });
    }
  }
  for (const pane of otherPanes) {
    if (pane.serverId === serverId && pane.workingDir) {
      paneSources.push({ serverId: pane.serverId, workingDir: pane.workingDir, kind: "pane" });
    }
  }
  return paneSources;
}

export function useTmuxProjectCounts(
  projects: SidebarProjectEntry[],
  serverId: string | null,
): Map<string, ProjectPaneCounts> {
  const { agents, otherPanes } = useAggregatedTmuxAgents();

  return useMemo(() => {
    if (!serverId || projects.length === 0) {
      return new Map<string, ProjectPaneCounts>();
    }

    const paneSources = buildPaneSources(agents, otherPanes, serverId);

    if (paneSources.length === 0) {
      return new Map<string, ProjectPaneCounts>();
    }

    const projectSources = buildProjectPathSources(projects, serverId);
    return matchTmuxToProjects(paneSources, projectSources);
  }, [agents, otherPanes, projects, serverId]);
}
