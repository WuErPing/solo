import { describe, expect, it, vi, afterEach } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
} from "./daemon-client-test-harness.js";

afterEach(() => {
  vi.useRealTimers();
});

function findSentMessage(
  transport: { sentMessages: Array<{ parsed: { type: string; message?: unknown } }> },
  messageType: string,
) {
  return transport.sentMessages.find(
    (m) =>
      m.parsed.type === "session" &&
      (m.parsed as { message?: { type?: string } }).message?.type === messageType,
  );
}

describe("VersionRpc", () => {
  it("listDaemonVersions sends request and resolves with payload", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.versions.listDaemonVersions("req-ver-list");

    const sent = findSentMessage(transport, "list_daemon_versions_request");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "list_daemon_versions_response",
      payload: {
        requestId: "req-ver-list",
        runningVersion: "v0.7.4",
        currentVersion: "solo-v0.7.4",
        versions: [
          { version: "solo-v0.8.0", mtimeMs: 2000 },
          { version: "solo-v0.7.4", mtimeMs: 1000 },
        ],
      },
    });

    const result = await promise;
    expect(result.runningVersion).toBe("v0.7.4");
    expect(result.currentVersion).toBe("solo-v0.7.4");
    expect(result.versions).toHaveLength(2);
    expect(result.versions[0]?.version).toBe("solo-v0.8.0");
    await cleanup();
  });

  it("switchDaemonVersion sends request and resolves on matching status", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.versions.switchDaemonVersion("solo-v0.8.0", "req-ver-switch");

    const sent = findSentMessage(transport, "switch_daemon_version_request");
    expect(sent).toBeDefined();
    expect(
      (sent?.parsed as { message?: { version?: string } }).message?.version,
    ).toBe("solo-v0.8.0");

    // A status for a different request must not resolve the call.
    simulateServerResponse(transport, {
      type: "status",
      payload: {
        status: "daemon_version_switch_requested",
        clientId: "test-client-id",
        version: "solo-v0.8.0",
        requestId: "req-other",
      },
    });

    simulateServerResponse(transport, {
      type: "status",
      payload: {
        status: "daemon_version_switch_requested",
        clientId: "test-client-id",
        version: "solo-v0.8.0",
        requestId: "req-ver-switch",
      },
    });

    const result = await promise;
    expect(result.status).toBe("daemon_version_switch_requested");
    expect(result.version).toBe("solo-v0.8.0");
    await cleanup();
  });

  it("switchDaemonVersion without version omits the field (latest)", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.versions.switchDaemonVersion(undefined, "req-ver-latest");

    const sent = findSentMessage(transport, "switch_daemon_version_request");
    expect(sent).toBeDefined();
    expect(
      (sent?.parsed as { message?: { version?: string } }).message,
    ).not.toHaveProperty("version");

    simulateServerResponse(transport, {
      type: "status",
      payload: {
        status: "daemon_version_switch_requested",
        clientId: "test-client-id",
        version: "solo-v0.8.0",
        requestId: "req-ver-latest",
      },
    });

    await promise;
    await cleanup();
  });
});
