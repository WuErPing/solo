/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act } from "@testing-library/react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { HostRuntimeBootstrapState } from "./_layout";

const { redirectMock, runtimeStoreAccessMock, workspaceStoreAccessMock, state } = vi.hoisted(() => {
  const hoistedState = {
    pathname: "/",
    storeReady: true,
    bootstrapState: {
      phase: "online",
      error: null,
      retry: vi.fn(),
      startupNavigation: null,
    } as HostRuntimeBootstrapState,
  };

  return {
    redirectMock: vi.fn(),
    runtimeStoreAccessMock: vi.fn(() => {
      throw new Error("index.tsx must not read the host runtime store during startup navigation");
    }),
    workspaceStoreAccessMock: vi.fn(() => {
      throw new Error(
        "index.tsx must not read the navigation workspace store during startup navigation",
      );
    }),
    state: hoistedState,
  };
});

vi.mock("expo-router", () => ({
  Redirect: ({ href }: { href: string }) => {
    redirectMock(href);
    return React.createElement("div", { "data-testid": "redirect", "data-href": href });
  },
  usePathname: () => state.pathname,
}));

vi.mock("@/app/_layout", () => ({
  useHostRuntimeBootstrapState: () => state.bootstrapState,
  useStoreReady: () => state.storeReady,
}));

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: runtimeStoreAccessMock,
}));

vi.mock("@/desktop/daemon/desktop-daemon", () => ({
  shouldUseDesktopDaemon: () => false,
}));

vi.mock("@/screens/startup-splash-screen", () => ({
  StartupSplashScreen: () => React.createElement("div", { "data-testid": "startup-splash" }),
}));

vi.mock("@/stores/navigation-active-workspace-store", () => ({
  getLastNavigationRoute: workspaceStoreAccessMock,
  hydrateLastNavigationRoute: workspaceStoreAccessMock,
}));

describe("Index route startup navigation", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.resetModules();
    state.pathname = "/";
    state.storeReady = true;
    state.bootstrapState = {
      phase: "online",
      error: null,
      retry: vi.fn(),
      startupNavigation: null,
    };
    redirectMock.mockReset();
    runtimeStoreAccessMock.mockClear();
    workspaceStoreAccessMock.mockClear();

    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
  });

  async function renderIndex() {
    const { default: Index } = await import("./index");
    await act(async () => {
      root.render(<Index />);
    });
  }

  it("shows the startup splash until bootstrap has resolved the startup navigation target", async () => {
    await renderIndex();

    expect(container.querySelector("[data-testid='startup-splash']")).not.toBeNull();
    expect(redirectMock).not.toHaveBeenCalled();
    expect(runtimeStoreAccessMock).not.toHaveBeenCalled();
    expect(workspaceStoreAccessMock).not.toHaveBeenCalled();
  });

  it("restores the exact last route when its host matches the startup target", async () => {
    state.bootstrapState.startupNavigation = {
      target: { serverId: "server-1" },
      lastRoute: "/h/server-1/workspace/workspace-a",
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/h/server-1/workspace/workspace-a");
    expect(runtimeStoreAccessMock).not.toHaveBeenCalled();
    expect(workspaceStoreAccessMock).not.toHaveBeenCalled();
  });

  it("restores a non-workspace last route exactly", async () => {
    state.bootstrapState.startupNavigation = {
      target: { serverId: "server-1" },
      lastRoute: "/h/server-1/sessions",
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/h/server-1/sessions");
  });

  it("restores a global last route regardless of the startup host", async () => {
    state.bootstrapState.startupNavigation = {
      target: { serverId: "server-2" },
      lastRoute: "/settings",
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/settings");
  });

  it("navigates to the startup host root when the last route is another host's workspace", async () => {
    state.bootstrapState.startupNavigation = {
      target: { serverId: "server-2" },
      lastRoute: "/h/server-1/workspace/workspace-a",
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/h/server-2");
  });

  it("remaps a non-workspace host route onto the startup host", async () => {
    state.bootstrapState.startupNavigation = {
      target: { serverId: "server-2" },
      lastRoute: "/h/server-1/sessions",
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/h/server-2/sessions");
  });

  it("navigates to the startup host root when no last route is persisted", async () => {
    state.bootstrapState.startupNavigation = {
      target: { serverId: "server-2" },
      lastRoute: null,
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/h/server-2");
  });

  it("falls back to welcome when bootstrap resolves no startup target", async () => {
    state.bootstrapState.startupNavigation = {
      target: null,
      lastRoute: null,
    };

    await renderIndex();

    expect(redirectMock).toHaveBeenCalledWith("/welcome");
  });
});
