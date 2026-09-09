import { describe, expect, it } from "vitest";
import { DaemonRpcError } from "@server/client/daemon-client";
import { classifyDaemonRequestError } from "./daemon-request-error";

describe("classifyDaemonRequestError", () => {
  it("classifies rpc errors with a code as refused", () => {
    const error = new DaemonRpcError({
      requestId: "r1",
      error: "daemon is not supervised",
      requestType: "switch_daemon_version_request",
      code: "NOT_SUPERVISED",
    });
    expect(classifyDaemonRequestError(error)).toEqual({
      kind: "refused",
      code: "NOT_SUPERVISED",
      message: error.message,
    });
  });

  it("classifies rpc errors without a code as unsupported", () => {
    const error = new DaemonRpcError({
      requestId: "r2",
      error: "unknown message type",
      requestType: "list_daemon_versions_request",
    });
    expect(classifyDaemonRequestError(error)).toMatchObject({ kind: "unsupported" });
  });

  it.each([
    "Timeout waiting for message (10000ms)",
    "Transport not connected (status: closed)",
    "Daemon client closed",
    "Connection lost",
    "Connection lost: transport_error",
    "TooManyPendingRequests: 500 waiters already pending",
  ])("classifies %s as unreachable", (message) => {
    expect(classifyDaemonRequestError(new Error(message))).toMatchObject({
      kind: "unreachable",
      message,
    });
  });

  it("falls back to unreachable for unknown errors", () => {
    expect(classifyDaemonRequestError(new Error("boom"))).toMatchObject({ kind: "unreachable" });
    expect(classifyDaemonRequestError(undefined)).toMatchObject({
      kind: "unreachable",
      message: "undefined",
    });
  });
});
