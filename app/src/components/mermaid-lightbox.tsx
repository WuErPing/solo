// Fullscreen viewer for a single mermaid diagram, opened from MermaidPreview.
// Follows the AttachmentLightbox modal pattern; the diagram is rendered as
// vector SVG (fetched on demand via renderMermaidVector when the cache only
// holds the rasterized PNG) so pinch zoom stays sharp at any scale. Falls
// back to the PNG when vector data is unavailable.
import { useEffect, useMemo, useState } from "react";
import { ActivityIndicator, Image, Modal, Pressable, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { SvgXml } from "react-native-svg";
import { X } from "lucide-react-native";
import type { MermaidSvgEntry } from "./mermaid-svg-cache";
import { renderMermaidVector } from "./mermaid-svg-service";
import { MermaidZoomView } from "./mermaid-zoom-view";

interface MermaidLightboxProps {
  entry: MermaidSvgEntry;
  source: string;
  themeName: string;
  onClose: () => void;
}

export function MermaidLightbox({ entry, source, themeName, onClose }: MermaidLightboxProps) {
  const { theme } = useUnistyles();
  const insets = useSafeAreaInsets();
  const [xml, setXml] = useState(entry.xml);

  useEffect(() => {
    if (xml) {
      return;
    }
    let alive = true;
    void renderMermaidVector(source, themeName).then((result) => {
      if (alive && result?.xml) {
        setXml(result.xml);
      }
    });
    return () => {
      alive = false;
    };
  }, [xml, source, themeName]);

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

  const ready = Boolean(xml) || Boolean(entry.pngDataUrl);

  return (
    <Modal transparent animationType="fade" statusBarTranslucent visible onRequestClose={onClose}>
      <View style={styles.root}>
        <View style={styles.backdrop} />
        {ready ? (
          <MermaidZoomView contentWidth={entry.widthPx} contentHeight={entry.heightPx}>
            {xml ? (
              <SvgXml xml={xml} width={entry.widthPx} height={entry.heightPx} />
            ) : (
              <Image
                source={{ uri: entry.pngDataUrl }}
                style={{ width: entry.widthPx, height: entry.heightPx }}
                resizeMode="contain"
              />
            )}
          </MermaidZoomView>
        ) : (
          <View style={styles.loading}>
            <ActivityIndicator size="large" color={theme.colors.foregroundMuted} />
          </View>
        )}
        <Pressable
          testID="mermaid-lightbox-close"
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
