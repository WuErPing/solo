/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HostTraceCard } from "@/screens/settings/host-trace-card";
import type { HostProfile } from "@/types/host-connection";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

interface MockDaemonClient {
  versions: {
    listDaemonVersions: ReturnType<typeof vi.fn>;
  };
}

const { state } = vi.hoisted(() => ({
  state: {
    connected: true,
    client: null as MockDaemonClient | null,
    runtimeSnapshot: null as Record<string, unknown> | null,
    sessions: {} as Record<string, { serverInfo: { version: string | null } | null } | undefined>,
    cachedSnapshot: null as Record<string, unknown> | null,
    saveSnapshot: vi.fn(),
  },
}));

vi.mock("@/stores/host-version-snapshots-store", () => ({
  saveHostVersionSnapshot: (...args: unknown[]) => state.saveSnapshot(...args),
  useHostVersionSnapshot: () => state.cachedSnapshot,
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeSnapshot: () => state.runtimeSnapshot,
  useHostRuntimeClient: () => state.client,
  useHostRuntimeIsConnected: () => state.connected,
}));

vi.mock("@/stores/session-store", () => ({
  useSessionStore: (selector: (value: { sessions: typeof state.sessions }) => unknown) =>
    selector({ sessions: state.sessions }),
}));

vi.mock("react-native", () => ({
  Platform: { OS: "web" },
  Pressable: ({
    children,
    onPress,
    testID,
  }: React.PropsWithChildren<{ onPress?: () => void; testID?: string }>) =>
    React.createElement("button", { type: "button", onClick: onPress, "data-testid": testID }, children),
  View: ({
    children,
    testID,
  }: React.PropsWithChildren<{ testID?: string }>) =>
    React.createElement("div", { "data-testid": testID }, children),
  Text: ({ children, testID }: React.PropsWithChildren<{ testID?: string }>) =>
    React.createElement("span", { "data-testid": testID }, children),
}));

vi.mock("react-native-unistyles", () => {
  const theme = {
    colors: {
      foreground: "#fff",
      foregroundMuted: "#999",
      border: "#333",
      palette: {
        green: { 500: "#22c55e" },
        amber: { 500: "#f59e0b" },
        red: { 300: "#fca5a5", 500: "#ef4444" },
      },
    },
    iconSize: { sm: 16 },
    spacing: { 1: 4, 2: 8, 3: 12 } as Record<number, number>,
    borderRadius: { full: 999 },
    fontSize: { xs: 11, sm: 14 },
    fontWeight: { normal: "normal", medium: "500" },
  };
  return {
    useUnistyles: () => ({ theme }),
    StyleSheet: {
      create: (input: unknown) => (typeof input === "function" ? input(theme) : input),
    },
  };
});

vi.mock("lucide-react-native", () => ({
  ChevronDown: () => React.createElement("span", { "data-icon": "ChevronDown" }),
  RefreshCw: () => React.createElement("span", { "data-icon": "RefreshCw" }),
}));

vi.mock("@/styles/settings", () => ({
  settingsStyles: {
    card: {},
  },
}));

const HOST: HostProfile = {
  serverId: "srv-1",
  label: "Test Host",
  lifecycle: {},
  connections: [
    {
      id: "conn-1",
      type: "relay",
      relayEndpoint: "solo.up2ai.top:443",
      daemonPublicKeyB64: "dGVzdC1rZXk=",
    },
  ],
  preferredConnectionId: "conn-1",
  createdAt: "2026-08-01T00:00:00.000Z",
  updatedAt: "2026-08-01T00:00:00.000Z",
};

const CACHED_SNAPSHOT = {
  versions: [
    { version: "solo-v0.12.0", mtimeMs: 200 },
    { version: "solo-v0.11.0", mtimeMs: 100 },
  ],
  currentVersion: "solo-v0.12.0",
  runningVersion: "v0.12.0",
  supervisor: {
    state: "fallback",
    pid: 9,
    spawnedBinary: "solo-bad",
    pointerVersion: "solo-good",
    consecutiveCrashes: 3,
    backoffMs: null,
    lastExitCode: 1,
    updatedAtMs: Date.now() - 60_000,
    lastEvent: "crash fallback to solo-good after 3 crashes",
  },
  fetchedAt: new Date(Date.now() - 5 * 60_000).toISOString(),
};

