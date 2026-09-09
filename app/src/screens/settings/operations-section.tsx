import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert, Text, View, type PressableStateCallbackType } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ChevronDown, RotateCw } from "lucide-react-native";
import { settingsStyles } from "@/styles/settings";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  getHostRuntimeStore,
  isHostRuntimeConnected,
  useHostRuntimeClient,
  useHostRuntimeIsConnected,
} from "@/runtime/host-runtime";
import { useSessionStore } from "@/stores/session-store";
import { saveHostVersionSnapshot, useHostVersionSnapshot } from "@/stores/host-version-snapshots-store";
import { classifyDaemonRequestError } from "@/utils/daemon-request-error";
import { formatTimeAgo } from "@/utils/time";
import { confirmDialog } from "@/utils/confirm-dialog";
import { useDaemonConfig } from "@/hooks/use-daemon-config";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { SettingsSection } from "@/screens/settings/settings-section";

const RESTART_CONFIRMATION_MESSAGE =
  "This will restart the daemon. Agents running on it will keep going; the app will reconnect automatically.";

const delay = (ms: number) =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, ms);
  });

export interface OperationsSectionProps {
  serverId: string;
  hostLabel: string;
}

export function OperationsSection({ serverId, hostLabel }: OperationsSectionProps) {
  return (
    <SettingsSection title="Operations">
      <RestartDaemonCard serverId={serverId} hostLabel={hostLabel} />
      <DaemonVersionCard serverId={serverId} hostLabel={hostLabel} />
      <InjectSoloToolsCard serverId={serverId} />
    </SettingsSection>
  );
}

/**
 * Shared disconnect → reconnect wait used when the daemon is about to restart
 * (manual restart or version switch). Returns the current-connection predicate
 * plus a waiter that resolves once the host is back online (or times out).
 * Handles the "already reconnected / no disconnect happened" case gracefully:
 * it simply proceeds straight to the reconnect wait.
 */
function useDaemonReconnectWait(serverId: string, hostLabel: string) {
  const runtime = getHostRuntimeStore();
  const isMountedRef = useRef(true);

  useEffect(() => {
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  const isHostConnected = useCallback(
    () => isHostRuntimeConnected(runtime.getSnapshot(serverId)),
    [serverId, runtime],
  );

  const waitForCondition = useCallback(
    async (predicate: () => boolean, timeoutMs: number, intervalMs = 250) => {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        if (!isMountedRef.current) return false;
        if (predicate()) return true;
        await delay(intervalMs);
      }
      return predicate();
    },
    [],
  );

  const waitForDaemonRestart = useCallback(async () => {
    const disconnectTimeoutMs = 7000;
    const reconnectTimeoutMs = 30000;
    if (isHostConnected()) {
      await waitForCondition(() => !isHostConnected(), disconnectTimeoutMs);
    }
    const reconnected = await waitForCondition(() => isHostConnected(), reconnectTimeoutMs);
    if (isMountedRef.current && !reconnected) {
      Alert.alert(
        "Unable to reconnect",
        `${hostLabel} did not come back online. Please verify it restarted.`,
      );
    }
    return reconnected;
  }, [hostLabel, isHostConnected, waitForCondition]);

  return { isHostConnected, waitForDaemonRestart, isMountedRef };
}

function RestartDaemonCard({ serverId, hostLabel }: { serverId: string; hostLabel: string }) {
  const { theme } = useUnistyles();
  const daemonClient = useHostRuntimeClient(serverId);
  const isConnected = useHostRuntimeIsConnected(serverId);
  const { isHostConnected, waitForDaemonRestart, isMountedRef } = useDaemonReconnectWait(
    serverId,
    hostLabel,
  );
  const [isRestarting, setIsRestarting] = useState(false);

  const handleRestart = useCallback(() => {
    if (!daemonClient) {
      Alert.alert(
        "Host unavailable",
        "This host is not connected. Wait for it to come online before restarting.",
      );
      return;
    }
    if (!isHostConnected()) {
      Alert.alert(
        "Host offline",
        "This host is offline. Solo reconnects automatically—wait until it's back online before restarting.",
      );
      return;
    }

    void confirmDialog({
      title: `Restart ${hostLabel}`,
      message: RESTART_CONFIRMATION_MESSAGE,
      confirmLabel: "Restart",
      cancelLabel: "Cancel",
      destructive: true,
    })
      .then((confirmed) => {
        if (!confirmed) return;
        setIsRestarting(true);
        void daemonClient
          .restartServer(`settings_daemon_restart_${serverId}`)
          .catch((error) => {
            console.error(`[OperationsSection] Failed to restart daemon ${hostLabel}`, error);
            if (!isMountedRef.current) return;
            setIsRestarting(false);
            Alert.alert(
              "Error",
              "Failed to send the restart request. Solo reconnects automatically—try again once the host shows as online.",
            );
          });
        void waitForDaemonRestart().then(() => {
          if (isMountedRef.current) {
            setIsRestarting(false);
          }
        });
        return;
      })
      .catch((error) => {
        console.error(`[OperationsSection] Failed to open restart confirmation for ${hostLabel}`, error);
        Alert.alert("Error", "Unable to open the restart confirmation dialog.");
      });
  }, [daemonClient, hostLabel, serverId, isHostConnected, waitForDaemonRestart, isMountedRef]);

  const restartIcon = useMemo(
    () => <RotateCw size={theme.iconSize.sm} color={theme.colors.foreground} />,
    [theme.iconSize.sm, theme.colors.foreground],
  );

  return (
    <View style={settingsStyles.card} testID="settings-operations-restart-card">
      <View style={settingsStyles.row}>
        <View style={settingsStyles.rowContent}>
          <Text style={settingsStyles.rowTitle}>Restart daemon</Text>
          <Text style={settingsStyles.rowHint}>
            Restarts the daemon process. The app will reconnect automatically
          </Text>
        </View>
        <Button
          variant="outline"
          size="sm"
          leftIcon={restartIcon}
          onPress={handleRestart}
          disabled={isRestarting || !daemonClient || !isConnected}
          testID="settings-operations-restart-button"
        >
          {isRestarting ? "Restarting..." : "Restart"}
        </Button>
      </View>
    </View>
  );
}

function stripSoloPrefix(filename: string): string {
  return filename.startsWith("solo-") ? filename.slice("solo-".length) : filename;
}

// MAX_MENU_VERSIONS caps how many recent builds the version picker lists.
const MAX_MENU_VERSIONS = 8;

const styles = StyleSheet.create((theme) => ({
  versionTrigger: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: theme.spacing[2],
    paddingVertical: theme.spacing[2],
    paddingHorizontal: theme.spacing[3],
    borderWidth: 1,
    borderRadius: theme.borderRadius.md,
    borderColor: theme.colors.borderAccent,
    backgroundColor: "transparent",
  },
  versionTriggerText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
}));

