import { parseHostPort } from "@server/shared/daemon-endpoints";
import type { ActiveConnection } from "@/runtime/host-runtime";

export interface ResolvedWorkspaceScriptLink {
  openUrl: string | null;
  labelUrl: string | null;
}

interface ScriptLike {
  type?: string;
  lifecycle: string;
  port?: number | null;
  proxyUrl?: string | null;
}

function isLoopbackHost(host: string): boolean {
  const normalizedHost = host.trim().toLowerCase();
  return (
    normalizedHost === "localhost" || normalizedHost === "127.0.0.1" || normalizedHost === "::1"
  );
}

function buildDirectServiceUrl(endpoint: string, port: number): string | null {
  try {
    const { host, isIpv6 } = parseHostPort(endpoint);
    const base = isIpv6 ? `[${host}]` : host;
    return `http://${base}:${port}`;
  } catch {
    return null;
  }
}

export function resolveWorkspaceScriptLink(input: {
  script: ScriptLike;
  activeConnection: ActiveConnection | null;
}): ResolvedWorkspaceScriptLink {
  const { script, activeConnection } = input;
  if (script.type !== "service" || script.lifecycle !== "running") {
    return { openUrl: null, labelUrl: null };
  }

  if (!activeConnection) {
    return { openUrl: null, labelUrl: script.proxyUrl ?? null };
  }

  if (activeConnection.type === "relay") {
    return { openUrl: null, labelUrl: script.proxyUrl ?? null };
  }

  if (activeConnection.type === "directSocket" || activeConnection.type === "directPipe") {
    return { openUrl: script.proxyUrl ?? null, labelUrl: script.proxyUrl ?? null };
  }

  try {
    const { host } = parseHostPort(activeConnection.endpoint);
    if (isLoopbackHost(host)) {
      return { openUrl: script.proxyUrl ?? null, labelUrl: script.proxyUrl ?? null };
    }
  } catch {
    return { openUrl: null, labelUrl: script.proxyUrl ?? null };
  }

  if (script.port == null) {
    return { openUrl: null, labelUrl: script.proxyUrl ?? null };
  }

  const directUrl = buildDirectServiceUrl(activeConnection.endpoint, script.port);
  return {
    openUrl: directUrl,
    labelUrl: directUrl ?? script.proxyUrl ?? null,
  };
}