function click(element: Element): void {
  element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
}

describe("HostTraceCard", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    state.connected = true;
    state.client = { versions: { listDaemonVersions: vi.fn() } };
    state.runtimeSnapshot = {
      connectionStatus: "offline",
      lastOnlineAt: new Date(Date.now() - 10 * 60_000).toISOString(),
      lastError: "Connection lost: transport_error",
      probeByConnectionId: new Map([["conn-1", { status: "available", latencyMs: 23 }]]),
    };
    state.sessions = { "srv-1": { serverInfo: { version: "v0.12.0" } } };
    state.cachedSnapshot = CACHED_SNAPSHOT;
    state.saveSnapshot = vi.fn();
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    if (root) {
      act(() => {
        root?.unmount();
      });
    }
    root = null;
    container?.remove();
    container = null;
    state.client = null;
    state.cachedSnapshot = null;
  });

  function renderCard(): Promise<void> {
    return act(async () => {
      root?.render(<HostTraceCard host={HOST} />);
    });
  }

  function query(testId: string): HTMLElement | null {
    return container?.querySelector(`[data-testid="${testId}"]`) ?? null;
  }

  async function toggle(): Promise<void> {
    await act(async () => {
      click(query("host-trace-toggle") as Element);
    });
  }

  it("renders collapsed by default", async () => {
    await renderCard();
    expect(query("host-trace-toggle")).not.toBeNull();
    expect(query("host-trace-content")).toBeNull();
  });

  it("shows connection, version and supervisor blocks when expanded", async () => {
    await renderCard();
    await toggle();

    const content = query("host-trace-content")?.textContent ?? "";
    expect(content).toContain("Status Offline");
    expect(content).toContain("Relay: 23ms");
    expect(content).toContain("Last error: Connection lost");
    expect(query("host-trace-version-srv-1")?.textContent).toContain("Running v0.12.0");
    expect(query("host-trace-version-srv-1")?.textContent).toContain("2 build(s) on host");
    expect(query("host-trace-version-srv-1")?.textContent).toContain("pointer solo-v0.12.0");
    expect(query("host-trace-supervisor-state")?.textContent).toContain("Rolled back");
    expect(query("host-trace-supervisor-state")?.textContent).toContain("solo-bad");
  });

  it("renders without cached data when the host was never seen", async () => {
    state.cachedSnapshot = null;
    await renderCard();
    await toggle();

    const content = query("host-trace-content")?.textContent ?? "";
    expect(content).toContain("No version info seen yet on this device");
    expect(query("host-trace-supervisor-state")).toBeNull();
  });

  it("refreshes version info through the daemon client", async () => {
    state.client = {
      versions: {
        listDaemonVersions: vi.fn().mockResolvedValue({
          requestId: "r",
          runningVersion: "v0.13.0",
          currentVersion: "solo-v0.13.0",
          versions: [{ version: "solo-v0.13.0", mtimeMs: 1 }],
          supervisor: { state: "running", pid: 1, consecutiveCrashes: 0 },
        }),
      },
    };
    await renderCard();
    await toggle();

    await act(async () => {
      click(query("host-trace-refresh") as Element);
    });

    expect(state.client.versions.listDaemonVersions).toHaveBeenCalledTimes(1);
    expect(state.saveSnapshot).toHaveBeenCalledWith(
      "srv-1",
      expect.objectContaining({
        runningVersion: "v0.13.0",
        supervisor: expect.objectContaining({ state: "running" }),
      }),
    );
  });

  it("surfaces a categorized refresh error", async () => {
    state.client = {
      versions: {
        listDaemonVersions: vi
          .fn()
          .mockRejectedValue(new Error("Timeout waiting for message (10000ms)")),
      },
    };
    await renderCard();
    await toggle();

    await act(async () => {
      click(query("host-trace-refresh") as Element);
    });

    expect(query("host-trace-content")?.textContent).toContain(
      "Version refresh failed: host unreachable.",
    );
  });
});
