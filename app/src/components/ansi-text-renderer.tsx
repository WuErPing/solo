import React, { useMemo } from "react";
import { Text, type TextStyle } from "react-native";
import { useUnistyles } from "react-native-unistyles";
import type { AnsiSegment } from "@/utils/ansi-parser";
import { ansiStyleToRN } from "@/utils/ansi-style-to-rn";

interface AnsiTextContentProps {
  segments: AnsiSegment[];
  style?: TextStyle;
  terminalColors?: { foreground: string; background: string; [key: string]: string };
  selectable?: boolean;
}

export const AnsiTextContent = React.memo(function AnsiTextContent({
  segments,
  style,
  terminalColors,
  selectable,
}: AnsiTextContentProps) {
  const { theme } = useUnistyles();
  const terminal = terminalColors ?? theme.colors.terminal;

  const children = useMemo(() => {
    return segments.map((seg, index) => {
      const segStyle = ansiStyleToRN(seg.style, terminal);
      return (
        <Text key={index} style={segStyle}>
          {seg.text}
        </Text>
      );
    });
  }, [segments, terminal]);

  return <Text style={style} selectable={selectable}>{children}</Text>;
});
