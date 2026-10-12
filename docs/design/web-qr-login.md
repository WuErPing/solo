# Web QR Login (pair-web)

> Date: 2026-10-12
> Status: Implemented

## Context

The Expo web build (`expo start --web` / `expo export --platform web`) can already pair with a
daemon via "Direct connection" or "Paste pairing link", but both require the user to copy a URL
from the CLI (`solo daemon pair`) into the browser. Users who already have the mobile app paired
expect the QR-code login pattern instead: the browser shows a QR, the phone scans it, the browser
is signed in.

The design constraint inherited from the existing architecture: the daemon is the trust anchor,
the relay is a zero-knowledge pipe, and the phone is the device the user already trusts. The
approval decision therefore happens on the phone; neither the daemon nor the relay learns that a
web login took place.

## Goals / Non-goals

**Goals**

- Web shows a one-time QR; phone scans → explicit approval → web receives a `ConnectionOffer` and
  connects to the daemon via the normal host flow.
- The relay only ever routes opaque ciphertext (E2EE between browser and phone).
- Zero changes to `daemon/` and `relay-go/`.

**Non-goals**

- No daemon-side pairing tokens or new RPCs — the daemon's trust model (bearer
  `ConnectionOffer` + its keypair) is unchanged.
- No account system; each QR login grants exactly one browser tab access to one daemon.
- No deep-link handling for the `solo://pair-web#` scheme (in-app scanner only; see Follow-ups).

## Protocol

### 1. Pairing request (Web → phone, via QR)

The web client generates a one-time X25519 keypair and a 128-bit `pairingId`, then connects to
the relay and renders the QR:

```
solo://pair-web#<base64url(JSON PairWebRequest)>
```

```json
{
  "v": 1,
  "kind": "pair-web",
  "pairingId": "r2EvLraklrQQzekWJ0f0gg",
  "webPublicKeyB64": "bOgN/gks7YRqVpCtN+CrU4eiFzD639A/88+fwkTyOwY=",
  "relay": { "endpoint": "solo.up2ai.top:443" },
  "exp": 1791816193
}
```

- `exp` = issuance + 120 s (`PAIR_WEB_REQUEST_TTL_SECONDS`). Expired QRs are rejected by the app
  before any network traffic.
- Both sides render `publicKeyFingerprint(webPublicKeyB64)` (first 8 bytes, grouped hex) so the
  user can eyeball that phone and browser share the same ephemeral key.

### 2. Pairing channel (relay, zero-knowledge)

Both ends open `wss://<relay>/ws?serverId=<pairingId>&role=<role>&v=2&connectionId=pair`
(`PAIR_WEB_RELAY_CONNECTION_ID = "pair"`):

- Web connects as `role=server` → its socket becomes the session's *server data socket*.
- App connects as `role=client` → relay forwards its frames to the web socket.

No control socket ever exists for the pairing session; all relay nudge/notify paths are no-ops in
that case. Client frames sent before the server data socket exists are buffered by the relay and
flushed on connect, which removes the connect-order race. The QR TTL (120 s) is well below the
relay's 5-minute read-idle timeout and nginx's 300 s proxy timeout, so no keepalive traffic is
needed.

### 3. Offer delivery (phone → Web, sealed)

The app shows an approval screen (warning copy, fingerprint, TTL countdown, host picker when more
than one relay-paired host exists). On approval it generates its own ephemeral keypair and sends a
single frame:

```json
{
  "v": 1,
  "kind": "pair-web-offer",
  "appPublicKeyB64": "C7q…=",
  "ciphertextB64": "base64([nonce(24)] [XSalsa20-Poly1305 ciphertext])"
}
```

The ciphertext is `X25519(webPublicKey, appEphemeralSecret)` → XSalsa20-Poly1305 over
`JSON.stringify(ConnectionOffer)` — the same NaCl primitives the daemon relay channel uses. The
web side closes the channel after the first envelope that decrypts *and* passes
`ConnectionOfferSchema` validation; everything else is ignored. Closing the server data socket
makes the relay close the app socket, which doubles as the app's delivery confirmation.

### 4. Web sign-in completion

The web client decrypts the envelope, validates the offer, probes the daemon once (same
`connectToDaemon` probe as `pair-scan`), saves the host via
`HostRuntimeStore.upsertConnectionFromOffer`, and navigates to the host workspace.

## Security model

- The `ConnectionOffer` remains a bearer credential; pair-web does not change who may hold one,
  it only changes how the user delivers it. Approval is the phone-side trust decision — same as
  scanning a CLI-printed QR today.
- Everything one-time: `pairingId` (128-bit), web keypair, app ephemeral keypair. The channel
  accepts exactly one valid offer and dies; the QR expires after 120 s.
- The relay sees only: session id (random), frame sizes/timing, and the sealed envelope. It never
  sees offer plaintext, daemon keys, or user identity.
- A guessed `pairingId` (2⁻¹²⁸) lets an attacker push a *sealed* envelope to the browser, but
  without the web secret key they cannot craft a valid one, and the browser discards anything
  that fails decryption/validation. They also cannot read the offer — it is sealed to the web
  public key.
- The app sends nothing except the sealed offer; the web accepts nothing except a valid sealed
  offer. Any other frame is dropped.

## Why no daemon/relay changes were needed

- The relay's `GetOrCreate(serverId)` accepts any server id, and role=server+connectionId data
  sockets are already a first-class relay concept (the daemon uses them per client connection).
- The bearer-offer model means "possession of the offer" *is* the credential, so shipping it
  through a fresh E2EE channel requires no new daemon concept.

## Failure modes

| Failure | Behavior |
|---|---|
| QR older than 120 s | App rejects at scan time; web `waitForOffer` rejects with `PairWebRequestExpiredError` → UI offers "Generate a new code" |
| Channel closed before offer | Web rejects with `PairWebChannelClosedError` → error state + retry |
| Tampered/garbage frames | Ignored by the web side; channel stays open until TTL |
| Daemon unreachable after approval | Web probe fails → error state + retry (host is not saved) |
| Phone has no relay-paired host | Approval screen explains nothing can be shared |

## Implementation map

| Piece | Path |
|---|---|
| Schemas, QR payload codecs, TTL, fingerprint | `app-bridge/src/shared/pair-web-offer.ts` |
| Seal/open + relay channel + web session + app sender | `app-bridge/src/relay/pair-web-channel.ts` |
| Web entry point (primary action on web) | `app/src/components/welcome-screen.tsx` |
| QR display modal (web) | `app/src/components/pair-web-login-modal.tsx` |
| Web login state machine | `app/src/hooks/use-pair-web-login.ts` |
| App scan branch | `app/src/app/pair-scan.tsx` |
| Approval screen (phone) | `app/src/components/pair-web-confirm-panel.tsx` |
| Endpoint/eligible-host/offer helpers | `app/src/utils/pair-web.ts` |
| Opt-in real-relay smoke test (`SOLO_PAIR_WEB_SMOKE=1`) | `app-bridge/src/relay/pair-web-channel.smoke.test.ts` |

The web pairing channel endpoint defaults to `solo.up2ai.top:443` and can be overridden per build
via `EXPO_PUBLIC_RELAY_ENDPOINT` (same variable the host-runtime bootstrap uses).

## Follow-ups

- Deep-link the `solo://pair-web#` scheme so the phone's *system* camera can hand the QR to the
  app (needs a route that replays the payload into the approval screen).
- Surface "Sign in with QR code" in the settings Add-host flow as well as the welcome screen.
- Rate-limit pairing-channel creation on the relay if abuse becomes visible in access logs.
