import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockTheme, mockPanelState } = vi.hoisted(() => ({
  mockTheme: {
    spacing: { 1: 4, 2: 8, 3: 12, 4: 16, 6: 24, 8: 32 },
    iconSize: { sm: 14, md: 18, lg: 22 },
    borderWidth: { 1: 1 },
    borderRadius: { full: 999, md: 6, lg: 8, "2xl": 16 },
    fontSize: { xs: 11, sm: 13, base: 15, lg: 18 },
    fontWeight: { normal: "400", medium: "500" },
    shadow: { md: {} },
    colors: {
      surface0: "#1a1a2e",
      surface1: "#111",
      surface2: "#222",
      surface3: "#333",
      surface4: "#888",
      foreground: "#ffffff",
      foregroundMuted: "#aaaaaa",
      popoverForeground: "#fff",
      border: "#555",
      borderAccent: "#444",
      accent: "#0a84ff",
      accentForeground: "#fff",
      destructive: "#ff453a",
      background: "#0f0f23",
      palette: {
        green: { 500: "#30d158", 600: "#24b14c", 800: "#126024" },
        red: { 500: "#ff453a", 600: "#d92d20" },
        zinc: { 600: "#52525b" },
      },
    },
  },
  mockPanelState: {
    mobileView: "agent",
    showMobileAgentList: vi.fn(),
    toggleMobileAgentList: vi.fn(),
    toggleDesktopAgentList: vi.fn(),
    openDesktopAgentList: vi.fn(),
    closeDesktopAgentList: vi.fn(),
    closeDesktopFileExplorer: vi.fn(),
    toggleFocusMode: vi.fn(),
    desktop: {
      focusModeEnabled: false,
      agentListOpen: false,
      fileExplorerOpen: false,
    },
  },
}));

// --- zustand/traditional ---
vi.mock("zustand/traditional", () => ({
  useStoreWithEqualityFn: (
    store: { getState: () => unknown },
    selector: (state: unknown) => unknown,
  ) => selector(store.getState()),
}));

// --- React Query ---
vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: undefined, isLoading: false, isError: false, dataUpdatedAt: 0 }),
  useMutation: () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false }),
  useQueryClient: () => ({
    setQueryData: vi.fn(),
    invalidateQueries: vi.fn(),
    getQueryData: () => undefined,
  }),
  QueryClientProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  QueryClient: class QueryClient {},
}));

// --- Expo Clipboard ---
vi.mock("expo-clipboard", () => ({
  setStringAsync: vi.fn(async () => {}),
}));

// --- Lucide Icons ---
vi.mock("lucide-react-native", () => {
  const createIcon = (name: string) => (props: Record<string, unknown>) =>
    React.createElement("span", { ...props, "data-icon": name });
  return {
    CopyX: createIcon("CopyX"),
    ArrowLeftToLine: createIcon("ArrowLeftToLine"),
    ArrowRightToLine: createIcon("ArrowRightToLine"),
    ChevronDown: createIcon("ChevronDown"),
    Copy: createIcon("Copy"),
    Ellipsis: createIcon("Ellipsis"),
    EllipsisVertical: createIcon("EllipsisVertical"),
    PanelRight: createIcon("PanelRight"),
    RotateCw: createIcon("RotateCw"),
    Settings: createIcon("Settings"),
    SquarePen: createIcon("SquarePen"),
    SquareTerminal: createIcon("SquareTerminal"),
    X: createIcon("X"),
  };
});

// --- React Native Gesture Handler ---
vi.mock("react-native-gesture-handler", () => ({
  GestureHandlerRootView: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  GestureDetector: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  Gesture: { Pan: () => ({ enabled: () => ({}) }) },
}));

// --- Safe Area Context ---
vi.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => ({ top: 0, right: 0, bottom: 0, left: 0 }),
}));

// --- Unistyles ---
vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(mockTheme) : factory),
  },
  withUnistyles: <T,>(component: T) => component,
  useUnistyles: () => ({ theme: mockTheme }),
}));

// --- tiny-invariant ---
vi.mock("tiny-invariant", () => ({
  default: (condition: unknown, message?: string) => {
    if (!condition) throw new Error(message ?? "Invariant failed");
  },
}));

// --- Components ---
vi.mock("@/components/headers/menu-header", () => ({ SidebarMenuToggle: () => null }));
vi.mock("@/components/headers/header-toggle-button", () => ({ HeaderToggleButton: () => null }));
vi.mock("@/components/headers/screen-header", () => ({ ScreenHeader: () => null }));
vi.mock("@/components/branch-switcher", () => ({ BranchSwitcher: () => null }));
vi.mock("@/components/ui/combobox", () => ({ Combobox: () => null }));
vi.mock("@/components/ui/shortcut", () => ({ Shortcut: () => null }));
vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuItem: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuSeparator: () => null,
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/explorer-sidebar", () => ({ ExplorerSidebar: () => null }));
vi.mock("@/components/split-container", () => ({ SplitContainer: () => null }));
vi.mock("@/components/icons/source-control-panel-icon", () => ({
  SourceControlPanelIcon: () => null,
}));

