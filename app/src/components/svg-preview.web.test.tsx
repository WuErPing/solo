import React, { act } from "react";
import { describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { JSDOM } from "jsdom";
import { SvgPreview } from "./svg-preview.web";

function setupDom() {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal("window", dom.window);
  vi.stubGlobal("document", dom.window.document);
  return dom;
}

describe("SvgPreview (web)", () => {
  it("renders an iframe with SVG HTML srcdoc", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    const svgContent = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';

    act(() => {
      root.render(<SvgPreview source={svgContent} />);
    });

    const wrapper = container.querySelector('[data-testid="svg-preview"]');
    expect(wrapper).not.toBeNull();

    const iframe = wrapper?.querySelector("iframe");
    expect(iframe).not.toBeNull();

    const srcdoc = iframe?.getAttribute("srcdoc");
    expect(srcdoc).toContain("<svg");
    expect(srcdoc).toContain("circle");
    expect(srcdoc).toContain("<!DOCTYPE html>");

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("sanitizes SVG content before rendering", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    const maliciousSvg = '<svg xmlns="http://www.w3.org/2000/svg"><script>alert("xss")</script><circle/></svg>';

    act(() => {
      root.render(<SvgPreview source={maliciousSvg} />);
    });

    const iframe = container.querySelector("iframe");
    const srcdoc = iframe?.getAttribute("srcdoc") ?? "";
    expect(srcdoc).not.toContain("<script>");
    expect(srcdoc).toContain("circle");

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("applies dark theme when isDark is true", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    const svgContent = '<svg xmlns="http://www.w3.org/2000/svg"><circle/></svg>';

    act(() => {
      root.render(<SvgPreview source={svgContent} isDark />);
    });

    const iframe = container.querySelector("iframe");
    const srcdoc = iframe?.getAttribute("srcdoc") ?? "";
    expect(srcdoc).toContain("#0d1117");

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("applies light theme by default", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    const svgContent = '<svg xmlns="http://www.w3.org/2000/svg"><circle/></svg>';

    act(() => {
      root.render(<SvgPreview source={svgContent} />);
    });

    const iframe = container.querySelector("iframe");
    const srcdoc = iframe?.getAttribute("srcdoc") ?? "";
    expect(srcdoc).toContain("#ffffff");

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("cleans up iframe on unmount", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    const svgContent = '<svg xmlns="http://www.w3.org/2000/svg"><circle/></svg>';

    act(() => {
      root.render(<SvgPreview source={svgContent} />);
    });

    expect(container.querySelector("iframe")).not.toBeNull();

    act(() => {
      root.unmount();
    });

    expect(container.querySelector("iframe")).toBeNull();
    vi.unstubAllGlobals();
  });

  it("renders error for empty source", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<SvgPreview source="" />);
    });

    const wrapper = container.querySelector('[data-testid="svg-preview"]');
    expect(wrapper).not.toBeNull();

    const errorDiv = container.querySelector('[data-testid="svg-preview-error"]');
    expect(errorDiv).not.toBeNull();

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("renders error for invalid SVG", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<SvgPreview source="not valid svg" />);
    });

    const wrapper = container.querySelector('[data-testid="svg-preview"]');
    expect(wrapper).not.toBeNull();

    const errorDiv = container.querySelector('[data-testid="svg-preview-error"]');
    expect(errorDiv).not.toBeNull();

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });
});
