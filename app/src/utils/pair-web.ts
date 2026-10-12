import { ConnectionOfferSchema, type ConnectionOffer } from "@server/shared/connection-offer";
import type { HostProfile, RelayHostConnection } from "@/types/host-connection";

/**
 * Support helpers for Web QR login: the web client renders a one-time pairing
 * QR, the phone approves it, and the app hands a ConnectionOffer back over an
 * E2EE relay channel. See docs/design/web-qr-login.md.
 */

export const DEFAULT_PAIR_WEB_RELAY_ENDPOINT = "solo.up2ai.top:443";

/**
 * Relay endpoint the web pairing channel uses before any host is known. The
 * app build can override it (self-hosted relay) via the same env var the host
 * runtime bootstrap uses.
 */
export function resolvePairWebRelayEndpoint(): string {
  const fromEnv = process.env.EXPO_PUBLIC_RELAY_ENDPOINT?.trim();
  return fromEnv && fromEnv.length > 0 ? fromEnv : DEFAULT_PAIR_WEB_RELAY_ENDPOINT;
}

export interface PairWebEligibleHost {
  host: HostProfile;
  connection: RelayHostConnection;
}

/**
 * Only hosts with a relay connection can be shared with a web client: the
 * offer is a bearer credential for relay access, so a LAN-only host is not
 * shareable this way. The preferred connection wins when it is a relay one.
 */
export function selectPairWebEligibleHosts(hosts: HostProfile[]): PairWebEligibleHost[] {
  const eligible: PairWebEligibleHost[] = [];
  for (const host of hosts) {
    const relayConnections = host.connections.filter(
      (connection): connection is RelayHostConnection => connection.type === "relay",
    );
    if (relayConnections.length === 0) {
      continue;
    }
    const preferred = relayConnections.find(
      (connection) => connection.id === host.preferredConnectionId,
    );
    eligible.push({ host, connection: preferred ?? relayConnections[0]! });
  }
  return eligible;
}

export function buildConnectionOfferFromRelayConnection(input: {
  serverId: string;
  connection: RelayHostConnection;
}): ConnectionOffer {
  return ConnectionOfferSchema.parse({
    v: 2,
    serverId: input.serverId,
    daemonPublicKeyB64: input.connection.daemonPublicKeyB64,
    relay: { endpoint: input.connection.relayEndpoint },
  });
}

/** "1:47" style countdown for the QR TTL. */
export function formatPairWebCountdown(totalSeconds: number): string {
  const clamped = Math.max(0, Math.floor(totalSeconds));
  const minutes = Math.floor(clamped / 60);
  const seconds = clamped % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}
