import type { DaemonClient } from "@server/client/daemon-client";
import { getHostRuntimeStore } from "@/runtime/host-runtime";

const MAX_DISPOSED_RETRIES = 3;
const RETRY_DELAY_MS = 150;

export async function withLiveTmuxClient<T>(
  serverId: string,
  op: (client: DaemonClient) => Promise<T>,
): Promise<T> {
  const store = getHostRuntimeStore();
  const client = store.getClient(serverId);
  if (!client || client.getConnectionState().status === "disposed") {
    throw new Error("Daemon client not available");
  }

  let lastError: unknown;

  for (let attempt = 0; attempt <= MAX_DISPOSED_RETRIES; attempt++) {
    const current = attempt === 0 ? client : store.getClient(serverId);
    if (!current || current.getConnectionState().status === "disposed") {
      if (attempt === 0) {
        throw new Error("Daemon client not available");
      }
      throw lastError ?? new Error("Daemon client not available");
    }
    try {
      return await op(current);
    } catch (error) {
      lastError = error;
      if (!isDisposedError(error, current)) {
        throw error;
      }
      if (attempt < MAX_DISPOSED_RETRIES) {
        await delay(RETRY_DELAY_MS);
      }
    }
  }
  throw lastError;
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isDisposedError(error: unknown, client: DaemonClient): boolean {
  if (client.getConnectionState().status === "disposed") {
    return true;
  }
  if (!(error instanceof Error)) {
    return false;
  }
  return /disposed/i.test(error.message);
}
