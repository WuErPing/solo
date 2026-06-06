import React, { useMemo } from "react";
import { View, Text, StyleSheet } from "react-native";
import { WebView } from "react-native-webview";

interface SvgPreviewProps {
  source: string;
  isDark?: boolean;
}

export function isSvgContent(source: string): boolean {
  if (!source) return false;
  const trimmed = source.trim();
  return trimmed.startsWith("<svg") && trimmed.includes('xmlns="http://www.w3.org/2000/svg"');
}

export function sanitizeSvgContent(source: string): string {
  let sanitized = source;
  sanitized = sanitized.replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, "");
  sanitized = sanitized.replace(/\bon\w+\s*=\s*["'][^"']*["']/gi, "");
  sanitized = sanitized.replace(/\bon\w+\s*=\s*[^\s>]*/gi, "");
  sanitized = sanitized.replace(/xlink:href\s*=\s*["']javascript:[^"']*["']/gi, "");
  sanitized = sanitized.replace(/href\s*=\s*["']javascript:[^"']*["']/gi, "");
  return sanitized;
}

function generateSvgHtml(source: string, isDark: boolean): string {
  const background = isDark ? "#0d1117" : "#ffffff";
  const sanitized = sanitizeSvgContent(source);

  return `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body {
  margin: 0;
  padding: 16px;
  display: flex;
  justify-content: center;
  align-items: center;
  background: ${background};
  min-height: 100vh;
  box-sizing: border-box;
}
svg {
  max-width: 100%;
  max-height: 100%;
  height: auto;
}
</style>
</head>
<body>
${sanitized}
</body>
</html>`;
}

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
        scrollEnabled={false}
        testID="svg-preview-webview"
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
  errorText: {
    padding: 16,
    color: "red",
    textAlign: "center",
  },
});
