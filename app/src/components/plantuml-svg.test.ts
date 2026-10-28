import { describe, expect, it } from "vitest";
import { parsePlantUmlSvg } from "./plantuml-svg";

describe("parsePlantUmlSvg", () => {
  it("parses PlantUML output with px sizes and a viewBox", () => {
    const raw =
      '<?plantuml 1.2026.2?><svg xmlns="http://www.w3.org/2000/svg" height="150px" style="width:108px;height:150px;background:#FFFFFF;" viewBox="0 0 108 150" width="108px"><g><text>Alice</text></g></svg>';
    const parsed = parsePlantUmlSvg(raw);
    expect(parsed).not.toBeNull();
    expect(parsed!.widthPx).toBe(108);
    expect(parsed!.heightPx).toBe(150);
    expect(parsed!.xml.startsWith('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 108 150">')).toBe(
      true,
    );
    // Inner markup survives; server background style does not leak into the root.
    expect(parsed!.xml).toContain("<g><text>Alice</text></g>");
    expect(parsed!.xml).not.toContain("background:#FFFFFF");
    expect(parsed!.xml).not.toContain("<?plantuml");
  });

  it("falls back to the viewBox when width/height are missing", () => {
    const parsed = parsePlantUmlSvg(
      '<svg viewBox="0 0 320 200"><g/></svg>',
    );
    expect(parsed).not.toBeNull();
    expect(parsed!.widthPx).toBe(320);
    expect(parsed!.heightPx).toBe(200);
  });

  it("synthesizes a viewBox when only width/height are present", () => {
    const parsed = parsePlantUmlSvg('<svg width="40px" height="30px"><g/></svg>');
    expect(parsed).not.toBeNull();
    expect(parsed!.xml).toContain('viewBox="0 0 40 30"');
  });

  it("strips processing instructions and scripts", () => {
    const parsed = parsePlantUmlSvg(
      '<?plantuml-src "A -> B"?><?plantuml 1.2026.2?><svg width="10px" height="10px"><script>alert(1)</script><g/></svg>',
    );
    expect(parsed).not.toBeNull();
    expect(parsed!.xml).not.toContain("<script");
    expect(parsed!.xml).not.toContain("<?");
    expect(parsed!.xml).toContain("<g/>");
  });

  it("returns null for malformed or non-SVG input", () => {
    expect(parsePlantUmlSvg("")).toBeNull();
    expect(parsePlantUmlSvg("not markup")).toBeNull();
    expect(parsePlantUmlSvg('<svg width="10px" height="10px"')).toBeNull();
    expect(parsePlantUmlSvg('<svg width="0px" height="0px"></svg>')).toBeNull();
  });
});
