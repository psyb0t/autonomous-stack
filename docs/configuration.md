# Configuration

Run `make init` from the checkout and fill in `.env` next to [`docker-compose.yml`](../docker-compose.yml). Compose reads it as dotenv. Quote spaces, multiline prompts, `#` and `$` as needed. Use single quotes when you want a dollar sign to stay a dollar sign. Keep untrusted dotenv files out of your shell.

Compose runs `prepare` before starting the agent runtime, local Mailbox or local KeePassBox. It writes only the selected services' configuration, including the agent's MCP settings. Generated files use mode 0600. Use `make setup` to run that step on its own.

## Local and remote services

List the local containers you want in `COMPOSE_PROFILES`. This is the default for a fresh install. The older `claude` profile name still enables the same agent service.

```dotenv
COMPOSE_PROFILES=agent,browser,mailbox,handoff,tailscale
```

Add `modembox` or `keepassbox` to run either locally. To use a service that already runs elsewhere, remove its profile and set its remote URL, with the exact MCP path the service serves, and the token issued by that service. PiBox's MCP bridge does not follow redirects, so it never sends the bearer token to another address: use `/mcp/` for Stealthy Auto-Browse, which redirects `/mcp` there, and `/mcp` for Mailbox. With the profile and URL both absent, the agent skips the integration. Use the `.env` setting to select profiles so the containers and generated MCP config agree.

| Local profile | Remote URL | Token |
| --- | --- | --- |
| `browser` | `BROWSER_MCP_URL` | `BROWSER_TOKEN` |
| `mailbox` | `MAILBOX_MCP_URL` | `MAILBOX_TOKEN` |
| `handoff` | `HANDOFF_MCP_URL` | `AUTONOMOUS_TOKEN` |
| `modembox` | `MODEMBOX_MCP_URL` | `MODEMBOX_TOKEN` |
| `keepassbox` | `KEEPASSBOX_MCP_URL` | `KEEPASSBOX_TOKEN` |

For an external Mailbox, set `MAIL_FROM` if the agent should have an approved email identity. IMAP and SMTP settings are needed only for local Mailbox. A local handoff service can use a remote browser viewer via `AUTONOMOUS_BROWSER_VIEWER_URL`. Set `AUTONOMOUS_VIEWER_URL` to the private address the owner should open if you're providing your own viewer ingress. The `tailscale` profile requires `handoff`.

## Agent runtime

`HARNESS_PLATFORM` selects the image for the single agent service. It defaults to `claudebox`. Change it to `pibox` to run Pi with the same workspace, Telegram bot, schedule, and MCP integrations. Run `make pull` and `make restart` after changing it. The two runtimes keep their own session and bot state.

### Claudebox

Choose `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_API_KEY`, or an existing `CLAUDE_AUTH_DIR`. Leave unused fields empty. The default auth directory is `./data/claude`; an external path must already exist and be readable by the selected runtime UID.

The auth directory mounts at `/claude-auth`, outside the base image's home ownership scan. It is writable for credential refresh. Never mount your entire home. A host login stored in an OS keychain may not work inside Linux; use a supported OAuth token or API key in that case.

`CLAUDE_MODEL` and `CLAUDE_EFFORT` configure both modes. `HEARTBEAT_SCHEDULE` accepts five cron fields, or six with seconds first. `TZ` sets the container timezone. `HEARTBEAT_PROMPT` defines each scheduled run; `AGENT_PROMPT` is appended to both modes' system prompts. `config/agent.md` supplies the MCP and handoff operating instructions.

The operating instructions require OS-level mouse and keyboard input for browser interactions, not Playwright input or JavaScript page mutations. Read-only page inspection remains available. These instructions do not bypass a website's access rules or guarantee model compliance.

### PiBox

Set these in your ignored `.env`:

```dotenv
HARNESS_PLATFORM=pibox
PIBOX_ZAI_AUTH_TOKEN=your-z-ai-token-here
PIBOX_ZAI_DEFAULT_MODEL=glm-5.3-flash
```

PiBox uses Pi's built-in `zai` provider, which calls the Z.ai Coding Plan API directly with the token as `ZAI_API_KEY`. The model shown is the default; any model Pi lists for `zai` works, such as `glm-5.3`. `PIBOX_ZAI_BASE_URL` is no longer read. Compose runs the same cron and Telegram modes with the model you select. The MCP bridge connects PiBox's stdio tool client to the configured HTTP MCP endpoints. The service still appears as `claude` in Compose to keep existing commands working.

## Telegram

`TELEGRAM_BOT_TOKEN` authenticates the bot. `TELEGRAM_CHAT_ID` selects the numeric destination, including a group's minus sign. `TELEGRAM_USER_ID` selects the owner allowed to issue commands. Setup generates explicit chat and user allowlists. Start the bot from that account before deployment. The selected runtime polls for updates. The helper sends messages.

