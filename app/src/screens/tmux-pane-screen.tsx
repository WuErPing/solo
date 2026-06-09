import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  FlatList,
  KeyboardAvoidingView,
  Platform,
  View,
  Text,
  Pressable,
  TextInput,
  type NativeSyntheticEvent,
  type NativeScrollEvent,
  type ListRenderItemInfo,
  type PressableStateCallbackType,
} from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Send, Palette, TextSelect } from "lucide-react-native";
import { router } from "expo-router";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { AnsiTextLine } from "@/components/ansi-text-line";
import { splitSegmentsByLine } from "@/utils/ansi-line-splitter";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useTmuxCapturePane } from "@/hooks/use-tmux-capture-pane";
import { useAppSettings } from "@/hooks/use-settings";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { parseAnsi, type AnsiSegment } from "@/utils/ansi-parser";
import { detectColorsFromAnsi } from "@/utils/detect-ansi-colors";
import {
  TERMINAL_THEME_IDS,
  TERMINAL_THEME_PRESETS,
  type TerminalThemeId,
} from "@/styles/terminal-themes";

export function TmuxPaneScreen() {
  return (
    <ErrorBoundary fallbackLabel="Tmux pane encountered an error">
      <TmuxPaneScreenInner />
    </ErrorBoundary>
  );
}

function mergeTerminalColors(
  base: { foreground: string; background: string; [key: string]: string },
  contentColors: { background: string | null; foreground: string | null },
): { foreground: string; background: string; [key: string]: string } {
  if (!contentColors.background && !contentColors.foreground) return base;
  return {
    ...base,
    foreground: contentColors.foreground || base.foreground,
    background: contentColors.background || base.background,
  };
}

const SCROLL_TOP_THRESHOLD = 20;

function themePickerTriggerStyle({ pressed }: PressableStateCallbackType) {
  return [styles.themePickerTrigger, pressed && { opacity: 0.7 }];
}

