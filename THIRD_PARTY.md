# Third-party software

The project code is MIT licensed. It builds on [servicepack](https://github.com/psyb0t/servicepack), whose copyright and license are retained in `LICENSE`.

Go dependencies are pinned in `go.mod` and vendored with their own license files. The runtime image includes the project license and vendored notices under `/usr/share/licenses/autonomous-stack/`. That notice collection also covers development dependencies present in the source checkout; it does not mean every vendored tool is linked into the runtime binary.

The [Tailscale](https://github.com/tailscale/tailscale) sidecar is a separate, digest-pinned upstream image. It provides private tailnet access and is governed by its own license and included software notices. No Tailscale executable is linked into or copied into the Go helper image.

[Claudebox](https://github.com/psyb0t/docker-claudebox), [Mailbox](https://github.com/psyb0t/docker-mailbox) and [Stealthy Auto-Browse](https://github.com/psyb0t/docker-stealthy-auto-browse) are separate Compose images, governed by their respective repositories and included software licenses. Claude Code is installed by Claudebox's supported startup procedure and remains subject to Anthropic's terms; it is not included in the Go helper image.
