import type { StreamItem } from "@/types/stream";

/**
 * Decides whether a pending create can be finalized based on whether the
 * canonical user_message (from the daemon) has arrived with a matching
 * clientMessageId.
 *
 * If the daemon doesn't propagate the clientMessageId, the canonical
 * user_message will have an empty or different id, and this function
 * returns false — leaving the pending create active indefinitely and
 * causing the UI to stay in a stuck loading state.
 */
export function shouldFinalizePendingCreate(params: {
  streamItems: StreamItem[];
  clientMessageId: string;
  canFinalize: boolean;
}): boolean {
  if (!params.canFinalize) return false;
  return params.streamItems.some(
    (item) => item.kind === "user_message" && item.id === params.clientMessageId,
  );
}
