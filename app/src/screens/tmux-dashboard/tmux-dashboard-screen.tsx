import { useCallback, useMemo, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  RefreshControl,
} from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import {
  Terminal,
  Monitor,
  Send,
} from "lucide-react-native";
import { MenuHeader } from "@/components/headers/menu-header";
import {
  AdaptiveModalSheet,
  AdaptiveTextInput,
} from "@/components/adaptive-modal-sheet";
import { useAggregatedTmuxAgents } from "@/hooks/use-tmux-agents";
import { useTmuxCapturePane } from "@/hooks/use-tmux-capture-pane";
import { useIsCompactFormFactor } from "@/constants/layout";
import { useStoreReady } from "@/app/_layout";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import type { TmuxAgent } from "@/hooks/use-tmux-agents";

interface AgentNameGroup {
  name: string;
  count: number;
}

function AgentCard({
  agent,
  onPress,
}: {
  agent: TmuxAgent;
  onPress: () => void;
}) {
  const { theme } = useUnistyles();

  return (
    <Pressable
      onPress={onPress}
      style={({ pressed }) => [
        styles.agentCard,
        pressed ? { opacity: 0.8 } : null,
      ]}
    >
      <View style={styles.agentCardHeader}>
        <View style={styles.agentNameRow}>
          <Terminal size={16} color={theme.colors.primary} />
          <Text style={styles.agentName}>{agent.agentName}</Text>
        </View>
        <Text style={styles.serverLabel}>{agent.serverLabel}</Text>
      </View>
      <View style={styles.agentCardBody}>
        <Text style={styles.detailLabel}>
          Session: <Text style={styles.detailValue}>{agent.sessionName}</Text>
        </Text>
        <Text style={styles.detailLabel}>
          Window: <Text style={styles.detailValue}>{agent.windowName}</Text>
        </Text>
        <Text style={styles.detailLabel}>
          Pane: <Text style={styles.detailValue}>{agent.paneId}</Text>
        </Text>
        {agent.workingDir ? (
          <Text style={styles.detailLabel} numberOfLines={1}>
            Dir: <Text style={styles.detailValue}>{agent.workingDir}</Text>
          </Text>
        ) : null}
        <Text style={styles.detailLabel}>
          PID: <Text style={styles.detailValue}>{agent.panePid}</Text>
        </Text>
      </View>
    </Pressable>
  );
}

function NameCard({
  group,
  isActive,
  onPress,
}: {
  group: AgentNameGroup;
  isActive: boolean;
  onPress: () => void;
}) {
  const { theme } = useUnistyles();

  return (
    <Pressable
      onPress={onPress}
      style={[
        styles.nameCard,
        isActive && { borderColor: theme.colors.primary, borderWidth: 2 },
      ]}
    >
      <Text style={styles.nameCardCount}>{group.count}</Text>
      <Text style={styles.nameCardLabel}>{group.name}</Text>
    </Pressable>
  );
}

function PaneContentModal({
  agent,
  onClose,
}: {
  agent: TmuxAgent;
  onClose: () => void;
}) {
  const { theme } = useUnistyles();
  const client = useHostRuntimeClient(agent.serverId);
  const { content, isLoading, error } = useTmuxCapturePane(
    agent.serverId,
    agent.paneId,
    true,
  );
  const [inputText, setInputText] = useState("");

  const handleSend = useCallback(() => {
    const trimmed = inputText.trim();
    if (!trimmed || !client) return;
    void client.tmuxSendKeys(agent.paneId, trimmed);
    setInputText("");
  }, [inputText, client, agent.paneId]);

  const sendKey = useCallback(
    (key: string) => {
      if (!client) return;
      void client.tmuxSendKeys(agent.paneId, key, false);
    },
    [client, agent.paneId],
  );

  return (
    <AdaptiveModalSheet
      title={agent.agentName}
      subtitle={`${agent.sessionName} / ${agent.windowName}`}
      visible
      onClose={onClose}
      snapPoints={["80%"]}
      desktopMaxWidth={800}
      scrollable={false}
    >
      <ScrollView
        style={styles.paneContentScroll}
        keyboardShouldPersistTaps="handled"
      >
        {isLoading && !content ? (
          <Text style={styles.paneContentLoading}>Capturing pane content...</Text>
        ) : error ? (
          <Text style={styles.paneContentError}>{error}</Text>
        ) : (
          <Text style={styles.paneContentText} selectable>
            {content || "(empty pane)"}
          </Text>
        )}
      </ScrollView>
      <View style={styles.keyButtonsRow}>
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
        <AdaptiveTextInput
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
    </AdaptiveModalSheet>
  );
}

