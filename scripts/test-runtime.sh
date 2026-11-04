#!/bin/bash
set -euo pipefail
umask 077
LOG_FILE="${LOG_FILE:-/tmp/autonomous-stack-runtime.log}"
exec > >(tee -a "$LOG_FILE") 2>&1
cd "$(dirname "${BASH_SOURCE[0]}")/.."
root="$PWD"
prefix="autonomous-stack-test-$$"
fixture_root=$(mktemp -d "$root/.testing/runtime.XXXXXX")
names=()
network_created=false
log() {
	printf '{"time":"%s","level":"%s","file":"test-runtime.sh","line":%s,"func":"%s","msg":"%s"}\n' \
		"$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1" "${BASH_LINENO[0]}" "${FUNCNAME[1]:-main}" "$2" >&2
}
cleanup() {
	local name
	for name in "${names[@]}"; do
		docker logs "$name" >"$fixture_root/$name.log" 2>&1 || log WARN 'Could not collect a fixture log.'
		docker stop --time 15 "$name" >/dev/null || log WARN 'Could not stop an owned fixture.'
	done
	if [[ "$network_created" == true ]]; then
		docker network rm "$prefix" >/dev/null || log WARN 'Could not remove the fixture network.'
	fi
}
trap cleanup EXIT
trap 'log ERROR "Runtime test failed at line $LINENO"' ERR
mkdir -p "$fixture_root/config/generated" "$fixture_root/data/workspace" "$fixture_root/data/mailbox" "$fixture_root/data/handoff" "$fixture_root/data/browser"
readonly fixture_token=EXAMPLE-DO-NOT-USE-0123456789-TEST-TOKEN
docker run --rm --name "$prefix-prepare" --user "$(id -u):$(id -g)" --network none \
	--cap-drop ALL --read-only --security-opt no-new-privileges:true \
	-v "$fixture_root:/stack" \
	-e TELEGRAM_BOT_TOKEN=EXAMPLE-DO-NOT-USE -e TELEGRAM_CHAT_ID=42 -e TELEGRAM_USER_ID=84 \
	-e "AUTONOMOUS_TOKEN=$fixture_token" -e "BROWSER_TOKEN=$fixture_token" -e "MAILBOX_TOKEN=$fixture_token" \
	-e HEARTBEAT_PROMPT='Check fixture tasks' -e AGENT_PROMPT='Test fixture only' \
	-e MAIL_IMAP_HOST=imap.example.com -e MAIL_SMTP_HOST=smtp.example.com \
	-e MAIL_USERNAME=fixture@example.com -e MAIL_PASSWORD=EXAMPLE-DO-NOT-USE -e MAIL_FROM=fixture@example.com \
	psyb0t/autonomous-stack:latest prepare
docker network create "$prefix" >/dev/null
network_created=true
docker run --rm -d --name "$prefix-browser" --network "$prefix" --network-alias browser \
	--init --memory 2g --cpus 2 --pids-limit 512 --shm-size 512m \
	--cap-drop ALL --cap-add CHOWN --cap-add DAC_OVERRIDE --cap-add FOWNER --cap-add SETUID --cap-add SETGID \
	--security-opt no-new-privileges:true --log-opt max-size=10m --log-opt max-file=2 \
	-v "$fixture_root/data/browser:/userdata" -e "AUTH_TOKEN=$fixture_token" \
	-e "PUID=$(id -u)" -e "PGID=$(id -g)" -e XVFB_RESOLUTION=1440x900 \
	psyb0t/stealthy-auto-browse:latest >/dev/null
names+=("$prefix-browser")
docker run --rm -d --name "$prefix-mailbox" --network "$prefix" --network-alias mailbox \
	--init --user "$(id -u):$(id -g)" --read-only --cap-drop ALL \
	--memory 512m --cpus 1 --pids-limit 128 --security-opt no-new-privileges:true \
	--log-opt max-size=10m --log-opt max-file=2 --tmpfs /tmp:size=64m,noexec,nosuid \
	-v "$fixture_root/config/generated/mailbox.yml:/etc/mailboxd/config.yaml:ro" \
	-v "$fixture_root/data/mailbox:/var/lib/mailboxd" \
	-e MAILBOXD_CONFIG=/etc/mailboxd/config.yaml -e "MAILBOX_TOKEN=$fixture_token" \
	-e MAIL_USERNAME=fixture@example.com -e MAIL_PASSWORD=EXAMPLE-DO-NOT-USE \
	psyb0t/mailbox:latest >/dev/null
names+=("$prefix-mailbox")
docker run --rm -d --name "$prefix-handoff" --network "$prefix" --network-alias handoff \
	--init --user "$(id -u):$(id -g)" --read-only --cap-drop ALL \
	--memory 256m --cpus 1 --pids-limit 128 --security-opt no-new-privileges:true \
	--log-opt max-size=10m --log-opt max-file=2 --tmpfs /tmp:size=64m,noexec,nosuid \
	-v "$fixture_root/data/handoff:/state" \
	-e SERVICES_ENABLED=handoff -e ENV=prod -e LOG_LEVEL=info -e LOG_FORMAT=json \
	-e "AUTONOMOUS_TOKEN=$fixture_token" -e TELEGRAM_BOT_TOKEN=EXAMPLE-DO-NOT-USE -e TELEGRAM_CHAT_ID=42 \
	psyb0t/autonomous-stack:latest run >/dev/null
names+=("$prefix-handoff")
ready=false
for ((attempt = 0; attempt < 60; attempt += 1)); do
	if docker exec "$prefix-handoff" ./app healthcheck >/dev/null 2>&1 &&
		docker exec "$prefix-browser" curl -fsS -o /dev/null http://127.0.0.1:8080/health &&
		docker exec "$prefix-browser" curl -fsS -o /dev/null http://mailbox:8000/health; then
		ready=true
		break
	fi
	sleep 1
done
[[ "$ready" == true ]] || {
	log ERROR 'Fixture services failed readiness.'
	exit 1
}
docker exec "$prefix-browser" curl -fsS -o /dev/null http://127.0.0.1:5900/vnc.html
status=$(docker exec "$prefix-browser" curl -s -o /dev/null -w '%{http_code}' http://handoff:8080/mcp)
[[ "$status" == 401 ]] || {
	log ERROR 'Unauthenticated MCP was not rejected.'
	exit 1
}
docker exec "$prefix-browser" curl -fsS \
	-H "Authorization: Bearer $fixture_token" -H 'Content-Type: application/json' \
	-H 'Accept: application/json, text/event-stream' \
	-d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' http://handoff:8080/mcp >"$fixture_root/tools.json"
log INFO 'Runtime checks passed. No Claude, Telegram polling or public tunnel was started.'
