# Push Notification Architecture

## Overview

Solo uses **Expo Push API** as the unified push gateway. Expo abstracts platform differences and routes to APNs (iOS) or FCM (Android) based on the token type. The daemon is the sole sender; the mobile app only registers its token.

---

## Token Dependency Chain

`getExpoPushTokenAsync` performs a two-layer token mapping. The Expo server acts
as a middleman between the device native token and the sender.

```
Android device
  └─ FCM SDK registers with Google
       └─ Native FCM token: "fXg3kL9..."
            │
            └─ Expo SDK POSTs to Expo servers
                 https://exp.host/--/api/v2/push/getExpoPushToken
                 body: { deviceToken: "fXg3kL9...", projectId: "72c50384..." }
                      │
                      └─ Expo stores mapping:
                           ExponentPushToken[xxxx]  →  FCM: "fXg3kL9..."
                         Returns ExponentPushToken[xxxx] to the app
```

At send time, the daemon posts `ExponentPushToken[xxxx]` to Expo. Expo looks up
the mapping and calls Google FCM or Apple APNs using its own service credentials.

### Dependency layers

| Layer | Replaceable? | Notes |
|-------|-------------|-------|
| Google FCM | No | Android native push infrastructure; mandatory |
| Apple APNs | No | iOS native push infrastructure; mandatory |
| Expo servers | **Yes — can bypass entirely** | See alternative below |
| `expo-notifications` SDK | Partial | Provides both APIs; keep the SDK, skip Expo servers |

### Bypassing Expo servers

`expo-notifications` exposes a second API that returns the raw native token
directly, without contacting Expo servers:

```ts
// Current: routed through Expo servers
const { data } = await Notifications.getExpoPushTokenAsync({ projectId })
// → "ExponentPushToken[xxxx]"

// Alternative: raw native token, no Expo server involved
const { data } = await Notifications.getDevicePushTokenAsync()
// Android → { type: "fcm",  data: "fXg3kL9..." }
// iOS     → { type: "apns", data: "abc123..." }
```

If the raw token is used, the daemon (`push/service.go`) must:

- Distinguish `fcm` vs `apns` token types
- Hold a FCM Service Account Key and call Google FCM HTTP v1 API directly
- Hold an APNs certificate/key and call APNs HTTP/2 directly
- Re-implement batching, retry, and invalid-token cleanup per platform

The current Expo-based approach offloads all of that to one unified HTTP
endpoint at the cost of a runtime dependency on Expo's cloud service.

---

## Phase 1: Token Registration (App → Daemon, one-time per device)

```
Solo App starts
  └─ usePushTokenRegistration hook
       │   app/src/hooks/use-push-token-registration.ts
       │
       ├─ expo-notifications.getExpoPushTokenAsync({ projectId: "72c50384-..." })
       │     Expo SDK contacts Expo servers, which exchange platform credentials
       │     (google-services.json for FCM / GoogleService-Info.plist for APNs)
       │     and return:  ExponentPushToken[xxxxxxxxxxxxxxxxxxxxxx]
       │
       ├─ Cache token in AsyncStorage
       │     key: @solo:expo-push-token:<serverId>
       │
       └─ WebSocket message → Daemon
             { type: "register_push_token", token: "ExponentPushToken[...]" }
               │
               └─ Session.handleRegisterPushToken()
                    daemon/internal/server/session.go:787
                    └─ PersistedTokenStore.Register(token)
                         deduplicates, then writes atomically to
                         ~/.solo/push-tokens.json
```

Token persists across daemon restarts. If the app reconnects it re-sends the
same token (deduplicated), so no stale registrations accumulate.

---

## Phase 2: Agent Triggers a Push (runtime)

