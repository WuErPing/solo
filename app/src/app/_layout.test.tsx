import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { throwFlag, mockTheme, mockStore, mockPanelState } = vi.hoisted(() => ({
  throwFlag: { current: false },
  mockTheme: {
    colors: {
      surface0: "#1a1a2e",
      foreground: "#ffffff",
      foregroundMuted: "#aaaaaa",
      background: "#0f0f23",
      destructive: "#ff453a",
    },
  },
  mockStore: {
    getEarliestOnlineHostServerId: () => null,
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
    desktop: { focusModeEnabled: false, agentListOpen: false, fileExplorerOpen: false },
  },
}));

// --- Side-effect import ---
vi.mock("@/styles/unistyles", () => ({}));

// --- Portal ---
vi.mock("@gorhom/portal", () => ({
  PortalProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// --- React Query ---
vi.mock("@tanstack/react-query", () => ({
  QueryClientProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  QueryClient: class QueryClient {},
}));

// --- Expo Linking ---
vi.mock("expo-linking", () => ({
  getInitialURL: vi.fn(async () => null),
  addEventListener: vi.fn(() => ({ remove: vi.fn() })),
}));

// --- Notifications ---
vi.mock("@/utils/notifications", () => ({
  setNotificationHandler: vi.fn(),
  addNotificationResponseReceivedListener: vi.fn(() => ({ remove: vi.fn() })),
  getLastNotificationResponseAsync: vi.fn(async () => null),
}));

// --- Expo Router ---
vi.mock("expo-router", () => {
  const Screen = (_props: Record<string, unknown>) => null;
  const Protected = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  const Stack = ({ children }: { children: React.ReactNode }) => <>{children}</>;
  Stack.Screen = Screen;
  Stack.Protected = Protected;
  return {
    Stack,
    useRouter: () => ({ navigate: vi.fn(), replace: vi.fn() }),
    usePathname: () => "/",
    useGlobalSearchParams: () => ({}),
    useNavigationContainerRef: () => ({
      addListener: vi.fn(() => vi.fn()),
    }),
  };
});

// --- React Native Gesture Handler ---
vi.mock("react-native-gesture-handler", () => ({
  GestureHandlerRootView: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  GestureDetector: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  Gesture: {
    Pan: () => {
      const chain: Record<string, unknown> = {};
      const methods = [
        "withRef",
        "enabled",
        "manualActivation",
        "failOffsetY",
        "onTouchesDown",
        "onTouchesMove",
        "onStart",
        "onUpdate",
        "onEnd",
        "onFinalize",
      ];
      for (const method of methods) {
        chain[method] = () => chain;
      }
      return chain;
    },
  },
}));

// --- Keyboard Controller ---
vi.mock("react-native-keyboard-controller", () => ({
  KeyboardProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// --- Reanimated ---
vi.mock("react-native-reanimated", () => ({
  useSharedValue: (value: unknown) => ({ value }),
  runOnJS: (fn: (...args: unknown[]) => unknown) => fn,
  interpolate: () => 0,
  Extrapolation: { CLAMP: "clamp" },
  useAnimatedStyle: (factory: () => unknown) => factory(),
}));

// --- Safe Area Context ---
vi.mock("react-native-safe-area-context", () => ({
  SafeAreaProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// --- Unistyles ---
vi.mock("react-native-unistyles", () => ({
  UnistylesRuntime: { setAdaptiveThemes: vi.fn(), setTheme: vi.fn() },
  useUnistyles: () => ({ theme: mockTheme }),
  StyleSheet: {
    create: (styles: Record<string, unknown>) => styles,
  },
}));

// --- Components (all null) ---
vi.mock("@/components/command-center", () => ({ CommandCenter: () => null }));
vi.mock("@/components/worktree-setup-callout-source", () => ({
  WorktreeSetupCalloutSource: () => null,
}));
vi.mock("@/components/download-toast", () => ({ DownloadToast: () => null }));
vi.mock("@/components/keyboard-shortcuts-dialog", () => ({ KeyboardShortcutsDialog: () => null }));
vi.mock("@/components/left-sidebar", () => ({ LeftSidebar: () => null }));
vi.mock("@/components/project-picker-modal", () => ({ ProjectPickerModal: () => null }));
vi.mock("@/components/workspace-setup-dialog", () => ({ WorkspaceSetupDialog: () => null }));
vi.mock("@/components/workspace-shortcut-targets-subscriber", () => ({
  WorkspaceShortcutTargetsSubscriber: () => null,
}));

// --- Sidebar Animation Context (controllable throw) ---
vi.mock("@/contexts/sidebar-animation-context", () => ({
  SidebarAnimationProvider: ({ children }: { children: React.ReactNode }) => {
    if (throwFlag.current) throw new Error("Test crash in AppShell");
    return <>{children}</>;
  },
  useSidebarAnimation: () => ({
    translateX: { value: 0 },
    backdropOpacity: { value: 0 },
    windowWidth: 300,
    animateToOpen: vi.fn(),
    animateToClose: vi.fn(),
    isGesturing: { value: false },
    gestureAnimatingRef: { current: false },
    openGestureRef: { current: null },
  }),
}));

// --- Other Contexts ---
vi.mock("@/contexts/horizontal-scroll-context", () => ({
  HorizontalScrollProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useHorizontalScrollOptional: () => null,
}));
vi.mock("@/contexts/session-context", () => ({
  SessionProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@/contexts/sidebar-callout-context", () => ({
  SidebarCalloutProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@/contexts/toast-context", () => ({
  ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// --- Constants ---
vi.mock("@/constants/platform", () => ({ isWeb: true, isNative: false }));
vi.mock("@/constants/layout", () => ({
  getIsElectronRuntime: () => false,
  useIsCompactFormFactor: () => false,
}));

// --- Desktop ---
vi.mock("@/desktop/daemon/desktop-daemon", () => ({ shouldUseDesktopDaemon: () => false }));
vi.mock("@/desktop/electron/events", () => ({ listenToDesktopEvent: vi.fn(async () => vi.fn()) }));
vi.mock("@/desktop/electron/window", () => ({
  updateDesktopWindowControls: vi.fn(async () => {}),
}));
vi.mock("@/desktop/host", () => ({ getDesktopHost: () => null }));
vi.mock("@/desktop/updates/update-callout-source", () => ({ UpdateCalloutSource: () => null }));

// --- Hooks ---
vi.mock("@/hooks/use-active-worktree-new-action", () => ({
  useActiveWorktreeNewAction: () => {},
}));
vi.mock("@/hooks/use-color-scheme", () => ({ useColorScheme: () => "dark" }));
vi.mock("@/hooks/use-favicon-status", () => ({ useFaviconStatus: () => {} }));
vi.mock("@/hooks/use-keyboard-shortcuts", () => ({ useKeyboardShortcuts: () => {} }));
vi.mock("@/hooks/use-open-project", () => ({ useOpenProject: () => vi.fn(async () => {}) }));
vi.mock("@/hooks/use-settings", () => ({
  loadSettingsFromStorage: vi.fn(async () => ({ theme: "dark" })),
  useAppSettings: () => ({
    settings: { theme: "dark" },
    isLoading: false,
    updateSettings: vi.fn(),
  }),
}));
vi.mock("@/hooks/use-stable-event", () => ({
  useStableEvent: (fn: (...args: unknown[]) => unknown) => fn,
}));
vi.mock("@/hooks/use-workspace-navigation", () => ({ navigateToWorkspace: vi.fn() }));

// --- Keyboard ---
vi.mock("@/keyboard/keyboard-action-dispatcher", () => ({
  keyboardActionDispatcher: { dispatch: vi.fn() },
}));

// --- Polyfills ---
vi.mock("@/polyfills/crypto", () => ({ polyfillCrypto: vi.fn() }));

// --- Query Client ---
vi.mock("@/query/query-client", () => ({ queryClient: {} }));

// --- Host Runtime ---
vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => mockStore,
  useHostMutations: () => ({ upsertConnectionFromOfferUrl: vi.fn() }),
  useHostRuntimeClient: () => null,
  useHosts: () => [],
}));
vi.mock("@/runtime/host-runtime-bootstrap", () => ({
  initializeHostRuntime: vi.fn(async () => null),
}));

// --- Stores ---
vi.mock("@/stores/navigation-active-workspace-store", () => ({
  addBrowserActiveWorkspaceLocationListener: vi.fn(() => vi.fn()),
  getLastNavigationWorkspaceRouteSelection: () => null,
  hydrateLastNavigationWorkspaceRouteSelection: vi.fn(async () => {}),
  syncNavigationActiveWorkspace: vi.fn(),
}));
vi.mock("@/stores/panel-store", () => {
  const usePanelStore = (selector: (state: typeof mockPanelState) => unknown) =>
    selector(mockPanelState);
  usePanelStore.getState = () => mockPanelState;
  return { usePanelStore };
});
vi.mock("@/stores/session-store", () => {
  const useSessionStore = (selector: (state: { sessions: Record<string, never> }) => unknown) =>
    selector({ sessions: {} });
  useSessionStore.getState = () => ({ sessions: {} });
  return { useSessionStore };
});

// --- Styles/Theme ---
vi.mock("@/styles/theme", () => ({ THEME_TO_UNISTYLES: {} }));

// --- Utils ---
vi.mock("@/utils/active-host", () => ({ resolveActiveHost: () => null }));
vi.mock("@/utils/desktop-sidebar-toggle", () => ({
  toggleDesktopSidebarsWithCheckoutIntent: vi.fn(),
}));
vi.mock("@/utils/host-routes", () => ({
  buildHostRootRoute: (serverId: string) => `/h/${serverId}`,
  mapPathnameToServer: (pathname: string) => pathname,
  parseHostAgentRouteFromPathname: () => null,
  parseServerIdFromPathname: () => null,
  parseWorkspaceOpenIntent: () => null,
}));
vi.mock("@/utils/notification-routing", () => ({
  buildNotificationRoute: () => "/",
  resolveNotificationTarget: () => ({ serverId: null, agentId: null, workspaceId: null }),
}));
vi.mock("@/utils/os-notifications", () => ({
  ensureOsNotificationPermission: vi.fn(async () => {}),
  WEB_NOTIFICATION_CLICK_EVENT: "web-notification-click",
}));
vi.mock("@/utils/workspace-execution", () => ({
  resolveWorkspaceIdByExecutionDirectory: () => null,
}));
vi.mock("@/utils/workspace-navigation", () => ({ prepareWorkspaceTab: vi.fn() }));

// Import after mocks are set up
import RootLayout from "./_layout";

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
  throwFlag.current = false;
});

afterEach(async () => {
  // Flush pending async work (e.g. initializeHostRuntime promise) before unmounting
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  if (root) {
    act(() => {
      root?.unmount();
    });
  }
  root = null;
  container = null;
  vi.unstubAllGlobals();
});

describe("RootLayout", () => {
  it("renders without error fallback when AppShell does not throw", () => {
    throwFlag.current = false;

    act(() => {
      root?.render(<RootLayout />);
    });

    expect(container?.textContent).not.toContain("The app encountered an error");
  });

  it("shows ErrorBoundary fallback when AppShell throws", () => {
    throwFlag.current = true;

    act(() => {
      root?.render(<RootLayout />);
    });

    expect(container?.textContent).toContain("The app encountered an error");
    expect(container?.textContent).toContain("Retry");
  });

  it("recovers when Retry is pressed after the error is resolved", () => {
    throwFlag.current = true;

    act(() => {
      root?.render(<RootLayout />);
    });

    expect(container?.textContent).toContain("The app encountered an error");

    // Resolve the error condition
    throwFlag.current = false;

    // Find the Retry button (Pressable renders as a clickable element in react-native-web)
    const allElements = container?.querySelectorAll("*") ?? [];
    let retryElement: Element | null = null;
    for (const el of allElements) {
      if (el.textContent === "Retry" && el.children.length === 0) {
        retryElement = el;
        break;
      }
    }

    expect(retryElement).not.toBeNull();

    // Click the Retry element (or its parent Pressable)
    const clickTarget = retryElement?.closest("[onClick]") ?? retryElement?.parentElement ?? retryElement;
    act(() => {
      clickTarget?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    expect(container?.textContent).not.toContain("The app encountered an error");
  });
});
