import React, { useEffect, useMemo, useState, useCallback } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  RefreshControl,
  TextInput,
} from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Terminal, Monitor, RefreshCw, SquareTerminal, Clock, Plus, Play, X } from "lucide-react-native";
import { router } from "expo-router";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { useAggregatedTmuxAgents } from "@/hooks/use-tmux-agents";
import { useTmuxStatusLines } from "@/hooks/use-tmux-status-lines";
import { useTmuxNewSession } from "@/hooks/use-tmux-new-session";
import { useTmuxKillSession } from "@/hooks/use-tmux-kill-session";
import { useTmuxDeleteCommandHistory } from "@/hooks/use-tmux-delete-command-history";
import { useIsCompactFormFactor } from "@/constants/layout";
import { useStoreReady } from "@/app/_layout";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { useHosts } from "@/runtime/host-runtime";
import { shortenPath } from "@/utils/shorten-path";
import { confirmDialog } from "@/utils/confirm-dialog";
import { parseAnsi } from "@/utils/ansi-parser";
import { AnsiTextContent } from "@/components/ansi-text-renderer";
import type { TmuxAgent, TmuxPane, AgentCommandEntry } from "@/hooks/use-tmux-agents";
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
  onClose,
}: {
  agent: TmuxAgent;
  statusLine?: TmuxStatusLineInfo;
  onPress: () => void;
  onClose: () => void;
}) {
  const { theme } = useUnistyles();
  const isExited = agent.status === "exited";

  return (
    <View
      style={[
        styles.agentCard,
        isExited ? { opacity: 0.6 } : null,
      ]}
    >
      <View style={styles.agentCardHeader}>
        <Pressable onPress={onPress} style={styles.agentNameRow}>
          <Terminal size={16} color={isExited ? theme.colors.foregroundMuted : theme.colors.primary} />
          <Text style={[styles.agentName, isExited && { color: theme.colors.foregroundMuted }]}>
            {agent.agentName}
          </Text>
          {isExited ? (
            <View style={styles.exitedBadge}>
              <Text style={styles.exitedBadgeText}>exited</Text>
            </View>
          ) : null}
          {!isExited && agent.activity === "busy" ? (
            <View style={styles.activityBadge}>
              <View style={[styles.activityDot, { backgroundColor: theme.colors.primary }]} />
              <Text style={styles.activityText}>busy</Text>
            </View>
          ) : null}
          {!isExited && agent.activity === "idle" ? (
            <View style={styles.activityBadge}>
              <View style={[styles.activityDot, { backgroundColor: theme.colors.foregroundMuted }]} />
              <Text style={[styles.activityText, { color: theme.colors.foregroundMuted }]}>idle</Text>
            </View>
          ) : null}
        </Pressable>
        <View style={styles.headerRightRow}>
          <Text style={styles.serverLabel}>{agent.serverLabel}</Text>
          <Pressable
            onPress={onClose}
            hitSlop={8}
            data-testid="close-session"
            style={{ padding: 4 }}
          >
            <X size={16} color={theme.colors.destructive} />
          </Pressable>
        </View>
      </View>
      <Pressable onPress={onPress}>
        <View style={styles.sessionBadgeRow}>
          <View style={[styles.sessionBadge, { backgroundColor: theme.colors.surface1 }]}>
            <Text style={styles.sessionBadgeText}>{agent.sessionName}</Text>
          </View>
          <Text style={styles.sessionBadgeSub}>
            W:{agent.windowName} P:{agent.paneId}
          </Text>
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
              const { title } = splitStatusRight(statusLine.statusRight);
              return (
                <AnsiTextContent
                  segments={parseAnsi(title)}
                  style={[
                    styles.statusLineText,
                    { color: theme.colors.foreground },
                  ]}
                />
              );
            })() : null}
          </View>
        ) : null}
        <View style={styles.agentCardBody}>
          <Text style={styles.detailLabel}>
            PID:{agent.panePid}
          </Text>
          {agent.lastContentChange ? (
            <View style={styles.activityTimeRow}>
              {agent.lastContentChangeHHMM ? (
                <Text style={styles.activityTimeText}>
                  {agent.lastContentChangeHHMM}
                </Text>
              ) : null}
              <Clock size={12} color={theme.colors.foregroundMuted} />
              <Text style={styles.activityTimeText}>
                {agent.lastContentChangeAgo}
              </Text>
            </View>
          ) : null}
        </View>
      </Pressable>
    </View>
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
  onClose,
}: {
  pane: TmuxPane;
  onPress: () => void;
  onClose: () => void;
}) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.agentCard}>
      <View style={styles.agentCardHeader}>
        <Pressable onPress={onPress} style={styles.agentNameRow}>
          <SquareTerminal size={16} color={theme.colors.foregroundMuted} />
          <Text style={styles.agentName}>{pane.currentCmd}</Text>
        </Pressable>
        <View style={styles.headerRightRow}>
          <Text style={styles.serverLabel}>{pane.serverLabel}</Text>
          <Pressable
            onPress={onClose}
            hitSlop={8}
            data-testid="close-session"
            style={{ padding: 4 }}
          >
            <X size={16} color={theme.colors.destructive} />
          </Pressable>
        </View>
      </View>
      <Pressable onPress={onPress}>
        <View style={styles.sessionBadgeRow}>
          <View style={[styles.sessionBadge, { backgroundColor: theme.colors.surface1 }]}>
            <Text style={styles.sessionBadgeText}>{pane.sessionName}</Text>
          </View>
          <Text style={styles.sessionBadgeSub}>
            W:{pane.windowName} P:{pane.paneId}
          </Text>
        </View>
        {pane.workingDir ? (
          <Text style={styles.agentTitle} numberOfLines={1}>
            {shortenPath(pane.workingDir)}
          </Text>
        ) : null}
        <View style={styles.agentCardBody}>
          <Text style={styles.detailLabel}>
            PID:{pane.panePid}
          </Text>
          {pane.lastContentChange ? (
            <View style={styles.activityTimeRow}>
              {pane.lastContentChangeHHMM ? (
                <Text style={styles.activityTimeText}>
                  {pane.lastContentChangeHHMM}
                </Text>
              ) : null}
              <Clock size={12} color={theme.colors.foregroundMuted} />
              <Text style={styles.activityTimeText}>
                {pane.lastContentChangeAgo}
              </Text>
            </View>
          ) : null}
        </View>
      </Pressable>
    </View>
  );
}

