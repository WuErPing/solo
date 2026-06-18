import { describe, expect, it } from "vitest";
import { countSoloAgentsByProject } from "./use-solo-agent-counts";

interface TestAgent {
  serverId: string;
  cwd: string;
  projectKey?: string | null;
  archivedAt?: Date | null;
}

function makeAgent(serverId: string, cwd: string, projectKey?: string | null): TestAgent {
  return { serverId, cwd, projectKey };
}

describe("countSoloAgentsByProject", () => {
  it("returns empty map for no agents", () => {
    expect(countSoloAgentsByProject([], "host1")).toEqual(new Map());
  });

  it("counts agent by projectPlacement projectKey", () => {
    const agents = [
      makeAgent("host1", "/repo", "/repo"),
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/repo")).toBe(1);
  });

  it("derives projectKey from cwd when projectPlacement is missing", () => {
    const agents = [
      makeAgent("host1", "/Users/me/solo"),
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/Users/me/solo")).toBe(1);
  });

  it("derives projectKey from worktree cwd (strips .solo/worktrees/)", () => {
    const agents = [
      makeAgent("host1", "/Users/me/solo/.solo/worktrees/feat-x"),
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/Users/me/solo")).toBe(1);
  });

  it("counts multiple agents in the same project", () => {
    const agents = [
      makeAgent("host1", "/repo", "/repo"),
      makeAgent("host1", "/repo/src", "/repo"),
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/repo")).toBe(2);
  });

  it("filters by serverId", () => {
    const agents = [
      makeAgent("host1", "/repo", "/repo"),
      makeAgent("host2", "/repo", "/repo"),
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/repo")).toBe(1);
  });

  it("excludes archived agents", () => {
    const agents = [
      makeAgent("host1", "/repo", "/repo"),
      { ...makeAgent("host1", "/repo", "/repo"), archivedAt: new Date() },
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/repo")).toBe(1);
  });

  it("handles agents across different projects", () => {
    const agents = [
      makeAgent("host1", "/repo-a", "/repo-a"),
      makeAgent("host1", "/repo-b", "/repo-b"),
      makeAgent("host1", "/repo-a/src", "/repo-a"),
    ];
    const result = countSoloAgentsByProject(agents, "host1");
    expect(result.get("/repo-a")).toBe(2);
    expect(result.get("/repo-b")).toBe(1);
  });
});
