// Renders a PlantUML fence through the daemon (local CLI or a locally
// configured PlantUML server) and caches the normalized SVG per host. Concurrent
// requests for the same diagram share one in-flight RPC; failures are not
// cached so a recovered daemon/config gets a fresh attempt on remount.

import { parsePlantUmlSvg } from "./plantuml-svg";
import {
  getCachedPlantUmlSvg,
  plantUmlCacheKey,
  setCachedPlantUmlSvg,
  type PlantUmlSvgEntry,
} from "./plantuml-svg-cache";

export interface PlantUmlRenderClient {
  plantuml: {
    render(
      source: string,
      dark: boolean,
      requestId?: string,
    ): Promise<{ svg: string; error: string | null }>;
  };
}

export type PlantUmlRenderResult =
  | { ok: true; entry: PlantUmlSvgEntry }
  | { ok: false; error: string };

const inFlight = new Map<string, Promise<PlantUmlRenderResult>>();

export function renderPlantUmlSvg(
  serverId: string,
  client: PlantUmlRenderClient,
  source: string,
  dark: boolean,
): Promise<PlantUmlRenderResult> {
  const cached = getCachedPlantUmlSvg(serverId, dark, source);
  if (cached) {
    return Promise.resolve({ ok: true, entry: cached });
  }

  const key = plantUmlCacheKey(serverId, dark, source);
  const pending = inFlight.get(key);
  if (pending) {
    return pending;
  }

  const task = (async (): Promise<PlantUmlRenderResult> => {
    try {
      const payload = await client.plantuml.render(source, dark);
      if (payload.error) {
        return { ok: false, error: payload.error };
      }
      const entry = parsePlantUmlSvg(payload.svg);
      if (!entry) {
        return { ok: false, error: "PlantUML returned SVG markup that could not be parsed." };
      }
      setCachedPlantUmlSvg(serverId, dark, source, entry);
      return { ok: true, entry };
    } catch (error) {
      return {
        ok: false,
        error: error instanceof Error ? error.message : "PlantUML render request failed.",
      };
    } finally {
      inFlight.delete(key);
    }
  })();

  inFlight.set(key, task);
  return task;
}

/** Test hook: drop pending requests (cache is cleared separately). */
export function clearPlantUmlInFlight(): void {
  inFlight.clear();
}