function CommandHistoryCard({
  entry,
  onRun,
  onDelete,
}: {
  entry: AgentCommandEntry;
  onRun: () => void;
  onDelete: () => void;
}) {
  const { theme } = useUnistyles();
  const relativeTime = useMemo(() => {
    const diff = Date.now() - new Date(entry.lastSeen).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "just now";
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  }, [entry.lastSeen]);

  return (
    <Pressable
      onPress={onRun}
      style={({ pressed }) => [
        styles.historyCard,
        pressed ? { opacity: 0.8 } : null,
      ]}
    >
      <View style={styles.historyCardHeader}>
        <View style={styles.agentNameRow}>
          <Clock size={14} color={theme.colors.foregroundMuted} />
          <Text style={styles.historyAgentName}>{entry.agentName}</Text>
        </View>
        <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
          <Text style={styles.historyTime}>{relativeTime}</Text>
          <Pressable onPress={onDelete} hitSlop={8} data-testid="delete-command" style={{ padding: 4 }}>
            <X size={14} color={theme.colors.destructive} />
          </Pressable>
        </View>
      </View>
      <View style={styles.historyCmdRow}>
        <Text style={styles.historyCmd} numberOfLines={1}>
          {entry.launchCmd}
        </Text>
        <Play size={14} color={theme.colors.primary} />
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

type DashboardTab = "agents" | "other-panes" | "history";

function TmuxDashboardScreenInner() {
  const { theme } = useUnistyles();
  const isCompact = useIsCompactFormFactor();
  const storeReady = useStoreReady();
  const hosts = useHosts();
  const { agents, otherPanes, commandHistory, isLoading, isInitialLoad, error, refreshAll } =
    useAggregatedTmuxAgents();
  const statusLines = useTmuxStatusLines(agents);
  const { createSession, isLoading: isCreating, error: createError } = useTmuxNewSession();
  const { killSession } = useTmuxKillSession();
  const { deleteCommand } = useTmuxDeleteCommandHistory();
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [selectedCmd, setSelectedCmd] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<DashboardTab>("agents");
  const [showNewSessionInput, setShowNewSessionInput] = useState(false);
  const [newSessionName, setNewSessionName] = useState("");
  const setSelectedAgent = useTmuxAgentStore((s) => s.setSelectedAgent);

  // Refresh tmux agents on launch so the daemon updates ~/.solo/agent-commands.json.
  useEffect(() => {
    refreshAll();
  }, [refreshAll]);

  const firstConnectedServerId = useMemo(() => {
    return hosts.length > 0 ? hosts[0].serverId : null;
  }, [hosts]);

  const handleCreateSession = useCallback(async () => {
    if (!firstConnectedServerId || !newSessionName.trim()) return;
    const name = newSessionName.trim();
    const sessionName = await createSession(firstConnectedServerId, { name });
    if (sessionName) {
      setShowNewSessionInput(false);
      setNewSessionName("");
      refreshAll();
    }
  }, [firstConnectedServerId, newSessionName, createSession, refreshAll]);

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

  const handleRunCommand = useCallback(async (entry: AgentCommandEntry) => {
    if (!firstConnectedServerId) return;
    const confirmed = await confirmDialog({
      title: "Run Command",
      message: `Launch "${entry.launchCmd}" in a new tmux session?`,
      confirmLabel: "Run",
      cancelLabel: "Cancel",
    });
    if (!confirmed) return;
    const name = `${entry.agentName}-${Date.now()}`;
    const sessionName = await createSession(firstConnectedServerId, {
      name,
      command: entry.launchCmd,
    });
    if (sessionName) {
      await refreshAll();
      setActiveTab("agents");
    }
  }, [firstConnectedServerId, createSession, refreshAll]);

  const handleCloseSession = useCallback(async (sessionName: string) => {
    if (!firstConnectedServerId) return;
    const confirmed = await confirmDialog({
      title: "Close Session",
      message: `Kill tmux session "${sessionName}"?`,
      confirmLabel: "Close",
      cancelLabel: "Cancel",
      destructive: true,
    });
    if (!confirmed) return;
    const ok = await killSession(firstConnectedServerId, sessionName);
    if (ok) refreshAll();
  }, [firstConnectedServerId, killSession, refreshAll]);

  const handleDeleteCommand = useCallback(async (entry: AgentCommandEntry) => {
    if (!firstConnectedServerId) return;
    const confirmed = await confirmDialog({
      title: "Delete Command",
      message: `Remove "${entry.launchCmd}" from history?`,
      confirmLabel: "Delete",
      cancelLabel: "Cancel",
      destructive: true,
    });
    if (!confirmed) return;
    const ok = await deleteCommand(firstConnectedServerId, entry.launchCmd);
    if (ok) refreshAll();
  }, [firstConnectedServerId, deleteCommand, refreshAll]);

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
    : activeTab === "other-panes"
      ? `${otherPanes.length} pane(s)`
      : `${commandHistory.length} cmd(s)`;

  const hasData = agents.length > 0 || otherPanes.length > 0;

  return (
    <View style={styles.container}>
      <BackHeader
        title="Tmux Dashboard"
        onBack={() => router.navigate("/")}
        rightContent={
          <View style={styles.headerRightRow}>
            {firstConnectedServerId ? (
              <Pressable
                style={[styles.headerButton, { backgroundColor: theme.colors.primary }]}
                onPress={() => setShowNewSessionInput((prev) => !prev)}
              >
                <Plus size={16} color={theme.colors.background} />
                <Text style={[styles.headerButtonText, { color: theme.colors.background }]}>New</Text>
              </Pressable>
            ) : null}
            {hasData ? (
              <View style={styles.badge}>
                <Text style={styles.badgeText}>{badgeText}</Text>
              </View>
            ) : undefined}
          </View>
        }
      />

      {showNewSessionInput ? (
        <View style={[styles.newSessionRow, { backgroundColor: theme.colors.surface0 }]}>
          <TextInput
            style={[styles.newSessionInput, { color: theme.colors.foreground, borderColor: theme.colors.border }]}
            placeholder="session name"
            placeholderTextColor={theme.colors.foregroundMuted}
            value={newSessionName}
            onChangeText={setNewSessionName}
            autoFocus
            onSubmitEditing={handleCreateSession}
          />
          <Pressable
            style={[styles.newSessionButton, { backgroundColor: theme.colors.primary }]}
            onPress={handleCreateSession}
            disabled={isCreating || !newSessionName.trim()}
          >
            <Text style={[styles.newSessionButtonText, { color: theme.colors.background }]}>
              {isCreating ? "..." : "Create"}
            </Text>
          </Pressable>
          <Pressable
            style={[styles.newSessionButton, { backgroundColor: theme.colors.surface1 }]}
            onPress={() => { setShowNewSessionInput(false); setNewSessionName(""); }}
          >
            <Text style={[styles.newSessionButtonText, { color: theme.colors.foregroundMuted }]}>Cancel</Text>
          </Pressable>
          {createError ? (
            <Text style={styles.errorText}>{createError}</Text>
          ) : null}
        </View>
      ) : null}

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
          <View style={styles.emptyActionsRow}>
            {firstConnectedServerId ? (
              <Pressable
                style={[styles.refreshButton, { backgroundColor: theme.colors.primary }]}
                onPress={() => setShowNewSessionInput(true)}
              >
                <Plus size={16} color={theme.colors.background} />
                <Text style={[styles.refreshButtonText, { color: theme.colors.background }]}>New tmux</Text>
              </Pressable>
            ) : null}
            <Pressable
              style={styles.refreshButton}
              onPress={refreshAll}
            >
              <RefreshCw size={16} color={theme.colors.primary} />
              <Text style={styles.refreshButtonText}>Refresh</Text>
            </Pressable>
          </View>
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
            <Pressable
              onPress={() => setActiveTab("history")}
              style={[
                styles.segmentButton,
                activeTab === "history" && { backgroundColor: theme.colors.primary },
              ]}
            >
              <Text
                style={[
                  styles.segmentText,
                  { color: activeTab === "history" ? theme.colors.background : theme.colors.foregroundMuted },
                ]}
              >
                History ({commandHistory.length})
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
                    onClose={() => handleCloseSession(agent.sessionName)}
                  />
                ))}
              </View>
            </>
          ) : activeTab === "other-panes" ? (
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
                    onClose={() => handleCloseSession(pane.sessionName)}
                  />
                ))}
              </View>
            </>
          ) : (
            <View style={styles.historyList}>
              {commandHistory.length === 0 ? (
                <Text style={styles.loadingText}>No command history yet.</Text>
              ) : null}
              {commandHistory.map((entry, index) => (
                <CommandHistoryCard
                  key={`${entry.launchCmd}-${index}`}
                  entry={entry}
                  onRun={() => void handleRunCommand(entry)}
                  onDelete={() => void handleDeleteCommand(entry)}
                />
              ))}
            </View>
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
    position: "relative",
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
  activityTimeRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
  },
  activityTimeText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
  },
  sessionBadgeRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginBottom: 4,
  },
  sessionBadge: {
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 4,
  },
  sessionBadgeText: {
    color: theme.colors.foreground,
    fontSize: 12,
    fontWeight: "600",
  },
  sessionBadgeSub: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
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
  activityBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    marginLeft: 6,
  },
  activityDot: {
    width: 6,
    height: 6,
    borderRadius: 3,
  },
  activityText: {
    color: theme.colors.primary,
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
  historyList: {
    padding: 16,
    gap: 8,
  },
  historyCard: {
    backgroundColor: theme.colors.surface0,
    borderRadius: 10,
    padding: 14,
  },
  historyCardHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 8,
  },
  historyAgentName: {
    color: theme.colors.primary,
    fontSize: 13,
    fontWeight: "600",
  },
  historyTime: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
  },
  historyCmdRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    gap: 8,
  },
  historyCmd: {
    color: theme.colors.foreground,
    fontSize: 13,
    fontFamily: "monospace",
    flex: 1,
  },
  headerRightRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  headerButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: 10,
    paddingVertical: 6,
    borderRadius: 8,
  },
  headerButtonText: {
    fontSize: 13,
    fontWeight: "600",
  },
  newSessionRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    paddingHorizontal: 16,
    paddingVertical: 10,
  },
  newSessionInput: {
    flex: 1,
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 8,
    fontSize: 14,
  },
  newSessionButton: {
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 8,
  },
  newSessionButtonText: {
    fontSize: 14,
    fontWeight: "600",
  },
  emptyActionsRow: {
    flexDirection: "row",
    gap: 12,
    marginTop: 16,
  },
}));
