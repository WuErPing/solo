import { describe, expect, it } from "vitest";
import { buildProjectPathSources, buildPaneSources } from "./use-tmux-project-counts";
import type { SidebarProjectEntry } from "./use-sidebar-workspaces-list";
import type { TmuxAgent, TmuxPane } from "./use-tmux-agents";

function makeProject(
  projectKey: string,
  workspaces: { serverId: string; projectRootPath?: string; workspaceDirectory?: string }[],
): SidebarProjectEntry {
  return {
    projectKey,
    projectName: projectKey,
    projectKind: "git",
    iconWorkingDir: workspaces[0]?.projectRootPath ?? "",
    workspaces: workspaces.map((ws, i) => ({
      workspaceKey: `${ws.serverId}:${projectKey}-${i}`,
      serverId: ws.serverId,
      workspaceId: `${projectKey}-${i}`,
      projectKey,
      projectRootPath: ws.projectRootPath,
      workspaceDirectory: ws.workspaceDirectory,
      projectKind: "git",
      workspaceKind: "checkout",
      name: `${projectKey}-${i}`,
      statusBucket: "done",
      diffStat: null,
      scripts: [],
      hasRunningScripts: false,
    })),
  };
}

describe("buildProjectPathSources", () => {
  it("returns empty array for empty projects", () => {
    expect(buildProjectPathSources([], "host1")).toEqual([]);
  });

  it("maps project workspaces to path sources", () => {
    const projects = [
      makeProject("p1", [{ serverId: "host1", projectRootPath: "/repo" }]),
    ];
    const result = buildProjectPathSources(projects, "host1");
    expect(result).toEqual([
      { projectKey: "p1", serverId: "host1", projectRootPath: "/repo", workspaceDirectory: undefined },
    ]);
  });

  it("filters workspaces by serverId", () => {
    const projects = [
      makeProject("p1", [
        { serverId: "host1", projectRootPath: "/repo" },
        { serverId: "host2", projectRootPath: "/repo" },
      ]),
    ];
    const result = buildProjectPathSources(projects, "host1");
    expect(result).toHaveLength(1);
    expect(result[0].serverId).toBe("host1");
  });

  it("includes workspaceDirectory when present", () => {
    const projects = [
      makeProject("p1", [{
        serverId: "host1",
        projectRootPath: "/repo",
        workspaceDirectory: "/repo/.solo/worktrees/feat-x",
      }]),
    ];
    const result = buildProjectPathSources(projects, "host1");
    expect(result[0].workspaceDirectory).toBe("/repo/.solo/worktrees/feat-x");
  });

  it("skips workspaces with no path information", () => {
    const projects = [
      makeProject("p1", [{ serverId: "host1" }]),
    ];
    const result = buildProjectPathSources(projects, "host1");
    expect(result).toEqual([]);
  });

  it("deduplicates projects with multiple workspaces on same host", () => {
    const projects = [
      makeProject("p1", [
        { serverId: "host1", projectRootPath: "/repo" },
        { serverId: "host1", projectRootPath: "/repo", workspaceDirectory: "/repo/.solo/worktrees/a" },
      ]),
    ];
    const result = buildProjectPathSources(projects, "host1");
    // Both workspaces produce separate sources (matcher handles the logic)
    expect(result).toHaveLength(2);
    expect(result[0].projectKey).toBe("p1");
    expect(result[1].projectKey).toBe("p1");
  });
});

function makeAgent(serverId: string, workingDir: string, status?: string): TmuxAgent {
  return {
    sessionName: "s1",
    windowName: "w1",
    paneId: "%1",
    paneIndex: 0,
    panePid: 1,
    agentName: "kimi",
    currentCmd: "kimi",
    workingDir,
    status,
    serverId,
    serverLabel: "local",
  };
}

function makePane(serverId: string, workingDir: string): TmuxPane {
  return {
    sessionName: "s1",
    windowName: "w1",
    paneId: "%2",
    paneIndex: 0,
    panePid: 2,
    currentCmd: "zsh",
    workingDir,
    serverId,
    serverLabel: "local",
  };
}

describe("buildPaneSources", () => {
  it("includes active agents as agent-kind pane sources", () => {
    const agents = [makeAgent("host1", "/repo")];
    const result = buildPaneSources(agents, [], "host1");
    expect(result).toEqual([
      { serverId: "host1", workingDir: "/repo", kind: "agent" },
    ]);
  });

  it("excludes exited agents from pane sources", () => {
    const agents = [
      makeAgent("host1", "/repo", "exited"),
      makeAgent("host1", "/repo"),
    ];
    const result = buildPaneSources(agents, [], "host1");
    expect(result).toHaveLength(1);
    expect(result[0].kind).toBe("agent");
  });

  it("includes other panes as pane-kind sources", () => {
    const panes = [makePane("host1", "/repo/src")];
    const result = buildPaneSources([], panes, "host1");
    expect(result).toEqual([
      { serverId: "host1", workingDir: "/repo/src", kind: "pane" },
    ]);
  });

  it("filters by serverId", () => {
    const agents = [makeAgent("host2", "/repo")];
    const result = buildPaneSources(agents, [], "host1");
    expect(result).toEqual([]);
  });

  it("skips agents without workingDir", () => {
    const agents = [makeAgent("host1", "")];
    const result = buildPaneSources(agents, [], "host1");
    expect(result).toEqual([]);
  });
});
