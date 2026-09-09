import { useCallback, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ChevronDown, RefreshCw } from "lucide-react-native";
import type { SupervisorState } from "@server/generated/protocol-schemas";
import type { HostProfile } from "@/types/host-connection";
import {
  useHostRuntimeClient,
  useHostRuntimeIsConnected,
  useHostRuntimeSnapshot,
  type HostRuntimeSnapshot,
} from "@/runtime/host-runtime";
import { useSessionStore } from "@/stores/session-store";
import {
  saveHostVersionSnapshot,
  useHostVersionSnapshot,
  type HostVersionSnapshot,
} from "@/stores/host-version-snapshots-store";
import { classifyDaemonRequestError } from "@/utils/daemon-request-error";
import { formatConnectionStatus, getConnectionStatusTone } from "@/utils/daemons";
import { formatTimeAgo } from "@/utils/time";
import { settingsStyles } from "@/styles/settings";

type Tone = "success" | "warning" | "error" | "muted";

const SUPERVISOR_STATE_LABELS: Record<string, string> = {
  running: "Running",
  backoff: "Crash backoff",
  fallback: "Rolled back",
  exhausted: "Gave up",
  stopped: "Stopped",
};

function supervisorStateTone(state: string): Tone {
  switch (state) {
    case "running":
      return "success";
    case "backoff":
    case "fallback":
      return "warning";
    case "exhausted":
      return "error";
    default:
      return "muted";
  }
}

function formatSupervisorState(state: string): string {
  return SUPERVISOR_STATE_LABELS[state] ?? state;
}

function formatConnectionLabel(connection: HostProfile["connections"][number]): string {
  if (connection.type === "relay") {
    return "Relay";
  }
  if (connection.type === "directTcp") {
    return `TCP ${connection.endpoint}`;
  }
  return "Local";
}

function formatProbeLine(
  probe: { status: "pending"; latencyMs: null } | { status: "unavailable"; latencyMs: null } | { status: "available"; latencyMs: number } | undefined,
): string {
  if (probe?.status === "available") {
    return `${probe.latencyMs}ms`;
  }
  return probe?.status === "pending" ? "probing" : "unavailable";
}

function toneColor(
  theme: ReturnType<typeof useUnistyles>["theme"],
  tone: Tone,
): string {
  switch (tone) {
    case "success":
      return theme.colors.palette.green[500];
    case "warning":
      return theme.colors.palette.amber[500];
    case "error":
      return theme.colors.palette.red[500];
    default:
      return theme.colors.foregroundMuted;
  }
}

function ConnectionBlock({
  host,
  snapshot,
}: {
  host: HostProfile;
  snapshot: HostRuntimeSnapshot | null;
}) {
  const statusLabel = formatConnectionStatus(snapshot?.connectionStatus ?? "idle");
  return (
    <View style={styles.block}>
      <Text style={styles.blockTitle}>Connection</Text>
      <Text style={styles.line}>
        {`Status ${statusLabel} · ${
          snapshot?.lastOnlineAt
            ? `last online ${formatTimeAgo(new Date(snapshot.lastOnlineAt))}`
            : "never connected"
        }`}
      </Text>
      {snapshot?.lastError ? (
        <Text style={styles.errorLine}>Last error: {snapshot.lastError}</Text>
      ) : null}
      {host.connections.map((connection) => (
        <Text key={connection.id} style={styles.line}>
          {`${formatConnectionLabel(connection)}: ${formatProbeLine(
            snapshot?.probeByConnectionId.get(connection.id),
          )}`}
        </Text>
      ))}
    </View>
  );
}

function VersionBlock({
  serverId,
  isConnected,
  runningVersion,
  cachedSnapshot,
  refreshError,
  isRefreshing,
  onRefresh,
  canRefresh,
}: {
  serverId: string;
  isConnected: boolean;
  runningVersion: string | null;
  cachedSnapshot: HostVersionSnapshot | null;
  refreshError: string | null;
  isRefreshing: boolean;
  onRefresh: () => void;
  canRefresh: boolean;
}) {
  const { theme } = useUnistyles();
  return (
    <View style={styles.block}>
      <View style={styles.blockHeader}>
        <Text style={styles.blockTitle}>Daemon version</Text>
        <Pressable
          accessibilityRole="button"
          disabled={isRefreshing || !canRefresh}
          onPress={onRefresh}
          style={styles.refreshButton}
          testID="host-trace-refresh"
        >
          <RefreshCw size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />
          <Text style={styles.refreshText}>{isRefreshing ? "Refreshing" : "Refresh"}</Text>
        </Pressable>
      </View>
      {cachedSnapshot ? (
        <>
          <Text style={styles.line} testID={`host-trace-version-${serverId}`}>
            {`Running ${cachedSnapshot.runningVersion ?? runningVersion ?? "unknown"} · ${
              cachedSnapshot.versions.length
            } build(s) on host${
              cachedSnapshot.currentVersion ? ` · pointer ${cachedSnapshot.currentVersion}` : ""
            }`}
          </Text>
          <Text style={styles.mutedLine}>
            {`Seen ${formatTimeAgo(new Date(cachedSnapshot.fetchedAt))}${
              isConnected ? "" : " (host unreachable)"
            }`}
          </Text>
        </>
      ) : (
        <Text style={styles.mutedLine}>
          No version info seen yet on this device{isConnected ? "" : " (host unreachable)"}.
        </Text>
      )}
      {refreshError ? <Text style={styles.errorLine}>{refreshError}</Text> : null}
    </View>
  );
}

