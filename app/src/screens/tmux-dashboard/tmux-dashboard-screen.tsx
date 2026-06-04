import React, { useMemo, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  RefreshControl,
} from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Terminal, Monitor } from "lucide-react-native";
import { router } from "expo-router";
import { MenuHeader } from "@/components/headers/menu-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { useAggregatedTmuxAgents } from "@/hooks/use-tmux-agents";
import { useIsCompactFormFactor } from "@/constants/layout";
import { useStoreReady } from "@/app/_layout";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
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

export function TmuxDashboardScreen() {
  return (
    <ErrorBoundary fallbackLabel="Tmux dashboard encountered an error">
      <TmuxDashboardScreenInner />
    </ErrorBoundary>
  );
}

function TmuxDashboardScreenInner() {
  const { theme } = useUnistyles();
  const isCompact = useIsCompactFormFactor();
  const storeReady = useStoreReady();
  const { agents, isLoading, isInitialLoad, error, refreshAll } =
    useAggregatedTmuxAgents();
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const setSelectedAgent = useTmuxAgentStore((s) => s.setSelectedAgent);

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

  const handleAgentPress = (agent: TmuxAgent) => {
    setSelectedAgent(agent);
    router.push("/tmux-pane");
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
                onPress={() => handleAgentPress(agent)}
              />
            ))}
          </View>
        </ScrollView>
      )}
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
}));
