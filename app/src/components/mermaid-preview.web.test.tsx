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

function mount(dom: JSDOM, element: React.ReactElement) {
  const container = dom.window.document.createElement("div");
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  act(() => {
    root.render(element);
  });
  return { container, root };
}

describe("MermaidPreview (web)", () => {
  it("renders a single iframe with the static mermaid shell", () => {
    const dom = setupDom();
    const { container, root } = mount(dom, <MermaidPreview source="graph TD; A -- B;" />);

    const wrapper = container.querySelector('[data-testid="mermaid-preview-web"]');
    expect(wrapper).not.toBeNull();

    const iframe = wrapper?.querySelector("iframe");
    expect(iframe).not.toBeNull();

    const srcdoc = iframe?.getAttribute("srcdoc") ?? "";
    expect(srcdoc).toContain("<!DOCTYPE html>");
    expect(srcdoc).toContain("mermaid.min.js");
    expect(srcdoc).toContain("__renderMermaid");
    // Source is pushed via postMessage, never embedded in the document.
    expect(srcdoc).not.toContain("graph TD; A -- B;");

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("reuses the same iframe instance when the source changes", () => {
    const dom = setupDom();
    const { container, root } = mount(dom, <MermaidPreview source="graph TD; A -- B;" />);

    const first = container.querySelector("iframe");
    expect(first).not.toBeNull();

    act(() => {
      root.render(<MermaidPreview source="graph TD; C -- D;" />);
    });

    const second = container.querySelector("iframe");
    // Core performance guard: editing the diagram must not tear down and
    // recreate the iframe (which would re-fetch mermaid from the CDN).
    expect(second).toBe(first);

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("pushes source and theme to the iframe via postMessage once ready", () => {
    const dom = setupDom();
    const { container, root } = mount(dom, <MermaidPreview source="graph TD; A -- B;" />);

    const iframe = container.querySelector("iframe") as HTMLIFrameElement;
    const postMessage = vi.spyOn(
      iframe.contentWindow as unknown as { postMessage: (data: unknown, origin: string) => void },
      "postMessage",
    );

    act(() => {
      dom.window.dispatchEvent(
        new dom.window.MessageEvent("message", {
          data: { type: "mermaid:ready" },
          source: iframe.contentWindow,
        }),
      );
    });

    act(() => {
      root.render(<MermaidPreview source="graph TD; C -- D;" isDark />);
    });

    expect(postMessage).toHaveBeenCalledWith(
      { type: "mermaid:render", source: "graph TD; C -- D;", theme: "dark" },
      "*",
    );

    act(() => {
      root.unmount();
    });
    vi.unstubAllGlobals();
  });

  it("cleans up iframe on unmount", () => {
    const dom = setupDom();
    const { container, root } = mount(dom, <MermaidPreview source="graph TD; A -- B;" />);

    expect(container.querySelector("iframe")).not.toBeNull();

    act(() => {
      root.unmount();
    });

    expect(container.querySelector("iframe")).toBeNull();
    vi.unstubAllGlobals();
  });
});
