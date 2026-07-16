import React, { useCallback, useEffect, useMemo, useState } from "react";
import { absoluteFillObject } from "@/utils/absolute-fill";
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  View,
} from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { router } from "expo-router";
import { Send, ChevronDown, ChevronUp } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import TerminalEmulator from "@/components/terminal-emulator";
import { useTmuxCapturePane } from "@/hooks/use-tmux-capture-pane";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";
import { toXtermTheme } from "@/utils/to-xterm-theme";

export function TmuxPaneXtermScreen() {
  return (
    <ErrorBoundary fallbackLabel="Tmux xterm pane encountered an error">
      <TmuxPaneXtermScreenInner />
    </ErrorBoundary>
  );
}

const VIRTUAL_KEYS: { label: string; key: string }[] = [
  { label: "Esc", key: "Escape" },
  { label: "Tab", key: "Tab" },
  { label: "↑", key: "ArrowUp" },
  { label: "↓", key: "ArrowDown" },
  { label: "Enter", key: "Enter" },
  { label: "C-c", key: "C-c" },
];

const TERMINAL_EMULATOR_DOM_PROPS = {
  style: { flex: 1 },
  matchContents: false,
  scrollEnabled: true,
  nestedScrollEnabled: true,
  overScrollMode: "never" as const,
  bounces: false,
  automaticallyAdjustContentInsets: false,
  contentInsetAdjustmentBehavior: "never" as const,
};

