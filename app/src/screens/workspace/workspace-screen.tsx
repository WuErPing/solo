import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useStoreWithEqualityFn } from "zustand/traditional";
import { BackHandler, Pressable, Text, View } from "react-native";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as Clipboard from "expo-clipboard";
import { GestureDetector } from "react-native-gesture-handler";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import invariant from "tiny-invariant";
import { SidebarMenuToggle } from "@/components/headers/menu-header";
import { HeaderToggleButton } from "@/components/headers/header-toggle-button";
import { ScreenHeader } from "@/components/headers/screen-header";
import { Shortcut } from "@/components/ui/shortcut";
import type { ShortcutKey } from "@/utils/format-shortcut";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ExplorerSidebar } from "@/components/explorer-sidebar";
import { SplitContainer } from "@/components/split-container";
import { WorkspaceGitActions } from "@/screens/workspace/workspace-git-actions";
import { WorkspaceOpenInEditorButton } from "@/screens/workspace/workspace-open-in-editor-button";
import { WorkspaceScriptsButton } from "@/screens/workspace/workspace-scripts-button";
import { ExplorerSidebarAnimationProvider } from "@/contexts/explorer-sidebar-animation-context";
import { useToast } from "@/contexts/toast-context";
import { useExplorerOpenGesture } from "@/hooks/use-explorer-open-gesture";
import { selectIsFileExplorerOpen, usePanelStore } from "@/stores/panel-store";
import { type ExplorerCheckoutContext } from "@/stores/explorer-checkout-context";
import { useSessionStore } from "@/stores/session-store";
import {
  buildWorkspaceTabPersistenceKey,
  collectAllTabs,
  useWorkspaceLayoutStore,
  useWorkspaceLayoutStoreHydrated,
} from "@/stores/workspace-layout-store";
import type { WorkspaceTab, WorkspaceTabTarget } from "@/stores/workspace-tabs-store";
import { useKeyboardActionHandler } from "@/hooks/use-keyboard-action-handler";
import type { KeyboardActionDefinition } from "@/keyboard/keyboard-action-dispatcher";
import { useCreateFlowStore } from "@/stores/create-flow-store";
import { normalizeWorkspaceTabTarget } from "@/utils/workspace-tab-identity";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";
import { useProvidersSnapshot } from "@/hooks/use-providers-snapshot";
import { shouldShowWorkspaceSetup, useWorkspaceSetupStore } from "@/stores/workspace-setup-store";
import { useWorkspace } from "@/stores/session-store-hooks";
import { useWorkspaceTerminalSessionRetention } from "@/terminal/hooks/use-workspace-terminal-session-retention";
import {
  checkoutStatusQueryKey,
  type CheckoutStatusPayload,
} from "@/hooks/use-checkout-status-query";
import { upsertTerminalListEntry } from "@/utils/terminal-list";
import { confirmDialog } from "@/utils/confirm-dialog";
import { useArchiveAgent } from "@/hooks/use-archive-agent";
import { useStableEvent } from "@/hooks/use-stable-event";
import { buildProviderCommand } from "@/utils/provider-command-templates";
import { generateDraftId } from "@/stores/draft-keys";
import {
  getWorkspaceExecutionAuthority,
  resolveWorkspaceRouteId,
} from "@/utils/workspace-execution";
import { WorkspaceTabPresentationResolver } from "@/screens/workspace/workspace-tab-presentation";
import {
  WorkspaceDesktopTabsRow,
  type WorkspaceDesktopTabRowItem,
} from "@/screens/workspace/workspace-desktop-tabs-row";
import type { WorkspaceTabDescriptor } from "@/screens/workspace/workspace-tabs-types";
import { shouldRenderMissingWorkspaceDescriptor } from "@/screens/workspace/workspace-header-source";
import {
  deriveWorkspaceAgentVisibility,
  workspaceAgentVisibilityEqual,
} from "@/screens/workspace/workspace-agent-visibility";
import { deriveWorkspacePaneState } from "@/screens/workspace/workspace-pane-state";
import {
  buildWorkspacePaneContentModel,
  type WorkspacePaneContentModel,
} from "@/screens/workspace/workspace-pane-content";
import { useMountedTabSet } from "@/screens/workspace/use-mounted-tab-set";
import { shouldSeedEmptyWorkspaceDraft } from "@/screens/workspace/workspace-empty-draft-seed";
import {
  buildBulkCloseConfirmationMessage,
  classifyBulkClosableTabs,
  closeBulkWorkspaceTabs,
} from "@/screens/workspace/workspace-bulk-close";
import { findAdjacentPane } from "@/utils/split-navigation";
import { useIsCompactFormFactor, supportsDesktopPaneSplits } from "@/constants/layout";
import { isWeb, isNative } from "@/constants/platform";
import { useContainerWidthBelow } from "@/hooks/use-container-width";
import {
  MENU_COPY_ICON,
  MENU_NEW_AGENT_ICON,
  MENU_NEW_TERMINAL_ICON,
  MENU_SETTINGS_ICON,
  ThemedActivityIndicator,
  ThemedPanelRight,
  ThemedSourceControlPanelIcon,
  foregroundColorMapping,
  mutedColorMapping,
  sourceControlPanelStrokeWidth15,
} from "@/screens/workspace/workspace-themed-icons";
import {
  containerWithWorkspaceBackgroundStyle,
  styles,
} from "@/screens/workspace/workspace-screen-styles";
import {
  MobileMountedTabSlot,
  MobileWorkspaceTabSwitcher,
  WorkspaceDocumentTitleEffect,
  getFallbackTabOptionDescription,
  getFallbackTabOptionLabel,
  noop,
  useStableTabDescriptorMap,
} from "@/screens/workspace/workspace-mobile-tab-switcher";
import { WorkspaceHeaderTitleBar } from "@/screens/workspace/workspace-header";
import {
  buildWorkspaceHeaderCheckoutState,
  decodeSegment,
  deriveWorkspaceHeaderFields,
  parsePaneDirection,
  reconcilePendingScriptTerminals,
  removeTerminalFromPayload,
  resolveWorkspaceAuthorityState,
  trimNonEmpty,
  type ListTerminalsPayload,
} from "@/screens/workspace/workspace-screen-helpers";

const TERMINALS_QUERY_STALE_TIME = 5_000;
const WORKSPACE_SETUP_AUTO_OPEN_WINDOW_MS = 30_000;
const EMPTY_UI_TABS: WorkspaceTab[] = [];
const EMPTY_PINNED_AGENT_IDS = new Set<string>();
const EMPTY_SET = new Set<string>();

interface WorkspaceScreenProps {
  serverId: string;
  workspaceId: string;
  isRouteFocused: boolean;
}

type WorkspaceScreenContentProps = WorkspaceScreenProps & {
  isRouteFocused: boolean;
};

export function WorkspaceScreen({ serverId, workspaceId, isRouteFocused }: WorkspaceScreenProps) {
  return (
    <ExplorerSidebarAnimationProvider>
      <WorkspaceScreenContent
        serverId={serverId}
        workspaceId={workspaceId}
        isRouteFocused={isRouteFocused}
      />
    </ExplorerSidebarAnimationProvider>
  );
}

interface UseCloseTabsResult {
  closingTabIds: Set<string>;
  closeTab: (tabId: string, action: () => Promise<void>) => Promise<void>;
}

function useCloseTabs(): UseCloseTabsResult {
  const pendingRef = useRef(new Set<string>());
  const [closingTabIds, setClosingTabIds] = useState<Set<string>>(EMPTY_SET);

  const closeTab = useCallback(async (tabId: string, action: () => Promise<void>) => {
    const normalized = tabId.trim();
    if (!normalized || pendingRef.current.has(normalized)) {
      return;
    }
    pendingRef.current.add(normalized);
    setClosingTabIds(new Set(pendingRef.current));
    try {
      await action();
    } finally {
      pendingRef.current.delete(normalized);
      setClosingTabIds(new Set(pendingRef.current));
    }
  }, []);

  return { closingTabIds, closeTab };
}

interface RenderWorkspaceContentInput {
  showMissingWorkspaceDescriptor: boolean;
  isMissingWorkspaceExecutionAuthority: boolean;
  activeTabDescriptor: WorkspaceTabDescriptor | null;
  hasHydratedAgents: boolean;
  mountedFocusedPaneTabIds: string[];
  focusedPaneTabDescriptorMap: Map<string, WorkspaceTabDescriptor>;
  isRouteFocused: boolean;
  focusedPaneId: string | null;
  buildMobilePaneContentModel: (input: {
    paneId: string | null;
    tab: WorkspaceTabDescriptor;
  }) => WorkspacePaneContentModel;
}

