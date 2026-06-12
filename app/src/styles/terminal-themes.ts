export type TerminalThemeId = "system" | "dark" | "light" | "tmux";

export interface TerminalThemePreset {
  label: string;
  swatch: string;
  background: string;
  foreground: string;
  [key: string]: string;
  black: string;
  red: string;
  green: string;
  yellow: string;
  blue: string;
  magenta: string;
  cyan: string;
  white: string;
  brightBlack: string;
  brightRed: string;
  brightGreen: string;
  brightYellow: string;
  brightBlue: string;
  brightMagenta: string;
  brightCyan: string;
  brightWhite: string;
}

export const TERMINAL_THEME_IDS: TerminalThemeId[] = ["system", "dark", "light", "tmux"];

export const TERMINAL_THEME_PRESETS: Record<
  Exclude<TerminalThemeId, "system" | "tmux">,
  TerminalThemePreset
> = {
  dark: {
    label: "Dark",
    swatch: "#181B1A",
    background: "#181B1A",
    foreground: "#fafafa",
    black: "#141716",
    red: "#e07070",
    green: "#5dba80",
    yellow: "#d4a44a",
    blue: "#6a9de0",
    magenta: "#b07ad0",
    cyan: "#4aabb8",
    white: "#d4d4d8",
    brightBlack: "#434645",
    brightRed: "#e89090",
    brightGreen: "#7ecf9a",
    brightYellow: "#e0be6e",
    brightBlue: "#8ab4e8",
    brightMagenta: "#c49ae0",
    brightCyan: "#6ec2cc",
    brightWhite: "#f0f0f2",
  },
  light: {
    label: "Light",
    swatch: "#ffffff",
    background: "#ffffff",
    foreground: "#1a1a1e",
    black: "#1a1a1e",
    red: "#dc2626",
    green: "#16a34a",
    yellow: "#ca8a04",
    blue: "#2563eb",
    magenta: "#9333ea",
    cyan: "#0891b2",
    white: "#ffffff",
    brightBlack: "#3f3f46",
    brightRed: "#ef4444",
    brightGreen: "#22c55e",
    brightYellow: "#f59e0b",
    brightBlue: "#3b82f6",
    brightMagenta: "#a855f7",
    brightCyan: "#06b6d4",
    brightWhite: "#fafafa",
  },
};
