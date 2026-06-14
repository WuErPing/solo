import React, { useMemo } from "react";
import { Text, type TextStyle } from "react-native";
import type { AnsiSegment } from "@/utils/ansi-parser";
import { ansiStyleToRN } from "@/utils/ansi-style-to-rn";

interface AnsiTextLineProps {
  segments: AnsiSegment[];
  style?: TextStyle;
  terminalColors: { foreground: string; background: string; [key: string]: string };
  selectable?: boolean;
}

export const AnsiTextLine = React.memo(function AnsiTextLine({
  segments,
  style,
  terminalColors,
  selectable,
}: AnsiTextLineProps) {
  const children = useMemo(() => {
    return segments.map((seg, index) => {
      const segStyle = ansiStyleToRN(seg.style, terminalColors);
      return (
        <Text key={index} style={segStyle}>
          {seg.text}
        </Text>
      );
    });
  }, [segments, terminalColors]);

  return <Text style={style} selectable={selectable}>{children}</Text>;
});