function renderWorkspaceContent(input: RenderWorkspaceContentInput): React.ReactNode {
  const {
    showMissingWorkspaceDescriptor,
    isMissingWorkspaceExecutionAuthority,
    activeTabDescriptor,
    hasHydratedAgents,
    mountedFocusedPaneTabIds,
    focusedPaneTabDescriptorMap,
    isRouteFocused,
    focusedPaneId,
    buildMobilePaneContentModel,
  } = input;

  if (showMissingWorkspaceDescriptor) {
    return (
      <View style={styles.emptyState}>
        <ThemedActivityIndicator uniProps={mutedColorMapping} />
      </View>
    );
  }
  if (isMissingWorkspaceExecutionAuthority) {
    return (
      <View style={styles.emptyState}>
        <Text style={styles.emptyStateText}>
          Workspace execution directory is missing. Reload workspace data before opening tabs.
        </Text>
      </View>
    );
  }
  if (!activeTabDescriptor && !hasHydratedAgents) {
    return (
      <View style={styles.emptyState}>
        <ThemedActivityIndicator uniProps={mutedColorMapping} />
      </View>
    );
  }
  if (!activeTabDescriptor) {
    return (
      <View style={styles.emptyState}>
        <Text style={styles.emptyStateText}>
          No tabs are available yet. Use New tab to create an agent or terminal.
        </Text>
      </View>
    );
  }
  return mountedFocusedPaneTabIds.map((tabId) => {
    const tabDescriptor = focusedPaneTabDescriptorMap.get(tabId);
    if (!tabDescriptor) {
      return null;
    }
    return (
      <MobileMountedTabSlot
        key={tabId}
        tabDescriptor={tabDescriptor}
        isVisible={isRouteFocused && tabId === activeTabDescriptor.tabId}
        isWorkspaceFocused={isRouteFocused}
        isPaneFocused={tabId === activeTabDescriptor.tabId}
        paneId={focusedPaneId}
        buildPaneContentModel={buildMobilePaneContentModel}
      />
    );
  });
}

