export type SessionTimelineSeqCursor =
  | {
      epoch: string;
      endSeq: number;
    }
  | null
  | undefined;

export type SessionTimelineSeqDecision = "accept" | "drop_stale" | "drop_epoch" | "gap" | "init";

export function classifySessionTimelineSeq({
  cursor,
  epoch,
  seq,
}: {
  cursor: SessionTimelineSeqCursor;
  epoch: string;
  seq: number;
}): SessionTimelineSeqDecision {
  if (!cursor) {
    return "init";
  }
  if (cursor.epoch !== epoch) {
    // Epochs are ULIDs (lexicographically sortable). A newer epoch means
    // a fresh timeline (e.g. after reconnect / new turn). Reset cursor
    // instead of dropping all events.
    if (epoch > cursor.epoch) {
      return "init";
    }
    return "drop_epoch";
  }
  if (seq <= cursor.endSeq) {
    return "drop_stale";
  }
  if (seq === cursor.endSeq + 1) {
    return "accept";
  }
  return "gap";
}
