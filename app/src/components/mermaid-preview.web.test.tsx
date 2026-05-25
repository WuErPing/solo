import React, { act } from "react";
import { describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { JSDOM } from "jsdom";
import { MermaidPreview } from "./mermaid-preview.web";

function setupDom() {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal("window", dom.window);
  vi.stubGlobal("document", dom.window.document);
  return dom;
}

describe("MermaidPreview (web)", () => {
  it("renders an iframe with mermaid HTML srcdoc", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<MermaidPreview source="graph TD; A -- B;" />);
    });

    const wrapper = container.querySelector('[data-testid="mermaid-preview-web"]');
    expect(wrapper).not.toBeNull();

    const iframe = wrapper?.querySelector("iframe");
    expect(iframe).not.toBeNull();

    const srcdoc = iframe?.getAttribute("srcdoc");
    expect(srcdoc).toContain("graph TD; A -- B;");
    expect(srcdoc).toContain("mermaid.min.js");
    expect(srcdoc).toContain("<!DOCTYPE html>");

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("uses dark theme when isDark is true", () => {
    const dom = setupDom();

    const container = dom.window.document.createElement("div");
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<MermaidPreview source="graph TD; A -- B;" isDark />);
    });

    const iframe = container.querySelector("iframe");
    const srcdoc = iframe?.getAttribute("srcdoc") ?? "";
    expect(srcdoc).toContain("theme: 'dark'");
    expect(srcdoc).toContain("#0d1117");

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

    act(() => {
      root.render(<MermaidPreview source="graph TD; A -- B;" />);
    });

    expect(container.querySelector("iframe")).not.toBeNull();

    act(() => {
      root.unmount();
    });

    expect(container.querySelector("iframe")).toBeNull();
    vi.unstubAllGlobals();
  });
});
