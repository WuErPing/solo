import { describe, expect, it } from "vitest";
import { buildTemplateCwdItems } from "./use-loop-project-counts";
import type { LoopTemplateSummary } from "@server/server/loop/rpc-schemas";

function makeTemplate(cwd: string): LoopTemplateSummary {
  return {
    id: `template-${cwd}`,
    name: `Template ${cwd}`,
    cwd,
    instanceCount: 1,
  };
}

describe("buildTemplateCwdItems", () => {
  it("returns empty array for empty templates", () => {
    expect(buildTemplateCwdItems([])).toEqual([]);
  });

  it("maps template cwd and serverId", () => {
    const items = [{ template: makeTemplate("/repo"), serverId: "host1" }];
    const result = buildTemplateCwdItems(items);
    expect(result).toEqual([{ cwd: "/repo", serverId: "host1" }]);
  });

  it("maps multiple templates", () => {
    const items = [
      { template: makeTemplate("/repo1"), serverId: "host1" },
      { template: makeTemplate("/repo2"), serverId: "host2" },
    ];
    const result = buildTemplateCwdItems(items);
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({ cwd: "/repo1", serverId: "host1" });
    expect(result[1]).toEqual({ cwd: "/repo2", serverId: "host2" });
  });
});