function TmuxPaneXtermScreenInner() {
  const { theme } = useUnistyles();
  const agent = useTmuxAgentStore((s) => s.selectedAgent);
  const [inputText, setInputText] = useState("");
  const [sendError, setSendError] = useState(false);
  const [inputPanelHidden, setInputPanelHidden] = useState(false);
  const [focusRequestToken, setFocusRequestToken] = useState(0);
  const [viewMode, setViewMode] = useState<"fit" | "original">("fit");

  // Always fetch the pane's native-width content (omit cols) so the full tmux
  // grid is rendered — never a lossy rewrapped approximation. paneCols drives
  // forceCols so xterm renders at the pane's native column count in both modes.
  const {
    content,
    isLoading,
    isLoadingMore,
    error,
    refetch,
    loadMoreHistory,
    hasMoreHistory,
    autoRefresh,
    setAutoRefresh,
    paneCols,
  } = useTmuxCapturePane(agent?.serverId ?? "", agent?.paneId ?? "", Boolean(agent), undefined);

  const forcedCols = paneCols ?? undefined;

  const xtermTheme = useMemo(() => toXtermTheme(theme.colors.terminal), [theme.colors.terminal]);

  const streamKey = useMemo(
    () => (agent ? `tmux-xterm:${agent.serverId}:${agent.paneId}` : "tmux-xterm:none"),
    [agent],
  );

  useEffect(() => {
    if (sendError) {
      const id = setTimeout(() => setSendError(false), 2000);
      return () => clearTimeout(id);
    }
  }, [sendError]);

  const sendKeys = useCallback(
    async (keys: string, sendEnter = false) => {
      if (!agent) return;
      try {
        await withLiveTmuxClient(agent.serverId, (c) => c.tmuxSendKeys(agent.paneId, keys, sendEnter));
        void refetch();
      } catch {
        setSendError(true);
      }
    },
    [agent, refetch],
  );

  const handleSendInput = useCallback(() => {
    const trimmed = inputText.trim();
    if (!trimmed) return;
    void sendKeys(trimmed, true);
    setInputText("");
  }, [inputText, sendKeys]);

  const handleTerminalInput = useCallback(
    (data: string) => {
      void sendKeys(data, false);
    },
    [sendKeys],
  );

  const title = agent && "agentName" in agent ? agent.agentName : agent?.currentCmd ?? "Tmux Pane";

  const autoRefreshToggle = (
    <Pressable
      onPress={() => setAutoRefresh(!autoRefresh)}
      style={({ pressed }) => [styles.toggleButton, pressed ? { opacity: 0.7 } : null]}
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

  const widthToggle = (
    <Pressable
      testID="tmux-xterm-width-toggle-button"
      onPress={() => setViewMode((prev) => (prev === "fit" ? "original" : "fit"))}
      style={({ pressed }) => [
        styles.keyButton,
        {
          backgroundColor:
            viewMode === "original"
              ? theme.colors.primary
              : pressed
                ? theme.colors.surface2
                : theme.colors.surface1,
        },
      ]}
    >
      <Text
        style={[
          styles.keyButtonLabel,
          { color: viewMode === "original" ? theme.colors.background : theme.colors.foreground },
        ]}
      >
        {viewMode === "fit" ? "1:1" : "Fit"}
      </Text>
    </Pressable>
  );

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

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <BackHeader
        title={title}
        onBack={() => router.back()}
        titleAccessory={
          <Text style={styles.subtitleText}>
            {agent.sessionName} / {agent.windowName}
          </Text>
        }
        rightContent={
          <View style={styles.headerRightRow}>
            {autoRefreshToggle}
            <Pressable
              testID="tmux-xterm-hide-input-button"
              onPress={() => setInputPanelHidden((prev) => !prev)}
              style={({ pressed }) => [
                styles.iconButton,
                inputPanelHidden ? { backgroundColor: theme.colors.primary } : null,
                pressed ? { opacity: 0.7 } : null,
              ]}
            >
              {inputPanelHidden ? (
                <ChevronUp size={16} color={theme.colors.background} />
              ) : (
                <ChevronDown size={16} color={theme.colors.foregroundMuted} />
              )}
            </Pressable>
          </View>
        }
      />

      <View style={styles.terminalContainer}>
        <TerminalEmulator
          dom={TERMINAL_EMULATOR_DOM_PROPS}
          streamKey={viewMode === "fit" ? streamKey : `${streamKey}:original`}
          testId="tmux-xterm-surface"
          xtermTheme={xtermTheme}
          convertEol
          snapshotText={content}
          onInput={handleTerminalInput}
          focusRequestToken={focusRequestToken}
          forceCols={forcedCols}
          fitToWidth={viewMode === "fit"}
          allowHorizontalScroll={viewMode === "original"}
        />
        {isLoading && !content ? (
          <View style={styles.loadingOverlay} pointerEvents="none">
            <ActivityIndicator size="small" color={theme.colors.foregroundMuted} />
          </View>
        ) : null}
        {error && !content ? (
          <View style={styles.errorOverlay} pointerEvents="none">
            <Text style={styles.errorText}>{error}</Text>
          </View>
        ) : null}
      </View>

      {!inputPanelHidden && (
        <>
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            style={styles.keyBar}
            contentContainerStyle={styles.keyBarContent}
          >
            {widthToggle}
            <Pressable
              testID="tmux-xterm-refresh-button"
              onPress={() => void refetch()}
              style={({ pressed }) => [
                styles.keyButton,
                { backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1 },
              ]}
            >
              <Text style={[styles.keyButtonLabel, { color: theme.colors.primary }]}>Refresh</Text>
            </Pressable>
            {VIRTUAL_KEYS.map(({ label, key }) => (
              <Pressable
                key={key}
                testID={`tmux-xterm-key-${key}`}
                onPress={() => sendKeys(key, false)}
                style={({ pressed }) => [
                  styles.keyButton,
                  { backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1 },
                ]}
              >
                <Text style={[styles.keyButtonLabel, { color: theme.colors.foreground }]}>{label}</Text>
              </Pressable>
            ))}
            <Pressable
              testID="tmux-xterm-load-more-button"
              onPress={() => loadMoreHistory()}
              disabled={!hasMoreHistory || isLoadingMore}
              style={({ pressed }) => [
                styles.keyButton,
                { backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1 },
                !hasMoreHistory ? { opacity: 0.4 } : null,
              ]}
            >
              <Text style={[styles.keyButtonLabel, { color: theme.colors.foregroundMuted }]}>
                {isLoadingMore ? "Loading..." : "History"}
              </Text>
            </Pressable>
          </ScrollView>

          {sendError ? (
            <Text style={[styles.sendErrorText, { color: theme.colors.destructive }]}>
              Connection lost — command not sent
            </Text>
          ) : null}

          <View style={styles.inputRow}>
            <TextInput
              testID="tmux-xterm-input"
              style={[
                styles.inputField,
                { color: theme.colors.foreground, borderColor: theme.colors.border },
              ]}
              placeholder="Type a command..."
              placeholderTextColor={theme.colors.foregroundMuted}
              value={inputText}
              onChangeText={setInputText}
              onSubmitEditing={handleSendInput}
              returnKeyType="send"
            />
            <Pressable
              testID="tmux-xterm-send-button"
              onPress={handleSendInput}
              style={[styles.sendButton, { backgroundColor: theme.colors.primary }]}
            >
              <Send size={16} color={theme.colors.background} />
            </Pressable>
          </View>
        </>
      )}

      {inputPanelHidden && (
        <Pressable
          testID="tmux-xterm-show-input-button"
          onPress={() => {
            setInputPanelHidden(false);
            setFocusRequestToken((t) => t + 1);
          }}
          style={({ pressed }) => [
            styles.floatingShowButton,
            pressed ? { opacity: 0.7 } : null,
          ]}
        >
          <ChevronUp size={20} color={theme.colors.background} />
        </Pressable>
      )}
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
  terminalContainer: {
    flex: 1,
    minHeight: 0,
    position: "relative",
  },
  loadingOverlay: {
    ...absoluteFillObject,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "rgba(0, 0, 0, 0.16)",
  },
  errorOverlay: {
    ...absoluteFillObject,
    alignItems: "center",
    justifyContent: "center",
    padding: 24,
    backgroundColor: theme.colors.background,
  },
  errorText: {
    color: theme.colors.destructive,
    fontSize: 14,
    textAlign: "center",
  },
  headerRightRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
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
  iconButton: {
    paddingHorizontal: 6,
    paddingVertical: 4,
    borderRadius: 6,
  },
  keyBar: {
    maxHeight: 48,
    borderTopWidth: 1,
    borderTopColor: theme.colors.border,
  },
  keyBarContent: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  keyButton: {
    borderWidth: 1,
    borderColor: theme.colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 6,
    alignItems: "center",
    justifyContent: "center",
  },
  keyButtonLabel: {
    fontSize: 12,
    fontWeight: "500",
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
  sendErrorText: {
    fontSize: 12,
    textAlign: "center",
    paddingVertical: 4,
  },
  floatingShowButton: {
    position: "absolute",
    bottom: 16,
    right: 16,
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: theme.colors.primary,
    alignItems: "center",
    justifyContent: "center",
    elevation: 4,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 4,
  },
}));
