import { describe, expect, it, vi } from "vitest";
import { buildAbsoluteExplorerPath, restorePersistedDirectoryPaths } from "./explorer-paths";

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

describe("restorePersistedDirectoryPaths", () => {
  it("prunes a deleted restored directory and reloads the project root", async () => {
    const requestDirectoryListing = vi.fn(async (path: string) => path !== ".VSCodeCounter/old");

    const restored = await restorePersistedDirectoryPaths({
      persistedPaths: [".", ".VSCodeCounter/old", "src"],
      requestDirectoryListing,
    });

    expect(restored).toEqual([".", "src"]);
    expect(requestDirectoryListing).toHaveBeenCalledWith(".", {
      recordHistory: false,
      setCurrentPath: false,
    });
  });

  it("keeps valid restored directories without an extra root request", async () => {
    const requestDirectoryListing = vi.fn(async () => true);

    const restored = await restorePersistedDirectoryPaths({
      persistedPaths: [".", "src"],
      requestDirectoryListing,
    });

    expect(restored).toEqual([".", "src"]);
    expect(requestDirectoryListing).toHaveBeenCalledTimes(1);
    expect(requestDirectoryListing).toHaveBeenCalledWith("src", {
      recordHistory: false,
      setCurrentPath: false,
    });
  });
});
