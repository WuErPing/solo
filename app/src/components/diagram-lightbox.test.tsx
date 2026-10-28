import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DiagramLightbox } from "./diagram-lightbox";

const { theme } = vi.hoisted(() => ({
  theme: {
    spacing: { 1: 4, 2: 8, 3: 12, 4: 16 },
    borderWidth: { 1: 1 },
    colors: {
      background: "#ffffff",
      surface2: "#222",
      foregroundMuted: "#aaa",
      border: "#555",
    },
  },
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => ({ top: 0, right: 0, bottom: 0, left: 0 }),
}));

vi.mock("lucide-react-native", () => {
  const createIcon = (name: string) => (props: Record<string, unknown>) =>
    React.createElement("span", { ...props, "data-icon": name });
  return { X: createIcon("X") };
});

vi.mock("react-native-svg", () => ({
  SvgXml: (props: { xml: string }) =>
    React.createElement("div", { "data-testid": "svg-xml", "data-xml": props.xml }),
}));

// The zoom container pulls in reanimated + gesture-handler; the shell only
// needs to know that its content is mounted inside it.
vi.mock("./mermaid-zoom-view", () => ({
  MermaidZoomView: ({ children }: { children?: React.ReactNode }) =>
    React.createElement("div", { "data-testid": "zoom-view" }, children),
}));

vi.mock("react-native", async () => {
  const actual = await vi.importActual<Record<string, unknown>>("react-native");
  const Modal = ({ visible = true, children }: { visible?: boolean; children?: React.ReactNode }) =>
    visible ? React.createElement("div", { "data-testid": "lightbox-modal" }, children) : null;
  const View = (props: { testID?: string; style?: unknown; children?: React.ReactNode }) =>
    React.createElement(
      "div",
      { "data-testid": props.testID, "data-style": JSON.stringify(props.style ?? null) },
      props.children,
    );
  const Image = (props: { testID?: string }) =>
    React.createElement("div", { "data-testid": props.testID });
  return { ...actual, Image, Modal, View };
});

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

function render(element: React.ReactElement) {
  act(() => {
    root?.render(element);
  });
}

function queryByTestId(testID: string): HTMLElement | null {
  return document.querySelector(`[data-testid="${testID}"]`);
}

function backgroundColorOf(element: HTMLElement): string | undefined {
  const parsed = JSON.parse(element.getAttribute("data-style") ?? "null") as unknown;
  const parts = Array.isArray(parsed) ? parsed.flat(Infinity) : [parsed];
  for (const part of parts) {
    const color = (part as { backgroundColor?: string } | null)?.backgroundColor;
    if (color) {
      return color;
    }
  }
  return undefined;
}

describe("DiagramLightbox", () => {
  it("paints vector diagrams on the app canvas color instead of the dark backdrop", () => {
    render(
      <DiagramLightbox xml="<svg></svg>" widthPx={400} heightPx={300} onClose={vi.fn()} />,
    );

    const surface = queryByTestId("diagram-lightbox-surface");
    expect(surface).not.toBeNull();
    expect(queryByTestId("zoom-view")?.contains(surface!)).toBe(true);
    expect(backgroundColorOf(surface!)).toBe(theme.colors.background);
    expect(queryByTestId("svg-xml")?.getAttribute("data-xml")).toBe("<svg></svg>");
  });

  it("falls back to the rasterized image when no vector markup is available", () => {
    render(
      <DiagramLightbox
        pngDataUrl="data:image/png;base64,AAAA"
        widthPx={400}
        heightPx={300}
        onClose={vi.fn()}
      />,
    );

    expect(queryByTestId("diagram-lightbox-image")).not.toBeNull();
    expect(queryByTestId("svg-xml")).toBeNull();
  });

  it("closes from the close button", () => {
    const onClose = vi.fn();
    render(
      <DiagramLightbox xml="<svg></svg>" widthPx={400} heightPx={300} onClose={onClose} />,
    );

    const closeButton = document.querySelector('[aria-label="Close diagram"]');
    expect(closeButton).not.toBeNull();
    act(() => {
      closeButton!.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
