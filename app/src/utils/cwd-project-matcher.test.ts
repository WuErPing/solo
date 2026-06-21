import { describe, expect, it } from "vitest";
import { matchCwdToProjects, type CwdItem } from "./cwd-project-matcher";
import type { ProjectPathSource } from "./tmux-project-matcher";

function project(
  projectKey: string,
  opts: { serverId?: string; root?: string; wsDir?: string } = {},
): ProjectPathSource {
  return {
    projectKey,
    serverId: opts.serverId ?? "host1",
    projectRootPath: opts.root,
    workspaceDirectory: opts.wsDir,
  };
}

function item(cwd: string, serverId = "host1"): CwdItem {
  return { cwd, serverId };
}

describe("matchCwdToProjects", () => {
  it("returns empty map for empty items", () => {
    const result = matchCwdToProjects([], [project("p1", { root: "/repo" })]);
    expect(result.size).toBe(0);
  });

  it("returns empty map for empty projects", () => {
    const result = matchCwdToProjects([item("/repo/src")], []);
    expect(result.size).toBe(0);
  });

  it("matches item cwd against project root path", () => {
    const result = matchCwdToProjects(
      [item("/repo/src")],
      [project("p1", { root: "/repo" })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("matches item cwd exactly equal to project root", () => {
    const result = matchCwdToProjects(
      [item("/repo")],
      [project("p1", { root: "/repo" })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("matches item cwd against workspace directory", () => {
    const result = matchCwdToProjects(
      [item("/repo/.solo/worktrees/feat-x/src")],
      [project("p1", { wsDir: "/repo/.solo/worktrees/feat-x" })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("matches worktree child path via backtracking", () => {
    const result = matchCwdToProjects(
      [item("/repo/.solo/worktrees/feat-x/src")],
      [project("p1", { root: "/repo" })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("does not match unrelated path", () => {
    const result = matchCwdToProjects(
      [item("/other/path")],
      [project("p1", { root: "/repo" })],
    );
    expect(result.has("p1")).toBe(false);
  });

  it("does not match item on different server", () => {
    const result = matchCwdToProjects(
      [item("/repo/src", "host2")],
      [project("p1", { serverId: "host1", root: "/repo" })],
    );
    expect(result.has("p1")).toBe(false);
  });

  it("counts multiple items matching the same project", () => {
    const result = matchCwdToProjects(
      [item("/repo/a"), item("/repo/b")],
      [project("p1", { root: "/repo" })],
    );
    expect(result.get("p1")).toBe(2);
  });

  it("matches multiple projects independently", () => {
    const result = matchCwdToProjects(
      [item("/repo1/src"), item("/repo2/src")],
      [
        project("p1", { root: "/repo1" }),
        project("p2", { root: "/repo2" }),
      ],
    );
    expect(result.get("p1")).toBe(1);
    expect(result.get("p2")).toBe(1);
  });

  it("handles trailing slashes in paths", () => {
    const result = matchCwdToProjects(
      [item("/repo/src/")],
      [project("p1", { root: "/repo/" })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("does not match partial directory name", () => {
    const result = matchCwdToProjects(
      [item("/repo-extra/src")],
      [project("p1", { root: "/repo" })],
    );
    expect(result.has("p1")).toBe(false);
  });

  it("expands ~ in cwd to match absolute project path", () => {
    const home = process.env.HOME ?? process.env.USERPROFILE;
    if (!home) return; // skip if no home dir available
    const result = matchCwdToProjects(
      [item("~/code/project")],
      [project("p1", { root: `${home}/code/project` })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("expands ~ in cwd for child path match", () => {
    const home = process.env.HOME ?? process.env.USERPROFILE;
    if (!home) return;
    const result = matchCwdToProjects(
      [item("~/code/project/src")],
      [project("p1", { root: `${home}/code/project` })],
    );
    expect(result.get("p1")).toBe(1);
  });

  it("infers home from project paths when process.env.HOME is unavailable", () => {
    const saved = process.env.HOME;
    const savedUp = process.env.USERPROFILE;
    delete process.env.HOME;
    delete process.env.USERPROFILE;
    try {
      const result = matchCwdToProjects(
        [item("~/code/wuerping/solo")],
        [project("p1", { root: "/Users/wuerping/code/wuerping/solo" })],
      );
      expect(result.get("p1")).toBe(1);
    } finally {
      if (saved !== undefined) process.env.HOME = saved;
      if (savedUp !== undefined) process.env.USERPROFILE = savedUp;
    }
  });

  it("infers home from project paths for child path match without HOME", () => {
    const saved = process.env.HOME;
    const savedUp = process.env.USERPROFILE;
    delete process.env.HOME;
    delete process.env.USERPROFILE;
    try {
      const result = matchCwdToProjects(
        [item("~/code/wuerping/solo/src")],
        [project("p1", { root: "/Users/wuerping/code/wuerping/solo" })],
      );
      expect(result.get("p1")).toBe(1);
    } finally {
      if (saved !== undefined) process.env.HOME = saved;
      if (savedUp !== undefined) process.env.USERPROFILE = savedUp;
    }
  });
});
