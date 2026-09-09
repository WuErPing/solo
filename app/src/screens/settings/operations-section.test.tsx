/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { Alert } from "react-native";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DaemonRpcError } from "@server/client/daemon-client";
import { OperationsSection } from "@/screens/settings/operations-section";

const SERVER_ID = "server-1";
const SWITCH_BUTTON_TESTID = "settings-operations-version-switch-button";

interface MockDaemonClient {
  restartServer: ReturnType<typeof vi.fn>;
  versions: {
    listDaemonVersions: ReturnType<typeof vi.fn>;
    switchDaemonVersion: ReturnType<typeof vi.fn>;
  };
}

const { state } = vi.hoisted(() => ({
  state: {
    connected: true,
    client: null as MockDaemonClient | null,
    sessions: {} as Record<string, { serverInfo: { version: string | null } | null } | undefined>,
    confirmDialog: vi.fn(),
    cachedSnapshot: null as Record<string, unknown> | null,
    saveSnapshot: vi.fn(),
  },
}));

vi.mock("@/stores/host-version-snapshots-store", () => ({
  saveHostVersionSnapshot: (...args: unknown[]) => state.saveSnapshot(...args),
  useHostVersionSnapshot: () => state.cachedSnapshot,
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
    ...props
  }: React.PropsWithChildren<{ style?: unknown } & Record<string, unknown>>) =>
    React.createElement("span", props, children),
  Alert: { alert: vi.fn() },
}));

vi.mock("react-native-unistyles", () => {
  const theme = {
    colors: { foreground: "#fff", borderAccent: "#444" },
    iconSize: { sm: 16 },
    spacing: { 2: 8, 3: 12 } as Record<number, number>,
    borderRadius: { md: 8 },
    fontSize: { sm: 14 },
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
  RotateCw: () => React.createElement("span", { "data-icon": "RotateCw" }),
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: React.PropsWithChildren) =>
    React.createElement("div", { "data-testid": "version-dropdown" }, children),
  DropdownMenuTrigger: ({
    children,
    disabled,
    testID,
  }: React.PropsWithChildren<{ disabled?: boolean; testID?: string }>) =>
    React.createElement("button", { type: "button", disabled, "data-testid": testID }, children),
  DropdownMenuContent: ({ children }: React.PropsWithChildren) =>
    React.createElement("div", null, children),
  DropdownMenuItem: ({
    children,
    onSelect,
    disabled,
    testID,
  }: React.PropsWithChildren<{ onSelect?: () => void; disabled?: boolean; testID?: string }>) =>
    React.createElement(
      "button",
      { type: "button", onClick: onSelect, disabled, "data-testid": testID },
      children,
    ),
}));

vi.mock("@/styles/settings", () => ({
  settingsStyles: {
    card: {},
    row: {},
    rowContent: {},
    rowTitle: {},
    rowHint: {},
  },
}));

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => ({
    getSnapshot: () => ({ connectionStatus: state.connected ? "online" : "offline" }),
  }),
  isHostRuntimeConnected: (snapshot: { connectionStatus: string } | null) =>
    snapshot?.connectionStatus === "online",
  useHostRuntimeClient: () => state.client,
  useHostRuntimeIsConnected: () => state.connected,
}));

vi.mock("@/stores/session-store", () => ({
  useSessionStore: (selector: (value: { sessions: typeof state.sessions }) => unknown) =>
    selector({ sessions: state.sessions }),
}));

vi.mock("@/utils/confirm-dialog", () => ({
  confirmDialog: (...args: unknown[]) => state.confirmDialog(...args),
}));

vi.mock("@/hooks/use-daemon-config", () => ({
  useDaemonConfig: () => ({ config: null, patchConfig: vi.fn() }),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
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
}));

