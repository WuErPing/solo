import { useCallback, useEffect, useMemo, useState } from "react";
import { View, Text, Pressable, ScrollView } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { AdaptiveModalSheet, AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { Button } from "@/components/ui/button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useScheduleMutations } from "@/hooks/use-schedule-mutations";
import { useProvidersSnapshot } from "@/hooks/use-providers-snapshot";
import { detectTimezone, cronToUTC, cronFromUTC } from "@/utils/cron-timezone";
import { HelpCircle, Sparkles } from "lucide-react-native";
import type { ScheduleSummary, ScheduleCadence, ScheduleTarget } from "@server/server/schedule/types";

interface ScheduleEditModalProps {
  visible: boolean;
  onClose: () => void;
  serverId: string;
  schedule: ScheduleSummary | null;
  onOpenAssistant?: () => void;
}

type FrequencyPreset = "daily" | "hourly" | "weekly" | "custom";
type CadenceType = "cron" | "every";

function buildCronFromPreset(preset: FrequencyPreset, hour: number, minute: number): string {
  switch (preset) {
    case "daily":
      return `${minute} ${hour} * * *`;
    case "hourly":
      return `${minute} * * * *`;
    case "weekly":
      return `${minute} ${hour} * * 1`;
    case "custom":
      return "";
  }
}

/** Try to detect which preset matches a local cron expression. */
function detectPreset(expression: string): { preset: FrequencyPreset; hour: string; minute: string } {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 5) return { preset: "custom", hour: "9", minute: "0" };
  const [m, h, dom, mon, dow] = parts;

  // daily: M H * * *
  if (dom === "*" && mon === "*" && dow === "*") {
    // hourly: M * * * *
    if (h === "*") {
      return { preset: "hourly", hour: "0", minute: m };
    }
    return { preset: "daily", hour: h, minute: m };
  }

  // weekly: M H * * 1 (Monday)
  if (dom === "*" && mon === "*" && dow === "1") {
    return { preset: "weekly", hour: h, minute: m };
  }

  return { preset: "custom", hour: "9", minute: "0" };
}

