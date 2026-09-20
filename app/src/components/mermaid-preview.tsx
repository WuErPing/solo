import React, { useEffect, useMemo, useRef } from "react";
import { View, StyleSheet } from "react-native";
import { WebView } from "react-native-webview";
import { generateMermaidShellHtml } from "./mermaid-preview-utils";

interface MermaidPreviewProps {
  source: string;
  isDark?: boolean;
}

interface RenderRequest {
  source: string;
  theme: string;
}

function buildRenderScript(source: string, theme: string): string {
  return `window.__renderMermaid(${JSON.stringify(source)}, ${JSON.stringify(theme)}); true;`;
}

export function MermaidPreview({ source, isDark }: MermaidPreviewProps) {
  const webRef = useRef<WebView>(null);
  const loadedRef = useRef(false);
  const pendingRef = useRef<RenderRequest | null>(null);

  // Static shell: independent of source/theme so the WebView document (and its
  // CDN mermaid bundle) is loaded once and never reloaded on edits.
  const shell = useMemo(() => generateMermaidShellHtml(), []);
  const theme = isDark ? "dark" : "default";

  useEffect(() => {
    if (loadedRef.current) {
      webRef.current?.injectJavaScript(buildRenderScript(source, theme));
    } else {
      pendingRef.current = { source, theme };
    }
  }, [source, theme]);

  return (
    <View style={styles.container}>
      <WebView
        ref={webRef}
        originWhitelist={["*"]}
        source={{ html: shell }}
        style={styles.webview}
        scrollEnabled={false}
        javaScriptEnabled
        onLoadEnd={() => {
          loadedRef.current = true;
          const pending = pendingRef.current;
          if (pending) {
            pendingRef.current = null;
            webRef.current?.injectJavaScript(
              buildRenderScript(pending.source, pending.theme),
            );
          }
        }}
        testID="mermaid-preview-webview"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    height: 400,
    marginVertical: 8,
    borderRadius: 8,
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "rgba(128,128,128,0.2)",
  },
  webview: {
    flex: 1,
    backgroundColor: "transparent",
  },
});
