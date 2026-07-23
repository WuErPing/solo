import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createClientChannel,
  createDaemonChannel,
  type EncryptedChannelEvents,
  type Transport,
} from "./encrypted-channel.js";
import {
  deriveSharedKey,
  encrypt,
  exportPublicKey,
  generateKeyPair,
  type KeyPair,
} from "./crypto.js";
import { arrayBufferToBase64 } from "./base64.js";

const READY = JSON.stringify({ type: "e2ee_ready" });

function hello(keyB64: string): string {
  return JSON.stringify({ type: "e2ee_hello", key: keyB64 });
}

/**
 * Channel message handling resolves over a few microtask hops (await chains
 * around synchronous crypto). Drain them deterministically before asserting
 * on receiver-side observations.
 */
async function settle(hops = 50): Promise<void> {
  for (let i = 0; i < hops; i++) {
    await Promise.resolve();
  }
}

/**
 * Minimal Transport with property-style handlers (as required by the relay
 * Transport interface). `send` delivers synchronously to the paired peer so
 * handshake flows complete within a single call stack.
 */
class TestTransport implements Transport {
  peer: TestTransport | null = null;
  sent: Array<string | ArrayBuffer> = [];
  closeCalls: Array<{ code: number; reason: string }> = [];
  onmessage: ((data: string | ArrayBuffer) => void) | null = null;
  onclose: ((code: number, reason: string) => void) | null = null;
  onerror: ((error: Error) => void) | null = null;

  send(data: string | ArrayBuffer): void {
    this.sent.push(data);
    this.peer?.onmessage?.(data);
  }

  close(code?: number, reason?: string): void {
    this.closeCalls.push({ code: code ?? 1000, reason: reason ?? "" });
  }

  /** Simulates incoming data from the network. */
  receive(data: string | ArrayBuffer): void {
    this.onmessage?.(data);
  }

  simulateClose(code = 1006, reason = "abnormal"): void {
    this.onclose?.(code, reason);
  }

  sentCountOf(type: string): number {
    return this.sent.filter((m) => {
      if (typeof m !== "string") return false;
      try {
        return (JSON.parse(m) as { type?: string }).type === type;
      } catch {
        return false;
      }
    }).length;
  }
}

function createTransportPair(): [TestTransport, TestTransport] {
  const a = new TestTransport();
  const b = new TestTransport();
  a.peer = b;
  b.peer = a;
  return [a, b];
}

function collector(): {
  messages: Array<string | ArrayBuffer>;
  events: EncryptedChannelEvents;
} {
  const messages: Array<string | ArrayBuffer> = [];
  return { messages, events: { onmessage: (data) => messages.push(data) } };
}

/**
 * Drives a daemon channel through a real handshake with a caller-controlled
 * client keypair, so tests can craft ciphertexts for the negotiated key.
 */
async function openDaemonChannel(events: EncryptedChannelEvents = {}) {
  const daemonKeyPair = generateKeyPair();
  const clientKeyPair = generateKeyPair();
  const transport = new TestTransport();
  const promise = createDaemonChannel(transport, daemonKeyPair, events);
  transport.receive(hello(exportPublicKey(clientKeyPair.publicKey)));
  const channel = await promise;
  const sharedKey = deriveSharedKey(clientKeyPair.secretKey, daemonKeyPair.publicKey);
  return { daemonKeyPair, clientKeyPair, transport, channel, sharedKey };
}

/** Completes a full client<->daemon handshake over a paired transport. */
async function openPairedChannels(
  clientEvents: EncryptedChannelEvents = {},
  daemonEvents: EncryptedChannelEvents = {},
) {
  const daemonKeyPair = generateKeyPair();
  const [clientTransport, daemonTransport] = createTransportPair();
  const daemonPromise = createDaemonChannel(daemonTransport, daemonKeyPair, daemonEvents);
  const clientChannel = await createClientChannel(
    clientTransport,
    exportPublicKey(daemonKeyPair.publicKey),
    clientEvents,
  );
  const daemonChannel = await daemonPromise;
  return { daemonKeyPair, clientTransport, daemonTransport, clientChannel, daemonChannel };
}

