# Autonomous Stack contributor guide

Autonomous Stack runs a Claude Code or Pi agent with scheduled work, Telegram, email and a persistent browser. Docker Compose runs the services. The Go helper uses servicepack for its lifecycle.

- `internal/pkg/setup/` validates deployment settings and generates the agent's MCP configuration.
- `internal/pkg/handoff/` owns human assistance state. `internal/pkg/http/server/` exposes the handoff MCP tools and browser viewer proxy.
- `config/agent.md` defines the deployed agent's operating instructions. `docs/` covers configuration, deployment and development.

Use `make lint` and `make test` for Go and shell changes, and `make check-compose` for Compose changes. See [development and CI](docs/development.md) for runtime tests and service registration.

Read [.agents/rules/runtime.md](.agents/rules/runtime.md) for deployment and configuration changes.
