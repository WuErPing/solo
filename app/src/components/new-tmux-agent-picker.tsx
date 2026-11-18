import { useCallback, useMemo, useState } from "react";
import { ActivityIndicator, Pressable, Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { SquareTerminal } from "lucide-react-native";
import { AdaptiveModalSheet } from "@/components/adaptive-modal-sheet";
import { useAggregatedTmuxAgents } from "@/hooks/use-tmux-agents";
import { useDaemonConfig } from "@/hooks/use-daemon-config";
import { useTmuxAgentLaunch } from "@/hooks/use-tmux-agent-launch";
import { mergeAgentPickerOptions, type AgentPickerOption } from "@/utils/tmux-agent-launch";

export interface NewTmuxAgentPickerProps {
  serverId: string;
  filePath: string;
  workspaceRoot: string;
  visible: boolean;
  onClose: () => void;
}

export function NewTmuxAgentPicker({
  serverId,
  filePath,
  workspaceRoot,
  visible,
  onClose,
}: NewTmuxAgentPickerProps) {
  const { theme } = useUnistyles();
  const { commandHistory, isLoading } = useAggregatedTmuxAgents({ enabled: visible });
  const { config } = useDaemonConfig(serverId);
  const { launch, isLaunching } = useTmuxAgentLaunch();
  const [error, setError] = useState<string | null>(null);

  const options = useMemo(() => {
    const history = commandHistory.filter((entry) => entry.serverId === serverId);
    return mergeAgentPickerOptions({
      history,
      customNames: config?.tmuxAgentNames ?? [],
    });
  }, [commandHistory, config?.tmuxAgentNames, serverId]);

  const handleSelect = useCallback(
    async (option: AgentPickerOption) => {
      setError(null);
      const result = await launch({
        serverId,
        agentName: option.agentName,
        launchCmd: option.launchCmd,
        filePath,
        workspaceRoot,
      });
      if (result.ok) {
        onClose();
      } else {
        setError(result.error);
      }
    },
    [launch, serverId, filePath, workspaceRoot, onClose],
  );

  return (
    <AdaptiveModalSheet
      title="New tmux agent"
      subtitle={
        <Text style={styles.subtitle} numberOfLines={1}>
          {filePath}
        </Text>
      }
      visible={visible}
      onClose={onClose}
      testID="new-tmux-agent-picker"
      desktopMaxWidth={480}
    >
      {error ? (
        <Text style={styles.errorText} testID="new-tmux-launch-error">
          {error}
        </Text>
      ) : null}
      {options.map((option) => (
        <Pressable
          key={option.key}
          testID={`new-tmux-option-${option.key}`}
          accessibilityRole="button"
          accessibilityLabel={`Launch ${option.agentName} in a new tmux session`}
          disabled={isLaunching}
          onPress={() => void handleSelect(option)}
          style={({ pressed }) => [
            styles.optionRow,
            pressed ? { opacity: 0.7 } : null,
            isLaunching ? { opacity: 0.5 } : null,
          ]}
        >
          <View style={styles.optionHeader}>
            <SquareTerminal size={14} color={theme.colors.foregroundMuted} />
            <Text style={styles.optionName}>{option.agentName}</Text>
            {option.source !== "history" ? (
              <Text style={styles.optionBadge}>{option.source}</Text>
            ) : null}
          </View>
          {option.source === "history" ? (
            <Text style={styles.optionCmd} numberOfLines={1}>
              {option.launchCmd}
            </Text>
          ) : null}
        </Pressable>
      ))}
      {isLoading ? (
        <View style={styles.loadingRow}>
          <ActivityIndicator size="small" testID="new-tmux-history-loading" />
        </View>
      ) : null}
    </AdaptiveModalSheet>
  );
}

const styles = StyleSheet.create((theme) => ({
  subtitle: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  errorText: {
    color: theme.colors.destructive,
    fontSize: theme.fontSize.sm,
    marginBottom: theme.spacing[2],
  },
  optionRow: {
    backgroundColor: theme.colors.surface0,
    borderRadius: theme.borderRadius.md,
    borderWidth: 1,
    borderColor: theme.colors.border,
    padding: theme.spacing[3],
    marginBottom: theme.spacing[2],
    gap: theme.spacing[1],
  },
  optionHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  optionName: {
    color: theme.colors.primary,
    fontSize: theme.fontSize.sm,
    fontWeight: "600",
  },
  optionBadge: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    opacity: 0.6,
  },
  optionCmd: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    fontFamily: "monospace",
  },
  loadingRow: {
    alignItems: "center",
    paddingVertical: theme.spacing[2],
  },
}));