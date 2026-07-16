import { useCallback, useEffect, useMemo, useState } from "react";
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
import { useIsFocused } from "@react-navigation/native";
import {
  Activity,
  Calendar,
  Clock,
  LayoutDashboard,
  Pause,
  Play,
  Plus,
  Server,
} from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { useAggregatedSchedules } from "@/hooks/use-aggregated-schedules";
import { useIsCompactFormFactor } from "@/constants/layout";
import { buildHostSchedulesRoute } from "@/utils/host-routes";
import type { AggregatedSchedule } from "@/hooks/use-aggregated-schedules";
import type { ScheduleStatus } from "@server/server/schedule/types";

interface StatusGroup {
  status: ScheduleStatus;
  label: string;
  icon: typeof Activity;
  color: string;
  count: number;
}

function formatCadenceText(schedule: AggregatedSchedule): string {
  const { cadence } = schedule;
  if (cadence.type === "cron") {
    return cadence.expression;
  }
  const ms = cadence.everyMs;
  if (ms < 60000) {
    return `Every ${ms / 1000}s`;
  }
  if (ms < 3600000) {
    return `Every ${Math.floor(ms / 60000)}m`;
  }
  if (ms < 86400000) {
    return `Every ${Math.floor(ms / 3600000)}h`;
  }
  return `Every ${Math.floor(ms / 86400000)}d`;
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

function ScheduleCard({ schedule }: { schedule: AggregatedSchedule }) {
  const { theme } = useUnistyles();
  const cadenceText = formatCadenceText(schedule);

  const statusColor = useMemo(() => {
    switch (schedule.status) {
      case "active":
        return theme.colors.palette.green[400];
      case "paused":
        return theme.colors.palette.amber[500];
      case "completed":
        return theme.colors.foregroundMuted;
      default:
        return theme.colors.foregroundMuted;
    }
  }, [schedule.status, theme]);

  const handlePress = useCallback(() => {
    const route = buildHostSchedulesRoute(schedule.serverId);
    router.navigate(route as never);
  }, [schedule.serverId]);

  return (
    <Pressable
      onPress={handlePress}
      style={({ pressed }) => [styles.scheduleCard, pressed && styles.scheduleCardPressed]}
    >
      <View style={styles.scheduleCardHeader}>
        <View style={styles.scheduleCardTitleRow}>
          <Calendar size={16} color={theme.colors.foregroundMuted} />
          <Text style={styles.scheduleCardTitle} numberOfLines={1}>
            {schedule.name ?? "Untitled"}
          </Text>
        </View>
        <View style={[styles.statusBadge, { borderColor: statusColor }]}>
          <View style={[styles.statusDot, { backgroundColor: statusColor }]} />
          <Text style={[styles.statusText, { color: statusColor }]}>
            {schedule.status}
          </Text>
        </View>
      </View>

      <View style={styles.scheduleCardMeta}>
        <View style={styles.metaRow}>
          <Clock size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText}>{cadenceText}</Text>
        </View>
        <View style={styles.metaRow}>
          <Server size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText} numberOfLines={1}>
            {schedule.serverLabel}
          </Text>
        </View>
      </View>
    </Pressable>
  );
}