/** Extracts the client public key the client channel advertised in its hello. */
function clientKeyFromHello(clientTransport: TestTransport): string {
  const raw = clientTransport.sent[0];
  if (typeof raw !== "string") throw new Error("expected hello as first send");
  return (JSON.parse(raw) as { key: string }).key;
}

afterEach(() => {
  vi.useRealTimers();
});

describe("handshake state machine", () => {
  it("completes hello -> ready -> open and exchanges messages both ways", async () => {
    const clientSide = collector();
    const daemonSide = collector();
    let clientOpened = 0;
    let daemonOpened = 0;
    clientSide.events.onopen = () => clientOpened++;
    daemonSide.events.onopen = () => daemonOpened++;

    const { clientTransport, daemonTransport, clientChannel, daemonChannel } =
      await openPairedChannels(clientSide.events, daemonSide.events);

    // The client advertised a 32-byte public key in a plaintext hello.
    const keyB64 = clientKeyFromHello(clientTransport);
    expect(keyB64).toMatch(/^[A-Za-z0-9+/]{43}=$/);
    expect(daemonTransport.sentCountOf("e2ee_ready")).toBe(1);
    expect(clientChannel.isOpen()).toBe(true);
    expect(daemonChannel.isOpen()).toBe(true);
    expect(clientOpened).toBe(1);
    expect(daemonOpened).toBe(1);

    await clientChannel.send("ping");
    await settle();
    expect(daemonSide.messages).toEqual(["ping"]);

    await daemonChannel.send("pong");
    await settle();
    expect(clientSide.messages).toEqual(["pong"]);

    // Post-handshake frames are base64 ciphertext, never plaintext.
    const pingFrame = clientTransport.sent[1];
    expect(typeof pingFrame).toBe("string");
    expect(pingFrame).not.toBe("ping");
  });

  it("delivers binary payloads as ArrayBuffer", async () => {
    const daemonSide = collector();
    const { clientChannel } = await openPairedChannels({}, daemonSide.events);

    const payload = new Uint8Array([0, 1, 2, 0xff, 0xfe]).buffer;
    await clientChannel.send(payload);
    await settle();

    expect(daemonSide.messages).toHaveLength(1);
    const received = daemonSide.messages[0] as ArrayBuffer;
    expect(received).toBeInstanceOf(ArrayBuffer);
    expect(new Uint8Array(received)).toEqual(new Uint8Array([0, 1, 2, 0xff, 0xfe]));
  });

  it("accepts ciphertext delivered as an ArrayBuffer-wrapped base64 text frame", async () => {
    const daemonSide = collector();
    const { transport, sharedKey } = await openDaemonChannel(daemonSide.events);

    const bundle = encrypt(sharedKey, "text frame payload");
    const asTextBuffer = new TextEncoder().encode(arrayBufferToBase64(bundle)).buffer;
    transport.receive(asTextBuffer);
    await settle();

    expect(daemonSide.messages).toEqual(["text frame payload"]);
    expect(transport.closeCalls).toHaveLength(0);
  });

  it("fires onTransitionToOpen callbacks when the handshake completes", async () => {
    vi.useFakeTimers();
    const transport = new TestTransport();
    const channel = await createClientChannel(
      transport,
      exportPublicKey(generateKeyPair().publicKey),
    );
    let fired = 0;
    channel.onTransitionToOpen(() => fired++);

    transport.receive(READY);

    expect(fired).toBe(1);
    expect(channel.isOpen()).toBe(true);
  });
});

