# Development and CI

`internal/pkg/services/handoff/` implements the servicepack lifecycle. `internal/pkg/handoff/` owns help requests, notifications and assistance coordination. `internal/pkg/http/server/` owns MCP and the viewer proxy. `internal/pkg/setup/` generates deployment configuration.

Use Make for tooling. Normal tooling has no Docker socket. Infrastructure tests receive it only through the framework's dedicated test target. `make test-coverage` uses the race detector, includes integration packages and enforces the coverage floor. Fixtures use local HTTP, real files and owned child processes, without paid calls, bot polling or public tunnels.

`make generate` discovers service implementations and regenerates registration. Edit its sources, not generated files.

`make test-runtime` builds the helper and starts isolated browser, Mailbox and helper containers with disposable account settings. It tests generated configuration, the real viewer, readiness and MCP authentication, then stops only its own fixtures. It never starts Claude, polls Telegram or opens a public tunnel. Pull the upstream service images with `make pull` first.

`make audit-compose` validates the rendered deployment without printing credentials. It rejects public port bindings, host access, unrestricted capabilities, mutable external images and missing runtime restrictions. Browser and Claudebox have the exact bootstrap capability exceptions described in the deployment guide. CI runs both the runtime smoke test and this audit.

`make test-tailnet` starts a disposable Headscale controller, two real Tailscale nodes and a helper with a local notification fixture. It verifies stable short-name access over WireGuard before, during and after assistance, without cookies, and preserved access after sidecar restart. It uses no owner credentials or existing tailnet and stops only its own containers. CI runs it alongside the browser smoke test.

`make test-captcha-config` renders the production Compose model with disabled defaults and all seven provider names, checking exact value forwarding and browser-only key exposure. It runs without contacting solver providers and is included in `make test-runtime` and CI.

`make test-notify` checks the send-only progress helper with a mocked Telegram HTTP boundary, including malformed configuration, message limits and credential-safe errors. The runtime test target includes it. Live phone tests require owner-provided Claude, Telegram and Dikciz credentials and must not expose SMS bodies or verification codes in reports.

## Publishing

Main pushes pass code checks before publishing `sha-<commit>` and `latest`. Scheduled runs refresh checks and badges without publishing images. Security reports use the reusable workflows.

A release tag calls `wait-for-default-branch.yml`, waiting for the successful main push of the exact tagged commit. A failed run, timeout or commit absent from main blocks release. Image promotion then retags that tested image for all platforms. The tag pipeline does not rebuild it. The GitHub release follows promotion; coverage comes from the matching main run.

Other callers mirror to GitLab/Codeberg, import mirror issues, archive public pages and close external PRs without executing their code.

Required Actions secrets are `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`, `GITLAB_TOKEN`, `CODEBERG_TOKEN`, `ARCHIVEORG_ACCESS_KEY` and `ARCHIVEORG_SECRET_KEY`. Set them before the first push. Runtime Claude, Telegram, mail and phone credentials never belong in Actions.
