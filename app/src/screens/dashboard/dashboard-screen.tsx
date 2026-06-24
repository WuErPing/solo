import { useCallback, useMemo, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ScrollView,
  RefreshControl,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { router } from "expo-router";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock,
  LayoutDashboard,
  Terminal,
} from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { useAggregatedAgents } from "@/hooks/use-aggregated-agents";
import { deriveSidebarStateBucket } from "@/utils/sidebar-agent-state";
import { getStatusDotColor } from "@/utils/status-dot-color";
import { formatTimeAgo } from "@/utils/time";
import { shortenPath } from "@/utils/shorten-path";
import { useIsCompactFormFactor } from "@/constants/layout";
import type { AggregatedAgent } from "@/hooks/use-aggregated-agents";
import type { SidebarStateBucket } from "@/utils/sidebar-agent-state";
import { buildHostAgentDetailRoute } from "@/utils/host-routes";

interface StatusGroup {
  bucket: SidebarStateBucket;
  label: string;
  icon: typeof Activity;
  agents: AggregatedAgent[];
}

function getAgentBucket(agent: AggregatedAgent): SidebarStateBucket {
  return deriveSidebarStateBucket({
    status: agent.status,
    pendingPermissionCount: agent.pendingPermissionCount,
    requiresAttention: agent.requiresAttention,
    attentionReason: agent.attentionReason,
  });
}

function StatusCard({
  label,
  count,
  icon: Icon,
  color,
  isActive,
  onPress,
}: {
  label: string;
  count: number;
  icon: typeof Activity;
  color: string;
  isActive: boolean;
  onPress: () => void;
}) {
  useUnistyles();

  return (
    <Pressable
      onPress={onPress}
      style={({ pressed }) => [
        styles.statusCard,
        isActive && styles.statusCardActive,
        pressed && styles.statusCardPressed,
      ]}
    >
      <View style={[styles.statusIconWrap, { backgroundColor: color + "18" }]}>
        <Icon size={20} color={color} />
      </View>
      <Text style={styles.statusCount}>{count}</Text>
      <Text style={styles.statusLabel}>{label}</Text>
    </Pressable>
  );
}

function AgentCard({ agent }: { agent: AggregatedAgent }) {
  const { theme } = useUnistyles();
  const bucket = getAgentBucket(agent);
  const statusColor = getStatusDotColor({ theme, bucket }) ?? theme.colors.foregroundMuted;
  const timeAgo = formatTimeAgo(agent.lastActivityAt);
  const projectPath = shortenPath(agent.cwd);

  const handlePress = useCallback(() => {
    const route = buildHostAgentDetailRoute(agent.serverId, agent.id);
    router.navigate(route as never);
  }, [agent.serverId, agent.id]);

  return (
    <Pressable
      onPress={handlePress}
      style={({ pressed }) => [styles.agentCard, pressed && styles.agentCardPressed]}
    >
      <View style={styles.agentCardHeader}>
        <View style={[styles.statusIndicator, { backgroundColor: statusColor }]} />
        <Text style={styles.agentTitle} numberOfLines={1}>
          {agent.title || "New session"}
        </Text>
      </View>

      <View style={styles.agentCardMeta}>
        <View style={styles.metaRow}>
          <Terminal size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText} numberOfLines={1}>
            {agent.serverLabel}
          </Text>
        </View>
        <View style={styles.metaRow}>
          <Clock size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText}>{timeAgo}</Text>
        </View>
        {projectPath ? (
          <View style={styles.metaRow}>
            <Activity size={12} color={theme.colors.foregroundMuted} />
            <Text style={styles.metaText} numberOfLines={1}>
              {projectPath}
            </Text>
          </View>
        ) : null}
      </View>

      {(agent.pendingPermissionCount ?? 0) > 0 ? (
        <View style={styles.badge}>
          <AlertTriangle size={12} color={theme.colors.palette.amber[500]} />
          <Text style={styles.badgeText}>{agent.pendingPermissionCount} pending</Text>
        </View>
      ) : null}
    </Pressable>
  );
}