function versionTriggerStyle({ pressed }: PressableStateCallbackType) {
  return [styles.versionTrigger, pressed && { opacity: 0.85 }];
}

type VersionFetchState =
  | { status: "ok" }
  | { status: "unreachable"; message: string }
  | { status: "refused"; code?: string; message: string };

function DaemonVersionCard({ serverId, hostLabel }: { serverId: string; hostLabel: string }) {
  const { theme } = useUnistyles();
  const daemonClient = useHostRuntimeClient(serverId);
  const isConnected = useHostRuntimeIsConnected(serverId);
  const { waitForDaemonRestart, isMountedRef } = useDaemonReconnectWait(serverId, hostLabel);
  const runningVersion = useSessionStore(
    (state) => state.sessions[serverId]?.serverInfo?.version ?? null,
  );
  const [versions, setVersions] = useState<{ version: string; mtimeMs: number }[]>([]);
  const [isSwitching, setIsSwitching] = useState(false);
  const [versionFetchState, setVersionFetchState] = useState<VersionFetchState>({ status: "ok" });
  const cachedSnapshot = useHostVersionSnapshot(serverId);
  // Older daemons don't know list_daemon_versions and fail the request (or
  // time out). Only that case hides the card; unreachable/refused hosts keep
  // it visible with the last-known snapshot instead of surfacing nothing.
  const [isSupported, setIsSupported] = useState(true);

  useEffect(() => {
    if (!daemonClient || !isConnected) return;
    let cancelled = false;
    void daemonClient.versions
      .listDaemonVersions(`settings_list_daemon_versions_${serverId}`)
      .then((payload) => {
        if (!cancelled) {
          setVersions(payload.versions ?? []);
          setVersionFetchState({ status: "ok" });
          saveHostVersionSnapshot(serverId, {
            versions: payload.versions ?? [],
            currentVersion: payload.currentVersion ?? null,
            runningVersion: payload.runningVersion ?? null,
            supervisor: payload.supervisor ?? null,
          });
        }
      })
      .catch((error) => {
        const classified = classifyDaemonRequestError(error);
        console.debug(
          `[OperationsSection] Daemon version info unavailable on ${hostLabel}`,
          classified,
          error,
        );
        if (!cancelled) {
          if (classified.kind === "unsupported") {
            setVersions([]);
            setIsSupported(false);
            return;
          }
          setVersionFetchState(
            classified.kind === "refused"
              ? { status: "refused", code: classified.code, message: classified.message }
              : { status: "unreachable", message: classified.message },
          );
        }
      });
    return () => {
      cancelled = true;
    };
  }, [daemonClient, isConnected, serverId, hostLabel]);

  const latestFilename = versions[0]?.version ?? null;
  const latestDisplay = latestFilename ? stripSoloPrefix(latestFilename) : null;

  const handleSelectVersion = useCallback(
    (filename: string, display: string) => {
      if (!daemonClient) return;

      void confirmDialog({
        title: `Switch daemon version on ${hostLabel}`,
        message: `This will restart the daemon onto ${display}. The app will reconnect automatically.`,
        confirmLabel: "Switch",
        cancelLabel: "Cancel",
      })
        .then((confirmed) => {
          if (!confirmed) return;
          setIsSwitching(true);
          void daemonClient.versions
            .switchDaemonVersion(filename)
            .then(() => {
              void waitForDaemonRestart().then(() => {
                if (isMountedRef.current) {
                  setIsSwitching(false);
                }
              });
            })
            .catch((error) => {
              console.error(
                `[OperationsSection] Failed to switch daemon version on ${hostLabel}`,
                error,
              );
              if (!isMountedRef.current) return;
              setIsSwitching(false);
              Alert.alert("Error", error instanceof Error ? error.message : String(error));
            });
          return;
        })
        .catch((error) => {
          console.error(
            `[OperationsSection] Failed to open version switch confirmation for ${hostLabel}`,
            error,
          );
          Alert.alert("Error", "Unable to open the version switch confirmation dialog.");
        });
    },
    [daemonClient, hostLabel, waitForDaemonRestart, isMountedRef],
  );

  const chevronIcon = useMemo(
    () => <ChevronDown size={theme.iconSize.sm} color={theme.colors.foreground} />,
    [theme.iconSize.sm, theme.colors.foreground],
  );

  const hintParts = [runningVersion ? `Running ${runningVersion}` : "Running version unknown"];
  if (latestDisplay) {
    hintParts.push(`Latest local build ${latestDisplay}`);
  }

  const detailLines: string[] = [];
  if (versionFetchState.status === "refused") {
    detailLines.push(
      `Version info unavailable${versionFetchState.code ? ` (${versionFetchState.code})` : ""}.`,
    );
  } else if (versionFetchState.status === "unreachable" || !isConnected) {
    detailLines.push("Host unreachable — showing last known version info.");
    if (cachedSnapshot) {
      const buildCount = cachedSnapshot.versions.length
        ? `, ${cachedSnapshot.versions.length} build(s) on host`
        : "";
      detailLines.push(
        `Last seen ${formatTimeAgo(new Date(cachedSnapshot.fetchedAt))}: running ${
          cachedSnapshot.runningVersion ?? "unknown"
        }${buildCount}.`,
      );
    } else {
      detailLines.push("No version info seen yet on this device.");
    }
  }

  if (!isSupported) return null;

  return (
    <View style={settingsStyles.card} testID="settings-operations-version-card">
      <View style={settingsStyles.row}>
        <View style={settingsStyles.rowContent}>
          <Text style={settingsStyles.rowTitle}>Daemon version</Text>
          <Text style={settingsStyles.rowHint}>{hintParts.join(" · ")}</Text>
          {detailLines.map((line, index) => (
            <Text
              key={index}
              style={settingsStyles.rowHint}
              testID={`settings-operations-version-detail-${index}`}
            >
              {line}
            </Text>
          ))}
        </View>
        {versions.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger
              disabled={isSwitching || !daemonClient || !isConnected}
              style={versionTriggerStyle}
              testID="settings-operations-version-switch-button"
            >
              <Text style={styles.versionTriggerText}>
                {isSwitching ? "Switching..." : "Switch version"}
              </Text>
              {chevronIcon}
            </DropdownMenuTrigger>
            <DropdownMenuContent side="bottom" align="end" width={280}>
              {versions.slice(0, MAX_MENU_VERSIONS).map((entry) => {
                const display = stripSoloPrefix(entry.version);
                const isRunning = runningVersion != null && display === runningVersion;
                const label =
                  display + (entry.version === latestFilename ? " (latest)" : "");
                return (
                  <DropdownMenuItem
                    key={entry.version}
                    testID={`settings-operations-version-item-${display}`}
                    selected={isRunning}
                    showSelectedCheck
                    disabled={isRunning || isSwitching}
                    onSelect={() => handleSelectVersion(entry.version, display)}
                  >
                    {label}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </View>
    </View>
  );
}

function InjectSoloToolsCard({ serverId }: { serverId: string }) {
  const isConnected = useHostRuntimeIsConnected(serverId);
  const { config, patchConfig } = useDaemonConfig(serverId);

  const handleValueChange = useCallback(
    (next: boolean) => {
      void patchConfig({
        mcp: {
          injectIntoAgents: next,
        },
      });
    },
    [patchConfig],
  );

  if (!isConnected) return null;

  return (
    <View style={settingsStyles.card} testID="settings-operations-inject-mcp-card">
      <View style={settingsStyles.row}>
        <View style={settingsStyles.rowContent}>
          <Text style={settingsStyles.rowTitle}>Inject Solo tools</Text>
          <Text style={settingsStyles.rowHint}>
            Automatically inject Solo MCP tools into new agents
          </Text>
        </View>
        <Switch
          value={config?.mcp?.injectIntoAgents !== false}
          onValueChange={handleValueChange}
          accessibilityLabel="Inject Solo tools"
        />
      </View>
    </View>
  );
}
