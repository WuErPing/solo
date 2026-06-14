import type { TerminalThemePreset, TerminalThemeId } from "@/styles/terminal-themes";

export interface DetectedDefaultColors {
  background: string | null;
  foreground: string | null;
}

export function resolveTerminalColors(
  themeId: TerminalThemeId,
  baseTerminal: { foreground: string; background: string; [key: string]: string },
  preset: TerminalThemePreset | undefined,
  detectedDefaults: DetectedDefaultColors | null = null,
) {
  if (themeId === "system") {
    return baseTerminal;
  }

  if (themeId === "auto") {
    const base = preset ?? baseTerminal;
    if (!detectedDefaults) return base;
    if (!detectedDefaults.background && !detectedDefaults.foreground) return base;
    return {
      ...base,
      ...(detectedDefaults.background ? { background: detectedDefaults.background } : {}),
      ...(detectedDefaults.foreground ? { foreground: detectedDefaults.foreground } : {}),
    };
  }

  return preset ?? baseTerminal;
}
