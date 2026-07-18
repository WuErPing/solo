import { useMemo } from "react";
import { Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import type { ScheduleCadence, ScheduleTarget, StoredSchedule } from "@server/server/schedule/types";
import { Button } from "@/components/ui/button";
import { useScheduleInspect } from "@/hooks/use-schedule-inspect";
import { cronFromUTC, describeCron, detectTimezone } from "@/utils/cron-timezone";
import type { ScheduleAssistProposal } from "@/stores/schedule-assistant-store";

export interface ProposalCardProps {
  serverId: string;
  messageId: string;
  proposal: ScheduleAssistProposal;
  applying?: boolean;
  applyError?: string;
  cancelled?: boolean;
  onConfirm: (messageId: string, proposal: ScheduleAssistProposal) => void;
  onEditInForm: (proposal: ScheduleAssistProposal) => void;
  onCancel: (messageId: string) => void;
}

interface Row {
  label: string;
  before: string | null;
  after: string;
}

/** Proposal cadences are already local — render them directly. */
function describeCadenceLocal(cadence: ScheduleCadence): string {
  if (cadence.type === "cron") {
    return describeCron(cadence.expression);
  }
  const ms = cadence.everyMs;
  if (ms % 60000 === 0) {
    return `every ${ms / 60000} min`;
  }
  return `every ${Math.round(ms / 1000)}s`;
}

/** Stored cadences keep cron in UTC — convert back to local for display. */
function describeCadenceStored(cadence: ScheduleCadence): string {
  if (cadence.type === "cron") {
    const timezone = cadence.timezone || detectTimezone();
    const localExpression =
      timezone && timezone !== "UTC"
        ? cronFromUTC(cadence.expression, timezone)
        : cadence.expression;
    return describeCron(localExpression);
  }
  return describeCadenceLocal(cadence);
}

function describeTarget(target?: ScheduleTarget): string | null {
  if (!target) {
    return null;
  }
  switch (target.type) {
    case "agent":
      return `agent · ${target.agentId}`;
    case "new-agent":
      return `new-agent · ${target.config.provider}`;
    case "provider":
      return `provider · ${target.providerId}`;
    default:
      return "unknown target";
  }
}

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return date.toLocaleString(undefined, {
    weekday: "short",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function buildCreateRows(proposal: ScheduleAssistProposal): Row[] {
  const rows: Row[] = [];
  if (proposal.prompt) {
    rows.push({ label: "Prompt", before: null, after: proposal.prompt });
  }
  if (proposal.cadence) {
    rows.push({ label: "Cadence", before: null, after: describeCadenceLocal(proposal.cadence) });
  }
  const target = describeTarget(proposal.target);
  if (target) {
    rows.push({ label: "Target", before: null, after: target });
  }
  if (proposal.cwd) {
    rows.push({ label: "Cwd", before: null, after: proposal.cwd });
  }
  if (typeof proposal.maxRuns === "number") {
    rows.push({ label: "Max runs", before: null, after: String(proposal.maxRuns) });
  }
  if (proposal.expiresAt) {
    rows.push({ label: "Expires", before: null, after: formatTimestamp(proposal.expiresAt) });
  }
  return rows;
}

function buildUpdateRows(
  proposal: ScheduleAssistProposal,
  current: StoredSchedule | null,
): Row[] {
  const rows: Row[] = [];
  if (proposal.name !== undefined && proposal.name !== (current?.name ?? "")) {
    rows.push({ label: "Name", before: current?.name ?? null, after: proposal.name || "Untitled" });
  }
  if (proposal.prompt !== undefined && proposal.prompt !== current?.prompt) {
    rows.push({ label: "Prompt", before: current?.prompt ?? null, after: proposal.prompt });
  }
  if (proposal.cadence) {
    const before = current ? describeCadenceStored(current.cadence) : null;
    const after = describeCadenceLocal(proposal.cadence);
    if (before !== after) {
      rows.push({ label: "Cadence", before, after });
    }
  }
  if (proposal.target) {
    const before = current ? describeTarget(current.target) : null;
    const after = describeTarget(proposal.target);
    if (after && before !== after) {
      rows.push({ label: "Target", before, after });
    }
  }
  if (proposal.cwd !== undefined && proposal.cwd !== (current?.cwd ?? "")) {
    rows.push({ label: "Cwd", before: current?.cwd ?? null, after: proposal.cwd || "—" });
  }
  if (typeof proposal.maxRuns === "number" && proposal.maxRuns !== current?.maxRuns) {
    rows.push({
      label: "Max runs",
      before: current?.maxRuns != null ? String(current.maxRuns) : null,
      after: String(proposal.maxRuns),
    });
  }
  if (proposal.expiresAt !== undefined && proposal.expiresAt !== (current?.expiresAt ?? "")) {
    rows.push({
      label: "Expires",
      before: current?.expiresAt ? formatTimestamp(current.expiresAt) : null,
      after: proposal.expiresAt ? formatTimestamp(proposal.expiresAt) : "—",
    });
  }
  return rows;
}

function buildLifecycleRows(proposal: ScheduleAssistProposal): Row[] {
  const rows: Row[] = [];
  if (proposal.cadence) {
    rows.push({ label: "Cadence", before: null, after: describeCadenceLocal(proposal.cadence) });
  }
  return rows;
}

export function ProposalCard({
  serverId,
  messageId,
  proposal,
  applying = false,
  applyError,
  cancelled = false,
  onConfirm,
  onEditInForm,
  onCancel,
}: ProposalCardProps) {
  const { theme } = useUnistyles();
  const { schedule: current } = useScheduleInspect({
    serverId,
    scheduleId: proposal.scheduleId ?? null,
    enabled: proposal.op === "update" && Boolean(proposal.scheduleId),
  });

  const rows = useMemo(() => {
    if (proposal.op === "create") {
      return buildCreateRows(proposal);
    }
    if (proposal.op === "update") {
      return buildUpdateRows(proposal, current);
    }
    return buildLifecycleRows(proposal);
  }, [proposal, current]);

  const badgeColor = useMemo(() => {
    switch (proposal.op) {
      case "create":
      case "resume":
        return theme.colors.palette.green[400];
      case "update":
        return theme.colors.palette.blue[400];
      case "pause":
        return theme.colors.palette.amber[500];
      case "delete":
        return theme.colors.palette.red[500];
    }
  }, [proposal.op, theme]);

  const title = proposal.name ?? (proposal.scheduleId ? "Selected schedule" : "New schedule");
  const showSummary =
    proposal.op !== "create" && proposal.op !== "update" && proposal.summary.length > 0;
  const canEditInForm = proposal.op === "create" || proposal.op === "update";

  return (
    <View style={styles.card} testID="proposal-card">
      <View style={styles.headerRow}>
        <View style={[styles.opBadge, { borderColor: badgeColor }]}>
          <View style={[styles.opDot, { backgroundColor: badgeColor }]} />
          <Text style={[styles.opText, { color: badgeColor }]}>{proposal.op}</Text>
        </View>
        <Text style={styles.title} numberOfLines={2}>
          {title}
        </Text>
      </View>

      {showSummary ? <Text style={styles.summary}>{proposal.summary}</Text> : null}

      {rows.length > 0 ? (
        <View style={styles.fields}>
          {rows.map((row) => (
            <View key={row.label} style={styles.fieldRow}>
              <Text style={styles.fieldLabel}>{row.label}</Text>
              {row.before !== null ? (
                <Text style={styles.fieldValue}>
                  {row.before}
                  <Text style={styles.diffArrow}> → </Text>
                  {row.after}
                </Text>
              ) : (
                <Text style={styles.fieldValue}>{row.after}</Text>
              )}
            </View>
          ))}
        </View>
      ) : null}

      {proposal.nextRunAt ? (
        <Text style={styles.nextRun}>Next run: {formatTimestamp(proposal.nextRunAt)}</Text>
      ) : null}

      {proposal.warnings?.map((warning) => (
        <Text key={warning} style={styles.warning}>
          ⚠ {warning}
        </Text>
      ))}

      {applyError ? <Text style={styles.applyError}>{applyError}</Text> : null}

      {cancelled ? (
        <Text style={styles.cancelled}>Cancelled</Text>
      ) : (
        <View style={styles.actions}>
          <Button
            size="sm"
            variant={proposal.op === "delete" ? "destructive" : "default"}
            disabled={applying}
            onPress={() => onConfirm(messageId, proposal)}
            testID="proposal-confirm-button"
          >
            {applying ? "Applying..." : "Confirm"}
          </Button>
          {canEditInForm ? (
            <Button
              size="sm"
              variant="ghost"
              onPress={() => onEditInForm(proposal)}
              testID="proposal-edit-button"
            >
              Edit in form
            </Button>
          ) : null}
          <Button
            size="sm"
            variant="ghost"
            onPress={() => onCancel(messageId)}
            testID="proposal-cancel-button"
          >
            Cancel
          </Button>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  card: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[4],
    gap: theme.spacing[3],
  },
  headerRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  opBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[1],
    paddingVertical: theme.spacing[1],
    paddingHorizontal: theme.spacing[2],
    borderRadius: theme.borderRadius.full,
    borderWidth: theme.borderWidth[1],
  },
  opDot: {
    width: 6,
    height: 6,
    borderRadius: theme.borderRadius.full,
  },
  opText: {
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.medium,
    textTransform: "uppercase",
  },
  title: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.medium,
    flexShrink: 1,
  },
  summary: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  fields: {
    gap: theme.spacing[2],
    borderTopWidth: theme.borderWidth[1],
    borderTopColor: theme.colors.border,
    paddingTop: theme.spacing[3],
  },
  fieldRow: {
    flexDirection: "row",
    gap: theme.spacing[3],
  },
  fieldLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    width: 72,
    flexShrink: 0,
  },
  fieldValue: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    flex: 1,
  },
  diffArrow: {
    color: theme.colors.foregroundMuted,
  },
  nextRun: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  warning: {
    color: theme.colors.palette.amber[500],
    fontSize: theme.fontSize.sm,
  },
  applyError: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
  cancelled: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontStyle: "italic",
  },
  actions: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    borderTopWidth: theme.borderWidth[1],
    borderTopColor: theme.colors.border,
    paddingTop: theme.spacing[3],
  },
}));