// --- Screens (workspace sub-modules) ---
vi.mock("@/screens/workspace/workspace-git-actions", () => ({ WorkspaceGitActions: () => null }));
vi.mock("@/screens/workspace/workspace-open-in-editor-button", () => ({
  WorkspaceOpenInEditorButton: () => null,
}));
vi.mock("@/screens/workspace/workspace-scripts-button", () => ({
  WorkspaceScriptsButton: () => null,
}));
vi.mock("@/screens/workspace/workspace-tab-presentation", () => ({
  WorkspaceTabPresentationResolver: () => null,
  WorkspaceTabIcon: () => null,
  WorkspaceTabOptionRow: () => null,
}));
vi.mock("@/screens/workspace/workspace-desktop-tabs-row", () => ({
  WorkspaceDesktopTabsRow: () => null,
}));
vi.mock("@/screens/workspace/workspace-tab-menu", () => ({
  buildWorkspaceTabMenuEntries: () => [],
}));
vi.mock("@/screens/workspace/workspace-header-source", () => ({
  resolveWorkspaceHeaderRenderState: () => ({ kind: "loading" }),
  shouldRenderMissingWorkspaceDescriptor: () => false,
}));
vi.mock("@/screens/workspace/workspace-agent-visibility", () => ({
  deriveWorkspaceAgentVisibility: () => ({
    activeAgentIds: new Set(),
    knownAgentIds: new Set(),
    pinnedAgentIds: new Set(),
    hydratedAgentCount: 0,
  }),
  workspaceAgentVisibilityEqual: () => true,
}));
vi.mock("@/screens/workspace/workspace-pane-state", () => ({
  deriveWorkspacePaneState: () => ({
    pane: null,
    tabs: [],
    focusedTabId: null,
    activeTabId: null,
    activeTab: null,
  }),
}));
vi.mock("@/screens/workspace/workspace-pane-content", () => ({
  buildWorkspacePaneContentModel: () => ({}),
  WorkspacePaneContent: () => null,
}));
vi.mock("@/screens/workspace/use-mounted-tab-set", () => ({
  useMountedTabSet: () => new Set(),
}));
vi.mock("@/screens/workspace/workspace-empty-draft-seed", () => ({
  shouldSeedEmptyWorkspaceDraft: () => false,
}));
vi.mock("@/screens/workspace/workspace-bulk-close", () => ({
  buildBulkCloseConfirmationMessage: () => "",
  classifyBulkClosableTabs: () => ({ closable: [], skipped: [] }),
  closeBulkWorkspaceTabs: vi.fn(async () => {}),
}));

