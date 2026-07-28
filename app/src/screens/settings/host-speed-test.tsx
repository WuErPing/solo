import { Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Globe, Monitor } from "lucide-react-native";
import type { SpeedTestResult, SpeedTestSegment } from "@server/client/daemon-client";
import type { HostConnection } from "@/types/host-connection";
import { getHostRuntimeStore } from "@/runtime/host-runtime";
import { connectToDaemon } from "@/utils/test-daemon-connection";
import { formatTimeAgo } from "@/utils/time";

export type SpeedTestState =
  | { status: "idle" }
  | { status: "running" }
  | { status: "success"; result: SpeedTestResult }
  | { status: "error"; message: string };

/**
 * Runs a speed test against one connection. Reuses the live client when the
 * connection is the active one; otherwise opens a temporary connection (the
 * same pattern as the latency probe in host-runtime) and closes it afterwards.
 */
export async function runConnectionSpeedTest(
  serverId: string,
  connection: HostConnection,
): Promise<SpeedTestResult> {
  const snapshot = getHostRuntimeStore().getSnapshot(serverId);
  const activeClient =
    snapshot?.connectionStatus === "online" &&
    snapshot.activeConnectionId === connection.id &&
    snapshot.client
      ? snapshot.client
      : null;
  if (activeClient) {
    return activeClient.speedTest();
  }

  const { client, serverId: connectedServerId } = await connectToDaemon(connection, { serverId });
  if (connectedServerId !== serverId) {
    await client.close().catch(() => undefined);
    throw new Error(`Connection resolved to ${connectedServerId}, expected ${serverId}.`);
  }
  try {
    return await client.speedTest();
  } finally {
    await client.close().catch(() => undefined);
  }
}

function speedTestSegmentColor(
  theme: ReturnType<typeof useUnistyles>["theme"],
  segmentId: SpeedTestSegment["id"],
): string {
  switch (segmentId) {
    case "appToRelay":
      return theme.colors.palette.blue[500];
    case "relayToDaemon":
      return theme.colors.palette.teal[200];
    case "daemonProcessing":
      return theme.colors.palette.amber[500];
    case "network":
      return theme.colors.palette.blue[500];
  }
}

/**
 * Per-segment latency breakdown shown under a connection row: one stacked bar
 * (segments sized by avg ms), a legend line per segment, the total-RTT summary
 * and a transport badge. Renders the running and error states as well; renders
 * nothing while idle.
 */
export function SpeedTestPanel({
  connectionId,
  state,
}: {
  connectionId: string;
  state: SpeedTestState;
}) {
  const { theme } = useUnistyles();

  if (state.status === "idle") {
    return null;
  }
  if (state.status === "running") {
    return (
      <View style={styles.panel} testID={`speed-test-panel-${connectionId}`}>
        <Text style={styles.hintText}>Testing...</Text>
      </View>
    );
  }
  if (state.status === "error") {
    return (
      <View style={styles.panel} testID={`speed-test-panel-${connectionId}`}>
        <Text style={styles.errorText}>{state.message}</Text>
      </View>
    );
  }

  const { result } = state;
  const totalAvgMs = result.segments.reduce((sum, segment) => sum + segment.stats.avgMs, 0);
  const transportBadge =
    result.transport === "relay"
      ? {
          icon: <Globe size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />,
          text: "Relay",
        }
      : {
          icon: <Monitor size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />,
          text: "Local",
        };

  return (
    <View style={styles.panel} testID={`speed-test-panel-${connectionId}`}>
      <View style={styles.bar} testID={`speed-test-bar-${connectionId}`}>
        {result.segments.map((segment) => (
          <View
            key={segment.id}
            style={{
              backgroundColor: speedTestSegmentColor(theme, segment.id),
              flexGrow: totalAvgMs > 0 ? Math.max(segment.stats.avgMs, 0) : 1,
            }}
          />
        ))}
      </View>
      {result.segments.map((segment) => (
        <View key={segment.id} style={styles.legendRow}>
          <View
            style={[styles.legendDot, { backgroundColor: speedTestSegmentColor(theme, segment.id) }]}
          />
          <Text style={styles.legendLabel} numberOfLines={1}>
            {segment.label}
          </Text>
          {segment.estimated ? <Text style={styles.estimatedTag}>~ estimated</Text> : null}
          <Text style={styles.legendValue}>
            {segment.stats.avgMs}ms avg · {segment.stats.minMs}–{segment.stats.maxMs}
          </Text>
        </View>
      ))}
      <Text style={styles.summary}>
        {`Avg ${result.totalRtt.avgMs}ms · min ${result.totalRtt.minMs} · max ${result.totalRtt.maxMs} · jitter ${result.totalRtt.jitterMs}ms · ${result.samples} samples`}
      </Text>
      <View style={styles.footer}>
        <View style={styles.badgePill}>
          {transportBadge.icon}
          <Text style={styles.badgeText} numberOfLines={1}>
            {transportBadge.text}
          </Text>
        </View>
        <Text style={styles.hintText}>measured {formatTimeAgo(new Date(result.measuredAt))}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  panel: {
    paddingHorizontal: theme.spacing[4],
    paddingBottom: theme.spacing[4],
    gap: theme.spacing[2],
  },
  bar: {
    flexDirection: "row",
    height: 8,
    borderRadius: theme.borderRadius.full,
    overflow: "hidden",
  },
  legendRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  legendDot: {
    width: 8,
    height: 8,
    borderRadius: theme.borderRadius.full,
  },
  legendLabel: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.xs,
  },
  estimatedTag: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  legendValue: {
    flex: 1,
    textAlign: "right",
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  summary: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.xs,
  },
  footer: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: theme.spacing[2],
  },
  badgePill: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: theme.spacing[2],
    paddingVertical: 4,
    borderRadius: theme.borderRadius.full,
    borderWidth: 1,
    borderColor: theme.colors.border,
    backgroundColor: theme.colors.surface3,
    maxWidth: 200,
  },
  badgeText: {
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.normal,
    color: theme.colors.foregroundMuted,
    flexShrink: 1,
  },
  hintText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  errorText: {
    color: theme.colors.palette.red[300],
    fontSize: theme.fontSize.xs,
  },
}));
