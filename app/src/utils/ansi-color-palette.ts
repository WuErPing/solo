import type { AnsiColor } from "./ansi-parser";

const BASIC_KEYS = [
  "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
  "brightBlack", "brightRed", "brightGreen", "brightYellow",
  "brightBlue", "brightMagenta", "brightCyan", "brightWhite",
] as const;

function hex(r: number, g: number, b: number): string {
  return `#${((1 << 24) | (r << 16) | (g << 8) | b).toString(16).slice(1)}`;
}

const PALETTE_256: string[] = (() => {
  const p: string[] = new Array(256);
  p[0] = "#000000"; p[1] = "#800000"; p[2] = "#008000"; p[3] = "#808000";
  p[4] = "#000080"; p[5] = "#800080"; p[6] = "#008080"; p[7] = "#c0c0c0";
  p[8] = "#808080"; p[9] = "#ff0000"; p[10] = "#00ff00"; p[11] = "#ffff00";
  p[12] = "#0000ff"; p[13] = "#ff00ff"; p[14] = "#00ffff"; p[15] = "#ffffff";
  for (let r = 0; r < 6; r++) {
    for (let g = 0; g < 6; g++) {
      for (let b = 0; b < 6; b++) {
        const idx = 16 + 36 * r + 6 * g + b;
        p[idx] = hex(r ? 55 + r * 40 : 0, g ? 55 + g * 40 : 0, b ? 55 + b * 40 : 0);
      }
    }
  }
  for (let k = 0; k < 24; k++) {
    const v = 8 + k * 10;
    p[232 + k] = hex(v, v, v);
  }
  return p;
})();

interface TerminalColors {
  foreground: string;
  background: string;
  [key: string]: string;
}

export function resolveColor(
  color: AnsiColor | null,
  terminal: TerminalColors,
  fallback: string | null,
): string | null {
  if (!color) return fallback;
  if (color.type === "basic") {
    const key = BASIC_KEYS[color.index];
    return key ? terminal[key] ?? fallback : fallback;
  }
  if (color.type === "palette") {
    if (color.index < 16) {
      const key = BASIC_KEYS[color.index];
      return key ? terminal[key] ?? fallback : fallback;
    }
    return PALETTE_256[color.index] ?? fallback;
  }
  return `rgb(${color.r},${color.g},${color.b})`;
}
