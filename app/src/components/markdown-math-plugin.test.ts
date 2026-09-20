import { describe, expect, it } from "vitest";
// @ts-expect-error - markdown-it ships no bundled types; it is `any` at this boundary.
import MarkdownIt from "markdown-it";
import { mathPlugin } from "./markdown-math-plugin";

interface MathTokenLike {
  type: string;
  content: string;
}

function collectMath(src: string): MathTokenLike[] {
  const md = MarkdownIt({ typographer: true, linkify: true }).use(mathPlugin);
  const tokens = md.parse(src, {});
  const out: MathTokenLike[] = [];
  const walk = (list: typeof tokens | null) => {
    if (!list) return;
    for (const token of list) {
      if (token.type === "math_inline" || token.type === "math_block") {
        out.push({ type: token.type, content: token.content });
      }
      if (token.children) {
        walk(token.children as unknown as typeof tokens);
      }
    }
  };
  walk(tokens);
  return out;
}

describe("mathPlugin", () => {
  it("tokenizes a single-line block $$...$$", () => {
    const math = collectMath("$$x^2$$");
    expect(math).toEqual([{ type: "math_block", content: "x^2" }]);
  });

  it("tokenizes a multi-line block and trims it", () => {
    const math = collectMath("$$\n\\frac{a}{b}\n$$");
    expect(math).toEqual([{ type: "math_block", content: "\\frac{a}{b}" }]);
  });

  it("tokenizes inline $...$", () => {
    const math = collectMath("energy $E=mc^2$ here");
    expect(math).toEqual([{ type: "math_inline", content: "E=mc^2" }]);
  });

  it("tokenizes multiple inline formulas in one paragraph", () => {
    const math = collectMath("a $x$ b $y$ c");
    expect(math).toEqual([
      { type: "math_inline", content: "x" },
      { type: "math_inline", content: "y" },
    ]);
  });

  it("tokenizes inline math inside a table cell", () => {
    const src = "| sym |\n| --- |\n| $x$ |";
    const math = collectMath(src);
    expect(math).toContainEqual({ type: "math_inline", content: "x" });
  });

  it("does not treat currency as math", () => {
    expect(collectMath("costs $5 and $6 today")).toEqual([]);
  });

  it("preserves raw tex including primes (typographer does not mangle it)", () => {
    const math = collectMath("$f'(net)$");
    expect(math).toEqual([{ type: "math_inline", content: "f'(net)" }]);
  });

  it("keeps escaped \\$ literal", () => {
    expect(collectMath("price \\$5 escaped")).toEqual([]);
  });

  it("handles the backpropagation block example with \\tag", () => {
    const src =
      "$$\\frac{\\partial L}{\\partial w} = \\frac{\\partial L}{\\partial pred} \\cdot \\frac{\\partial pred}{\\partial w} \\tag{链式法则}$$";
    const math = collectMath(src);
    expect(math).toHaveLength(1);
    expect(math[0].type).toBe("math_block");
    expect(math[0].content).toContain("\\tag{链式法则}");
  });
});
