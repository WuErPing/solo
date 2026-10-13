import React, { memo, useEffect, useState } from "react";
import {
  ActivityIndicator,
  Image,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from "react-native";
import { useUnistyles } from "react-native-unistyles";
import { SvgXml } from "react-native-svg";
import { Fonts } from "@/constants/theme";
import { fitScale } from "./math-svg";
import { getCachedMermaidSvg, type MermaidSvgEntry } from "./mermaid-svg-cache";
import { renderMermaidSvg } from "./mermaid-svg-service";
import { perfLog, perfNow } from "@/utils/perf-log";

interface MermaidPreviewProps {
  source: string;
  isDark?: boolean;
}

/**
 * Paints a mermaid diagram rendered to SVG by the shared service
 * (MermaidSvgServiceHost). Cache hits paint synchronously in the same pass as
 * the surrounding markdown; misses show a lightweight placeholder until the
 * SVG arrives — no per-diagram WebView, no CDN fetch, no fixed 400px box.
 */
export const MermaidPreview = memo(function MermaidPreview({
  source,
  isDark,
}: MermaidPreviewProps) {
  const { theme } = useUnistyles();
  const themeName = isDark ? "dark" : "default";
  const { width: windowWidth } = useWindowDimensions();
  // Matches the markdown scroll content padding (see file-pane
  // previewMarkdownScrollContent) minus the container border on each side.
  const contentWidth = Math.max(0, windowWidth - 2 * theme.spacing[4] - 2);

  const [entry, setEntry] = useState<MermaidSvgEntry | null | undefined>(() =>
    getCachedMermaidSvg(themeName, source),
  );

  useEffect(() => {
    const cached = getCachedMermaidSvg(themeName, source);
    if (cached !== undefined) {
      setEntry(cached);
      return;
    }
    let alive = true;
    const startedAt = perfNow();
    void renderMermaidSvg(source, themeName).then((result) => {
      if (!alive) {
        return;
      }
      perfLog("mermaid.preview.resolved", {
        ms: Math.round(perfNow() - startedAt),
        ok: result != null,
      });
      setEntry(result);
    });
    return () => {
      alive = false;
    };
  }, [source, themeName]);

  if (entry === undefined) {
    return (
      <View style={[styles.container, styles.placeholder]}>
        <ActivityIndicator size="small" />
      </View>
    );
  }

  if (entry === null) {
    return (
      <View style={styles.container}>
        <ScrollView style={styles.errorScroll} nestedScrollEnabled>
          <Text style={styles.errorText}>{source}</Text>
        </ScrollView>
      </View>
    );
  }

  const scale = fitScale(entry.widthPx, contentWidth);
  const displayWidth = entry.widthPx * scale;
  const displayHeight = entry.heightPx * scale;
  return (
    <View style={styles.container}>
      <View style={styles.diagram}>
        {entry.pngDataUrl ? (
          <Image
            source={{ uri: entry.pngDataUrl }}
            style={{ width: displayWidth, height: displayHeight }}
            resizeMode="contain"
          />
        ) : entry.xml ? (
          <SvgXml xml={entry.xml} width={displayWidth} height={displayHeight} />
        ) : null}
      </View>
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
  errorScroll: {
    maxHeight: 200,
  },
  errorText: {
    fontFamily: Fonts.mono,
    fontSize: 12,
    color: "#c0392b",
    padding: 12,
  },
});