```
Agent completes / needs attention
  └─ agent/manager.go emits attention_required event
       reason: "finished" | "permission" | "error"
         │
         └─ Session.broadcastAgentAttention(agentID, reason)
              daemon/internal/server/attention.go:14
              │
              ├─ [1] Build payload
              │       push.BuildAttentionNotification()
              │       daemon/internal/push/notification.go
              │
              │       reason      title                   body
              │       ─────────── ─────────────────────── ─────────────────────────────────
              │       "finished"  "Agent finished"         last assistant message (≤220 chars)
              │       "permission""Agent needs permission" "Permission requested."
              │       "error"     "Agent needs attention"  "Encountered an error."
              │
              │       data: { agentId, workspaceId, ... }  (used for deep-link on tap)
              │
              ├─ [2] Decide whether to push
              │       ComputeNotificationPlan(clientStates, agentID, reason, nowMs)
              │       daemon/internal/server/attention_policy.go
              │
              │       Client state                              → Action
              │       ──────────────────────────────────────── → ──────────────────
              │       App visible + focused on this agent       → suppress all
              │       App visible + active (within 3 min)       → in-app only
              │       No clients online OR all backgrounded     → ShouldPush = true
              │       reason == "error"                         → ShouldPush = false
              │
              └─ [3] Send if ShouldPush == true
                      tokens := pushTokenStore.GetAll()
                      go pusher.Send(tokens, notification)
                           daemon/internal/push/service.go
                           HTTP POST https://exp.host/--/api/v2/push/send
                           body: [{ to, title, body, data, sound: "default" }]
                           batched at 100 tokens/request
                           retried up to 3× with exponential back-off (1s, 2s, 4s)
```

---

## Phase 3: Expo → Platform → Device

```
Expo Push servers
  └─ Android token  →  Google FCM API  →  Android OS notification tray
  └─ iOS token      →  Apple APNs      →  iOS notification center
```

The notification icon and color on Android come from the Expo plugin config:

```js
// app/app.config.js
["expo-notifications", { icon: "./assets/images/notification-icon.png", color: "#20744A" }]
```

---

## Key Files

| File | Role |
|------|------|
| `app/src/hooks/use-push-token-registration.ts` | Token acquisition and registration |
| `app-bridge/src/client/daemon-client.ts:1322` | Sends `register_push_token` over WebSocket |
| `daemon/internal/server/session.go:787` | Receives and stores token |
| `daemon/internal/push/token_store.go` | In-memory + file-persisted token store |
| `daemon/internal/push/notification.go` | Builds notification payload |
| `daemon/internal/push/service.go` | Expo HTTP client with retry |
| `daemon/internal/server/attention.go` | Orchestrates push on agent event |
| `daemon/internal/server/attention_policy.go` | Decides push vs in-app vs suppress |

---

## Reliability Mechanisms

| Concern | Solution |
|---------|----------|
| Daemon restart loses tokens | `PersistedTokenStore` writes to `~/.solo/push-tokens.json` |
| Duplicate token registrations | `Register()` skips if token already exists |
| Stale / uninstalled device token | Expo returns `DeviceNotRegistered`; `handleResponse()` calls `tokenStore.Remove()` |
| App open — don't interrupt user | `ComputeNotificationPlan` suppresses push when app is visible and focused |
| Error noise | `reason == "error"` never triggers a push |
| Network flakiness | Exponential back-off retry (3 attempts: 1s → 2s → 4s) |

---

## Why iOS Shows Paseo Icon (Current Issue)

The `~/.solo/push-tokens.json` file may contain a token that was registered by
the **Paseo app** before Solo was installed. That token is bound to the Paseo
bundle ID in APNs. When the daemon sends to it:

1. Expo routes the push to APNs using the Paseo bundle credentials.
2. APNs delivers it to the Paseo app still installed on the device.
3. iOS displays the Paseo icon and branding.

**Fix**: Install Solo on iOS, connect to the daemon once. The new Solo token is
registered and persisted. On the next send the old Paseo token will return
`DeviceNotRegistered` (once Paseo is uninstalled) and be auto-removed.
