export interface OscDefaultColors {
  background: string | null;
  foreground: string | null;
}

// Matches OSC 10 / 11 replies of the form:
//   ESC ] 10 ; rgb:XXXX/YYYY/ZZZZ (BEL | ESC \)
//   ESC ] 11 ; rgb:XXXX/YYYY/ZZZZ (BEL | ESC \)
// where each component is 1–4 hex digits. Terminals vary the bit width
// (iTerm2: 16-bit, Alacritty: 8-bit, some legacy: 4-bit). We normalize
// each component to 16 bits by nibble-replication (X11 convention):
//   "1"   -> "1111"
//   "1e"  -> "1e1e"
//   "1e1" -> "1e11e1" truncated to "1e11" — but no real terminal emits 3-digit.
//   "1e1e"-> "1e1e"
// Then take the high byte for 8-bit output.
const OSC_REPLY_RE = /\x1b](10|11);[rR][gG][bB]:([0-9a-fA-F]{1,4})\/([0-9a-fA-F]{1,4})\/([0-9a-fA-F]{1,4})(?:\x07|\x1b\\)/g;

function normalizeComponent(hex: string): string {
  let v: number;
  switch (hex.length) {
    case 1:
      v = parseInt(hex + hex + hex + hex, 16);
      break;
    case 2:
      v = parseInt(hex + hex, 16);
      break;
    case 3:
      // Non-standard but seen: treat as high 12 bits of a 16-bit value.
      v = parseInt((hex + "0").slice(0, 4), 16);
      break;
    default:
      v = parseInt(hex.slice(0, 4), 16);
  }
  const highByte = (v >> 8) & 0xff;
  return highByte.toString(16).padStart(2, "0");
}

export function parseOscDefaultColors(content: string): OscDefaultColors {
  let background: string | null = null;
  let foreground: string | null = null;

  OSC_REPLY_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = OSC_REPLY_RE.exec(content)) !== null) {
    const code = m[1];
    const r = normalizeComponent(m[2]!);
    const g = normalizeComponent(m[3]!);
    const b = normalizeComponent(m[4]!);
    const hex = `#${r}${g}${b}`;
    if (code === "10") foreground = hex;
    else if (code === "11") background = hex;
  }

  return { background, foreground };
}
