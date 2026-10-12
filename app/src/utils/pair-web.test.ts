import { describe, expect, it } from "vitest";

import type { HostProfile, RelayHostConnection } from "@/types/host-connection";
import {
  buildConnectionOfferFromRelayConnection,
  formatPairWebCountdown,
  resolvePairWebRelayEndpoint,
  selectPairWebEligibleHosts,
} from "./pair-web";

function relayConnection(id: string, endpoint: string): RelayHostConnection {
  return {
    id,
    type: "relay",
    relayEndpoint: endpoint,
    daemonPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  };
}

function host(overrides?: Partial<HostProfile>): HostProfile {
  return {
    serverId: "srv_1",
    label: "Workstation",
    lifecycle: {},
    connections: [relayConnection("relay:solo.up2ai.top:443", "solo.up2ai.top:443")],
    preferredConnectionId: "relay:solo.up2ai.top:443",
    createdAt: "2026-10-12T00:00:00.000Z",
    updatedAt: "2026-10-12T00:00:00.000Z",
    ...overrides,
  };
}

describe("resolvePairWebRelayEndpoint", () => {
  it("falls back to the production relay endpoint", () => {
    expect(resolvePairWebRelayEndpoint()).toBe("solo.up2ai.top:443");
  });
});

describe("selectPairWebEligibleHosts", () => {
  it("keeps only hosts with a relay connection", () => {
    const directOnly = host({
      serverId: "srv_local",
      connections: [{ id: "direct:localhost:17612", type: "directTcp", endpoint: "localhost:17612" }],
      preferredConnectionId: "direct:localhost:17612",
    });
    const withRelay = host({ serverId: "srv_remote" });
    expect(selectPairWebEligibleHosts([directOnly, withRelay])).toEqual([
      { host: withRelay, connection: withRelay.connections[0] },
    ]);
  });

  it("prefers the preferred relay connection over the first one", () => {
    const first = relayConnection("relay:a:443", "a:443");
    const second = relayConnection("relay:b:443", "b:443");
    const target = host({ connections: [first, second], preferredConnectionId: second.id });
    expect(selectPairWebEligibleHosts([target])).toEqual([{ host: target, connection: second }]);
  });

  it("falls back to the first relay connection when the preferred one is direct", () => {
    const relay = relayConnection("relay:a:443", "a:443");
    const target = host({
      connections: [{ id: "direct:localhost:17612", type: "directTcp", endpoint: "localhost:17612" }, relay],
      preferredConnectionId: "direct:localhost:17612",
    });
    expect(selectPairWebEligibleHosts([target])).toEqual([{ host: target, connection: relay }]);
  });
});

describe("buildConnectionOfferFromRelayConnection", () => {
  it("builds a schema-valid v2 offer", () => {
    const connection = relayConnection("relay:solo.up2ai.top:443", "solo.up2ai.top:443");
    const offer = buildConnectionOfferFromRelayConnection({ serverId: "srv_1", connection });
    expect(offer).toEqual({
      v: 2,
      serverId: "srv_1",
      daemonPublicKeyB64: connection.daemonPublicKeyB64,
      relay: { endpoint: "solo.up2ai.top:443" },
    });
  });

  it("throws when the connection cannot form a valid offer", () => {
    const connection = { ...relayConnection("relay:a:443", "a:443"), daemonPublicKeyB64: "" };
    expect(() =>
      buildConnectionOfferFromRelayConnection({ serverId: "srv_1", connection }),
    ).toThrow();
  });
});

describe("formatPairWebCountdown", () => {
  it("formats minutes and seconds", () => {
    expect(formatPairWebCountdown(120)).toBe("2:00");
    expect(formatPairWebCountdown(107)).toBe("1:47");
    expect(formatPairWebCountdown(9)).toBe("0:09");
    expect(formatPairWebCountdown(-3)).toBe("0:00");
  });
});
