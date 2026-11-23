# Take over the browser

Open `http://autonomous-stack` on your tailnet. Your Tailscale or Headscale ACLs decide who gets a keyboard and mouse. Keep the viewer on that trusted network.

| Tool | Input | Effect |
| --- | --- | --- |
| `request_help` | `key`, `reason` | Persist a help request and notify the owner with the stable viewer URL |
| `handoff_status` | None | Read coordination state |
| `start_handoff` | `user_ready`, optional `request_id` | Mark assistance active after the owner says ready |
| `finish_handoff` | Optional `request_id` | Complete assistance so automation can resume |

Only one request can be pending or active. Repeating its task key is idempotent. A different task conflicts until the current request is completed. Failed pending notifications can be retried using the same key. Pending requests expire according to `AUTONOMOUS_REQUEST_TTL`; active assistance has no timer.

Reply `ready` in Telegram when you're taking over and `done` when the agent can come back. The agent matches those replies to the current request. It handles the coordination. The helper checks the MCP bearer token on every call.

The agent should leave the browser alone while you work and check the page after you say `done`. If you open the viewer on your own, tell it to pause before you start clicking.

Completing or cancelling a request never revokes viewer access or disconnects its streams. Restart preserves active assistance and pending requests. Legacy active state is upgraded by clearing its obsolete access deadline. Old `/connect/...` links redirect to the normal viewer without creating a cookie.

The root viewer URL redirects to noVNC. Host and Origin checks reject unexpected browser origins. The proxy strips credentials and forwarded identity headers before reaching noVNC. Tailscale Serve publishes only that viewer listener, not MCP or browser automation. An actual helper restart interrupts its connections while the process restarts; completing a request does not.

State is atomically saved in `data/handoff/handoff.json`. Run one replica with that directory. Multi-replica coordination is not implemented. See [tailnet configuration](configuration.md#tailnet-browser-access).
