import { describe, expect, it } from "vitest";

import type { ConnectionOffer } from "../shared/connection-offer.js";
import {
  extractPairWebRequestFromQrContent,
  isPairWebRequestExpired,
  type PairWebRequest,
} from "../shared/pair-web-offer.js";
import {
  PairWebChannelClosedError,
  PairWebRequestExpiredError,
  connectPairWebChannel,
  createPairWebHostSession,
  generatePairingId,
  openPairWebOffer,
  sealPairWebOffer,
  sendPairWebOffer,
  type PairWebSocket,
  type PairWebSocketFactory,
  type PairWebSocketHandlers,
} from "./pair-web-channel.js";
import { exportPublicKey, generateKeyPair } from "./crypto.js";

function sampleOffer(): ConnectionOffer {
  return {
    v: 2,
    serverId: "srv_test_123",
    daemonPublicKeyB64: exportPublicKey(generateKeyPair().publicKey),
    relay: { endpoint: "solo.up2ai.top:443" },
  };
}

function sampleRequest(overrides?: Partial<PairWebRequest>): PairWebRequest {
  return {
    v: 1,
    kind: "pair-web",
    pairingId: generatePairingId(),
    webPublicKeyB64: exportPublicKey(generateKeyPair().publicKey),
    relay: { endpoint: "127.0.0.1:18089" },
    exp: Math.floor(Date.now() / 1000) + 120,
    ...overrides,
  };
}

describe("sealPairWebOffer / openPairWebOffer", () => {
  it("roundtrips an offer end to end", () => {
    const webKeyPair = generateKeyPair();
    const request = sampleRequest({ webPublicKeyB64: exportPublicKey(webKeyPair.publicKey) });
    const offer = sampleOffer();

    const envelope = sealPairWebOffer({ request, offer });
    expect(envelope.kind).toBe("pair-web-offer");
    expect(envelope.ciphertextB64).not.toContain(offer.serverId);

    const opened = openPairWebOffer({ webSecretKey: webKeyPair.secretKey, envelope });
    expect(opened).toEqual(offer);
  });

  it("produces a different ciphertext for each seal (random nonce + ephemeral key)", () => {
    const webKeyPair = generateKeyPair();
    const request = sampleRequest({ webPublicKeyB64: exportPublicKey(webKeyPair.publicKey) });
    const offer = sampleOffer();
    const first = sealPairWebOffer({ request, offer });
    const second = sealPairWebOffer({ request, offer });
    expect(first.ciphertextB64).not.toBe(second.ciphertextB64);
    expect(first.appPublicKeyB64).not.toBe(second.appPublicKeyB64);
  });

  it("rejects a tampered ciphertext", () => {
    const webKeyPair = generateKeyPair();
    const request = sampleRequest({ webPublicKeyB64: exportPublicKey(webKeyPair.publicKey) });
    const envelope = sealPairWebOffer({ request, offer: sampleOffer() });
    const tampered = {
      ...envelope,
      ciphertextB64: `${envelope.ciphertextB64.slice(0, -4)}AAAA`,
    };
    expect(() => openPairWebOffer({ webSecretKey: webKeyPair.secretKey, envelope: tampered })).toThrow();
  });

  it("rejects decryption with the wrong web secret key", () => {
    const request = sampleRequest();
    const envelope = sealPairWebOffer({ request, offer: sampleOffer() });
    const wrongKeyPair = generateKeyPair();
    expect(() =>
      openPairWebOffer({ webSecretKey: wrongKeyPair.secretKey, envelope }),
    ).toThrow();
  });

  it("rejects envelopes that do not match the schema", () => {
    const webKeyPair = generateKeyPair();
    expect(() =>
      openPairWebOffer({ webSecretKey: webKeyPair.secretKey, envelope: { kind: "offer" } }),
    ).toThrow();
  });
});

// ---------------------------------------------------------------------------
// Channel flow over an in-memory relay double
// ---------------------------------------------------------------------------

interface MockPeer {
  handlers: PairWebSocketHandlers;
  sent: string[];
  closed: boolean;
  closeCode: number | null;
}

function createMockRelay() {
  const peers: { server: MockPeer | null; client: MockPeer | null } = { server: null, client: null };

  const makeFactory =
    (role: "server" | "client"): PairWebSocketFactory =>
    (_url, handlers) => {
      const peer: MockPeer = { handlers, sent: [], closed: false, closeCode: null };
      peers[role] = peer;
      queueMicrotask(() => handlers.onOpen());
      const socket: PairWebSocket = {
        send: (data) => {
          peer.sent.push(data);
          // Relay forwards client frames to the server data socket.
          if (role === "client" && peers.server && !peers.server.closed) {
            const server = peers.server;
            queueMicrotask(() => server.handlers.onMessage(data));
          }
        },
        close: (code = 1000, _reason = "") => {
          if (peer.closed) return;
          peer.closed = true;
          peer.closeCode = code;
          // Relay closes client sockets when the server data socket goes away.
          if (role === "server" && peers.client && !peers.client.closed) {
            const client = peers.client;
            client.closed = true;
            queueMicrotask(() => client.handlers.onClose(1012, "Server disconnected"));
          }
        },
      };
      return socket;
    };

  return { peers, serverFactory: makeFactory("server"), clientFactory: makeFactory("client") };
}

