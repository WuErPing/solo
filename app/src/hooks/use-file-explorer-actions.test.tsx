/**
 * @vitest-environment jsdom
 */
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: {
    getItem: vi.fn(async () => null),
    setItem: vi.fn(async () => undefined),
    removeItem: vi.fn(async () => undefined),
  },
}));

import { useSessionStore } from "@/stores/session-store";
import { useFileExplorerActions } from "./use-file-explorer-actions";

const SERVER_ID = "server-1";

describe("useFileExplorerActions", () => {
  beforeEach(() => {
    useSessionStore.setState({ sessions: {} });
  });

  it("returns false and preserves the error when a directory cannot be listed", async () => {
    const error = "open /repo/.VSCodeCounter/old: no such file or directory";
    const exploreFileSystem = vi.fn(async () => ({ error }));
    useSessionStore.getState().initializeSession(SERVER_ID, { exploreFileSystem } as never);
    const { result } = renderHook(() =>
      useFileExplorerActions({
        serverId: SERVER_ID,
        workspaceId: "workspace-1",
        workspaceRoot: "/repo",
      }),
    );

    let succeeded = true;
    await act(async () => {
      succeeded = await result.current.requestDirectoryListing(".VSCodeCounter/old");
    });

    expect(succeeded).toBe(false);
    const explorer = useSessionStore
      .getState()
      .sessions[SERVER_ID]?.fileExplorer.get("workspace:workspace-1");
    expect(explorer?.lastError).toBe(error);
  });
});
