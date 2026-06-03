import React, { useCallback, useRef, useState } from "react";
import { View, Text, Pressable, ScrollView, TextInput } from "react-native";
import type { ScrollView as ScrollViewType } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ArrowLeft, Send } from "lucide-react-native";
import { router } from "expo-router";
import { MenuHeader } from "@/components/headers/menu-header";
import { useTmuxCapturePane } from "@/hooks/use-tmux-capture-pane";
import { useHostRuntimeClient, getHostRuntimeStore } from "@/runtime/host-runtime";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { isNative } from "@/constants/platform";

// Strip Unicode box-drawing and block characters that render as garbage on
// React Native's default monospace font.  These are purely decorative in TUI
// apps (kimi, pi, etc.) so removing them keeps the text readable.
function sanitizeForNative(text: string): string {
  if (!isNative) return text;
  // Strip ANSI escape sequences left over from tmux capture
  // Then strip box-drawing (U+2500-257F), block elements (U+2580-259F),
  // and braille patterns (U+2800-28FF) which render as garbage on mobile.
  return text
    .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, "")
    .replace(/[─-▟⠀-⣿]/g, "");
}

export function TmuxPaneScreen() {
  const { theme } = useUnistyles();
  const agent = useTmuxAgentStore((s) => s.selectedAgent);
  const hookClient = useHostRuntimeClient(agent?.serverId ?? "");
  const { content, isLoading, error } = useTmuxCapturePane(
    agent?.serverId ?? "",
    agent?.paneId ?? "",
    Boolean(agent),
  );
  const [inputText, setInputText] = useState("");
  const scrollRef = useRef<ScrollViewType>(null);

  const scrollToTop = useCallback(() => {
    scrollRef.current?.scrollTo({ y: 0, animated: true });
  }, []);

  const scrollToBottom = useCallback(() => {
    scrollRef.current?.scrollToEnd({ animated: true });
  }, []);

  const getClient = useCallback(() => {
    if (hookClient) return hookClient;
    if (!agent) return null;
    return getHostRuntimeStore().getClient(agent.serverId);
  }, [hookClient, agent]);

  const handleBack = useCallback(() => {
    router.back();
  }, []);

  const handleSend = useCallback(() => {
    const trimmed = inputText.trim();
    const client = getClient();
    if (!trimmed || !client || !agent) return;
    void client.tmuxSendKeys(agent.paneId, trimmed);
    setInputText("");
  }, [inputText, getClient, agent]);

  const sendKey = useCallback(
    (key: string) => {
      const client = getClient();
      if (!client || !agent) return;
      void client.tmuxSendKeys(agent.paneId, key, false);
    },
    [getClient, agent],
  );

  if (!agent) {
    return (
      <View style={styles.container}>
        <MenuHeader title="Tmux Pane" />
        <View style={styles.centerContent}>
          <Text style={styles.emptyText}>No agent selected</Text>
        </View>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <MenuHeader
        title={agent.agentName}
        subtitle={`${agent.sessionName} / ${agent.windowName}`}
        leftContent={
          <Pressable onPress={handleBack} style={styles.backButton} accessibilityLabel="Back" accessibilityRole="button">
            <ArrowLeft size={20} color={theme.colors.foreground} />
          </Pressable>
        }
      />
      <ScrollView ref={scrollRef} style={styles.contentScroll} keyboardShouldPersistTaps="handled">
        {isLoading && !content ? (
          <Text style={styles.loadingText}>Capturing pane content...</Text>
        ) : error ? (
          <Text style={styles.errorText}>{error}</Text>
        ) : (
          <Text style={styles.contentText}>
            {sanitizeForNative(content) || "(empty pane)"}
          </Text>
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
          { label: "←", key: "Left" },
          { label: "→", key: "Right" },
          { label: "Enter", key: "Enter" },
          { label: "Esc", key: "Escape" },
          { label: "Tab", key: "Tab" },
          { label: "Ctrl+C", key: "C-c" },
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
    </View>
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
  backButton: {
    padding: 8,
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
