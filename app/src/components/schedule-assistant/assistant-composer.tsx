import { useCallback } from "react";
import { View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Send } from "lucide-react-native";
import { AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { Button } from "@/components/ui/button";

export interface AssistantComposerProps {
  value: string;
  onChangeText: (text: string) => void;
  onSend: () => void;
  isSending: boolean;
  disabled?: boolean;
}

export function AssistantComposer({
  value,
  onChangeText,
  onSend,
  isSending,
  disabled = false,
}: AssistantComposerProps) {
  const { theme } = useUnistyles();
  const canSend = !disabled && !isSending && value.trim().length > 0;

  const handleSend = useCallback(() => {
    if (canSend) {
      onSend();
    }
  }, [canSend, onSend]);

  return (
    <View style={styles.row}>
      <AdaptiveTextInput
        style={styles.input}
        placeholder="Ask to create or change a schedule…"
        placeholderTextColor={theme.colors.foregroundMuted}
        value={value}
        onChangeText={onChangeText}
        multiline
        editable={!disabled}
        testID="assistant-composer-input"
      />
      <Button
        size="sm"
        variant="default"
        leftIcon={Send}
        disabled={!canSend}
        onPress={handleSend}
        testID="assistant-composer-send"
      >
        Send
      </Button>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  row: {
    flexDirection: "row",
    alignItems: "flex-end",
    gap: theme.spacing[2],
    borderTopWidth: theme.borderWidth[1],
    borderTopColor: theme.colors.border,
    paddingTop: theme.spacing[3],
  },
  input: {
    flex: 1,
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.lg,
    paddingHorizontal: theme.spacing[4],
    paddingVertical: theme.spacing[3],
    color: theme.colors.foreground,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    fontSize: theme.fontSize.base,
    maxHeight: 120,
  },
}));
