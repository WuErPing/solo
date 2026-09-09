import { DaemonRpcError } from "@server/client/daemon-client";

export type DaemonRequestErrorKind = "unreachable" | "unsupported" | "refused";

export interface ClassifiedDaemonRequestError {
  kind: DaemonRequestErrorKind;
  code?: string;
  message: string;
}

// Message prefixes produced by app-bridge's ConnectionManager for
// connection-level failures (as opposed to daemon RPC errors).
const TRANSPORT_FAILURE_PREFIXES = [
  "Transport not connected",
  "Daemon client closed",
  "Connection lost",
  "TooManyPendingRequests",
];

// Maps a thrown request error to a user-facing category: the daemon answered
// and refused (refused), the daemon predates the RPC (unsupported), or the
// request never reached a daemon (unreachable).
export function classifyDaemonRequestError(error: unknown): ClassifiedDaemonRequestError {
  const message = error instanceof Error ? error.message : String(error);
  if (error instanceof DaemonRpcError) {
    return error.code
      ? { kind: "refused", code: error.code, message }
      : { kind: "unsupported", message };
  }
  const unreachable =
    message.startsWith("Timeout waiting for message") ||
    TRANSPORT_FAILURE_PREFIXES.some((prefix) => message.startsWith(prefix));
  if (unreachable) {
    return { kind: "unreachable", message };
  }
  return { kind: "unreachable", message };
}
