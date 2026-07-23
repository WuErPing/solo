import { useCallback, useMemo, useState } from "react";
import {
  View,
  Text,
  ScrollView,
  RefreshControl,
  Pressable,
} from "react-native";
import { useIsFocused } from "@/hooks/use-is-focused";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Calendar, ChevronLeft, Pause, Pencil, Play, Plus, Sparkles, Trash2, Clock } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useSchedules } from "@/hooks/use-schedules";
import { useScheduleMutations } from "@/hooks/use-schedule-mutations";
import { ScheduleCreateModal } from "@/components/schedule-create-modal";
import { ScheduleEditModal } from "@/components/schedule-edit-modal";
import { ScheduleAssistantPanel } from "@/components/schedule-assistant/schedule-assistant-panel";
import { buildHostOpenProjectRoute, buildHostScheduleDetailRoute } from "@/utils/host-routes";
import { cronFromUTC, describeCron, detectTimezone } from "@/utils/cron-timezone";
import type { ScheduleSummary, ScheduleCadence, ScheduleStatus } from "@server/server/schedule/types";

export function SchedulesScreen({
  serverId,
  initialCreateOpen = false,
}: {
  serverId: string;
  initialCreateOpen?: boolean;
}) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return <SchedulesScreenContent serverId={serverId} initialCreateOpen={initialCreateOpen} />;
}

