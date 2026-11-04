# Human handoff

| Tool | Input | Effect |
| --- | --- | --- |
| `request_help` | `key`, `reason` | Persist and notify, without a tunnel |
| `handoff_status` | None | Read the current request and phase |
| `start_handoff` | `request_id`, `user_ready` | Start the viewer tunnel and send its link |
| `finish_handoff` | `request_id` | Revoke viewer access and stop the tunnel |

Only one request can be pending or active. The same task key returns its existing request without another notification. A different key conflicts. Failed pending notifications can be retried using the same key.

The agent must wait for the configured owner to reply `ready <request_id>` before calling `start_handoff` with `user_ready=true`. The helper checks its bearer token, request ID, phase and deadline. It does not poll Telegram or independently prove the human's reply. An agent with its MCP credential can assert readiness. This is an operating procedure, not a security boundary against a compromised agent.

Cloudflared forwards only to the loopback viewer listener. Its public endpoint cannot reach MCP or browser automation. The initial random link sets a Secure, HttpOnly, SameSite cookie and redirects away from the secret-bearing path. Viewer and WebSocket requests require that credential; foreign Origin headers are rejected.

The link controls the entire browser desktop. Treat it as a temporary password. It is sent through Telegram, whose chat history retains it. Link previews are disabled. The helper does not log or persist the viewer credential.

Completion, expiry, shutdown and child exit revoke access. The helper stops and reaps its own cloudflared child. Killing the tunnel closes its forwarded connections. Restart expires active requests but preserves pending requests. Tunnels are never recreated automatically.

State is atomically saved in `data/handoff/handoff.json`. Run one replica with that directory. Multi-replica coordination is not implemented.

Quick Tunnels require outbound Cloudflare access and use an ephemeral `trycloudflare.com` hostname. No Cloudflare account key is needed. They are temporary assistance ingress, not permanent hosting. Cloudflare handles public TLS, so do not use this path for material you cannot entrust to that service.
