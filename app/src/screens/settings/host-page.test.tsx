/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SpeedTestResult } from "@server/client/daemon-client";
import type { HostProfile } from "@/types/host-connection";
import { HostPage } from "@/screens/settings/host-page";

const CONNECTION_ID = "relay:relay.example.com";
const SPEED_TEST_BUTTON_TESTID = `connection-speed-test-${CONNECTION_ID}`;
const SPEED_TEST_PANEL_TESTID = `speed-test-panel-${CONNECTION_ID}`;

const { state } = vi.hoisted(() => ({
  state: {
    hosts: [] as HostProfile[],
    snapshot: null as {
      connectionStatus: string;
      activeConnectionId: string | null;
      activeConnection: { type: "relay"; endpoint: string; display: string } | null;
      lastError: string | null;
      probeByConnectionId: Map<string, { status: string; latencyMs: number | null }>;
      client: { speedTest: () => Promise<SpeedTestResult> } | null;
    } | null,
    connectToDaemon: vi.fn(),
  },
}));

vi.mock("react-native", () => ({
  Platform: { OS: "web" },
  View: ({
    children,
    style: _style,
    testID,
    ...props
  }: React.PropsWithChildren<{ style?: unknown; testID?: string } & Record<string, unknown>>) =>
    React.createElement("div", { ...props, "data-testid": testID }, children),
  Text: ({
    children,
    style: _style,
    numberOfLines: _numberOfLines,
    ...props
  }: React.PropsWithChildren<{ style?: unknown; numberOfLines?: number } & Record<string, unknown>>) =>
    React.createElement("span", props, children),
  Pressable: ({
    children,
    onPress,
    disabled,
    testID,
  }: React.PropsWithChildren<{ onPress?: () => void; disabled?: boolean; testID?: string }>) =>
    React.createElement(
      "button",
      { type: "button", onClick: onPress, disabled, "data-testid": testID },
      children,
    ),
  TextInput: (props: Record<string, unknown>) => React.createElement("input", props),
  ActivityIndicator: () => React.createElement("div", { "data-testid": "activity-indicator" }),
  Alert: { alert: vi.fn() },
}));

vi.mock("react-native-unistyles", () => {
  const theme = {
    spacing: { 1: 4, 2: 8, 3: 12, 4: 16, 6: 24 },
    fontSize: { xs: 12, sm: 14, base: 15 },
    fontWeight: { normal: "400" },
    colors: {
      surface0: "#111",
      surface1: "#222",
      surface3: "#333",
      foreground: "#fff",
      foregroundMuted: "#999",
      border: "#444",
      destructive: "#ef4444",
      palette: {
        green: { 400: "#4ade80" },
        amber: { 500: "#f59e0b" },
        red: { 300: "#fca5a5" },
        blue: { 500: "#3b82f6" },
        teal: { 200: "#99f6e4" },
      },
    },
    borderRadius: { md: 6, lg: 8, full: 9999 },
    iconSize: { sm: 16 },
  };

  return {
    StyleSheet: {
      create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
    },
    useUnistyles: () => ({ theme }),
  };
});

vi.mock("lucide-react-native", () => ({
  ChevronRight: () => React.createElement("span", { "data-icon": "ChevronRight" }),
  Gauge: () => React.createElement("span", { "data-icon": "Gauge" }),
  Globe: () => React.createElement("span", { "data-icon": "Globe" }),
  Monitor: () => React.createElement("span", { "data-icon": "Monitor" }),
  Pencil: () => React.createElement("span", { "data-icon": "Pencil" }),
  Trash2: () => React.createElement("span", { "data-icon": "Trash2" }),
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHosts: () => state.hosts,
  useHostRuntimeSnapshot: () => state.snapshot,
  useHostMutations: () => ({
    upsertDirectConnection: vi.fn(),
    upsertRelayConnection: vi.fn(),
    upsertConnectionFromOffer: vi.fn(),
    upsertConnectionFromOfferUrl: vi.fn(),
    renameHost: vi.fn(),
    removeHost: vi.fn(),
    removeConnection: vi.fn(),
  }),
  getHostRuntimeStore: () => ({
    getSnapshot: () => state.snapshot,
  }),
}));