function TmuxPaneScreenInner() {
  const { theme } = useUnistyles();
  const { settings, updateSettings } = useAppSettings();
  const agent = useTmuxAgentStore((s) => s.selectedAgent);
  const {
    content,
    isLoading,
    isLoadingMore,
    error,
    refetch,
    hasMoreHistory,
    loadMoreHistory,
    autoRefresh,
    setAutoRefresh,
  } = useTmuxCapturePane(
    agent?.serverId ?? "",
    agent?.paneId ?? "",
    Boolean(agent),
  );
  const terminalThemeId = settings.terminalTheme;
  const cachedColorsRef = useRef<{ background: string | null; foreground: string | null } | null>(null);
  const colorContentRef = useRef<string | null>(null);
  const prevColorContentRef = useRef<string | null>(null);
  const [colorResetKey, setColorResetKey] = useState(0);

  // Reset cached colors when content transitions from empty to non-empty (pane navigation)
  if (!prevColorContentRef.current && content) {
    cachedColorsRef.current = null;
    setColorResetKey((k) => k + 1);
  }
  prevColorContentRef.current = content;
  colorContentRef.current = content;

  const terminalColors = useMemo(() => {
    if (terminalThemeId !== "system") {
      return TERMINAL_THEME_PRESETS[terminalThemeId] ?? theme.colors.terminal;
    }
    if (!cachedColorsRef.current && colorContentRef.current) {
      cachedColorsRef.current = detectColorsFromAnsi(colorContentRef.current);
    }
    return mergeTerminalColors(theme.colors.terminal, cachedColorsRef.current ?? { background: null, foreground: null });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- content read via ref; re-detect only on pane change
  }, [terminalThemeId, theme.colors.terminal, colorResetKey]);
  const segments = useMemo(
    () => (content ? parseAnsi(content) : null),
    [content],
  );
  const lines = useMemo(
    () => (segments ? splitSegmentsByLine(segments) : []),
    [segments],
  );

  const [inputText, setInputText] = useState("");
  const [sendError, setSendError] = useState(false);
  const [loadTimedOut, setLoadTimedOut] = useState(false);
  const [selectMode, setSelectMode] = useState(false);
  const preSelectAutoRefreshRef = useRef<boolean | null>(null);
  const flatListRef = useRef<FlatList<AnsiSegment[]>>(null);

  const keyExtractor = useCallback((_item: AnsiSegment[], index: number) => String(index), []);

  const renderLine = useCallback(
    ({ item }: ListRenderItemInfo<AnsiSegment[]>) => (
      <AnsiTextLine segments={item} style={styles.contentText} terminalColors={terminalColors} selectable={selectMode} />
    ),
    [terminalColors, selectMode],
  );

  useEffect(() => {
    if (isLoading && !content) {
      const id = setTimeout(() => setLoadTimedOut(true), 8000);
      return () => clearTimeout(id);
    }
    setLoadTimedOut(false);
  }, [isLoading, content]);

  const scrollToTop = useCallback(() => {
    flatListRef.current?.scrollToOffset({ offset: 0, animated: true });
  }, []);

  const scrollToBottom = useCallback(() => {
    flatListRef.current?.scrollToEnd({ animated: false });
  }, []);

  const isAtBottomRef = useRef(false);
  const hasInitialScrolledRef = useRef(false);

  // Initial scroll to bottom on first content load
  useEffect(() => {
    if (content && !hasInitialScrolledRef.current && autoRefresh) {
      hasInitialScrolledRef.current = true;
      const id = setTimeout(() => {
        flatListRef.current?.scrollToEnd({ animated: false });
        isAtBottomRef.current = true;
      }, 50);
      return () => clearTimeout(id);
    }
  }, [content, autoRefresh]);

  // Auto-scroll to bottom when new content arrives and user is already at bottom
  useEffect(() => {
    if (isAtBottomRef.current && autoRefresh && !isLoadingMore && lines.length > 0) {
      const id = setTimeout(() => flatListRef.current?.scrollToEnd({ animated: false }), 50);
      return () => clearTimeout(id);
    }
  }, [lines, autoRefresh, isLoadingMore]);

  // Auto-clear send error after 2 seconds
  useEffect(() => {
    if (sendError) {
      const id = setTimeout(() => setSendError(false), 2000);
      return () => clearTimeout(id);
    }
  }, [sendError]);

  const handleSend = useCallback(() => {
    const trimmed = inputText.trim();
    if (!trimmed || !agent) return;
    withLiveTmuxClient(agent.serverId, (c) => c.tmuxSendKeys(agent.paneId, trimmed))
      .then(() => refetch())
      .catch(() => setSendError(true));
    setInputText("");
  }, [inputText, agent, refetch]);

  const sendKey = useCallback(
    (key: string) => {
      if (!agent) return;
      withLiveTmuxClient(agent.serverId, (c) => c.tmuxSendKeys(agent.paneId, key, false))
        .then(() => refetch())
        .catch(() => setSendError(true));
    },
    [agent, refetch],
  );

  const handleScroll = useCallback(
    (event: NativeSyntheticEvent<NativeScrollEvent>) => {
      if (selectMode) return;
      const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
      isAtBottomRef.current =
        contentOffset.y + layoutMeasurement.height >= contentSize.height - SCROLL_TOP_THRESHOLD;
      if (
        contentOffset.y < SCROLL_TOP_THRESHOLD &&
        !isLoadingMore &&
        hasMoreHistory
      ) {
        loadMoreHistory();
      }
    },
    [selectMode, isLoadingMore, hasMoreHistory, loadMoreHistory],
  );

  const toggleSelectMode = useCallback(() => {
    setSelectMode((prev) => {
      if (!prev) {
        preSelectAutoRefreshRef.current = autoRefresh;
        setAutoRefresh(false);
      } else if (preSelectAutoRefreshRef.current !== null) {
        setAutoRefresh(preSelectAutoRefreshRef.current);
        preSelectAutoRefreshRef.current = null;
      }
      return !prev;
    });
  }, [autoRefresh, setAutoRefresh]);

  if (!agent) {
    return (
      <View style={styles.container}>
        <BackHeader title="Tmux Pane" onBack={() => router.back()} />
        <View style={styles.centerContent}>
          <Text style={styles.emptyText}>No agent selected</Text>
        </View>
      </View>
    );
  }

  const autoRefreshToggle = (
    <Pressable
      onPress={() => setAutoRefresh(!autoRefresh)}
      style={({ pressed }) => [
        styles.toggleButton,
        pressed ? { opacity: 0.7 } : null,
      ]}
    >
      <Text
        style={[
          styles.toggleText,
          { color: autoRefresh ? theme.colors.primary : theme.colors.foregroundMuted },
        ]}
      >
        Auto
      </Text>
      <View
        style={[
          styles.toggleIndicator,
          {
            backgroundColor: autoRefresh ? theme.colors.primary : theme.colors.surface1,
            borderColor: autoRefresh ? theme.colors.primary : theme.colors.border,
          },
        ]}
      >
        <View
          style={[
            styles.toggleKnob,
            {
              backgroundColor: theme.colors.background,
              transform: [{ translateX: autoRefresh ? 12 : 0 }],
            },
          ]}
        />
      </View>
    </Pressable>
  );

  const handleTerminalThemeChange = useCallback(
    (id: TerminalThemeId) => {
      updateSettings({ terminalTheme: id });
    },
    [updateSettings],
  );

  const themePicker = (
    <DropdownMenu>
      <DropdownMenuTrigger style={themePickerTriggerStyle}>
        <Palette size={16} color={theme.colors.foregroundMuted} />
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end" width={180}>
        {(["system", "dark", "light"] as const).map((id) => {
          const label = id === "system" ? "System" : TERMINAL_THEME_PRESETS[id].label;
          const swatchColor = id === "system"
            ? theme.colors.terminal.background
            : TERMINAL_THEME_PRESETS[id].background;
          return (
            <DropdownMenuItem
              key={id}
              selected={terminalThemeId === id}
              onSelect={() => handleTerminalThemeChange(id)}
              leading={
                <View
                  style={{
                    width: 12,
                    height: 12,
                    borderRadius: 6,
                    backgroundColor: swatchColor,
                    borderWidth: 1,
                    borderColor: "rgba(255,255,255,0.15)",
                  }}
                />
              }
            >
              {label}
            </DropdownMenuItem>
          );
        })}
        <DropdownMenuSeparator />
        {(["midnight", "ghostty", "solarized-dark", "monokai", "dracula"] as const).map((id) => (
          <DropdownMenuItem
            key={id}
            selected={terminalThemeId === id}
            onSelect={() => handleTerminalThemeChange(id)}
            leading={
              <View
                style={{
                  width: 12,
                  height: 12,
                  borderRadius: 6,
                  backgroundColor: TERMINAL_THEME_PRESETS[id].background,
                  borderWidth: 1,
                  borderColor: "rgba(255,255,255,0.15)",
                }}
              />
            }
          >
            {TERMINAL_THEME_PRESETS[id].label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <BackHeader
        title={agent.agentName}
        onBack={() => router.back()}
        titleAccessory={
          <Text style={styles.subtitleText}>
            {agent.sessionName} / {agent.windowName}
          </Text>
        }
        rightContent={
          <View style={styles.headerRightRow}>
            {autoRefreshToggle}
            {themePicker}
            <Pressable
              onPress={toggleSelectMode}
              style={({ pressed }) => [
                styles.selectToggleButton,
                selectMode ? { backgroundColor: theme.colors.primary } : null,
                pressed ? { opacity: 0.7 } : null,
              ]}
            >
              <TextSelect
                size={16}
                color={selectMode ? theme.colors.background : theme.colors.foregroundMuted}
              />
              <Text
                style={[
                  styles.selectToggleText,
                  { color: selectMode ? theme.colors.primary : theme.colors.foregroundMuted },
                ]}
              >
                Select
              </Text>
            </Pressable>
          </View>
        }
      />
      <FlatList<AnsiSegment[]>
        ref={flatListRef}
        testID="tmux-pane-scroll"
        style={[styles.contentScroll, { backgroundColor: terminalColors.background }]}
        contentContainerStyle={styles.flatListContent}
        data={lines}
        renderItem={renderLine}
        keyExtractor={keyExtractor}
        keyboardShouldPersistTaps="handled"
        scrollEnabled={!selectMode}
        onScroll={handleScroll}
        scrollEventThrottle={400}
        initialNumToRender={40}
        maxToRenderPerBatch={40}
        windowSize={21}
        maintainVisibleContentPosition={{ minIndexForVisible: 0 }}
        ListFooterComponent={
          isLoadingMore ? <Text style={styles.loadingMoreText}>Loading more history...</Text> : null
        }
        ListEmptyComponent={
          isLoading && !loadTimedOut ? (
            <Text style={styles.loadingText}>Capturing pane content...</Text>
          ) : isLoading && loadTimedOut ? (
            <Text style={styles.errorText}>Pane content too large or unavailable</Text>
          ) : error ? (
            <Text style={styles.errorText}>{error}</Text>
          ) : (
            <Text style={styles.contentText}>(empty pane)</Text>
          )
        }
      />
      <View style={styles.keyButtonsRow}>
        <View style={styles.keyGroup}>
          <Text style={[styles.keyGroupLabel, { color: theme.colors.foregroundMuted }]}>View</Text>
          <View style={styles.keyGroupRow}>
            <Pressable
              onPress={scrollToTop}
              style={({ pressed }) => [
                styles.keyButtonSolid,
                styles.navKeyButton,
                {
                  backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
                  borderColor: theme.colors.border,
                },
              ]}
            >
              <Text style={[styles.keyButtonLabel, { color: theme.colors.foregroundMuted }]}>
                Home
              </Text>
            </Pressable>
            <Pressable
              onPress={scrollToBottom}
              style={({ pressed }) => [
                styles.keyButtonSolid,
                styles.navKeyButton,
                {
                  backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
                  borderColor: theme.colors.border,
                },
              ]}
            >
              <Text style={[styles.keyButtonLabel, { color: theme.colors.foregroundMuted }]}>
                End
              </Text>
            </Pressable>
          </View>
          {!autoRefresh && (
            <View style={styles.keyGroupRow}>
              <Pressable
                onPress={() => refetch()}
                style={({ pressed }) => [
                  styles.keyButtonGhost,
                  pressed ? { backgroundColor: theme.colors.surface1 } : null,
                ]}
              >
                <Text style={[styles.keyButtonLabel, styles.keyButtonLabelGhost, { color: theme.colors.primary }]}>
                  Refresh
                </Text>
              </Pressable>
            </View>
          )}
        </View>
        <View style={styles.keyGroupDivider} />
        <View style={[styles.keyGroup, styles.keyGroupMain]}>
          <Text style={[styles.keyGroupLabel, { color: theme.colors.foregroundMuted }]}>Send</Text>
          <View style={styles.keyGroupRow}>
            {[
              { label: "↑", key: "Up" },
              { label: "↓", key: "Down" },
              { label: "Enter", key: "Enter" },
              { label: "Esc", key: "Escape" },
              { label: "Tab", key: "Tab" },
              { label: "S-Tab", key: "BTab" },
              { label: "1", key: "1" },
              { label: "2", key: "2" },
              { label: "3", key: "3" },
              { label: "4", key: "4" },
            ].map(({ label, key }) => (
              <Pressable
                key={key}
                onPress={() => sendKey(key)}
                style={({ pressed }) => [
                  styles.keyButtonSolid,
                  {
                    backgroundColor: pressed ? theme.colors.primary : theme.colors.surface1,
                    borderColor: theme.colors.border,
                  },
                ]}
              >
                <Text style={[styles.keyButtonLabel, { color: theme.colors.foreground }]}>
                  {label}
                </Text>
              </Pressable>
            ))}
          </View>
        </View>
      </View>
      {sendError ? (
        <Text style={[styles.sendErrorText, { color: theme.colors.destructive }]}>
          Connection lost — command not sent
        </Text>
      ) : null}
      <View style={styles.inputRow}>
        <TextInput
          style={[
            styles.inputField,
            { color: theme.colors.foreground, borderColor: theme.colors.border },
          ]}
          placeholder="Type a command..."
          placeholderTextColor={theme.colors.foregroundMuted}
          value={inputText}
          onChangeText={setInputText}
          onSubmitEditing={handleSend}
          returnKeyType="send"
        />
        <Pressable
          testID="send-button"
          onPress={handleSend}
          style={[
            styles.sendButton,
            { backgroundColor: theme.colors.primary },
          ]}
        >
          <Send size={16} color={theme.colors.background} />
        </Pressable>
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.background,
  },
  centerContent: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: 24,
  },
  emptyText: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
  },
  subtitleText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
  },
  contentScroll: {
    flex: 1,
    padding: 16,
  },
  flatListContent: {
    flexGrow: 1,
  },
  loadingText: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
    textAlign: "center",
    padding: 24,
  },
  loadingMoreText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
    textAlign: "center",
    paddingVertical: 8,
  },
  errorText: {
    color: theme.colors.destructive,
    fontSize: 14,
    textAlign: "center",
    padding: 24,
  },
  contentText: {
    fontFamily: "monospace",
    fontSize: 12,
    color: theme.colors.foreground,
    lineHeight: 18,
  },
  keyButtonsRow: {
    flexDirection: "row",
    alignItems: "flex-start",
    paddingHorizontal: 12,
    paddingVertical: 8,
    gap: 10,
    borderTopWidth: 1,
    borderTopColor: theme.colors.border,
  },
  keyGroup: {
    gap: 4,
  },
  keyGroupMain: {
    flex: 1,
  },
  keyGroupLabel: {
    fontSize: 10,
    fontWeight: "600",
    letterSpacing: 0.5,
    textTransform: "uppercase",
    paddingHorizontal: 2,
  },
  keyGroupRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 6,
  },
  navKeyButton: {
    minWidth: 72,
  },
  keyGroupDivider: {
    width: 1,
    alignSelf: "stretch",
    marginVertical: 4,
    backgroundColor: theme.colors.border,
  },
  keyButtonGhost: {
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
    minWidth: 36,
    alignItems: "center",
    justifyContent: "center",
  },
  keyButtonSolid: {
    borderWidth: 1,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 6,
    minWidth: 36,
    alignItems: "center",
    justifyContent: "center",
  },
  keyButtonLabel: {
    fontSize: 12,
    fontWeight: "500",
  },
  keyButtonLabelGhost: {
    fontSize: 11,
    fontWeight: "500",
  },
  sendErrorText: {
    fontSize: 12,
    textAlign: "center",
    paddingVertical: 4,
  },
  inputRow: {
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: 12,
    paddingVertical: 8,
    gap: 8,
    borderTopWidth: 1,
    borderTopColor: theme.colors.border,
  },
  inputField: {
    flex: 1,
    height: 40,
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 12,
    fontSize: 14,
  },
  sendButton: {
    width: 40,
    height: 40,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
  },
  headerRightRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
  },
  themePickerTrigger: {
    padding: 6,
    borderRadius: 6,
  },
  toggleButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
  },
  toggleText: {
    fontSize: 12,
    fontWeight: "600",
  },
  toggleIndicator: {
    width: 28,
    height: 16,
    borderRadius: 8,
    borderWidth: 1,
    justifyContent: "center",
    paddingHorizontal: 2,
  },
  toggleKnob: {
    width: 12,
    height: 12,
    borderRadius: 6,
  },
  selectToggleButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: 6,
    paddingVertical: 4,
    borderRadius: 6,
  },
  selectToggleText: {
    fontSize: 12,
    fontWeight: "500",
  },
}));
