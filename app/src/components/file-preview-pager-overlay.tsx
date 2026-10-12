import { Pressable, View } from "react-native";
import { ChevronDown, ChevronUp } from "lucide-react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";

interface FilePreviewPagerOverlayProps {
  canPageUp: boolean;
  canPageDown: boolean;
  onPageUp: () => void;
  onPageDown: () => void;
}

export function FilePreviewPagerOverlay({
  canPageUp,
  canPageDown,
  onPageUp,
  onPageDown,
}: FilePreviewPagerOverlayProps) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.container} pointerEvents="box-none" testID="file-preview-pager">
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Page up"
        accessibilityState={{ disabled: !canPageUp }}
        disabled={!canPageUp}
        hitSlop={8}
        onPress={onPageUp}
        testID="file-preview-pager-up"
        style={({ pressed }) => [
          styles.button,
          !canPageUp && styles.buttonDisabled,
          pressed && styles.buttonPressed,
        ]}
      >
        <ChevronUp size={theme.iconSize.lg} color={theme.colors.foregroundMuted} />
      </Pressable>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Page down"
        accessibilityState={{ disabled: !canPageDown }}
        disabled={!canPageDown}
        hitSlop={8}
        onPress={onPageDown}
        testID="file-preview-pager-down"
        style={({ pressed }) => [
          styles.button,
          !canPageDown && styles.buttonDisabled,
          pressed && styles.buttonPressed,
        ]}
      >
        <ChevronDown size={theme.iconSize.lg} color={theme.colors.foregroundMuted} />
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    position: "absolute",
    right: theme.spacing[3],
    bottom: theme.spacing[4],
    flexDirection: "column",
    gap: theme.spacing[2],
  },
  button: {
    width: 40,
    height: 40,
    borderRadius: 20,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: theme.colors.surface2,
    opacity: 0.7,
  },
  buttonDisabled: {
    opacity: 0.25,
  },
  buttonPressed: {
    opacity: 0.95,
  },
}));
