import { spawn, type ChildProcess, execFileSync, execSync } from "node:child_process";
import { existsSync } from "node:fs";
import { chmod, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import net from "node:net";
import { Buffer } from "node:buffer";
import dotenv from "dotenv";
import { forkSoloHomeMetadata, resolveSoloHomePath } from "./helpers/solo-home-fork";

interface WaitForServerOptions {
  host?: string;
  timeoutMs?: number;
  label: string;
  childProcess?: ChildProcess | null;
  getRecentOutput?: () => string;
}

async function getAvailablePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once("error", reject);
    server.listen(0, () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        server.close(() => reject(new Error("Failed to acquire port")));
        return;
      }
      server.close(() => resolve(address.port));
    });
  });
}

function createLineBuffer(maxLines = 120): { add: (line: string) => void; dump: () => string } {
  const lines: string[] = [];
  return {
    add(line: string) {
      lines.push(line);
      if (lines.length > maxLines) {
        lines.shift();
      }
    },
    dump() {
      return lines.join("\n");
    },
  };
}

function formatRecentOutput(getRecentOutput?: () => string): string {
  if (!getRecentOutput) {
    return "";
  }
  const output = getRecentOutput().trim();
  if (!output) {
    return "";
  }
  return `\nRecent output:\n${output}`;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitForServer(port: number, options: WaitForServerOptions): Promise<void> {
  const { host = "127.0.0.1", timeoutMs = 15000, label, childProcess, getRecentOutput } = options;
  const start = Date.now();
  let lastConnectionError: unknown = null;

  while (Date.now() - start < timeoutMs) {
    if (childProcess && childProcess.exitCode !== null) {
      const signal = childProcess.signalCode ? `, signal ${childProcess.signalCode}` : "";
      throw new Error(
        `${label} exited before listening on ${host}:${port} (exit code ${childProcess.exitCode}${signal}).${formatRecentOutput(getRecentOutput)}`,
      );
    }

    try {
      await new Promise<void>((resolve, reject) => {
        const socket = net.connect(port, host, () => {
          socket.end();
          resolve();
        });
        socket.setTimeout(1000, () => {
          socket.destroy();
          reject(new Error(`Connection timed out to ${host}:${port}`));
        });
        socket.on("error", reject);
      });
      return;
    } catch (error) {
      lastConnectionError = error;
      await new Promise((r) => setTimeout(r, 100));
    }
  }

  const reason =
    lastConnectionError instanceof Error
      ? ` Last connection error: ${lastConnectionError.message}`
      : "";
  throw new Error(
    `${label} did not start on ${host}:${port} within ${timeoutMs}ms.${reason}${formatRecentOutput(getRecentOutput)}`,
  );
}

function parseRelayStartupFailure(line: string): string | null {
  const clean = stripAnsi(line);
  if (/Address already in use/i.test(clean)) {
    return clean;
  }
  if (/failed: ::bind\(/i.test(clean)) {
    return clean;
  }
  if (/Fatal uncaught/i.test(clean)) {
    return clean;
  }
  return null;
}

async function stopProcess(child: ChildProcess | null): Promise<void> {
  if (!child) {
    return;
  }
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }
  child.kill("SIGTERM");
  await new Promise<void>((resolve) => {
    let pendingResolve: (() => void) | null = resolve;
    const settle = () => {
      if (!pendingResolve) return;
      const fn = pendingResolve;
      pendingResolve = null;
      clearTimeout(timeout);
      fn();
    };
    const timeout = setTimeout(() => {
      if (child.exitCode === null && child.signalCode === null) {
        child.kill("SIGKILL");
      }
      settle();
    }, 5000);
    child.once("exit", settle);
  });
}

let daemonProcess: ChildProcess | null = null;
let metroProcess: ChildProcess | null = null;
let soloHome: string | null = null;
let fakeGhBinDir: string | null = null;
let relayProcess: ChildProcess | null = null;

function resolveOptionalSoloHomeEnv(value: string | undefined): string | null {
  const trimmed = value?.trim();
  if (!trimmed) {
    return null;
  }
  if (trimmed === "current") {
    return resolveSoloHomePath("~/.solo");
  }
    return resolveSoloHomePath(trimmed);
}

interface OfferPayload {
  v: 2;
  serverId: string;
  daemonPublicKeyB64: string;
  relay: { endpoint: string };
}

async function createFakeGhBin(): Promise<string> {
  const binDir = await mkdtemp(path.join(tmpdir(), "solo-e2e-gh-bin-"));
  const ghPath = path.join(binDir, "gh");
  await writeFile(
    ghPath,
    `#!/usr/bin/env node
const args = process.argv.slice(2);

if (args[0] === "auth" && args[1] === "status") {
  process.exit(0);
}

if (args[0] === "pr" && args[1] === "list") {
  console.log(JSON.stringify([
    {
      number: 515,
      title: "Review selected start ref",
      url: "https://github.com/getsolo/solo/pull/515",
      state: "OPEN",
      body: "Fixture pull request for app e2e.",
      labels: [],
      baseRefName: "main",
      headRefName: "feature/start-from-pr"
    }
  ]));
  process.exit(0);
}

if (args[0] === "pr" && args[1] === "view" && args[2] === "--json" && args[3]) {
  console.error("no pull requests found for branch");
  process.exit(1);
}

if (args[0] === "issue" && args[1] === "list") {
  console.log("[]");
  process.exit(0);
}

console.error("Unsupported fake gh invocation: " + args.join(" "));
process.exit(1);
`,
  );
  await chmod(ghPath, 0o755);
  return binDir;
}

const ANSI_PATTERN = new RegExp(`${String.fromCharCode(0x1b)}\\[[0-9;]*m`, "g");

function stripAnsi(input: string): string {
  return input.replace(ANSI_PATTERN, "");
}

function ensureRelayBuildArtifact(repoRoot: string): void {
  const relayBin = path.join(repoRoot, "output", "solo-relay");
  if (existsSync(relayBin)) {
    return;
  }

  console.log("[e2e] Building solo-relay...");
  execSync("make solo-relay", {
    cwd: repoRoot,
    stdio: "inherit",
  });
}

function ensureAppBridgeBuildArtifact(repoRoot: string): void {
  const appBridgeDist = path.join(repoRoot, "app-bridge", "dist", "client", "daemon-client.js");
  if (existsSync(appBridgeDist)) {
    return;
  }

  console.log("[e2e] Building app-bridge daemon-client...");
  execSync("npx esbuild src/client/daemon-client.ts --bundle --platform=node --format=cjs --outdir=dist/client --external:zod", {
    cwd: path.join(repoRoot, "app-bridge"),
    stdio: "inherit",
  });
}

function decodeOfferFromFragmentUrl(url: string): OfferPayload {
  const marker = "#offer=";
  const idx = url.indexOf(marker);
  if (idx === -1) {
    throw new Error(`missing ${marker} fragment: ${url}`);
  }
  const encoded = url.slice(idx + marker.length);
  const json = Buffer.from(encoded, "base64url").toString("utf8");
  const offer = JSON.parse(json) as Partial<OfferPayload>;
  if (offer.v !== 2) throw new Error("offer.v missing/invalid");
  if (!offer.serverId) throw new Error("offer.serverId missing");
  if (!offer.daemonPublicKeyB64) throw new Error("offer.daemonPublicKeyB64 missing");
  if (!offer.relay?.endpoint) throw new Error("offer.relay.endpoint missing");
  return offer as OfferPayload;
}

function loadPairingOfferFromCli(repoRoot: string, soloHomePath: string): OfferPayload {
  const cliBin = path.join(repoRoot, "output", "solo-cli");
  const stdout = execFileSync(
    cliBin,
    ["daemon", "pair", "--json"],
    {
      cwd: repoRoot,
      env: {
        ...process.env,
        SOLO_HOME: soloHomePath,
      },
      encoding: "utf8",
    },
  );
  const payload = JSON.parse(stdout) as { relayEnabled?: boolean | string; url?: string | null };
  if ((payload.relayEnabled !== true && payload.relayEnabled !== "true") || typeof payload.url !== "string") {
    throw new Error(`Unexpected daemon pair response: ${stdout}`);
  }
  return decodeOfferFromFragmentUrl(payload.url);
}

async function waitForPairingOfferFromCli(args: {
  repoRoot: string;
  soloHome: string;
  timeoutMs?: number;
}): Promise<OfferPayload> {
  const timeoutMs = args.timeoutMs ?? 15000;
  const start = Date.now();
  let lastError: unknown = null;

  while (Date.now() - start < timeoutMs) {
    try {
      return loadPairingOfferFromCli(args.repoRoot, args.soloHome);
    } catch (error) {
      lastError = error;
      await sleep(100);
    }
  }

  throw new Error(
    `Timed out waiting for \`solo-cli daemon pair --json\` to produce a pairing offer: ${
      lastError instanceof Error ? lastError.message : String(lastError)
    }`,
  );
}

async function loadEnvTestFile(repoRoot: string): Promise<void> {
  const envTestPath = path.join(repoRoot, ".env.test");
  if (existsSync(envTestPath)) {
    dotenv.config({ path: envTestPath });
  }
}

async function applySoloHomeFork(targetHome: string): Promise<void> {
  const forkSourceHome = resolveOptionalSoloHomeEnv(process.env.E2E_FORK_SOLO_HOME_FROM);
  if (!forkSourceHome) {
    return;
  }
  const forkResult = await forkSoloHomeMetadata({
    sourceHome: forkSourceHome,
    targetHome,
  });
  process.env.E2E_FORK_SOURCE_SOLO_HOME = forkResult.sourceHome;
  process.env.E2E_FORK_TARGET_SOLO_HOME = forkResult.targetHome;
  process.env.E2E_FORK_COPIED_FILES = String(forkResult.copiedFiles);
  process.env.E2E_FORK_COPIED_BYTES = String(forkResult.copiedBytes);
  console.log(
    `[e2e] Forked Solo metadata from ${forkResult.sourceHome} to ${forkResult.targetHome} ` +
      `(${forkResult.agentFiles} agent files, ${forkResult.projectFiles} project registry files, ` +
      `${forkResult.copiedBytes} bytes)`,
  );
  if (forkResult.skippedMissing.length > 0) {
    console.warn(
      `[e2e] Solo metadata fork skipped missing paths: ${forkResult.skippedMissing.join(", ")}`,
    );
  }
}

interface RelayStreamState {
  failureLine: string | null;
}

function attachRelayStreamHandlers(
  child: ChildProcess,
  relayPort: number,
  buffer: ReturnType<typeof createLineBuffer>,
  state: RelayStreamState,
): void {
  function handleChunk(data: Buffer, streamTag: "stdout" | "stderr") {
    const lines = data
      .toString()
      .split("\n")
      .filter((line) => line.trim());
    for (const line of lines) {
      buffer.add(`[${streamTag}] ${line}`);
      const failure = parseRelayStartupFailure(line);
      if (failure) {
        state.failureLine = failure;
      }
      if (streamTag === "stdout") {
        console.log(`[relay] ${line}`);
      } else {
        console.error(`[relay] ${line}`);
      }
    }
  }

  child.stdout?.on("data", (data: Buffer) => handleChunk(data, "stdout"));
  child.stderr?.on("data", (data: Buffer) => handleChunk(data, "stderr"));
}

async function awaitRelayReady(
  child: ChildProcess,
  relayPort: number,
  state: RelayStreamState,
  buffer: ReturnType<typeof createLineBuffer>,
): Promise<void> {
  await waitForServer(relayPort, {
    label: "Relay server",
    timeoutMs: 30000,
    childProcess: child,
    getRecentOutput: buffer.dump,
  });

  if (state.failureLine) {
    throw new Error(`Relay startup failed: ${state.failureLine}`);
  }
  if (child.exitCode !== null || child.signalCode !== null) {
    throw new Error(
      `Relay process exited before startup completed (exit code ${child.exitCode}, signal ${child.signalCode}).${formatRecentOutput(
        buffer.dump,
      )}`,
    );
  }
}

async function startRelay(repoRoot: string): Promise<number> {
  const relayBin = path.join(repoRoot, "output", "solo-relay");
  const maxRelayStartupAttempts = 5;
  let lastRelayStartupError: unknown = null;

  for (let attempt = 1; attempt <= maxRelayStartupAttempts; attempt += 1) {
    const relayPort = await getAvailablePort();
    const buffer = createLineBuffer();
    const state: RelayStreamState = { failureLine: null };

    relayProcess = spawn(relayBin, [], {
      cwd: repoRoot,
      env: {
        ...process.env,
        PORT: String(relayPort),
        HOST: "127.0.0.1",
      },
      stdio: ["ignore", "pipe", "pipe"],
      detached: false,
    });
    attachRelayStreamHandlers(relayProcess, relayPort, buffer, state);

    try {
      await awaitRelayReady(relayProcess, relayPort, state, buffer);
      return relayPort;
    } catch (error) {
      lastRelayStartupError = error;
      await stopProcess(relayProcess);
      relayProcess = null;
    }
  }

  const message =
    lastRelayStartupError instanceof Error
      ? lastRelayStartupError.message
      : String(lastRelayStartupError);
  throw new Error(
    `Failed to start relay after ${maxRelayStartupAttempts} attempts. ${message}`,
  );
}

function startMetro(metroPort: number, buffer: ReturnType<typeof createLineBuffer>): ChildProcess {
  const appDir = path.resolve(__dirname, "..");
  const child = spawn("npx", ["expo", "start", "--web", "--port", String(metroPort)], {
    cwd: appDir,
    env: {
      ...process.env,
      BROWSER: "none",
    },
    stdio: ["ignore", "pipe", "pipe"],
    detached: false,
  });

  child.stdout?.on("data", (data: Buffer) => {
    const lines = data
      .toString()
      .split("\n")
      .filter((line) => line.trim());
    for (const line of lines) {
      buffer.add(`[stdout] ${line}`);
      console.log(`[metro] ${line}`);
    }
  });

  child.stderr?.on("data", (data: Buffer) => {
    const lines = data
      .toString()
      .split("\n")
      .filter((line) => line.trim());
    for (const line of lines) {
      buffer.add(`[stderr] ${line}`);
      console.error(`[metro] ${line}`);
    }
  });

  return child;
}

interface DaemonSpawnArgs {
  port: number;
  relayPort: number;
  metroPort: number;
  repoRoot: string;
  soloHome: string;
  fakeGhBinDir: string;
  buffer: ReturnType<typeof createLineBuffer>;
}

function startDaemon(args: DaemonSpawnArgs): ChildProcess {
  const soloBin = path.join(args.repoRoot, "output", "solo");

  const child = spawn(soloBin, [], {
    cwd: args.repoRoot,
    env: {
      ...process.env,
      PATH: `${args.fakeGhBinDir}${path.delimiter}${process.env.PATH ?? ""}`,
      SOLO_HOME: args.soloHome,
      SOLO_LISTEN: `0.0.0.0:${args.port}`,
      SOLO_RELAY_ENDPOINT: `127.0.0.1:${args.relayPort}`,
      SOLO_RELAY_PUBLIC_ENDPOINT: `127.0.0.1:${args.relayPort}`,
      SOLO_CORS_ORIGINS: `http://localhost:${args.metroPort}`,
      SOLO_APP_BASE_URL: `http://localhost:${args.metroPort}`,
      SOLO_ENABLE_MOCK_PROVIDER: "1",
      NODE_ENV: "development",
    },
    stdio: ["ignore", "pipe", "pipe"],
    detached: false,
  });

  let stdoutBuffer = "";
  child.stdout?.on("data", (data: Buffer) => {
    stdoutBuffer += data.toString("utf8");
    const lines = stdoutBuffer.split("\n");
    stdoutBuffer = lines.pop() ?? "";
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      args.buffer.add(`[stdout] ${trimmed}`);
      console.log(`[daemon] ${trimmed}`);
    }
  });

  child.stderr?.on("data", (data: Buffer) => {
    const lines = data
      .toString()
      .split("\n")
      .filter((line) => line.trim());
    for (const line of lines) {
      args.buffer.add(`[stderr] ${line}`);
      console.error(`[daemon] ${line}`);
    }
  });

  return child;
}

