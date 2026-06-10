import { useCallback, useMemo, useRef, useState } from "react";
import { Alert, Pressable, Text, TextInput, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Plus, X } from "lucide-react-native";
import { settingsStyles } from "@/styles/settings";
import { useHostRuntimeIsConnected } from "@/runtime/host-runtime";
import { useDaemonConfig } from "@/hooks/use-daemon-config";
import { Button } from "@/components/ui/button";
import { SettingsSection } from "@/screens/settings/settings-section";

const BUILT_IN_AGENT_NAMES = ["claude", "opencode", "qodercli", "pi", "cursor", "kimi", "kimi-cli", "codex"];

export interface TmuxAgentsSectionProps {
  serverId: string;
}

export function TmuxAgentsSection({ serverId }: TmuxAgentsSectionProps) {
  const { theme } = useUnistyles();
  const { config, patchConfig } = useDaemonConfig(serverId);
  const isConnected = useHostRuntimeIsConnected(serverId);
  const [newAgentName, setNewAgentName] = useState("");
  const [isAdding, setIsAdding] = useState(false);
  const inputRef = useRef<TextInput>(null);

  const customNames = useMemo(() => {
    if (!config?.tmuxAgentNames) return [];
    const builtInSet = new Set(BUILT_IN_AGENT_NAMES);
    return config.tmuxAgentNames.filter((n) => !builtInSet.has(n));
  }, [config?.tmuxAgentNames]);

  const handleAdd = useCallback(() => {
    const name = newAgentName.trim().toLowerCase();
    if (!name) return;
    if (BUILT_IN_AGENT_NAMES.includes(name)) {
      Alert.alert("Built-in agent", `"${name}" is already a built-in agent name.`);
      return;
    }
    if (customNames.includes(name)) {
      Alert.alert("Duplicate", `"${name}" is already in the list.`);
      return;
    }
    const next = [...customNames, name];
    void patchConfig({ tmuxAgentNames: next });
    setNewAgentName("");
    setIsAdding(false);
  }, [newAgentName, customNames, patchConfig]);

  const handleRemove = useCallback(
    (name: string) => {
      const next = customNames.filter((n) => n !== name);
      void patchConfig({ tmuxAgentNames: next });
    },
    [customNames, patchConfig],
  );

  const handleStartAdd = useCallback(() => {
    setIsAdding(true);
    setTimeout(() => inputRef.current?.focus(), 50);
  }, []);

  const handleCancelAdd = useCallback(() => {
    setIsAdding(false);
    setNewAgentName("");
  }, []);

  if (!isConnected) return null;

  return (
    <SettingsSection title="Tmux agents">
      <View style={settingsStyles.card} testID="settings-tmux-agents-card">
        <View style={settingsStyles.row}>
          <View style={settingsStyles.rowContent}>
            <Text style={settingsStyles.rowTitle}>Detected agents</Text>
            <Text style={settingsStyles.rowHint}>
              Agent names used to identify tmux panes. Built-in names are always active.
            </Text>
          </View>
        </View>

        {BUILT_IN_AGENT_NAMES.map((name) => (
          <View key={name} style={[settingsStyles.row, settingsStyles.rowBorder]}>
            <View style={styles.agentRowContent}>
              <Text style={styles.agentBuiltInName}>{name}</Text>
              <Text style={styles.agentBuiltInBadge}>built-in</Text>
            </View>
          </View>
        ))}

        {customNames.map((name) => (
          <View key={name} style={[settingsStyles.row, settingsStyles.rowBorder]}>
            <View style={styles.agentRowContent}>
              <Text style={styles.agentCustomName}>{name}</Text>
            </View>
            <Pressable
              onPress={() => handleRemove(name)}
              hitSlop={8}
              accessibilityLabel={`Remove ${name}`}
              testID={`tmux-agent-remove-${name}`}
            >
              <X size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />
            </Pressable>
          </View>
        ))}

        {isAdding ? (
          <View style={[settingsStyles.row, settingsStyles.rowBorder, styles.agentAddRow]}>
            <TextInput
              ref={inputRef}
              value={newAgentName}
              onChangeText={setNewAgentName}
              placeholder="agent-name"
              placeholderTextColor={theme.colors.foregroundMuted}
              autoCapitalize="none"
              autoCorrect={false}
              onSubmitEditing={handleAdd}
              style={styles.agentInput}
              testID="tmux-agent-input"
            />
            <Button size="sm" onPress={handleAdd} disabled={!newAgentName.trim()} testID="tmux-agent-add-confirm">
              Add
            </Button>
            <Pressable onPress={handleCancelAdd} hitSlop={8} testID="tmux-agent-add-cancel">
              <X size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />
            </Pressable>
          </View>
        ) : (
          <Pressable
            style={[settingsStyles.row, settingsStyles.rowBorder]}
            onPress={handleStartAdd}
            accessibilityRole="button"
            testID="tmux-agent-add-button"
          >
            <View style={styles.agentRowContent}>
              <Plus size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />
              <Text style={styles.agentAddText}>Add custom agent</Text>
            </View>
          </Pressable>
        )}
      </View>
    </SettingsSection>
  );
}

const styles = StyleSheet.create((theme) => ({
  agentRowContent: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    flex: 1,
  },
  agentBuiltInName: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
  agentBuiltInBadge: {
    fontSize: theme.fontSize.xs,
    color: theme.colors.foregroundMuted,
    opacity: 0.5,
  },
  agentCustomName: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foreground,
  },
  agentAddRow: {
    gap: theme.spacing[2],
  },
  agentInput: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
    color: theme.colors.foreground,
    paddingVertical: theme.spacing[2],
    paddingHorizontal: theme.spacing[3],
    borderRadius: theme.borderRadius.md,
    borderWidth: 1,
    borderColor: theme.colors.border,
    fontSize: theme.fontSize.sm,
  },
  agentAddText: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
  },
}));
