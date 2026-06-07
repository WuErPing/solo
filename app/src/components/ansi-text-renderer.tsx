import React, { useMemo } from "react";
import { Text, type TextStyle } from "react-native";
import { useUnistyles } from "react-native-unistyles";
import type { AnsiSegment, AnsiStyle } from "@/utils/ansi-parser";
import { resolveColor } from "@/utils/ansi-color-palette";

interface AnsiTextContentProps {
  segments: AnsiSegment[];
  style?: TextStyle;
  terminalColors?: { foreground: string; background: string; [key: string]: string };
}

export const AnsiTextContent = React.memo(function AnsiTextContent({
  segments,
  style,
  terminalColors,
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

  return <Text style={style}>{children}</Text>;
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
