import React, { useMemo } from "react";
import { StyleSheet, Text, useWindowDimensions, View } from "react-native";
import { useUnistyles } from "react-native-unistyles";
import { SvgXml } from "react-native-svg";
import { Fonts } from "@/constants/theme";
import { fitScale, texToMathSvg, type MathSvgData } from "./math-svg";

// MathJax measures in ex units; for its fonts 1ex ≈ 0.53em. Body text is
// ~16px, so inline math lands at ~8.5px/ex; display math gets a bit more air.
const INLINE_PX_PER_EX = 8.5;
const BLOCK_PX_PER_EX = 9.8;

interface MathTextProps {
  tex: string;
  display?: boolean;
  isDark?: boolean;
}

function MathSvgView({
  data,
  scale = 1,
  translateY = 0,
}: {
  data: MathSvgData;
  scale?: number;
  translateY?: number;
}) {
  return (
    <SvgXml
      xml={data.xml}
      width={data.widthPx * scale}
      height={data.heightPx * scale}
      {...(translateY !== 0 ? { style: { transform: [{ translateY }] } } : null)}
    />
  );
}

export function MathText({ tex, display = false }: MathTextProps) {
  const { theme } = useUnistyles();
  const color = theme.colors.foreground;
  // SVG size is derived synchronously from the TeX source, so formulas paint
  // in the same pass as the surrounding text — no measuring, no re-layout,
  // no flicker (this replaces the previous WebView-based renderer).
  const data = useMemo(
    () => texToMathSvg(tex, display, color, display ? BLOCK_PX_PER_EX : INLINE_PX_PER_EX),
    [tex, display, color],
  );

  if (!data) {
    return <Text style={styles.fallback}>{tex}</Text>;
  }
  if (!display) {
    // Inline formula inside the paragraph's <Text>. On the new architecture
    // any View child becomes an inline attachment, so the SVG flows with the
    // text and wraps across lines like regular characters. baselineShiftPx
    // drops the box so its baseline sits on the text baseline.
    return (
      <View style={{ width: data.widthPx, height: data.heightPx }}>
        <MathSvgView data={data} translateY={data.baselineShiftPx} />
      </View>
    );
  }
  return <MathBlock data={data} />;
}

function MathBlock({ data }: { data: MathSvgData }) {
  const { width: windowWidth } = useWindowDimensions();
  const { theme } = useUnistyles();
  // The markdown preview pads its scroll content with spacing[4] on both
  // sides (see file-pane previewMarkdownScrollContent). onLayout proved
  // unreliable here, so the available width is derived from the window.
  const contentWidth = Math.max(0, windowWidth - 2 * theme.spacing[4]);
  const scale = fitScale(data.widthPx, contentWidth);
  return (
    <View style={styles.blockContainer}>
      <MathSvgView data={data} scale={scale} />
    </View>
  );
}

const styles = StyleSheet.create({
  blockContainer: {
    alignItems: "center",
    marginVertical: 8,
  },
  fallback: {
    fontFamily: Fonts.mono,
  },
});
