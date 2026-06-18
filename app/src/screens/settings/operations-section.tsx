import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert, Text, View } from "react-native";
import { useUnistyles } from "react-native-unistyles";
import { RotateCw } from "lucide-react-native";
import { settingsStyles } from "@/styles/settings";
import {
  getHostRuntimeStore,
  isHostRuntimeConnected,
  useHostRuntimeClient,
  useHostRuntimeIsConnected,
} from "@/runtime/host-runtime";
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
      <InjectSoloToolsCard serverId={serverId} />
    </SettingsSection>
  );
}

function RestartDaemonCard({ serverId, hostLabel }: { serverId: string; hostLabel: string }) {
  const { theme } = useUnistyles();
  const daemonClient = useHostRuntimeClient(serverId);
  const isConnected = useHostRuntimeIsConnected(serverId);
  const runtime = getHostRuntimeStore();
  const [isRestarting, setIsRestarting] = useState(false);
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
    if (isMountedRef.current) {
      setIsRestarting(false);
      if (!reconnected) {
        Alert.alert(
          "Unable to reconnect",
          `${hostLabel} did not come back online. Please verify it restarted.`,
        );
      }
    }
  }, [hostLabel, isHostConnected, waitForCondition]);

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
        void waitForDaemonRestart();
        return;
      })
      .catch((error) => {
        console.error(`[OperationsSection] Failed to open restart confirmation for ${hostLabel}`, error);
        Alert.alert("Error", "Unable to open the restart confirmation dialog.");
      });
  }, [daemonClient, hostLabel, serverId, isHostConnected, waitForDaemonRestart]);

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