describe("hello retry", () => {
  it("re-sends hello every second until the channel opens, then stops", async () => {
    vi.useFakeTimers();
    const transport = new TestTransport(); // peer never answers
    const channel = await createClientChannel(
      transport,
      exportPublicKey(generateKeyPair().publicKey),
    );

    expect(transport.sentCountOf("e2ee_hello")).toBe(1);
    vi.advanceTimersByTime(999);
    expect(transport.sentCountOf("e2ee_hello")).toBe(1);
    vi.advanceTimersByTime(1);
    expect(transport.sentCountOf("e2ee_hello")).toBe(2);
    vi.advanceTimersByTime(3000);
    expect(transport.sentCountOf("e2ee_hello")).toBe(5);

    transport.receive(READY);
    expect(channel.isOpen()).toBe(true);
    vi.advanceTimersByTime(10_000);
    expect(transport.sentCountOf("e2ee_hello")).toBe(5);
  });

  it("stops retrying after the channel is closed", async () => {
    vi.useFakeTimers();
    const transport = new TestTransport();
    const channel = await createClientChannel(
      transport,
      exportPublicKey(generateKeyPair().publicKey),
    );
    vi.advanceTimersByTime(2000);
    expect(transport.sentCountOf("e2ee_hello")).toBe(3);

    channel.close();
    transport.simulateClose(1000, "bye");
    vi.advanceTimersByTime(10_000);
    expect(transport.sentCountOf("e2ee_hello")).toBe(3);
  });

  it("reports send failures via onerror instead of throwing from the timer", async () => {
    vi.useFakeTimers();
    const transport = new TestTransport();
    transport.send = () => {
      throw new Error("socket closing");
    };
    const errors: Error[] = [];
    await createClientChannel(transport, exportPublicKey(generateKeyPair().publicKey), {
      onerror: (error) => errors.push(error),
    });
    vi.advanceTimersByTime(2000);
    expect(errors.length).toBeGreaterThanOrEqual(3);
    expect(errors[0]?.message).toBe("socket closing");
  });
});

describe("daemon handshake validation", () => {
  const newDaemonKeyPair = (): KeyPair => generateKeyPair();

  it.each([
    ["non-JSON text", "this is not json"],
    ["wrong message type", JSON.stringify({ type: "e2ee_ready" })],
    ["missing key", JSON.stringify({ type: "e2ee_hello" })],
    ["empty key", JSON.stringify({ type: "e2ee_hello", key: "  " })],
  ])("rejects %s", async (_label, payload) => {
    const transport = new TestTransport();
    const promise = createDaemonChannel(transport, newDaemonKeyPair());
    transport.receive(payload);
    await expect(promise).rejects.toThrow("Invalid hello message");
  });

  it("rejects a hello with an invalid key length", async () => {
    const transport = new TestTransport();
    const promise = createDaemonChannel(transport, newDaemonKeyPair());
    transport.receive(hello(arrayBufferToBase64(new Uint8Array(16).buffer)));
    await expect(promise).rejects.toThrow("Invalid public key length");
  });

  it("rejects when the transport closes during the handshake", async () => {
    const transport = new TestTransport();
    const promise = createDaemonChannel(transport, newDaemonKeyPair());
    transport.simulateClose(1006, "lost carrier");
    await expect(promise).rejects.toThrow("Connection closed during handshake: 1006 lost carrier");
  });

  it("rejects when the transport errors during the handshake", async () => {
    const transport = new TestTransport();
    const promise = createDaemonChannel(transport, newDaemonKeyPair());
    transport.onerror?.(new Error("boom"));
    await expect(promise).rejects.toThrow("boom");
  });
});

