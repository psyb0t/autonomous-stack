# Autonomous Stack

[![Go Reference](https://pkg.go.dev/badge/github.com/psyb0t/autonomous-stack.svg)](https://pkg.go.dev/github.com/psyb0t/autonomous-stack)
[![CI](https://github.com/psyb0t/autonomous-stack/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/autonomous-stack/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/coverage.svg)](https://github.com/psyb0t/autonomous-stack/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/version.svg)](https://github.com/psyb0t/autonomous-stack/releases)
[![license](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/license.svg)](LICENSE)
[![imported by](https://raw.githubusercontent.com/psyb0t/autonomous-stack/badges/importers.svg)](https://github.com/psyb0t/autonomous-stack/blob/badges/importers.md)
[![Docker Pulls](https://img.shields.io/docker/pulls/psyb0t/autonomous-stack?style=flat-square)](https://hub.docker.com/r/psyb0t/autonomous-stack)

Self-hosted autonomous agent stack with scheduled work, Telegram, email, a persistent browser and temporary human handoffs. Currently runs Claude Code through Claudebox.

Claude does the work. Claudebox wakes it on a schedule and receives your Telegram messages. Mailbox and the browser expose MCP tools. A small Go service manages human-help requests and temporary browser access.

## Quick start

Requires Docker with Compose v2, Make and Git. Use a machine you control and dedicated agent accounts.

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

Start a conversation with the bot before deploying. Do not run another Telegram poller with the same bot token.

```bash
make pull
make up
make status
```

`make up` builds the local helper image, validates settings, generates private configuration and starts the stack. The other service images come from Docker Hub. No host Go installation is needed.

`make init` generates distinct internal API tokens and never replaces an existing `.env`. Real credentials, runtime settings and data stay ignored by Git.

## Components

| Service | Responsibility |
| --- | --- |
| `claude` | Claude Code, heartbeat cron and owner-only Telegram |
| `browser` | Browser MCP, screenshots, persistent profile and viewer |
| `mailbox` | Email MCP, IMAP reading and SMTP sending |
| `handoff` | Help requests, Telegram notifications and expiring browser tunnels |

The helper uses [servicepack](https://github.com/psyb0t/servicepack) for its lifecycle. Claudebox owns scheduling and Telegram polling. No Docker socket, USB modem or extra Telegram client is included.

No ports are published on the host. Tool APIs use bearer authentication on the Compose network. The temporary tunnel exposes only an authenticated viewer, not the MCP or browser automation API.

## Human help

The agent can try an encountered CAPTCHA once using a screenshot. If it fails, it requests help and pauses that page.

1. Telegram receives the reason and a request ID. No tunnel opens yet.
2. Reply `ready <request_id>` when available.
3. The agent opens the handoff and you receive an HTTPS viewer link.
4. Complete the task and reply `done <request_id>`.
5. The agent closes the tunnel and checks the page before continuing.

Links expire after ten minutes by default. Restarting the helper invalidates active links. Readiness depends on the agent following the owner's reply, not independent verification by the helper. See [handoff behavior](docs/handoff.md).

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

Tracked `.gitkeep` files keep these directories present. Their runtime contents are ignored. Back up `.env`, `data/` and any external auth directory privately. Cookies and login state are credentials.

## Configuration and operation

[`.env.example`](.env.example) lists operator settings. [Configuration](docs/configuration.md) covers auth, prompts, mail and optional Dikciz phone MCP access.

After changing `.env` or pulling source, run `make restart`. It rebuilds the helper, regenerates configuration and recreates services. `make pull` refreshes upstream service images. `make stop` stops services; `make down` removes containers and the network without deleting persistent bind data.

Claude Code runs with unattended tool permissions. Its accounts, mounts and network define its reach. Use dedicated accounts and a narrow task scope. Prompt instructions do not make hostile pages or email safe.

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
