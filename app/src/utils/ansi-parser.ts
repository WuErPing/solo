export type AnsiColor =
  | { type: "basic"; index: number }
  | { type: "palette"; index: number }
  | { type: "rgb"; r: number; g: number; b: number };

export interface AnsiStyle {
  bold: boolean;
  dim: boolean;
  italic: boolean;
  underline: boolean;
  strikethrough: boolean;
  inverse: boolean;
  fg: AnsiColor | null;
  bg: AnsiColor | null;
}

export interface AnsiSegment {
  text: string;
  style: AnsiStyle;
}

function defaultStyle(): AnsiStyle {
  return {
    bold: false,
    dim: false,
    italic: false,
    underline: false,
    strikethrough: false,
    inverse: false,
    fg: null,
    bg: null,
  };
}

function stylesEqual(a: AnsiStyle, b: AnsiStyle): boolean {
  return (
    a.bold === b.bold &&
    a.dim === b.dim &&
    a.italic === b.italic &&
    a.underline === b.underline &&
    a.strikethrough === b.strikethrough &&
    a.inverse === b.inverse &&
    colorEqual(a.fg, b.fg) &&
    colorEqual(a.bg, b.bg)
  );
}

function colorEqual(a: AnsiColor | null, b: AnsiColor | null): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  if (a.type !== b.type) return false;
  if (a.type === "basic") return a.index === (b as { type: "basic"; index: number }).index;
  if (a.type === "palette") return a.index === (b as { type: "palette"; index: number }).index;
  return (
    a.r === (b as { type: "rgb"; r: number; g: number; b: number }).r &&
    a.g === (b as { type: "rgb"; r: number; g: number; b: number }).g &&
    a.b === (b as { type: "rgb"; r: number; g: number; b: number }).b
  );
}

function applySgr(params: number[], style: AnsiStyle): void {
  let j = 0;
  while (j < params.length) {
    const code = params[j]!;
    switch (true) {
      case code === 0:
        Object.assign(style, defaultStyle());
        break;
      case code === 1:
        style.bold = true;
        break;
      case code === 2:
        style.dim = true;
        break;
      case code === 3:
        style.italic = true;
        break;
      case code === 4:
        style.underline = true;
        break;
      case code === 7:
        style.inverse = true;
        break;
      case code === 9:
        style.strikethrough = true;
        break;
      case code === 21:
        style.bold = false;
        break;
      case code === 22:
        style.bold = false;
        style.dim = false;
        break;
      case code === 23:
        style.italic = false;
        break;
      case code === 24:
        style.underline = false;
        break;
      case code === 27:
        style.inverse = false;
        break;
      case code === 29:
        style.strikethrough = false;
        break;
      case code >= 30 && code <= 37:
        style.fg = { type: "basic", index: code - 30 };
        break;
      case code === 38:
        if (params[j + 1] === 5 && j + 2 < params.length) {
          style.fg = { type: "palette", index: params[j + 2]! };
          j += 2;
        } else if (params[j + 1] === 2 && j + 4 < params.length) {
          style.fg = { type: "rgb", r: params[j + 2]!, g: params[j + 3]!, b: params[j + 4]! };
          j += 4;
        }
        break;
      case code === 39:
        style.fg = null;
        break;
      case code >= 40 && code <= 47:
        style.bg = { type: "basic", index: code - 40 };
        break;
      case code === 48:
        if (params[j + 1] === 5 && j + 2 < params.length) {
          style.bg = { type: "palette", index: params[j + 2]! };
          j += 2;
        } else if (params[j + 1] === 2 && j + 4 < params.length) {
          style.bg = { type: "rgb", r: params[j + 2]!, g: params[j + 3]!, b: params[j + 4]! };
          j += 4;
        }
        break;
      case code === 49:
        style.bg = null;
        break;
      case code >= 90 && code <= 97:
        style.fg = { type: "basic", index: code - 90 + 8 };
        break;
      case code >= 100 && code <= 107:
        style.bg = { type: "basic", index: code - 100 + 8 };
        break;
    }
    j++;
  }
}

export function parseAnsi(input: string): AnsiSegment[] {
  const segments: AnsiSegment[] = [];
  const style = defaultStyle();
  let textBuf = "";
  let i = 0;

  const flush = () => {
    if (textBuf.length > 0) {
      const last = segments[segments.length - 1];
      if (last && stylesEqual(last.style, style)) {
        last.text += textBuf;
      } else {
        segments.push({ text: textBuf, style: { ...style } });
      }
      textBuf = "";
    }
  };

  while (i < input.length) {
    const ch = input.charCodeAt(i);

    if (ch === 0x1b && i + 1 < input.length) {
      const next = input.charCodeAt(i + 1);

      if (next === 0x5b) {
        flush();
        let end = i + 2;
        while (end < input.length && input.charCodeAt(end) < 0x40) end++;
        if (end >= input.length) break;
        const final = input.charAt(end);
        if (final === "m") {
          const paramStr = input.slice(i + 2, end);
          const params = paramStr.length === 0 ? [0] : paramStr.split(";").map(Number);
          applySgr(params, style);
        }
        i = end + 1;
        continue;
      }

      if (next === 0x5d) {
        flush();
        let end = i + 2;
        while (end < input.length && input.charCodeAt(end) !== 0x07) {
          if (
            input.charCodeAt(end) === 0x1b &&
            end + 1 < input.length &&
            input.charCodeAt(end + 1) === 0x5c
          ) {
            end++;
            break;
          }
          end++;
        }
        i = end + 1;
        continue;
      }

      if (next === 0x28 || next === 0x29) {
        flush();
        i += 3;
        continue;
      }

      i++;
      continue;
    }

    if (ch === 0x00 || (ch >= 0x01 && ch <= 0x08) || ch === 0x0b || ch === 0x0c || (ch >= 0x0e && ch <= 0x1f) || ch === 0x7f) {
      i++;
      continue;
    }

    const cp = input.codePointAt(i)!;
    if ((cp >= 0x2500 && cp <= 0x259f) || (cp >= 0x2800 && cp <= 0x28ff)) {
      i += cp > 0xffff ? 2 : 1;
      continue;
    }

    if (cp > 0xffff) {
      textBuf += String.fromCodePoint(cp);
      i += 2;
    } else {
      textBuf += input[i]!;
      i++;
    }
  }

  flush();
  return segments;
}