// --- Contexts ---
vi.mock("@/contexts/explorer-sidebar-animation-context", () => ({
  ExplorerSidebarAnimationProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));
vi.mock("@/contexts/toast-context", () => ({
  useToast: () => ({ error: vi.fn(), success: vi.fn() }),
}));

// --- Hooks ---
vi.mock("@/hooks/use-explorer-open-gesture", () => ({
  useExplorerOpenGesture: () => null,
}));
vi.mock("@/hooks/use-keyboard-action-handler", () => ({
  useKeyboardActionHandler: () => {},
}));
vi.mock("@/hooks/use-providers-snapshot", () => ({
  useProvidersSnapshot: () => null,
}));
vi.mock("@/hooks/use-archive-agent", () => ({
  useArchiveAgent: () => ({ archiveAgent: vi.fn(async () => {}) }),
}));
vi.mock("@/hooks/use-stable-event", () => ({
  useStableEvent: (fn: (...args: unknown[]) => unknown) => fn,
}));
vi.mock("@/hooks/use-checkout-status-query", () => ({
  checkoutStatusQueryKey: (...args: unknown[]) => ["checkout-status", ...args],
}));
vi.mock("@/hooks/use-container-width", () => ({
  useContainerWidthBelow: () => false,
}));

// --- Stores ---
vi.mock("@/stores/panel-store", () => {
  const usePanelStore = (selector: (state: typeof mockPanelState) => unknown) =>
    selector(mockPanelState);
  usePanelStore.getState = () => mockPanelState;
  return {
    usePanelStore,
    selectIsFileExplorerOpen: (state: typeof mockPanelState) =>
      state.desktop.fileExplorerOpen,
  };
});
vi.mock("@/stores/session-store", () => {
  const state = {
    sessions: {},
    setFocusedAgentId: vi.fn(),
  };
  const useSessionStore = (selector: (s: typeof state) => unknown) => selector(state);
  useSessionStore.getState = () => state;
  return { useSessionStore };
});
vi.mock("@/stores/session-store-hooks", () => ({
  useWorkspace: () => null,
}));
vi.mock("@/stores/workspace-layout-store", () => {
  const storeState = {
    openTabFocused: vi.fn(),
    focusPane: vi.fn(),
    focusTab: vi.fn(),
    closeTab: vi.fn(),
    unpinAgent: vi.fn(),
    hideAgent: vi.fn(),
    retargetTab: vi.fn(),
    reconcileTabs: vi.fn(),
    splitPane: vi.fn(),
    splitPaneEmpty: vi.fn(),
    moveTabToPane: vi.fn(),
    resizeSplit: vi.fn(),
    reorderTabsInPane: vi.fn(),
    tabs: new Map(),
    layoutByWorkspace: {} as Record<string, unknown>,
    pinnedAgentIdsByWorkspace: {} as Record<string, Set<string>>,
    hiddenAgentIdsByWorkspace: {} as Record<string, Set<string>>,
  };
  const useWorkspaceLayoutStore = (selector: (state: typeof storeState) => unknown) =>
    selector(storeState);
  useWorkspaceLayoutStore.getState = () => storeState;
  return {
    useWorkspaceLayoutStore,
    useWorkspaceLayoutStoreHydrated: () => true,
    buildWorkspaceTabPersistenceKey: () => "server:/workspace",
    collectAllTabs: () => [],
  };
});
vi.mock("@/stores/create-flow-store", () => ({
  useCreateFlowStore: () => ({ isCreating: false }),
}));
vi.mock("@/stores/workspace-setup-store", () => ({
  shouldShowWorkspaceSetup: () => false,
  useWorkspaceSetupStore: () => ({ lastAutoOpenTimestamp: null }),
}));
vi.mock("@/stores/draft-keys", () => ({
  generateDraftId: () => "draft-1",
}));

// --- Runtime ---
vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => null,
  useHostRuntimeIsConnected: () => false,
}));

// --- Terminal ---
vi.mock("@/terminal/hooks/use-workspace-terminal-session-retention", () => ({
  useWorkspaceTerminalSessionRetention: () => {},
}));

// --- Constants ---
vi.mock("@/constants/layout", () => ({
  useIsCompactFormFactor: () => false,
  supportsDesktopPaneSplits: () => true,
}));
vi.mock("@/constants/platform", () => ({ isWeb: true, isNative: false }));

// --- Utils ---
vi.mock("@/utils/workspace-tab-identity", () => ({
  normalizeWorkspaceTabTarget: (target: unknown) => target,
  workspaceTabTargetsEqual: () => true,
}));
vi.mock("@/utils/terminal-list", () => ({
  upsertTerminalListEntry: ({ terminals }: { terminals: unknown[] }) => terminals,
}));
vi.mock("@/utils/confirm-dialog", () => ({
  confirmDialog: vi.fn(async () => true),
}));
vi.mock("@/utils/provider-command-templates", () => ({
  buildProviderCommand: () => "",
}));
vi.mock("@/utils/workspace-execution", () => ({
  getWorkspaceExecutionAuthority: () => ({ kind: "workspace", directory: "/workspace" }),
  resolveWorkspaceRouteId: ({ routeWorkspaceId }: { routeWorkspaceId: string }) =>
    routeWorkspaceId,
}));
vi.mock("@/utils/split-navigation", () => ({
  findAdjacentPane: () => null,
}));
vi.mock("@/utils/path", () => ({
  isAbsolutePath: (p: string) => p.startsWith("/"),
}));
vi.mock("@/utils/format-shortcut", () => ({}));

// Import after mocks
import { WorkspaceScreen } from "./workspace-screen";

let root: Root | null = null;
let container: HTMLElement | null = null;

beforeEach(() => {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal("window", dom.window);
  vi.stubGlobal("document", dom.window.document);
  vi.stubGlobal("HTMLElement", dom.window.HTMLElement);
  vi.stubGlobal("Node", dom.window.Node);
  vi.stubGlobal("navigator", dom.window.navigator);

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
  container = null;
  vi.unstubAllGlobals();
});

describe("WorkspaceScreen", () => {
  it("renders without crashing", () => {
    act(() => {
      root?.render(
        <WorkspaceScreen serverId="test-server" workspaceId="/workspace" isRouteFocused />,
      );
    });

    expect(container).not.toBeNull();
    expect(container?.innerHTML).not.toBe("");
  });
});