describe("pair-web channel over relay", () => {
  it("delivers a sealed offer from app to web and closes the channel", async () => {
    const relay = createMockRelay();
    const session = await createPairWebHostSession({
      relayEndpoint: "127.0.0.1:18089",
      socketFactory: relay.serverFactory,
    });
    expect(session.request.kind).toBe("pair-web");
    expect(extractPairWebRequestFromQrContent(session.qrContent)).toEqual(session.request);
    expect(isPairWebRequestExpired(session.request)).toBe(false);

    const offer = sampleOffer();
    const waitPromise = session.waitForOffer();
    await sendPairWebOffer({
      request: session.request,
      offer,
      socketFactory: relay.clientFactory,
      confirmTimeoutMs: 500,
    });

    await expect(waitPromise).resolves.toEqual(offer);
    expect(relay.peers.server?.closed).toBe(true);
    expect(relay.peers.client?.closed).toBe(true);

    // The relay only ever sees the sealed envelope — never offer fields.
    const wireText = relay.peers.client?.sent.join("\n") ?? "";
    expect(wireText).not.toContain(offer.serverId);
    expect(wireText).not.toContain(offer.daemonPublicKeyB64);
  });

  it("ignores malformed messages and keeps waiting for a valid offer", async () => {
    const relay = createMockRelay();
    const session = await createPairWebHostSession({
      relayEndpoint: "127.0.0.1:18089",
      socketFactory: relay.serverFactory,
    });
    const offer = sampleOffer();
    const waitPromise = session.waitForOffer();

    const clientChannel = await connectPairWebChannel({
      relayEndpoint: "127.0.0.1:18089",
      pairingId: session.request.pairingId,
      role: "client",
      socketFactory: relay.clientFactory,
    });
    clientChannel.sendJson({ hello: "world" });
    clientChannel.sendJson({ v: 1, kind: "pair-web-offer", appPublicKeyB64: "bad", ciphertextB64: "bad" });
    clientChannel.sendJson(sealPairWebOffer({ request: session.request, offer }));
    clientChannel.close();

    await expect(waitPromise).resolves.toEqual(offer);
  });

  it("rejects waitForOffer once the request expires", async () => {
    const relay = createMockRelay();
    const session = await createPairWebHostSession({
      relayEndpoint: "127.0.0.1:18089",
      socketFactory: relay.serverFactory,
      nowMs: Date.now() - 200_000,
    });
    await expect(session.waitForOffer()).rejects.toBeInstanceOf(PairWebRequestExpiredError);
  });

  it("rejects waitForOffer when the channel closes before any offer", async () => {
    const relay = createMockRelay();
    const session = await createPairWebHostSession({
      relayEndpoint: "127.0.0.1:18089",
      socketFactory: relay.serverFactory,
    });
    const waitPromise = session.waitForOffer();
    session.close();
    // The mock relay does not echo a server-initiated close back to the server
    // socket itself; simulate the relay's close notification.
    relay.peers.server?.handlers.onClose(1000, "Normal closure");
    await expect(waitPromise).rejects.toBeInstanceOf(PairWebChannelClosedError);
  });

  it("refuses to send an offer for an expired request", async () => {
    const request = sampleRequest({ exp: Math.floor(Date.now() / 1000) - 5 });
    await expect(
      sendPairWebOffer({ request, offer: sampleOffer(), socketFactory: createMockRelay().clientFactory }),
    ).rejects.toBeInstanceOf(PairWebRequestExpiredError);
  });

  it("rejects offers that fail ConnectionOffer validation before connecting", async () => {
    const request = sampleRequest();
    await expect(
      sendPairWebOffer({
        request,
        offer: { v: 1 } as unknown as ConnectionOffer,
        socketFactory: createMockRelay().clientFactory,
      }),
    ).rejects.toThrow();
  });
});

describe("generatePairingId", () => {
  it("is 128 bits of base64url entropy and unique per call", () => {
    const first = generatePairingId();
    const second = generatePairingId();
    expect(first).toMatch(/^[A-Za-z0-9_-]{22}$/);
    expect(second).toMatch(/^[A-Za-z0-9_-]{22}$/);
    expect(first).not.toBe(second);
  });
});
