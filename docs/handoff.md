# Human assistance

The browser stays available at `http://autonomous-stack`. Tailscale or Headscale membership and ACLs control who can reach it. Every permitted client can control the browser desktop. There is no viewer password, expiring link or handoff cookie. Never publish the viewer listener outside the trusted tailnet.

| Tool | Input | Effect |
| --- | --- | --- |
| `request_help` | `key`, `reason` | Persist a help request and notify the owner with the stable viewer URL |
| `handoff_status` | None | Read coordination state |
| `start_handoff` | `user_ready`, optional `request_id` | Mark assistance active after the owner says ready |
| `finish_handoff` | Optional `request_id` | Complete assistance so automation can resume |

Only one request can be pending or active. Repeating its task key is idempotent. A different task conflicts until the current request is completed. Failed pending notifications can be retried using the same key. Pending requests expire according to `AUTONOMOUS_REQUEST_TTL`; active assistance has no timer.

The owner replies `ready` and `done` in the allowed Telegram chat. The agent maps these replies to the current request internally. No request ID needs copying. Readiness is agent-enforced coordination, not an independent proof of the Telegram reply. The helper still requires bearer authentication for all MCP calls.

Pause automation when requesting assistance. While assistance is active, the agent leaves the browser alone. After completion, it checks the page before continuing. Opening the viewer independently does not automatically pause an agent; tell it when taking control.

Completing or cancelling a request never revokes viewer access or disconnects its streams. Restart preserves active assistance and pending requests. Legacy active state is upgraded by clearing its obsolete access deadline. Old `/connect/...` links redirect to the normal viewer without creating a cookie.

The root viewer URL redirects to noVNC. Host and Origin checks reject unexpected browser origins. The proxy strips credentials and forwarded identity headers before reaching noVNC. Tailscale Serve publishes only that viewer listener, not MCP or browser automation. An actual helper restart interrupts its connections while the process restarts; completing a request does not.

State is atomically saved in `data/handoff/handoff.json`. Run one replica with that directory. Multi-replica coordination is not implemented. See [tailnet configuration](configuration.md#tailnet-browser-access).
