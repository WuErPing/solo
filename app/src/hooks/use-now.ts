import { useEffect, useState } from "react";

/** Ticking clock for countdown UIs. */
export function useNowMs(intervalMs = 1000, active = true): number {
  const [nowMs, setNowMs] = useState(() => Date.now());

  useEffect(() => {
    if (!active) {
      return;
    }
    const timer = setInterval(() => setNowMs(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs, active]);

  return nowMs;
}
