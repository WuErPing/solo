import { useCallback, useEffect, useState } from "react";
import {
  PairWebChannelClosedError,
  PairWebRequestExpiredError,
  createPairWebHostSession,
  type PairWebHostSession,
} from "@server/relay/pair-web-channel";
import { useHostMutations } from "@/runtime/host-runtime";
import { useStableEvent } from "@/hooks/use-stable-event";
import { connectToDaemon } from "@/utils/test-daemon-connection";
import { normalizeHostPort } from "@/utils/daemon-endpoints";
import { resolvePairWebRelayEndpoint } from "@/utils/pair-web";

export type PairWebLoginStatus =
  | "preparing"
  | "waiting"
  | "verifying"
  | "success"
  | "expired"
  | "error";

export interface PairWebLoginState {
  status: PairWebLoginStatus;
  /** `solo://pair-web#…` QR content, once the relay channel is live. */
  qrContent: string | null;
  /** Web ephemeral public key fingerprint — the phone shows the same value. */
  fingerprint: string | null;
  expiresAtMs: number | null;
  errorMessage: string | null;
  /** Discards the current QR and starts a fresh pairing channel. */
  restart: () => void;
}

interface PairWebLoginView {
  status: PairWebLoginStatus;
  qrContent: string | null;
  fingerprint: string | null;
  expiresAtMs: number | null;
  errorMessage: string | null;
}

const PREPARING_VIEW: PairWebLoginView = {
  status: "preparing",
  qrContent: null,
  fingerprint: null,
  expiresAtMs: null,
  errorMessage: null,
};

function toLoginErrorMessage(error: unknown): string {
  if (error instanceof PairWebChannelClosedError) {
    return "The pairing channel closed unexpectedly. Generate a new QR code and try again.";
  }
  return error instanceof Error ? error.message : "Unable to complete QR sign-in";
}

/**
 * Drives the web side of QR login: create the one-time pairing channel,
 * surface the QR content, wait for the phone to approve, then register the
 * received offer as a host and hand off via `onPaired`.
 */
export function usePairWebLogin(input: {
  active: boolean;
  onPaired: (serverId: string) => void;
}): PairWebLoginState {
  const { active } = input;
  const onPaired = useStableEvent(input.onPaired);
  const { upsertConnectionFromOffer } = useHostMutations();
  const [restartToken, setRestartToken] = useState(0);
  const [view, setView] = useState<PairWebLoginView>(PREPARING_VIEW);

  useEffect(() => {
    if (!active) {
      return;
    }
    let cancelled = false;
    let session: PairWebHostSession | null = null;

    const run = async (): Promise<void> => {
      try {
        const current = await createPairWebHostSession({
          relayEndpoint: resolvePairWebRelayEndpoint(),
        });
        session = current;
        if (cancelled) {
          current.close();
          return;
        }
        setView({
          status: "waiting",
          qrContent: current.qrContent,
          fingerprint: current.fingerprint,
          expiresAtMs: current.expiresAtMs,
          errorMessage: null,
        });

        const offer = await current.waitForOffer();
        if (cancelled) {
          return;
        }
        setView((previous) => ({ ...previous, status: "verifying", errorMessage: null }));

        // Probe before saving: proves the daemon is reachable and yields its
        // hostname for the host label (mirrors the pair-scan flow).
        const { client, hostname } = await connectToDaemon(
          {
            id: "probe",
            type: "relay",
            relayEndpoint: normalizeHostPort(offer.relay.endpoint),
            daemonPublicKeyB64: offer.daemonPublicKeyB64,
          },
          { serverId: offer.serverId },
        );
        await client.close().catch(() => undefined);
        if (cancelled) {
          return;
        }
        await upsertConnectionFromOffer(offer, hostname ?? undefined);
        if (cancelled) {
          return;
        }
        setView((previous) => ({ ...previous, status: "success" }));
        onPaired(offer.serverId);
      } catch (error) {
        if (cancelled) {
          return;
        }
        if (error instanceof PairWebRequestExpiredError) {
          setView((previous) => ({ ...previous, status: "expired" }));
          return;
        }
        setView((previous) => ({
          ...previous,
          status: "error",
          errorMessage: toLoginErrorMessage(error),
        }));
      }
    };

    void run();

    return () => {
      cancelled = true;
      session?.close();
    };
  }, [active, restartToken, onPaired, upsertConnectionFromOffer]);

  const restart = useCallback(() => {
    setView(PREPARING_VIEW);
    setRestartToken((token) => token + 1);
  }, []);

  return { ...view, restart };
}
