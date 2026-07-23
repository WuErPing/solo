import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Keyboard, Pressable, Text, View } from "react-native";
import { Combobox, type ComboboxOption } from "@/components/ui/combobox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { isNative } from "@/constants/platform";
import {
  WorkspaceTabPresentationResolver,
  WorkspaceTabIcon,
  WorkspaceTabOptionRow,
  type WorkspaceTabPresentation,
} from "@/screens/workspace/workspace-tab-presentation";
import {
  buildWorkspaceTabMenuEntries,
  type WorkspaceTabMenuEntry,
} from "@/screens/workspace/workspace-tab-menu";
import type { WorkspaceTabDescriptor } from "@/screens/workspace/workspace-tabs-types";
import {
  WorkspacePaneContent,
  type WorkspacePaneContentModel,
} from "@/screens/workspace/workspace-pane-content";
import { workspaceTabTargetsEqual } from "@/utils/workspace-tab-identity";
import {
  ThemedEllipsis,
  ThemedCopy,
  ThemedRotateCw,
  ThemedArrowLeftToLine,
  ThemedArrowRightToLine,
  ThemedCopyX,
  ThemedX,
  ThemedChevronDown,
  mutedColorMapping,
} from "@/screens/workspace/workspace-themed-icons";
import { styles } from "@/screens/workspace/workspace-screen-styles";

export function getFallbackTabOptionLabel(tab: WorkspaceTabDescriptor): string {
  if (tab.target.kind === "draft") {
    return "New Agent";
  }
  if (tab.target.kind === "setup") {
    return "Setup";
  }
  if (tab.target.kind === "terminal") {
    return "Terminal";
  }
  if (tab.target.kind === "file") {
    return tab.target.path.split("/").findLast(Boolean) ?? tab.target.path;
  }
  return "Agent";
}

export function getFallbackTabOptionDescription(tab: WorkspaceTabDescriptor): string {
  if (tab.target.kind === "draft") {
    return "New Agent";
  }
  if (tab.target.kind === "setup") {
    return "Workspace setup";
  }
  if (tab.target.kind === "agent") {
    return "Agent";
  }
  if (tab.target.kind === "terminal") {
    return "Terminal";
  }
  return tab.target.path;
}

interface MobileWorkspaceTabSwitcherProps {
  tabs: WorkspaceTabDescriptor[];
  activeTabKey: string;
  activeTab: WorkspaceTabDescriptor | null;
  tabSwitcherOptions: ComboboxOption[];
  tabByKey: Map<string, WorkspaceTabDescriptor>;
  normalizedServerId: string;
  normalizedWorkspaceId: string;
  onSelectSwitcherTab: (key: string) => void;
  onCopyResumeCommand: (agentId: string) => Promise<void> | void;
  onCopyAgentId: (agentId: string) => Promise<void> | void;
  onReloadAgent: (agentId: string) => Promise<void> | void;
  onCloseTab: (tabId: string) => Promise<void> | void;
  onCloseTabsAbove: (tabId: string) => Promise<void> | void;
  onCloseTabsBelow: (tabId: string) => Promise<void> | void;
  onCloseOtherTabs: (tabId: string) => Promise<void> | void;
}

function MobileActiveTabTrigger({
  activeTab,
  normalizedServerId,
  normalizedWorkspaceId,
}: {
  activeTab: WorkspaceTabDescriptor | null;
  normalizedServerId: string;
  normalizedWorkspaceId: string;
}) {
  if (!activeTab) {
    return null;
  }

  return (
    <ResolvedMobileActiveTabTrigger
      activeTab={activeTab}
      normalizedServerId={normalizedServerId}
      normalizedWorkspaceId={normalizedWorkspaceId}
    />
  );
}

