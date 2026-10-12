import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ScrollView as RNScrollView } from "react-native";
import { FilePreviewScrollView } from "./file-preview-scroll-view";

const { theme } = vi.hoisted(() => ({
  theme: {
    spacing: { 2: 8, 3: 12, 4: 16 },
    iconSize: { lg: 20 },
    colors: { surface2: "#eee", foregroundMuted: "#999" },
  },
}));

const scrollViewCapture = vi.hoisted(() => ({
  props: null as {
    onLayout?: (event: unknown) => void;
    onScroll?: (event: unknown) => void;
    onContentSizeChange?: (width: number, height: number) => void;
  } | null,
  scrollTo: vi.fn(),
}));

vi.mock("react-native", () => ({
  View: ({ children, testID }: { children?: React.ReactNode; testID?: string }) =>
    React.createElement("div", { "data-testid": testID }, children),
  Pressable: ({
    accessibilityLabel,
    accessibilityState,
    children,
    disabled,
    onPress,
    testID,
  }: {
    accessibilityLabel?: string;
    accessibilityState?: { disabled?: boolean };
    children?: React.ReactNode;
    disabled?: boolean;
    onPress?: () => void;
    testID?: string;
  }) =>
    React.createElement(
      "button",
      {
        "aria-disabled": accessibilityState?.disabled ?? disabled ?? false,
        "aria-label": accessibilityLabel,
        "data-testid": testID,
        disabled: disabled ?? false,
        onClick: () => onPress?.(),
        type: "button",
      },
      children,
    ),
  ScrollView: React.forwardRef(function ScrollViewMock(
    { children, ...props }: { children?: React.ReactNode } & Record<string, unknown>,
    ref: React.Ref<unknown>,
  ) {
    scrollViewCapture.props = props as unknown as typeof scrollViewCapture.props;
    React.useImperativeHandle(ref, () => ({ scrollTo: scrollViewCapture.scrollTo }));
    return React.createElement("div", { "data-testid": "preview-scrollview" }, children);
  }),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("lucide-react-native", () => ({
  ChevronUp: () => React.createElement("span", { "data-icon": "chevron-up" }),
  ChevronDown: () => React.createElement("span", { "data-icon": "chevron-down" }),
}));

describe("FilePreviewScrollView", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    const dom = new JSDOM("<!doctype html><html><body></body></html>");
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    vi.stubGlobal("window", dom.window);
    vi.stubGlobal("document", dom.window.document);
    vi.stubGlobal("HTMLElement", dom.window.HTMLElement);
    vi.stubGlobal("Node", dom.window.Node);
    vi.stubGlobal("navigator", dom.window.navigator);

    scrollViewCapture.props = null;
    scrollViewCapture.scrollTo.mockReset();

    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    if (root) {
      act(() => {
        root?.unmount();
      });
    }
    root = null;
    container = null;
    vi.unstubAllGlobals();
  });

  function createScrollbar() {
    return {
      onScroll: vi.fn(),
      onLayout: vi.fn(),
      onContentSizeChange: vi.fn(),
      overlay: null,
    };
  }

  function renderPreview(options?: { pagerEnabled?: boolean }) {
    const scrollbar = createScrollbar();
    const scrollRef = React.createRef<RNScrollView>();
    act(() => {
      root?.render(
        <FilePreviewScrollView
          scrollRef={scrollRef}
          scrollbar={scrollbar}
          pagerEnabled={options?.pagerEnabled ?? true}
          showsVerticalScrollIndicator
        >
          <span data-testid="preview-content">markdown</span>
        </FilePreviewScrollView>,
      );
    });
    return { scrollbar, scrollRef };
  }

  function fireLayout(height: number) {
    act(() => {
      scrollViewCapture.props?.onLayout?.({ nativeEvent: { layout: { height } } });
    });
  }

  function fireContentSizeChange(height: number) {
    act(() => {
      scrollViewCapture.props?.onContentSizeChange?.(800, height);
    });
  }

  function fireScroll(offset: number, viewportHeight: number, contentHeight: number) {
    act(() => {
      scrollViewCapture.props?.onScroll?.({
        nativeEvent: {
          contentOffset: { y: offset },
          contentSize: { height: contentHeight },
          layoutMeasurement: { height: viewportHeight },
        },
      });
    });
  }

  function getButton(testId: string): HTMLButtonElement {
    const button = container?.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement | null;
    if (!button) {
      throw new Error(`Expected ${testId} to render`);
    }
    return button;
  }

  function click(testId: string) {
    act(() => {
      getButton(testId).dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });
  }

  it("renders children inside the scroll view", () => {
    renderPreview();

    expect(container?.querySelector('[data-testid="preview-content"]')).not.toBeNull();
  });

  it("hides the pager buttons when the pager is disabled", () => {
    renderPreview({ pagerEnabled: false });

    expect(container?.querySelector('[data-testid="file-preview-pager"]')).toBeNull();
  });

  it("keeps both pager buttons disabled before layout metrics arrive", () => {
    renderPreview();

    expect(getButton("file-preview-pager-up").getAttribute("aria-disabled")).toBe("true");
    expect(getButton("file-preview-pager-down").getAttribute("aria-disabled")).toBe("true");
  });

  it("enables page down once metrics show overflow content", () => {
    renderPreview();

    fireLayout(600);
    fireContentSizeChange(2000);

    expect(getButton("file-preview-pager-up").getAttribute("aria-disabled")).toBe("true");
    expect(getButton("file-preview-pager-down").getAttribute("aria-disabled")).toBe("false");
  });

  it("pages down by 90% of the viewport with an animated scroll", () => {
    renderPreview();

    fireLayout(600);
    fireContentSizeChange(2000);
    click("file-preview-pager-down");

    expect(scrollViewCapture.scrollTo).toHaveBeenCalledWith({ y: 540, animated: true });
  });

  it("pages up from the bottom after scrolling", () => {
    renderPreview();

    fireLayout(600);
    fireContentSizeChange(2000);
    fireScroll(1400, 600, 2000);

    expect(getButton("file-preview-pager-up").getAttribute("aria-disabled")).toBe("false");
    expect(getButton("file-preview-pager-down").getAttribute("aria-disabled")).toBe("true");

    click("file-preview-pager-up");

    expect(scrollViewCapture.scrollTo).toHaveBeenCalledWith({ y: 860, animated: true });
  });

  it("forwards scroll events to the desktop scrollbar handlers", () => {
    const { scrollbar } = renderPreview();

    fireScroll(100, 600, 2000);

    expect(scrollbar.onScroll).toHaveBeenCalledTimes(1);
  });

  it("does not track pager metrics while the pager is disabled", () => {
    const { scrollbar } = renderPreview({ pagerEnabled: false });

    fireLayout(600);
    fireContentSizeChange(2000);
    fireScroll(100, 600, 2000);

    expect(scrollbar.onScroll).toHaveBeenCalledTimes(1);
    expect(container?.querySelector('[data-testid="file-preview-pager"]')).toBeNull();
  });
});
