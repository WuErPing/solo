import React, { useMemo, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  RefreshControl,
} from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Terminal, Monitor, RefreshCw, SquareTerminal } from "lucide-react-native";
import { router } from "expo-router";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { useAggregatedTmuxAgents } from "@/hooks/use-tmux-agents";
import { useTmuxStatusLines } from "@/hooks/use-tmux-status-lines";
import { useIsCompactFormFactor } from "@/constants/layout";
import { useStoreReady } from "@/app/_layout";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { shortenPath } from "@/utils/shorten-path";
import { parseAnsi } from "@/utils/ansi-parser";
import { AnsiTextContent } from "@/components/ansi-text-renderer";
import type { TmuxAgent, TmuxPane } from "@/hooks/use-tmux-agents";
import type { TmuxStatusLineInfo } from "@/hooks/use-tmux-status-lines";

interface AgentNameGroup {
  name: string;
  count: number;
}

// Splits tmux statusRight like '"pane title" 22:45 06-Jun-26' into title and time/date parts.
function splitStatusRight(statusRight: string): { title: string; timeDate?: string } {
  const match = statusRight.match(/^(".*?")\s+(.+)$/);
  if (match) {
    return { title: match[1], timeDate: match[2] };
  }
  return { title: statusRight };
}

function AgentCard({
  agent,
  statusLine,
  onPress,
}: {
  agent: TmuxAgent;
  statusLine?: TmuxStatusLineInfo;
  onPress: () => void;
}) {
  const { theme } = useUnistyles();
  const isExited = agent.status === "exited";

  return (
    <Pressable
      onPress={onPress}
      style={({ pressed }) => [
        styles.agentCard,
        pressed ? { opacity: 0.8 } : null,
        isExited ? { opacity: 0.6 } : null,
      ]}
    >
      <View style={styles.agentCardHeader}>
        <View style={styles.agentNameRow}>
          <Terminal size={16} color={isExited ? theme.colors.foregroundMuted : theme.colors.primary} />
          <Text style={[styles.agentName, isExited && { color: theme.colors.foregroundMuted }]}>
            {agent.agentName}
          </Text>
          {isExited ? (
            <View style={styles.exitedBadge}>
              <Text style={styles.exitedBadgeText}>exited</Text>
            </View>
          ) : null}
        </View>
        <Text style={styles.serverLabel}>{agent.serverLabel}</Text>
      </View>
      {agent.workingDir ? (
        <Text style={styles.agentTitle} numberOfLines={1}>
          {shortenPath(agent.workingDir)}
        </Text>
      ) : null}
      {statusLine ? (
        <View
          style={[
            styles.statusLineContainer,
            { backgroundColor: theme.colors.surface1 },
          ]}
        >
          {statusLine.statusRight ? (() => {
            const { title, timeDate } = splitStatusRight(statusLine.statusRight);
            return (
              <>
                <AnsiTextContent
                  segments={parseAnsi(title)}
                  style={[
                    styles.statusLineText,
                    { color: theme.colors.foreground },
                  ]}
                />
                {timeDate ? (
                  <AnsiTextContent
                    segments={parseAnsi(timeDate)}
                    style={[
                      styles.statusLineText,
                      { color: theme.colors.foreground },
                    ]}
                  />
                ) : null}
              </>
            );
          })() : null}
        </View>
      ) : null}
      <View style={styles.agentCardBody}>
        <Text style={styles.detailLabel}>
          S:{agent.sessionName} W:{agent.windowName} P:{agent.paneId} PID:{agent.panePid}
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

function PaneCard({
  pane,
  onPress,
}: {
  pane: TmuxPane;
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
          <SquareTerminal size={16} color={theme.colors.foregroundMuted} />
          <Text style={styles.agentName}>{pane.currentCmd}</Text>
        </View>
        <Text style={styles.serverLabel}>{pane.serverLabel}</Text>
      </View>
      {pane.workingDir ? (
        <Text style={styles.agentTitle} numberOfLines={1}>
          {shortenPath(pane.workingDir)}
        </Text>
      ) : null}
      <View style={styles.agentCardBody}>
        <Text style={styles.detailLabel}>
          S:{pane.sessionName} W:{pane.windowName} P:{pane.paneId} PID:{pane.panePid}
        </Text>
      </View>
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

type DashboardTab = "agents" | "other-panes";

function TmuxDashboardScreenInner() {
  const { theme } = useUnistyles();
  const isCompact = useIsCompactFormFactor();
  const storeReady = useStoreReady();
  const { agents, otherPanes, isLoading, isInitialLoad, error, refreshAll } =
    useAggregatedTmuxAgents();
  const statusLines = useTmuxStatusLines(agents);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [selectedCmd, setSelectedCmd] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<DashboardTab>("agents");
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

  const handlePanePress = (pane: TmuxPane) => {
    setSelectedAgent(pane);
    router.push("/tmux-pane");
  };

  const cmdGroups = useMemo(() => {
    const map = new Map<string, number>();
    for (const pane of otherPanes) {
      map.set(pane.currentCmd, (map.get(pane.currentCmd) ?? 0) + 1);
    }
    return Array.from(map.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
  }, [otherPanes]);

  const filteredOtherPanes = useMemo(() => {
    if (!selectedCmd) return otherPanes;
    return otherPanes.filter((p) => p.currentCmd === selectedCmd);
  }, [otherPanes, selectedCmd]);

  const badgeText = activeTab === "agents"
    ? `${agents.length} agent(s)${agents.filter((a) => a.status === "exited").length > 0
        ? `, ${agents.filter((a) => a.status === "exited").length} exited`
        : ""}`
    : `${otherPanes.length} pane(s)`;

  const hasData = agents.length > 0 || otherPanes.length > 0;

  return (
    <View style={styles.container}>
      <BackHeader
        title="Tmux Dashboard"
        onBack={() => router.navigate("/")}
        rightContent={
          hasData ? (
            <View style={styles.badge}>
              <Text style={styles.badgeText}>{badgeText}</Text>
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
      ) : !hasData ? (
        <View style={styles.centerContent}>
          <Monitor size={48} color={theme.colors.foregroundMuted} />
          <Text style={styles.emptyTitle}>No tmux panes detected</Text>
          <Text style={styles.emptySubtitle}>
            Open a tmux session to see panes here.
          </Text>
          <Pressable
            style={styles.refreshButton}
            onPress={refreshAll}
          >
            <RefreshCw size={16} color={theme.colors.primary} />
            <Text style={styles.refreshButtonText}>Refresh</Text>
          </Pressable>
        </View>
      ) : (
        <ScrollView
          style={styles.scrollView}
          refreshControl={
            <RefreshControl refreshing={isLoading} onRefresh={refreshAll} />
          }
        >
          {/* Segmented toggle */}
          <View style={styles.segmentedRow}>
            <Pressable
              onPress={() => setActiveTab("agents")}
              style={[
                styles.segmentButton,
                activeTab === "agents" && { backgroundColor: theme.colors.primary },
              ]}
            >
              <Text
                style={[
                  styles.segmentText,
                  { color: activeTab === "agents" ? theme.colors.background : theme.colors.foregroundMuted },
                ]}
              >
                Agents ({agents.length})
              </Text>
            </Pressable>
            <Pressable
              onPress={() => setActiveTab("other-panes")}
              style={[
                styles.segmentButton,
                activeTab === "other-panes" && { backgroundColor: theme.colors.primary },
              ]}
            >
              <Text
                style={[
                  styles.segmentText,
                  { color: activeTab === "other-panes" ? theme.colors.background : theme.colors.foregroundMuted },
                ]}
              >
                Other Panes ({otherPanes.length})
              </Text>
            </Pressable>
          </View>

          {activeTab === "agents" ? (
            <>
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
                    statusLine={statusLines.find(
                      (s) => s.serverId === agent.serverId && s.sessionName === agent.sessionName,
                    )}
                    onPress={() => handleAgentPress(agent)}
                  />
                ))}
              </View>
            </>
          ) : (
            <>
              {cmdGroups.length > 0 ? (
                <ScrollView
                  horizontal
                  showsHorizontalScrollIndicator={false}
                  style={styles.nameCardsScroll}
                  contentContainerStyle={styles.nameCardsContainer}
                >
                  {cmdGroups.map((group) => (
                    <NameCard
                      key={group.name}
                      group={group}
                      isActive={selectedCmd === group.name}
                      onPress={() => setSelectedCmd((prev) => (prev === group.name ? null : group.name))}
                    />
                  ))}
                </ScrollView>
              ) : null}

              <View
                style={isCompact ? styles.agentGridCompact : styles.agentGrid}
              >
                {filteredOtherPanes.map((pane, index) => (
                  <PaneCard
                    key={`${pane.serverId}-${pane.paneId}-${index}`}
                    pane={pane}
                    onPress={() => handlePanePress(pane)}
                  />
                ))}
              </View>
            </>
          )}
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
  refreshButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginTop: 16,
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: theme.colors.surface0,
  },
  refreshButtonText: {
    color: theme.colors.primary,
    fontSize: 14,
    fontWeight: "500",
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
  agentTitle: {
    color: theme.colors.foreground,
    fontSize: 13,
    fontWeight: "500",
    marginBottom: 4,
  },
  agentCardBody: {
    gap: 4,
  },
  detailLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
  },
  statusLineContainer: {
    backgroundColor: theme.colors.background,
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 6,
    marginBottom: 10,
    gap: 2,
  },
  statusLineText: {
    color: theme.colors.foreground,
    fontSize: 11,
    fontFamily: "monospace",
  },
  exitedBadge: {
    backgroundColor: theme.colors.surface1,
    paddingHorizontal: 6,
    paddingVertical: 2,
    borderRadius: 4,
    marginLeft: 6,
  },
  exitedBadgeText: {
    color: theme.colors.foregroundMuted,
    fontSize: 10,
    fontWeight: "500",
  },
  segmentedRow: {
    flexDirection: "row",
    marginHorizontal: 16,
    marginTop: 12,
    marginBottom: 4,
    borderRadius: 8,
    backgroundColor: theme.colors.surface0,
    overflow: "hidden",
  },
  segmentButton: {
    flex: 1,
    paddingVertical: 8,
    alignItems: "center",
    borderRadius: 8,
  },
  segmentText: {
    fontSize: 13,
    fontWeight: "600",
  },
  sessionGroup: {
    marginBottom: 8,
  },
  sessionGroupLabel: {
    fontSize: 12,
    fontWeight: "600",
    paddingHorizontal: 16,
    paddingTop: 12,
    paddingBottom: 4,
  },
}));
