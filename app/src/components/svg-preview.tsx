import React, { useMemo } from "react";
import { View, Text, StyleSheet } from "react-native";
import { WebView } from "react-native-webview";
import { isSvgContent, generateSvgHtml } from "./svg-preview-utils";

interface SvgPreviewProps {
  source: string;
  isDark?: boolean;
}

export { isSvgContent, sanitizeSvgContent } from "./svg-preview-utils";

export function SvgPreview({ source, isDark = false }: SvgPreviewProps) {
  const isValid = useMemo(() => isSvgContent(source), [source]);
  const html = useMemo(() => (isValid ? generateSvgHtml(source, isDark) : ""), [source, isDark, isValid]);

  if (!isValid) {
    return (
      <View style={styles.container} testID="svg-preview">
        <Text style={styles.errorText} testID="svg-preview-error">
          Invalid SVG content
        </Text>
      </View>
    );
  }

  return (
    <View style={styles.container} testID="svg-preview">
      <WebView
        originWhitelist={["*"]}
        source={{ html }}
        style={styles.webview}
        scrollEnabled={true}
        scalesPageToFit={true}
        pinchGestureEnabled={true}
        showsHorizontalScrollIndicator={false}
        showsVerticalScrollIndicator={false}
        javaScriptEnabled={true}
        domStorageEnabled={true}
        androidLayerType="hardware"
        testID="svg-preview-webview"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    width: "100%",
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
  errorText: {
    padding: 16,
    color: "red",
    textAlign: "center",
  },
});
