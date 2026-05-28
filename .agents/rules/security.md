# Security Rules

Rules for maintaining security properties of the Solo platform.

## E2EE (End-to-End Encryption)

- All relay-mediated communication MUST use E2EE. The relay server must never see plaintext message content.
- Crypto implementation lives in `app-bridge/src/relay/`. Changes to this directory require careful review.
- Key exchange: X25519 (Curve25519 Diffie-Hellman). Encryption: XSalsa20-Poly1305 (NaCl secretbox).
- Never reuse nonces. Each encrypted message must use a unique nonce derived from a counter or random generation.
- Daemon keypair (`~/.solo/daemon-keypair.json`) must be generated with cryptographic randomness and never transmitted.
- Connection offers (`ConnectionOfferV2`) contain the daemon's public key. They are shared out-of-band and must not be logged.

## Credential & Secret Handling

- Never hardcode API keys, tokens, or passwords in source code. Use environment variables or config files.
- Config files (`~/.solo/config.json`) may contain provider API keys. These files must have restrictive permissions (0600).
- Never log credentials. When logging request/response data, redact `Authorization`, `Cookie`, and `X-API-Key` headers.
- The redactor stack (`memory/redact/`) handles automatic redaction of known secret patterns. Extend it when adding new providers.
- Git: never commit files containing secrets. Add patterns to `.gitignore` proactively (e.g., `*.key`, `.env.local`).

## Input Validation

- Validate all input at system boundaries: WebSocket message handlers, HTTP endpoints, CLI flag parsing.
- WebSocket messages from clients must be validated before processing. Use typed structs with JSON unmarshaling; reject malformed payloads.
- File paths from user input must be validated against path traversal (`..` components). Use `filepath.Clean` and verify the result is within the expected directory.
- Terminal input goes through PTY; validate terminal size parameters and reject unreasonable values.

## Network Security

- The daemon binds to `127.0.0.1:17612` only. Never change this to `0.0.0.0` or a public interface.
- The relay binds to `127.0.0.1:8081` behind Nginx. Direct relay access is only for local development.
- TLS termination happens at Nginx. The relay and daemon use plain WebSocket on localhost.
- Connection verification: the client validates the daemon's public key against the connection offer before trusting the E2EE channel.
- Rate limiting: implement at the Nginx layer for production. The relay should not be directly exposed.

## Provider Security

- AI provider backends run as subprocesses. Never pass user input directly as shell arguments.
- Use `exec.Command` with explicit argument lists, not shell string concatenation.
- Provider stdout/stderr may contain sensitive data (API keys in error messages). Redact before logging or displaying to users.
- Provider processes should run with the same user permissions as the daemon. Never escalate privileges.

## Session & Memory Security

- Session memory files (`~/.solo/memory/`) may contain code snippets and user prompts. Treat as sensitive data.
- The redactor stack runs on all persisted turns. Never bypass redaction for "convenience."
- Session data in memory (RAM) should be zeroed when no longer needed where practical.
- Timeline stores are per-session. Never leak timeline data across sessions.

## Dependency Security

- Go: run `go mod verify` to check module checksums. CI enforces this.
- npm: `npm ci` uses lockfile-pinned versions. Never use `npm install` in CI (it may resolve differently).
- Review new dependencies before adding them. Check maintenance status, download counts, and known vulnerabilities.
- Keep dependencies minimal, especially in `protocol/` (which must remain dependency-free) and `app-bridge/` (crypto-sensitive).

## Incident Response

- If a private key is accidentally committed: rotate immediately, revoke the old key, and scrub git history.
- If an API key leaks: revoke and regenerate from the provider dashboard.
- Document security-relevant decisions in commit messages and PR descriptions.
