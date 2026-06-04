export interface DetectedColors {
  background: string | null;
  foreground: string | null;
}

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

  if (!bg) {
    const BG_TRUE = /\x1b\[48;2;(\d+);(\d+);(\d+)m/g;
    const BG_256 = /\x1b\[48;5;(\d+)m/g;
    let m: RegExpExecArray | null;
    while (!bg && (m = BG_TRUE.exec(content)) !== null) {
      bg = hex(parseInt(m[1]!, 10), parseInt(m[2]!, 10), parseInt(m[3]!, 10));
    }
    while (!bg && (m = BG_256.exec(content)) !== null) {
      bg = PALETTE_256[parseInt(m[1]!, 10)] ?? null;
    }
  }

  if (!fg) {
    const FG_TRUE = /\x1b\[38;2;(\d+);(\d+);(\d+)m/g;
    const FG_256 = /\x1b\[38;5;(\d+)m/g;
    let m: RegExpExecArray | null;
    while (!fg && (m = FG_TRUE.exec(content)) !== null) {
      fg = hex(parseInt(m[1]!, 10), parseInt(m[2]!, 10), parseInt(m[3]!, 10));
    }
    while (!fg && (m = FG_256.exec(content)) !== null) {
      fg = PALETTE_256[parseInt(m[1]!, 10)] ?? null;
    }
  }

  return { background: bg, foreground: fg };
}