vi.mock("@/components/ui/switch", () => ({
  Switch: () => React.createElement("input", { type: "checkbox" }),
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

function makeClient(overrides?: {
  versions?: { version: string; mtimeMs: number }[];
  listImpl?: () => Promise<unknown>;
  switchImpl?: () => Promise<unknown>;
  supervisor?: Record<string, unknown> | null;
}): MockDaemonClient {
  return {
    restartServer: vi.fn().mockResolvedValue({}),
    versions: {
      listDaemonVersions: vi.fn().mockImplementation(
        overrides?.listImpl ??
          (() =>
            Promise.resolve({
              requestId: "req-1",
              runningVersion: "v0.7.4",
              currentVersion: "solo-v0.7.4",
              versions: overrides?.versions ?? [
                { version: "solo-v0.8.0-dev-20260820", mtimeMs: 2000 },
                { version: "solo-v0.7.4", mtimeMs: 1000 },
              ],
              supervisor: overrides?.supervisor ?? null,
              error: null,
            })),
      ),
      switchDaemonVersion: vi.fn().mockImplementation(
        overrides?.switchImpl ??
          (() =>
            Promise.resolve({
              status: "daemon_version_switch_requested",
              clientId: "client-1",
              version: "solo-v0.8.0-dev-20260820",
              requestId: "req-2",
            })),
      ),
    },
  };
}

function click(element: Element): void {
  element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
}

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

describe("OperationsSection daemon version card", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    state.connected = true;
    state.client = makeClient();
    state.sessions = { [SERVER_ID]: { serverInfo: { version: "v0.7.4" } } };
    state.confirmDialog = vi.fn().mockResolvedValue(true);
    state.cachedSnapshot = null;
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
    state.sessions = {};
    vi.mocked(Alert.alert).mockClear();
  });

  async function renderSection(): Promise<void> {
    await act(async () => {
      root?.render(<OperationsSection serverId={SERVER_ID} hostLabel="Test Host" />);
    });
  }

  function switchButton(): HTMLButtonElement | null {
    return container?.querySelector(`[data-testid="${SWITCH_BUTTON_TESTID}"]`) ?? null;
  }

  function versionItem(display: string): HTMLButtonElement | null {
    return (
      container?.querySelector(`[data-testid="settings-operations-version-item-${display}"]`) ??
      null
    );
  }

  it("renders the running version and lists recent builds in the picker", async () => {
    await renderSection();

    expect(state.client?.versions.listDaemonVersions).toHaveBeenCalledTimes(1);
    expect(container?.textContent).toContain("Daemon version");
    expect(container?.textContent).toContain("Running v0.7.4");
    expect(container?.textContent).toContain("Latest local build v0.8.0-dev-20260820");

    const button = switchButton();
    expect(button).not.toBeNull();
    expect(button?.disabled).toBe(false);
    expect(container?.textContent).toContain("Switch version");

    // The picker lists newest first and tags the latest build.
    const latest = versionItem("v0.8.0-dev-20260820");
    expect(latest).not.toBeNull();
    expect(latest?.textContent).toContain("(latest)");
    expect(latest?.disabled).toBe(false);
    // The running version is listed but disabled.
    expect(versionItem("v0.7.4")?.disabled).toBe(true);
  });

  it("keeps the picker enabled when on latest but disables the running item", async () => {
    state.sessions = { [SERVER_ID]: { serverInfo: { version: "v0.8.0-dev-20260820" } } };

    await renderSection();

    const button = switchButton();
    expect(button).not.toBeNull();
    expect(button?.disabled).toBe(false);
    expect(versionItem("v0.8.0-dev-20260820")?.disabled).toBe(true);
    expect(versionItem("v0.7.4")?.disabled).toBe(false);
  });

  it("hides the switch button when there are no local versions", async () => {
    state.client = makeClient({ versions: [] });

    await renderSection();

    expect(switchButton()).toBeNull();
    expect(container?.textContent).toContain("Running v0.7.4");
  });

  it("hides the version card quietly when the daemon does not support it", async () => {
    // Old daemons answer the unknown request with a code-less rpc_error; the
    // card must disappear without a console.error.
    state.client = makeClient({
      listImpl: () =>
        Promise.reject(
          new DaemonRpcError({
            requestId: "req-1",
            error: "unsupported message type",
            requestType: "list_daemon_versions_request",
          }),
        ),
    });
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    await renderSection();

    expect(container?.textContent).not.toContain("Daemon version");
    expect(switchButton()).toBeNull();
    const ownErrorLogs = errorSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("[OperationsSection]"),
    );
    expect(ownErrorLogs).toEqual([]);
    errorSpy.mockRestore();
  });

  it("keeps the card visible with a hint when the host is unreachable", async () => {
    state.client = makeClient({
      listImpl: () => Promise.reject(new Error("Timeout waiting for message (10000ms)")),
    });

    await renderSection();

    expect(container?.textContent).toContain("Daemon version");
    expect(container?.textContent).toContain("Host unreachable");
    expect(container?.textContent).toContain("No version info seen yet on this device.");
  });

  it("shows the cached snapshot when the host is unreachable", async () => {
    state.client = makeClient({
      listImpl: () => Promise.reject(new Error("Timeout waiting for message (10000ms)")),
    });
    state.cachedSnapshot = {
      versions: [
        { version: "solo-v0.12.0", mtimeMs: 200 },
        { version: "solo-v0.11.0", mtimeMs: 100 },
      ],
      currentVersion: "solo-v0.12.0",
      runningVersion: "v0.12.0",
      supervisor: null,
      fetchedAt: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
    };

    await renderSection();

    expect(container?.textContent).toContain("Host unreachable");
    expect(container?.textContent).toContain("running v0.12.0, 2 build(s) on host.");
  });

  it("shows the refused reason with its code", async () => {
    state.client = makeClient({
      listImpl: () =>
        Promise.reject(
          new DaemonRpcError({
            requestId: "req-1",
            error: "not allowed",
            requestType: "list_daemon_versions_request",
            code: "NOT_SUPERVISED",
          }),
        ),
    });

    await renderSection();

    expect(container?.textContent).toContain("Version info unavailable (NOT_SUPERVISED).");
  });

  it("persists the version snapshot including supervisor state on success", async () => {
    const supervisor = {
      state: "backoff",
      pid: 9,
      spawnedBinary: "solo-bad",
      pointerVersion: "solo-bad",
      consecutiveCrashes: 3,
      backoffMs: 4000,
      lastExitCode: 1,
      updatedAtMs: 1700000000000,
      lastEvent: "daemon crashed exit=1",
    };
    state.client = makeClient({ supervisor });

    await renderSection();

    expect(state.saveSnapshot).toHaveBeenCalledWith(
      SERVER_ID,
      expect.objectContaining({
        versions: expect.arrayContaining([
          expect.objectContaining({ version: "solo-v0.8.0-dev-20260820" }),
        ]),
        runningVersion: "v0.7.4",
        currentVersion: "solo-v0.7.4",
        supervisor,
      }),
    );
  });

  it("switches to the selected build after confirmation and waits for reconnect", async () => {
    state.confirmDialog = vi.fn().mockImplementation(async () => {
      // The daemon drops the connection once the switch request lands.
      state.connected = false;
      return true;
    });

    await renderSection();
    const item = versionItem("v0.8.0-dev-20260820");
    expect(item).not.toBeNull();

    await act(async () => {
      click(item as Element);
    });

    expect(state.confirmDialog).toHaveBeenCalledTimes(1);
    expect(state.client?.versions.switchDaemonVersion).toHaveBeenCalledWith(
      "solo-v0.8.0-dev-20260820",
    );
    expect(container?.textContent).toContain("Switching...");

    // The host comes back online; the reconnect wait should settle.
    state.connected = true;
    await act(async () => {
      await sleep(400);
    });

    expect(container?.textContent).not.toContain("Switching...");
    expect(vi.mocked(Alert.alert)).not.toHaveBeenCalled();
  });

  it("can roll back to an older build", async () => {
    // Run the latest build so the older one is selectable.
    state.sessions = { [SERVER_ID]: { serverInfo: { version: "v0.8.0-dev-20260820" } } };
    await renderSection();

    const older = versionItem("v0.7.4");
    expect(older?.disabled).toBe(false);
    await act(async () => {
      click(older as Element);
    });

    expect(state.client?.versions.switchDaemonVersion).toHaveBeenCalledWith("solo-v0.7.4");
  });

  it("shows an error alert when the switch request is rejected", async () => {
    state.client = makeClient({
      switchImpl: () => Promise.reject(new Error("daemon returned rpc_error: VERSION_NOT_FOUND")),
    });

    await renderSection();
    const item = versionItem("v0.8.0-dev-20260820");
    expect(item).not.toBeNull();

    await act(async () => {
      click(item as Element);
    });

    expect(state.client?.versions.switchDaemonVersion).toHaveBeenCalledTimes(1);
    expect(vi.mocked(Alert.alert)).toHaveBeenCalledWith(
      "Error",
      "daemon returned rpc_error: VERSION_NOT_FOUND",
    );
    expect(container?.textContent).not.toContain("Switching...");
  });
});