async function performCleanup(shouldRemoveSoloHome: boolean): Promise<void> {
  await Promise.all([
    stopProcess(daemonProcess),
    stopProcess(metroProcess),
    stopProcess(relayProcess),
  ]);
  daemonProcess = null;
  metroProcess = null;
  relayProcess = null;
  if (soloHome && shouldRemoveSoloHome) {
    await rm(soloHome, { recursive: true, force: true });
    soloHome = null;
  } else if (soloHome) {
    console.log(`[e2e] Preserving SOLO_HOME: ${soloHome}`);
  }
  if (fakeGhBinDir) {
    await rm(fakeGhBinDir, { recursive: true, force: true });
    fakeGhBinDir = null;
  }
}

export default async function globalSetup() {
  const repoRoot = path.resolve(__dirname, "../..");
  ensureRelayBuildArtifact(repoRoot);
  ensureAppBridgeBuildArtifact(repoRoot);
  await loadEnvTestFile(repoRoot);

  const port = await getAvailablePort();
  const metroPort = await getAvailablePort();
  const requestedSoloHome = resolveOptionalSoloHomeEnv(process.env.E2E_SOLO_HOME);
  const shouldRemoveSoloHome = !requestedSoloHome && process.env.E2E_KEEP_SOLO_HOME !== "1";
  soloHome = requestedSoloHome ?? (await mkdtemp(path.join(tmpdir(), "solo-e2e-home-")));
  fakeGhBinDir = await createFakeGhBin();
  const metroLineBuffer = createLineBuffer();
  const daemonLineBuffer = createLineBuffer();

  await applySoloHomeFork(soloHome);

  const cleanup = () => performCleanup(shouldRemoveSoloHome);

  try {
    const relayPort = await startRelay(repoRoot);
    metroProcess = startMetro(metroPort, metroLineBuffer);
    daemonProcess = startDaemon({
      port,
      relayPort,
      metroPort,
      repoRoot,
      soloHome,
      fakeGhBinDir,
      buffer: daemonLineBuffer,
    });

    await Promise.all([
      waitForServer(port, {
        label: "Solo daemon",
        childProcess: daemonProcess,
        getRecentOutput: daemonLineBuffer.dump,
      }),
      waitForServer(metroPort, {
        label: "Metro web server",
        timeoutMs: 120000,
        childProcess: metroProcess,
        getRecentOutput: metroLineBuffer.dump,
      }),
    ]);

    const offer = await waitForPairingOfferFromCli({
      repoRoot,
      soloHome,
    });

    process.env.E2E_DAEMON_PORT = String(port);
    process.env.E2E_RELAY_PORT = String(relayPort);
    process.env.E2E_SERVER_ID = offer.serverId;
    process.env.E2E_RELAY_DAEMON_PUBLIC_KEY = offer.daemonPublicKeyB64;
    process.env.E2E_METRO_PORT = String(metroPort);
    console.log(
      `[e2e] Test daemon started on port ${port}, Metro on port ${metroPort}, home: ${soloHome}`,
    );

    return async () => {
      await cleanup();
      console.log("[e2e] Test daemon stopped");
    };
  } catch (error) {
    await cleanup();
    throw error;
  }
}