function SupervisorBlock({ supervisor }: { supervisor: SupervisorState | null }) {
  const { theme } = useUnistyles();
  if (!supervisor) {
    return null;
  }
  const updatedAt = supervisor.updatedAtMs
    ? formatTimeAgo(new Date(supervisor.updatedAtMs))
    : null;
  const details = [
    supervisor.pointerVersion ? `pointer ${supervisor.pointerVersion}` : null,
    supervisor.consecutiveCrashes > 0
      ? `${supervisor.consecutiveCrashes} consecutive crash(es)`
      : null,
    supervisor.backoffMs ? `backoff ${Math.round(supervisor.backoffMs / 100) / 10}s` : null,
    updatedAt ? `updated ${updatedAt}` : null,
  ]
    .filter(Boolean)
    .join(" · ");
  return (
    <View style={styles.block}>
      <Text style={styles.blockTitle}>Supervisor</Text>
      <Text
        style={[
          styles.line,
          { color: toneColor(theme, supervisorStateTone(supervisor.state)) },
        ]}
        testID="host-trace-supervisor-state"
      >
        {`${formatSupervisorState(supervisor.state)}${
          supervisor.spawnedBinary ? ` · ${supervisor.spawnedBinary}` : ""
        }`}
      </Text>
      {details ? <Text style={styles.mutedLine}>{details}</Text> : null}
      {supervisor.lastEvent ? <Text style={styles.mutedLine}>{supervisor.lastEvent}</Text> : null}
    </View>
  );
}

/**
 * Per-host diagnostics trail next to the latency probes: live connection
 * state, the last daemon-version snapshot (persisted, so it survives host
 * unreachability) and the supervisor's spawn-loop health relayed by the
 * daemon. Everything renders from snapshot/cache data — an unreachable host
 * still shows what was last known, which is the point.
 */
export function HostTraceCard({ host }: { host: HostProfile }) {
  const snapshot = useHostRuntimeSnapshot(host.serverId);
  const daemonClient = useHostRuntimeClient(host.serverId);
  const isConnected = useHostRuntimeIsConnected(host.serverId);
  const runningVersion = useSessionStore(
    (state) => state.sessions[host.serverId]?.serverInfo?.version ?? null,
  );
  const cachedSnapshot = useHostVersionSnapshot(host.serverId);
  const [isExpanded, setIsExpanded] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState<string | null>(null);

  const handleRefresh = useCallback(() => {
    if (!daemonClient || isRefreshing) return;
    setIsRefreshing(true);
    setRefreshError(null);
    void daemonClient.versions
      .listDaemonVersions(`settings_host_trace_${host.serverId}`)
      .then((payload) => {
        saveHostVersionSnapshot(host.serverId, {
          versions: payload.versions ?? [],
          currentVersion: payload.currentVersion ?? null,
          runningVersion: payload.runningVersion ?? null,
          supervisor: payload.supervisor ?? null,
        });
      })
      .catch((error) => {
        const classified = classifyDaemonRequestError(error);
        const reason =
          classified.kind === "refused"
            ? `daemon refused (${classified.code ?? "no code"})`
            : classified.kind === "unsupported"
              ? "daemon does not support version info"
              : "host unreachable";
        setRefreshError(`Version refresh failed: ${reason}.`);
      })
      .finally(() => {
        setIsRefreshing(false);
      });
  }, [daemonClient, isRefreshing, host.serverId]);

  const statusLabel = formatConnectionStatus(snapshot?.connectionStatus ?? "idle");
  const statusTone = getConnectionStatusTone(snapshot?.connectionStatus ?? "idle");
  const { theme } = useUnistyles();

  return (
    <View style={settingsStyles.card} testID="host-trace-card">
      <Pressable
        accessibilityRole="button"
        onPress={() => setIsExpanded((value) => !value)}
        style={styles.headerRow}
        testID="host-trace-toggle"
      >
        <Text style={styles.title}>Host trace</Text>
        <View style={styles.headerEnd}>
          <Text style={[styles.statusChip, { color: toneColor(theme, statusTone) }]}>
            {statusLabel}
          </Text>
          <ChevronDown
            size={theme.iconSize.sm}
            color={theme.colors.foregroundMuted}
            style={isExpanded ? styles.chevronUp : undefined}
          />
        </View>
      </Pressable>

      {isExpanded ? (
        <View style={styles.content} testID="host-trace-content">
          <ConnectionBlock host={host} snapshot={snapshot} />
          <VersionBlock
            serverId={host.serverId}
            isConnected={isConnected}
            runningVersion={runningVersion}
            cachedSnapshot={cachedSnapshot}
            refreshError={refreshError}
            isRefreshing={isRefreshing}
            onRefresh={handleRefresh}
            canRefresh={Boolean(daemonClient)}
          />
          <SupervisorBlock supervisor={cachedSnapshot?.supervisor ?? null} />
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  headerRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: theme.spacing[2],
  },
  title: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  headerEnd: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  statusChip: {
    fontSize: theme.fontSize.xs,
  },
  chevronUp: {
    transform: [{ rotate: "180deg" }],
  },
  content: {
    gap: theme.spacing[3],
    paddingTop: theme.spacing[2],
  },
  block: {
    gap: theme.spacing[1],
  },
  blockHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  blockTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.medium,
  },
  line: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.xs,
  },
  mutedLine: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  errorLine: {
    color: theme.colors.palette.red[300],
    fontSize: theme.fontSize.xs,
  },
  refreshButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: theme.spacing[2],
    paddingVertical: 4,
    borderRadius: theme.borderRadius.full,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  refreshText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
}));
