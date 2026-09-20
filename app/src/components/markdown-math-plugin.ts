/**
 * Minimal markdown-it plugin that tokenizes LaTeX math into `math_inline`
 * (`$...$`) and `math_block` (`$$...$$`) tokens, preserving the raw TeX in
 * `token.content`. It only produces tokens — rendering is handled by the
 * react-native-markdown-display render rules (see MathText), so this stays
 * platform-agnostic and unit-testable.
 *
 * Registering the inline rule after `escape` keeps `\$` literal, and capturing
 * raw TeX as a single token keeps markdown-it's `typographer` smart-quote pass
 * from rewriting things like `f'(net)`.
 */

interface MathToken {
  markup: string;
  content: string;
  block?: boolean;
  map?: [number, number] | null;
}

interface InlineState {
  src: string;
  pos: number;
  posMax: number;
  push: (type: string, tag: string, nesting: number) => MathToken;
}

interface BlockState {
  src: string;
  bMarks: number[];
  eMarks: number[];
  tShift: number[];
  sCount: number[];
  blkIndent: number;
  line: number;
  push: (type: string, tag: string, nesting: number) => MathToken;
}

interface MathMarkdownIt {
  inline: { ruler: { after: (name: string, ruleName: string, fn: unknown) => void } };
  block: {
    ruler: {
      before: (name: string, ruleName: string, fn: unknown, options?: unknown) => void;
    };
  };
}

const DOLLAR = 0x24; // $
const BACKSLASH = 0x5c; // \

function isSpace(code: number): boolean {
  return code === 0x20 || code === 0x09;
}

function isDigit(code: number): boolean {
  return code >= 0x30 && code <= 0x39;
}

/**
 * Inline `$...$`. Guards against currency false positives: the opening `$` must
 * not be followed by whitespace, and the closing `$` must not be preceded by
 * whitespace nor followed by a digit (so `$5 and $6` stays literal text).
 */
function mathInline(state: InlineState, silent: boolean): boolean {
  const start = state.pos;
  if (state.src.charCodeAt(start) !== DOLLAR) {
    return false;
  }
  // `$$` is the block marker; leave it to the block rule.
  if (state.src.charCodeAt(start + 1) === DOLLAR) {
    return false;
  }

  const max = state.posMax;
  let pos = start + 1;
  if (pos >= max || isSpace(state.src.charCodeAt(pos))) {
    return false;
  }

  let end = -1;
  while (pos < max) {
    const code = state.src.charCodeAt(pos);
    if (code === BACKSLASH) {
      pos += 2;
      continue;
    }
    if (code === DOLLAR) {
      const prev = state.src.charCodeAt(pos - 1);
      const next = state.src.charCodeAt(pos + 1);
      if (!isSpace(prev) && !isDigit(next)) {
        end = pos;
        break;
      }
    }
    pos += 1;
  }

  if (end < 0) {
    return false;
  }

  const content = state.src.slice(start + 1, end);
  if (content.length === 0) {
    return false;
  }

  if (!silent) {
    const token = state.push("math_inline", "math", 0);
    token.markup = "$";
    token.content = content;
  }

  state.pos = end + 1;
  return true;
}

/**
 * Block `$$...$$`, supporting both a single line (`$$ x^2 $$`) and a fenced
 * multi-line block. An unclosed block auto-closes at the end of the document.
 */
function mathBlock(
  state: BlockState,
  startLine: number,
  endLine: number,
  silent: boolean,
): boolean {
  const startPos = state.bMarks[startLine] + state.tShift[startLine];
  const max = state.eMarks[startLine];
  if (startPos + 2 > max) {
    return false;
  }
  if (state.src.slice(startPos, startPos + 2) !== "$$") {
    return false;
  }
  if (silent) {
    return true;
  }

  let content: string;
  let lastLine = startLine;

  const firstLine = state.src.slice(startPos + 2, max);
  const trimmedFirst = firstLine.trim();

  if (trimmedFirst.length >= 2 && trimmedFirst.endsWith("$$")) {
    // Single-line block: $$ ... $$
    content = trimmedFirst.slice(0, -2).trim();
  } else {
    const lines: string[] = [];
    if (trimmedFirst.length > 0) {
      lines.push(firstLine);
    }
    lastLine = startLine;
    for (;;) {
      lastLine += 1;
      if (lastLine >= endLine) {
        // Unclosed: auto-close at end of document.
        lastLine = endLine - 1;
        break;
      }
      const s = state.bMarks[lastLine] + state.tShift[lastLine];
      const e = state.eMarks[lastLine];
      const line = state.src.slice(s, e);
      const trimmed = line.trim();
      if (trimmed.startsWith("$$")) {
        break;
      }
      lines.push(line);
    }
    content = lines.join("\n").trim();
  }

  const token = state.push("math_block", "math", 0);
  token.block = true;
  token.markup = "$$";
  token.content = content;
  token.map = [startLine, lastLine + 1];
  state.line = lastLine + 1;
  return true;
}

export function mathPlugin(md: MathMarkdownIt): void {
  md.inline.ruler.after("escape", "math_inline", mathInline);
  md.block.ruler.before("fence", "math_block", mathBlock, {
    alt: ["paragraph", "blockquote", "list"],
  });
}
