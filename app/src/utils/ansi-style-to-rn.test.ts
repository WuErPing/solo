import { describe, expect, it } from "vitest";
import { ansiStyleToRN, MIN_ANSI_CONTRAST_RATIO } from "./ansi-style-to-rn";
import { contrastRatio } from "./color-contrast";
import type { AnsiStyle } from "./ansi-parser";

const defaultStyle: AnsiStyle = {
  bold: false,
  dim: false,
  italic: false,
  underline: false,
  strikethrough: false,
  inverse: false,
  fg: null,
  bg: null,
};

const darkTerminal = {
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

describe("ansiStyleToRN", () => {
  it("returns undefined for the default style", () => {
    expect(ansiStyleToRN(defaultStyle, darkTerminal)).toBeUndefined();
  });

  it("does not adjust the terminal's default foreground", () => {
    const style: AnsiStyle = { ...defaultStyle };
    const result = ansiStyleToRN(style, darkTerminal);
    expect(result).toBeUndefined();
  });

  it("keeps high-contrast 256-color foregrounds unchanged", () => {
    const style: AnsiStyle = { ...defaultStyle, fg: { type: "palette", index: 231 } };
    const result = ansiStyleToRN(style, darkTerminal);
    expect(result?.color).toBe("#ffffff");
  });

  it("lightens low-contrast dark-gray foregrounds on a dark background", () => {
    const style: AnsiStyle = { ...defaultStyle, fg: { type: "palette", index: 59 } };
    const result = ansiStyleToRN(style, darkTerminal);
    expect(result?.color).toBeDefined();
    expect(result?.color).not.toBe("#5f5f5f");
    expect(
      contrastRatio(result?.color as string, darkTerminal.background),
    ).toBeGreaterThanOrEqual(MIN_ANSI_CONTRAST_RATIO);
  });

  it("keeps low-contrast colors readable on a light background", () => {
    const lightTerminal = {
      ...darkTerminal,
      background: "#ffffff",
      foreground: "#1a1a1e",
    };
    const style: AnsiStyle = { ...defaultStyle, fg: { type: "palette", index: 59 } };
    const result = ansiStyleToRN(style, lightTerminal);
    expect(result?.color).toBe("#5f5f5f");
    expect(
      contrastRatio(result?.color as string, lightTerminal.background),
    ).toBeGreaterThanOrEqual(MIN_ANSI_CONTRAST_RATIO);
  });

  it("adjusts true-color foregrounds when needed", () => {
    const style: AnsiStyle = { ...defaultStyle, fg: { type: "rgb", r: 24, g: 24, b: 37 } };
    const result = ansiStyleToRN(style, darkTerminal);
    expect(
      contrastRatio(result?.color as string, darkTerminal.background),
    ).toBeGreaterThanOrEqual(MIN_ANSI_CONTRAST_RATIO);
  });

  it("preserves explicit background colors", () => {
    const style: AnsiStyle = {
      ...defaultStyle,
      fg: { type: "palette", index: 231 },
      bg: { type: "palette", index: 59 },
    };
    const result = ansiStyleToRN(style, darkTerminal);
    expect(result?.backgroundColor).toBe("#5f5f5f");
  });

  it("swaps colors for inverse styles", () => {
    const style: AnsiStyle = {
      ...defaultStyle,
      fg: { type: "palette", index: 59 },
      inverse: true,
    };
    const result = ansiStyleToRN(style, darkTerminal);
    // Inverse: fg becomes the terminal background, bg becomes the palette color.
    expect(result?.color).toBe(darkTerminal.background);
    expect(result?.backgroundColor).toBe("#5f5f5f");
  });

  it("preserves text decoration flags", () => {
    const style: AnsiStyle = { ...defaultStyle, bold: true, underline: true };
    const result = ansiStyleToRN(style, darkTerminal);
    expect(result).toEqual({ fontWeight: "bold", textDecorationLine: "underline" });
  });
});
