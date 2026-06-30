import { describe, expect, it, vi, afterEach } from "vitest";
import type { Logger } from "../shared/logger.js";
import type { SessionOutboundMessage } from "../shared/messages.js";
import {
  createConnectedClient,
  simulateServerResponse,
} from "./daemon-client-test-harness.js";

function spyLogger(): Logger {
  return { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() };
}

const serverInfoStatus: SessionOutboundMessage = {
  type: "status",
  payload: {
    status: "server_info",
    serverId: "test-server-id",
    hostname: "test-host",
  },
} as SessionOutboundMessage;

afterEach(() => {
  vi.useRealTimers();
});

describe("DaemonClient — error observability for swallowed callbacks", () => {
  it("logs warn when a connection-status listener throws (connection_listener_failed)", async () => {
    const logger = spyLogger();
    const { client, transport, cleanup } = createConnectedClient({ logger });

    client.subscribeConnectionStatus((state) => {
      if (state.status === "disconnected") {
        throw new Error("listener boom");
      }
    });
    transport.simulateClose({ code: 1006, reason: "lost" });

    expect(logger.warn).toHaveBeenCalledWith(
      expect.objectContaining({ err: expect.anything() }),
      "connection_listener_failed",
    );
    await cleanup();
  });

  it("logs warn when a raw-message listener throws (raw_message_listener_failed)", async () => {
    const logger = spyLogger();
    const { client, transport, cleanup } = createConnectedClient({ logger });

    client.subscribeRawMessages(() => {
      throw new Error("raw boom");
    });
    simulateServerResponse(transport, serverInfoStatus);

    expect(logger.warn).toHaveBeenCalledWith(
      expect.objectContaining({ err: expect.anything() }),
      "raw_message_listener_failed",
    );
    await cleanup();
  });

  it("logs warn when a typed message handler throws (message_handler_failed)", async () => {
    const logger = spyLogger();
    const { client, transport, cleanup } = createConnectedClient({ logger });

    client.on("status", () => {
      throw new Error("handler boom");
    });
    simulateServerResponse(transport, serverInfoStatus);

    expect(logger.warn).toHaveBeenCalledWith(
      expect.objectContaining({ err: expect.anything() }),
      "message_handler_failed",
    );
    await cleanup();
  });

  it("logs debug when an inbound frame is not valid JSON (json_parse_failed)", async () => {
    const logger = spyLogger();
    const { transport, cleanup } = createConnectedClient({ logger });

    expect(() => transport.simulateMessage("not-json{")).not.toThrow();

    expect(logger.debug).toHaveBeenCalledWith(
      expect.objectContaining({ err: expect.anything() }),
      "json_parse_failed",
    );
    await cleanup();
  });
});
