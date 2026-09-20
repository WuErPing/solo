import React, { useCallback, useEffect, useMemo, useRef } from "react";
import { generateMermaidShellHtml } from "./mermaid-preview-utils";

interface MermaidPreviewProps {
  source: string;
  isDark?: boolean;
}

interface RenderRequest {
  source: string;
  theme: string;
}

export function MermaidPreview({ source, isDark }: MermaidPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const readyRef = useRef(false);
  const requestRef = useRef<RenderRequest | null>(null);

  // Static shell: independent of source/theme so the iframe is created once and
  // mermaid is fetched from the CDN a single time for this preview's lifetime.
  const shell = useMemo(() => generateMermaidShellHtml(), []);

  const postRender = useCallback(() => {
    const request = requestRef.current;
    const iframe = iframeRef.current;
    if (!request || !iframe) return;
    iframe.contentWindow?.postMessage(
      { type: "mermaid:render", source: request.source, theme: request.theme },
      "*",
    );
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const iframe = document.createElement("iframe");
    iframe.style.width = "100%";
    iframe.style.height = "100%";
    iframe.style.border = "none";
    iframe.setAttribute("data-testid", "mermaid-iframe");

    // Primary trigger: the srcdoc document (and its render-blocking mermaid
    // bundle) is fully loaded, so the iframe's message listener is attached.
    const handleLoad = () => {
      readyRef.current = true;
      postRender();
    };

    // Backup trigger: the shell also announces readiness explicitly. Ignore
    // messages from other previews' iframes via the source check.
    const handleMessage = (event: MessageEvent) => {
      if (event.source !== iframe.contentWindow) return;
      const data = event.data as { type?: string } | null;
      if (data?.type !== "mermaid:ready") return;
      readyRef.current = true;
      postRender();
    };

    iframe.addEventListener("load", handleLoad);
    window.addEventListener("message", handleMessage);
    iframe.srcdoc = shell;
    container.innerHTML = "";
    container.appendChild(iframe);
    iframeRef.current = iframe;

    return () => {
      iframe.removeEventListener("load", handleLoad);
      window.removeEventListener("message", handleMessage);
      container.innerHTML = "";
      iframeRef.current = null;
      readyRef.current = false;
    };
  }, [shell, postRender]);

  useEffect(() => {
    requestRef.current = { source, theme: isDark ? "dark" : "default" };
    if (readyRef.current) {
      postRender();
    }
  }, [source, isDark, postRender]);

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