Before actual work, the agent sends a short start update using the mounted send-only helper. Meaningful progress and blockers also go to the owner. The selected runtime sends the final reply for both chat and cron runs. Idle heartbeats stay quiet. These are agent instructions, not a scheduler-level guarantee; process crashes can prevent an agent-generated update. The helper accepts text on stdin, fixes the destination to `TELEGRAM_CHAT_ID` and never polls for messages.

## Mail

`MAIL_IMAP_HOST`, `MAIL_IMAP_PORT`, `MAIL_IMAP_TLS` control reading. `MAIL_SMTP_HOST`, `MAIL_SMTP_PORT`, `MAIL_SMTP_TLS` control sending. Setup accepts `ssl` and `starttls`, not plaintext transport.

`MAIL_USERNAME` and `MAIL_PASSWORD` authenticate both protocols. Use a dedicated mailbox and an app password when required. `MAIL_FROM` is the permitted sender address. Set `MAIL_SAVE_SENT=false` if the provider already files sent copies.

The generated token has `read`, `draft`, `organize` and `send` scopes on `primary` only. It cannot permanently delete mail or administer Mailbox. Mailbox's default send limits remain in effect.

`MAIL_TRUSTED_AUTHSERV_ID` is the exact identity your receiving server stamps in `Authentication-Results`. It selects whose SPF/DKIM/DMARC verdicts Mailbox may trust. Copy it from your own server's genuine received header, not the sender address. Leave it empty if that trust cannot be established. It does not configure your domain's authentication records.

## Optional phone

