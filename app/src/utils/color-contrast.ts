interface Rgb {
  r: number;
  g: number;
  b: number;
}

function parseColor(color: string): Rgb | null {
  const hex6 = /^#([0-9a-fA-F]{6})$/;
  const hex3 = /^#([0-9a-fA-F]{3})$/;
  const rgb = /^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$/;

  if (hex6.test(color)) {
    const hex = color.slice(1);
    return {
      r: parseInt(hex.slice(0, 2), 16),
      g: parseInt(hex.slice(2, 4), 16),
      b: parseInt(hex.slice(4, 6), 16),
    };
  }

  if (hex3.test(color)) {
    const hex = color.slice(1);
    return {
      r: parseInt(hex[0]! + hex[0]!, 16),
      g: parseInt(hex[1]! + hex[1]!, 16),
      b: parseInt(hex[2]! + hex[2]!, 16),
    };
  }

  const rgbMatch = rgb.exec(color);
  if (rgbMatch) {
    return {
      r: Math.max(0, Math.min(255, parseInt(rgbMatch[1]!, 10))),
      g: Math.max(0, Math.min(255, parseInt(rgbMatch[2]!, 10))),
      b: Math.max(0, Math.min(255, parseInt(rgbMatch[3]!, 10))),
    };
  }

  return null;
}

function rgbToHex({ r, g, b }: Rgb): string {
  const toHex = (v: number) => Math.round(v).toString(16).padStart(2, "0");
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
}

function blend(a: Rgb, b: Rgb, t: number): Rgb {
  return {
    r: a.r + (b.r - a.r) * t,
    g: a.g + (b.g - a.g) * t,
    b: a.b + (b.b - a.b) * t,
  };
}

function channelLuminance(channel: number): number {
  const c = channel / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function relativeLuminance({ r, g, b }: Rgb): number {
  return 0.2126 * channelLuminance(r) + 0.7152 * channelLuminance(g) + 0.0722 * channelLuminance(b);
}

export function contrastRatio(a: string, b: string): number {
  const colorA = parseColor(a);
  const colorB = parseColor(b);
  if (!colorA || !colorB) return 0;
  const lumA = relativeLuminance(colorA);
  const lumB = relativeLuminance(colorB);
  return (Math.max(lumA, lumB) + 0.05) / (Math.min(lumA, lumB) + 0.05);
}

const contrastCache = new Map<string, string>();
const MAX_CONTRAST_CACHE_SIZE = 512;

/**
 * Ensures `color` is readable against `background` by blending it toward the
 * contrasting extreme (white on dark backgrounds, black on light backgrounds)
 * until the WCAG contrast ratio reaches `minRatio`.
 *
 * Unparseable colors are returned unchanged so the renderer never drops text.
 */
export function ensureContrast(
  color: string,
  background: string,
  minRatio: number,
): string {
  const key = `${color}|${background}|${minRatio}`;
  const cached = contrastCache.get(key);
  if (cached) return cached;

  const fg = parseColor(color);
  const bg = parseColor(background);
  if (!fg || !bg) {
    contrastCache.set(key, color);
    return color;
  }

  if (contrastRatio(color, background) >= minRatio) {
    contrastCache.set(key, color);
    return color;
  }

  // Dark backgrounds need a lighter foreground; light backgrounds need darker.
  const bgLum = relativeLuminance(bg);
  const target: Rgb = bgLum >= 0.5 ? { r: 0, g: 0, b: 0 } : { r: 255, g: 255, b: 255 };

  let low = 0;
  let high = 1;
  let result = target;
  for (let i = 0; i < 12; i++) {
    const mid = (low + high) / 2;
    const candidate = blend(fg, target, mid);
    if (contrastRatio(rgbToHex(candidate), background) >= minRatio) {
      result = candidate;
      high = mid;
    } else {
      low = mid;
    }
  }

  const hex = rgbToHex(result);
  if (contrastCache.size >= MAX_CONTRAST_CACHE_SIZE) {
    contrastCache.clear();
  }
  contrastCache.set(key, hex);
  return hex;
}
