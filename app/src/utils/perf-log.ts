import { isDev } from "@/constants/platform";

/**
 * Dev-only performance logging. Emits `[perf] <event> <json>` lines through
 * console.log so they show up in Metro output and `adb logcat ReactNativeJS`.
 * Compiled to no-ops in production bundles.
 */
export function perfLog(event: string, data?: Record<string, unknown>): void {
  if (!isDev) {
    return;
  }
  console.log(`[perf] ${event}${data ? ` ${JSON.stringify(data)}` : ""}`);
}

/** Monotonic milliseconds. */
export function perfNow(): number {
  return performance.now();
}
