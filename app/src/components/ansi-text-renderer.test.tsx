/**
 * @vitest-environment jsdom
 */
import React from "react";
import { render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("react-native-unistyles", () => ({
  useUnistyles: () => ({
    theme: {
      colors: {
        terminal: {
          background: "#000",
          foreground: "#fff",
          black: "#000",
          red: "#f00",
          green: "#0f0",
          yellow: "#ff0",
          blue: "#00f",
          magenta: "#f0f",
          cyan: "#0ff",
          white: "#fff",
        },
      },
    },
  }),
  StyleSheet: { create: (s: unknown) => s },
}));

vi.mock("@/utils/ansi-color-palette", () => ({
  resolveColor: (c: string | null) => c,
}));

import { AnsiTextContent } from "./ansi-text-renderer";
import type { AnsiSegment } from "@/utils/ansi-parser";

const stableSegments: AnsiSegment[] = [{ text: "hello world", style: {} }];
const stableTerminalColors = {
  background: "#000",
  foreground: "#fff",
  black: "#000",
  red: "#f00",
  green: "#0f0",
  yellow: "#ff0",
  blue: "#00f",
  magenta: "#f0f",
  cyan: "#0ff",
  white: "#fff",
};

describe("AnsiTextContent", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("is wrapped in React.memo so stable props do not cause re-renders", () => {
    // TDD contract for "no jitter when content unchanged":
    // React.memo guarantees that when parent re-renders with the same
    // segments/terminalColors references, AnsiTextContent is skipped and
    // the Android Text tree is not reconciled.
    //
    // React.memo returns a component whose $$typeof is REACT_MEMO_TYPE.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const anyComp = AnsiTextContent as any;
    expect(typeof anyComp).toBe("object");
    // Detect REACT_MEMO_TYPE symbol (description is "react.memo" in React 18/19)
    const sym = anyComp.$$typeof;
    expect(typeof sym).toBe("symbol");
    expect(String(sym)).toContain("react.memo");
    // Inner render function
    expect(typeof anyComp.type).toBe("function");
    expect(anyComp.type.name).toMatch(/^AnsiTextContent/);
  });

  it("renders segments as Text children", () => {
    const { container } = render(
      <AnsiTextContent segments={stableSegments} terminalColors={stableTerminalColors} />,
    );
    expect(container.textContent).toContain("hello world");
  });

  it("re-renders when segments reference changes", () => {
    const { container, rerender } = render(
      <AnsiTextContent segments={stableSegments} terminalColors={stableTerminalColors} />,
    );
    expect(container.textContent).toContain("hello world");

    const newSegments: AnsiSegment[] = [{ text: "changed", style: {} }];
    rerender(<AnsiTextContent segments={newSegments} terminalColors={stableTerminalColors} />);
    expect(container.textContent).toContain("changed");
  });
});