vi.mock("@/stores/session-store", () => ({
  useSessionStore: (selector: (value: { sessions: Record<string, never> }) => unknown) =>
    selector({ sessions: {} }),
}));

vi.mock("@/utils/daemons", () => ({
  formatConnectionStatus: (status: string) => status,
  getConnectionStatusTone: () => "success",
}));

vi.mock("@/utils/test-daemon-connection", () => ({
  connectToDaemon: (...args: unknown[]) => state.connectToDaemon(...args),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    testID,
  }: React.PropsWithChildren<{ onPress?: () => void; testID?: string }>) =>
    React.createElement("button", { type: "button", onClick: onPress, "data-testid": testID }, children),
}));

vi.mock("@/components/adaptive-modal-sheet", () => ({
  AdaptiveModalSheet: ({
    children,
    visible,
  }: React.PropsWithChildren<{ visible?: boolean }>) =>
    visible ? React.createElement("div", null, children) : null,
}));

vi.mock("@/hooks/use-is-local-daemon", () => ({
  useIsLocalDaemon: () => false,
}));

vi.mock("@/screens/settings/settings-section", () => ({
  SettingsSection: ({
    title,
    children,
    testID,
  }: React.PropsWithChildren<{ title: string; testID?: string }>) =>
    React.createElement(
      "section",
      { "data-testid": testID },
      React.createElement("h2", null, title),
      children,
    ),
}));

vi.mock("@/desktop/components/pair-device-modal", () => ({
  PairDeviceModal: () => null,
}));

vi.mock("@/desktop/components/desktop-updates-section", () => ({
  LocalDaemonSection: () => null,
}));

function makeHost(): HostProfile {
  return {
    serverId: "server-1",
    label: "Test Host",
    lifecycle: {},
    connections: [
      {
        id: CONNECTION_ID,
        type: "relay",
        relayEndpoint: "relay.example.com",
        daemonPublicKeyB64: "daemon-public-key",
      },
    ],
    preferredConnectionId: null,
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
  };
}

function makeSpeedTestResult(): SpeedTestResult {
  return {
    transport: "relay",
    samples: 5,
    totalRtt: { minMs: 100, avgMs: 123, maxMs: 150, jitterMs: 50 },
    segments: [
      {
        id: "appToRelay",
        label: "App ↔ Relay",
        estimated: true,
        stats: { minMs: 30, avgMs: 40, maxMs: 50 },
      },
      {
        id: "relayToDaemon",
        label: "Relay ↔ Daemon",
        estimated: false,
        stats: { minMs: 60, avgMs: 70, maxMs: 80 },
      },
      {
        id: "daemonProcessing",
        label: "Daemon processing",
        estimated: false,
        stats: { minMs: 10, avgMs: 13, maxMs: 20 },
      },
    ],
    measuredAt: new Date().toISOString(),
  };
}

function useActiveConnectionSnapshot(speedTest: () => Promise<SpeedTestResult>): void {
  state.snapshot = {
    connectionStatus: "online",
    activeConnectionId: CONNECTION_ID,
    activeConnection: { type: "relay", endpoint: "relay.example.com", display: "relay" },
    lastError: null,
    probeByConnectionId: new Map([[CONNECTION_ID, { status: "available", latencyMs: 42 }]]),
    client: { speedTest },
  };
}

function click(element: Element): void {
  element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
}

