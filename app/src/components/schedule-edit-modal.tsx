import { useCallback, useEffect, useMemo, useState } from "react";
import { View, Text, Pressable, ScrollView } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { AdaptiveModalSheet, AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { Button } from "@/components/ui/button";
import { useAllAgentsList } from "@/hooks/use-all-agents-list";
import { useScheduleMutations } from "@/hooks/use-schedule-mutations";
import type { ScheduleSummary, ScheduleCadence, ScheduleTarget } from "@server/server/schedule/types";

interface ScheduleEditModalProps {
  visible: boolean;
  onClose: () => void;
  serverId: string;
  schedule: ScheduleSummary | null;
}

type CadenceType = "cron" | "every";

export function ScheduleEditModal({ visible, onClose, serverId, schedule }: ScheduleEditModalProps) {
  const { theme } = useUnistyles();
  const { agents, isInitialLoad } = useAllAgentsList({ serverId });
  const { updateSchedule, isUpdating } = useScheduleMutations({ serverId });

  const [name, setName] = useState("");
  const [prompt, setPrompt] = useState("");
  const [cadenceType, setCadenceType] = useState<CadenceType>("cron");
  const [cronExpression, setCronExpression] = useState("0 9 * * *");
  const [everyMs, setEveryMs] = useState("3600000");
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!schedule) return;
    setName(schedule.name ?? "");
    setPrompt(schedule.prompt);
    if (schedule.cadence.type === "cron") {
      setCadenceType("cron");
      setCronExpression(schedule.cadence.expression);
    } else {
      setCadenceType("every");
      setEveryMs(String(schedule.cadence.everyMs));
    }
    if (schedule.target.type === "agent") {
      setSelectedAgentId(schedule.target.agentId);
    } else {
      setSelectedAgentId(null);
    }
    setError(null);
  }, [schedule]);

  const agentOptions = useMemo(() => {
    return agents.map((agent) => ({
      id: agent.id,
      label: agent.title || "New session",
      description: agent.cwd,
    }));
  }, [agents]);

  const handleClose = useCallback(() => {
    setError(null);
    onClose();
  }, [onClose]);

  const handleSubmit = useCallback(async () => {
    if (!schedule) return;
    setError(null);
    if (!prompt.trim()) {
      setError("Prompt is required");
      return;
    }
    if (!selectedAgentId) {
      setError("Please select an agent");
      return;
    }

    let cadence: ScheduleCadence;
    if (cadenceType === "cron") {
      cadence = { type: "cron", expression: cronExpression.trim() };
    } else {
      const ms = parseInt(everyMs.trim(), 10);
      if (Number.isNaN(ms) || ms <= 0) {
        setError("Interval must be a positive number");
        return;
      }
      cadence = { type: "every", everyMs: ms };
    }

    const target: ScheduleTarget = { type: "agent", agentId: selectedAgentId };

    try {
      await updateSchedule({
        scheduleId: schedule.id,
        name: name.trim() || null,
        prompt: prompt.trim(),
        cadence,
        target,
      });
      handleClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update schedule");
    }
  }, [schedule, prompt, selectedAgentId, cadenceType, cronExpression, everyMs, name, updateSchedule, handleClose]);

  const updating = schedule ? isUpdating(schedule.id) : false;

  return (
    <AdaptiveModalSheet title="Edit Schedule" visible={visible} onClose={handleClose} scrollable={false}>
      <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
        {error ? (
          <View style={styles.errorBanner}>
            <Text style={styles.errorText}>{error}</Text>
          </View>
        ) : null}

        <View style={styles.field}>
          <Text style={styles.label}>Name</Text>
          <AdaptiveTextInput
            style={styles.input}
            placeholder="Optional name"
            value={name}
            onChangeText={setName}
            placeholderTextColor={theme.colors.foregroundMuted}
          />
        </View>

        <View style={styles.field}>
          <Text style={styles.label}>Prompt *</Text>
          <AdaptiveTextInput
            style={[styles.input, styles.textArea]}
            placeholder="What should the agent do?"
            value={prompt}
            onChangeText={setPrompt}
            multiline
            numberOfLines={3}
            placeholderTextColor={theme.colors.foregroundMuted}
          />
        </View>

        <View style={styles.field}>
          <Text style={styles.label}>Cadence *</Text>
          <View style={styles.segmentRow}>
            <Pressable
              style={[styles.segmentButton, cadenceType === "cron" && styles.segmentButtonActive]}
              onPress={() => setCadenceType("cron")}
            >
              <Text
                style={[styles.segmentText, cadenceType === "cron" && styles.segmentTextActive]}
              >
                Cron
              </Text>
            </Pressable>
            <Pressable
              style={[styles.segmentButton, cadenceType === "every" && styles.segmentButtonActive]}
              onPress={() => setCadenceType("every")}
            >
              <Text
                style={[styles.segmentText, cadenceType === "every" && styles.segmentTextActive]}
              >
                Interval
              </Text>
            </Pressable>
          </View>
          {cadenceType === "cron" ? (
            <>
              <AdaptiveTextInput
                style={[styles.input, styles.mt2]}
                placeholder="0 9 * * *"
                value={cronExpression}
                onChangeText={setCronExpression}
                placeholderTextColor={theme.colors.foregroundMuted}
              />
              <Text style={styles.helperText}>
                Standard cron expression (minute hour day month weekday). {"\""}0 9 * * *{"\""} = every day at 9:00 AM.
              </Text>
            </>
          ) : (
            <>
              <AdaptiveTextInput
                style={[styles.input, styles.mt2]}
                placeholder="3600000"
                value={everyMs}
                onChangeText={setEveryMs}
                keyboardType="numeric"
                placeholderTextColor={theme.colors.foregroundMuted}
              />
              <Text style={styles.helperText}>
                Interval in milliseconds. 3600000 = 1 hour, 60000 = 1 minute.
              </Text>
            </>
          )}
        </View>

        <View style={styles.field}>
          <Text style={styles.label}>Target Agent *</Text>
          {isInitialLoad ? (
            <Text style={styles.helperText}>Loading agents...</Text>
          ) : agentOptions.length === 0 ? (
            <Text style={styles.helperText}>No agents available</Text>
          ) : (
            <View style={styles.agentList}>
              {agentOptions.map((agent) => (
                <Pressable
                  key={agent.id}
                  style={[
                    styles.agentCard,
                    selectedAgentId === agent.id && styles.agentCardActive,
                  ]}
                  onPress={() => setSelectedAgentId(agent.id)}
                >
                  <Text
                    style={[
                      styles.agentName,
                      selectedAgentId === agent.id && styles.agentNameActive,
                    ]}
                    numberOfLines={1}
                  >
                    {agent.label}
                  </Text>
                  <Text style={styles.agentDesc} numberOfLines={1}>
                    {agent.description}
                  </Text>
                </Pressable>
              ))}
            </View>
          )}
        </View>

        <View style={styles.actions}>
          <Button variant="ghost" onPress={handleClose}>
            Cancel
          </Button>
          <Button onPress={handleSubmit} disabled={updating}>
            {updating ? "Saving..." : "Save Changes"}
          </Button>
        </View>
      </ScrollView>
    </AdaptiveModalSheet>
  );
}

