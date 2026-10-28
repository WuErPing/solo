import React, { memo, useEffect, useState } from "react";
import {
  ActivityIndicator,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from "react-native";
import { useUnistyles } from "react-native-unistyles";
import { SvgXml } from "react-native-svg";
import { Maximize2 } from "lucide-react-native";
import { Fonts } from "@/constants/theme";
import { useSessionStore } from "@/stores/session-store";
import { useHostRuntimeIsConnected } from "@/runtime/host-runtime";
import { fitScale } from "./math-svg";
import { DiagramLightbox } from "./diagram-lightbox";
import { getCachedPlantUmlSvg, type PlantUmlSvgEntry } from "./plantuml-svg-cache";
import { renderPlantUmlSvg } from "./plantuml-render";

interface PlantUmlPreviewProps {
  source: string;
  isDark?: boolean;
  serverId: string;
}

interface RenderState {
  entry: PlantUmlSvgEntry | null;
  error: string | null;
}

/**
 * Paints a PlantUML fence rendered on the daemon host (local CLI or a locally
 * configured PlantUML server), so diagram source never leaves the machine.
 * Cache hits paint synchronously with the surrounding markdown; misses show a
 * placeholder until the SVG arrives. Failures fall back to the source plus the
 * daemon's (actionable) error text.
 */
export const PlantUmlPreview = memo(function PlantUmlPreview({
  source,
  isDark,
  serverId,
}: PlantUmlPreviewProps) {
  const { theme } = useUnistyles();
  const dark = Boolean(isDark);
  const { width: windowWidth } = useWindowDimensions();
  // Matches the markdown scroll content padding (see file-pane
  // previewMarkdownScrollContent) minus the container border on each side.
  const contentWidth = Math.max(0, windowWidth - 2 * theme.spacing[4] - 2);

  const client = useSessionStore((state) => state.sessions[serverId]?.client ?? null);
  const isConnected = useHostRuntimeIsConnected(serverId);
  const [state, setState] = useState<RenderState | undefined>(() => {
    const cached = getCachedPlantUmlSvg(serverId, dark, source);
    return cached ? { entry: cached, error: null } : undefined;
  });
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    const cached = getCachedPlantUmlSvg(serverId, dark, source);
    if (cached) {
      setState({ entry: cached, error: null });
      return;
    }
    if (!client) {
      setState({ entry: null, error: "Host is not connected." });
      return;
    }
    let alive = true;
    void renderPlantUmlSvg(serverId, client, source, dark).then((result) => {
      if (!alive) {
        return;
      }
      setState(
        result.ok
          ? { entry: result.entry, error: null }
          : { entry: null, error: result.error },
      );
    });
    return () => {
      alive = false;
    };
    // Failures are not cached, so re-run when the host reconnects: a daemon that
    // lacked PlantUML support at first attempt may now serve the diagram.
  }, [client, dark, isConnected, serverId, source]);

  if (state === undefined) {
    return (
      <View style={[styles.container, styles.placeholder]}>
        <ActivityIndicator size="small" />
      </View>
    );
  }

  if (state.entry === null) {
    return (
      <View style={styles.container} testID="plantuml-preview-error">
        <ScrollView style={styles.errorScroll} nestedScrollEnabled>
          {state.error ? <Text style={styles.errorText}>{state.error}</Text> : null}
          <Text style={styles.sourceText}>{source}</Text>
        </ScrollView>
      </View>
    );
  }

  const scale = fitScale(state.entry.widthPx, contentWidth);
  const displayWidth = state.entry.widthPx * scale;
  const displayHeight = state.entry.heightPx * scale;
  return (
    <View style={styles.container} testID="plantuml-preview">
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Expand diagram"
        onPress={() => setExpanded(true)}
        style={styles.diagram}
      >
        <SvgXml xml={state.entry.xml} width={displayWidth} height={displayHeight} />
        <View
          style={[styles.expandBadge, { backgroundColor: theme.colors.surface2 }]}
          pointerEvents="none"
        >
          <Maximize2 size={12} color={theme.colors.foregroundMuted} />
        </View>
      </Pressable>
      {expanded ? (
        <DiagramLightbox
          xml={state.entry.xml}
          widthPx={state.entry.widthPx}
          heightPx={state.entry.heightPx}
          onClose={() => setExpanded(false)}
        />
      ) : null}
    </View>
  );
});

const styles = StyleSheet.create({
  container: {
    marginVertical: 8,
    borderRadius: 8,
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "rgba(128,128,128,0.2)",
  },
  placeholder: {
    height: 240,
    alignItems: "center",
    justifyContent: "center",
  },
  diagram: {
    alignItems: "center",
    paddingVertical: 16,
  },
  expandBadge: {
    position: "absolute",
    top: 6,
    right: 6,
    width: 24,
    height: 24,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
    opacity: 0.85,
  },
  errorScroll: {
    maxHeight: 200,
  },
  errorText: {
    fontFamily: Fonts.mono,
    fontSize: 12,
    color: "#c0392b",
    paddingHorizontal: 12,
    paddingTop: 12,
  },
  sourceText: {
    fontFamily: Fonts.mono,
    fontSize: 12,
    color: "#8b949e",
    padding: 12,
  },
});
