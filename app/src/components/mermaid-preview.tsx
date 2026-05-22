import React from "react";
import { View, StyleSheet } from "react-native";
import { WebView } from "react-native-webview";
import { generateMermaidHtml } from "./mermaid-preview-utils";

interface MermaidPreviewProps {
  source: string;
  isDark?: boolean;
}

export function MermaidPreview({ source, isDark }: MermaidPreviewProps) {
  const html = React.useMemo(() => generateMermaidHtml(source, isDark), [source, isDark]);

  return (
    <View style={styles.container}>
      <WebView
        originWhitelist={["*"]}
        source={{ html }}
        style={styles.webview}
        scrollEnabled={false}
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
