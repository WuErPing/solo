import { describe, expect, it } from "vitest";
import { buildAbsoluteExplorerPath } from "./explorer-paths";

describe("buildAbsoluteExplorerPath", () => {
  it("builds a POSIX absolute path from a relative explorer path", () => {
    expect(
      buildAbsoluteExplorerPath({
        workspaceRoot: "/Users/moboudra/dev/solo",
        entryPath: "packages/app/src/components/file-explorer-pane.tsx",
      }),
    ).toBe("/Users/moboudra/dev/solo/packages/app/src/components/file-explorer-pane.tsx");
  });

  it("returns workspace root when entry path points to explorer root", () => {
    expect(
      buildAbsoluteExplorerPath({
        workspaceRoot: "/Users/moboudra/dev/solo",
        entryPath: ".",
      }),
    ).toBe("/Users/moboudra/dev/solo");
  });

  it("trims trailing separators from workspace root before joining", () => {
    expect(
      buildAbsoluteExplorerPath({
        workspaceRoot: "/Users/moboudra/dev/solo/",
        entryPath: "README.md",
      }),
    ).toBe("/Users/moboudra/dev/solo/README.md");
  });

  it("builds a Windows absolute path with backslash separators", () => {
    expect(
      buildAbsoluteExplorerPath({
        workspaceRoot: "C:\\repo\\solo",
        entryPath: "packages/app/src/components/file-explorer-pane.tsx",
      }),
    ).toBe("C:\\repo\\solo\\packages\\app\\src\\components\\file-explorer-pane.tsx");
  });

  it("passes through an already-absolute entry path", () => {
    expect(
      buildAbsoluteExplorerPath({
        workspaceRoot: "/Users/moboudra/dev/solo",
        entryPath: "/tmp/another/location.txt",
      }),
    ).toBe("/tmp/another/location.txt");
  });
});
