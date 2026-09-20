import React, { act } from "react";
import { describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { JSDOM } from "jsdom";
import { MathText } from "./math-text.web";

function setupDom() {
  const dom = new JSDOM("<!doctype html><html><head></head><body></body></html>");
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

function cleanup(root: ReturnType<typeof createRoot>) {
  act(() => {
    root.unmount();
  });
  vi.unstubAllGlobals();
}

describe("MathText (web)", () => {
  it("renders inline math as a span with KaTeX markup", () => {
    const dom = setupDom();
    const { container, root } = mount(dom, <MathText tex="x^2" />);
    const el = container.querySelector('[data-testid="math-text-web"]');
    expect(el).not.toBeNull();
    expect(el?.tagName.toLowerCase()).toBe("span");
    expect(el?.innerHTML).toContain("katex");
    cleanup(root);
  });

  it("renders block math as a div", () => {
    const dom = setupDom();
    const { container, root } = mount(dom, <MathText tex="\\frac{a}{b}" display />);
    const el = container.querySelector('[data-testid="math-text-web"]');
    expect(el?.tagName.toLowerCase()).toBe("div");
    expect(el?.innerHTML).toContain("katex-display");
    cleanup(root);
  });

  it("injects the KaTeX stylesheet exactly once across many formulas", () => {
    const dom = setupDom();
    const { root } = mount(
      dom,
      <div>
        <MathText tex="a" />
        <MathText tex="b" />
        <MathText tex="c" display />
      </div>,
    );
    const styles = dom.window.document.querySelectorAll("style[data-solo-katex]");
    expect(styles.length).toBe(1);
    expect(styles[0].textContent).toContain(".katex");
    cleanup(root);
  });
});
