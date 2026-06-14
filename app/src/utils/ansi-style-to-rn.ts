import type { TextStyle } from "react-native";
import type { AnsiStyle } from "@/utils/ansi-parser";
import { resolveColor } from "@/utils/ansi-color-palette";
import { ensureContrast } from "@/utils/color-contrast";

/**
 * Minimum contrast ratio for ANSI-rendered text.
 *
 * WCAG AA requires 3:1 for large text / UI components and 4.5:1 for normal
 * text. Terminal applications intentionally use subtle colors for decorative
 * elements, so 3:1 strikes a balance: it prevents "invisible" low-contrast
 * text while preserving the intended muted appearance.
 */
export const MIN_ANSI_CONTRAST_RATIO = 3;

export function ansiStyleToRN(
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

  // Only adjust explicitly-set foreground colors. The terminal's default
  // foreground is already chosen to contrast with its background.
  // Inverse styles intentionally swap colors (e.g., selected text), so leave
  // those untouched to preserve the author's intent.
  const effectiveBg = bg ?? terminal.background;
  const safeFg =
    !style.inverse && style.fg && fg
      ? ensureContrast(fg, effectiveBg, MIN_ANSI_CONTRAST_RATIO)
      : fg;

  const result: TextStyle = {};
  let hasProps = false;

  if (safeFg && safeFg !== terminal.foreground) {
    result.color = safeFg;
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