function formatCadence(cadence: ScheduleCadence): string {
  if (cadence.type === "cron") {
    const tz = cadence.timezone || detectTimezone();
    if (tz && tz !== "UTC") {
      const localExpr = cronFromUTC(cadence.expression, tz);
      return `${describeCron(localExpr)} (${tz})`;
    }
    return describeCron(cadence.expression);
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

function statusLabel(status: ScheduleStatus): string {
  return status;
}

function ScheduleStatusBadge({ status }: { status: ScheduleStatus }) {
  const { theme } = useUnistyles();
  const color = useMemo(() => {
    switch (status) {
      case "active":
        return theme.colors.palette.green[400];
      case "paused":
        return theme.colors.palette.amber[500];
      case "completed":
        return theme.colors.foregroundMuted;
      default:
        return theme.colors.foregroundMuted;
    }
  }, [status, theme]);

  return (
    <View style={[styles.statusBadge, { borderColor: color }]}>
      <View style={[styles.statusDot, { backgroundColor: color }]} />
      <Text style={[styles.statusText, { color }]}>{statusLabel(status)}</Text>
    </View>
  );
}

function ScheduleCard({
  serverId,
  schedule,
  onEdit,
  onPause,
  onResume,
  onDelete,
  isPausing,
  isResuming,
  isDeleting,
}: {
  serverId: string;
  schedule: ScheduleSummary;
  onEdit: (schedule: ScheduleSummary) => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onDelete: (id: string) => void;
  isPausing: boolean;
  isResuming: boolean;
  isDeleting: boolean;
}) {
  const { theme } = useUnistyles();
  const name = schedule.name ?? "Untitled";
  const cadenceText = formatCadence(schedule.cadence);

  const handleOpenDetail = useCallback(() => {
    router.navigate(buildHostScheduleDetailRoute(serverId, schedule.id));
  }, [serverId, schedule.id]);

  return (
    <View style={styles.card}>
      <Pressable style={styles.cardHeader} onPress={handleOpenDetail} data-action={`detail-${schedule.id}`}>
        <View style={styles.cardTitleRow}>
          <Calendar size={theme.iconSize.md} color={theme.colors.foregroundMuted} />
          <Text style={styles.cardTitle} numberOfLines={1}>
            {name}
          </Text>
        </View>
        <ScheduleStatusBadge status={schedule.status} />
      </Pressable>

      <View style={styles.cardMetaRow}>
        <Clock size={14} color={theme.colors.foregroundMuted} />
        <Text style={styles.cardMetaText}>{cadenceText}</Text>
      </View>

      <View style={styles.cardActions}>
        <Pressable
          style={styles.actionButton}
          onPress={() => onEdit(schedule)}
          data-action={`edit-${schedule.id}`}
        >
          <Pencil size={16} color={theme.colors.foregroundMuted} />
          <Text style={styles.actionText}>Edit</Text>
        </Pressable>
        {schedule.status === "active" ? (
          <Pressable
            style={styles.actionButton}
            onPress={() => onPause(schedule.id)}
            disabled={isPausing}
            data-action={`pause-${schedule.id}`}
          >
            <Pause size={16} color={theme.colors.foregroundMuted} />
            <Text style={styles.actionText}>{isPausing ? "Pausing..." : "Pause"}</Text>
          </Pressable>
        ) : (
          <Pressable
            style={styles.actionButton}
            onPress={() => onResume(schedule.id)}
            disabled={isResuming}
            data-action={`resume-${schedule.id}`}
          >
            <Play size={16} color={theme.colors.palette.green[400]} />
            <Text style={[styles.actionText, { color: theme.colors.palette.green[400] }]}>
              {isResuming ? "Resuming..." : "Resume"}
            </Text>
          </Pressable>
        )}
        <Pressable
          style={styles.actionButton}
          onPress={() => onDelete(schedule.id)}
          disabled={isDeleting}
          data-action={`delete-${schedule.id}`}
        >
          <Trash2 size={16} color={theme.colors.palette.red[500]} />
          <Text style={[styles.actionText, { color: theme.colors.palette.red[500] }]}>
            {isDeleting ? "Deleting..." : "Delete"}
          </Text>
        </Pressable>
      </View>
    </View>
  );
}

function SchedulesScreenContent({
  serverId,
  initialCreateOpen,
}: {
  serverId: string;
  initialCreateOpen: boolean;
}) {
  const { theme } = useUnistyles();
  const { schedules, isInitialLoad, isRevalidating, error, refreshAll } = useSchedules({ serverId });
  const { pauseSchedule, resumeSchedule, deleteSchedule, isPausing, isResuming, isDeleting } =
    useScheduleMutations({ serverId });

  const [isManualRefresh, setIsManualRefresh] = useState(false);
  const [isCreateModalVisible, setIsCreateModalVisible] = useState(initialCreateOpen);
  const [editingSchedule, setEditingSchedule] = useState<ScheduleSummary | null>(null);
  const [isAssistantOpen, setIsAssistantOpen] = useState(false);
  const [assistantContextScheduleId, setAssistantContextScheduleId] = useState<string | null>(null);

  const handleRefresh = useCallback(() => {
    setIsManualRefresh(true);
    refreshAll();
  }, [refreshAll]);

  const [prevRevalidating, setPrevRevalidating] = useState(isRevalidating);
  if (prevRevalidating !== isRevalidating) {
    setPrevRevalidating(isRevalidating);
    if (!isRevalidating && isManualRefresh) {
      setIsManualRefresh(false);
    }
  }

  const handleBack = useCallback(() => {
    router.navigate(buildHostOpenProjectRoute(serverId));
  }, [serverId]);

  const handlePause = useCallback(
    async (id: string) => {
      try {
        await pauseSchedule(id);
      } catch {
        // Error is handled by mutation; UI can be enhanced later
      }
    },
    [pauseSchedule],
  );

  const handleResume = useCallback(
    async (id: string) => {
      try {
        await resumeSchedule(id);
      } catch {
        // Error is handled by mutation
      }
    },
    [resumeSchedule],
  );

  const handleDelete = useCallback(
    async (id: string) => {
      try {
        await deleteSchedule(id);
      } catch {
        // Error is handled by mutation
      }
    },
    [deleteSchedule],
  );

  const handleOpenCreateModal = useCallback(() => {
    setIsCreateModalVisible(true);
  }, []);

  const handleCloseCreateModal = useCallback(() => {
    setIsCreateModalVisible(false);
  }, []);

  const handleOpenAssistant = useCallback(() => {
    setIsAssistantOpen(true);
  }, []);

  const handleCloseAssistant = useCallback(() => {
    setIsAssistantOpen(false);
    setAssistantContextScheduleId(null);
  }, []);

  const handleOpenAssistantFromEdit = useCallback(() => {
    setAssistantContextScheduleId(editingSchedule?.id ?? null);
    setEditingSchedule(null);
    setIsAssistantOpen(true);
  }, [editingSchedule]);

  const handleOpenEditModal = useCallback((schedule: ScheduleSummary) => {
    setEditingSchedule(schedule);
  }, []);

  const handleCloseEditModal = useCallback(() => {
    setEditingSchedule(null);
  }, []);

  return (
    <View style={styles.container}>
      <BackHeader
        title="Schedules"
        onBack={() => router.navigate("/")}
        rightContent={
          <View style={styles.headerRight}>
            <Button
              variant="ghost"
              size="sm"
              leftIcon={Sparkles}
              onPress={handleOpenAssistant}
              testID="schedule-assistant-button"
            >
              Ask AI
            </Button>
            <Button
              variant="ghost"
              size="sm"
              leftIcon={Plus}
              onPress={handleOpenCreateModal}
              testID="new-schedule-button"
            >
              New
            </Button>
          </View>
        }
      />

      {isInitialLoad ? (
        <View style={styles.loadingContainer}>
          <LoadingSpinner size="large" color={theme.colors.foregroundMuted} />
        </View>
      ) : null}

      {!isInitialLoad && schedules.length === 0 ? (
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>No schedules yet</Text>
          <Button variant="ghost" leftIcon={ChevronLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      ) : null}

      {!isInitialLoad && schedules.length > 0 ? (
        <ScrollView
          style={styles.list}
          contentContainerStyle={styles.listContent}
          refreshControl={
            <RefreshControl
              refreshing={isManualRefresh && isRevalidating}
              onRefresh={handleRefresh}
              tintColor={theme.colors.foregroundMuted}
            />
          }
        >
          {error ? (
            <View style={styles.errorBanner}>
              <Text style={styles.errorText}>{error}</Text>
            </View>
          ) : null}
          {schedules.map((schedule) => (
            <ScheduleCard
              key={schedule.id}
              serverId={serverId}
              schedule={schedule}
              onEdit={handleOpenEditModal}
              onPause={handlePause}
              onResume={handleResume}
              onDelete={handleDelete}
              isPausing={isPausing(schedule.id)}
              isResuming={isResuming(schedule.id)}
              isDeleting={isDeleting(schedule.id)}
            />
          ))}
        </ScrollView>
      ) : null}
      <ScheduleCreateModal
        visible={isCreateModalVisible}
        onClose={handleCloseCreateModal}
        serverId={serverId}
      />
      <ScheduleEditModal
        visible={editingSchedule !== null}
        onClose={handleCloseEditModal}
        serverId={serverId}
        schedule={editingSchedule}
        onOpenAssistant={handleOpenAssistantFromEdit}
      />
      <ScheduleAssistantPanel
        visible={isAssistantOpen}
        onClose={handleCloseAssistant}
        serverId={serverId}
        contextScheduleId={assistantContextScheduleId ?? undefined}
      />
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
  },
  headerRight: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  loadingContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
  emptyContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    gap: theme.spacing[6],
    padding: theme.spacing[6],
  },
  emptyText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.lg,
  },
  list: {
    flex: 1,
  },
  listContent: {
    padding: theme.spacing[4],
    gap: theme.spacing[3],
  },
  card: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[4],
    gap: theme.spacing[3],
  },
  cardHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: theme.spacing[2],
  },
  cardTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    flexShrink: 1,
  },
  cardTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.lg,
    fontWeight: "600",
    flexShrink: 1,
  },
  cardMetaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  cardMetaText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  cardActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[4],
    paddingTop: theme.spacing[2],
    borderTopWidth: theme.borderWidth[1],
    borderTopColor: theme.colors.border,
  },
  actionButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    paddingVertical: theme.spacing[2],
  },
  actionText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  statusBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    paddingVertical: theme.spacing[1],
    paddingHorizontal: theme.spacing[3],
    borderRadius: theme.borderRadius.full,
    borderWidth: theme.borderWidth[1],
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
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[4],
    marginBottom: theme.spacing[3],
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
}));