describe("buffered message replay", () => {
  /**
   * Transport that queues inbound frames until a message handler is attached,
   * then flushes them synchronously into the new handler. This models
   * transports that buffer early traffic and exercises the daemon's
   * post-hello buffering path.
   */
  class FlushOnAttachTransport implements Transport {
    queued: Array<string | ArrayBuffer> = [];
    sent: Array<string | ArrayBuffer> = [];
    closeCalls: Array<{ code: number; reason: string }> = [];
    private handler: ((data: string | ArrayBuffer) => void) | null = null;
    onclose: ((code: number, reason: string) => void) | null = null;
    onerror: ((error: Error) => void) | null = null;

    get onmessage(): ((data: string | ArrayBuffer) => void) | null {
      return this.handler;
    }

    set onmessage(value: ((data: string | ArrayBuffer) => void) | null) {
      this.handler = value;
      if (!value) return;
      const pending = this.queued;
      this.queued = [];
      for (const item of pending) value(item);
    }

    send(data: string | ArrayBuffer): void {
      this.sent.push(data);
    }

    close(code?: number, reason?: string): void {
      this.closeCalls.push({ code: code ?? 1000, reason: reason ?? "" });
    }
  }

  it("replays messages buffered during key derivation and skips stray handshake frames", async () => {
    const daemonSide = collector();
    const daemonKeyPair = generateKeyPair();
    const clientKeyPair = generateKeyPair();
    const sharedKey = deriveSharedKey(clientKeyPair.secretKey, daemonKeyPair.publicKey);

    const transport = new FlushOnAttachTransport();
    const promise = createDaemonChannel(transport, daemonKeyPair, daemonSide.events);

    const bundle = encrypt(sharedKey, "buffered-payload");
    transport.queued.push(
      READY, // stray handshake frame: must be skipped on replay
      hello(exportPublicKey(generateKeyPair().publicKey)), // stray re-hello: must be skipped
      arrayBufferToBase64(bundle),
    );

    // Delivering the hello swaps onmessage to the buffering handler, which
    // flushes the queued frames into the buffer before the channel exists.
    transport.onmessage?.(hello(exportPublicKey(clientKeyPair.publicKey)));
    const channel = await promise;
    await settle();

    expect(channel.isOpen()).toBe(true);
    expect(daemonSide.messages).toEqual(["buffered-payload"]);
    // Exactly one ready: the stray buffered hello must not trigger a re-hello.
    expect(transport.sent).toHaveLength(1);
    expect(transport.sent[0]).toBe(READY);
    expect(transport.closeCalls).toHaveLength(0);
  });
});

describe("re-hello handling on an open daemon channel", () => {
  it("re-sends ready without re-keying when the same client retries its hello", async () => {
    const daemonSide = collector();
    const { clientTransport, daemonTransport, clientChannel } = await openPairedChannels(
      {},
      daemonSide.events,
    );
    const originalHello = clientTransport.sent[0] as string;
    expect(daemonTransport.sentCountOf("e2ee_ready")).toBe(1);

    daemonTransport.receive(originalHello);
    await settle();

    expect(daemonTransport.sentCountOf("e2ee_ready")).toBe(2);
    // Shared key unchanged: the original client can still communicate.
    await clientChannel.send("still works");
    await settle();
    expect(daemonSide.messages).toEqual(["still works"]);
  });

  it("re-keys when a different client key arrives", async () => {
    const daemonSide = collector();
    const { daemonKeyPair, daemonTransport } = await openPairedChannels({}, daemonSide.events);

    const newClient = generateKeyPair();
    daemonTransport.receive(hello(exportPublicKey(newClient.publicKey)));
    await settle();

    expect(daemonTransport.sentCountOf("e2ee_ready")).toBe(2);

    const newSharedKey = deriveSharedKey(newClient.secretKey, daemonKeyPair.publicKey);
    const bundle = encrypt(newSharedKey, "from new client");
    daemonTransport.receive(arrayBufferToBase64(bundle));
    await settle();
    expect(daemonSide.messages).toEqual(["from new client"]);
  });
});