function WorkspaceScreenContent({
  serverId,
  workspaceId,
  isRouteFocused,
}: WorkspaceScreenContentProps) {
  const _insets = useSafeAreaInsets();
  const toast = useToast();
  const isMobile = useIsCompactFormFactor();
  const isFocusModeEnabled = usePanelStore((state) => state.desktop.focusModeEnabled);

  const normalizedServerId = useMemo(() => trimNonEmpty(decodeSegment(serverId)) ?? "", [serverId]);

  const normalizedWorkspaceId = useMemo(
    () => resolveWorkspaceRouteId({ routeWorkspaceId: workspaceId }) ?? "",
    [workspaceId],
  );
  const workspaceDescriptor = useWorkspace(normalizedServerId, normalizedWorkspaceId);

  const workspaceTerminalScopeKey = useMemo(
    () =>
      normalizedServerId && normalizedWorkspaceId
        ? `${normalizedServerId}:${normalizedWorkspaceId}`
        : null,
    [normalizedServerId, normalizedWorkspaceId],
  );
  useWorkspaceTerminalSessionRetention({
    scopeKey: workspaceTerminalScopeKey,
  });

  const queryClient = useQueryClient();
  const client = useHostRuntimeClient(normalizedServerId);
  const isConnected = useHostRuntimeIsConnected(normalizedServerId);
  const workspaceAuthority = useMemo(
    () =>
      getWorkspaceExecutionAuthority({
        workspace: workspaceDescriptor,
      }),
    [workspaceDescriptor],
  );
  const { workspaceDirectory, isMissingWorkspaceExecutionAuthority } =
    resolveWorkspaceAuthorityState(workspaceAuthority, workspaceDescriptor);

  // Warm the global provider snapshot so the model picker is ready when opened.
  useProvidersSnapshot(normalizedServerId, {
    enabled: isRouteFocused,
  });
  const [pendingTerminalCreateInput, setPendingTerminalCreateInput] = useState<{
    paneId?: string;
  } | null>(null);
  const canCreateTerminalNow = useMemo(
    () => Boolean(isRouteFocused && client && isConnected && workspaceDirectory),
    [isRouteFocused, client, isConnected, workspaceDirectory],
  );

  const workspaceAgentVisibility = useStoreWithEqualityFn(
    useSessionStore,
    (state) =>
      deriveWorkspaceAgentVisibility({
        sessionAgents: state.sessions[normalizedServerId]?.agents,
        agentDetails: state.sessions[normalizedServerId]?.agentDetails,
        workspaceDirectory,
      }),
    workspaceAgentVisibilityEqual,
  );

  const terminalsQueryKey = useMemo(
    () => ["terminals", normalizedServerId, workspaceDirectory] as const,
    [normalizedServerId, workspaceDirectory],
  );
  const terminalsQuery = useQuery({
    queryKey: terminalsQueryKey,
    enabled: canCreateTerminalNow,
    queryFn: async () => {
      if (!client || !workspaceDirectory) {
        throw new Error("Host is not connected");
      }
      return await client.listTerminals(workspaceDirectory);
    },
    staleTime: TERMINALS_QUERY_STALE_TIME,
  });
  const terminals = useMemo(() => terminalsQuery.data?.terminals ?? [], [terminalsQuery.data]);
  const liveTerminalIds = useMemo(() => terminals.map((terminal) => terminal.id), [terminals]);
  const [pendingScriptTerminalIds, setPendingScriptTerminalIds] = useState<Map<string, number>>(
    () => new Map(),
  );
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset pending script terminals on workspace change
    setPendingScriptTerminalIds(new Map());
  }, [normalizedServerId, normalizedWorkspaceId]);
  const terminalsDataUpdatedAt = terminalsQuery.dataUpdatedAt;
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reconcile pending terminals from query data
    setPendingScriptTerminalIds(
      reconcilePendingScriptTerminals(liveTerminalIds, terminalsDataUpdatedAt),
    );
  }, [liveTerminalIds, terminalsDataUpdatedAt]);
  const knownTerminalIds = useMemo(() => {
    const terminalIds = new Set(liveTerminalIds);
    for (const terminalId of pendingScriptTerminalIds.keys()) {
      terminalIds.add(terminalId);
    }
    return Array.from(terminalIds);
  }, [liveTerminalIds, pendingScriptTerminalIds]);
  const scriptTerminalIds = useMemo(() => {
    const terminalIds = new Set(pendingScriptTerminalIds.keys());
    for (const script of workspaceDescriptor?.scripts ?? []) {
      if (script.terminalId) {
        terminalIds.add(script.terminalId);
      }
    }
    return terminalIds;
  }, [pendingScriptTerminalIds, workspaceDescriptor?.scripts]);
  const standaloneTerminalIds = useMemo(
    () =>
      terminals
        .filter((terminal) => !scriptTerminalIds.has(terminal.id))
        .map((terminal) => terminal.id),
    [scriptTerminalIds, terminals],
  );
  const createTerminalMutation = useMutation({
    mutationFn: async (_input?: { paneId?: string }) => {
      if (!client || !workspaceDirectory) {
        throw new Error("Host is not connected");
      }
      return await client.createTerminal(workspaceDirectory);
    },
    onSuccess: (payload, input) => {
      const createdTerminal = payload.terminal;
      if (createdTerminal) {
        queryClient.setQueryData<ListTerminalsPayload>(terminalsQueryKey, (current) => {
          const nextTerminals = upsertTerminalListEntry({
            terminals: current?.terminals ?? [],
            terminal: createdTerminal,
          });
          const cwd = current?.cwd ?? workspaceDirectory;
          return {
            ...(cwd ? { cwd } : {}),
            terminals: nextTerminals,
            requestId: current?.requestId ?? `terminal-create-${createdTerminal.id}`,
          };
        });
      }

      void queryClient.invalidateQueries({ queryKey: terminalsQueryKey });
      if (createdTerminal) {
        const workspaceKey = buildWorkspaceTabPersistenceKey({
          serverId: normalizedServerId,
          workspaceId: normalizedWorkspaceId,
        });
        if (!workspaceKey) {
          return;
        }
        if (input?.paneId) {
          focusWorkspacePane(workspaceKey, input.paneId);
        }
        useWorkspaceLayoutStore
          .getState()
          .openTabFocused(workspaceKey, { kind: "terminal", terminalId: createdTerminal.id });
      }
    },
  });
  const killTerminalMutation = useMutation({
    mutationFn: async (terminalId: string) => {
      if (!client) {
        throw new Error("Host is not connected");
      }
      const payload = await client.killTerminal(terminalId);
      if (!payload.success) {
        throw new Error("Unable to close terminal");
      }
      return payload;
    },
  });
  const { archiveAgent } = useArchiveAgent();

  useEffect(() => {
    if (!isRouteFocused || !client || !isConnected || !workspaceDirectory) {
      return;
    }

    const unsubscribeChanged = client.on("terminals_changed", (message) => {
      if (message.payload.cwd !== workspaceDirectory) {
        return;
      }

      queryClient.setQueryData<ListTerminalsPayload>(terminalsQueryKey, (current) => ({
        cwd: message.payload.cwd,
        terminals: message.payload.terminals,
        requestId: current?.requestId ?? `terminals-changed-${Date.now()}`,
      }));
    });

    client.subscribeTerminals({ cwd: workspaceDirectory });

    return () => {
      unsubscribeChanged();
      client.unsubscribeTerminals({ cwd: workspaceDirectory });
    };
  }, [client, isConnected, isRouteFocused, queryClient, terminalsQueryKey, workspaceDirectory]);

  const isCheckoutQueryEnabled = useMemo(
    () => Boolean(isRouteFocused && client && isConnected && workspaceDirectory),
    [isRouteFocused, client, isConnected, workspaceDirectory],
  );
  const checkoutQuery = useQuery({
    queryKey: checkoutStatusQueryKey(
      normalizedServerId,
      workspaceDirectory ?? `missing-workspace-directory:${normalizedWorkspaceId}`,
    ),
    enabled: isCheckoutQueryEnabled,
    queryFn: async () => {
      if (!client || !workspaceDirectory) {
        throw new Error("Host is not connected");
      }
      return (await client.getCheckoutStatus(workspaceDirectory)) as CheckoutStatusPayload;
    },
    staleTime: Infinity,
    refetchOnMount: false,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
  });
  const isCheckoutStatusLoading = useMemo(
    () => isCheckoutQueryEnabled && checkoutQuery.data === undefined && !checkoutQuery.isError,
    [isCheckoutQueryEnabled, checkoutQuery.data, checkoutQuery.isError],
  );
  const hasHydratedWorkspaces = useSessionStore(
    (state) => state.sessions[normalizedServerId]?.hasHydratedWorkspaces ?? false,
  );
  const hasHydratedAgents = useSessionStore(
    (state) => state.sessions[normalizedServerId]?.hasHydratedAgents ?? false,
  );
  useEffect(() => {
    if (!pendingTerminalCreateInput) {
      return;
    }

    if (canCreateTerminalNow && !createTerminalMutation.isPending) {
      const pendingInput = pendingTerminalCreateInput;
      // eslint-disable-next-line react-hooks/set-state-in-effect -- consume pending terminal create request
      setPendingTerminalCreateInput(null);
      createTerminalMutation.mutate(pendingInput);
      return;
    }

    if (hasHydratedWorkspaces && isMissingWorkspaceExecutionAuthority) {
      setPendingTerminalCreateInput(null);
      toast.error("Workspace path is not available yet");
    }
  }, [
    canCreateTerminalNow,
    createTerminalMutation,
    hasHydratedWorkspaces,
    isMissingWorkspaceExecutionAuthority,
    pendingTerminalCreateInput,
    toast,
  ]);
  const workspaceHeaderCheckoutState = buildWorkspaceHeaderCheckoutState({
    isCheckoutStatusLoading,
    isError: checkoutQuery.isError,
    data: checkoutQuery.data,
  });
  const {
    isWorkspaceHeaderLoading,
    workspaceHeaderTitle,
    workspaceHeaderSubtitle,
    shouldShowWorkspaceHeaderSubtitle,
    isGitCheckout,
    currentBranchName,
  } = deriveWorkspaceHeaderFields({
    workspace: workspaceDescriptor,
    checkoutState: workspaceHeaderCheckoutState,
  });

  const isExplorerOpen = usePanelStore((state) =>
    selectIsFileExplorerOpen(state, { isCompact: isMobile }),
  );
  const canOpenExplorerFromAgentView = usePanelStore(
    (state) =>
      state.mobileView === "agent" && !selectIsFileExplorerOpen(state, { isCompact: true }),
  );
  const openFileExplorerForCheckout = usePanelStore((state) => state.openFileExplorerForCheckout);
  const toggleFileExplorerForCheckout = usePanelStore(
    (state) => state.toggleFileExplorerForCheckout,
  );
  const showMobileAgent = usePanelStore((state) => state.showMobileAgent);

  const activeExplorerCheckout = useMemo<ExplorerCheckoutContext | null>(() => {
    if (!normalizedServerId || !workspaceDirectory) {
      return null;
    }
    return {
      serverId: normalizedServerId,
      cwd: workspaceDirectory,
      isGit: isGitCheckout,
    };
  }, [isGitCheckout, normalizedServerId, workspaceDirectory]);

  const openExplorerForWorkspace = useCallback(() => {
    if (!activeExplorerCheckout) {
      return;
    }
    openFileExplorerForCheckout({
      isCompact: isMobile,
      checkout: activeExplorerCheckout,
    });
  }, [activeExplorerCheckout, isMobile, openFileExplorerForCheckout]);

  const handleToggleExplorer = useCallback(() => {
    if (!activeExplorerCheckout) {
      return;
    }
    toggleFileExplorerForCheckout({
      isCompact: isMobile,
      checkout: activeExplorerCheckout,
    });
  }, [activeExplorerCheckout, isMobile, toggleFileExplorerForCheckout]);

  const explorerToggleStyle = useCallback(
    ({ hovered, pressed }: { hovered?: boolean; pressed?: boolean }) => [
      styles.sourceControlButton,
      (Boolean(hovered) || Boolean(pressed) || isExplorerOpen) && styles.sourceControlButtonHovered,
    ],
    [isExplorerOpen],
  );
  const explorerToggleAccessibilityState = useMemo(
    () => ({ expanded: isExplorerOpen }),
    [isExplorerOpen],
  );

  const explorerOpenGesture = useExplorerOpenGesture({
    enabled: isMobile && canOpenExplorerFromAgentView,
    onOpen: openExplorerForWorkspace,
  });

  useEffect(() => {
    if (!isRouteFocused || isWeb || !isExplorerOpen) {
      return;
    }

    const handler = BackHandler.addEventListener("hardwareBackPress", () => {
      if (isExplorerOpen) {
        showMobileAgent();
        return true;
      }
      return false;
    });

    return () => handler.remove();
  }, [isExplorerOpen, isRouteFocused, showMobileAgent]);

  const persistenceKey = useMemo(
    () =>
      buildWorkspaceTabPersistenceKey({
        serverId: normalizedServerId,
        workspaceId: normalizedWorkspaceId,
      }),
    [normalizedServerId, normalizedWorkspaceId],
  );

  const workspaceLayout = useWorkspaceLayoutStore((state) =>
    persistenceKey ? (state.layoutByWorkspace[persistenceKey] ?? null) : null,
  );
  const hasHydratedWorkspaceLayoutStore = useWorkspaceLayoutStoreHydrated();
  const workspaceSetupSnapshot = useWorkspaceSetupStore((state) =>
    persistenceKey ? (state.snapshots[persistenceKey] ?? null) : null,
  );
  const upsertWorkspaceSetupProgress = useWorkspaceSetupStore((state) => state.upsertProgress);
  const showWorkspaceSetup = shouldShowWorkspaceSetup(workspaceSetupSnapshot);
  const uiTabs = useMemo(
    () => (workspaceLayout ? collectAllTabs(workspaceLayout.root) : EMPTY_UI_TABS),
    [workspaceLayout],
  );
  const openWorkspaceTabFocused = useWorkspaceLayoutStore((state) => state.openTabFocused);
  const openWorkspaceTabInBackground = useWorkspaceLayoutStore(
    (state) => state.openTabInBackground,
  );
  const focusWorkspaceTab = useWorkspaceLayoutStore((state) => state.focusTab);
  const closeWorkspaceTab = useWorkspaceLayoutStore((state) => state.closeTab);
  const unpinWorkspaceAgent = useWorkspaceLayoutStore((state) => state.unpinAgent);
  const hideWorkspaceAgent = useWorkspaceLayoutStore((state) => state.hideAgent);
  const retargetWorkspaceTab = useWorkspaceLayoutStore((state) => state.retargetTab);
  const convertWorkspaceDraftToAgent = useWorkspaceLayoutStore(
    (state) => state.convertDraftToAgent,
  );
  const reconcileWorkspaceTabs = useWorkspaceLayoutStore((state) => state.reconcileTabs);
  const splitWorkspacePane = useWorkspaceLayoutStore((state) => state.splitPane);
  const splitWorkspacePaneEmpty = useWorkspaceLayoutStore((state) => state.splitPaneEmpty);
  const moveWorkspaceTabToPane = useWorkspaceLayoutStore((state) => state.moveTabToPane);
  const focusWorkspacePane = useWorkspaceLayoutStore((state) => state.focusPane);
  const handleScriptTerminalStarted = useCallback(
    (terminalId: string) => {
      setPendingScriptTerminalIds((pendingTerminalIds) => {
        if (pendingTerminalIds.get(terminalId) === terminalsQuery.dataUpdatedAt) {
          return pendingTerminalIds;
        }
        const nextTerminalIds = new Map(pendingTerminalIds);
        nextTerminalIds.set(terminalId, terminalsQuery.dataUpdatedAt);
        return nextTerminalIds;
      });
      if (persistenceKey) {
        openWorkspaceTabFocused(persistenceKey, { kind: "terminal", terminalId });
      }
      void queryClient.invalidateQueries({ queryKey: terminalsQueryKey });
    },
    [
      openWorkspaceTabFocused,
      persistenceKey,
      queryClient,
      terminalsQuery.dataUpdatedAt,
      terminalsQueryKey,
    ],
  );
  const handleViewScriptTerminal = useCallback(
    (terminalId: string) => {
      if (!persistenceKey) {
        return;
      }
      openWorkspaceTabFocused(persistenceKey, { kind: "terminal", terminalId });
    },
    [openWorkspaceTabFocused, persistenceKey],
  );
  const paneFocusSuppressedRef = useRef(false);
  const resizeWorkspaceSplit = useWorkspaceLayoutStore((state) => state.resizeSplit);
  const reorderWorkspaceTabsInPane = useWorkspaceLayoutStore((state) => state.reorderTabsInPane);
  const _pinnedAgentIds = useWorkspaceLayoutStore((state) =>
    persistenceKey
      ? (state.pinnedAgentIdsByWorkspace[persistenceKey] ?? EMPTY_PINNED_AGENT_IDS)
      : EMPTY_PINNED_AGENT_IDS,
  );
  const _hiddenAgentIds = useWorkspaceLayoutStore((state) =>
    persistenceKey ? (state.hiddenAgentIdsByWorkspace[persistenceKey] ?? EMPTY_SET) : EMPTY_SET,
  );
  const pendingByDraftId = useCreateFlowStore((state) => state.pendingByDraftId);
  const { closingTabIds, closeTab } = useCloseTabs();
  const { onLayout: onHeaderLayout, isBelow: showCompactButtonLabels } =
    useContainerWidthBelow(700);
  const closeWorkspaceTabWithCleanup = useCallback(
    function closeWorkspaceTabWithCleanup(input: {
      tabId: string;
      target?: WorkspaceTabTarget | null;
    }) {
      const normalizedTabId = trimNonEmpty(input.tabId);
      if (!normalizedTabId || !persistenceKey) {
        return;
      }

      if (input.target?.kind === "agent") {
        unpinWorkspaceAgent(persistenceKey, input.target.agentId);
        hideWorkspaceAgent(persistenceKey, input.target.agentId);
      }
      closeWorkspaceTab(persistenceKey, normalizedTabId);
    },
    [closeWorkspaceTab, hideWorkspaceAgent, persistenceKey, unpinWorkspaceAgent],
  );

  const focusedPaneTabState = useMemo(
    () =>
      deriveWorkspacePaneState({
        layout: workspaceLayout,
        tabs: uiTabs,
      }),
    [uiTabs, workspaceLayout],
  );
  const setFocusedAgentId = useSessionStore((state) => state.setFocusedAgentId);
  const focusedPaneAgentId = useMemo(() => {
    const target = focusedPaneTabState.activeTab?.descriptor.target;
    if (target?.kind !== "agent") {
      return null;
    }
    return target.agentId;
  }, [focusedPaneTabState.activeTab]);

  useEffect(() => {
    if (!isRouteFocused) {
      return;
    }
    setFocusedAgentId(normalizedServerId, focusedPaneAgentId);
  }, [focusedPaneAgentId, isRouteFocused, normalizedServerId, setFocusedAgentId]);

  useEffect(() => {
    if (!isRouteFocused) {
      return;
    }
    return () => {
      setFocusedAgentId(normalizedServerId, null);
    };
  }, [isRouteFocused, normalizedServerId, setFocusedAgentId]);

  const openWorkspaceDraftTab = useCallback(
    function openWorkspaceDraftTab(input?: { draftId?: string; focus?: boolean }) {
      if (!persistenceKey) {
        return null;
      }

      const target = normalizeWorkspaceTabTarget({
        kind: "draft",
        draftId: trimNonEmpty(input?.draftId) ?? generateDraftId(),
      });
      invariant(target?.kind === "draft", "Draft tab target must be valid");
      if (input?.focus === false) {
        return openWorkspaceTabInBackground(persistenceKey, target);
      }
      return openWorkspaceTabFocused(persistenceKey, target);
    },
    [openWorkspaceTabFocused, openWorkspaceTabInBackground, persistenceKey],
  );

  useEffect(() => {
    if (!isRouteFocused) {
      return;
    }
    if (!normalizedServerId || !normalizedWorkspaceId || !persistenceKey) {
      return;
    }
    if (!hasHydratedWorkspaceLayoutStore) {
      return;
    }

    const hasActivePendingDraftCreateInWorkspace = uiTabs.some((tab) => {
      if (tab.target.kind !== "draft") {
        return false;
      }
      const pending = pendingByDraftId[tab.target.draftId];
      return pending?.serverId === normalizedServerId && pending.lifecycle === "active";
    });

    reconcileWorkspaceTabs(persistenceKey, {
      agentsHydrated: hasHydratedAgents,
      terminalsHydrated: terminalsQuery.isSuccess,
      activeAgentIds: Array.from(workspaceAgentVisibility.activeAgentIds),
      knownAgentIds: Array.from(workspaceAgentVisibility.knownAgentIds),
      knownTerminalIds,
      standaloneTerminalIds,
      hasActivePendingDraftCreate: hasActivePendingDraftCreateInWorkspace,
    });
  }, [
    hasHydratedAgents,
    hasHydratedWorkspaceLayoutStore,
    isRouteFocused,
    normalizedServerId,
    normalizedWorkspaceId,
    pendingByDraftId,
    persistenceKey,
    reconcileWorkspaceTabs,
    knownTerminalIds,
    standaloneTerminalIds,
    terminalsQuery.isSuccess,
    uiTabs,
    workspaceAgentVisibility,
  ]);

  const activeTabId = focusedPaneTabState.activeTabId;
  const activeTab = focusedPaneTabState.activeTab;

  const tabs = useMemo<WorkspaceTabDescriptor[]>(
    () => focusedPaneTabState.tabs.map((tab) => tab.descriptor),
    [focusedPaneTabState.tabs],
  );
  const hasSetupTab = useMemo(
    () =>
      uiTabs.some(
        (tab) => tab.target.kind === "setup" && tab.target.workspaceId === normalizedWorkspaceId,
      ),
    [normalizedWorkspaceId, uiTabs],
  );

  const navigateToTabId = useCallback(
    function navigateToTabId(tabId: string) {
      if (!tabId || !persistenceKey) {
        return;
      }
      focusWorkspaceTab(persistenceKey, tabId);
    },
    [focusWorkspaceTab, persistenceKey],
  );

  const emptyWorkspaceSeedRef = useRef<string | null>(null);
  const autoOpenedSetupTabWorkspaceRef = useRef<string | null>(null);
  const requestedWorkspaceSetupStatusKeyRef = useRef<string | null>(null);

  useEffect(() => {
    if (!isRouteFocused) {
      return;
    }
    if (!client || !normalizedServerId || !normalizedWorkspaceId || !persistenceKey) {
      return;
    }
    if (workspaceSetupSnapshot) {
      return;
    }
    if (requestedWorkspaceSetupStatusKeyRef.current === persistenceKey) {
      return;
    }

    requestedWorkspaceSetupStatusKeyRef.current = persistenceKey;
    let isCancelled = false;

    client
      .fetchWorkspaceSetupStatus(normalizedWorkspaceId)
      .then((response) => {
        if (isCancelled || response.workspaceId !== normalizedWorkspaceId || !response.snapshot) {
          return;
        }
        upsertWorkspaceSetupProgress({
          serverId: normalizedServerId,
          payload: { workspaceId: response.workspaceId, ...response.snapshot },
        });
        return;
      })
      .catch(() => {
        if (requestedWorkspaceSetupStatusKeyRef.current === persistenceKey) {
          requestedWorkspaceSetupStatusKeyRef.current = null;
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [
    client,
    isRouteFocused,
    normalizedServerId,
    normalizedWorkspaceId,
    persistenceKey,
    upsertWorkspaceSetupProgress,
    workspaceSetupSnapshot,
  ]);

  useEffect(() => {
    if (
      !shouldSeedEmptyWorkspaceDraft({
        isRouteFocused,
        hasPersistenceKey: Boolean(persistenceKey),
        hasWorkspaceDirectory: Boolean(workspaceDirectory),
        hasHydratedWorkspaceLayoutStore,
        hasHydratedAgents,
        hasLoadedTerminals: terminalsQuery.isSuccess,
        activeAgentCount: workspaceAgentVisibility.activeAgentIds.size,
        terminalCount: terminals.length,
        tabCount: tabs.length,
      })
    ) {
      emptyWorkspaceSeedRef.current = null;
      return;
    }
    const workspaceKey = `${normalizedServerId}:${normalizedWorkspaceId}`;
    if (emptyWorkspaceSeedRef.current === workspaceKey) {
      return;
    }
    emptyWorkspaceSeedRef.current = workspaceKey;
    openWorkspaceDraftTab();
  }, [
    normalizedServerId,
    normalizedWorkspaceId,
    openWorkspaceDraftTab,
    persistenceKey,
    hasHydratedAgents,
    hasHydratedWorkspaceLayoutStore,
    isRouteFocused,
    terminals.length,
    terminalsQuery.isSuccess,
    tabs.length,
    workspaceDirectory,
    workspaceAgentVisibility.activeAgentIds.size,
  ]);

  useEffect(() => {
    if (!isRouteFocused) {
      return;
    }
    if (!persistenceKey) {
      return;
    }
    if (!workspaceSetupSnapshot || !showWorkspaceSetup) {
      if (autoOpenedSetupTabWorkspaceRef.current === persistenceKey) {
        autoOpenedSetupTabWorkspaceRef.current = null;
      }
      return;
    }

    const snapshotAge = Date.now() - workspaceSetupSnapshot.updatedAt;
    const shouldAutoOpen =
      workspaceSetupSnapshot.status === "running" ||
      snapshotAge <= WORKSPACE_SETUP_AUTO_OPEN_WINDOW_MS;
    if (!shouldAutoOpen) {
      return;
    }
    if (hasSetupTab) {
      autoOpenedSetupTabWorkspaceRef.current = persistenceKey;
      return;
    }
    if (autoOpenedSetupTabWorkspaceRef.current === persistenceKey) {
      return;
    }

    const target = normalizeWorkspaceTabTarget({
      kind: "setup",
      workspaceId: normalizedWorkspaceId,
    });
    if (!target) {
      return;
    }

    const tabId = openWorkspaceTabInBackground(persistenceKey, target);
    if (!tabId) {
      return;
    }

    autoOpenedSetupTabWorkspaceRef.current = persistenceKey;
  }, [
    hasSetupTab,
    isRouteFocused,
    normalizedWorkspaceId,
    openWorkspaceTabInBackground,
    persistenceKey,
    showWorkspaceSetup,
    workspaceSetupSnapshot,
  ]);

  const handleOpenFileFromExplorer = useCallback(
    function handleOpenFileFromExplorer(filePath: string) {
      if (isMobile) {
        showMobileAgent();
      }
      if (!persistenceKey) {
        return;
      }
      const tabId = openWorkspaceTabFocused(persistenceKey, { kind: "file", path: filePath });
      if (tabId) {
        navigateToTabId(tabId);
      }
    },
    [isMobile, navigateToTabId, openWorkspaceTabFocused, persistenceKey, showMobileAgent],
  );

  const handleOpenFileFromChat = useCallback(
    ({ filePath }: { filePath: string }) => {
      const normalizedFilePath = filePath.trim();
      if (!normalizedFilePath) {
        return;
      }
      handleOpenFileFromExplorer(normalizedFilePath);
    },
    [handleOpenFileFromExplorer],
  );

  const [_hoveredTabKey, setHoveredTabKey] = useState<string | null>(null);
  const [hoveredCloseTabKey, setHoveredCloseTabKey] = useState<string | null>(null);

  const tabByKey = useMemo(() => {
    const map = new Map<string, WorkspaceTabDescriptor>();
    for (const tab of tabs) {
      map.set(tab.key, tab);
    }
    return map;
  }, [tabs]);

  const allTabDescriptorsById = useMemo(() => {
    const map = new Map<string, WorkspaceTabDescriptor>();
    for (const tab of uiTabs) {
      map.set(tab.tabId, {
        key: tab.tabId,
        tabId: tab.tabId,
        kind: tab.target.kind,
        target: tab.target,
      });
    }
    return map;
  }, [uiTabs]);

  const activeTabKey = useMemo(() => activeTabId ?? "", [activeTabId]);

  const tabSwitcherOptions = useMemo(
    () =>
      tabs.map((tab) => ({
        id: tab.key,
        label: getFallbackTabOptionLabel(tab),
        description: getFallbackTabOptionDescription(tab),
      })),
    [tabs],
  );

  const handleCreateDraftTab = useCallback(
    (input?: { paneId?: string }) => {
      if (input?.paneId && persistenceKey) {
        focusWorkspacePane(persistenceKey, input.paneId);
      }
      openWorkspaceDraftTab();
    },
    [focusWorkspacePane, openWorkspaceDraftTab, persistenceKey],
  );

  const handleCreateTerminal = useStableEvent((input?: { paneId?: string }) => {
    if (createTerminalMutation.isPending || pendingTerminalCreateInput) {
      return;
    }

    if (canCreateTerminalNow) {
      createTerminalMutation.mutate(input);
      return;
    }

    if (hasHydratedWorkspaces && isMissingWorkspaceExecutionAuthority) {
      toast.error("Workspace path is not available yet");
      return;
    }

    setPendingTerminalCreateInput(input ?? {});
    toast.show("Preparing workspace, opening terminal when ready...");
  });

  const handleSelectSwitcherTab = useCallback(
    (key: string) => {
      navigateToTabId(key);
    },
    [navigateToTabId],
  );

  const handleCreateDraftSplit = useCallback(
    (input: { targetPaneId: string; position: "left" | "right" | "top" | "bottom" }) => {
      if (!persistenceKey) {
        return;
      }

      const paneId = splitWorkspacePaneEmpty(persistenceKey, input);
      if (!paneId) {
        return;
      }

      handleCreateDraftTab({ paneId });
    },
    [handleCreateDraftTab, persistenceKey, splitWorkspacePaneEmpty],
  );

  const killTerminalAsync = killTerminalMutation.mutateAsync;

  const handleCloseTerminalTab = useCallback(
    async (input: { tabId: string; terminalId: string }) => {
      const { tabId, terminalId } = input;
      await closeTab(tabId, async () => {
        const confirmed = await confirmDialog({
          title: "Close terminal?",
          message: "Any running process in this terminal will be stopped immediately.",
          confirmLabel: "Close",
          cancelLabel: "Cancel",
          destructive: true,
        });
        if (!confirmed) {
          return;
        }

        queryClient.setQueryData<ListTerminalsPayload>(
          terminalsQueryKey,
          removeTerminalFromPayload(terminalId),
        );
        setHoveredTabKey((current) => (current === tabId ? null : current));
        setHoveredCloseTabKey((current) => (current === tabId ? null : current));
        if (persistenceKey) {
          closeWorkspaceTabWithCleanup({
            tabId,
            target: { kind: "terminal", terminalId },
          });
        }

        void killTerminalAsync(terminalId).catch(() => {
          void queryClient.invalidateQueries({ queryKey: terminalsQueryKey });
        });
      });
    },
    [
      closeTab,
      closeWorkspaceTabWithCleanup,
      killTerminalAsync,
      persistenceKey,
      queryClient,
      terminalsQueryKey,
    ],
  );

  const handleCloseAgentTab = useCallback(
    async (input: { tabId: string; agentId: string }) => {
      const { tabId, agentId } = input;
      await closeTab(tabId, async () => {
        if (!normalizedServerId) {
          return;
        }

        const agent =
          useSessionStore.getState().sessions[normalizedServerId]?.agents?.get(agentId) ?? null;
        const isRunning = agent?.status === "running" || agent?.status === "initializing";

        if (isRunning) {
          const confirmed = await confirmDialog({
            title: "Archive running agent?",
            message:
              "This agent is still running. Archiving it will stop the agent and close the tab.",
            confirmLabel: "Archive",
            cancelLabel: "Cancel",
            destructive: true,
          });
          if (!confirmed) {
            return;
          }
        }

        setHoveredTabKey((current) => (current === tabId ? null : current));
        setHoveredCloseTabKey((current) => (current === tabId ? null : current));
        if (persistenceKey) {
          closeWorkspaceTabWithCleanup({
            tabId,
            target: { kind: "agent", agentId },
          });
        }

        // Errors (e.g. timeout) are handled by the mutation's onSettled callback
        void archiveAgent({ serverId: normalizedServerId, agentId }).catch(() => {});
      });
    },
    [archiveAgent, closeTab, closeWorkspaceTabWithCleanup, normalizedServerId, persistenceKey],
  );

  const handleCloseDraftOrFileTab = useCallback(
    function handleCloseDraftOrFileTab(tabId: string) {
      setHoveredTabKey((current) => (current === tabId ? null : current));
      setHoveredCloseTabKey((current) => (current === tabId ? null : current));
      if (persistenceKey) {
        closeWorkspaceTabWithCleanup({ tabId });
      }
    },
    [closeWorkspaceTabWithCleanup, persistenceKey],
  );

  const handleCloseTabById = useCallback(
    async (tabId: string) => {
      const tab = allTabDescriptorsById.get(tabId);
      if (!tab) {
        return;
      }
      if (tab.target.kind === "terminal") {
        await handleCloseTerminalTab({ tabId, terminalId: tab.target.terminalId });
        return;
      }
      if (tab.target.kind === "agent") {
        await handleCloseAgentTab({ tabId, agentId: tab.target.agentId });
        return;
      }
      handleCloseDraftOrFileTab(tabId);
    },
    [allTabDescriptorsById, handleCloseAgentTab, handleCloseDraftOrFileTab, handleCloseTerminalTab],
  );

  const handleCopyAgentId = useCallback(
    async (agentId: string) => {
      if (!agentId) return;
      try {
        await Clipboard.setStringAsync(agentId);
        toast.copied("Agent ID");
      } catch {
        toast.error("Copy failed");
      }
    },
    [toast],
  );

  const handleCopyResumeCommand = useCallback(
    async (agentId: string) => {
      if (!agentId) return;
      const agent =
        useSessionStore.getState().sessions[normalizedServerId]?.agents?.get(agentId) ?? null;
      const providerSessionId =
        agent?.runtimeInfo?.sessionId ?? agent?.persistence?.sessionId ?? null;
      if (!agent || !providerSessionId) {
        toast.error("Resume ID not available");
        return;
      }

      const command =
        buildProviderCommand({
          provider: agent.provider,
          id: "resume",
          sessionId: providerSessionId,
        }) ?? null;
      if (!command) {
        toast.show("Resume command not available");
        return;
      }
      try {
        await Clipboard.setStringAsync(command);
        toast.copied("resume command");
      } catch {
        toast.error("Copy failed");
      }
    },
    [normalizedServerId, toast],
  );

  const handleReloadAgent = useCallback(
    async (agentId: string) => {
      if (!client || !isConnected) {
        toast.error("Host is not connected");
        return;
      }

      try {
        await client.refreshAgent(agentId);
        toast.show("Reloaded agent", { variant: "success" });
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "Failed to reload agent");
      }
    },
    [client, isConnected, toast],
  );

  const handleCopyWorkspacePath = useCallback(async () => {
    if (!workspaceDirectory) {
      toast.error("Workspace path not available");
      return;
    }

    try {
      await Clipboard.setStringAsync(workspaceDirectory);
      toast.copied("Workspace path");
    } catch {
      toast.error("Copy failed");
    }
  }, [toast, workspaceDirectory]);

  const handleCopyBranchName = useCallback(async () => {
    if (!currentBranchName) {
      toast.error("Branch name not available");
      return;
    }

    try {
      await Clipboard.setStringAsync(currentBranchName);
      toast.copied("Branch name");
    } catch {
      toast.error("Copy failed");
    }
  }, [currentBranchName, toast]);

  const handleOpenSetupTab = useCallback(() => {
    if (!persistenceKey) {
      return;
    }
    const target = normalizeWorkspaceTabTarget({
      kind: "setup",
      workspaceId: normalizedWorkspaceId,
    });
    if (!target) {
      return;
    }
    openWorkspaceTabFocused(persistenceKey, target);
  }, [normalizedWorkspaceId, openWorkspaceTabFocused, persistenceKey]);

  const handleBulkCloseTabs = useCallback(
    async (input: { tabsToClose: WorkspaceTabDescriptor[]; title: string; logLabel: string }) => {
      const { tabsToClose, title, logLabel } = input;
      if (tabsToClose.length === 0) {
        return;
      }

      const groups = classifyBulkClosableTabs(tabsToClose);
      const confirmed = await confirmDialog({
        title,
        message: buildBulkCloseConfirmationMessage(groups),
        confirmLabel: "Close",
        cancelLabel: "Cancel",
        destructive: true,
      });
      if (!confirmed) {
        return;
      }

      await closeBulkWorkspaceTabs({
        client,
        groups,
        closeTab,
        closeWorkspaceTabWithCleanup: (cleanupInput) => {
          if (!persistenceKey) {
            return;
          }
          closeWorkspaceTabWithCleanup(cleanupInput);
        },
        logLabel,
        warn: (message, payload) => {
          console.warn(message, payload);
        },
      });

      const closedKeys = new Set(tabsToClose.map((tab) => tab.key));
      setHoveredTabKey((current) => (current && closedKeys.has(current) ? null : current));
      setHoveredCloseTabKey((current) => (current && closedKeys.has(current) ? null : current));
    },
    [client, closeTab, closeWorkspaceTabWithCleanup, persistenceKey],
  );

  const handleCloseTabsToLeftInPane = useCallback(
    async (tabId: string, paneTabs: WorkspaceTabDescriptor[]) => {
      const index = paneTabs.findIndex((tab) => tab.tabId === tabId);
      if (index < 0) {
        return;
      }
      await handleBulkCloseTabs({
        tabsToClose: paneTabs.slice(0, index),
        title: "Close tabs to the left?",
        logLabel: "to the left",
      });
    },
    [handleBulkCloseTabs],
  );

  const handleCloseTabsToLeft = useCallback(
    async (tabId: string) => {
      await handleCloseTabsToLeftInPane(tabId, tabs);
    },
    [handleCloseTabsToLeftInPane, tabs],
  );

  const handleCloseTabsToRightInPane = useCallback(
    async (tabId: string, paneTabs: WorkspaceTabDescriptor[]) => {
      const index = paneTabs.findIndex((tab) => tab.tabId === tabId);
      if (index < 0) {
        return;
      }
      await handleBulkCloseTabs({
        tabsToClose: paneTabs.slice(index + 1),
        title: "Close tabs to the right?",
        logLabel: "to the right",
      });
    },
    [handleBulkCloseTabs],
  );

  const handleCloseTabsToRight = useCallback(
    async (tabId: string) => {
      await handleCloseTabsToRightInPane(tabId, tabs);
    },
    [handleCloseTabsToRightInPane, tabs],
  );

  const handleCloseOtherTabsInPane = useCallback(
    async (tabId: string, paneTabs: WorkspaceTabDescriptor[]) => {
      const tabsToClose = paneTabs.filter((tab) => tab.tabId !== tabId);
      await handleBulkCloseTabs({
        tabsToClose,
        title: "Close other tabs?",
        logLabel: "from close other tabs",
      });
    },
    [handleBulkCloseTabs],
  );

  const handleCloseOtherTabs = useCallback(
    async (tabId: string) => {
      await handleCloseOtherTabsInPane(tabId, tabs);
    },
    [handleCloseOtherTabsInPane, tabs],
  );

  const handleWorkspaceTabAction = useCallback(
    (action: KeyboardActionDefinition): boolean => {
      switch (action.id) {
        case "workspace.tab.new":
          handleCreateDraftTab();
          return true;
        case "workspace.terminal.new":
          handleCreateTerminal();
          return true;
        case "workspace.tab.close-current":
          if (activeTabId) {
            void handleCloseTabById(activeTabId);
          }
          return true;
        case "workspace.tab.navigate-index": {
          const next = tabs[action.index - 1] ?? null;
          if (next?.tabId) {
            navigateToTabId(next.tabId);
          }
          return true;
        }
        case "workspace.tab.navigate-relative": {
          if (tabs.length > 0) {
            const currentIndex = tabs.findIndex((tab) => tab.tabId === activeTabId);
            const fromIndex = currentIndex >= 0 ? currentIndex : 0;
            const nextIndex = (fromIndex + action.delta + tabs.length) % tabs.length;
            const next = tabs[nextIndex] ?? null;
            if (next?.tabId) {
              navigateToTabId(next.tabId);
            }
          }
          return true;
        }
        default:
          return false;
      }
    },
    [
      activeTabId,
      handleCloseTabById,
      handleCreateDraftTab,
      handleCreateTerminal,
      navigateToTabId,
      tabs,
    ],
  );

  const handleWorkspaceSidebarAction = useCallback(
    (action: KeyboardActionDefinition): boolean => {
      if (action.id !== "sidebar.toggle.right") {
        return false;
      }
      handleToggleExplorer();
      return true;
    },
    [handleToggleExplorer],
  );

  const handleWorkspacePaneAction = useCallback(
    (action: KeyboardActionDefinition): boolean => {
      if (!persistenceKey || !workspaceLayout) {
        return true;
      }

      const focusedPane = focusedPaneTabState.pane;
      if (!focusedPane) {
        return true;
      }

      if (action.id === "workspace.pane.split.right") {
        handleCreateDraftSplit({
          targetPaneId: focusedPane.id,
          position: "right",
        });
        return true;
      }

      if (action.id === "workspace.pane.split.down") {
        handleCreateDraftSplit({
          targetPaneId: focusedPane.id,
          position: "bottom",
        });
        return true;
      }

      if (action.id.startsWith("workspace.pane.focus.")) {
        const direction = parsePaneDirection(action.id);
        if (direction) {
          const adjacentPaneId = findAdjacentPane(workspaceLayout.root, focusedPane.id, direction);
          if (adjacentPaneId) {
            focusWorkspacePane(persistenceKey, adjacentPaneId);
          }
        }
        return true;
      }

      if (action.id.startsWith("workspace.pane.move-tab.")) {
        const direction = parsePaneDirection(action.id);
        if (direction) {
          const activePaneTabId = focusedPaneTabState.activeTabId;
          const adjacentPaneId = findAdjacentPane(workspaceLayout.root, focusedPane.id, direction);
          if (activePaneTabId && adjacentPaneId) {
            paneFocusSuppressedRef.current = true;
            moveWorkspaceTabToPane(persistenceKey, activePaneTabId, adjacentPaneId);
            requestAnimationFrame(() => {
              paneFocusSuppressedRef.current = false;
            });
          }
        }
        return true;
      }

      if (action.id === "workspace.pane.close") {
        for (const tabId of focusedPane.tabIds) {
          closeWorkspaceTabWithCleanup({
            tabId,
            target: allTabDescriptorsById.get(tabId)?.target ?? null,
          });
        }
        return true;
      }

      return false;
    },
    [
      allTabDescriptorsById,
      closeWorkspaceTabWithCleanup,
      focusWorkspacePane,
      handleCreateDraftSplit,
      moveWorkspaceTabToPane,
      persistenceKey,
      focusedPaneTabState.activeTabId,
      focusedPaneTabState.pane,
      workspaceLayout,
    ],
  );

  useKeyboardActionHandler({
    handlerId: `workspace-tab-actions:${normalizedServerId}:${normalizedWorkspaceId}`,
    actions: [
      "workspace.tab.new",
      "workspace.tab.close-current",
      "workspace.tab.navigate-index",
      "workspace.tab.navigate-relative",
      "workspace.terminal.new",
    ] as const,
    enabled: Boolean(isRouteFocused && normalizedServerId && normalizedWorkspaceId),
    priority: 100,
    isActive: () => true,
    handle: handleWorkspaceTabAction,
  });

  useKeyboardActionHandler({
    handlerId: `workspace-pane-actions:${normalizedServerId}:${normalizedWorkspaceId}`,
    actions: [
      "workspace.pane.split.right",
      "workspace.pane.split.down",
      "workspace.pane.focus.left",
      "workspace.pane.focus.right",
      "workspace.pane.focus.up",
      "workspace.pane.focus.down",
      "workspace.pane.move-tab.left",
      "workspace.pane.move-tab.right",
      "workspace.pane.move-tab.up",
      "workspace.pane.move-tab.down",
      "workspace.pane.close",
    ] as const,
    enabled: Boolean(isRouteFocused && normalizedServerId && normalizedWorkspaceId),
    priority: 100,
    isActive: () => true,
    handle: handleWorkspacePaneAction,
  });

  useKeyboardActionHandler({
    handlerId: `workspace-sidebar-actions:${normalizedServerId}:${normalizedWorkspaceId}`,
    actions: ["sidebar.toggle.right"] as const,
    enabled: Boolean(isRouteFocused && normalizedServerId && normalizedWorkspaceId),
    priority: 100,
    isActive: () => true,
    handle: handleWorkspaceSidebarAction,
  });

  const activeTabDescriptor = useMemo(() => activeTab?.descriptor ?? null, [activeTab]);
  const canRenderDesktopPaneSplits = supportsDesktopPaneSplits();
  const shouldRenderDesktopPaneFallback = useMemo(
    () => !isMobile && !canRenderDesktopPaneSplits,
    [isMobile, canRenderDesktopPaneSplits],
  );
  useEffect(() => {
    if (!isRouteFocused || isNative || typeof document === "undefined" || activeTabDescriptor) {
      return;
    }
    document.title = "Workspace";
  }, [activeTabDescriptor, isRouteFocused]);
  const buildPaneContentModel = useCallback(
    (input: {
      tab: WorkspaceTabDescriptor;
      paneId?: string | null;
      focusPaneBeforeOpen?: boolean;
    }) =>
      buildWorkspacePaneContentModel({
        tab: input.tab,
        normalizedServerId,
        normalizedWorkspaceId,
        onOpenTab: (target) => {
          if (!persistenceKey) {
            return;
          }
          if (input.focusPaneBeforeOpen && input.paneId) {
            focusWorkspacePane(persistenceKey, input.paneId);
          }
          const tabId = openWorkspaceTabFocused(persistenceKey, target);
          if (tabId) {
            navigateToTabId(tabId);
          }
        },
        onCloseCurrentTab: () => {
          void handleCloseTabById(input.tab.tabId);
        },
        onRetargetCurrentTab: (target) => {
          if (!persistenceKey) {
            return;
          }
          if (input.tab.kind === "draft" && target.kind === "agent") {
            convertWorkspaceDraftToAgent(persistenceKey, input.tab.tabId, target.agentId);
            return;
          }
          retargetWorkspaceTab(persistenceKey, input.tab.tabId, target);
        },
        onOpenWorkspaceFile: (filePath) => {
          if (input.focusPaneBeforeOpen && input.paneId && persistenceKey) {
            focusWorkspacePane(persistenceKey, input.paneId);
          }
          handleOpenFileFromChat({ filePath });
        },
      }),
    [
      handleCloseTabById,
      handleOpenFileFromChat,
      focusWorkspacePane,
      navigateToTabId,
      normalizedServerId,
      normalizedWorkspaceId,
      openWorkspaceTabFocused,
      persistenceKey,
      convertWorkspaceDraftToAgent,
      retargetWorkspaceTab,
    ],
  );
  const focusedPaneId = useMemo(
    () => focusedPaneTabState.pane?.id ?? null,
    [focusedPaneTabState.pane],
  );
  const focusedPaneTabIds = useMemo(() => tabs.map((tab) => tab.tabId), [tabs]);
  const focusedPaneTabDescriptorMap = useStableTabDescriptorMap(tabs);
  const { mountedTabIds: mountedFocusedPaneTabIdsSet } = useMountedTabSet({
    activeTabId: activeTabDescriptor?.tabId ?? null,
    allTabIds: focusedPaneTabIds,
    cap: 3,
  });
  const mountedFocusedPaneTabIds = useMemo(
    () => focusedPaneTabIds.filter((tabId) => mountedFocusedPaneTabIdsSet.has(tabId)),
    [focusedPaneTabIds, mountedFocusedPaneTabIdsSet],
  );
  const buildMobilePaneContentModel = useCallback(
    function buildMobilePaneContentModel(input: {
      paneId: string | null;
      tab: WorkspaceTabDescriptor;
    }) {
      return buildPaneContentModel({
        tab: input.tab,
        paneId: input.paneId,
        focusPaneBeforeOpen: false,
      });
    },
    [buildPaneContentModel],
  );
  const showMissingWorkspaceDescriptor = shouldRenderMissingWorkspaceDescriptor({
    workspace: workspaceDescriptor,
    hasHydratedWorkspaces,
  });
  const content = renderWorkspaceContent({
    showMissingWorkspaceDescriptor,
    isMissingWorkspaceExecutionAuthority,
    activeTabDescriptor,
    hasHydratedAgents,
    mountedFocusedPaneTabIds,
    focusedPaneTabDescriptorMap,
    isRouteFocused,
    focusedPaneId,
    buildMobilePaneContentModel,
  });

  const buildDesktopPaneContentModel = useCallback(
    function buildDesktopPaneContentModel(input: { paneId: string; tab: WorkspaceTabDescriptor }) {
      return buildPaneContentModel({
        tab: input.tab,
        paneId: input.paneId,
        focusPaneBeforeOpen: true,
      });
    },
    [buildPaneContentModel],
  );

  const desktopTabRowItems = useMemo<WorkspaceDesktopTabRowItem[]>(
    () =>
      tabs.map((tab) => ({
        tab,
        isActive: tab.tabId === activeTabDescriptor?.tabId,
        isCloseHovered: hoveredCloseTabKey === tab.key,
        isClosingTab: closingTabIds.has(tab.tabId),
      })),
    [activeTabDescriptor?.tabId, closingTabIds, hoveredCloseTabKey, tabs],
  );

  const handleFocusPane = useStableEvent(function handleFocusPane(paneId: string) {
    if (!persistenceKey || paneFocusSuppressedRef.current) {
      return;
    }
    focusWorkspacePane(persistenceKey, paneId);
  });

  const handleSplitPane = useCallback(
    function handleSplitPane(input: {
      tabId: string;
      targetPaneId: string;
      position: "left" | "right" | "top" | "bottom";
    }) {
      if (!persistenceKey) {
        return;
      }
      splitWorkspacePane(persistenceKey, input);
    },
    [persistenceKey, splitWorkspacePane],
  );

  const handleMoveTabToPane = useCallback(
    function handleMoveTabToPane(tabId: string, toPaneId: string) {
      if (!persistenceKey) {
        return;
      }
      moveWorkspaceTabToPane(persistenceKey, tabId, toPaneId);
    },
    [moveWorkspaceTabToPane, persistenceKey],
  );

  const handleResizePaneSplit = useCallback(
    function handleResizePaneSplit(groupId: string, sizes: number[]) {
      if (!persistenceKey) {
        return;
      }
      resizeWorkspaceSplit(persistenceKey, groupId, sizes);
    },
    [persistenceKey, resizeWorkspaceSplit],
  );

  const handleReorderTabsInPane = useCallback(
    function handleReorderTabsInPane(paneId: string, tabIds: string[]) {
      if (!persistenceKey) {
        return;
      }
      reorderWorkspaceTabsInPane(persistenceKey, paneId, tabIds);
    },
    [persistenceKey, reorderWorkspaceTabsInPane],
  );

  const handleReorderTabsInFocusedPane = useCallback(
    (nextTabs: WorkspaceTabDescriptor[]) => {
      if (!focusedPaneId) {
        return;
      }
      handleReorderTabsInPane(
        focusedPaneId,
        nextTabs.map((tab) => tab.tabId),
      );
    },
    [focusedPaneId, handleReorderTabsInPane],
  );

  const renderSplitPaneEmptyState = useCallback(function renderSplitPaneEmptyState() {
    return (
      <View style={styles.emptyState}>
        <Text style={styles.emptyStateText}>No tabs in this pane.</Text>
      </View>
    );
  }, []);

  const containerStyle = containerWithWorkspaceBackgroundStyle;

  const menuNewAgentIcon = MENU_NEW_AGENT_ICON;
  const menuNewTerminalIcon = MENU_NEW_TERMINAL_ICON;
  const menuCopyIcon = MENU_COPY_ICON;
  const menuSettingsIcon = MENU_SETTINGS_ICON;

  const headerRight = useMemo(
    () => (
      <View style={styles.headerRight}>
        {!isMobile && workspaceDescriptor && workspaceDescriptor.scripts.length > 0 ? (
          <WorkspaceScriptsButton
            serverId={normalizedServerId}
            workspaceId={normalizedWorkspaceId}
            scripts={workspaceDescriptor.scripts}
            liveTerminalIds={liveTerminalIds}
            onScriptTerminalStarted={handleScriptTerminalStarted}
            onViewTerminal={handleViewScriptTerminal}
            hideLabels={showCompactButtonLabels}
          />
        ) : null}
        {!isMobile ? (
          <WorkspaceOpenInEditorButton
            serverId={normalizedServerId}
            cwd={normalizedWorkspaceId}
            hideLabels={showCompactButtonLabels}
          />
        ) : null}
        {!isMobile && isGitCheckout ? (
          <>
            <WorkspaceGitActions
              serverId={normalizedServerId}
              cwd={normalizedWorkspaceId}
              hideLabels={showCompactButtonLabels}
            />
            <Tooltip delayDuration={0} enabledOnDesktop enabledOnMobile={false}>
              <TooltipTrigger asChild>
                <Pressable
                  testID="workspace-explorer-toggle"
                  onPress={handleToggleExplorer}
                  accessibilityRole="button"
                  accessibilityLabel={isExplorerOpen ? "Close explorer" : "Open explorer"}
                  accessibilityState={explorerToggleAccessibilityState}
                  style={explorerToggleStyle}
                >
                  {({ hovered, pressed }) => {
                    const active = isExplorerOpen || hovered || pressed;
                    const colorMapping = active ? foregroundColorMapping : mutedColorMapping;
                    return (
                      <ThemedSourceControlPanelIcon size={16} uniProps={colorMapping} />
                    );
                  }}
                </Pressable>
              </TooltipTrigger>
              <TooltipContent
                testID="workspace-explorer-toggle-tooltip"
                side="left"
                align="center"
                offset={8}
              >
                <View style={styles.explorerTooltipRow}>
                  <Text style={styles.explorerTooltipText}>Toggle explorer</Text>
                  <Shortcut keys={EXPLORER_TOGGLE_KEYS} style={styles.explorerTooltipShortcut} />
                </View>
              </TooltipContent>
            </Tooltip>
          </>
        ) : null}
        {!isMobile && !isGitCheckout ? (
          <HeaderToggleButton
            testID="workspace-explorer-toggle"
            onPress={handleToggleExplorer}
            tooltipLabel="Toggle explorer"
            tooltipKeys={EXPLORER_TOGGLE_KEYS}
            tooltipSide="left"
            style={styles.headerActionButton}
            accessible
            accessibilityRole="button"
            accessibilityLabel={isExplorerOpen ? "Close explorer" : "Open explorer"}
            accessibilityState={explorerToggleAccessibilityState}
          >
            {({ hovered }) => {
              const colorMapping =
                isExplorerOpen || hovered ? foregroundColorMapping : mutedColorMapping;
              return <ThemedPanelRight size={16} uniProps={colorMapping} />;
            }}
          </HeaderToggleButton>
        ) : null}
        {isMobile ? (
          <HeaderToggleButton
            testID="workspace-explorer-toggle"
            onPress={handleToggleExplorer}
            tooltipLabel="Toggle explorer"
            tooltipKeys={EXPLORER_TOGGLE_KEYS}
            tooltipSide="left"
            style={styles.headerActionButton}
            accessible
            accessibilityRole="button"
            accessibilityLabel={isExplorerOpen ? "Close explorer" : "Open explorer"}
            accessibilityState={explorerToggleAccessibilityState}
          >
            {({ hovered }) => {
              const colorMapping =
                isExplorerOpen || hovered ? foregroundColorMapping : mutedColorMapping;
              return isGitCheckout ? (
                <ThemedSourceControlPanelIcon
                  size={20}
                  uniProps={colorMapping}
                  {...sourceControlPanelStrokeWidth15}
                />
              ) : (
                <ThemedPanelRight size={20} uniProps={colorMapping} />
              );
            }}
          </HeaderToggleButton>
        ) : null}
      </View>
    ),
    [
      isMobile,
      workspaceDescriptor,
      normalizedServerId,
      normalizedWorkspaceId,
      liveTerminalIds,
      handleScriptTerminalStarted,
      handleViewScriptTerminal,
      showCompactButtonLabels,
      isGitCheckout,
      handleToggleExplorer,
      isExplorerOpen,
      explorerToggleAccessibilityState,
      explorerToggleStyle,
    ],
  );

  const documentTitleEffectTab = useMemo(
    () => (isRouteFocused && isWeb ? activeTabDescriptor : null),
    [isRouteFocused, activeTabDescriptor],
  );
  const showScreenHeader = useMemo(
    () => !isFocusModeEnabled || isMobile,
    [isFocusModeEnabled, isMobile],
  );
  const showExplorerSidebar = useMemo(
    () => isRouteFocused && (!isFocusModeEnabled || isMobile),
    [isRouteFocused, isFocusModeEnabled, isMobile],
  );
  const createTerminalDisabled = useMemo(
    () => createTerminalMutation.isPending || pendingTerminalCreateInput !== null,
    [createTerminalMutation.isPending, pendingTerminalCreateInput],
  );
  const focusedPaneIdOrUndefined = useMemo(() => focusedPaneId ?? undefined, [focusedPaneId]);
  const desktopFocusModeEnabled = useMemo(
    () => isFocusModeEnabled && !isMobile,
    [isFocusModeEnabled, isMobile],
  );
  const desktopContent = useMemo(() => {
    if (!canRenderDesktopPaneSplits || !workspaceLayout || !persistenceKey) {
      return content;
    }
    return (
      <SplitContainer
        layout={workspaceLayout}
        focusModeEnabled={desktopFocusModeEnabled}
        workspaceKey={persistenceKey}
        normalizedServerId={normalizedServerId}
        normalizedWorkspaceId={normalizedWorkspaceId}
        isWorkspaceFocused={isRouteFocused}
        uiTabs={uiTabs}
        hoveredCloseTabKey={hoveredCloseTabKey}
        setHoveredTabKey={setHoveredTabKey}
        setHoveredCloseTabKey={setHoveredCloseTabKey}
        closingTabIds={closingTabIds}
        onNavigateTab={navigateToTabId}
        onCloseTab={handleCloseTabById}
        onCopyResumeCommand={handleCopyResumeCommand}
        onCopyAgentId={handleCopyAgentId}
        onReloadAgent={handleReloadAgent}
        onCloseTabsToLeft={handleCloseTabsToLeftInPane}
        onCloseTabsToRight={handleCloseTabsToRightInPane}
        onCloseOtherTabs={handleCloseOtherTabsInPane}
        onCreateDraftTab={handleCreateDraftTab}
        onCreateTerminalTab={handleCreateTerminal}
        buildPaneContentModel={buildDesktopPaneContentModel}
        onFocusPane={handleFocusPane}
        onSplitPane={handleSplitPane}
        onSplitPaneEmpty={handleCreateDraftSplit}
        onMoveTabToPane={handleMoveTabToPane}
        onResizeSplit={handleResizePaneSplit}
        onReorderTabsInPane={handleReorderTabsInPane}
        renderPaneEmptyState={renderSplitPaneEmptyState}
      />
    );
  }, [
    content,
    canRenderDesktopPaneSplits,
    workspaceLayout,
    persistenceKey,
    desktopFocusModeEnabled,
    normalizedServerId,
    normalizedWorkspaceId,
    isRouteFocused,
    uiTabs,
    hoveredCloseTabKey,
    closingTabIds,
    navigateToTabId,
    handleCloseTabById,
    handleCopyResumeCommand,
    handleCopyAgentId,
    handleReloadAgent,
    handleCloseTabsToLeftInPane,
    handleCloseTabsToRightInPane,
    handleCloseOtherTabsInPane,
    handleCreateDraftTab,
    handleCreateTerminal,
    buildDesktopPaneContentModel,
    handleFocusPane,
    handleSplitPane,
    handleCreateDraftSplit,
    handleMoveTabToPane,
    handleResizePaneSplit,
    handleReorderTabsInPane,
    renderSplitPaneEmptyState,
  ]);

  return (
    <View style={containerStyle}>
      {documentTitleEffectTab ? (
        <WorkspaceTabPresentationResolver
          tab={documentTitleEffectTab}
          serverId={normalizedServerId}
          workspaceId={normalizedWorkspaceId}
        >
          {(presentation) => (
            <WorkspaceDocumentTitleEffect
              label={presentation.label}
              titleState={presentation.titleState}
            />
          )}
        </WorkspaceTabPresentationResolver>
      ) : null}
      <View style={styles.threePaneRow}>
        <View style={styles.centerColumn}>
          {showScreenHeader && (
            <ScreenHeader
              onRowLayout={onHeaderLayout}
              left={
                <>
                  <SidebarMenuToggle />
                  <WorkspaceHeaderTitleBar
                    isLoading={isWorkspaceHeaderLoading}
                    title={workspaceHeaderTitle}
                    subtitle={workspaceHeaderSubtitle}
                    showSubtitle={shouldShowWorkspaceHeaderSubtitle}
                    currentBranchName={currentBranchName}
                    isGitCheckout={isGitCheckout}
                    normalizedServerId={normalizedServerId}
                    normalizedWorkspaceId={normalizedWorkspaceId}
                    showWorkspaceSetup={showWorkspaceSetup}
                    isMobile={isMobile}
                    createTerminalDisabled={createTerminalDisabled}
                    menuNewAgentIcon={menuNewAgentIcon}
                    menuNewTerminalIcon={menuNewTerminalIcon}
                    menuCopyIcon={menuCopyIcon}
                    menuSettingsIcon={menuSettingsIcon}
                    onCreateDraftTab={handleCreateDraftTab}
                    onCreateTerminal={handleCreateTerminal}
                    onCopyWorkspacePath={handleCopyWorkspacePath}
                    onCopyBranchName={handleCopyBranchName}
                    onOpenSetupTab={handleOpenSetupTab}
                  />
                </>
              }
              right={headerRight}
            />
          )}

          {isMobile ? (
            <MobileWorkspaceTabSwitcher
              tabs={tabs}
              activeTabKey={activeTabKey}
              activeTab={activeTabDescriptor}
              tabSwitcherOptions={tabSwitcherOptions}
              tabByKey={tabByKey}
              normalizedServerId={normalizedServerId}
              normalizedWorkspaceId={normalizedWorkspaceId}
              onSelectSwitcherTab={handleSelectSwitcherTab}
              onCopyResumeCommand={handleCopyResumeCommand}
              onCopyAgentId={handleCopyAgentId}
              onReloadAgent={handleReloadAgent}
              onCloseTab={handleCloseTabById}
              onCloseTabsAbove={handleCloseTabsToLeft}
              onCloseTabsBelow={handleCloseTabsToRight}
              onCloseOtherTabs={handleCloseOtherTabs}
            />
          ) : null}

          {shouldRenderDesktopPaneFallback ? (
            <WorkspaceDesktopTabsRow
              paneId={focusedPaneIdOrUndefined}
              isFocused={isRouteFocused}
              tabs={desktopTabRowItems}
              normalizedServerId={normalizedServerId}
              normalizedWorkspaceId={normalizedWorkspaceId}
              setHoveredTabKey={setHoveredTabKey}
              setHoveredCloseTabKey={setHoveredCloseTabKey}
              onNavigateTab={navigateToTabId}
              onCloseTab={handleCloseTabById}
              onCopyResumeCommand={handleCopyResumeCommand}
              onCopyAgentId={handleCopyAgentId}
              onReloadAgent={handleReloadAgent}
              onCloseTabsToLeft={handleCloseTabsToLeft}
              onCloseTabsToRight={handleCloseTabsToRight}
              onCloseOtherTabs={handleCloseOtherTabs}
              onCreateDraftTab={handleCreateDraftTab}
              onCreateTerminalTab={handleCreateTerminal}
              disableCreateTerminal={createTerminalMutation.isPending}
              isWaitingOnTerminalReadiness={pendingTerminalCreateInput !== null}
              onReorderTabs={handleReorderTabsInFocusedPane}
              onSplitRight={noop}
              onSplitDown={noop}
              showPaneSplitActions={false}
            />
          ) : null}

          <View style={styles.centerContent}>
            {isMobile ? (
              <GestureDetector gesture={explorerOpenGesture} touchAction="pan-y">
                <View style={styles.content}>{content}</View>
              </GestureDetector>
            ) : (
              <View style={styles.content}>{desktopContent}</View>
            )}
          </View>
        </View>

        {showExplorerSidebar && workspaceDirectory ? (
          <ExplorerSidebar
            serverId={normalizedServerId}
            workspaceId={normalizedWorkspaceId}
            workspaceRoot={workspaceDirectory}
            isGit={isGitCheckout}
            onOpenFile={handleOpenFileFromExplorer}
            isRouteFocused={isRouteFocused}
          />
        ) : null}
      </View>
    </View>
  );
}

const EXPLORER_TOGGLE_KEYS: ShortcutKey[] = ["mod", "E"];