Set both `DIKCIZ_MCP_URL` and `DIKCIZ_API_KEY` to add a [Dikciz Launcher](https://github.com/psyb0t/dikciz-launcher) `phone` MCP server. Include the `/mcp` path. The URL must be reachable from the agent container; a URL working only on your desktop is insufficient. Prefer a private network or authenticated TLS endpoint. Both empty disables phone access. SMS permissions, numbers and SIM selection come from Dikciz's tools.

### ModemBox

For local ModemBox, add its profile, set `MODEMBOX_DEVICE` to the host modem's control port, and set `MODEMBOX_DEVICE_GID` to the device's group ID. Compose passes through only that serial device. For a known Huawei layout, select the port for USB interface `02`. To add voice, map the audio port at the same path inside the container with a Compose override. Recreate the container if the USB device path changes. The standalone ModemBox install can manage discovery across multiple modems and serve this stack remotely.

For a remote ModemBox, set `MODEMBOX_MCP_URL` to that instance's full MCP URL, including `/mcp`, and `MODEMBOX_TOKEN` to its operator token. The agent container needs to reach it over HTTPS or an isolated trusted network. An operator token can use every feature on that instance, so give the agent a dedicated instance or SIM. Leave both settings empty to skip ModemBox. See the [ModemBox MCP guide](https://psyb0t.github.io/docker-modembox/mcp/).

For SMS verification, set `MODEMBOX_MODEM_ID` to the device ID from ModemBox that you have confirmed serves `AUTONOMOUS_PHONE_NUMBER`. Setup records the mapping as `modembox_modem_id` in the generated identity. Update it if you change the SIM. Both Dikciz and ModemBox can be configured, and the agent uses the mapping to select the right inbox.

## KeePassBox

For local KeePassBox, add its profile and supply `KEEPASSBOX_TOKEN`. Setup writes the private access definition for the `automation` owner. Vaults persist in `data/keepassbox/`.

For a remote KeePassBox, give the agent its own vault and set `KEEPASSBOX_MCP_URL` to the instance's full MCP URL, including `/mcp`, and `KEEPASSBOX_TOKEN` to its bearer token. Grant entry read and write access, plus secret access if the agent needs passwords or TOTP codes. The agent needs to reach the vault over HTTPS or an isolated trusted network before it can submit a signup. Leave both settings empty to skip KeePassBox. See the [KeePassBox MCP guide](https://psyb0t.github.io/docker-keepassbox/mcp/).

Run `make restart` after changing a profile, endpoint or token so Compose replaces the containers and regenerates the MCP configuration.

## Agent identity and account passwords

Set `AUTONOMOUS_PHONE_NUMBER` to the owner-approved verification number in E.164 format, including its leading `+` and no spaces. Leave it empty to require an owner decision before phone verification. This selects an identity; it does not change Android's default SIM or grant phone permissions. The agent must confirm the matching subscription or ask when the device cannot distinguish numbers.

Setup writes `/config/identity.json` for the agent with `MAIL_FROM`, the selected phone number and the local mailbox login when the `mailbox` profile is active. The host file is `config/generated/identity.json`, mode 0600. Real values stay out of reusable prompts. Mailbox MCP remains the normal mail interface.

With KeePassBox configured, the agent stores new account credentials in the approved vault. Without it, credentials go in `/workspace/agent/private/passwords.json`, backed by `data/workspace/agent/private/passwords.json` here. Setup creates an empty mode-0600 file in a mode-0700 directory and preserves existing entries. It rejects unsafe permissions, directories and symbolic links at the password-file path. Existing local entries stay where they are until the owner moves them. The local password file and generated identity contain plaintext secrets. Both are ignored by Git and excluded from image builds.

Verification instructions restrict the agent to relevant recent messages for the current approved task. Missing permissions, ambiguous SIM mapping or undelivered codes require owner help rather than permission changes or repeated sends.

## Internal services

`make init` generates distinct tokens for handoff, the browser, Mailbox, ModemBox and KeePassBox. On an existing install, fill in any new local service token before enabling its profile. Rotate local tokens with `make restart` so callers and services get the change together.

`AUTONOMOUS_REQUEST_TTL` defaults to `24h` and permits one minute through seven days. It expires pending help requests, not browser access. Active assistance remains active until the owner finishes, including across helper restarts.

`BROWSER_LANG`, `BROWSER_RESOLUTION` and `TZ` describe the browser environment. Resolution is width x height, such as `1440x900`, without a depth suffix. `*_MEMORY` and `*_CPUS` set runtime limits. `STACK_UID` and `STACK_GID` match bind ownership and are filled by init. `AUTONOMOUS_IMAGE` selects the locally built helper image name.

## Optional CAPTCHA solver

Solvers are disabled by default and require Stealthy Auto-Browse v3.3.0 or newer. Run `make pull` to refresh the browser image, then configure these settings in your existing `.env`:

```dotenv
CAPTCHA_SOLVER_ENABLED=true
CAPTCHA_SOLVER_PROVIDER=capsolver
CAPTCHA_SOLVER_API_KEY=your-token-here
```

Supported provider names are `nopecha`, `capsolver`, `capmonster`, `nocaptchaai`, `yescaptcha`, `2captcha` and `anticaptcha`. All except NopeCHA require a key. NopeCHA may operate without one subject to its own quotas. Only one provider is active. The browser validates settings at startup and rejects unknown providers, missing required keys, malformed enable values, or `false` with a populated provider/key. To disable, set `CAPTCHA_SOLVER_ENABLED=false` and clear both other fields. Apply changes with `make restart`; `make init` preserves existing `.env` files.

The first six providers use verified Firefox extensions. `anticaptcha` exposes API-backed `solve_captcha`; it does not automatically solve every visible challenge. The agent discovers the tool schema, attempts the selected approach once and requests human help on failure. Enabling a provider authorizes its ordinary usage fees for approved tasks, not subscriptions or credit purchases. Configure provider-side budgets or quotas where available. Extensions can run automatically while browsing, so the agent's one-attempt instruction is not a billing cap.

Only the browser receives these keys. Keep them in the ignored `.env`. Docker administrators can inspect environment values. Extensions can read pages, send challenge data to their provider and retain configured keys in the persistent profile. Protect `data/browser/` and its backups. See the browser's [solver configuration and secret-file option](https://github.com/psyb0t/docker-stealthy-auto-browse/blob/main/docs/configuration.md#captcha-solver-providers) for provider details and custom Compose secret mounts.

## Tailnet browser access

`TS_HOSTNAME` is a unique lowercase DNS label, defaulting to `autonomous-stack`. The helper sends links starting with `http://<TS_HOSTNAME>`. Use that exact name, not the raw container address. Enable MagicDNS or equivalent Headscale DNS on the computer opening the link.

`TS_AUTHKEY` registers the sidecar on first use. Obtain it from your Tailscale administration console or Headscale server. `TS_AUTH_ONCE=true` and `data/tailscale/` preserve the node identity across restarts. Keep that state private. A key is not required again while the saved login remains valid.

`TS_EXTRA_ARGS` defaults to `--accept-dns=false`. For Headscale, set it to `--accept-dns=false --login-server=https://headscale.example.com`. Tailscale validates these flags during startup. `TAILSCALE_MEMORY` and `TAILSCALE_CPUS` limit the sidecar. No host ports, TUN device or extra capabilities are required by this inbound-only userspace configuration.

See the upstream [Docker configuration parameters](https://tailscale.com/docs/features/containers/docker/docker-params) for Tailscale's environment-variable reference.

Compose runs a one-shot Serve command when the Tailscale sidecar becomes healthy. It exposes the helper's viewer listener on tailnet port 80.

The short-name URL uses HTTP inside encrypted WireGuard traffic, matching Headscale setups without HTTPS certificates. Tailnet membership and ACLs are the viewer's access boundary. There is no application login, cookie requirement or access expiry. Every client permitted to reach the node can control its browser. Restrict ACLs to trusted owners and never forward that listener onto a public network. Host and Origin checks remain in place.

When upgrading from the public-tunnel or expiring-viewer version, add the `TS_*` settings and remove `AUTONOMOUS_TUNNEL_TTL`, `AUTONOMOUS_HANDOFF_TTL` and `AUTONOMOUS_TUNNEL_STARTUP_TIMEOUT` from your existing `.env`. They no longer control anything. `make init` deliberately does not overwrite existing settings.
