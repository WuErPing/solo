/**
 * @vitest-environment jsdom
 */
import React from "react";
(globalThis as Record<string, unknown>).React = React;

import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: <T,>(styles: T) => styles,
  },
  useUnistyles: () => ({
    theme: {
      colors: { foregroundMuted: "#888" },
      iconSize: { md: 16, sm: 14 },
      borderWidth: { 1: 1 },
      borderRadius: { full: 9999, lg: 8 },
      spacing: { 1: 4, 2: 8, 3: 12 },
    },
    rt: {},
    breakpoint: undefined,
  }),
  UnistylesRuntime: {
    setTheme: vi.fn(),
    themeName: "light",
  },
}));

vi.mock("react-native-reanimated", () => ({
  FadeIn: { duration: () => ({}) },
  FadeOut: { duration: () => ({}) },
  default: { View: ({ children }: { children: React.ReactNode }) => children },
}));

vi.mock("@gorhom/bottom-sheet", () => ({
  BottomSheetScrollView: ({ children }: { children: React.ReactNode }) => children,
  BottomSheetBackdrop: () => null,
  BottomSheetTextInput: React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
    (props, ref) => React.createElement("input", { ...props, ref }),
  ),
}));

vi.mock("./isolated-bottom-sheet-modal", () => ({
  IsolatedBottomSheetModal: React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
    (props, ref) => React.createElement("div", { ...props, ref }),
  ),
  useIsolatedBottomSheetVisibility: () => ({
    sheetRef: { current: null },
    handleSheetChange: () => {},
  }),
}));

vi.mock("@floating-ui/react-native", () => ({
  flip: () => ({}),
  offset: () => ({}),
  shift: () => ({}),
  size: () => ({}),
  useFloating: () => ({
    refs: { setReference: () => {}, setFloating: () => {} },
    floatingStyles: {},
  }),
}));

vi.mock("lucide-react-native", () => ({
  Check: () => null,
  File: () => null,
  Folder: () => null,
  Search: () => null,
}));

vi.mock("@/constants/layout", () => ({
  useIsCompactFormFactor: () => false,
}));

vi.mock("@/constants/platform", () => ({
  isWeb: true,
}));

import { SearchInput } from "./combobox";

describe("SearchInput", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    if (root) {
      root.unmount();
    }
    root = null;
    container?.remove();
    container = null;
  });

  it("disables browser autocomplete to prevent duplicate suggestion overlays", async () => {
    await new Promise<void>((resolve) => {
      root?.render(
        <SearchInput
          placeholder="Search..."
          value="turn_started"
          onChangeText={() => {}}
        />,
      );
      setTimeout(resolve, 0);
    });

    const input = container?.querySelector("input");
    expect(input).toBeTruthy();
    expect(input?.getAttribute("autocomplete")).toBe("off");
  });
});
