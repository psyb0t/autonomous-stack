# Autonomous Stack

[![Go Reference](https://pkg.go.dev/badge/github.com/psyb0t/autonomous-stack.svg)](https://pkg.go.dev/github.com/psyb0t/autonomous-stack)
[![CI](https://github.com/psyb0t/autonomous-stack/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/autonomous-stack/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/coverage.svg)](https://github.com/psyb0t/autonomous-stack/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/version.svg)](https://github.com/psyb0t/autonomous-stack/releases)
[![license](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/license.svg)](LICENSE)
[![imported by](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/importers.svg)](https://github.com/psyb0t/autonomous-stack/blob/badges/importers.md)
[![Docker Pulls](https://img.shields.io/docker/pulls/psyb0t/autonomous-stack?style=flat-square)](https://hub.docker.com/r/psyb0t/autonomous-stack)

Run Claude Code on your own box. Give it work over Telegram, put jobs on a schedule, and keep the same browser session between runs. When it gets stuck, open that browser on your tailnet and take the wheel.

Mailbox handles email. Plug in ModemBox for verification texts and KeePassBox for passwords and TOTP. Docker Compose holds the whole thing together.

## Quick start

Grab a machine you control, Docker with Compose v2.20 or newer, Make and Git. Give the agent its own accounts.

```bash
git clone https://github.com/psyb0t/autonomous-stack.git
cd autonomous-stack
make init
```

Edit `.env` and configure:

1. Claude OAuth, an Anthropic API key, or an existing Claude auth directory.
2. The Telegram bot token, numeric owner chat ID and numeric owner user ID.
3. IMAP/SMTP credentials and the allowed sender address.
4. The heartbeat schedule and prompts describing your tasks.
5. A Tailscale or Headscale auth key and a unique `TS_HOSTNAME` for browser access.

Message the bot from your owner account before you start the stack. If an old process is polling with that token, stop it first.

```bash
make pull
make up
make status
```

`make up` starts the services selected in `COMPOSE_PROFILES`. Compose uses the published Go image for handoffs and runs its one-shot command to write the config files. Use `make image` if you're working on the Go code.

`make init` generates distinct internal API tokens and never replaces an existing `.env`. Real credentials, runtime settings and data stay ignored by Git.

## What's running in this thing?

| Service | Project | Responsibility |
| --- | --- | --- |
| `claude` | [Claudebox](https://github.com/psyb0t/docker-claudebox) | Claude Code, heartbeat cron and owner-only Telegram |
| `browser` | [Stealthy Auto-Browse](https://github.com/psyb0t/docker-stealthy-auto-browse) | Browser MCP, screenshots, persistent profile and viewer |
| `mailbox` | [Mailbox](https://github.com/psyb0t/docker-mailbox) | Email MCP, IMAP reading and SMTP sending |
| `modembox` (optional) | [ModemBox](https://github.com/psyb0t/docker-modembox) | SMS and modem control over MCP from an existing USB modem gateway |
| `keepassbox` (optional) | [KeePassBox](https://github.com/psyb0t/docker-keepassbox) | Encrypted account storage and TOTP over MCP from an existing vault service |
| `handoff` | [Autonomous Stack](https://github.com/psyb0t/autonomous-stack) | Help requests, Telegram notifications and the persistent viewer proxy |
| `tailscale` | [Tailscale](https://github.com/tailscale/tailscale) | Private browser access under a persistent tailnet node name |

Each service has a matching Compose profile. Reuse a browser, Mailbox, ModemBox or KeePassBox you run elsewhere by removing its profile and setting its MCP URL and token. See [local and remote services](docs/configuration.md#local-and-remote-services).

The helper uses [servicepack](https://github.com/psyb0t/servicepack) for its lifecycle. Claudebox owns scheduling and Telegram polling. For USB modem access, run ModemBox on the host that has the modem attached.

Open `http://autonomous-stack` on your tailnet to see the browser. Anyone your ACLs let through can drive it, so give that access to people you'd trust with the keyboard. Tailscale Serve exposes only the viewer. The stack's MCP and browser automation APIs stay on the Compose network behind bearer authentication.

## When Claude gets stuck

The agent can try an encountered CAPTCHA once using a screenshot, or use an optional owner-configured solver. Solvers default off; see [CAPTCHA settings](docs/configuration.md#optional-captcha-solver). If the selected approach fails, it requests help and pauses that page.

1. Telegram receives the reason and the stable browser URL. The agent pauses automation.
2. Reply `ready` when available.
3. Open `http://autonomous-stack` and complete the task.
4. Reply `done` so the agent can inspect the page and resume.

The same link works any time. Drop in to watch, or tell the agent to pause if you're taking over. See [how handoffs work](docs/handoff.md).

## Persistent data

| Directory | Contents |
| --- | --- |
| `data/browser/` | Browser profile and cookies |
| `data/recordings/` | Browser recordings |
| `data/workspace/agent/` | Tasks, artifacts and generated MCP settings |
| `data/claude/` | Default Claude auth directory |
| `data/claudebox/` | Cron history and Telegram state |
| `data/mailbox/` | Mailbox operation state |
| `data/handoff/` | Current help request |
| `data/tailscale/` | Persistent tailnet identity and Serve settings |

Tracked `.gitkeep` files keep these directories present. Their runtime contents are ignored. Back up `.env`, `data/` and any external auth directory privately. Cookies and login state are credentials.

## Configuration and operation

[`.env.example`](.env.example) lists operator settings. [Configuration](docs/configuration.md) covers auth, prompts, mail and optional Dikciz, ModemBox and KeePassBox connections.

After changing `.env`, run `make restart` to recreate the selected services and remove containers from disabled profiles. `make pull` refreshes the selected images. `make stop` stops the containers, and `make down` removes them while keeping persistent data.

Claude can use its tools while you're away. Its accounts, mounts and network access decide what it can touch. Give it what it needs for the job and watch the first few runs.

See [deployment](docs/deployment.md) for ownership, backup and privilege details.

## Development

```bash
make format
make lint
make test-coverage
make sec
make generate
```

Tooling runs in the development image. Normal tests use the race detector and local fixtures, without live Telegram polling, paid Claude calls or public tunnels.

[Development and CI](docs/development.md) describes the release pipeline. A release tag waits for the matching main commit to pass, then promotes its image without rebuilding it.

## License

MIT. See [LICENSE](LICENSE), [third-party notices](THIRD_PARTY.md) and [CHANGELOG.md](CHANGELOG.md).
