import { useCallback } from "react";
import { ScrollView, Text, View } from "react-native";
import { router } from "expo-router";
import { StyleSheet } from "react-native-unistyles";
import { Button } from "@/components/ui/button";
import type { AssistantMessage, ScheduleAssistProposal } from "@/stores/schedule-assistant-store";
import { buildHostScheduleDetailRoute, buildSettingsSectionRoute } from "@/utils/host-routes";
import { ProposalCard } from "./proposal-card";

export interface AssistantMessageListProps {
  serverId: string;
  messages: AssistantMessage[];
  isSending: boolean;
  llmProvider: string | null;
  onConfirm: (messageId: string, proposal: ScheduleAssistProposal) => void;
  onEditInForm: (proposal: ScheduleAssistProposal) => void;
  onCancel: (messageId: string) => void;
}

function ErrorBubble({ message }: { message: AssistantMessage }) {
  const openSettings = useCallback(() => {
    router.push(buildSettingsSectionRoute("general"));
  }, []);

  return (
    <View style={styles.errorBubble}>
      <Text style={styles.errorText}>{message.text}</Text>
      {message.error === "no_llm_provider" ? (
        <Button size="sm" variant="ghost" onPress={openSettings} testID="assistant-error-settings">
          Open Settings
        </Button>
      ) : null}
    </View>
  );
}

function ReceiptCard({ serverId, message }: { serverId: string; message: AssistantMessage }) {
  const openSchedule = useCallback(() => {
    if (message.receiptScheduleId) {
      router.push(buildHostScheduleDetailRoute(serverId, message.receiptScheduleId));
    }
  }, [serverId, message.receiptScheduleId]);

  return (
    <View style={styles.receiptCard}>
      <Text style={styles.receiptText}>{message.text}</Text>
      {message.receiptScheduleId ? (
        <Button size="sm" variant="ghost" onPress={openSchedule} testID="assistant-receipt-link">
          View schedule
        </Button>
      ) : null}
    </View>
  );
}

export function AssistantMessageList({
  serverId,
  messages,
  isSending,
  llmProvider,
  onConfirm,
  onEditInForm,
  onCancel,
}: AssistantMessageListProps) {
  return (
    <ScrollView
      style={styles.scroll}
      contentContainerStyle={styles.content}
      keyboardShouldPersistTaps="handled"
      showsVerticalScrollIndicator={false}
      testID="assistant-message-list"
    >
      {messages.map((message) => {
        if (message.role === "user") {
          return (
            <View key={message.id} style={styles.userRow}>
              <View style={styles.userBubble}>
                <Text style={styles.userText}>{message.text}</Text>
              </View>
            </View>
          );
        }
        if (message.kind === "proposal" && message.proposal) {
          return (
            <ProposalCard
              key={message.id}
              serverId={serverId}
              messageId={message.id}
              proposal={message.proposal}
              applying={message.applying}
              applyError={message.applyError}
              cancelled={message.cancelled}
              onConfirm={onConfirm}
              onEditInForm={onEditInForm}
              onCancel={onCancel}
            />
          );
        }
        if (message.kind === "error") {
          return <ErrorBubble key={message.id} message={message} />;
        }
        if (message.kind === "receipt") {
          return <ReceiptCard key={message.id} serverId={serverId} message={message} />;
        }
        return (
          <View key={message.id} style={styles.assistantRow}>
            <View style={styles.assistantBubble}>
              <Text style={styles.assistantText}>{message.text}</Text>
            </View>
          </View>
        );
      })}
      {isSending ? (
        <View style={styles.assistantRow}>
          <View style={styles.assistantBubble} testID="assistant-pending">
            <Text style={styles.pendingText}>
              Thinking…{llmProvider ? ` (${llmProvider})` : ""}
            </Text>
          </View>
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create((theme) => ({
  scroll: {
    flex: 1,
    minHeight: 120,
  },
  content: {
    gap: theme.spacing[3],
    paddingVertical: theme.spacing[2],
  },
  userRow: {
    flexDirection: "row",
    justifyContent: "flex-end",
  },
  userBubble: {
    backgroundColor: theme.colors.surface3,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[3],
    paddingVertical: theme.spacing[2],
    maxWidth: "85%",
  },
  userText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  assistantRow: {
    flexDirection: "row",
    justifyContent: "flex-start",
  },
  assistantBubble: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[3],
    paddingVertical: theme.spacing[2],
    maxWidth: "85%",
  },
  assistantText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  pendingText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontStyle: "italic",
  },
  errorBubble: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[3],
    gap: theme.spacing[2],
    alignItems: "flex-start",
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
  receiptCard: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[3],
    gap: theme.spacing[1],
    alignItems: "flex-start",
  },
  receiptText: {
    color: theme.colors.palette.green[400],
    fontSize: theme.fontSize.sm,
  },
}));
