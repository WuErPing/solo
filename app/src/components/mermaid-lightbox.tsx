// Fullscreen viewer for a single mermaid diagram, opened from MermaidPreview.
// Fetches vector SVG on demand (renderMermaidVector) when the cache only holds
// the rasterized PNG so pinch zoom stays sharp at any scale; falls back to the
// PNG when vector data is unavailable.
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
    if (xml) {
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
  }, [xml, source, themeName]);

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
