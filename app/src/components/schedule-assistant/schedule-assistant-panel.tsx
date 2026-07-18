import { useCallback, useMemo, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { router } from "expo-router";
import { StyleSheet } from "react-native-unistyles";
import type { ScheduleSummary } from "@server/server/schedule/types";
import { AdaptiveModalSheet } from "@/components/adaptive-modal-sheet";
import { Button } from "@/components/ui/button";
import {
  ScheduleCreateModal,
  type ScheduleCreateInitialValues,
} from "@/components/schedule-create-modal";
import { ScheduleEditModal } from "@/components/schedule-edit-modal";
import { useAssistantThread } from "@/hooks/use-assistant-thread";
import { useDaemonConfig } from "@/hooks/use-daemon-config";
import { useProposalConfirm } from "@/hooks/use-proposal-confirm";
import { useScheduleAssist } from "@/hooks/use-schedule-assist";
import { useHostRuntimeClient, useHosts } from "@/runtime/host-runtime";
import {
  useScheduleAssistantStore,
  type ScheduleAssistProposal,
} from "@/stores/schedule-assistant-store";
import { buildSettingsSectionRoute } from "@/utils/host-routes";
import { AssistantComposer } from "./assistant-composer";
import { AssistantMessageList } from "./assistant-message-list";

const SUGGESTIONS = [
  "Every morning at 9, run the daily standup summary",
  "Pause the disk cleanup",
  "What runs this week?",
];

export interface ScheduleAssistantHost {
  serverId: string;
  label: string;
}

export interface ScheduleAssistantPanelProps {
  visible: boolean;
  onClose: () => void;
  serverId: string;
  contextScheduleId?: string;
  /** Online hosts for the dashboard; renders a chip row when more than one. */
  availableHosts?: ScheduleAssistantHost[];
}

function SetupCard() {
  const openSettings = useCallback(() => {
    router.push(buildSettingsSectionRoute("general"));
  }, []);

  return (
    <View style={styles.setupCard}>
      <Text style={styles.setupTitle}>No LLM provider configured</Text>
      <Text style={styles.setupText}>
        Add an LLM provider with a default model in Settings → General → LLM Providers to use the
        assistant.
      </Text>
      <Button size="sm" onPress={openSettings} testID="assistant-setup-settings">
        Open Settings
      </Button>
    </View>
  );
}

export function ScheduleAssistantPanel({
  visible,
  onClose,
  serverId,
  contextScheduleId,
  availableHosts,
}: ScheduleAssistantPanelProps) {
  const [selectedHostId, setSelectedHostId] = useState<string | null>(null);
  const effectiveServerId = selectedHostId ?? serverId;

  const { messages, isSending, llmProvider, model } = useAssistantThread(effectiveServerId);
  const { send } = useScheduleAssist({ serverId: effectiveServerId, contextScheduleId });
  const { confirmProposal } = useProposalConfirm({ serverId: effectiveServerId });
  const updateMessage = useScheduleAssistantStore((state) => state.updateMessage);
  const { config } = useDaemonConfig(visible ? effectiveServerId : null);
  const client = useHostRuntimeClient(effectiveServerId);
  const hosts = useHosts();

  const [draft, setDraft] = useState("");
  const [createInitialValues, setCreateInitialValues] =
    useState<ScheduleCreateInitialValues | null>(null);
  const [editingSchedule, setEditingSchedule] = useState<ScheduleSummary | null>(null);

  const hostLabel = useMemo(() => {
    const fromAvailable = availableHosts?.find((host) => host.serverId === effectiveServerId);
    if (fromAvailable) {
      return fromAvailable.label;
    }
    return hosts.find((host) => host.serverId === effectiveServerId)?.label ?? effectiveServerId;
  }, [availableHosts, effectiveServerId, hosts]);

  // Setup card only when config is loaded and no enabled provider has a model;
  // while config is unknown (loading/offline) keep the regular empty state.
  const hasLlmProvider = useMemo(() => {
    if (!config) {
      return true;
    }
    return (config.llmProviders ?? []).some(
      (provider) => provider.enabled !== false && (provider.models?.length ?? 0) > 0,
    );
  }, [config]);

  const handleSend = useCallback(() => {
    const text = draft;
    setDraft("");
    void send(text);
  }, [draft, send]);

  const handleCancelProposal = useCallback(
    (messageId: string) => {
      updateMessage(effectiveServerId, messageId, { cancelled: true });
    },
    [effectiveServerId, updateMessage],
  );

  const handleEditInForm = useCallback(
    async (proposal: ScheduleAssistProposal) => {
      if (proposal.op === "create") {
        setCreateInitialValues({
          name: proposal.name ?? null,
          prompt: proposal.prompt,
          cadence: proposal.cadence,
          target: proposal.target,
          cwd: proposal.cwd ?? null,
          maxRuns: proposal.maxRuns,
          expiresAt: proposal.expiresAt,
        });
        return;
      }
      if (proposal.op === "update" && proposal.scheduleId && client) {
        try {
          const payload = await client.scheduleInspect({ id: proposal.scheduleId });
          if (payload.schedule) {
            setEditingSchedule(payload.schedule);
          }
        } catch {
          // Inspect failed — the user can retry from the card.
        }
      }
    },
    [client],
  );

  const subtitle = (
    <View style={styles.headerChips}>
      <Text style={styles.hostChip} numberOfLines={1}>
        {hostLabel}
      </Text>
      {llmProvider ? (
        <Text style={styles.llmChip} numberOfLines={1}>
          {model ? `${llmProvider} · ${model}` : llmProvider}
        </Text>
      ) : null}
    </View>
  );

  return (
    <>
      <AdaptiveModalSheet
        title="Schedule Assistant"
        subtitle={subtitle}
        visible={visible}
        onClose={onClose}
        scrollable={false}
        testID="schedule-assistant-panel"
      >
        <View style={styles.body}>
          {availableHosts && availableHosts.length > 1 ? (
            <View style={styles.hostChipRow}>
              {availableHosts.map((host) => {
                const isSelected = host.serverId === effectiveServerId;
                return (
                  <Pressable
                    key={host.serverId}
                    onPress={() => setSelectedHostId(host.serverId)}
                    style={[styles.hostSelectorChip, isSelected && styles.hostSelectorChipActive]}
                    testID="assistant-host-chip"
                  >
                    <Text
                      style={[styles.hostSelectorText, isSelected && styles.hostSelectorTextActive]}
                    >
                      {host.label}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
          ) : null}

          {messages.length === 0 ? (
            hasLlmProvider ? (
              <View style={styles.emptyState}>
                <Text style={styles.emptyHint}>Try an example:</Text>
                <View style={styles.suggestionList}>
                  {SUGGESTIONS.map((suggestion) => (
                    <Pressable
                      key={suggestion}
                      onPress={() => setDraft(suggestion)}
                      style={styles.suggestionChip}
                      testID="assistant-suggestion-chip"
                    >
                      <Text style={styles.suggestionText}>{suggestion}</Text>
                    </Pressable>
                  ))}
                </View>
              </View>
            ) : (
              <SetupCard />
            )
          ) : (
            <AssistantMessageList
              serverId={effectiveServerId}
              messages={messages}
              isSending={isSending}
              llmProvider={llmProvider}
              onConfirm={confirmProposal}
              onEditInForm={handleEditInForm}
              onCancel={handleCancelProposal}
            />
          )}

          <AssistantComposer
            value={draft}
            onChangeText={setDraft}
            onSend={handleSend}
            isSending={isSending}
            disabled={!hasLlmProvider}
          />
        </View>
      </AdaptiveModalSheet>

      <ScheduleCreateModal
        visible={createInitialValues !== null}
        onClose={() => setCreateInitialValues(null)}
        serverId={effectiveServerId}
        initialValues={createInitialValues ?? undefined}
      />
      <ScheduleEditModal
        visible={editingSchedule !== null}
        onClose={() => setEditingSchedule(null)}
        serverId={effectiveServerId}
        schedule={editingSchedule}
      />
    </>
  );
}

const styles = StyleSheet.create((theme) => ({
  body: {
    flex: 1,
    minHeight: 0,
    gap: theme.spacing[3],
  },
  headerChips: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  hostChip: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.full,
    paddingHorizontal: theme.spacing[2],
    paddingVertical: theme.spacing[1],
    maxWidth: 220,
  },
  llmChip: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    borderRadius: theme.borderRadius.full,
    paddingHorizontal: theme.spacing[2],
    paddingVertical: theme.spacing[1],
    maxWidth: 220,
  },
  hostChipRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: theme.spacing[2],
  },
  hostSelectorChip: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.full,
    paddingHorizontal: theme.spacing[3],
    paddingVertical: theme.spacing[2],
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
  },
  hostSelectorChipActive: {
    borderColor: theme.colors.accent,
    backgroundColor: theme.colors.surface3,
  },
  hostSelectorText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  hostSelectorTextActive: {
    color: theme.colors.foreground,
  },
  emptyState: {
    flex: 1,
    gap: theme.spacing[3],
    justifyContent: "center",
  },
  emptyHint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  suggestionList: {
    gap: theme.spacing[2],
  },
  suggestionChip: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    paddingHorizontal: theme.spacing[3],
    paddingVertical: theme.spacing[2],
  },
  suggestionText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  setupCard: {
    flex: 1,
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[4],
    gap: theme.spacing[3],
    alignItems: "flex-start",
    justifyContent: "center",
  },
  setupTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: theme.fontWeight.medium,
  },
  setupText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
}));
