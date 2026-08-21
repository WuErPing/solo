import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const asyncStorageMock = vi.hoisted(() => ({
  getItem: vi.fn<(_: string) => Promise<string | null>>(),
  setItem: vi.fn<(_: string, __: string) => Promise<void>>(),
}));

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: asyncStorageMock,
}));

vi.mock("@/constants/platform", () => ({
  isWeb: true,
}));

const LAST_WORKSPACE_ROUTE_SELECTION_STORAGE_KEY = "solo:last-workspace-route-selection";
const LAST_ROUTE_STORAGE_KEY = "solo:last-route";

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function installWindowStub(pathname: string) {
  const windowStub = {
    location: {
      href: "",
      origin: "http://localhost",
      pathname: "",
      search: "",
      hash: "",
    },
    history: {
      pushState: vi.fn((_state: unknown, _title: string, url: string) => {
        updateLocation(url);
      }),
      replaceState: vi.fn((_state: unknown, _title: string, url: string) => {
        updateLocation(url);
      }),
    },
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  };

  function updateLocation(url: string) {
    const next = new URL(url, windowStub.location.origin);
    windowStub.location = {
      href: next.href,
      origin: next.origin,
      pathname: next.pathname,
      search: next.search,
      hash: next.hash,
    };
  }

  updateLocation(pathname);
  vi.stubGlobal("window", windowStub);
  return windowStub;
}

function createNavigationRef(serverId: string, workspaceId: string) {
  return {
    current: {
      getCurrentRoute: () => ({
        params: { serverId, workspaceId },
      }),
    },
  };
}

function createNavigationPathRef(path: string) {
  return {
    current: {
      getCurrentRoute: () => ({
        path,
      }),
    },
  };
}

function createNavigationPathWithParamsRef(path: string, serverId: string, workspaceId: string) {
  return {
    current: {
      getCurrentRoute: () => ({
        path,
        params: { serverId, workspaceId },
      }),
    },
  };
}

