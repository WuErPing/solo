import { useCallback, useMemo } from "react";
import {
  View,
  Text,
  ScrollView,
} from "react-native";
import { useIsFocused } from "@react-navigation/native";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ArrowLeft, Calendar, Clock, CheckCircle, XCircle, Loader, Globe } from "lucide-react-native";
import { MenuHeader } from "@/components/headers/menu-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useScheduleInspect } from "@/hooks/use-schedule-inspect";
import { buildHostSchedulesRoute } from "@/utils/host-routes";
import { cronFromUTC, describeCron, detectTimezone } from "@/utils/cron-timezone";
import type { ScheduleCadence, ScheduleRun, ScheduleStatus } from "@server/server/schedule/types";

export function ScheduleDetailScreen({
  serverId,
  scheduleId,
}: {
  serverId: string;
  scheduleId: string;
}) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return <ScheduleDetailScreenContent serverId={serverId} scheduleId={scheduleId} />;
}

function formatCadence(cadence: ScheduleCadence): { text: string; raw?: string; timezone?: string } {
  if (cadence.type === "cron") {
    const tz = cadence.timezone || detectTimezone();
    if (tz && tz !== "UTC") {
      const localExpr = cronFromUTC(cadence.expression, tz);
      return { text: describeCron(localExpr), raw: cadence.expression, timezone: tz };
    }
    return { text: describeCron(cadence.expression), raw: cadence.expression };
  }
  const ms = cadence.everyMs;
  let text: string;
  if (ms < 60000) {
    text = `Every ${ms / 1000}s`;
  } else if (ms < 3600000) {
    text = `Every ${Math.floor(ms / 60000)}m`;
  } else if (ms < 86400000) {
    text = `Every ${Math.floor(ms / 3600000)}h`;
  } else {
    text = `Every ${Math.floor(ms / 86400000)}d`;
  }
  return { text };
}

function formatDuration(startedAt: string, endedAt: string | null): string {
  if (!endedAt) {
    return "Running...";
  }
  const ms = new Date(endedAt).getTime() - new Date(startedAt).getTime();
  if (ms < 1000) {
    return `${ms}ms`;
  }
  if (ms < 60000) {
    return `${(ms / 1000).toFixed(1)}s`;
  }
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
}

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  const tz = detectTimezone();
  if (tz && tz !== "UTC") {
    return date.toLocaleString("zh-CN", {
      timeZone: tz,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }
  return date.toLocaleString();
}

function RunStatusIcon({ status }: { status: ScheduleRun["status"] }) {
  const { theme } = useUnistyles();
  if (status === "succeeded") {
    return <CheckCircle size={16} color={theme.colors.palette.green[400]} />;
  }
  if (status === "failed") {
    return <XCircle size={16} color={theme.colors.palette.red[500]} />;
  }
  return <Loader size={16} color={theme.colors.palette.amber[500]} />;
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
      <Text style={[styles.statusText, { color }]}>{status}</Text>
    </View>
  );
}

function RunCard({ run }: { run: ScheduleRun }) {
  const { theme } = useUnistyles();
  const duration = formatDuration(run.startedAt, run.endedAt);

  return (
    <View style={styles.runCard}>
      <View style={styles.runHeader}>
        <RunStatusIcon status={run.status} />
        <Text style={styles.runStatus}>{run.status}</Text>
        <Text style={styles.runDuration}>{duration}</Text>
      </View>
      <View style={styles.runMetaRow}>
        <Clock size={12} color={theme.colors.foregroundMuted} />
        <Text style={styles.runMetaText}>{formatTimestamp(run.startedAt)}</Text>
      </View>
      {run.output ? (
        <View style={styles.runOutputContainer}>
          <Text style={styles.runOutput} numberOfLines={4}>
            {run.output}
          </Text>
        </View>
      ) : null}
      {run.error ? (
        <View style={styles.runErrorContainer}>
          <Text style={styles.runError} numberOfLines={4}>
            {run.error}
          </Text>
        </View>
      ) : null}
    </View>
  );
}

