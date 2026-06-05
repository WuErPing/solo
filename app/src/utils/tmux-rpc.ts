import type { DaemonClient } from "@server/client/daemon-client";
import { getHostRuntimeStore } from "@/runtime/host-runtime";

export async function withLiveTmuxClient<T>(
  serverId: string,
  op: (client: DaemonClient) => Promise<T>,
): Promise<T> {
  const store = getHostRuntimeStore();
  const client = store.getClient(serverId);
  if (!client || client.getConnectionState().status === "disposed") {
    throw new Error("Daemon client not available");
  }
  try {
    return await op(client);
  } catch (error) {
    if (!isDisposedError(error, client)) {
      throw error;
    }
    const retryClient = store.getClient(serverId);
    if (
      !retryClient ||
      retryClient === client ||
      retryClient.getConnectionState().status === "disposed"
    ) {
      throw error;
    }
    return await op(retryClient);
  }
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
