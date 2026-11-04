FROM cloudflare/cloudflared:2026.10.5@sha256:1b1d34d0a9c0bfca9de72db8460d328db3f19edd2b96169114e3e893a97bbe53 AS cloudflared
FROM golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS builder
ARG BUILD_COMMIT=""
ARG BUILD_VERSION="dev"
WORKDIR /app
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY LICENSE ./LICENSE
COPY licenses ./licenses
RUN mkdir -p /notices/vendor && cp LICENSE /notices/LICENSE && \
    cp licenses/cloudflared-LICENSE /notices/cloudflared-LICENSE && \
    find vendor -type f \( -iname '*license*' -o -iname '*notice*' -o -iname '*copying*' \) \
    -exec sh -c 'for source do destination="/notices/$source"; mkdir -p "$(dirname "$destination")"; cp "$source" "$destination"; done' sh {} +
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath \
    -ldflags "-s -w -X main.appName=autonomous-stack -X main.buildCommit=${BUILD_COMMIT} -X main.buildVersion=${BUILD_VERSION}" \
    -o /app/app ./cmd
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser
COPY --from=builder --chown=10001:10001 /app/app /app/app
COPY --from=cloudflared /usr/local/bin/cloudflared /usr/local/bin/cloudflared
COPY --from=builder /notices /usr/share/licenses/autonomous-stack
WORKDIR /app
ENV HOME=/tmp
USER 10001:10001
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 CMD ["./app", "healthcheck"]
ENTRYPOINT ["./app"]
CMD ["run"]