export function ScheduleEditModal({ visible, onClose, serverId, schedule, onOpenAssistant }: ScheduleEditModalProps) {
  const { theme } = useUnistyles();
  const { entries: providers, isLoading: isProvidersLoading } = useProvidersSnapshot(serverId, {
    enabled: visible,
  });
  const { updateSchedule, isUpdating } = useScheduleMutations({ serverId });

  const [name, setName] = useState("");
  const [cwd, setCwd] = useState("");
  const [prompt, setPrompt] = useState("");
  const [cadenceType, setCadenceType] = useState<CadenceType>("cron");
  const [frequencyPreset, setFrequencyPreset] = useState<FrequencyPreset>("daily");
  const [hour, setHour] = useState("9");
  const [minute, setMinute] = useState("0");
  const [cronExpression, setCronExpression] = useState("0 9 * * *");
  const [everyMs, setEveryMs] = useState("3600000");
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(null);
  const [timezone, setTimezone] = useState(detectTimezone());
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!schedule) return;
    setName(schedule.name ?? "");
    setCwd(schedule.cwd ?? "");
    setPrompt(schedule.prompt);
    const tz = schedule.cadence.timezone || detectTimezone();
    setTimezone(tz);
    if (schedule.cadence.type === "cron") {
      setCadenceType("cron");
      // Convert stored UTC expression back to local for editing
      const localExpr = cronFromUTC(schedule.cadence.expression, tz);
      const detected = detectPreset(localExpr);
      setFrequencyPreset(detected.preset);
      setHour(detected.hour);
      setMinute(detected.minute);
      setCronExpression(localExpr);
    } else {
      setCadenceType("every");
      setEveryMs(String(schedule.cadence.everyMs));
    }
    if (schedule.target.type === "provider") {
      setSelectedProviderId(schedule.target.providerId);
    } else {
      setSelectedProviderId(null);
    }
    setError(null);
  }, [schedule]);

  const providerOptions = useMemo(() => {
    return (providers ?? []).map((provider) => ({
      id: provider.provider,
      label: provider.label || provider.provider,
      description: provider.description || "",
      status: provider.status,
      enabled: provider.enabled,
    }));
  }, [providers]);

  // Compute the local cron expression from preset + time
  const localCron = useMemo(() => {
    if (cadenceType === "every") return null;
    if (frequencyPreset === "custom") return cronExpression.trim();
    const h = parseInt(hour, 10);
    const m = parseInt(minute, 10);
    if (Number.isNaN(h) || Number.isNaN(m) || h < 0 || h > 23 || m < 0 || m > 59) return null;
    return buildCronFromPreset(frequencyPreset, h, m);
  }, [cadenceType, frequencyPreset, hour, minute, cronExpression]);

  // Compute UTC preview
  const utcCron = useMemo(() => {
    if (!localCron) return null;
    return cronToUTC(localCron, timezone);
  }, [localCron, timezone]);

  const handlePresetChange = useCallback((preset: FrequencyPreset) => {
    setFrequencyPreset(preset);
    if (preset !== "custom") {
      const h = parseInt(hour, 10) || 9;
      const m = parseInt(minute, 10) || 0;
      setCronExpression(buildCronFromPreset(preset, h, m));
    }
  }, [hour, minute]);

  const handleCadenceTypeChange = useCallback((type: CadenceType) => {
    setCadenceType(type);
  }, []);

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
    if (!selectedProviderId) {
      setError("Please select a provider");
      return;
    }

    let cadence: ScheduleCadence;
    if (cadenceType === "cron") {
      if (!localCron) {
        setError("Invalid schedule expression");
        return;
      }
      const utcExpression = cronToUTC(localCron, timezone);
      cadence = { type: "cron", expression: utcExpression, timezone };
    } else {
      const ms = parseInt(everyMs.trim(), 10);
      if (Number.isNaN(ms) || ms <= 0) {
        setError("Interval must be a positive number");
        return;
      }
      cadence = { type: "every", everyMs: ms };
    }

    const target: ScheduleTarget = { type: "provider", providerId: selectedProviderId };

    try {
      await updateSchedule({
        scheduleId: schedule.id,
        name: name.trim() || null,
        prompt: prompt.trim(),
        cadence,
        target,
        cwd: cwd.trim() || null,
      });
      handleClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update schedule");
    }
  }, [schedule, prompt, selectedProviderId, cadenceType, localCron, everyMs, name, cwd, timezone, updateSchedule, handleClose]);

  const updating = schedule ? isUpdating(schedule.id) : false;

  return (
    <AdaptiveModalSheet title="Edit Schedule" visible={visible} onClose={handleClose} scrollable={false} testID="schedule-edit-modal">
      <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
        {error ? (
          <View style={styles.errorBanner} testID="schedule-edit-error">
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
          <Text style={styles.label}>Working Directory</Text>
          <AdaptiveTextInput
            style={styles.input}
            placeholder="/path/to/project"
            value={cwd}
            onChangeText={setCwd}
            placeholderTextColor={theme.colors.foregroundMuted}
          />
          <Text style={styles.helperText}>Used to group this schedule with a project</Text>
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
          <SegmentedControl
            size="sm"
            options={[
              { value: "cron" as CadenceType, label: "Cron" },
              { value: "every" as CadenceType, label: "Interval" },
            ]}
            value={cadenceType}
            onValueChange={handleCadenceTypeChange}
          />
        </View>

        {cadenceType === "cron" ? (
          <>
            <View style={styles.field}>
              <Text style={styles.label}>频率</Text>
              <SegmentedControl
                size="sm"
                options={[
                  { value: "daily" as FrequencyPreset, label: "每天" },
                  { value: "hourly" as FrequencyPreset, label: "每小时" },
                  { value: "weekly" as FrequencyPreset, label: "每周" },
                  { value: "custom" as FrequencyPreset, label: "自定义" },
                ]}
                value={frequencyPreset}
                onValueChange={handlePresetChange}
              />
            </View>

            {frequencyPreset !== "hourly" ? (
              <View style={styles.field}>
                <Text style={styles.label}>时间</Text>
                <View style={styles.timeRow}>
                  <View style={styles.timeField}>
                    <AdaptiveTextInput
                      style={styles.timeInput}
                      placeholder="00"
                      value={hour}
                      onChangeText={setHour}
                      keyboardType="numeric"
                      maxLength={2}
                      placeholderTextColor={theme.colors.foregroundMuted}
                    />
                    <Text style={styles.timeFieldLabel}>时</Text>
                  </View>
                  <Text style={styles.timeSeparator}>:</Text>
                  <View style={styles.timeField}>
                    <AdaptiveTextInput
                      style={styles.timeInput}
                      placeholder="00"
                      value={minute}
                      onChangeText={setMinute}
                      keyboardType="numeric"
                      maxLength={2}
                      placeholderTextColor={theme.colors.foregroundMuted}
                    />
                    <Text style={styles.timeFieldLabel}>分</Text>
                  </View>
                  <Text style={styles.timeHint}>(本地时间)</Text>
                </View>
              </View>
            ) : null}

            {frequencyPreset === "custom" ? (
              <View style={styles.field}>
                <Text style={styles.label}>Cron Expression</Text>
                <AdaptiveTextInput
                  style={styles.input}
                  placeholder="0 9 * * *"
                  value={cronExpression}
                  onChangeText={setCronExpression}
                  placeholderTextColor={theme.colors.foregroundMuted}
                />
                <Text style={styles.helperText}>
                  Standard cron (minute hour day month weekday)
                </Text>
              </View>
            ) : null}

            <View style={styles.field}>
              <View style={styles.labelRow}>
                <Text style={styles.label}>时区</Text>
                <Tooltip enabledOnDesktop enabledOnMobile delayDuration={0}>
                  <TooltipTrigger>
                    <HelpCircle size={14} color={theme.colors.foregroundMuted} />
                  </TooltipTrigger>
                  <TooltipContent side="top" align="center">
                    <Text style={styles.tooltipText}>
                      Cron expressions are stored in UTC.{"\n"}
                      Your local time is automatically{"\n"}
                      converted to UTC for scheduling.
                    </Text>
                  </TooltipContent>
                </Tooltip>
              </View>
              <Text style={styles.timezoneValue}>{timezone}</Text>
            </View>

            {utcCron && localCron ? (
              <View style={styles.utcPreview}>
                <View style={styles.utcPreviewRow}>
                  <Text style={styles.utcLabel}>本地时间</Text>
                  <Text style={styles.utcValue}>{localCron}</Text>
                </View>
                <View style={styles.utcPreviewRow}>
                  <Text style={styles.utcLabel}>UTC 存储值</Text>
                  <Text style={styles.utcValue}>{utcCron}</Text>
                </View>
              </View>
            ) : null}
          </>
        ) : (
          <View style={styles.field}>
            <Text style={styles.label}>Interval (ms)</Text>
            <AdaptiveTextInput
              style={styles.input}
              placeholder="3600000"
              value={everyMs}
              onChangeText={setEveryMs}
              keyboardType="numeric"
              placeholderTextColor={theme.colors.foregroundMuted}
            />
            <Text style={styles.helperText}>
              3600000 = 1 hour, 60000 = 1 minute
            </Text>
          </View>
        )}

        <View style={styles.field}>
          <Text style={styles.label}>Target Agent *</Text>
          {isProvidersLoading ? (
            <Text style={styles.helperText}>Loading providers...</Text>
          ) : providerOptions.length === 0 ? (
            <Text style={styles.helperText}>No providers available</Text>
          ) : (
            <View style={styles.agentList}>
              {providerOptions.map((provider) => {
                const isSelected = selectedProviderId === provider.id;
                return (
                  <Pressable
                    key={provider.id}
                    testID="schedule-edit-provider-card"
                    data-selected={isSelected}
                    style={[styles.agentCard, isSelected && styles.agentCardActive]}
                    onPress={() => setSelectedProviderId(provider.id)}
                  >
                    <Text
                      style={[styles.agentName, isSelected && styles.agentNameActive]}
                      numberOfLines={1}
                    >
                      {provider.label}
                    </Text>
                    <Text style={styles.agentDesc} numberOfLines={1}>
                      {provider.status} · {provider.enabled ? "enabled" : "disabled"}
                      {provider.description ? ` · ${provider.description}` : ""}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
          )}
        </View>

        <View style={styles.actions}>
          {onOpenAssistant ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={Sparkles}
              onPress={onOpenAssistant}
              testID="schedule-edit-ask-ai"
            >
              Ask AI
            </Button>
          ) : null}
          <Button variant="ghost" onPress={handleClose}>
            Cancel
          </Button>
          <Button onPress={handleSubmit} disabled={updating} testID="schedule-edit-submit-button">
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
  labelRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[1],
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
  timeRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  timeInput: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[3],
    paddingVertical: theme.spacing[3],
    color: theme.colors.foreground,
    borderWidth: 1,
    borderColor: theme.colors.border,
    fontSize: theme.fontSize.base,
    width: 56,
    textAlign: "center",
  },
  timeField: {
    alignItems: "center",
    gap: theme.spacing[1],
  },
  timeFieldLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
  timeSeparator: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.medium,
  },
  timeHint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  timezoneValue: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[4],
    paddingVertical: theme.spacing[3],
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  tooltipText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.xs,
    lineHeight: 18,
  },
  utcPreview: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[4],
    paddingVertical: theme.spacing[3],
    borderWidth: 1,
    borderColor: theme.colors.border,
    gap: theme.spacing[2],
  },
  utcPreviewRow: {
    gap: theme.spacing[1],
  },
  utcLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.medium,
  },
  utcValue: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontFamily: "monospace",
  },
  helperText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
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
