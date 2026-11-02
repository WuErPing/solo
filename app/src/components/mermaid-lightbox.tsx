// Fullscreen viewer for a single mermaid diagram, opened from MermaidPreview.
// Keeps the inline preview's raster image when available so the fullscreen
// diagram retains exactly the same colors; vector markup is the fallback.
import { useEffect, useState } from "react";
import type { MermaidSvgEntry } from "./mermaid-svg-cache";
import { renderMermaidVector } from "./mermaid-svg-service";
import { DiagramLightbox } from "./diagram-lightbox";

interface MermaidLightboxProps {
  entry: MermaidSvgEntry;
  source: string;
  themeName: string;
  onClose: () => void;
}

export function MermaidLightbox({ entry, source, themeName, onClose }: MermaidLightboxProps) {
  const [xml, setXml] = useState(entry.xml);

  useEffect(() => {
    if (xml || entry.pngDataUrl) {
      return;
    }
    let alive = true;
    void renderMermaidVector(source, themeName).then((result) => {
      if (alive && result?.xml) {
        setXml(result.xml);
      }
    });
    return () => {
      alive = false;
    };
  }, [entry.pngDataUrl, xml, source, themeName]);

  return (
    <DiagramLightbox
      xml={xml}
      pngDataUrl={entry.pngDataUrl}
      widthPx={entry.widthPx}
      heightPx={entry.heightPx}
      onClose={onClose}
    />
  );
}
