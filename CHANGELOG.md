# Changelog

## v0.5.0 (2026-11-23)

- Ship the deployment as `docker-compose.yml`. If your scripts pass `-f compose.yml`, update the filename.
- Add a README contents list, documentation links and sponsor links.
- Select Claudebox or PiBox with `HARNESS_PLATFORM`. Claudebox remains the default. PiBox supports Z.ai, Telegram, cron and the stack's MCP tools.

## v0.4.0 (2026-11-23)

- Use the published Go image by default. Select local services with Compose profiles, or point the agent at remote browser, Mailbox, handoff, ModemBox and KeePassBox endpoints.
- For an existing install, add `COMPOSE_PROFILES=claude,browser,mailbox,handoff,tailscale` to `.env` to keep the original service selection. Then pull the new images and run `make restart`.
- Apply the private viewer's Tailscale Serve configuration at container startup.
- Build and test with Go 1.26.11 to pick up the HTTP security fix.
- Rewrite the README and operator guides around setup, browser handoffs and optional MCP connections.
- Include the agent identity setup, verification SMS, vault credentials and TOTP, and OS-level browser input from v0.3.0. Existing local account records stay in place.

## v0.3.0 (2026-11-23)

- Connect to optional ModemBox and KeePassBox MCP servers for verification SMS, encrypted account credentials and TOTP. Set the endpoint URLs and bearer tokens in `.env` and run `make restart` to enable them.
- Direct the agent to use OS-level keyboard and mouse input for browser interactions.
- Configure an approved verification phone number and generate the agent's private email, phone and mailbox identity.
- Preserve existing local account records across setup runs. Use the approved vault for new records when KeePassBox is configured.

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
