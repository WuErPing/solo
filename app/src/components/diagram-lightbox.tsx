// Fullscreen viewer shell shared by diagram previews (mermaid, PlantUML).
// Follows the AttachmentLightbox modal pattern; content is painted at its
// natural size inside MermaidZoomView so pinch/pan/double-tap zoom stays sharp.
import { useMemo } from "react";
import { ActivityIndicator, Image, Modal, Pressable, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { SvgXml } from "react-native-svg";
import { X } from "lucide-react-native";
import { MermaidZoomView } from "./mermaid-zoom-view";

interface DiagramLightboxProps {
  /** Vector markup to paint; preferred over `pngDataUrl`. */
  xml?: string;
  pngDataUrl?: string;
  /** Natural (unscaled) diagram size in CSS px. */
  widthPx: number;
  heightPx: number;
  onClose: () => void;
}

export function DiagramLightbox({
  xml,
  pngDataUrl,
  widthPx,
  heightPx,
  onClose,
}: DiagramLightboxProps) {
  const { theme } = useUnistyles();
  const insets = useSafeAreaInsets();

  const closeButtonStyle = useMemo(
    () => [
      styles.closeButton,
      {
        top: insets.top + theme.spacing[3],
        right: insets.right + theme.spacing[3],
      },
    ],
    [insets.top, insets.right, theme.spacing],
  );

  return (
    <Modal transparent animationType="fade" statusBarTranslucent visible onRequestClose={onClose}>
      <View style={styles.root}>
        <View style={styles.backdrop} />
        {xml || pngDataUrl ? (
          <MermaidZoomView contentWidth={widthPx} contentHeight={heightPx}>
            {/* Diagrams are rendered for the app theme with a transparent
                background, so the card reproduces the inline preview's surface;
                without it dark-on-transparent text vanishes into the backdrop. */}
            <View
              testID="diagram-lightbox-surface"
              style={[styles.content, { width: widthPx, height: heightPx, backgroundColor: theme.colors.background }]}
            >
              {xml ? (
                <SvgXml xml={xml} width={widthPx} height={heightPx} />
              ) : (
                <Image
                  testID="diagram-lightbox-image"
                  source={{ uri: pngDataUrl }}
                  style={{ width: widthPx, height: heightPx }}
                  resizeMode="contain"
                />
              )}
            </View>
          </MermaidZoomView>
        ) : (
          <View style={styles.loading}>
            <ActivityIndicator size="large" color={theme.colors.foregroundMuted} />
          </View>
        )}
        <Pressable
          testID="diagram-lightbox-close"
          accessibilityRole="button"
          accessibilityLabel="Close diagram"
          hitSlop={8}
          onPress={onClose}
          style={closeButtonStyle}
        >
          <X size={16} color={theme.colors.foregroundMuted} />
        </Pressable>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create((theme) => ({
  root: {
    flex: 1,
  },
  backdrop: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: "rgba(0,0,0,0.92)",
  },
  loading: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  content: {
    overflow: "hidden",
  },
  closeButton: {
    position: "absolute",
    width: 32,
    height: 32,
    borderRadius: 16,
    backgroundColor: theme.colors.surface2,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    alignItems: "center",
    justifyContent: "center",
    zIndex: 1,
  },
}));
