import { afterEach, describe, expect, it, vi } from "vitest";
import {
  consoleLogger,
  getDefaultLogger,
  resetDefaultLogger,
  setDefaultLogger,
  toErrorInfo,
  type Logger,
} from "./logger.js";

function createSpyLogger(): Logger {
  return {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  };
}

describe("shared logger", () => {
  afterEach(() => {
    resetDefaultLogger();
  });

  it("exposes the four structured log methods on the default logger", () => {
    const logger = getDefaultLogger();
    for (const method of ["debug", "info", "warn", "error"] as const) {
      expect(typeof logger[method]).toBe("function");
    }
  });

  it("defaults to the console logger", () => {
    expect(getDefaultLogger()).toBe(consoleLogger);
  });

  it("routes log calls to an injected logger", () => {
    const spy = createSpyLogger();
    setDefaultLogger(spy);

    getDefaultLogger().warn({ a: 1 }, "boom");

    expect(spy.warn).toHaveBeenCalledWith({ a: 1 }, "boom");
    expect(getDefaultLogger()).toBe(spy);
  });

  it("restores the console logger on reset", () => {
    setDefaultLogger(createSpyLogger());
    resetDefaultLogger();
    expect(getDefaultLogger()).toBe(consoleLogger);
  });

  it("normalizes Error and non-Error values for structured logging", () => {
    expect(toErrorInfo(new TypeError("bad"))).toEqual({
      name: "TypeError",
      message: "bad",
    });
    expect(toErrorInfo("plain string")).toEqual({
      name: "NonError",
      message: "plain string",
    });
  });
});
