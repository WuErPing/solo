import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { KeyboardAvoidingView, Platform, View, Text, Pressable, ScrollView, TextInput } from "react-native";
import type { ScrollView as ScrollViewType } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Send } from "lucide-react-native";
import { router } from "expo-router";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { AnsiTextContent } from "@/components/ansi-text-renderer";
import { useTmuxCapturePane } from "@/hooks/use-tmux-capture-pane";
import { useTmuxTheme } from "@/hooks/use-tmux-theme";
import type { TmuxThemeColors } from "@/hooks/use-tmux-theme";
import { getHostRuntimeStore } from "@/runtime/host-runtime";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { parseAnsi } from "@/utils/ansi-parser";
import { detectColorsFromAnsi } from "@/utils/detect-ansi-colors";

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
  tmuxTheme: TmuxThemeColors | null,
): { foreground: string; background: string; [key: string]: string } {
  const bg = contentColors.background ?? tmuxTheme?.background ?? null;
  const fg = contentColors.foreground ?? tmuxTheme?.foreground ?? null;
  if (!bg && !fg) return base;
  return {
    ...base,
    foreground: fg || base.foreground,
    background: bg || base.background,
  };
}

function TmuxPaneScreenInner() {
  const { theme } = useUnistyles();
  const agent = useTmuxAgentStore((s) => s.selectedAgent);
  const { content, isLoading, error } = useTmuxCapturePane(
    agent?.serverId ?? "",
    agent?.paneId ?? "",
    Boolean(agent),
  );
  const { theme: tmuxTheme } = useTmuxTheme(
    agent?.serverId ?? "",
    agent?.sessionName ?? "",
    Boolean(agent),
  );
  const contentColors = useMemo(
    () => content ? detectColorsFromAnsi(content) : { background: null, foreground: null },
    [content],
  );
  const terminalColors = useMemo(
    () => mergeTerminalColors(theme.colors.terminal, contentColors, tmuxTheme),
    [theme.colors.terminal, contentColors, tmuxTheme],
  );
  const segments = useMemo(
    () => (content ? parseAnsi(content) : null),
    [content],
  );
  const [inputText, setInputText] = useState("");
  const [sendError, setSendError] = useState(false);
  const [loadTimedOut, setLoadTimedOut] = useState(false);
  const scrollRef = useRef<ScrollViewType>(null);

  useEffect(() => {
    if (isLoading && !content) {
      const id = setTimeout(() => setLoadTimedOut(true), 8000);
      return () => clearTimeout(id);
    }
    setLoadTimedOut(false);
  }, [isLoading, content]);

  const scrollToTop = useCallback(() => {
    scrollRef.current?.scrollTo({ y: 0, animated: true });
  }, []);

  const scrollToBottom = useCallback(() => {
    scrollRef.current?.scrollToEnd({ animated: true });
  }, []);

  // Auto-scroll to bottom when content refreshes
  useEffect(() => {
    if (content) {
      const id = setTimeout(() => scrollToBottom(), 50);
      return () => clearTimeout(id);
    }
  }, [content, scrollToBottom]);

  // Auto-clear send error after 2 seconds
  useEffect(() => {
    if (sendError) {
      const id = setTimeout(() => setSendError(false), 2000);
      return () => clearTimeout(id);
    }
  }, [sendError]);

  const getClient = useCallback(() => {
    if (!agent) return null;
    return getHostRuntimeStore().getClient(agent.serverId);
  }, [agent]);

  const handleSend = useCallback(() => {
    const trimmed = inputText.trim();
    const client = getClient();
    if (!trimmed || !client || !agent) return;
    client.tmuxSendKeys(agent.paneId, trimmed).catch(() => setSendError(true));
    setInputText("");
  }, [inputText, getClient, agent]);

  const sendKey = useCallback(
    (key: string) => {
      const client = getClient();
      if (!client || !agent) return;
      client.tmuxSendKeys(agent.paneId, key, false).catch(() => setSendError(true));
    },
    [getClient, agent],
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
        title={agent.agentName}
        onBack={() => router.back()}
        titleAccessory={
          <Text style={styles.subtitleText}>
            {agent.sessionName} / {agent.windowName}
          </Text>
        }
      />
      <ScrollView ref={scrollRef} style={[styles.contentScroll, { backgroundColor: terminalColors.background }]} keyboardShouldPersistTaps="handled">
        {isLoading && !segments && !loadTimedOut ? (
          <Text style={styles.loadingText}>Capturing pane content...</Text>
        ) : isLoading && !segments && loadTimedOut ? (
          <Text style={styles.errorText}>Pane content too large or unavailable</Text>
        ) : error ? (
          <Text style={styles.errorText}>{error}</Text>
        ) : segments && segments.length > 0 ? (
          <AnsiTextContent segments={segments} style={styles.contentText} terminalColors={terminalColors} />
        ) : (
          <Text style={styles.contentText}>(empty pane)</Text>
        )}
      </ScrollView>
      <View style={styles.keyButtonsRow}>
        <Pressable
          onPress={scrollToTop}
          style={({ pressed }) => [
            styles.keyButton,
            { borderColor: theme.colors.border },
            pressed ? { backgroundColor: theme.colors.surface1 } : null,
          ]}
        >
          <Text style={[styles.keyButtonLabel, { color: theme.colors.foreground }]}>Home</Text>
        </Pressable>
        <Pressable
          onPress={scrollToBottom}
          style={({ pressed }) => [
            styles.keyButton,
            { borderColor: theme.colors.border },
            pressed ? { backgroundColor: theme.colors.surface1 } : null,
          ]}
        >
          <Text style={[styles.keyButtonLabel, { color: theme.colors.foreground }]}>End</Text>
        </Pressable>
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
              styles.keyButton,
              { borderColor: theme.colors.border },
              pressed ? { backgroundColor: theme.colors.surface1 } : null,
            ]}
          >
            <Text style={[styles.keyButtonLabel, { color: theme.colors.foreground }]}>
              {label}
            </Text>
          </Pressable>
        ))}
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
  loadingText: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
    textAlign: "center",
    padding: 24,
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
    flexWrap: "wrap",
    paddingHorizontal: 12,
    paddingVertical: 6,
    gap: 6,
    borderTopWidth: 1,
    borderTopColor: theme.colors.border,
  },
  keyButton: {
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
}));
