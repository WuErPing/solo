import { afterEach, describe, expect, it, vi } from "vitest";
import {
  resetDefaultLogger,
  setDefaultLogger,
  type Logger,
} from "../shared/logger.js";
import { createWebSocketTransportFactory } from "./daemon-client-websocket-transport.js";
import type { WebSocketLike } from "./daemon-client-transport-types.js";

function spyLogger(): Logger {
  return { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() };
}

describe("createWebSocketTransportFactory", () => {
  afterEach(() => {
    resetDefaultLogger();
  });

  it("returns the transport even when the WebSocket binaryType setter throws", () => {
    const logger = spyLogger();
    setDefaultLogger(logger);

    const factory = vi.fn((_url: string) => {
      const ws: WebSocketLike = {
        readyState: 1,
        send: vi.fn(),
        close: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        set binaryType(_value: string) {
          throw new Error("unsupported");
        },
        get binaryType() {
          return "";
        },
      };
      return ws;
    });

    const create = createWebSocketTransportFactory(factory);
    const transport = create({ url: "ws://localhost:17612" });

    expect(transport).toBeDefined();
    expect(factory).toHaveBeenCalled();
    expect(logger.debug).toHaveBeenCalledWith(
      expect.objectContaining({ err: expect.anything() }),
      "ws_binarytype_unsupported",
    );
  });
});
