import { describe, expect, it } from "vitest";
import {
  matchTmuxToProjects,
  matchesWorkingDir,
  type TmuxPaneSource,
  type ProjectPathSource,
} from "./tmux-project-matcher";

function pane(serverId: string, workingDir: string, kind: "agent" | "pane" = "pane"): TmuxPaneSource {
  return { serverId, workingDir, kind };
}

function project(
  projectKey: string,
  serverId: string,
  projectRootPath?: string,
  workspaceDirectory?: string,
): ProjectPathSource {
  return { projectKey, serverId, projectRootPath, workspaceDirectory };
}

describe("matchTmuxToProjects", () => {
  it("returns empty map for empty inputs", () => {
    expect(matchTmuxToProjects([], [])).toEqual(new Map());
  });

  it("returns empty map when no panes exist", () => {
    const result = matchTmuxToProjects(
      [],
      [project("p1", "host1", "/repo")],
    );
    expect(result).toEqual(new Map());
  });

  it("returns empty map when no projects exist", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/repo")],
      [],
    );
    expect(result).toEqual(new Map());
  });

  it("matches exact path", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/Users/me/project")],
      [project("p1", "host1", "/Users/me/project")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("matches child path with / boundary", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/Users/me/project/src")],
      [project("p1", "host1", "/Users/me/project")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("does not match partial directory name", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/Users/me/project-extended")],
      [project("p1", "host1", "/Users/me/project")],
    );
    expect(result.get("p1")).toBeUndefined();
  });

  it("does not match across different serverIds", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/repo")],
      [project("p1", "host2", "/repo")],
    );
    expect(result.get("p1")).toBeUndefined();
  });

  it("matches worktree path back to parent repo", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/Users/me/project/.solo/worktrees/feature-x")],
      [project("p1", "host1", "/Users/me/project")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("matches worktree path with nested directories", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/Users/me/project/.solo/worktrees/feature-x/src")],
      [project("p1", "host1", "/Users/me/project")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("matches via workspaceDirectory for worktrees", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/Users/me/project/.solo/worktrees/feat-x")],
      [project("p1", "host1", "/Users/me/project", "/Users/me/project/.solo/worktrees/feat-x")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("counts multiple panes for the same project", () => {
    const result = matchTmuxToProjects(
      [
        pane("host1", "/repo/src"),
        pane("host1", "/repo/tests"),
        pane("host1", "/repo"),
      ],
      [project("p1", "host1", "/repo")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 3, loopCount: 0, scheduleCount: 0 });
  });

  it("matches panes to different projects independently", () => {
    const result = matchTmuxToProjects(
      [
        pane("host1", "/repo-a"),
        pane("host1", "/repo-b/src"),
      ],
      [
        project("pA", "host1", "/repo-a"),
        project("pB", "host1", "/repo-b"),
      ],
    );
    expect(result.get("pA")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
    expect(result.get("pB")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("handles trailing slashes in paths", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/repo/")],
      [project("p1", "host1", "/repo")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 0, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("ignores projects with no rootPath or workspaceDirectory", () => {
    const result = matchTmuxToProjects(
      [pane("host1", "/repo")],
      [project("p1", "host1", undefined, undefined)],
    );
    expect(result.get("p1")).toBeUndefined();
  });

  it("distinguishes agent panes from regular panes", () => {
    const result = matchTmuxToProjects(
      [
        pane("host1", "/repo/src", "agent"),
        pane("host1", "/repo/tests", "agent"),
        pane("host1", "/repo", "pane"),
      ],
      [project("p1", "host1", "/repo")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 2, paneCount: 3, loopCount: 0, scheduleCount: 0 });
  });

  it("counts only agents when no regular panes match", () => {
    const result = matchTmuxToProjects(
      [
        pane("host1", "/repo", "agent"),
      ],
      [project("p1", "host1", "/repo")],
    );
    expect(result.get("p1")).toEqual({ agentCount: 1, paneCount: 1, loopCount: 0, scheduleCount: 0 });
  });

  it("does not double-count a pane when a project has multiple workspaces", () => {
    // A project with 2 workspaces (e.g. main checkout + worktree) both
    // share the same projectRootPath. buildProjectPathSources creates one
    // ProjectPathSource per workspace, so the pane matches both entries.
    // It must be counted only ONCE for the project.
    const result = matchTmuxToProjects(
      [
        pane("host1", "/repo/src", "agent"),
        pane("host1", "/repo/tests", "pane"),
      ],
      [
        project("p1", "host1", "/repo"),
        project("p1", "host1", "/repo"),
      ],
    );
    expect(result.get("p1")).toEqual({ agentCount: 1, paneCount: 2, loopCount: 0, scheduleCount: 0 });
  });
});

describe("matchesWorkingDir", () => {
  it("matches exact path", () => {
    expect(matchesWorkingDir("/repo", "/repo")).toBe(true);
  });

  it("matches child path", () => {
    expect(matchesWorkingDir("/repo/src", "/repo")).toBe(true);
  });

  it("does not match partial directory name", () => {
    expect(matchesWorkingDir("/repo-extended", "/repo")).toBe(false);
  });

  it("handles trailing slashes", () => {
    expect(matchesWorkingDir("/repo/", "/repo")).toBe(true);
  });

  it("matches worktree path back to parent", () => {
    expect(matchesWorkingDir("/repo/.solo/worktrees/feat-x", "/repo")).toBe(true);
  });

  it("returns false for empty inputs", () => {
    expect(matchesWorkingDir("", "/repo")).toBe(false);
    expect(matchesWorkingDir("/repo", "")).toBe(false);
  });
});
