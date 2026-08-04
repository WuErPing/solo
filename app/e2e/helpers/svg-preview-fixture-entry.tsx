// Fixture entry for svg-preview.spec.ts. Bundled with esbuild at test time
// (see the spec's beforeAll) so the component tests run fully offline against
// the repo's local react / react-dom instead of an esm.sh CDN import.
import React from "react";
import { createRoot } from "react-dom/client";
import { SvgPreview } from "../../src/components/svg-preview.web";

declare global {
  interface Window {
    __renderSvgPreview?: (source: string) => void;
  }
}

window.__renderSvgPreview = (source: string) => {
  const container = document.getElementById("root");
  if (!container) {
    throw new Error("#root container missing from fixture page");
  }
  createRoot(container).render(React.createElement(SvgPreview, { source }));
};
