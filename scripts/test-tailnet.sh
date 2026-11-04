#!/bin/bash
set -euo pipefail
umask 077
LOG_FILE="${LOG_FILE:-/tmp/autonomous-stack-tailnet.log}"
exec > >(tee -a "$LOG_FILE") 2>&1
cd "$(dirname "${BASH_SOURCE[0]}")/.."
root="$PWD"
prefix="autonomous-stack-tailnet-$$"
fixture=$(mktemp -d "$root/.testing/tailnet.XXXXXX")
names=()
network_created=false
log() {
	printf '{"time":"%s","level":"%s","file":"test-tailnet.sh","line":%s,"func":"%s","msg":"%s"}\n' \
		"$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1" "${BASH_LINENO[0]}" "${FUNCNAME[1]:-main}" "$2" >&2
}
cleanup() {
	local name index
	for ((index = ${#names[@]} - 1; index >= 0; index -= 1)); do
		name=${names[index]}
		docker logs "$name" >"$fixture/$name.log" 2>&1 || log WARN 'Could not collect fixture log.'
		docker stop --time 10 "$name" >/dev/null || log WARN 'Could not stop owned fixture.'
	done
	if [[ "$network_created" == true ]]; then
		docker network rm "$prefix" >/dev/null || log WARN 'Could not remove fixture network.'
	fi
}
trap cleanup EXIT
trap 'log ERROR "Tailnet test failed at line $LINENO"' ERR
[[ $# == 0 ]] || {
	log ERROR 'Usage: make test-tailnet'
	exit 2
}
readonly headscale_image=headscale/headscale:0.29.6@sha256:cabe84f0699913dbca9320f0883dca7565dd08df3c5be29edfabbebeff763966
tailscale_image=$(docker compose config --format json | jq -r '.services.tailscale.image')
readonly tailscale_image
readonly token=EXAMPLE-DO-NOT-USE-0123456789-TEST-TOKEN
mkdir -p "$fixture/controller" "$fixture/server" "$fixture/client" "$fixture/handoff" "$fixture/browser"
docker run --rm --network none --user "$(id -u):$(id -g)" -e HOME=/tmp \
	-v "$root:$root" -w "$root" autonomous-stack-dev \
	go run -mod=vendor ./tests/testinfra/telegram --certificates "$fixture/controller"
docker network create "$prefix" >/dev/null
network_created=true
docker run --rm -d --name "$prefix-controller" --network "$prefix" --network-alias headscale \
	-v "$root/tests/testinfra/headscale.yml:/etc/headscale/config.yaml:ro" \
	-v "$fixture/controller:/var/lib/headscale" "$headscale_image" serve >/dev/null
names+=("$prefix-controller")
ready=false
for ((attempt = 0; attempt < 60; attempt += 1)); do
	if docker exec "$prefix-controller" headscale users list -o json >"$fixture/users.json" 2>"$fixture/control-error.log"; then
		ready=true
		break
	fi
	sleep 1
done
[[ "$ready" == true ]] || {
	log ERROR 'Controller failed readiness.'
	exit 1
}
docker exec "$prefix-controller" headscale users create fixture -o json >"$fixture/user.json"
user_id=$(jq -r '.id' "$fixture/user.json")
docker exec "$prefix-controller" headscale preauthkeys create --user "$user_id" --reusable --expiration 1h -o json >"$fixture/key.json"
auth_key=$(jq -r '.key' "$fixture/key.json")
[[ -n "$auth_key" && "$auth_key" != null ]] || {
	log ERROR 'No fixture key.'
	exit 1
}
docker run --rm -d --name "$prefix-telegram" --network "$prefix" --network-alias telegram --init \
	--user "$(id -u):$(id -g)" -e HOME=/tmp -v "$root:$root" -w "$root" \
	autonomous-stack-dev go run -race -mod=vendor ./tests/testinfra/telegram "$fixture/link" >/dev/null
names+=("$prefix-telegram")
docker run --rm -d --name "$prefix-browser" --network "$prefix" --network-alias browser --init \
	--memory 2g --cpus 2 --pids-limit 512 --shm-size 512m --cap-drop ALL \
	--cap-add CHOWN --cap-add DAC_OVERRIDE --cap-add FOWNER --cap-add SETUID --cap-add SETGID \
	--security-opt no-new-privileges:true -v "$fixture/browser:/userdata" \
	-e "AUTH_TOKEN=$token" -e "PUID=$(id -u)" -e "PGID=$(id -g)" -e XVFB_RESOLUTION=1440x900 \
	psyb0t/stealthy-auto-browse:latest >/dev/null
names+=("$prefix-browser")
docker run --rm -d --name "$prefix-handoff" --network "$prefix" --network-alias handoff --init \
	--user "$(id -u):$(id -g)" --cap-drop ALL --read-only --tmpfs /tmp:size=64m \
	-v "$fixture/handoff:/state" -e SERVICES_ENABLED=handoff -e ENV=prod -e LOG_FORMAT=json \
	-e "AUTONOMOUS_TOKEN=$token" -e TELEGRAM_BOT_TOKEN=EXAMPLE-DO-NOT-USE -e TELEGRAM_CHAT_ID=42 \
	-e AUTONOMOUS_TELEGRAM_API_URL=http://telegram:8089 \
	psyb0t/autonomous-stack:latest run >/dev/null
names+=("$prefix-handoff")
for role in server client; do
	hostname=autonomous-stack
	[[ "$role" == server ]] || hostname='fixture-client'
	docker run --rm -d --name "$prefix-$role" --network "$prefix" --init \
		--user "$(id -u):$(id -g)" --cap-drop ALL --read-only \
		--tmpfs /tmp:size=64m --tmpfs /var/run:size=8m,mode=1777 \
		-v "$fixture/$role:/var/lib/tailscale" \
		-v "$fixture/controller/cert.pem:/fixture-ca.pem:ro" -e SSL_CERT_FILE=/fixture-ca.pem \
		-e "TS_AUTHKEY=$auth_key" -e "TS_HOSTNAME=$hostname" -e TS_STATE_DIR=/var/lib/tailscale \
		-e TS_AUTH_ONCE=true -e TS_USERSPACE=true \
		-e TS_SOCKS5_SERVER=0.0.0.0:1055 \
		-e 'TS_EXTRA_ARGS=--accept-dns=false --login-server=https://headscale:8080' \
		-e TS_ENABLE_HEALTH_CHECK=true -e TS_LOCAL_ADDR_PORT=127.0.0.1:9002 \
		"$tailscale_image" >/dev/null
	names+=("$prefix-$role")
	ready=false
	for ((attempt = 0; attempt < 90; attempt += 1)); do
		if docker exec "$prefix-$role" wget -q --spider http://127.0.0.1:9002/healthz; then
			ready=true
			break
		fi
		sleep 1
	done
	[[ "$ready" == true ]] || {
		log ERROR 'Tailnet node failed readiness.'
		exit 1
	}
done
ready=false
for ((attempt = 0; attempt < 90; attempt += 1)); do
	if docker exec "$prefix-server" wget -q --spider http://telegram:8089 &&
		docker exec "$prefix-browser" curl -fsS -o /dev/null http://127.0.0.1:5900/vnc.html; then
		ready=true
		break
	fi
	sleep 1
done
[[ "$ready" == true ]] || {
	log ERROR 'Notification or browser fixture failed readiness.'
	exit 1
}
docker exec "$prefix-server" tailscale serve --bg --yes --http=80 http://handoff:8081 >"$fixture/serve.log"
tail_request() {
	docker exec "$prefix-telegram" curl -sS -i --max-time 20 \
		--retry 5 --retry-all-errors --retry-delay 1 \
		--socks5-hostname "$prefix-client:1055" \
		-H "Cookie: ${2:-}" "http://autonomous-stack$1"
}
tail_request /vnc.html >"$fixture/before.http"
grep -q '200 OK' "$fixture/before.http"
mcp_call() {
	docker exec "$prefix-server" wget -qO- \
		--header="Authorization: Bearer $token" --header='Content-Type: application/json' \
		--header='Accept: application/json, text/event-stream' \
		--post-data="$1" http://handoff:8080/mcp
}
mcp_call '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_help","arguments":{"key":"fixture","reason":"Local tailnet test"}}}' >"$fixture/request.json"
request_id=$(jq -r '.result.structuredContent.id' "$fixture/request.json")
[[ -n "$request_id" && "$request_id" != null ]] || {
	log ERROR 'No request ID.'
	exit 1
}
mcp_call "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"start_handoff\",\"arguments\":{\"request_id\":\"$request_id\",\"user_ready\":true}}}" >"$fixture/start.json"
link=$(<"$fixture/link")
path=${link#http://autonomous-stack}
tail_request "$path" >"$fixture/connect.http"
grep -q '303 See Other' "$fixture/connect.http"
if grep -qi '^Set-Cookie:' "$fixture/connect.http"; then
	log ERROR 'Persistent viewer must not require a handoff cookie.'
	exit 1
fi
tail_request /vnc.html >"$fixture/viewer.http"
grep -q '200 OK' "$fixture/viewer.http"
grep -qi 'novnc' "$fixture/viewer.http"
mcp_call "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"finish_handoff\",\"arguments\":{\"request_id\":\"$request_id\"}}}" >"$fixture/finish.json"
tail_request /vnc.html >"$fixture/after.http"
grep -q '200 OK' "$fixture/after.http"
node_id=$(docker exec "$prefix-server" tailscale status --json | jq -r '.Self.ID')
docker restart "$prefix-server" >/dev/null
ready=false
for ((attempt = 0; attempt < 60; attempt += 1)); do
	if docker exec "$prefix-server" wget -q --spider http://127.0.0.1:9002/healthz; then
		ready=true
		break
	fi
	sleep 1
done
[[ "$ready" == true ]] || {
	log ERROR 'Restarted sidecar failed readiness.'
	exit 1
}
[[ "$node_id" == "$(docker exec "$prefix-server" tailscale status --json | jq -r '.Self.ID')" ]]
tail_request /vnc.html >"$fixture/restarted.http"
grep -q '200 OK' "$fixture/restarted.http"
log INFO 'Persistent tailnet viewer passed before and after handoff completion and sidecar restart.'
