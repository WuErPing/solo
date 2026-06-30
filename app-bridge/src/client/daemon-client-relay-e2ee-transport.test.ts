import { afterEach, describe, expect, it, vi } from "vitest";
import {
  resetDefaultLogger,
  setDefaultLogger,
  type Logger,
} from "../shared/logger.js";
import { createEncryptedTransport } from "./daemon-client-relay-e2ee-transport.js";
import { MockTransport } from "./mock-transport.js";

function spyLogger(): Logger {
  return { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() };
}

describe("createEncryptedTransport — handler error observability", () => {
  afterEach(() => {
    resetDefaultLogger();
  });

  it("logs warn when a consumer handler throws instead of swallowing it (relay_handler_failed)", () => {
    const logger = spyLogger();
    setDefaultLogger(logger);

    const base = new MockTransport();
    const transport = createEncryptedTransport(base, "ZHVtbXkta2V5", {
      warn: vi.fn(),
    });

    transport.onClose(() => {
      throw new Error("close handler boom");
    });

    expect(() => base.simulateClose({ code: 1000, reason: "bye" })).not.toThrow();

    expect(logger.warn).toHaveBeenCalledWith(
      expect.objectContaining({ err: expect.anything() }),
      "relay_handler_failed",
    );
  });
});
