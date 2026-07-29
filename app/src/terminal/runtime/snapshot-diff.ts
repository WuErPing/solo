export interface SnapshotDiffResult {
  patch: string | null;
  fullRewrite: boolean;
}

const FULL_REWRITE_THRESHOLD = 0.8;

export function diffSnapshots(prev: string, next: string): SnapshotDiffResult {
  if (prev === next) {
    return { patch: null, fullRewrite: false };
  }

  const prevLines = prev.split("\n");
  const nextLines = next.split("\n");

  if (prevLines.length !== nextLines.length) {
    return { patch: null, fullRewrite: true };
  }

  const total = nextLines.length;
  if (total === 0) {
    return { patch: null, fullRewrite: false };
  }

  const changedIndices: number[] = [];
  for (let i = 0; i < total; i++) {
    if (prevLines[i] !== nextLines[i]) {
      changedIndices.push(i);
    }
  }

  if (changedIndices.length === 0) {
    return { patch: null, fullRewrite: false };
  }

  if (changedIndices.length / total > FULL_REWRITE_THRESHOLD) {
    return { patch: null, fullRewrite: true };
  }

  let patch = "";
  for (const i of changedIndices) {
    patch += `\x1b[${i + 1};1H\x1b[2K${nextLines[i]}`;
  }

  return { patch, fullRewrite: false };
}
