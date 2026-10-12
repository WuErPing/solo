/**
 * Opt-in smoke test for the pair-web channel against a real local relay.
 *
 * It builds `relay-go` from this repo, starts it on a loopback port, then runs
 * the web side (createPairWebHostSession, role=server) and the app side
 * (sendPairWebOffer, role=client) over real WebSocket connections using Node's
 * global WebSocket — no mocks.
 *
 * Run with:
 *
 *   SOLO_PAIR_WEB_SMOKE=1 npx vitest run src/relay/pair-web-channel.smoke.test.ts
 *
 * (skipped by default so CI never needs Go or network sockets)
 */
import { execFile, spawn, type ChildProcess } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import type { ConnectionOffer } from "../shared/connection-offer.js";
import { extractPairWebRequestFromQrContent } from "../shared/pair-web-offer.js";
import { exportPublicKey, generateKeyPair } from "./crypto.js";
import { createPairWebHostSession, sendPairWebOffer } from "./pair-web-channel.js";

const execFileAsync = promisify(execFile);

async function resolveGoBinary(): Promise<string> {
  if (process.env.SOLO_GO_BIN) {
    return process.env.SOLO_GO_BIN;
  }
  try {
    await execFileAsync("go", ["version"]);
    return "go";
  } catch {
    for (const candidate of ["/opt/homebrew/bin/go", "/usr/local/go/bin/go"]) {
      if (existsSync(candidate)) {
        return candidate;
      }
    }
    throw new Error("go toolchain not found; set SOLO_GO_BIN to the go binary path");
  }
}

const SMOKE_ENABLED = process.env.SOLO_PAIR_WEB_SMOKE === "1";
const RELAY_PORT = 18089;
const RELAY_ENDPOINT = `127.0.0.1:${RELAY_PORT}`;
const REPO_ROOT = path.resolve(import.meta.dirname, "../../..");
const RELAY_GO_DIR = path.join(REPO_ROOT, "relay-go");

async function waitForRelayHealthy(timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    try {
      const response = await fetch(`http://${RELAY_ENDPOINT}/health`);
      if (response.ok) {
        return;
      }
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) {
      throw new Error(`relay did not become healthy within ${timeoutMs}ms`);
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
}

describe.skipIf(!SMOKE_ENABLED)("pair-web channel smoke (real relay-go)", () => {
  let relayProcess: ChildProcess | null = null;

  beforeAll(async () => {
    const goBin = await resolveGoBinary();
    const buildDir = await mkdtemp(path.join(tmpdir(), "solo-relay-smoke-"));
    const relayBin = path.join(buildDir, "solo-relay");
    await execFileAsync(goBin, ["build", "-o", relayBin, "./cmd/relay"], { cwd: RELAY_GO_DIR });

    relayProcess = spawn(relayBin, {
      env: {
        ...process.env,
        PORT: String(RELAY_PORT),
        HOST: "127.0.0.1",
        LOG_LEVEL: "error",
      },
      stdio: "ignore",
    });
    await waitForRelayHealthy(10_000);
  }, 120_000);

  afterAll(async () => {
    if (relayProcess && !relayProcess.killed) {
      relayProcess.kill("SIGTERM");
    }
  });

  it(
    "delivers an end-to-end encrypted offer from app to web through the relay",
    async () => {
      const session = await createPairWebHostSession({ relayEndpoint: RELAY_ENDPOINT });

      // The app scans the QR content the web UI would render.
      const request = extractPairWebRequestFromQrContent(session.qrContent);
      expect(request).not.toBeNull();
      expect(request!.pairingId).toBe(session.request.pairingId);

      const offer: ConnectionOffer = {
        v: 2,
        serverId: "srv_smoke_test",
        daemonPublicKeyB64: exportPublicKey(generateKeyPair().publicKey),
        relay: { endpoint: "solo.up2ai.top:443" },
      };

      const [received] = await Promise.all([
        session.waitForOffer(),
        sendPairWebOffer({ request: request!, offer, confirmTimeoutMs: 5_000 }),
      ]);
      expect(received).toEqual(offer);

      session.close();
    },
    60_000,
  );
});
