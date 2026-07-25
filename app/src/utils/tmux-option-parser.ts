export interface TmuxContextOption {
  digit: string;
  label: string;
}

const MAX_LABEL_LENGTH = 12;

export function stripAnsi(input: string): string {
  return input
    .replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "")
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, "")
    .replace(/\x1b[()][0-9A-B]/g, "")
    .replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, "");
}

const OPTION_RE = /^\s*[（(]?\s*([1-4])\s*[)）.：:]\s*(.+)$/;
const OPTION_RE_ALT = /^\s*([1-4])\.\s+(.+)$/;

export function parseContextOptions(
  rawContent: string,
  opts?: { tailLines?: number },
): TmuxContextOption[] {
  const tailLines = opts?.tailLines ?? 10;
  const plain = stripAnsi(rawContent);
  const lines = plain.split("\n").slice(-tailLines);

  const found: TmuxContextOption[] = [];
  const seenDigits = new Set<string>();

  for (const line of lines) {
    const match = OPTION_RE.exec(line) ?? OPTION_RE_ALT.exec(line);
    if (!match) continue;
    const digit = match[1]!;
    if (seenDigits.has(digit)) continue;
    seenDigits.add(digit);

    const rawLabel = match[2]!.trim();
    const label =
      rawLabel.length > MAX_LABEL_LENGTH ? rawLabel.slice(0, MAX_LABEL_LENGTH) + "…" : rawLabel;
    found.push({ digit, label });
  }

  if (found.length < 2) return [];
  found.sort((a, b) => a.digit.localeCompare(b.digit));
  if (found[0]!.digit !== "1") return [];

  return found;
}
