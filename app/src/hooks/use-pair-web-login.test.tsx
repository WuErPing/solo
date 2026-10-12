/**
 * @vitest-environment jsdom
 */
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ConnectionOffer } from "@server/shared/connection-offer";
import {
  PairWebRequestExpiredError,
  type PairWebHostSession,
} from "@server/relay/pair-web-channel";
import { usePairWebLogin } from "./use-pair-web-login";

const {
  mockCreateSession,
  mockConnectToDaemon,
  mockUpsertConnectionFromOffer,
  sessionControls,
} = vi.hoisted(() => {
  const controls = {
    nextSession: null as PairWebHostSession | null,
    createError: null as Error | null,
  };
  return {
    sessionControls: controls,
    mockCreateSession: vi.fn(async () => {
      if (controls.createError) {
        throw controls.createError;
      }
      if (!controls.nextSession) {
        throw new Error("test must provide sessionControls.nextSession");
      }
      return controls.nextSession;
    }),
    mockConnectToDaemon: vi.fn(),
    mockUpsertConnectionFromOffer: vi.fn(),
  };
});

vi.mock("@server/relay/pair-web-channel", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@server/relay/pair-web-channel")>();
  return { ...actual, createPairWebHostSession: mockCreateSession };
});

vi.mock("@/utils/test-daemon-connection", () => ({
  connectToDaemon: mockConnectToDaemon,
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostMutations: () => ({ upsertConnectionFromOffer: mockUpsertConnectionFromOffer }),
}));

function makeSession(input?: {
  offer?: ConnectionOffer;
  waitError?: Error;
  /** When set, waitForOffer only settles once the test resolves/rejects it. */
  manual?: { resolve?: (offer: ConnectionOffer) => void; reject?: (error: Error) => void };
}): PairWebHostSession {
  const offer: ConnectionOffer = input?.offer ?? {
    v: 2,
    serverId: "srv_test",
    daemonPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    relay: { endpoint: "solo.up2ai.top:443" },
  };
  return {
    request: {
      v: 1,
      kind: "pair-web",
      pairingId: "pairing-1",
      webPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      relay: { endpoint: "solo.up2ai.top:443" },
      exp: Math.floor(Date.now() / 1000) + 120,
    },
    qrContent: "solo://pair-web#cGF5bG9hZA",
    fingerprint: "0000 0000 0000 0000",
    expiresAtMs: Date.now() + 120_000,
    waitForOffer: vi.fn(
      () =>
        new Promise<ConnectionOffer>((resolve, reject) => {
          if (input?.manual) {
            input.manual.resolve = resolve;
            input.manual.reject = reject;
            return;
          }
          if (input?.waitError) {
            reject(input.waitError);
            return;
          }
          resolve(offer);
        }),
    ),
    close: vi.fn(),
  };
}

describe("usePairWebLogin", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionControls.nextSession = null;
    sessionControls.createError = null;
  });

  it("stays idle while inactive", () => {
    const { result } = renderHook(() => usePairWebLogin({ active: false, onPaired: vi.fn() }));
    expect(result.current.status).toBe("preparing");
    expect(result.current.qrContent).toBeNull();
    expect(mockCreateSession).not.toHaveBeenCalled();
  });

  it("surfaces the QR content then completes pairing on offer receipt", async () => {
    const offer: ConnectionOffer = {
      v: 2,
      serverId: "srv_real",
      daemonPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      relay: { endpoint: "solo.up2ai.top:443" },
    };
    const manual: { resolve?: (offer: ConnectionOffer) => void } = {};
    sessionControls.nextSession = makeSession({ manual });
    mockConnectToDaemon.mockResolvedValue({
      client: { close: vi.fn(async () => undefined) },
      serverId: "srv_real",
      hostname: "workstation",
    });
    mockUpsertConnectionFromOffer.mockResolvedValue({ serverId: "srv_real" });
    const onPaired = vi.fn();

    const { result } = renderHook(() => usePairWebLogin({ active: true, onPaired }));

    await waitFor(() => expect(result.current.status).toBe("waiting"));
    expect(result.current.qrContent).toBe("solo://pair-web#cGF5bG9hZA");
    expect(result.current.fingerprint).toBe("0000 0000 0000 0000");

    manual.resolve?.(offer);

    await waitFor(() => expect(result.current.status).toBe("success"));
    expect(mockConnectToDaemon).toHaveBeenCalledWith(
      {
        id: "probe",
        type: "relay",
        relayEndpoint: "solo.up2ai.top:443",
        daemonPublicKeyB64: offer.daemonPublicKeyB64,
      },
      { serverId: "srv_real" },
    );
    expect(mockUpsertConnectionFromOffer).toHaveBeenCalledWith(offer, "workstation");
    expect(onPaired).toHaveBeenCalledWith("srv_real");
  });

  it("maps request expiry to the expired state and supports restart", async () => {
    sessionControls.nextSession = makeSession({ waitError: new PairWebRequestExpiredError() });
    const { result } = renderHook(() => usePairWebLogin({ active: true, onPaired: vi.fn() }));

    await waitFor(() => expect(result.current.status).toBe("expired"));
    expect(mockCreateSession).toHaveBeenCalledTimes(1);

    sessionControls.nextSession = makeSession();
    mockConnectToDaemon.mockResolvedValue({
      client: { close: vi.fn(async () => undefined) },
      serverId: "srv_test",
      hostname: null,
    });
    mockUpsertConnectionFromOffer.mockResolvedValue({ serverId: "srv_test" });

    result.current.restart();
    await waitFor(() => expect(mockCreateSession).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.status).toBe("success"));
  });

  it("surfaces probe failures as an error state", async () => {
    sessionControls.nextSession = makeSession();
    mockConnectToDaemon.mockRejectedValue(new Error("Connection timed out"));

    const { result } = renderHook(() => usePairWebLogin({ active: true, onPaired: vi.fn() }));

    await waitFor(() => expect(result.current.status).toBe("error"));
    expect(result.current.errorMessage).toBe("Connection timed out");
    expect(mockUpsertConnectionFromOffer).not.toHaveBeenCalled();
  });

  it("closes the session when deactivated mid-wait", async () => {
    const manual: { resolve?: (offer: ConnectionOffer) => void } = {};
    const session = makeSession({ manual });
    sessionControls.nextSession = session;

    const { result, rerender } = renderHook(
      ({ active }) => usePairWebLogin({ active, onPaired: vi.fn() }),
      { initialProps: { active: true } },
    );
    await waitFor(() => expect(result.current.status).toBe("waiting"));

    rerender({ active: false });
    await waitFor(() => expect(session.close).toHaveBeenCalled());
  });
});
