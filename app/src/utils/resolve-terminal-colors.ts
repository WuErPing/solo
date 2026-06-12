import type { DetectedColors } from "./detect-ansi-colors";
import type { TerminalThemePreset, TerminalThemeId } from "@/styles/terminal-themes";

export function resolveTerminalColors(
  themeId: TerminalThemeId,
  baseTerminal: { foreground: string; background: string; [key: string]: string },
  preset: TerminalThemePreset | undefined,
  detectedColors: DetectedColors | null,
  tmuxColors: { background?: string; foreground?: string } | null,
) {
  if (themeId === "dark" || themeId === "light") {
    return preset ?? baseTerminal;
  }

  if (themeId === "tmux") {
    if (!tmuxColors?.background && !tmuxColors?.foreground) return baseTerminal;
    return {
      ...baseTerminal,
      background: tmuxColors.background || baseTerminal.background,
      foreground: tmuxColors.foreground || baseTerminal.foreground,
    };
  }

  // "system": per-color priority — ANSI > tmux > base
  const ansiBg = detectedColors?.background;
  const ansiFg = detectedColors?.foreground;
  const tmuxBg = tmuxColors?.background;
  const tmuxFg = tmuxColors?.foreground;
  if (ansiBg || ansiFg || tmuxBg || tmuxFg) {
    return {
      ...baseTerminal,
      background: ansiBg || tmuxBg || baseTerminal.background,
      foreground: ansiFg || tmuxFg || baseTerminal.foreground,
    };
  }

  return baseTerminal;
}
