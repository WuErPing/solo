/**
 * Shared logging strategy for app-bridge.
 *
 * A single structured logger that any module (including free functions and
 * helper modules that hold no DaemonClient instance) can import, so swallowed
 * errors always produce a signal instead of disappearing into `catch {}`.
 *
 * Hosts (app / cli) can route bridge logs into their own pipeline via
 * `setDefaultLogger`.
 */
export interface Logger {
  debug(obj: object, msg?: string): void;
  info(obj: object, msg?: string): void;
  warn(obj: object, msg?: string): void;
  error(obj: object, msg?: string): void;
}

/** Console-backed default. Debug is intentionally a no-op unless overridden. */
export const consoleLogger: Logger = {
  debug: () => {
    /* no-op: debug disabled by default */
  },
  info: (obj, msg) => console.log(msg, obj),
  warn: (obj, msg) => console.warn(msg, obj),
  error: (obj, msg) => console.error(msg, obj),
};

let activeLogger: Logger = consoleLogger;

/** Returns the logger that free functions should log through. */
export function getDefaultLogger(): Logger {
  return activeLogger;
}

/** Routes app-bridge module-level logs into a host-provided logger. */
export function setDefaultLogger(logger: Logger): void {
  activeLogger = logger;
}

/** Restores the built-in console logger. Primarily for tests. */
export function resetDefaultLogger(): void {
  activeLogger = consoleLogger;
}

/** Normalizes a caught value into a structured shape for logging. */
export function toErrorInfo(error: unknown): { name: string; message: string } {
  if (error instanceof Error) {
    return { name: error.name, message: error.message };
  }
  return { name: "NonError", message: String(error) };
}
