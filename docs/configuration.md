# Configuration — `~/.solo/`

All Solo host-side state lives under `~/.solo/` (override with the `SOLO_HOME` env var).
This page inventories every file and directory there, and documents the two files
you edit by hand: `config.json` (daemon) and `usage.json` (usage providers).

## Directory inventory

| Path | Purpose | Created by | When |
|------|---------|-----------|------|
| `server-id` | Stable daemon identity (random hex) | daemon | auto on first start |
| `solo.pid` | Daemon process ID (removed on clean exit) | daemon | auto on first start |
| `agents/` | Per-agent runtime state | daemon | auto on first start |
| `cli-client-id` | CLI client identity | CLI (`solo onboard`) | auto on pairing |
| `daemon-keypair.json` | E2EE keypair for the daemon | CLI (`solo onboard`) | auto on pairing |
| `config.json` | Daemon configuration (see below) | daemon / you | sparse-saved on first mutation, or manual |
| `loops.json` | Loop definitions | daemon | lazy on first write |
| `schedules.json` | Schedule definitions | daemon | lazy on first write |
| `workspaces.json` | Workspace list | daemon | lazy on first write |
| `projects/` | Per-project state | daemon | lazy on first write |
| `push-tokens.json` | Push notification tokens | daemon | lazy on first write |
| `agent-commands.json` | Agent command presets | daemon | lazy on first write |
| `logs/` | Daemon logs | daemon | lazy on first write |
| `memory/` | Session memory store | daemon | lazy on first write |
| `worktrees/` | Git worktrees managed by Solo | daemon | lazy on first write |
| `timeline/` | Timeline data (legacy, no longer written) | daemon | legacy |
| `usage.json` | Usage/quota provider credentials (see below) | you | manual (`solo-usage init`) |
| `*.cookie` | Session cookie files referenced from `usage.json` | you | manual |

## `config.json` — daemon configuration

The daemon **reads** `config.json` at startup but never writes defaults to it.
Defaults live in code (`DefaultConfig()` in `daemon/internal/config/config.go`),
so upgrading the daemon improves behavior without touching your file. The file is
only created/updated when a setting is first mutated through the app (sparse save:
only explicitly-set fields are persisted). You can also create it by hand; every
field is optional and omitted fields fall back to the defaults below.

A full-field reference file is available at
[`docs/examples/config.json.example`](examples/config.json.example).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `daemon.listen` | string | `127.0.0.1:17612` | Listen address: `host:port`, `unix:///path`, or `pipe:///path` |
| `daemon.hostnames` | string[] | `[]` | Extra hostnames the daemon accepts |
| `daemon.mcp.injectIntoAgents` | bool | `false` | Inject Solo's MCP server into agent sessions |
| `daemon.cors.origins` | string[] | `["https://solo.up2ai.top", "http://localhost:19000"]` | Allowed CORS origins |
| `daemon.relay.enabled` | bool | `true` | Connect to the relay for remote access |
| `daemon.relay.endpoint` | string | `relay.solo.sh:443` | Relay control endpoint |
| `daemon.relay.publicEndpoint` | string | `relay.solo.sh:443` | Public relay endpoint advertised to clients |
| `daemon.relay.disableControlKeepalive` | bool | `false` | Disable keepalive on the relay control connection |
| `daemon.providers.customModels` | map | `{}` | Extra models per agent provider (key = provider ID); each entry: `id`, `label`, `description`, `isDefault`, `thinkingOptions[]` (`id`/`label`/`isDefault`), `defaultThinkingOptionId` |
| `daemon.providers.providerSettings` | map | `{}` | Per-provider overrides (key = provider ID): `enabled`, `label`, `description` |
| `daemon.llmProviders` | object[] | `[]` | User-configured LLM API providers (used e.g. by the schedule assistant): `id`, `label`, `description`, `enabled`, `baseURL`, `apiKey`, `models[]` (`id`/`label`/`description`/`isDefault`) |
| `daemon.tmuxAgentNames` | string[] | `[]` | Additional tmux agent names, merged with the built-in set (`claude`, `opencode`, `qodercli`, `pi`, `cursor`, `kimi`, `kimi-cli`, `codex`) |
| `daemon.timelineMaxRowsPerAgent` | int | `10000` | Hard upper bound for in-memory timeline rows per agent |
| `app.baseUrl` | string | `https://solo.up2ai.top` | Base URL of the web app |

Most fields can also be set via environment variables (`SOLO_LISTEN`, `PORT`,
`SOLO_RELAY_ENABLED`, `SOLO_RELAY_ENDPOINT`, `SOLO_RELAY_PUBLIC_ENDPOINT`,
`SOLO_RELAY_DISABLE_CONTROL_KEEPALIVE`, `SOLO_CORS_ORIGINS`, `SOLO_HOSTNAMES`,
`SOLO_APP_BASE_URL`, `SOLO_SUPERVISED`), which take precedence over the file.
The daemon keeps the file owner-only (`0600`) because it may contain API keys.

## `usage.json` — usage/quota providers

`solo-usage` (and the app's usage dashboards, via the daemon) reads
`~/.solo/usage.json` to fetch plan usage and quota. It is the only config you
must create fully by hand — bootstrap it with:

```sh
solo-usage init            # writes ~/.solo/usage.json (0600), refuses to overwrite
solo-usage init --print    # print the template to stdout
solo-usage init --force    # overwrite an existing file
```

Then edit the file: set `"enabled": false` (or delete the entry) for providers
you don't use, and fill in credentials.

### Placeholder syntax

Every string value supports two placeholder forms, expanded at load time:

- `${VAR}` — replaced by the environment variable `VAR` (kept verbatim when unset).
- `${file:/path}` — replaced by the trimmed contents of the file (leading `~`
  supported; kept verbatim when unreadable). Ideal for rotating secrets like
  session cookies: update the file, no daemon restart needed.

### Provider entries

Common fields per entry: `enabled` (bool), `apiKey` (string),
`endpoint` (optional string), `extra` (optional string map).

| Provider | Auth | Fields | Notes |
|----------|------|--------|-------|
| `kimi` | API key | `apiKey` (`sk-kimi-xxx`), `endpoint` | `endpoint` defaults to `https://api.kimi.com/coding/v1` |
| `deepseek` | API key | `apiKey` | Platform API key from platform.deepseek.com |
| `qoder` (org mode) | API key | `apiKey` + `extra.organizationId` | Teams OpenAPI, org admin only — see https://docs.qoder.com/zh/account/teams/openapi/usage |
| `qoder` (personal mode) | cookie | `extra.cookie` | Browser session cookie from qoder.com; takes precedence over org mode when set |
| `xiaomimimo` | cookie | `extra.cookie` | Browser session cookie from platform.xiaomimimo.com (`api-platform_ph`, `api-platform_serviceToken`, `api-platform_slh`, `userId`); `tp-` API keys are not supported |

Only one `qoder` entry is possible, so the generated template shows the org
mode; for a personal account replace it with `"extra": {"cookie": "${file:~/.solo/qoder.cookie}"}`.

### Cookie refresh workflow

Browser session cookies expire. Keep them in files and reference them with
`${file:...}` so refreshing is a single command — the new value is picked up on
the next load, no config edit or restart needed:

```sh
# copy the Cookie header value from the browser's devtools, then:
pbpaste > ~/.solo/xiaomimimo.cookie
```

A `401` from a cookie-based provider means the session expired — re-copy the
cookie from the browser and repeat the command above.