export function DashboardScreen() {
  const { theme } = useUnistyles();
  const insets = useSafeAreaInsets();
  const isCompact = useIsCompactFormFactor();
  const { agents, isInitialLoad, isRevalidating, refreshAll } = useAggregatedAgents();

  const [selectedBucket, setSelectedBucket] = useState<SidebarStateBucket | null>(null);

  const groups = useMemo<StatusGroup[]>(() => {
    const buckets: Record<SidebarStateBucket, StatusGroup> = {
      running: {
        bucket: "running",
        label: "Running",
        icon: Activity,
        agents: [],
      },
      needs_input: {
        bucket: "needs_input",
        label: "Needs Input",
        icon: AlertTriangle,
        agents: [],
      },
      failed: {
        bucket: "failed",
        label: "Failed",
        icon: AlertTriangle,
        agents: [],
      },
      attention: {
        bucket: "attention",
        label: "Attention",
        icon: CheckCircle2,
        agents: [],
      },
      done: {
        bucket: "done",
        label: "Idle",
        icon: Clock,
        agents: [],
      },
    };

    for (const agent of agents) {
      const bucket = getAgentBucket(agent);
      buckets[bucket].agents.push(agent);
    }

    return Object.values(buckets);
  }, [agents]);

  const displayedAgents = useMemo(() => {
    if (!selectedBucket) return agents;
    return agents.filter((agent) => getAgentBucket(agent) === selectedBucket);
  }, [agents, selectedBucket]);

  const handleRefresh = useCallback(() => {
    refreshAll();
  }, [refreshAll]);

  const totalCount = agents.length;
  const activeCount = agents.filter(
    (a) => getAgentBucket(a) !== "done"
  ).length;

  return (
    <View style={styles.container}>
      <BackHeader
        title="Agents"
        onBack={() => router.navigate("/")}
        rightContent={
          <View style={styles.headerBadge}>
            <Text style={styles.headerBadgeText}>
              {activeCount}/{totalCount} active
            </Text>
          </View>
        }
      />

      <ScrollView
        style={styles.scrollView}
        contentContainerStyle={[
          styles.content,
          { paddingBottom: insets.bottom + 16 },
        ]}
        refreshControl={
          <RefreshControl
            refreshing={isRevalidating}
            onRefresh={handleRefresh}
            tintColor={theme.colors.foregroundMuted}
          />
        }
      >
        {/* Status Summary Cards */}
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.statusCardsContainer}
        >
          {groups.map((group) => {
            if (group.agents.length === 0) return null;
            const color =
              getStatusDotColor({ theme, bucket: group.bucket }) ??
              theme.colors.foregroundMuted;
            return (
              <StatusCard
                key={group.bucket}
                label={group.label}
                count={group.agents.length}
                icon={group.icon}
                color={color}
                isActive={selectedBucket === group.bucket}
                onPress={() =>
                  setSelectedBucket(
                    selectedBucket === group.bucket ? null : group.bucket
                  )
                }
              />
            );
          })}
        </ScrollView>

        {/* Agent Grid */}
        {isInitialLoad ? (
          <View style={styles.loadingContainer}>
            <Activity size={32} color={theme.colors.foregroundMuted} />
            <Text style={styles.loadingText}>Loading agents...</Text>
          </View>
        ) : displayedAgents.length === 0 ? (
          <View style={styles.emptyContainer}>
            <LayoutDashboard size={48} color={theme.colors.foregroundMuted} />
            <Text style={styles.emptyTitle}>
              {selectedBucket ? "No agents in this category" : "No agents yet"}
            </Text>
            <Text style={styles.emptySubtitle}>
              {selectedBucket
                ? "Try selecting a different filter"
                : "Connect to a host to see your agents"}
            </Text>
          </View>
        ) : (
          <View style={isCompact ? styles.agentGridCompact : styles.agentGrid}>
            {displayedAgents.map((agent) => (
              <AgentCard key={`${agent.serverId}:${agent.id}`} agent={agent} />
            ))}
          </View>
        )}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
  },
  scrollView: {
    flex: 1,
  },
  content: {
    padding: 16,
    gap: 16,
  },
  headerBadge: {
    backgroundColor: theme.colors.surface2,
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: theme.borderRadius.full,
  },
  headerBadgeText: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
    color: theme.colors.foregroundMuted,
  },
  statusCardsContainer: {
    gap: 12,
    paddingHorizontal: 4,
  },
  statusCard: {
    width: 120,
    padding: 16,
    borderRadius: theme.borderRadius.xl,
    backgroundColor: theme.colors.surface1,
    alignItems: "center",
    gap: 8,
    borderWidth: 2,
    borderColor: "transparent",
  },
  statusCardActive: {
    borderColor: theme.colors.primary,
  },
  statusCardPressed: {
    opacity: 0.8,
  },
  statusIconWrap: {
    width: 40,
    height: 40,
    borderRadius: theme.borderRadius.lg,
    alignItems: "center",
    justifyContent: "center",
  },
  statusCount: {
    fontSize: 24,
    fontWeight: theme.fontWeight.bold,
    color: theme.colors.foreground,
  },
  statusLabel: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
    fontWeight: theme.fontWeight.medium,
  },
  agentGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 12,
  },
  agentGridCompact: {
    gap: 12,
  },
  agentCard: {
    flex: 1,
    minWidth: 280,
    maxWidth: "100%",
    padding: 16,
    borderRadius: theme.borderRadius.xl,
    backgroundColor: theme.colors.surface1,
    gap: 12,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  agentCardPressed: {
    backgroundColor: theme.colors.surface2,
  },
  agentCardHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  statusIndicator: {
    width: 8,
    height: 8,
    borderRadius: theme.borderRadius.full,
  },
  agentTitle: {
    flex: 1,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.semibold,
    color: theme.colors.foreground,
  },
  agentCardMeta: {
    gap: 6,
  },
  metaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  metaText: {
    flex: 1,
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
  badge: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    alignSelf: "flex-start",
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: theme.borderRadius.full,
    backgroundColor: theme.colors.surface2,
  },
  badgeText: {
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.medium,
    color: theme.colors.palette.amber[500],
  },
  loadingContainer: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 12,
    paddingVertical: 60,
  },
  loadingText: {
    fontSize: theme.fontSize.base,
    color: theme.colors.foregroundMuted,
  },
  emptyContainer: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 12,
    paddingVertical: 60,
  },
  emptyTitle: {
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.semibold,
    color: theme.colors.foreground,
  },
  emptySubtitle: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
}));
