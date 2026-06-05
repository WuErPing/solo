import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { DaemonClient } from "@server/client/daemon-client";

type ConnectionStateLike =
  | { status: "idle" }
  | { status: "connecting"; attempt?: number }
  | { status: "connected" }
  | { status: "disconnected"; reason?: string }
  | { status: "disposed" };

interface FakeClient {
  state: ConnectionStateLike;
  tmuxCapturePane: ReturnType<typeof vi.fn>;
  getConnectionState: () => ConnectionStateLike;
}

function makeFakeClient(state: ConnectionStateLike = { status: "connected" }): FakeClient {
  return {
    state,
    tmuxCapturePane: vi.fn().mockResolvedValue({ content: "ok", error: null }),
    getConnectionState() {
      return this.state;
    },
  };
}

const { mockStore } = vi.hoisted(() => ({
  mockStore: {
    getClient: vi.fn(),
  },
}));

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => mockStore,
}));

const { withLiveTmuxClient } = await import("./tmux-rpc");

describe("withLiveTmuxClient", () => {
  beforeEach(() => {
    mockStore.getClient.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("forwards the result on first success", async () => {
    const client = makeFakeClient();
    mockStore.getClient.mockReturnValue(client as unknown as DaemonClient);

    const op = vi.fn().mockResolvedValue("first-try-result");
    const result = await withLiveTmuxClient("server-1", op);

    expect(result).toBe("first-try-result");
    expect(op).toHaveBeenCalledTimes(1);
    expect(op).toHaveBeenCalledWith(client);
    expect(mockStore.getClient).toHaveBeenCalledTimes(1);
  });

  it("retries with a fresh client when the first call throws a disposed error", async () => {
    const firstClient = makeFakeClient({ status: "connected" });
    const freshClient = makeFakeClient({ status: "connected" });
    freshClient.tmuxCapturePane.mockResolvedValue({ content: "fresh", error: null });

    // After the first call fails, the helper asks the store again and gets a fresh client.
    mockStore.getClient
      .mockReturnValueOnce(firstClient as unknown as DaemonClient)
      .mockReturnValueOnce(freshClient as unknown as DaemonClient);

    const op = vi
      .fn()
      .mockRejectedValueOnce(new Error("Transport not connected (status: disposed)"))
      .mockResolvedValueOnce("retry-result");

    const result = await withLiveTmuxClient("server-1", op);

    expect(result).toBe("retry-result");
    expect(op).toHaveBeenCalledTimes(2);
    expect(op).toHaveBeenNthCalledWith(1, firstClient);
    expect(op).toHaveBeenNthCalledWith(2, freshClient);
  });

  it("rethrows the original error when the retry client is the same disposed instance", async () => {
    const firstClient = makeFakeClient({ status: "connected" });
    mockStore.getClient.mockReturnValue(firstClient as unknown as DaemonClient);

    const op = vi
      .fn()
      .mockRejectedValueOnce(new Error("Transport not connected (status: disposed)"));

    await expect(withLiveTmuxClient("server-1", op)).rejects.toThrow(
      /Transport not connected/,
    );
    expect(op).toHaveBeenCalledTimes(1);
  });

  it("rethrows the original error when the store has no client on retry", async () => {
    const firstClient = makeFakeClient({ status: "connected" });
    mockStore.getClient
      .mockReturnValueOnce(firstClient as unknown as DaemonClient)
      .mockReturnValueOnce(null);

    const op = vi
      .fn()
      .mockRejectedValueOnce(new Error("Transport not connected (status: disposed)"));

    await expect(withLiveTmuxClient("server-1", op)).rejects.toThrow(
      /Transport not connected/,
    );
    expect(op).toHaveBeenCalledTimes(1);
  });

  it("retries when the client was disposed mid-flight even if the error message is generic", async () => {
    const firstClient = makeFakeClient({ status: "connected" });
    const freshClient = makeFakeClient({ status: "connected" });
    mockStore.getClient
      .mockReturnValueOnce(firstClient as unknown as DaemonClient)
      .mockReturnValueOnce(freshClient as unknown as DaemonClient);

    // Client is disposed during the first op, but the error message is generic.
    const op = vi
      .fn()
      .mockImplementationOnce(async () => {
        firstClient.state = { status: "disposed" };
        throw new Error("RPC timed out");
      })
      .mockResolvedValueOnce("retry-result");

    const result = await withLiveTmuxClient("server-1", op);

    expect(result).toBe("retry-result");
    expect(op).toHaveBeenCalledTimes(2);
  });

  it("rethrows non-disposed errors without retrying", async () => {
    const client = makeFakeClient({ status: "connected" });
    mockStore.getClient.mockReturnValue(client as unknown as DaemonClient);

    const op = vi.fn().mockRejectedValueOnce(new Error("RPC timed out"));

    await expect(withLiveTmuxClient("server-1", op)).rejects.toThrow(/RPC timed out/);
    expect(op).toHaveBeenCalledTimes(1);
  });

  it("throws 'Daemon client not available' when the store has no client up-front", async () => {
    mockStore.getClient.mockReturnValue(null);

    const op = vi.fn();
    await expect(withLiveTmuxClient("server-1", op)).rejects.toThrow(
      /Daemon client not available/,
    );
    expect(op).not.toHaveBeenCalled();
  });

  it("throws 'Daemon client not available' when the initial client is already disposed", async () => {
    const disposedClient = makeFakeClient({ status: "disposed" });
    mockStore.getClient.mockReturnValue(disposedClient as unknown as DaemonClient);

    const op = vi.fn();
    await expect(withLiveTmuxClient("server-1", op)).rejects.toThrow(
      /Daemon client not available/,
    );
    expect(op).not.toHaveBeenCalled();
  });
});
