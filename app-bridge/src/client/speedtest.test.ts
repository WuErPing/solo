import { describe, expect, it, vi } from "vitest";
import {
  runSpeedTest,
  type SpeedTestPingResult,
} from "./speedtest.js";

function stubPing(
  pongs: Array<Omit<SpeedTestPingResult, "requestId" | "clientSentAt">>,
): (opts: { timeoutMs: number }) => Promise<SpeedTestPingResult> {
  let index = 0;
  return async () => {
    const pong = pongs[Math.min(index, pongs.length - 1)]!;
    index += 1;
    return { requestId: `req-${index}`, clientSentAt: 0, ...pong };
  };
}

describe("runSpeedTest — relay transport", () => {
  it("splits RTT into app↔relay, relay↔daemon and daemon processing segments", async () => {
    const result = await runSpeedTest(
      stubPing([
        {
          rttMs: 100,
          serverReceivedAt: 1000,
          serverSentAt: 1004,
          relayRttMs: 40,
        },
      ]),
      { samples: 1, intervalMs: 0 },
    );

    expect(result.transport).toBe("relay");
    expect(result.samples).toBe(1);
    expect(result.segments.map((s) => s.id)).toEqual([
      "appToRelay",
      "relayToDaemon",
      "daemonProcessing",
    ]);

    const [appToRelay, relayToDaemon, daemonProcessing] = result.segments;
    // appToRelay = 100 - 4 - 40 = 56, derived → estimated
    expect(appToRelay!.stats).toEqual({ minMs: 56, avgMs: 56, maxMs: 56 });
    expect(appToRelay!.estimated).toBe(true);
    expect(appToRelay!.label).toBe("App ↔ Relay");

    expect(relayToDaemon!.stats).toEqual({ minMs: 40, avgMs: 40, maxMs: 40 });
    expect(relayToDaemon!.estimated).toBe(false);

    expect(daemonProcessing!.stats).toEqual({ minMs: 4, avgMs: 4, maxMs: 4 });
    expect(daemonProcessing!.estimated).toBe(false);

    expect(result.totalRtt).toEqual({ minMs: 100, avgMs: 100, maxMs: 100, jitterMs: 0 });
    expect(typeof result.measuredAt).toBe("string");
  });
});

describe("runSpeedTest — direct transport", () => {
  it("reports network and daemon processing segments when no relayRttMs is present", async () => {
    const result = await runSpeedTest(
      stubPing([{ rttMs: 30, serverReceivedAt: 1000, serverSentAt: 1005 }]),
      { samples: 1, intervalMs: 0 },
    );

    expect(result.transport).toBe("direct");
    expect(result.segments.map((s) => s.id)).toEqual(["network", "daemonProcessing"]);

    const [network, daemonProcessing] = result.segments;
    expect(network!.label).toBe("App ↔ Daemon");
    expect(network!.estimated).toBe(true);
    expect(network!.stats).toEqual({ minMs: 25, avgMs: 25, maxMs: 25 });
    expect(daemonProcessing!.stats).toEqual({ minMs: 5, avgMs: 5, maxMs: 5 });
  });

  it("measures RTT around the ping call when rttMs is absent", async () => {
    const pingFn = async (): Promise<SpeedTestPingResult> => {
      await new Promise((resolve) => setTimeout(resolve, 15));
      return { requestId: "req", clientSentAt: 0, serverReceivedAt: 1, serverSentAt: 2 };
    };
    const result = await runSpeedTest(pingFn, { samples: 1, intervalMs: 0 });
    expect(result.transport).toBe("direct");
    expect(result.totalRtt.minMs).toBeGreaterThanOrEqual(10);
    const network = result.segments.find((s) => s.id === "network")!;
    // daemonMs = 1, so network = total - 1
    expect(network.stats.minMs).toBeGreaterThanOrEqual(9);
  });
});