export function SchedulesDashboardScreen() {
  const { theme } = useUnistyles();
  const insets = useSafeAreaInsets();
  const isCompact = useIsCompactFormFactor();
  const isFocused = useIsFocused();
  const {
    schedules,
    configuredHosts,
    connectedHosts,
    isInitialLoad,
    isRevalidating,
    error,
    refreshAll,
  } = useAggregatedSchedules();

  const [selectedStatus, setSelectedStatus] = useState<ScheduleStatus | null>(null);

  useEffect(() => {
    if (isFocused) {
      refreshAll();
    }
  }, [isFocused, refreshAll]);

  const groups = useMemo<StatusGroup[]>(() => {
    const activeCount = schedules.filter((s) => s.status === "active").length;
    const pausedCount = schedules.filter((s) => s.status === "paused").length;
    const completedCount = schedules.filter((s) => s.status === "completed").length;

    return [
      {
        status: "active",
        label: "Active",
        icon: Play,
        color: theme.colors.palette.green[400],
        count: activeCount,
      },
      {
        status: "paused",
        label: "Paused",
        icon: Pause,
        color: theme.colors.palette.amber[500],
        count: pausedCount,
      },
      {
        status: "completed",
        label: "Completed",
        icon: Activity,
        color: theme.colors.foregroundMuted,
        count: completedCount,
      },
    ].filter((g) => g.count > 0) as StatusGroup[];
  }, [schedules, theme]);

  const displayedSchedules = useMemo(() => {
    if (!selectedStatus) return schedules;
    return schedules.filter((s) => s.status === selectedStatus);
  }, [schedules, selectedStatus]);

  const totalCount = schedules.length;
  const activeCount = schedules.filter((s) => s.status === "active").length;

  const handleRefresh = useCallback(() => {
    refreshAll();
  }, [refreshAll]);

  const handleNewSchedule = useCallback(() => {
    const firstHost = connectedHosts[0];
    if (!firstHost) {
      return;
    }
    const route = `${buildHostSchedulesRoute(firstHost.serverId)}?create=true`;
    router.navigate(route as never);
  }, [connectedHosts]);

  return (
    <View style={styles.container}>
      <BackHeader
        title="Schedules"
        onBack={() => router.navigate("/")}
        rightContent={
          <View style={styles.headerRight}>
            {connectedHosts.length > 0 ? (
              <Button
                variant="ghost"
                size="sm"
                leftIcon={Plus}
                onPress={handleNewSchedule}
                testID="new-schedule-button"
              >
                New
              </Button>
            ) : null}
            <View style={styles.headerBadge}>
              <Text style={styles.headerBadgeText}>
                {activeCount}/{totalCount} active
              </Text>
            </View>
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
        {groups.length > 0 ? (
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.statusCardsContainer}
          >
            {groups.map((group) => (
              <StatusCard
                key={group.status}
                label={group.label}
                count={group.count}
                icon={group.icon}
                color={group.color}
                isActive={selectedStatus === group.status}
                onPress={() =>
                  setSelectedStatus(
                    selectedStatus === group.status ? null : group.status
                  )
                }
              />
            ))}
          </ScrollView>
        ) : null}

        {error ? (
          <View style={styles.errorBanner}>
            <Text style={styles.errorText}>{error}</Text>
          </View>
        ) : null}

        {/* Schedule Grid */}
        {isInitialLoad ? (
          <View style={styles.loadingContainer}>
            <Activity size={32} color={theme.colors.foregroundMuted} />
            <Text style={styles.loadingText}>Loading schedules...</Text>
          </View>
        ) : displayedSchedules.length === 0 ? (
          <View style={styles.emptyContainer}>
            <LayoutDashboard size={48} color={theme.colors.foregroundMuted} />
            {configuredHosts.length === 0 ? (
              <>
                <Text style={styles.emptyTitle}>No hosts connected</Text>
                <Text style={styles.emptySubtitle}>
                  Connect to a host to see your schedules
                </Text>
              </>
            ) : connectedHosts.length === 0 ? (
              <>
                <Text style={styles.emptyTitle}>Hosts are offline</Text>
                <Text style={styles.emptySubtitle}>
                  Connect to a host to see your schedules
                </Text>
              </>
            ) : selectedStatus ? (
              <>
                <Text style={styles.emptyTitle}>No schedules in this category</Text>
                <Text style={styles.emptySubtitle}>
                  Try selecting a different filter
                </Text>
              </>
            ) : (
              <>
                <Text style={styles.emptyTitle}>No schedules yet</Text>
                <Text style={styles.emptySubtitle}>
                  Create a schedule to get started
                </Text>
              </>
            )}
          </View>
        ) : (
          <View style={isCompact ? styles.scheduleGridCompact : styles.scheduleGrid}>
            {displayedSchedules.map((schedule) => (
              <ScheduleCard key={`${schedule.serverId}:${schedule.id}`} schedule={schedule} />
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
  headerRight: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
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
  scheduleGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 12,
  },
  scheduleGridCompact: {
    gap: 12,
  },
  scheduleCard: {
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
  scheduleCardPressed: {
    backgroundColor: theme.colors.surface2,
  },
  scheduleCardHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
  },
  scheduleCardTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    flexShrink: 1,
  },
  scheduleCardTitle: {
    flex: 1,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.semibold,
    color: theme.colors.foreground,
  },
  statusBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingVertical: 4,
    paddingHorizontal: 10,
    borderRadius: theme.borderRadius.full,
    borderWidth: 1,
    flexShrink: 0,
  },
  statusDot: {
    width: 6,
    height: 6,
    borderRadius: theme.borderRadius.full,
  },
  statusText: {
    fontSize: theme.fontSize.sm,
    textTransform: "capitalize",
  },
  scheduleCardMeta: {
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
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: 12,
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
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