const styles = StyleSheet.create((theme) => ({
  scroll: {
    flex: 1,
  },
  content: {
    gap: theme.spacing[4],
    paddingBottom: theme.spacing[6],
  },
  field: {
    gap: theme.spacing[2],
  },
  label: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
  },
  input: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[4],
    paddingVertical: theme.spacing[3],
    color: theme.colors.foreground,
    borderWidth: 1,
    borderColor: theme.colors.border,
    fontSize: theme.fontSize.base,
  },
  textArea: {
    minHeight: 80,
    textAlignVertical: "top",
  },
  segmentRow: {
    flexDirection: "row",
    gap: theme.spacing[2],
  },
  segmentButton: {
    flex: 1,
    paddingVertical: theme.spacing[3],
    paddingHorizontal: theme.spacing[4],
    borderRadius: theme.borderRadius.lg,
    backgroundColor: theme.colors.surface2,
    borderWidth: 1,
    borderColor: theme.colors.border,
    alignItems: "center",
  },
  segmentButtonActive: {
    backgroundColor: theme.colors.accent,
    borderColor: theme.colors.accent,
  },
  segmentText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
  },
  segmentTextActive: {
    color: theme.colors.accentForeground,
  },
  mt2: {
    marginTop: theme.spacing[2],
  },
  agentList: {
    gap: theme.spacing[2],
  },
  agentCard: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    padding: theme.spacing[3],
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  agentCardActive: {
    borderColor: theme.colors.accent,
    backgroundColor: theme.colors.surface3,
  },
  agentName: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.medium,
  },
  agentNameActive: {
    color: theme.colors.accent,
  },
  agentDesc: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    marginTop: theme.spacing[1],
  },
  helperText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  actions: {
    flexDirection: "row",
    justifyContent: "flex-end",
    gap: theme.spacing[3],
    marginTop: theme.spacing[2],
  },
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[4],
    borderWidth: 1,
    borderColor: theme.colors.palette.red[500],
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
}));