export function TmuxDashboardScreen() {
  const { theme } = useUnistyles();
  const isCompact = useIsCompactFormFactor();
  const storeReady = useStoreReady();
  const { agents, isLoading, isInitialLoad, error, refreshAll } =
    useAggregatedTmuxAgents();
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<TmuxAgent | null>(null);

  const nameGroups = useMemo(() => {
    const map = new Map<string, number>();
    for (const agent of agents) {
      map.set(agent.agentName, (map.get(agent.agentName) ?? 0) + 1);
    }
    return Array.from(map.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
  }, [agents]);

  const filteredAgents = useMemo(() => {
    if (!selectedName) return agents;
    return agents.filter((a) => a.agentName === selectedName);
  }, [agents, selectedName]);

  const handleNamePress = (name: string) => {
    setSelectedName((prev) => (prev === name ? null : name));
  };

  return (
    <View style={styles.container}>
      <MenuHeader
        title="Tmux Dashboard"
        rightContent={
          agents.length > 0 ? (
            <View style={styles.badge}>
              <Text style={styles.badgeText}>{agents.length} agent(s)</Text>
            </View>
          ) : undefined
        }
      />

      {!storeReady || isInitialLoad ? (
        <View style={styles.centerContent}>
          <Text style={styles.loadingText}>Scanning tmux panes...</Text>
        </View>
      ) : error ? (
        <View style={styles.centerContent}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : agents.length === 0 ? (
        <View style={styles.centerContent}>
          <Monitor size={48} color={theme.colors.foregroundMuted} />
          <Text style={styles.emptyTitle}>No AI agents detected</Text>
          <Text style={styles.emptySubtitle}>
            Start an AI agent (claude, pi, kimi, etc.) in a tmux session to see
            it here.
          </Text>
        </View>
      ) : (
        <ScrollView
          style={styles.scrollView}
          refreshControl={
            <RefreshControl refreshing={isLoading} onRefresh={refreshAll} />
          }
        >
          {nameGroups.length > 0 ? (
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              style={styles.nameCardsScroll}
              contentContainerStyle={styles.nameCardsContainer}
            >
              {nameGroups.map((group) => (
                <NameCard
                  key={group.name}
                  group={group}
                  isActive={selectedName === group.name}
                  onPress={() => handleNamePress(group.name)}
                />
              ))}
            </ScrollView>
          ) : null}

          <View
            style={isCompact ? styles.agentGridCompact : styles.agentGrid}
          >
            {filteredAgents.map((agent, index) => (
              <AgentCard
                key={`${agent.serverId}-${agent.paneId}-${index}`}
                agent={agent}
                onPress={() => setSelectedAgent(agent)}
              />
            ))}
          </View>
        </ScrollView>
      )}

      {selectedAgent ? (
        <PaneContentModal
          agent={selectedAgent}
          onClose={() => setSelectedAgent(null)}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.background,
  },
  scrollView: {
    flex: 1,
  },
  centerContent: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: 24,
    gap: 12,
  },
  loadingText: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
  },
  errorText: {
    color: theme.colors.destructive,
    fontSize: 14,
  },
  emptyTitle: {
    color: theme.colors.foreground,
    fontSize: 16,
    fontWeight: "600",
    marginTop: 8,
  },
  emptySubtitle: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
    textAlign: "center",
    maxWidth: 300,
  },
  badge: {
    backgroundColor: theme.colors.surface0,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 12,
  },
  badgeText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
    fontWeight: "500",
  },
  nameCardsScroll: {
    maxHeight: 80,
  },
  nameCardsContainer: {
    paddingHorizontal: 16,
    paddingVertical: 12,
    gap: 8,
  },
  nameCard: {
    backgroundColor: theme.colors.surface0,
    borderRadius: 8,
    paddingHorizontal: 16,
    paddingVertical: 10,
    alignItems: "center",
    minWidth: 72,
    borderWidth: 2,
    borderColor: "transparent",
  },
  nameCardCount: {
    color: theme.colors.foreground,
    fontSize: 18,
    fontWeight: "700",
  },
  nameCardLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    marginTop: 2,
  },
  agentGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    padding: 16,
    gap: 12,
  },
  agentGridCompact: {
    padding: 16,
    gap: 12,
  },
  agentCard: {
    backgroundColor: theme.colors.surface0,
    borderRadius: 10,
    padding: 14,
    width: 280,
  },
  agentCardHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 10,
  },
  agentNameRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  agentName: {
    color: theme.colors.foreground,
    fontSize: 15,
    fontWeight: "600",
  },
  serverLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
  },
  agentCardBody: {
    gap: 4,
  },
  detailLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
  },
  detailValue: {
    color: theme.colors.foreground,
    fontSize: 12,
  },
  paneContentScroll: {
    flex: 1,
    padding: 16,
  },
  paneContentLoading: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
    textAlign: "center",
    padding: 24,
  },
  paneContentError: {
    color: theme.colors.destructive,
    fontSize: 14,
    textAlign: "center",
    padding: 24,
  },
  paneContentText: {
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
