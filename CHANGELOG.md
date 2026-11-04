# Changelog

## v0.2.0 (2026-11-04)

- Keep the tailnet browser permanently available, removing expiring viewer links and cookies. Plain `ready` and `done` coordinate automation without copied request IDs.
- Default the tailnet hostname and browser viewer URL to `autonomous-stack`.
- Add send-only Telegram progress updates and start/result reporting instructions.
- Add optional CAPTCHA provider settings, browser-only key forwarding and solver-aware human handoff instructions.
- Replace public Cloudflare tunnels with a persistent Tailscale sidecar and configurable browser node name, including Headscale support.
- Preserve owner-readiness coordination while keeping browser access independent of help requests.
- Remove obsolete tunnel and viewer expiry settings. Existing deployments must add tailnet settings to `.env` before restarting.
- Add isolated Headscale/Tailscale integration checks and component repository links.
- Fix comma-separated tmpfs options in Compose and reject malformed mount entries.

## v0.1.0 (2026-11-04)

- Add Claudebox cron and Telegram, Mailbox and a persistent browser in Compose.
- Generate private runtime configuration from operator-owned environment settings.
- Add optional Dikciz phone MCP access.
- Add durable human-help requests and expiring authenticated browser handoffs.
- Add containerized checks and reusable image, release, mirror, archive and badge workflows.
