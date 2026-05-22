import React, { useEffect, useRef, useMemo } from "react";
import { generateMermaidHtml } from "./mermaid-preview-utils";

interface MermaidPreviewProps {
  source: string;
  isDark?: boolean;
}

export function MermaidPreview({ source, isDark }: MermaidPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const html = useMemo(() => generateMermaidHtml(source, isDark), [source, isDark]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

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
  }, [html]);

  return (
    <div
      ref={containerRef}
      style={{
        height: 400,
        marginTop: 8,
        marginBottom: 8,
        borderRadius: 8,
        overflow: "hidden",
        border: "1px solid rgba(128,128,128,0.2)",
      }}
      data-testid="mermaid-preview-web"
    />
  );
}