function ResolvedMobileActiveTabTrigger({
  activeTab,
  normalizedServerId,
  normalizedWorkspaceId,
}: {
  activeTab: WorkspaceTabDescriptor;
  normalizedServerId: string;
  normalizedWorkspaceId: string;
}) {
  return (
    <WorkspaceTabPresentationResolver
      tab={activeTab}
      serverId={normalizedServerId}
      workspaceId={normalizedWorkspaceId}
    >
      {(presentation) => (
        <>
          <View style={styles.switcherTriggerIcon} testID="workspace-active-tab-icon">
            <WorkspaceTabIcon presentation={presentation} active />
          </View>

          <Text style={styles.switcherTriggerText} numberOfLines={1}>
            {presentation.titleState === "loading" ? "Loading..." : presentation.label}
          </Text>
        </>
      )}
    </WorkspaceTabPresentationResolver>
  );
}

export function WorkspaceDocumentTitleEffect({
  label,
  titleState,
}: {
  label: string;
  titleState: "ready" | "loading";
}) {
  useEffect(() => {
    if (isNative || typeof document === "undefined") {
      return;
    }
    const resolvedLabel = label.trim();
    document.title = titleState === "loading" ? "Loading..." : resolvedLabel || "Workspace";
  }, [label, titleState]);

  return null;
}

export function noop() {}

function mobileTabMenuTriggerStyle({ open, pressed }: { open?: boolean; pressed?: boolean }) {
  return [
    styles.mobileTabMenuTrigger,
    (Boolean(open) || Boolean(pressed)) && styles.mobileTabMenuTriggerActive,
  ];
}

function switcherTriggerStyle({ pressed }: { pressed?: boolean }) {
  return [styles.switcherTrigger, Boolean(pressed) && styles.switcherTriggerPressed];
}

