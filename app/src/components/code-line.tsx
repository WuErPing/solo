import React, { useMemo } from "react";
import { Text, View } from "react-native";
import { StyleSheet } from "react-native-unistyles";
import { Fonts } from "@/constants/theme";
import type { HighlightStyle, HighlightToken } from "@getsolo/highlight";

export interface CodeLineProps {
  tokens: HighlightToken[];
  lineNumber: number;
  gutterWidth: number;
  colorMap: Record<HighlightStyle, string>;
  baseColor: string;
}

export const CodeLine = React.memo(function CodeLine({
  tokens,
  lineNumber,
  gutterWidth,
  colorMap,
  baseColor,
}: CodeLineProps) {
  const gutterStyle = useMemo(() => [codeLineStyles.gutter, { width: gutterWidth }], [gutterWidth]);
  const gutterTextStyle = useMemo(
    () => [codeLineStyles.gutterText, { color: baseColor }],
    [baseColor],
  );
  const keyedTokens = useMemo(
    () => tokens.map((token, index) => ({ key: `${index}-${token.text}`, token })),
    [tokens],
  );
  return (
    <View style={codeLineStyles.line}>
      <View style={gutterStyle}>
        <Text style={gutterTextStyle}>{String(lineNumber)}</Text>
      </View>
      <Text selectable style={codeLineStyles.lineText}>
        {keyedTokens.map(({ key, token }) => (
          <CodeLineToken
            key={key}
            color={token.style ? (colorMap[token.style] ?? baseColor) : baseColor}
            text={token.text}
          />
        ))}
      </Text>
    </View>
  );
});

interface CodeLineTokenProps {
  color: string;
  text: string;
}

function CodeLineToken({ color, text }: CodeLineTokenProps) {
  const style = useMemo(() => ({ color }), [color]);
  return <Text style={style}>{text}</Text>;
}

const codeLineStyles = StyleSheet.create((theme) => ({
  line: {
    flexDirection: "row",
  },
  gutter: {
    alignItems: "flex-end",
    paddingRight: theme.spacing[3],
    flexShrink: 0,
  },
  gutterText: {
    fontFamily: Fonts.mono,
    fontSize: theme.fontSize.sm,
    lineHeight: theme.fontSize.sm * 1.45,
    opacity: 0.4,
    userSelect: "none",
  },
  lineText: {
    fontFamily: Fonts.mono,
    fontSize: theme.fontSize.sm,
    lineHeight: theme.fontSize.sm * 1.45,
    flex: 1,
  },
}));