describe("runSpeedTest — clamping", () => {
  it("never produces negative segment values when daemonMs exceeds totalMs", async () => {
    const result = await runSpeedTest(
      stubPing([
        { rttMs: 10, serverReceivedAt: 1000, serverSentAt: 1050, relayRttMs: 20 },
      ]),
      { samples: 1, intervalMs: 0 },
    );

    for (const segment of result.segments) {
      expect(segment.stats.minMs).toBeGreaterThanOrEqual(0);
      expect(segment.stats.maxMs).toBeGreaterThanOrEqual(0);
    }
    const appToRelay = result.segments.find((s) => s.id === "appToRelay")!;
    expect(appToRelay.stats.maxMs).toBe(0);
  });

  it("treats missing daemon timestamps as zero daemon processing time", async () => {
    const result = await runSpeedTest(stubPing([{ rttMs: 42 }]), {
      samples: 1,
      intervalMs: 0,
    });
    const daemon = result.segments.find((s) => s.id === "daemonProcessing")!;
    expect(daemon.stats).toEqual({ minMs: 0, avgMs: 0, maxMs: 0 });
    const network = result.segments.find((s) => s.id === "network")!;
    expect(network.stats.maxMs).toBe(42);
  });
});

describe("runSpeedTest — aggregation", () => {
  it("computes min/avg/max and jitter across samples", async () => {
    const result = await runSpeedTest(
      stubPing([
        { rttMs: 100, serverReceivedAt: 0, serverSentAt: 10, relayRttMs: 40 },
        { rttMs: 110, serverReceivedAt: 0, serverSentAt: 12, relayRttMs: 44 },
        { rttMs: 105, serverReceivedAt: 0, serverSentAt: 11, relayRttMs: 42 },
      ]),
      { samples: 3, intervalMs: 0 },
    );

    expect(result.samples).toBe(3);
    expect(result.totalRtt).toEqual({ minMs: 100, avgMs: 105, maxMs: 110, jitterMs: 10 });

    const relayToDaemon = result.segments.find((s) => s.id === "relayToDaemon")!;
    expect(relayToDaemon.stats).toEqual({ minMs: 40, avgMs: 42, maxMs: 44 });

    const daemon = result.segments.find((s) => s.id === "daemonProcessing")!;
    expect(daemon.stats).toEqual({ minMs: 10, avgMs: 11, maxMs: 12 });

    const appToRelay = result.segments.find((s) => s.id === "appToRelay")!;
    // 100-10-40=50, 110-12-44=54, 105-11-42=52
    expect(appToRelay.stats).toEqual({ minMs: 50, avgMs: 52, maxMs: 54 });
  });

  it("sends the requested number of pings with the configured interval", async () => {
    vi.useFakeTimers();
    try {
      const pingFn = vi.fn(async (): Promise<SpeedTestPingResult> => ({
        requestId: "req",
        clientSentAt: 0,
        rttMs: 10,
      }));

      const promise = runSpeedTest(pingFn, { samples: 3, intervalMs: 200 });
      expect(pingFn).toHaveBeenCalledTimes(1); // first ping is immediate

      await vi.advanceTimersByTimeAsync(200);
      expect(pingFn).toHaveBeenCalledTimes(2);

      await vi.advanceTimersByTimeAsync(200);
      const result = await promise;
      expect(pingFn).toHaveBeenCalledTimes(3);
      expect(pingFn).toHaveBeenCalledWith({ timeoutMs: 5000 });
      expect(result.samples).toBe(3);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("runSpeedTest — failure handling", () => {
  it("aborts with a descriptive error when a ping fails", async () => {
    let calls = 0;
    const pingFn = async (): Promise<SpeedTestPingResult> => {
      calls += 1;
      if (calls === 2) {
        throw new Error("connection lost");
      }
      return { requestId: "req", clientSentAt: 0, rttMs: 10 };
    };

    await expect(
      runSpeedTest(pingFn, { samples: 5, intervalMs: 0 }),
    ).rejects.toThrow("Speed test failed on sample 2 of 5: connection lost");
    expect(calls).toBe(2);
  });
});
