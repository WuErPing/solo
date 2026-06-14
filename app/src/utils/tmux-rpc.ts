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
  let hasEnsuredConnected = false;

  for (let attempt = 0; attempt <= MAX_DISPOSED_RETRIES; attempt++) {
    const current = attempt === 0 ? client : store.getClient(serverId);
    if (!current || current.getConnectionState().status === "disposed") {
      if (attempt === 0) {
        throw new Error("Daemon client not available");
      }
      throw lastError ?? new Error("Daemon client not available");
    }

    // If the runtime has a disconnected client, nudge it to reconnect immediately
    // instead of waiting for the next probe cycle. A user action (send key, send
    // command, refresh) should recover from a transient disconnect on its own.
    if (current.getConnectionState().status === "disconnected" && !hasEnsuredConnected) {
      current.ensureConnected();
      hasEnsuredConnected = true;
    }

    try {
      return await op(current);
    } catch (error) {
      lastError = error;
      if (!isRetryableError(error, current)) {
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

function isRetryableError(error: unknown, client: DaemonClient): boolean {
  const status = client.getConnectionState().status;
  if (status === "disposed") {
    return true;
  }
  if (!(error instanceof Error)) {
    return false;
  }
  if (/disposed/i.test(error.message)) {
    return true;
  }
  // Retry connection-lost errors so a user action can recover from a
  // transient disconnect faster than the probe cycle.
  if (status === "disconnected" && /not connected/i.test(error.message)) {
    return true;
  }
  return false;
}
