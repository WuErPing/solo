import React, { useMemo } from "react";
import { Text, type TextStyle } from "react-native";
import type { AnsiSegment } from "@/utils/ansi-parser";
import { resolveColor } from "@/utils/ansi-color-palette";
import type { AnsiStyle } from "@/utils/ansi-parser";

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

function ansiStyleToRN(
  style: AnsiStyle,
  terminal: { foreground: string; background: string; [key: string]: string },
): TextStyle | undefined {
  let fg = resolveColor(style.fg, terminal, terminal.foreground);
  let bg = resolveColor(style.bg, terminal, null);

  if (style.inverse) {
    const tmp = fg;
    fg = bg ?? terminal.background;
    bg = tmp ?? terminal.foreground;
  }

  const result: TextStyle = {};
  let hasProps = false;

  if (fg && fg !== terminal.foreground) {
    result.color = fg;
    hasProps = true;
  }
  if (bg) {
    result.backgroundColor = bg;
    hasProps = true;
  }
  if (style.bold) {
    result.fontWeight = "bold";
    hasProps = true;
  }
  if (style.dim) {
    result.opacity = 0.5;
    hasProps = true;
  }
  if (style.italic) {
    result.fontStyle = "italic";
    hasProps = true;
  }
  if (style.underline && style.strikethrough) {
    result.textDecorationLine = "underline line-through";
    hasProps = true;
  } else if (style.underline) {
    result.textDecorationLine = "underline";
    hasProps = true;
  } else if (style.strikethrough) {
    result.textDecorationLine = "line-through";
    hasProps = true;
  }

  return hasProps ? result : undefined;
}