describe("HostPage speed test", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    state.hosts = [makeHost()];
    state.snapshot = null;
    state.connectToDaemon = vi.fn();
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
    state.hosts = [];
    state.snapshot = null;
  });

  async function renderPage(): Promise<void> {
    await act(async () => {
      root?.render(<HostPage serverId="server-1" />);
    });
  }

  function speedTestButton(): HTMLElement {
    const button = container?.querySelector(`[data-testid="${SPEED_TEST_BUTTON_TESTID}"]`);
    expect(button).not.toBeNull();
    return button as HTMLElement;
  }

  it("renders a speed test action next to the connection latency", async () => {
    useActiveConnectionSnapshot(vi.fn().mockResolvedValue(makeSpeedTestResult()));

    await renderPage();

    expect(speedTestButton()).not.toBeNull();
    expect(container?.textContent).toContain("Relay (relay.example.com)");
    expect(container?.textContent).toContain("42ms");
  });

  it("shows a testing state and then the segment breakdown on success", async () => {
    let resolveSpeedTest: ((result: SpeedTestResult) => void) | null = null;
    const speedTest = vi.fn(
      () =>
        new Promise<SpeedTestResult>((resolve) => {
          resolveSpeedTest = resolve;
        }),
    );
    useActiveConnectionSnapshot(speedTest);

    await renderPage();
    await act(async () => {
      click(speedTestButton());
    });

    expect(speedTest).toHaveBeenCalledTimes(1);
    expect(container?.textContent).toContain("Testing...");

    await act(async () => {
      resolveSpeedTest?.(makeSpeedTestResult());
    });

    const panel = container?.querySelector(`[data-testid="${SPEED_TEST_PANEL_TESTID}"]`);
    expect(panel).not.toBeNull();
    expect(container?.textContent).toContain("App ↔ Relay");
    expect(container?.textContent).toContain("Relay ↔ Daemon");
    expect(container?.textContent).toContain("Daemon processing");
    expect(container?.textContent).toContain("~ estimated");
    expect(container?.textContent).toContain("40ms avg · 30–50");
    expect(container?.textContent).toContain(
      "Avg 123ms · min 100 · max 150 · jitter 50ms · 5 samples",
    );
    expect(container?.textContent).toContain("measured just now");
    // Active client was reused; no temporary connection was opened.
    expect(state.connectToDaemon).not.toHaveBeenCalled();
  });

  it("toggles the breakdown panel when tapping the action after a successful run", async () => {
    const speedTest = vi.fn().mockResolvedValue(makeSpeedTestResult());
    useActiveConnectionSnapshot(speedTest);

    await renderPage();
    await act(async () => {
      click(speedTestButton());
    });
    expect(container?.querySelector(`[data-testid="${SPEED_TEST_PANEL_TESTID}"]`)).not.toBeNull();

    await act(async () => {
      click(speedTestButton());
    });
    expect(container?.querySelector(`[data-testid="${SPEED_TEST_PANEL_TESTID}"]`)).toBeNull();

    await act(async () => {
      click(speedTestButton());
    });
    expect(container?.querySelector(`[data-testid="${SPEED_TEST_PANEL_TESTID}"]`)).not.toBeNull();
    // Toggling never re-runs the test.
    expect(speedTest).toHaveBeenCalledTimes(1);
  });

  it("shows an inline error on failure and retries when tapping the action again", async () => {
    const speedTest = vi
      .fn()
      .mockRejectedValueOnce(new Error("Speed test failed on sample 1 of 5: timeout"))
      .mockResolvedValueOnce(makeSpeedTestResult());
    useActiveConnectionSnapshot(speedTest);

    await renderPage();
    await act(async () => {
      click(speedTestButton());
    });

    expect(container?.textContent).toContain("Speed test failed on sample 1 of 5: timeout");

    await act(async () => {
      click(speedTestButton());
    });

    expect(speedTest).toHaveBeenCalledTimes(2);
    expect(container?.textContent).toContain("Avg 123ms");
  });

  it("opens a temporary connection when the tested connection is not active", async () => {
    const tempClient = { speedTest: vi.fn().mockResolvedValue(makeSpeedTestResult()) };
    const close = vi.fn().mockResolvedValue(undefined);
    state.connectToDaemon = vi.fn().mockResolvedValue({
      client: { ...tempClient, close },
      serverId: "server-1",
      hostname: null,
    });
    state.snapshot = {
      connectionStatus: "online",
      activeConnectionId: "other-connection",
      activeConnection: null,
      lastError: null,
      probeByConnectionId: new Map([[CONNECTION_ID, { status: "available", latencyMs: 42 }]]),
      client: null,
    };

    await renderPage();
    await act(async () => {
      click(speedTestButton());
    });

    expect(state.connectToDaemon).toHaveBeenCalledTimes(1);
    expect(tempClient.speedTest).toHaveBeenCalledTimes(1);
    expect(close).toHaveBeenCalledTimes(1);
    expect(container?.textContent).toContain("Avg 123ms");
  });
});
