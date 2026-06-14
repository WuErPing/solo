import { describe, expect, it } from "vitest";
import { resolveTerminalColors } from "./resolve-terminal-colors";
import type { TerminalThemePreset } from "@/styles/terminal-themes";

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

const bashPreset: TerminalThemePreset = {
  label: "Bash",
  swatch: "#000000",
  background: "#000000",
  foreground: "#c0c0c0",
  black: "#000000",
  red: "#aa0000",
  green: "#00aa00",
  yellow: "#aa5500",
  blue: "#0000aa",
  magenta: "#aa00aa",
  cyan: "#00aaaa",
  white: "#aaaaaa",
  brightBlack: "#555555",
  brightRed: "#ff5555",
  brightGreen: "#55ff55",
  brightYellow: "#ffff55",
  brightBlue: "#5555ff",
  brightMagenta: "#ff55ff",
  brightCyan: "#55ffff",
  brightWhite: "#ffffff",
};

describe("resolveTerminalColors", () => {
  it("returns preset for dark", () => {
    expect(resolveTerminalColors("dark", baseTerminal, darkPreset)).toBe(darkPreset);
  });

  it("returns preset for light", () => {
    expect(resolveTerminalColors("light", baseTerminal, darkPreset)).toBe(darkPreset);
  });

  it("returns preset for bash", () => {
    expect(resolveTerminalColors("bash", baseTerminal, bashPreset)).toBe(bashPreset);
  });

  it("returns baseTerminal when preset is undefined", () => {
    expect(resolveTerminalColors("dark", baseTerminal, undefined)).toBe(baseTerminal);
  });

  it("returns baseTerminal for system (app base palette, no host injection)", () => {
    expect(resolveTerminalColors("system", baseTerminal, darkPreset)).toBe(baseTerminal);
  });

  it("does not merge or overlay — preset is returned as-is", () => {
    const result = resolveTerminalColors("bash", baseTerminal, bashPreset);
    expect(result.background).toBe("#000000");
    expect(result.foreground).toBe("#c0c0c0");
    expect(result.red).toBe("#aa0000");
  });

  describe("auto theme (opt-in: preset + host-reported defaults via OSC 10/11)", () => {
    it("overlays detected default bg onto the preset's background, keeping ANSI 0-15 unchanged", () => {
      const result = resolveTerminalColors("auto", baseTerminal, darkPreset, {
        background: "#1e1e2e",
        foreground: null,
      });
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe(darkPreset.foreground);
      expect(result.red).toBe(darkPreset.red);
      expect(result.green).toBe(darkPreset.green);
    });

    it("overlays detected default fg onto the preset's foreground", () => {
      const result = resolveTerminalColors("auto", baseTerminal, darkPreset, {
        background: null,
        foreground: "#cdd6f4",
      });
      expect(result.background).toBe(darkPreset.background);
      expect(result.foreground).toBe("#cdd6f4");
    });

    it("overlays both detected bg and fg when both are present", () => {
      const result = resolveTerminalColors("auto", baseTerminal, darkPreset, {
        background: "#1e1e2e",
        foreground: "#cdd6f4",
      });
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe("#cdd6f4");
      expect(result.red).toBe(darkPreset.red);
    });

    it("falls back to preset when detectedDefaults is null", () => {
      const result = resolveTerminalColors("auto", baseTerminal, darkPreset, null);
      expect(result).toBe(darkPreset);
    });

    it("falls back to baseTerminal when preset is undefined AND detectedDefaults is null", () => {
      const result = resolveTerminalColors("auto", baseTerminal, undefined, null);
      expect(result).toBe(baseTerminal);
    });

    it("applies detected defaults even when preset is undefined (base + overlay)", () => {
      const result = resolveTerminalColors("auto", baseTerminal, undefined, {
        background: "#1e1e2e",
        foreground: "#cdd6f4",
      });
      expect(result.background).toBe("#1e1e2e");
      expect(result.foreground).toBe("#cdd6f4");
      expect(result.red).toBe(baseTerminal.red);
    });

    it("returns a new object — does not mutate preset or baseTerminal", () => {
      const result = resolveTerminalColors("auto", baseTerminal, darkPreset, {
        background: "#1e1e2e",
        foreground: "#cdd6f4",
      });
      expect(result).not.toBe(darkPreset);
      expect(result).not.toBe(baseTerminal);
      expect(darkPreset.background).toBe("#181B1A");
      expect(baseTerminal.background).toBe("#000000");
    });
  });
});