function MobileTabTrailingAccessory({
  menuTestIDBase,
  presentationLabel,
  menuEntries,
}: {
  menuTestIDBase: string;
  presentationLabel: string;
  menuEntries: WorkspaceTabMenuEntry[];
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        testID={`${menuTestIDBase}-trigger`}
        accessibilityRole="button"
        accessibilityLabel={`Open menu for ${presentationLabel}`}
        hitSlop={8}
        style={mobileTabMenuTriggerStyle}
      >
        <ThemedEllipsis size={14} uniProps={mutedColorMapping} />
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end" width={220} testID={menuTestIDBase}>
        {menuEntries.map((entry) =>
          entry.kind === "separator" ? (
            <DropdownMenuSeparator key={entry.key} />
          ) : (
            <MobileTabDropdownMenuItem key={entry.key} entry={entry} />
          ),
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function MobileTabDropdownMenuItem({
  entry,
}: {
  entry: Extract<WorkspaceTabMenuEntry, { kind: "item" }>;
}) {
  const leading = useMemo(() => {
    switch (entry.icon) {
      case "copy":
        return <ThemedCopy size={16} uniProps={mutedColorMapping} />;
      case "rotate-cw":
        return <ThemedRotateCw size={16} uniProps={mutedColorMapping} />;
      case "arrow-left-to-line":
        return <ThemedArrowLeftToLine size={16} uniProps={mutedColorMapping} />;
      case "arrow-right-to-line":
        return <ThemedArrowRightToLine size={16} uniProps={mutedColorMapping} />;
      case "copy-x":
        return <ThemedCopyX size={16} uniProps={mutedColorMapping} />;
      case "x":
        return <ThemedX size={16} uniProps={mutedColorMapping} />;
      default:
        return undefined;
    }
  }, [entry.icon]);
  const trailing = useMemo(
    () => (entry.hint ? <Text style={styles.menuItemHint}>{entry.hint}</Text> : undefined),
    [entry.hint],
  );
  return (
    <DropdownMenuItem
      testID={entry.testID}
      disabled={entry.disabled}
      destructive={entry.destructive}
      onSelect={entry.onSelect}
      tooltip={entry.tooltip}
      leading={leading}
      trailing={trailing}
    >
      {entry.label}
    </DropdownMenuItem>
  );
}

function MobileWorkspaceTabOption({
  tab,
  tabIndex,
  tabCount,
  normalizedServerId,
  normalizedWorkspaceId,
  selected,
  active,
  onPress,
  onCopyResumeCommand,
  onCopyAgentId,
  onReloadAgent,
  onCloseTab,
  onCloseTabsAbove,
  onCloseTabsBelow,
  onCloseOtherTabs,
}: {
  tab: WorkspaceTabDescriptor;
  tabIndex: number;
  tabCount: number;
  normalizedServerId: string;
  normalizedWorkspaceId: string;
  selected: boolean;
  active: boolean;
  onPress: () => void;
  onCopyResumeCommand: (agentId: string) => Promise<void> | void;
  onCopyAgentId: (agentId: string) => Promise<void> | void;
  onReloadAgent: (agentId: string) => Promise<void> | void;
  onCloseTab: (tabId: string) => Promise<void> | void;
  onCloseTabsAbove: (tabId: string) => Promise<void> | void;
  onCloseTabsBelow: (tabId: string) => Promise<void> | void;
  onCloseOtherTabs: (tabId: string) => Promise<void> | void;
}) {
  const menuTestIDBase = `workspace-tab-menu-${tab.key}`;
  const menuEntries = buildWorkspaceTabMenuEntries({
    surface: "mobile",
    tab,
    index: tabIndex,
    tabCount,
    menuTestIDBase,
    onCopyResumeCommand,
    onCopyAgentId,
    onReloadAgent,
    onCloseTab,
    onCloseTabsBefore: onCloseTabsAbove,
    onCloseTabsAfter: onCloseTabsBelow,
    onCloseOtherTabs,
  });

  const fallbackLabel = getFallbackTabOptionLabel(tab);
  const trailingAccessory = useMemo(
    () => (
      <MobileTabTrailingAccessory
        menuTestIDBase={menuTestIDBase}
        presentationLabel={fallbackLabel}
        menuEntries={menuEntries}
      />
    ),
    [menuTestIDBase, fallbackLabel, menuEntries],
  );

  const renderPresentation = useCallback(
    (presentation: WorkspaceTabPresentation) => (
      <WorkspaceTabOptionRow
        presentation={presentation}
        selected={selected}
        active={active}
        onPress={onPress}
        trailingAccessory={trailingAccessory}
      />
    ),
    [selected, active, onPress, trailingAccessory],
  );

  return (
    <WorkspaceTabPresentationResolver
      tab={tab}
      serverId={normalizedServerId}
      workspaceId={normalizedWorkspaceId}
    >
      {renderPresentation}
    </WorkspaceTabPresentationResolver>
  );
}

export const MobileWorkspaceTabSwitcher = memo(function MobileWorkspaceTabSwitcher({
  tabs,
  activeTabKey,
  activeTab,
  tabSwitcherOptions,
  tabByKey,
  normalizedServerId,
  normalizedWorkspaceId,
  onSelectSwitcherTab,
  onCopyResumeCommand,
  onCopyAgentId,
  onReloadAgent,
  onCloseTab,
  onCloseTabsAbove,
  onCloseTabsBelow,
  onCloseOtherTabs,
}: MobileWorkspaceTabSwitcherProps) {
  const [isOpen, setIsOpen] = useState(false);
  const anchorRef = useRef<View>(null);
  const tabIndexByKey = useMemo(() => {
    const map = new Map<string, number>();
    tabs.forEach((tab, index) => {
      map.set(tab.key, index);
    });
    return map;
  }, [tabs]);

  const handleOpenSwitcher = useCallback(() => {
    Keyboard.dismiss();
    setIsOpen(true);
  }, []);

  const renderTabOption = useCallback(
    ({
      option,
      selected,
      active,
      onPress,
    }: {
      option: ComboboxOption;
      selected: boolean;
      active: boolean;
      onPress: () => void;
    }) => {
      const tab = tabByKey.get(option.id);
      if (!tab) {
        return <View />;
      }
      const tabIndex = tabIndexByKey.get(tab.key) ?? -1;
      if (tabIndex < 0) {
        return <View />;
      }
      return (
        <MobileWorkspaceTabOption
          tab={tab}
          tabIndex={tabIndex}
          tabCount={tabs.length}
          normalizedServerId={normalizedServerId}
          normalizedWorkspaceId={normalizedWorkspaceId}
          selected={selected}
          active={active}
          onPress={onPress}
          onCopyResumeCommand={onCopyResumeCommand}
          onCopyAgentId={onCopyAgentId}
          onReloadAgent={onReloadAgent}
          onCloseTab={onCloseTab}
          onCloseTabsAbove={onCloseTabsAbove}
          onCloseTabsBelow={onCloseTabsBelow}
          onCloseOtherTabs={onCloseOtherTabs}
        />
      );
    },
    [
      tabByKey,
      tabIndexByKey,
      tabs.length,
      normalizedServerId,
      normalizedWorkspaceId,
      onCopyResumeCommand,
      onCopyAgentId,
      onReloadAgent,
      onCloseTab,
      onCloseTabsAbove,
      onCloseTabsBelow,
      onCloseOtherTabs,
    ],
  );

  return (
    <View style={styles.mobileTabsRow} testID="workspace-tabs-row">
      <Pressable
        ref={anchorRef}
        testID="workspace-tab-switcher-trigger"
        accessibilityRole="button"
        accessibilityLabel={`Switch tabs (${tabs.length} open)`}
        style={switcherTriggerStyle}
        onPress={handleOpenSwitcher}
      >
        <View style={styles.switcherTriggerLeft}>
          <MobileActiveTabTrigger
            activeTab={activeTab}
            normalizedServerId={normalizedServerId}
            normalizedWorkspaceId={normalizedWorkspaceId}
          />
        </View>
        <ThemedChevronDown size={14} uniProps={mutedColorMapping} />
      </Pressable>

      <Combobox
        options={tabSwitcherOptions}
        value={activeTabKey}
        onSelect={onSelectSwitcherTab}
        searchable={false}
        title="Switch tab"
        searchPlaceholder="Search tabs"
        open={isOpen}
        onOpenChange={setIsOpen}
        anchorRef={anchorRef}
        renderOption={renderTabOption}
      />
    </View>
  );
});

interface MobileMountedTabSlotProps {
  tabDescriptor: WorkspaceTabDescriptor;
  isVisible: boolean;
  isWorkspaceFocused: boolean;
  isPaneFocused: boolean;
  paneId: string | null;
  buildPaneContentModel: (input: {
    paneId: string | null;
    tab: WorkspaceTabDescriptor;
  }) => WorkspacePaneContentModel;
}

export const MobileMountedTabSlot = memo(function MobileMountedTabSlot({
  tabDescriptor,
  isVisible,
  isWorkspaceFocused,
  isPaneFocused,
  paneId,
  buildPaneContentModel,
}: MobileMountedTabSlotProps) {
  const content = useMemo(
    () =>
      buildPaneContentModel({
        paneId,
        tab: tabDescriptor,
      }),
    [buildPaneContentModel, paneId, tabDescriptor],
  );

  const slotStyle = useMemo(
    () => ({ display: isVisible ? ("flex" as const) : ("none" as const), flex: 1 }),
    [isVisible],
  );

  return (
    <View style={slotStyle}>
      <WorkspacePaneContent
        content={content}
        isWorkspaceFocused={isWorkspaceFocused}
        isPaneFocused={isPaneFocused}
      />
    </View>
  );
});

export function useStableTabDescriptorMap(tabDescriptors: WorkspaceTabDescriptor[]) {
  const cacheRef = useRef(new Map<string, WorkspaceTabDescriptor>());
  const tabDescriptorMap = useMemo(() => {
    const next = new Map<string, WorkspaceTabDescriptor>();
    for (const tabDescriptor of tabDescriptors) {
      /* eslint-disable react-hooks/refs -- stable-reference cache: reading ref inside useMemo to preserve object identity */
      const cachedDescriptor = cacheRef.current.get(tabDescriptor.tabId);
      if (
        cachedDescriptor &&
        cachedDescriptor.key === tabDescriptor.key &&
        cachedDescriptor.kind === tabDescriptor.kind &&
        workspaceTabTargetsEqual(cachedDescriptor.target, tabDescriptor.target)
      ) {
        next.set(tabDescriptor.tabId, cachedDescriptor);
        continue;
      }
      /* eslint-enable react-hooks/refs */
      next.set(tabDescriptor.tabId, tabDescriptor);
    }
    return next;
  }, [tabDescriptors]);
  useEffect(() => {
    cacheRef.current = tabDescriptorMap;
  }, [tabDescriptorMap]);

  return tabDescriptorMap;
}
