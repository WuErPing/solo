import { describe, expect, it } from "vitest";
import { buildLoopCwdItems } from "./use-loop-project-counts";
import type { LoopListItem } from "@server/server/loop/rpc-schemas";

function makeLoop(cwd: string, serverId: string): LoopListItem & { serverId: string } {
  return {
    id: `loop-${Math.random().toString(36).slice(2)}`,
    name: null,
    status: "running",
    cwd,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    activeIteration: null,
    serverId,
  };
}

describe("buildLoopCwdItems", () => {
  it("returns empty array for empty loops", () => {
    expect(buildLoopCwdItems([])).toEqual([]);
  });

  it("maps loop cwd and serverId", () => {
    const loops = [makeLoop("/repo", "host1")];
    const result = buildLoopCwdItems(loops);
    expect(result).toEqual([{ cwd: "/repo", serverId: "host1" }]);
  });

  it("maps multiple loops", () => {
    const loops = [makeLoop("/repo1", "host1"), makeLoop("/repo2", "host2")];
    const result = buildLoopCwdItems(loops);
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({ cwd: "/repo1", serverId: "host1" });
    expect(result[1]).toEqual({ cwd: "/repo2", serverId: "host2" });
  });
});
