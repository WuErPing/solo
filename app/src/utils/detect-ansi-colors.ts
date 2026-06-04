import type { AnsiColor } from "./ansi-parser";

export interface DetectedColors {
  background: string | null;
  foreground: string | null;
}

function ansiColorToHex(color: AnsiColor): string | null {
  if (color.type === "rgb") {
    return `#${((1 << 24) | (color.r << 16) | (color.g << 8) | color.b).toString(16).slice(1)}`;
  }
  return null;
}

function parseRgbHex(hex: string): string {
  return hex.padStart(2, "0");
}

export function detectColorsFromAnsi(content: string): DetectedColors {
  let bg: string | null = null;
  let fg: string | null = null;

  const OSC_REGEX = /\x1b\](\d+);([^\x07\x1b]*)(?:\x07|\x1b\\)/g;
  let oscMatch: RegExpExecArray | null;
  while ((oscMatch = OSC_REGEX.exec(content)) !== null) {
    const code = parseInt(oscMatch[1]!, 10);
    const value = oscMatch[2]?.trim() ?? "";
    if (code === 11 && value.startsWith("rgb:")) {
      const parts = value.slice(4).split("/");
      if (parts.length === 3) {
        bg = `#${parseRgbHex(parts[0]!)}${parseRgbHex(parts[1]!)}${parseRgbHex(parts[2]!)}`;
      }
    } else if (code === 10 && value.startsWith("rgb:")) {
      const parts = value.slice(4).split("/");
      if (parts.length === 3) {
        fg = `#${parseRgbHex(parts[0]!)}${parseRgbHex(parts[1]!)}${parseRgbHex(parts[2]!)}`;
      }
    }
  }

  if (!bg || !fg) {
    const SGR_BG_REGEX = /\x1b\[48;2;(\d+);(\d+);(\d+)m/g;
    const SGR_FG_REGEX = /\x1b\[38;2;(\d+);(\d+);(\d+)m/g;

    let sgrMatch: RegExpExecArray | null;
    while (!bg && (sgrMatch = SGR_BG_REGEX.exec(content)) !== null) {
      const r = parseInt(sgrMatch[1]!, 10);
      const g = parseInt(sgrMatch[2]!, 10);
      const b = parseInt(sgrMatch[3]!, 10);
      bg = `#${((1 << 24) | (r << 16) | (g << 8) | b).toString(16).slice(1)}`;
    }
    while (!fg && (sgrMatch = SGR_FG_REGEX.exec(content)) !== null) {
      const r = parseInt(sgrMatch[1]!, 10);
      const g = parseInt(sgrMatch[2]!, 10);
      const b = parseInt(sgrMatch[3]!, 10);
      fg = `#${((1 << 24) | (r << 16) | (g << 8) | b).toString(16).slice(1)}`;
    }
  }

  return { background: bg, foreground: fg };
}
