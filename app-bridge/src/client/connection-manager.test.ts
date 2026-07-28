import { describe, expect, it, vi, afterEach } from "vitest";
import { ConnectionManager } from "./connection-manager.js";
import { MockTransport, createMockTransportFactory } from "./mock-transport.js";
import type { DaemonClientConfig } from "./daemon-client.js";

function buildConfig(overrides?: Partial<DaemonClientConfig>): DaemonClientConfig {
  return {
    url: "ws://localhost:17612",
    clientId: "test-client",
    clientType: "cli",
    reconnect: { enabled: true, baseDelayMs: 100, maxDelayMs: 1000 },
    transportFactory: createMockTransportFactory(new MockTransport()),
    connectTimeoutMs: 60000,
    suppressSendErrors: true,
    ...overrides,
  };
}

function serverInfoMessage(): string {
  return JSON.stringify({
    type: "session",
    message: {
      type: "status",
      payload: {
        status: "server_info",
        serverId: "test-server-id",
        hostname: "test-host",
      },
    },
  });
}

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("ConnectionManager — reconnect", () => {
  it("arms a reconnect timer and re-attempts connection after transport close", async () => {
    vi.useFakeTimers();
    vi.spyOn(Math, "random").mockReturnValue(0); // jitter = 0.8

    const transports: MockTransport[] = [];
    const manager = new ConnectionManager(
      buildConfig({
        transportFactory: () => {
          const t = new MockTransport();
          transports.push(t);
          return t;
        },
      }),
    );

    const connectPromise = manager.connect();
    transports[0]!.simulateOpen();
    transports[0]!.simulateMessage(serverInfoMessage());
    await connectPromise;

    expect(manager.getConnectionState().status).toBe("connected");

    // Simulate transport close.
    transports[0]!.simulateClose({ code: 1006, reason: "abnormal" });

    expect(manager.getConnectionState().status).toBe("disconnected");

    // Advance past the first reconnect delay (baseDelay 100 * jitter 0.8 = 80ms).
    await vi.advanceTimersByTimeAsync(100);

    expect(transports.length).toBeGreaterThanOrEqual(2);
    transports[1]!.simulateOpen();
    transports[1]!.simulateMessage(serverInfoMessage());

    expect(manager.getConnectionState().status).toBe("connected");
    await manager.close();
  });

  it("increases reconnect delay on successive failures", async () => {
    vi.useFakeTimers();
    vi.spyOn(Math, "random").mockReturnValue(0);

    const transports: MockTransport[] = [];
    const manager = new ConnectionManager(
      buildConfig({
        transportFactory: () => {
          const t = new MockTransport();
          transports.push(t);
          return t;
        },
      }),
    );

    void manager.connect();
    transports[0]!.simulateOpen();
    transports[0]!.simulateMessage(serverInfoMessage());

    // Close and measure how long until the next attempt.
    transports[0]!.simulateClose({ code: 1006 });
    await vi.advanceTimersByTimeAsync(80);
    expect(transports.length).toBe(2);

    // Close the second attempt before it connects and measure next delay.
    transports[1]!.simulateClose({ code: 1006 });
    // Second attempt delay: 100 * 2^1 * 0.8 = 160ms.
    await vi.advanceTimersByTimeAsync(160);
    expect(transports.length).toBe(3);

    await manager.close();
  });

  it("does not reconnect when reconnect is disabled", async () => {
    vi.useFakeTimers();

    const transports: MockTransport[] = [];
    const manager = new ConnectionManager(
      buildConfig({
        reconnect: { enabled: false },
        transportFactory: () => {
          const t = new MockTransport();
          transports.push(t);
          return t;
        },
      }),
    );

    const connectPromise = manager.connect();
    transports[0]!.simulateOpen();
    transports[0]!.simulateMessage(serverInfoMessage());
    await connectPromise;

    transports[0]!.simulateClose({ code: 1006 });

    // Advance well past any would-be reconnect window.
    await vi.advanceTimersByTimeAsync(5000);

    expect(transports.length).toBe(1);
    expect(manager.getConnectionState().status).toBe("disconnected");
  });
});
