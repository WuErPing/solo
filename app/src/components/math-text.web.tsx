import React, { useEffect, useMemo } from "react";
import { KATEX_CSS, renderKatexHtml } from "./math-text-utils";

interface MathTextProps {
  tex: string;
  display?: boolean;
  isDark?: boolean;
}

const CSS_MARKER = "data-solo-katex";

// Idempotent: the inlined stylesheet (fonts included) is attached to the
// document head at most once, so many formulas share it and no CDN fetch
// is needed before the math can paint.
function ensureKatexCss(): void {
  if (typeof document === "undefined") return;
  if (document.querySelector(`style[${CSS_MARKER}]`)) return;
  const style = document.createElement("style");
  style.setAttribute(CSS_MARKER, "true");
  style.textContent = KATEX_CSS;
  document.head.appendChild(style);
}

const blockStyle: React.CSSProperties = {
  margin: "8px 0",
  overflowX: "auto",
  overflowY: "hidden",
  textAlign: "center",
};

export function MathText({ tex, display = false }: MathTextProps) {
  const html = useMemo(() => renderKatexHtml(tex, display), [tex, display]);

  useEffect(() => {
    ensureKatexCss();
  }, []);

  return React.createElement(display ? "div" : "span", {
    className: "solo-katex",
    "data-testid": "math-text-web",
    ...(display ? { style: blockStyle } : null),
    dangerouslySetInnerHTML: { __html: html },
  });
}
