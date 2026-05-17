type TimelineDirection = "tail" | "before" | "after";
type InitRequestDirection = "tail" | "after";

type BootstrapTailCursor = {
  seq: number;
} | null;

type InitialTimelineCursor = {
  epoch: string;
  seq: number;
} | null;

export function deriveInitialTimelineRequest({
  cursor,
  hasAuthoritativeHistory,
  initialTimelineLimit,
}: {
  cursor: InitialTimelineCursor;
  hasAuthoritativeHistory: boolean;
  initialTimelineLimit: number;
}): {
  direction: "tail" | "after";
  cursor?: { epoch: string; seq: number };
  limit: number;
  projection: "canonical";
} {
  if (!hasAuthoritativeHistory || !cursor) {
    return {
      direction: "tail",
      limit: initialTimelineLimit,
      projection: "canonical",
    };
  }

  return {
    direction: "after",
    cursor: { epoch: cursor.epoch, seq: cursor.seq },
    limit: 0,
    projection: "canonical",
  };
}

export function deriveBootstrapTailTimelinePolicy({
  direction,
  reset,
  epoch,
  endCursor,
  isInitializing,
  hasActiveInitDeferred,
}: {
  direction: TimelineDirection;
  reset: boolean;
  epoch: string;
  endCursor: BootstrapTailCursor;
  isInitializing: boolean;
  hasActiveInitDeferred: boolean;
}): {
  replace: boolean;
  catchUpCursor: { epoch: string; endSeq: number } | null;
} {
  if (reset) {
    return { replace: true, catchUpCursor: null };
  }

  const isBootstrapTailInit = direction === "tail" && isInitializing && hasActiveInitDeferred;
  if (!isBootstrapTailInit) {
    return { replace: false, catchUpCursor: null };
  }

  return {
    replace: true,
    catchUpCursor: endCursor ? { epoch, endSeq: endCursor.seq } : null,
  };
}

export function shouldResolveTimelineInit({
  hasActiveInitDeferred,
  isInitializing,
  initRequestDirection,
  responseDirection,
  reset,
}: {
  hasActiveInitDeferred: boolean;
  isInitializing: boolean;
  initRequestDirection: InitRequestDirection;
  responseDirection: TimelineDirection;
  reset: boolean;
}): boolean {
  if (!hasActiveInitDeferred || !isInitializing) {
    return false;
  }
  if (reset) {
    return true;
  }
  return responseDirection === initRequestDirection;
}
