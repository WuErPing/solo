import { describe, expect, it } from "vitest";
import { resolveTerminalColors } from "./resolve-terminal-colors";
import type { DetectedColors } from "./detect-ansi-colors";
import type { TerminalThemePreset, TerminalThemeId } from "@/styles/terminal-themes";

const baseTerminal = {
  background: "#000000",
  foreground: "#ffffff",
  black: "#111111",
  red: "#ff0000",
  green: "#00ff00",
  yellow: "#ffff00",
  blue: "#0000ff",
  magenta: "#ff00ff",
  cyan: "#00ffff",
  white: "#cccccc",
  brightBlack: "#333333",
  brightRed: "#ff3333",
  brightGreen: "#33ff33",
  brightYellow: "#ffff33",
  brightBlue: "#3333ff",
  brightMagenta: "#ff33ff",
  brightCyan: "#33ffff",
  brightWhite: "#ffffff",
  cursor: "#fff",
  cursorAccent: "#000",
  selectionBackground: "#333",
  selectionForeground: "#fff",
};

const darkPreset: TerminalThemePreset = {
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
};

describe("resolveTerminalColors", () => {
  describe("dark/light themes", () => {
    it("returns preset for dark", () => {
      const result = resolveTerminalColors("dark", baseTerminal, darkPreset, null, null);
      expect(result).toBe(darkPreset);
    });

    it("returns base when preset is undefined", () => {
      const result = resolveTerminalColors("dark", baseTerminal, undefined, null, null);
      expect(result).toBe(baseTerminal);
    });
  });

  describe("tmux theme", () => {
    it("overlays tmux bg/fg onto base palette", () => {
      const tmuxColors = { background: "#1e1e2e", foreground: "#cdd6f4" };
      const result = resolveTerminalColors("tmux", baseTerminal, undefined, null, tmuxColors);
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe("#cdd6f4");
      expect(result.black).toBe(baseTerminal.black);
      expect(result.red).toBe(baseTerminal.red);
    });

    it("returns base when tmuxColors is null", () => {
      const result = resolveTerminalColors("tmux", baseTerminal, undefined, null, null);
      expect(result).toBe(baseTerminal);
    });

    it("returns base when tmuxColors has no bg/fg", () => {
      const result = resolveTerminalColors("tmux", baseTerminal, undefined, null, {});
      expect(result).toBe(baseTerminal);
    });

    it("uses tmux bg with base fg when only bg is provided", () => {
      const tmuxColors = { background: "#1e1e2e" };
      const result = resolveTerminalColors("tmux", baseTerminal, undefined, null, tmuxColors);
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe(baseTerminal.foreground);
    });
  });

  describe("system theme", () => {
    it("uses ANSI detected colors (highest priority)", () => {
      const detected: DetectedColors = { background: "#2a2a3e", foreground: "#f0f0f0" };
      const tmuxColors = { background: "#1e1e2e", foreground: "#cdd6f4" };
      const result = resolveTerminalColors("system", baseTerminal, undefined, detected, tmuxColors);
      expect(result.background).toBe("#2a2a3e");
      expect(result.foreground).toBe("#f0f0f0");
    });

    it("falls back to tmux colors when ANSI detection returns null", () => {
      const detected: DetectedColors = { background: null, foreground: null };
      const tmuxColors = { background: "#1e1e2e", foreground: "#cdd6f4" };
      const result = resolveTerminalColors("system", baseTerminal, undefined, detected, tmuxColors);
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe("#cdd6f4");
    });

    it("falls back to base when both ANSI and tmux are null", () => {
      const result = resolveTerminalColors("system", baseTerminal, undefined, null, null);
      expect(result).toBe(baseTerminal);
    });

    it("ANSI bg + tmux fg partial mix", () => {
      const detected: DetectedColors = { background: "#2a2a3e", foreground: null };
      const tmuxColors = { foreground: "#cdd6f4" };
      const result = resolveTerminalColors("system", baseTerminal, undefined, detected, tmuxColors);
      expect(result.background).toBe("#2a2a3e");
      expect(result.foreground).toBe("#cdd6f4");
    });

    it("ANSI fg only falls back bg to tmux", () => {
      const detected: DetectedColors = { background: null, foreground: "#f0f0f0" };
      const tmuxColors = { background: "#1e1e2e" };
      const result = resolveTerminalColors("system", baseTerminal, undefined, detected, tmuxColors);
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe("#f0f0f0");
    });
  });
});
