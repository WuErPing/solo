/**
 * @vitest-environment jsdom
 */
import React from "react";
import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AnsiTextLine } from "./ansi-text-line";
import type { AnsiSegment, AnsiStyle } from "@/utils/ansi-parser";

vi.mock("react-native-unistyles", () => ({
  useUnistyles: () => ({ theme: { colors: { terminal: { foreground: "#fff", background: "#000" } } } }),
}));

vi.mock("@/utils/ansi-color-palette", () => ({
  resolveColor: (color: unknown, _terminal: unknown, fallback: string | null) => {
    if (color && typeof color === "object" && "index" in color) return `color-${(color as { index: number }).index}`;
    return fallback;
  },
}));

const defaultStyle: AnsiStyle = {
  bold: false, dim: false, italic: false, underline: false,
  strikethrough: false, inverse: false, fg: null, bg: null,
};

const terminalColors = { foreground: "#ffffff", background: "#000000" };

describe("AnsiTextLine", () => {
  it("renders segments as Text children", () => {
    const segments: AnsiSegment[] = [
      { text: "hello ", style: defaultStyle },
      { text: "world", style: { ...defaultStyle, bold: true } },
    ];
    const { container } = render(
      <AnsiTextLine segments={segments} terminalColors={terminalColors} />,
    );
    expect(container.textContent).toBe("hello world");
  });

  it("is wrapped in React.memo", () => {
    expect(AnsiTextLine.$$typeof).toBe(Symbol.for("react.memo"));
  });
});
