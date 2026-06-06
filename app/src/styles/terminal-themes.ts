export type TerminalThemeId =
  | "system"
  | "dark"
  | "light"
  | "midnight"
  | "ghostty"
  | "solarized-dark"
  | "monokai"
  | "dracula";

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

export const TERMINAL_THEME_IDS: TerminalThemeId[] = [
  "system",
  "dark",
  "light",
  "midnight",
  "ghostty",
  "solarized-dark",
  "monokai",
  "dracula",
];

export const TERMINAL_THEME_PRESETS: Record<
  Exclude<TerminalThemeId, "system">,
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
  midnight: {
    label: "Midnight",
    swatch: "#161820",
    background: "#161820",
    foreground: "#c0c8d8",
    black: "#121420",
    red: "#e07070",
    green: "#5dba80",
    yellow: "#d4a44a",
    blue: "#6a9de0",
    magenta: "#b07ad0",
    cyan: "#4aabb8",
    white: "#c0c8d8",
    brightBlack: "#3c3e4c",
    brightRed: "#e89090",
    brightGreen: "#7ecf9a",
    brightYellow: "#e0be6e",
    brightBlue: "#8ab4e8",
    brightMagenta: "#c49ae0",
    brightCyan: "#6ec2cc",
    brightWhite: "#f0f0f2",
  },
  ghostty: {
    label: "Ghostty",
    swatch: "#282c34",
    background: "#282c34",
    foreground: "#c8ccd8",
    black: "#21252d",
    red: "#e07070",
    green: "#5dba80",
    yellow: "#d4a44a",
    blue: "#6a9de0",
    magenta: "#b07ad0",
    cyan: "#4aabb8",
    white: "#c8ccd8",
    brightBlack: "#4a4f5e",
    brightRed: "#e89090",
    brightGreen: "#7ecf9a",
    brightYellow: "#e0be6e",
    brightBlue: "#8ab4e8",
    brightMagenta: "#c49ae0",
    brightCyan: "#6ec2cc",
    brightWhite: "#f0f0f2",
  },
  "solarized-dark": {
    label: "Solarized Dark",
    swatch: "#002b36",
    background: "#002b36",
    foreground: "#839496",
    black: "#073642",
    red: "#dc322f",
    green: "#859900",
    yellow: "#b58900",
    blue: "#268bd2",
    magenta: "#d33682",
    cyan: "#2aa198",
    white: "#eee8d5",
    brightBlack: "#586e75",
    brightRed: "#cb4b16",
    brightGreen: "#586e75",
    brightYellow: "#657b83",
    brightBlue: "#839496",
    brightMagenta: "#6c71c4",
    brightCyan: "#93a1a1",
    brightWhite: "#fdf6e3",
  },
  monokai: {
    label: "Monokai",
    swatch: "#272822",
    background: "#272822",
    foreground: "#f8f8f2",
    black: "#272822",
    red: "#f92672",
    green: "#a6e22e",
    yellow: "#f4bf75",
    blue: "#66d9ef",
    magenta: "#ae81ff",
    cyan: "#a1efe4",
    white: "#f8f8f2",
    brightBlack: "#75715e",
    brightRed: "#f92672",
    brightGreen: "#a6e22e",
    brightYellow: "#f4bf75",
    brightBlue: "#66d9ef",
    brightMagenta: "#ae81ff",
    brightCyan: "#a1efe4",
    brightWhite: "#f9f8f5",
  },
  dracula: {
    label: "Dracula",
    swatch: "#282a36",
    background: "#282a36",
    foreground: "#f8f8f2",
    black: "#21222c",
    red: "#ff5555",
    green: "#50fa7b",
    yellow: "#f1fa8c",
    blue: "#bd93f9",
    magenta: "#ff79c6",
    cyan: "#8be9fd",
    white: "#f8f8f2",
    brightBlack: "#6272a4",
    brightRed: "#ff6e6e",
    brightGreen: "#69ff94",
    brightYellow: "#ffffa5",
    brightBlue: "#d6acff",
    brightMagenta: "#ff92df",
    brightCyan: "#a4ffff",
    brightWhite: "#ffffff",
  },
};
