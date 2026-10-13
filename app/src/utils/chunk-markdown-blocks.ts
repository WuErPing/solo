// Merge small non-fence markdown blocks into larger chunks so the preview
// pays the fixed per-<Markdown> parse/render cost fewer times. Fence blocks
// (``` / ~~~) stay isolated: they carry mermaid diagrams or code that must
// not merge with prose, and their component identity must stay stable.

function isFenceBlock(block: string): boolean {
  return /^\s{0,3}(`{3,}|~{3,})/.test(block);
}

export function chunkMarkdownBlocks(blocks: string[], maxChunkChars = 1200): string[] {
  const chunks: string[] = [];
  let pending: string[] = [];
  let pendingChars = 0;

  const flush = () => {
    if (pending.length > 0) {
      chunks.push(pending.join("\n\n"));
      pending = [];
      pendingChars = 0;
    }
  };

  for (const block of blocks) {
    if (isFenceBlock(block)) {
      flush();
      chunks.push(block);
      continue;
    }
    if (pendingChars > 0 && pendingChars + block.length > maxChunkChars) {
      flush();
    }
    pending.push(block);
    pendingChars += block.length;
  }
  flush();
  return chunks;
}