describe("navigation active workspace store", () => {
  beforeEach(() => {
    vi.resetModules();
    asyncStorageMock.getItem.mockReset();
    asyncStorageMock.setItem.mockReset();
    asyncStorageMock.getItem.mockResolvedValue(null);
    asyncStorageMock.setItem.mockResolvedValue();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps explicit web workspace activation ahead of a stale navigation route sync", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(createNavigationRef("server-1", "workspace-a"));
    expect(store.getNavigationActiveWorkspaceSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });

    store.activateNavigationWorkspaceSelection(
      { serverId: "server-1", workspaceId: "workspace-b" },
      { updateBrowserHistory: true, historyMode: "push" },
    );
    store.syncNavigationActiveWorkspace(createNavigationRef("server-1", "workspace-a"));

    expect(store.getNavigationActiveWorkspaceSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-b",
    });
  });

  it("clears a stale browser workspace when navigation sync reports a non-workspace route", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(createNavigationRef("server-1", "workspace-a"));
    expect(store.getNavigationActiveWorkspaceSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });
    expect(store.getLastNavigationWorkspaceRouteSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });

    store.syncNavigationActiveWorkspace(createNavigationPathRef("/h/server-1/sessions"));

    expect(store.getNavigationActiveWorkspaceSelection()).toBeNull();
    expect(store.getLastNavigationWorkspaceRouteSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });
  });

  it("clears stale workspace params when navigation sync reports a non-workspace path", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(createNavigationRef("server-1", "workspace-a"));
    expect(store.getNavigationActiveWorkspaceSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });

    store.syncNavigationActiveWorkspace(
      createNavigationPathWithParamsRef("/h/server-1/sessions", "server-1", "workspace-a"),
    );

    expect(store.getNavigationActiveWorkspaceSelection()).toBeNull();
  });

  it("uses a one-shot workspace route override when returning to a retained shell", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(
      createNavigationPathRef("/h/server-1/workspace/workspace-a"),
    );
    store.overrideNextNavigationWorkspaceRouteSelection({
      serverId: "server-1",
      workspaceId: "workspace-b",
    });

    store.syncNavigationActiveWorkspace(
      createNavigationPathRef("/h/server-1/workspace/workspace-a"),
    );

    expect(store.getNavigationActiveWorkspaceSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-b",
    });

    store.syncNavigationActiveWorkspace(
      createNavigationPathRef("/h/server-1/workspace/workspace-a"),
    );

    expect(store.getNavigationActiveWorkspaceSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });
  });

  it("hydrates the last workspace route selection from storage on startup", async () => {
    const storedSelection = createDeferred<string | null>();
    asyncStorageMock.getItem.mockReturnValue(storedSelection.promise);
    installWindowStub("/open-project");

    const store = await import("@/stores/navigation-active-workspace-store");

    expect(store.getIsLastNavigationWorkspaceRouteSelectionLoaded()).toBe(false);
    const hydration = store.hydrateLastNavigationWorkspaceRouteSelection();
    storedSelection.resolve(JSON.stringify({ serverId: "server-1", workspaceId: "workspace-a" }));
    await hydration;

    expect(asyncStorageMock.getItem).toHaveBeenCalledWith(
      LAST_WORKSPACE_ROUTE_SELECTION_STORAGE_KEY,
    );
    expect(store.getLastNavigationWorkspaceRouteSelection()).toEqual({
      serverId: "server-1",
      workspaceId: "workspace-a",
    });
    expect(store.getIsLastNavigationWorkspaceRouteSelectionLoaded()).toBe(true);
  });

  it("hydrates empty and corrupt workspace route storage as null", async () => {
    installWindowStub("/open-project");

    asyncStorageMock.getItem.mockResolvedValueOnce(null);
    let store = await import("@/stores/navigation-active-workspace-store");
    await store.hydrateLastNavigationWorkspaceRouteSelection();

    expect(store.getLastNavigationWorkspaceRouteSelection()).toBeNull();
    expect(store.getIsLastNavigationWorkspaceRouteSelectionLoaded()).toBe(true);

    vi.resetModules();
    asyncStorageMock.getItem.mockResolvedValueOnce("{not json");
    store = await import("@/stores/navigation-active-workspace-store");
    await store.hydrateLastNavigationWorkspaceRouteSelection();

    expect(store.getLastNavigationWorkspaceRouteSelection()).toBeNull();
    expect(store.getIsLastNavigationWorkspaceRouteSelectionLoaded()).toBe(true);
  });

  it("persists valid last workspace route selections", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");
    await store.hydrateLastNavigationWorkspaceRouteSelection();

    store.syncNavigationActiveWorkspace(createNavigationRef("server-1", "workspace-a"));

    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
      LAST_WORKSPACE_ROUTE_SELECTION_STORAGE_KEY,
      JSON.stringify({ serverId: "server-1", workspaceId: "workspace-a" }),
    );
  });
});

