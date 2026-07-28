import { useMemo } from "react";
import { View, Text } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { AlertTriangle, Gauge, Server } from "lucide-react-native";
import type { UsageQuota, UsageQuotaSnapshot } from "@server/client/usage-rpc";
import { formatTimeAgo } from "@/utils/time";

function capitalize(value: string): string {
  if (value.length === 0) {
    return value;
  }
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function formatQuotaValue(value: number | null): string {
  if (value === null) {
    return "—";
  }
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

function formatUpdatedAt(fetchedAt: string): string {
  const date = new Date(fetchedAt);
  if (Number.isNaN(date.getTime())) {
    return "unknown";
  }
  return formatTimeAgo(date);
}

function QuotaRow({ quota }: { quota: UsageQuota }) {
  const { theme } = useUnistyles();

  const barColor = useMemo(() => {
    const pct = quota.usedPct;
    if (pct === null) {
      return theme.colors.foregroundMuted;
    }
    if (pct >= 80) {
      return theme.colors.palette.red[500];
    }
    if (pct >= 50) {
      return theme.colors.palette.amber[500];
    }
    return theme.colors.palette.green[400];
  }, [quota.usedPct, theme]);

  const pct = quota.usedPct === null ? null : Math.min(100, Math.max(0, quota.usedPct));
  const usageText = `${formatQuotaValue(quota.used)}/${formatQuotaValue(quota.limit)}${
    quota.unit ? ` ${quota.unit}` : ""
  }`;

  return (
    <View style={styles.quotaRow}>
      <View style={styles.quotaHeader}>
        <Text style={styles.quotaLabel} numberOfLines={1}>
          {quota.label}
        </Text>
        <Text style={styles.quotaValue}>{usageText}</Text>
      </View>
      <View style={styles.progressTrack}>
        {pct !== null ? (
          <View
            style={[styles.progressFill, { width: `${pct}%`, backgroundColor: barColor }]}
          />
        ) : null}
      </View>
      {quota.resetIn ? (
        <Text style={styles.resetText}>Resets in {quota.resetIn}</Text>
      ) : null}
    </View>
  );
}

export function UsageProviderCard({
  snapshot,
  hostLabel,
}: {
  snapshot: UsageQuotaSnapshot;
  hostLabel?: string;
}) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.card}>
      <View style={styles.cardHeader}>
        <View style={styles.cardTitleRow}>
          <Gauge size={16} color={theme.colors.foregroundMuted} />
          <Text style={styles.cardTitle} numberOfLines={1}>
            {capitalize(snapshot.provider)}
          </Text>
        </View>
        {snapshot.plan ? (
          <View style={styles.planBadge}>
            <Text style={styles.planBadgeText} numberOfLines={1}>
              {snapshot.plan.tier
                ? `${snapshot.plan.name} · ${snapshot.plan.tier}`
                : snapshot.plan.name}
            </Text>
          </View>
        ) : null}
      </View>

      {hostLabel ? (
        <View style={styles.metaRow}>
          <Server size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText} numberOfLines={1}>
            {hostLabel}
          </Text>
        </View>
      ) : null}

      <View style={styles.quotas}>
        {snapshot.quotas.map((quota) => (
          <QuotaRow key={quota.name} quota={quota} />
        ))}
      </View>

      <Text style={styles.footerText}>Updated {formatUpdatedAt(snapshot.fetchedAt)}</Text>
    </View>
  );
}

export function UsageProviderErrorCard({
  provider,
  message,
  hostLabel,
}: {
  provider: string;
  message: string;
  hostLabel?: string;
}) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.card}>
      <View style={styles.cardHeader}>
        <View style={styles.cardTitleRow}>
          <Gauge size={16} color={theme.colors.foregroundMuted} />
          <Text style={styles.cardTitle} numberOfLines={1}>
            {capitalize(provider)}
          </Text>
        </View>
      </View>

      {hostLabel ? (
        <View style={styles.metaRow}>
          <Server size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText} numberOfLines={1}>
            {hostLabel}
          </Text>
        </View>
      ) : null}

      <View style={styles.warningRow}>
        <AlertTriangle size={14} color={theme.colors.palette.amber[500]} />
        <Text style={styles.warningText}>{message}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  card: {
    // flexGrow only (no flex: 1): when the grid falls back to a column
    // layout (Unistyles web), flex-shrink would squash tall card content.
    flexGrow: 1,
    minWidth: 280,
    maxWidth: "100%",
    padding: 16,
    borderRadius: theme.borderRadius.xl,
    backgroundColor: theme.colors.surface1,
    gap: 12,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  cardHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
  },
  cardTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    flexShrink: 1,
  },
  cardTitle: {
    flex: 1,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.semibold,
    color: theme.colors.foreground,
  },
  planBadge: {
    paddingVertical: 4,
    paddingHorizontal: 10,
    borderRadius: theme.borderRadius.full,
    borderWidth: 1,
    borderColor: theme.colors.border,
    backgroundColor: theme.colors.surface2,
    flexShrink: 0,
  },
  planBadgeText: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
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
  quotas: {
    gap: 12,
  },
  quotaRow: {
    gap: 6,
  },
  quotaHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
  },
  quotaLabel: {
    flex: 1,
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
    color: theme.colors.foreground,
  },
  quotaValue: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
  progressTrack: {
    height: 6,
    borderRadius: theme.borderRadius.full,
    backgroundColor: theme.colors.surface2,
    overflow: "hidden",
  },
  progressFill: {
    height: 6,
    borderRadius: theme.borderRadius.full,
  },
  resetText: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
  warningRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  warningText: {
    flex: 1,
    fontSize: theme.fontSize.sm,
    color: theme.colors.palette.amber[500],
  },
  footerText: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
}));
