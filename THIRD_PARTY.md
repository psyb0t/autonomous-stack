# Third-party software

The project code is MIT licensed. It builds on [servicepack](https://github.com/psyb0t/servicepack), whose copyright and license are retained in `LICENSE`.

Go dependencies are pinned in `go.mod` and vendored with their own license files. The runtime image includes the project license and vendored notices under `/usr/share/licenses/autonomous-stack/`. That notice collection also covers development dependencies present in the source checkout; it does not mean every vendored tool is linked into the runtime binary.

The image includes [cloudflared](https://github.com/cloudflare/cloudflared), under Apache License 2.0. Its unmodified release license is in `licenses/cloudflared-LICENSE` and copied into the runtime notice directory. The binary is taken from the digest-pinned upstream image declared in `Dockerfile`.

Claudebox, Mailbox and Stealthy Auto-Browse are separate Compose images, governed by their respective repositories and included software licenses. Claude Code is installed by Claudebox's supported startup procedure and remains subject to Anthropic's terms; it is not included in the Go helper image.