describe("plaintext and decryption failures on an open channel", () => {
  it("closes with 1011 when plaintext app traffic arrives (client side)", async () => {
    const { clientTransport } = await openPairedChannels();
    clientTransport.receive(JSON.stringify({ type: "session_message", id: 1 }));
    await settle();
    expect(clientTransport.closeCalls).toEqual([
      { code: 1011, reason: "Received plaintext frame on encrypted channel" },
    ]);
  });

  it("closes with 1011 when plaintext app traffic arrives (daemon side)", async () => {
    const { daemonTransport } = await openPairedChannels();
    daemonTransport.receive(JSON.stringify({ type: "session_message", id: 1 }));
    await settle();
    expect(daemonTransport.closeCalls).toEqual([
      { code: 1011, reason: "Received plaintext frame on encrypted channel" },
    ]);
  });

  it("ignores a stray e2ee_ready while open", async () => {
    const { clientTransport, clientChannel } = await openPairedChannels();
    clientTransport.receive(READY);
    await settle();
    expect(clientTransport.closeCalls).toHaveLength(0);
    expect(clientChannel.isOpen()).toBe(true);
  });

  it("ignores a stray e2ee_hello on a client channel (no daemon key pair)", async () => {
    const { clientTransport, clientChannel } = await openPairedChannels();
    clientTransport.receive(hello(exportPublicKey(generateKeyPair().publicKey)));
    await settle();
    expect(clientTransport.closeCalls).toHaveLength(0);
    expect(clientChannel.isOpen()).toBe(true);
  });

  it("closes with 1011 when decryption fails", async () => {
    const { transport } = await openDaemonChannel();
    const wrongKey = deriveSharedKey(generateKeyPair().secretKey, generateKeyPair().publicKey);
    transport.receive(arrayBufferToBase64(encrypt(wrongKey, "hi")));
    await settle();
    expect(transport.closeCalls).toHaveLength(1);
    expect(transport.closeCalls[0]?.code).toBe(1011);
  });

  it("closes with 1011 on a truncated ciphertext bundle", async () => {
    const { transport } = await openDaemonChannel();
    transport.receive(arrayBufferToBase64(new Uint8Array(10).fill(1).buffer));
    await settle();
    expect(transport.closeCalls).toHaveLength(1);
    expect(transport.closeCalls[0]?.code).toBe(1011);
  });
});

describe("pending send queue", () => {
  it("buffers sends while handshaking and flushes them in order once open", async () => {
    vi.useFakeTimers();
    const daemonSide = collector();
    const daemonKeyPair = generateKeyPair();
    const [clientTransport, daemonTransport] = createTransportPair();

    const clientChannel = await createClientChannel(
      clientTransport,
      exportPublicKey(daemonKeyPair.publicKey),
    );
    void clientChannel.send("first");
    void clientChannel.send("second");

    const daemonPromise = createDaemonChannel(daemonTransport, daemonKeyPair, daemonSide.events);
    vi.advanceTimersByTime(1000); // hello retry reaches the daemon
    await daemonPromise;
    await settle();

    expect(daemonSide.messages).toEqual(["first", "second"]);
  });

  it("drops the oldest entries beyond the 200-message cap", async () => {
    vi.useFakeTimers();
    const daemonSide = collector();
    const daemonKeyPair = generateKeyPair();
    const [clientTransport, daemonTransport] = createTransportPair();

    const clientChannel = await createClientChannel(
      clientTransport,
      exportPublicKey(daemonKeyPair.publicKey),
    );
    for (let i = 0; i < 250; i++) {
      void clientChannel.send(`msg-${i}`);
    }
    const pending = (clientChannel as unknown as { pendingSends: unknown[] }).pendingSends;
    expect(pending).toHaveLength(200);

    const daemonPromise = createDaemonChannel(daemonTransport, daemonKeyPair, daemonSide.events);
    vi.advanceTimersByTime(1000);
    await daemonPromise;
    await settle(1500);

    expect(daemonSide.messages).toHaveLength(200);
    expect(daemonSide.messages[0]).toBe("msg-50");
    expect(daemonSide.messages[199]).toBe("msg-249");
  });

  it("rejects sends on a closed channel", async () => {
    const { clientChannel, clientTransport } = await openPairedChannels();
    clientChannel.close();
    expect(clientTransport.closeCalls).toEqual([{ code: 1000, reason: "Normal closure" }]);
    await expect(clientChannel.send("nope")).rejects.toThrow("Channel not open");
  });
});

describe("close propagation", () => {
  it("marks the channel closed and fires callbacks when the transport closes", async () => {
    const closes: Array<{ code: number; reason: string }> = [];
    let closeCallbackFired = 0;
    const { clientChannel, clientTransport } = await openPairedChannels({
      onclose: (code, reason) => closes.push({ code, reason }),
    });
    clientChannel.onClose(() => closeCallbackFired++);

    clientTransport.simulateClose(1006, "network gone");

    expect(clientChannel.isOpen()).toBe(false);
    expect(closes).toEqual([{ code: 1006, reason: "network gone" }]);
    expect(closeCallbackFired).toBe(1);
    await expect(clientChannel.send("after close")).rejects.toThrow("Channel not open");
  });
});