describe("navigation last route persistence", () => {
  beforeEach(() => {
    vi.resetModules();
    asyncStorageMock.getItem.mockReset();
    asyncStorageMock.setItem.mockReset();
    asyncStorageMock.getItem.mockResolvedValue(null);
    asyncStorageMock.setItem.mockResolvedValue();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("persists the workspace route path on navigation sync", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(
      createNavigationPathRef("/h/server-1/workspace/workspace-a"),
    );

    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
      LAST_ROUTE_STORAGE_KEY,
      "/h/server-1/workspace/workspace-a",
    );
    expect(store.getLastNavigationRoute()).toBe("/h/server-1/workspace/workspace-a");
  });

  it("persists any non-workspace route path on navigation sync", async () => {
    installWindowStub("/h/server-1/sessions");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(createNavigationPathRef("/h/server-1/sessions"));
    store.syncNavigationActiveWorkspace(createNavigationPathRef("/settings"));

    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
      LAST_ROUTE_STORAGE_KEY,
      "/h/server-1/sessions",
    );
    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(LAST_ROUTE_STORAGE_KEY, "/settings");
    expect(store.getLastNavigationRoute()).toBe("/settings");
  });

  it("does not persist transient routes", async () => {
    installWindowStub("/");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(createNavigationPathRef("/"));
    store.syncNavigationActiveWorkspace(createNavigationPathRef("/welcome"));
    store.syncNavigationActiveWorkspace(createNavigationPathRef("/pair-scan"));

    expect(asyncStorageMock.setItem).not.toHaveBeenCalledWith(LAST_ROUTE_STORAGE_KEY, expect.anything());
  });

  it("persists the retained workspace switch triggered without a router state change", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.activateNavigationWorkspaceSelection(
      { serverId: "server-1", workspaceId: "workspace-b" },
      { updateBrowserHistory: true, historyMode: "push" },
    );

    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
      LAST_ROUTE_STORAGE_KEY,
      "/h/server-1/workspace/workspace-b",
    );
    expect(store.getLastNavigationRoute()).toBe("/h/server-1/workspace/workspace-b");
  });

  it("persists the one-shot override selection instead of the stale route path", async () => {
    installWindowStub("/h/server-1/workspace/workspace-a");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.overrideNextNavigationWorkspaceRouteSelection({
      serverId: "server-1",
      workspaceId: "workspace-b",
    });
    store.syncNavigationActiveWorkspace(
      createNavigationPathRef("/h/server-1/workspace/workspace-a"),
    );

    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
      LAST_ROUTE_STORAGE_KEY,
      "/h/server-1/workspace/workspace-b",
    );
  });

  it("prefers the browser workspace url over the route path on web", async () => {
    installWindowStub("/h/server-1/workspace/workspace-b");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(
      createNavigationPathRef("/h/server-1/workspace/workspace-a"),
    );

    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(
      LAST_ROUTE_STORAGE_KEY,
      "/h/server-1/workspace/workspace-b",
    );
  });

  it("writes storage once for repeated syncs of the same route", async () => {
    installWindowStub("/h/server-1/sessions");
    const store = await import("@/stores/navigation-active-workspace-store");

    store.syncNavigationActiveWorkspace(createNavigationPathRef("/h/server-1/sessions"));
    store.syncNavigationActiveWorkspace(createNavigationPathRef("/h/server-1/sessions"));

    expect(asyncStorageMock.setItem).toHaveBeenCalledTimes(1);
  });

  it("hydrates the last route from storage on startup", async () => {
    asyncStorageMock.getItem.mockReset();
    // First eager hydration call reads the legacy workspace key; the second reads the last route.
    asyncStorageMock.getItem.mockResolvedValueOnce(null);
    asyncStorageMock.getItem.mockResolvedValueOnce("/h/server-1/dashboard");
    installWindowStub("/open-project");
    const store = await import("@/stores/navigation-active-workspace-store");

    await store.hydrateLastNavigationRoute();

    expect(asyncStorageMock.getItem).toHaveBeenCalledWith(LAST_ROUTE_STORAGE_KEY);
    expect(store.getLastNavigationRoute()).toBe("/h/server-1/dashboard");
  });

  it("hydrates empty, transient, and corrupt last route storage as null", async () => {
    installWindowStub("/open-project");

    for (const stored of [null, "not-a-route", "/", "/welcome"]) {
      vi.resetModules();
      asyncStorageMock.getItem.mockReset();
      asyncStorageMock.getItem.mockResolvedValueOnce(null);
      asyncStorageMock.getItem.mockResolvedValueOnce(stored);
      const store = await import("@/stores/navigation-active-workspace-store");
      await store.hydrateLastNavigationRoute();
      expect(store.getLastNavigationRoute()).toBeNull();
    }
  });
});
