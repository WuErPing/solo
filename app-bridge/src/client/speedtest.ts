const perfNow: () => number =
  typeof performance !== "undefined" && typeof performance.now === "function"
    ? () => performance.now()
    : () => Date.now();

const DEFAULT_SAMPLES = 5;
const DEFAULT_INTERVAL_MS = 200;
const PING_TIMEOUT_MS = 5000;

export interface LatencySegmentStats {
  minMs: number;
  avgMs: number;
  maxMs: number;
}

export interface SpeedTestSegment {
  id: "appToRelay" | "relayToDaemon" | "daemonProcessing" | "network";
  label: string;
  estimated: boolean;
  stats: LatencySegmentStats;
}

export interface SpeedTestResult {
  transport: "relay" | "direct";
  samples: number;
  totalRtt: LatencySegmentStats & { jitterMs: number };
  segments: SpeedTestSegment[];
  measuredAt: string;
}

export interface SpeedTestPingResult {
  requestId: string;
  clientSentAt: number;
  serverReceivedAt?: number;
  serverSentAt?: number;
  rttMs?: number;
  relayRttMs?: number;
}

export interface SpeedTestOptions {
  samples?: number;
  intervalMs?: number;
}

interface SpeedTestSample {
  totalMs: number;
  daemonMs: number;
  relayRttMs?: number;
}

function round1(value: number): number {
  return Math.round(value * 10) / 10;
}

function aggregate(values: number[]): LatencySegmentStats {
  const minMs = Math.min(...values);
  const maxMs = Math.max(...values);
  const avgMs = values.reduce((sum, value) => sum + value, 0) / values.length;
  return { minMs: round1(minMs), avgMs: round1(avgMs), maxMs: round1(maxMs) };
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Measures per-segment latency between the app and the daemon by sending a
 * series of pings and splitting each RTT into network / relay / daemon parts.
 *
 * Transport detection: if any pong carries `relayRttMs` (the relay↔daemon leg
 * RTT measured by the daemon), the session is treated as a relay session;
 * otherwise it is treated as a direct connection.
 */
export async function runSpeedTest(
  pingFn: (opts: { timeoutMs: number }) => Promise<SpeedTestPingResult>,
  opts?: SpeedTestOptions,
): Promise<SpeedTestResult> {
  const samples = Math.max(1, Math.floor(opts?.samples ?? DEFAULT_SAMPLES));
  const intervalMs = Math.max(0, opts?.intervalMs ?? DEFAULT_INTERVAL_MS);

  const collected: SpeedTestSample[] = [];
  for (let i = 0; i < samples; i += 1) {
    if (i > 0) {
      await sleep(intervalMs);
    }
    const startedAt = perfNow();
    let pong: SpeedTestPingResult;
    try {
      pong = await pingFn({ timeoutMs: PING_TIMEOUT_MS });
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new Error(`Speed test failed on sample ${i + 1} of ${samples}: ${detail}`);
    }
    const totalMs = typeof pong.rttMs === "number" ? pong.rttMs : perfNow() - startedAt;
    const daemonMs =
      typeof pong.serverReceivedAt === "number" && typeof pong.serverSentAt === "number"
        ? Math.max(0, pong.serverSentAt - pong.serverReceivedAt)
        : 0;
    collected.push({ totalMs, daemonMs, relayRttMs: pong.relayRttMs });
  }

  const transport: SpeedTestResult["transport"] = collected.some(
    (sample) => typeof sample.relayRttMs === "number",
  )
    ? "relay"
    : "direct";

  const segments: SpeedTestSegment[] = [];
  if (transport === "relay") {
    const relayLegValues = collected.map((sample) => sample.relayRttMs ?? 0);
    const appRelayValues = collected.map((sample, index) =>
      Math.max(0, sample.totalMs - sample.daemonMs - relayLegValues[index]!),
    );
    segments.push(
      {
        id: "appToRelay",
        label: "App ↔ Relay",
        estimated: true,
        stats: aggregate(appRelayValues),
      },
      {
        id: "relayToDaemon",
        label: "Relay ↔ Daemon",
        estimated: false,
        stats: aggregate(relayLegValues),
      },
      {
        id: "daemonProcessing",
        label: "Daemon processing",
        estimated: false,
        stats: aggregate(collected.map((sample) => sample.daemonMs)),
      },
    );
  } else {
    segments.push(
      {
        id: "network",
        label: "App ↔ Daemon",
        estimated: true,
        stats: aggregate(
          collected.map((sample) => Math.max(0, sample.totalMs - sample.daemonMs)),
        ),
      },
      {
        id: "daemonProcessing",
        label: "Daemon processing",
        estimated: false,
        stats: aggregate(collected.map((sample) => sample.daemonMs)),
      },
    );
  }

  const totalStats = aggregate(collected.map((sample) => sample.totalMs));

  return {
    transport,
    samples: collected.length,
    totalRtt: {
      ...totalStats,
      jitterMs: round1(totalStats.maxMs - totalStats.minMs),
    },
    segments,
    measuredAt: new Date().toISOString(),
  };
}
