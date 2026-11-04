# Deployment

Docker must run on the machine holding this checkout. Bind paths are resolved by its daemon. A remote Docker context with another filesystem is not supported by local setup.

## Data and ownership

Run init as the checkout's owner. The helper and Mailbox use `STACK_UID:STACK_GID`; browser and Claudebox bootstrap a matching runtime user. Do not switch ownership without correcting the bind directories deliberately.

Back up `.env`, `data/` and any external Claude auth directory privately. Stop the stack for a consistent copy, including Mailbox's SQLite state and the browser profile. Test restore into a separate directory. Never share one live browser profile between running instances.

Setup replaces generated MCP/settings files, not task artifacts. `config/generated/` is recreated from `.env` and is not a separate configuration source. Down does not delete bind data.

## Network and privilege boundaries

No host ports are published. Do not attach unrelated containers to this Compose network. The browser's raw viewer is unauthenticated within that network; never publish it directly.

The helper, Mailbox and Tailscale sidecar run non-root with no capabilities and read-only roots. Each has only its required writable state and temporary storage.

Browser and Claudebox have writable roots and narrow bootstrap capabilities. Browser fixes profile ownership before dropping privileges. Claudebox installs the supported Claude CLI, matches UID/GID and drops privileges. These exceptions are required by the upstream entrypoints. No container receives the Docker socket, host devices or the whole home directory.

The selected Claude auth directory is writable for refresh. Mounting an existing auth directory shares those credentials with the container. Use a dedicated token or repo-local directory if that sharing is unwanted.

Root and Docker-group users can inspect container credentials. `.env` permissions do not hide them from the Docker administrator. Generated MCP settings also contain tokens.

## Operation

`make status` shows health. `make logs` follows bounded logs. `make stop` pauses services. `make restart` rebuilds the helper, validates settings and recreates services. `make pull` refreshes the other first-party images without restarting them. Back up state before updates and read upstream compatibility notes.

Claudebox has no readiness endpoint in Telegram mode. Its healthcheck checks the native bot process, which also starts the cron scheduler in a thread. It does not prove an external Telegram connection or Claude authentication. Check an owner message and a heartbeat after initial deployment.

Claude has unattended tool access. Use dedicated accounts, approve a narrow task scope and watch early runs. Prompt rules do not make hostile email or web pages safe. Cron and Telegram may launch separate processes sharing the workspace; coordinate tasks in your operating instructions.

## Troubleshooting

Missing Telegram/mail configuration is rejected by setup. Start the bot chat before deploying and stop other pollers using that token. A phone URL must work from the container network, including any VPN routing.

A sidecar login or Serve failure stops `make up` with an error. Check `make logs`, tailnet key validity and control-server reachability. Notification failure affects help coordination, not viewer access. Finish a broken help request and inspect health rather than exposing raw viewer ports.

The sidecar has its own tailnet identity and does not change the host's Tailscale login. Userspace networking is sufficient because it only accepts incoming viewer connections. Other stack containers do not automatically use the sidecar for outgoing tailnet traffic. Keep `data/tailscale/` in private backups; `make down` preserves it. Restrict its node's port 80 to the intended owner through tailnet ACLs.