function ScheduleDetailScreenContent({
  serverId,
  scheduleId,
}: {
  serverId: string;
  scheduleId: string;
}) {
  const { theme } = useUnistyles();
  const { schedule, isLoading, error } = useScheduleInspect({ serverId, scheduleId });

  const handleBack = useCallback(() => {
    router.navigate(buildHostSchedulesRoute(serverId));
  }, [serverId]);

  const leftContent = useMemo(
    () => (
      <Button variant="ghost" size="sm" leftIcon={ArrowLeft} onPress={handleBack}>
        Back
      </Button>
    ),
    [handleBack],
  );

  if (isLoading) {
    return (
      <View style={styles.container}>
        <MenuHeader title="Schedule" leftContent={leftContent} />
        <View style={styles.loadingContainer}>
          <LoadingSpinner size="large" color={theme.colors.foregroundMuted} />
        </View>
      </View>
    );
  }

  if (error) {
    return (
      <View style={styles.container}>
        <MenuHeader title="Schedule" leftContent={leftContent} />
        <View style={styles.emptyContainer}>
          <Text style={styles.errorText}>{error}</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      </View>
    );
  }

  if (!schedule) {
    return (
      <View style={styles.container}>
        <MenuHeader title="Schedule" leftContent={leftContent} />
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>Schedule not found</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      </View>
    );
  }

  const name = schedule.name ?? "Untitled";
  const cadenceInfo = formatCadence(schedule.cadence);

  return (
    <View style={styles.container}>
      <MenuHeader title={name} leftContent={leftContent} />

      <ScrollView style={styles.scroll} contentContainerStyle={styles.scrollContent}>
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Calendar size={theme.iconSize.md} color={theme.colors.foregroundMuted} />
            <Text style={styles.sectionTitle}>Details</Text>
            <ScheduleStatusBadge status={schedule.status} />
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Prompt</Text>
            <Text style={styles.detailValue}>{schedule.prompt}</Text>
          </View>

          {schedule.cwd ? (
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Working Directory</Text>
              <Text style={[styles.detailValue, styles.monoValue]}>{schedule.cwd}</Text>
            </View>
          ) : null}

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Cadence</Text>
            <Text style={styles.detailValue}>{cadenceInfo.text}</Text>
            {cadenceInfo.timezone ? (
              <View style={styles.timezoneRow}>
                <Globe size={12} color={theme.colors.foregroundMuted} />
                <Text style={styles.timezoneText}>{cadenceInfo.timezone}</Text>
              </View>
            ) : null}
            {cadenceInfo.raw ? (
              <View style={styles.rawCronBox}>
                <Text style={styles.rawCronLabel}>原始表达式 (UTC)</Text>
                <Text style={styles.rawCronValue}>{cadenceInfo.raw}</Text>
              </View>
            ) : null}
          </View>

          {schedule.nextRunAt ? (
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Next run</Text>
              <Text style={styles.detailValue}>{formatTimestamp(schedule.nextRunAt)}</Text>
            </View>
          ) : null}

          {schedule.lastRunAt ? (
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Last run</Text>
              <Text style={styles.detailValue}>{formatTimestamp(schedule.lastRunAt)}</Text>
            </View>
          ) : null}
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Run History</Text>

          {schedule.runs.length === 0 ? (
            <Text style={styles.emptyText}>No runs yet</Text>
          ) : (
            <View style={styles.runsList}>
              {schedule.runs
                .slice()
                .reverse()
                .map((run) => (
                  <RunCard key={run.id} run={run} />
                ))}
            </View>
          )}
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
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
    fontSize: theme.fontSize.base,
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.base,
  },
  scroll: {
    flex: 1,
  },
  scrollContent: {
    padding: theme.spacing[4],
    gap: theme.spacing[4],
  },
  section: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[4],
    gap: theme.spacing[3],
  },
  sectionHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  sectionTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.lg,
    fontWeight: "600",
    flexShrink: 1,
  },
  detailRow: {
    gap: theme.spacing[1],
  },
  detailLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontWeight: "500",
  },
  detailValue: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
  },
  monoValue: {
    fontFamily: "monospace",
  },
  timezoneRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[1],
    marginTop: theme.spacing[1],
  },
  timezoneText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  rawCronBox: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    paddingHorizontal: theme.spacing[3],
    paddingVertical: theme.spacing[2],
    marginTop: theme.spacing[2],
    gap: theme.spacing[1],
  },
  rawCronLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    fontWeight: "500",
  },
  rawCronValue: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    fontFamily: "monospace",
  },
  statusBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    paddingVertical: theme.spacing[1],
    paddingHorizontal: theme.spacing[3],
    borderRadius: theme.borderRadius.full,
    borderWidth: theme.borderWidth[1],
    marginLeft: "auto" as unknown as number,
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
  runsList: {
    gap: theme.spacing[3],
  },
  runCard: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[3],
    gap: theme.spacing[2],
  },
  runHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  runStatus: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    fontWeight: "600",
    textTransform: "capitalize",
  },
  runDuration: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    marginLeft: "auto" as unknown as number,
  },
  runMetaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[1],
  },
  runMetaText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  runOutputContainer: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[2],
  },
  runOutput: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.xs,
    fontFamily: "monospace",
  },
  runErrorContainer: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[2],
    borderWidth: 1,
    borderColor: theme.colors.palette.red[500],
  },
  runError: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.xs,
    fontFamily: "monospace",
  },
}));
