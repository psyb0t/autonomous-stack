#!/bin/bash
# Literal dollars in synthetic keys and jq programs must not expand in Bash.
# shellcheck disable=SC2016
set -Eeuo pipefail
umask 077
LOG_FILE="${LOG_FILE:-/tmp/autonomous-stack-captcha-config.log}"
exec > >(tee -a "$LOG_FILE") 2>&1
cd "$(dirname "${BASH_SOURCE[0]}")/.."
log() {
	printf '{"time":"%s","level":"%s","file":"test-captcha-config.sh","line":%s,"func":"%s","msg":"%s"}\n' \
		"$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1" "${BASH_LINENO[0]}" "${FUNCNAME[1]:-main}" "$2" >&2
}
trap 'log ERROR "Configuration test failed at line $LINENO"' ERR
rendered=$(mktemp)
trap 'rm -f "$rendered"' EXIT
check() {
	docker run --rm -i --network none --read-only --cap-drop ALL \
		--security-opt no-new-privileges:true --user "$(id -u):$(id -g)" \
		"${DEV_IMAGE:?DEV_IMAGE is required; use make test-captcha-config}" jq -e "$@" <"$rendered" >/dev/null
}
for provider in '' nopecha capsolver capmonster nocaptchaai yescaptcha 2captcha anticaptcha; do
	enabled=true
	key='EXAMPLE-DO-NOT-USE $literal:# unicode-é'
	if [[ -z "$provider" ]]; then
		enabled=false
		key=''
	fi
	CAPTCHA_SOLVER_ENABLED="$enabled" CAPTCHA_SOLVER_PROVIDER="$provider" CAPTCHA_SOLVER_API_KEY="$key" \
		docker compose --env-file .env.example config --no-env-resolution --format json >"$rendered"
	check --arg enabled "$enabled" --arg provider "$provider" --arg key "$key" '
		.services.browser.environment as $browser |
		$browser.CAPTCHA_SOLVER_ENABLED == $enabled and
		$browser.CAPTCHA_SOLVER_PROVIDER == $provider and
		$browser.CAPTCHA_SOLVER_API_KEY == ($key | gsub("\\$"; "$$")) and
		([.services | to_entries[] | select(.key != "browser") |
			.value.environment.CAPTCHA_SOLVER_API_KEY? // empty | select(length > 0)] | length == 0)
	'
done
env -u CAPTCHA_SOLVER_ENABLED -u CAPTCHA_SOLVER_PROVIDER -u CAPTCHA_SOLVER_API_KEY \
	docker compose --env-file .env.example config --no-env-resolution --format json >"$rendered"
check '.services.browser.environment |
	.CAPTCHA_SOLVER_ENABLED == "false" and .CAPTCHA_SOLVER_PROVIDER == "" and .CAPTCHA_SOLVER_API_KEY == ""'
log INFO 'CAPTCHA settings passed for disabled defaults and all seven providers.'
