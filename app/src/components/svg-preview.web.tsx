import React, { useEffect, useRef, useMemo } from "react";
import { isSvgContent, generateSvgHtml } from "./svg-preview-utils";

interface SvgPreviewProps {
  source: string;
  isDark?: boolean;
}

export function SvgPreview({ source, isDark = false }: SvgPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const isValid = useMemo(() => isSvgContent(source), [source]);
  const html = useMemo(() => (isValid ? generateSvgHtml(source, isDark) : ""), [source, isDark, isValid]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    if (!isValid) {
      container.innerHTML = '<div data-testid="svg-preview-error" style="padding:16px;color:red;text-align:center;">Invalid SVG content</div>';
      return;
    }

    const iframe = document.createElement("iframe");
    iframe.style.width = "100%";
    iframe.style.height = "100%";
    iframe.style.border = "none";
    iframe.srcdoc = html;

    container.innerHTML = "";
    container.appendChild(iframe);

    return () => {
      container.innerHTML = "";
    };
  }, [html, isValid]);

  return (
    <div
      ref={containerRef}
      data-testid="svg-preview"
      style={{
        height: 400,
        marginTop: 8,
        marginBottom: 8,
        borderRadius: 8,
        overflow: "hidden",
        border: "1px solid rgba(128,128,128,0.2)",
      }}
    />
  );
}
