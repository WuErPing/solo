/**
 * @vitest-environment jsdom
 */
import React from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useTmuxTheme } from "./use-tmux-theme";

const { mockStore, mockClient } = vi.hoisted(() => {
  const store = {
    getClient: vi.fn(),
    getSnapshot: vi.fn(),
  };
  const client = {
    tmuxGetTheme: vi.fn(),
    getConnectionState: vi.fn(),
  };
  return {
    mockStore: store,
    mockClient: client,
  };
});

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => mockStore,
  isHostRuntimeConnected: () => true,
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderThemeHook(opts?: { serverId?: string; sessionId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useTmuxTheme(
        opts?.serverId ?? "server-1",
        opts?.sessionId ?? "my-session",
        opts?.enabled ?? true,
      ),
    { wrapper },
  );
}

beforeEach(() => {
  mockStore.getClient.mockReturnValue(mockClient);
  mockStore.getSnapshot.mockReturnValue({ connectionStatus: "online" });
  mockClient.getConnectionState.mockReturnValue({ status: "connected" });
  mockClient.tmuxGetTheme.mockResolvedValue({
    theme: {
      background: "#181825",
      foreground: "#cdd6f4",
      statusBackground: "#181825",
      statusForeground: "#cdd6f4",
      paneActiveBorder: "#89b4fa",
    },
    error: null,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTmuxTheme", () => {
  it("fetches theme colors from the daemon", async () => {
    const { result } = renderThemeHook();

    await waitFor(() => {
      expect(result.current.theme).not.toBeNull();
    });

    expect(result.current.theme?.background).toBe("#181825");
    expect(result.current.theme?.foreground).toBe("#cdd6f4");
    expect(result.current.theme?.paneActiveBorder).toBe("#89b4fa");
    expect(mockClient.tmuxGetTheme).toHaveBeenCalledWith("my-session");
  });

  it("returns null theme when sessionId is empty", () => {
    const { result } = renderThemeHook({ sessionId: "" });

    expect(result.current.theme).toBeNull();
    expect(mockClient.tmuxGetTheme).not.toHaveBeenCalled();
  });

  it("returns null theme when disabled", () => {
    const { result } = renderThemeHook({ enabled: false });

    expect(result.current.theme).toBeNull();
    expect(mockClient.tmuxGetTheme).not.toHaveBeenCalled();
  });

  it("returns error from daemon", async () => {
    mockClient.tmuxGetTheme.mockResolvedValue({
      theme: { background: "", foreground: "" },
      error: "session not found",
    });

    const { result } = renderThemeHook();

    await waitFor(() => {
      expect(result.current.error).toBe("session not found");
    });
  });

  it("handles client disposal gracefully", async () => {
    const { result } = renderThemeHook();

    await waitFor(() => {
      expect(result.current.theme).not.toBeNull();
    });

    mockClient.getConnectionState.mockReturnValue({ status: "disposed" });

    expect(result.current.theme?.background).toBe("#181825");
  });
});
