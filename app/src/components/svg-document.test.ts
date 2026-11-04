import { describe, expect, it } from "vitest";
import { parseSvgDocument } from "./svg-document";

const HAND_AUTHORED = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 830" font-family="PingFang SC, sans-serif">
  <defs><marker id="arrow" markerWidth="10"><polygon points="0 0, 10 3.5" fill="#2563eb"/></marker></defs>
  <rect width="960" height="830" fill="#ffffff"/>
  <text x="40" y="30" font-size="17">Title</text>
</svg>`;

describe("parseSvgDocument", () => {
  it("derives the natural size from the viewBox and keeps root attributes", () => {
    const parsed = parseSvgDocument(HAND_AUTHORED);
    expect(parsed?.widthPx).toBe(960);
    expect(parsed?.heightPx).toBe(830);
    expect(parsed?.xml).toContain('font-family="PingFang SC, sans-serif"');
    expect(parsed?.xml).toContain('viewBox="0 0 960 830"');
    expect(parsed?.xml).toContain("<marker");
  });

  it("prefers explicit pixel dimensions over the viewBox", () => {
    const parsed = parseSvgDocument(
      '<svg xmlns="http://www.w3.org/2000/svg" width="120px" height="40px" viewBox="0 0 960 830"><rect/></svg>',
    );
    expect(parsed?.widthPx).toBe(120);
    expect(parsed?.heightPx).toBe(40);
    // The viewBox is preserved so the drawing still scales to the new size.
    expect(parsed?.xml).toContain('viewBox="0 0 960 830"');
  });

  it("drops width, height and style so the caller controls sizing", () => {
    const parsed = parseSvgDocument(
      '<svg xmlns="http://www.w3.org/2000/svg" width="960" height="830" style="max-width:100%"><rect/></svg>',
    );
    expect(parsed?.xml).not.toContain("width=");
    expect(parsed?.xml).not.toContain("height=");
    expect(parsed?.xml).not.toContain("style=");
    expect(parsed?.xml).toContain('viewBox="0 0 960 830"');
  });

  it("synthesizes a viewBox when only pixel dimensions are present", () => {
    const parsed = parseSvgDocument(
      '<svg xmlns="http://www.w3.org/2000/svg" width="300" height="150"><rect/></svg>',
    );
    expect(parsed?.xml).toContain('viewBox="0 0 300 150"');
  });

  it("strips processing instructions and scripts", () => {
    const parsed = parseSvgDocument(
      '<?xml version="1.0"?>\n<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><script>alert(1)</script><rect/></svg>',
    );
    expect(parsed?.xml).not.toContain("<?xml");
    expect(parsed?.xml).not.toContain("alert(1)");
    expect(parsed?.xml).toContain("<rect/>");
  });

  it("returns null for non-SVG markup", () => {
    expect(parseSvgDocument("<html><body>hi</body></html>")).toBeNull();
    expect(parseSvgDocument("")).toBeNull();
  });

  it("returns null when the natural size cannot be measured", () => {
    expect(
      parseSvgDocument('<svg xmlns="http://www.w3.org/2000/svg" width="100%"><rect/></svg>'),
    ).toBeNull();
  });
});
